package seed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/util"
)

// Config configures the seed data generator.
type Config struct {
	VaultNumber      int
	SealDate         time.Time
	TargetPopulation int
	FamilyHouseholds int
	SingleHouseholds int
	RandomSeed       int64
}

// DefaultConfig returns a default seed configuration.
func DefaultConfig(vaultNumber int) Config {
	return Config{
		VaultNumber:      vaultNumber,
		SealDate:         time.Date(2077, 10, 23, 9, 47, 0, 0, time.UTC),
		TargetPopulation: 500,
		FamilyHouseholds: 100,
		SingleHouseholds: 80,
		RandomSeed:       2077,
	}
}

// Generator generates seed data for a vault.
type Generator struct {
	db        *sql.DB
	cfg       Config
	rng       *rand.Rand
	idGen     *util.IDGenerator
	regNumGen *util.RegistryNumberGenerator

	// Tracking
	residentCount int
	residents     []*models.Resident
	households    []*models.Household
}

// NewGenerator creates a new seed data generator.
func NewGenerator(db *sql.DB, cfg Config) *Generator {
	return &Generator{
		db:        db,
		cfg:       cfg,
		rng:       rand.New(rand.NewSource(cfg.RandomSeed)),
		idGen:     util.NewIDGenerator(),
		regNumGen: util.NewRegistryNumberGenerator(cfg.VaultNumber),
	}
}

// Generate creates all seed data.
func (g *Generator) Generate(ctx context.Context) error {
	slog.Info("starting seed data generation",
		"vault", g.cfg.VaultNumber,
		"target_population", g.cfg.TargetPopulation,
	)

	// Start transaction
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Generate quarters first
	if err := g.generateQuarters(ctx, tx); err != nil {
		return fmt.Errorf("generating quarters: %w", err)
	}

	// Generate vocations
	if err := g.generateVocations(ctx, tx); err != nil {
		return fmt.Errorf("generating vocations: %w", err)
	}

	// Generate family households with members
	if err := g.generateFamilyHouseholds(ctx, tx); err != nil {
		return fmt.Errorf("generating family households: %w", err)
	}

	// Generate single-person households
	if err := g.generateSingleHouseholds(ctx, tx); err != nil {
		return fmt.Errorf("generating single households: %w", err)
	}

	// Fill remaining population if needed
	for g.residentCount < g.cfg.TargetPopulation {
		if err := g.generateSingleHousehold(ctx, tx); err != nil {
			return fmt.Errorf("generating additional resident: %w", err)
		}
	}

	// Generate resources
	if err := g.generateResources(ctx, tx); err != nil {
		return fmt.Errorf("generating resources: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	slog.Info("seed data generation complete",
		"residents", g.residentCount,
		"households", len(g.households),
	)

	return nil
}

func (g *Generator) generateQuarters(ctx context.Context, tx *sql.Tx) error {
	slog.Debug("generating quarters")

	query := `INSERT INTO quarters (
		id, unit_code, sector, level, unit_type, capacity,
		square_meters, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format(time.RFC3339)

	unitNum := 1
	for _, sector := range QuartersSectors {
		for level := 1; level <= QuartersLevels; level++ {
			// Mix of unit types per level
			units := []struct {
				Type     string
				Capacity int
				SqM      float64
				Count    int
			}{
				{"SINGLE", 1, 20, 5},
				{"DOUBLE", 2, 30, 8},
				{"FAMILY", 5, 55, 8},
				{"EXECUTIVE", 3, 70, 2},
				{"DORMITORY", 12, 100, 2},
			}

			for _, unit := range units {
				for i := 0; i < unit.Count; i++ {
					id := g.idGen.NewID()
					code := fmt.Sprintf("R-%s-%d%02d", sector, level, unitNum%100)

					_, err := tx.ExecContext(ctx, query,
						id, code, sector, level, unit.Type, unit.Capacity,
						unit.SqM, "AVAILABLE", now, now,
					)
					if err != nil {
						return fmt.Errorf("inserting quarters %s: %w", code, err)
					}
					unitNum++
				}
			}
		}
	}

	slog.Debug("quarters generated", "count", unitNum-1)
	return nil
}

func (g *Generator) generateVocations(ctx context.Context, tx *sql.Tx) error {
	slog.Debug("generating vocations")

	query := `INSERT INTO vocations (
		id, code, title, department, required_clearance,
		headcount_authorized, headcount_minimum, shift_pattern,
		hazard_level, is_active, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0

	for dept, vocations := range DepartmentVocations {
		for _, voc := range vocations {
			id := g.idGen.NewID()

			// Calculate headcounts based on department size
			authorized := 10
			minimum := 3
			if dept == "ADMINISTRATION" {
				authorized = 5
				minimum = 2
			}
			if voc.Code == "ADM-OVSR-01" {
				authorized = 1
				minimum = 1
			}

			_, err := tx.ExecContext(ctx, query,
				id, voc.Code, voc.Title, dept, voc.Clearance,
				authorized, minimum, "STANDARD",
				voc.HazardLevel, 1, now, now,
			)
			if err != nil {
				return fmt.Errorf("inserting vocation %s: %w", voc.Code, err)
			}
			count++
		}
	}

	slog.Debug("vocations generated", "count", count)
	return nil
}

func (g *Generator) generateFamilyHouseholds(ctx context.Context, tx *sql.Tx) error {
	slog.Debug("generating family households", "count", g.cfg.FamilyHouseholds)

	for i := 0; i < g.cfg.FamilyHouseholds && g.residentCount < g.cfg.TargetPopulation; i++ {
		if err := g.generateFamilyHousehold(ctx, tx); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateFamilyHousehold(ctx context.Context, tx *sql.Tx) error {
	// Family composition: 2 adults + 0-4 children
	numChildren := g.rng.Intn(5) // 0-4 children

	// Generate adults (couple)
	husbandAge := 25 + g.rng.Intn(35)          // 25-59
	wifeAge := husbandAge - 5 + g.rng.Intn(11) // ±5 years
	if wifeAge < 20 {
		wifeAge = 20
	}

	surname := Surnames[g.rng.Intn(len(Surnames))]

	husband := g.generateResident(surname, models.SexMale, husbandAge, nil, nil)
	wife := g.generateResident(surname, models.SexFemale, wifeAge, nil, nil)

	// Create household ID first
	householdID := g.idGen.NewID()
	designation := fmt.Sprintf("H-%04d", len(g.households)+1)

	household := &models.Household{
		ID:                householdID,
		Designation:       designation,
		HouseholdType:     models.HouseholdTypeFamily,
		HeadOfHouseholdID: &husband.ID,
		RationClass:       models.RationClassStandard,
		Status:            models.HouseholdStatusActive,
		FormedDate:        g.cfg.SealDate,
	}

	// Insert household FIRST (before residents that reference it)
	if err := g.insertHousehold(ctx, tx, household); err != nil {
		return err
	}

	// Assign household to residents
	husband.HouseholdID = &householdID
	wife.HouseholdID = &householdID

	// Insert adults
	if err := g.insertResident(ctx, tx, husband); err != nil {
		return err
	}
	if err := g.insertResident(ctx, tx, wife); err != nil {
		return err
	}

	// Generate children
	for c := 0; c < numChildren && g.residentCount < g.cfg.TargetPopulation; c++ {
		maxChildAge := husbandAge - 18
		if maxChildAge < 1 {
			continue
		}
		childAge := g.rng.Intn(maxChildAge)
		if childAge > 17 {
			childAge = 17 // Cap at 17
		}

		sex := models.SexMale
		if g.rng.Float32() < 0.5 {
			sex = models.SexFemale
		}

		child := g.generateResident(surname, sex, childAge, &husband.ID, &wife.ID)
		child.HouseholdID = &householdID

		if err := g.insertResident(ctx, tx, child); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateSingleHouseholds(ctx context.Context, tx *sql.Tx) error {
	slog.Debug("generating single households", "count", g.cfg.SingleHouseholds)

	for i := 0; i < g.cfg.SingleHouseholds && g.residentCount < g.cfg.TargetPopulation; i++ {
		if err := g.generateSingleHousehold(ctx, tx); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateSingleHousehold(ctx context.Context, tx *sql.Tx) error {
	surname := Surnames[g.rng.Intn(len(Surnames))]
	age := 18 + g.rng.Intn(47) // 18-64

	sex := models.SexMale
	if g.rng.Float32() < 0.5 {
		sex = models.SexFemale
	}

	resident := g.generateResident(surname, sex, age, nil, nil)

	// Create household first
	householdID := g.idGen.NewID()
	designation := fmt.Sprintf("H-%04d", len(g.households)+1)

	household := &models.Household{
		ID:                householdID,
		Designation:       designation,
		HouseholdType:     models.HouseholdTypeIndividual,
		HeadOfHouseholdID: &resident.ID,
		RationClass:       models.RationClassStandard,
		Status:            models.HouseholdStatusActive,
		FormedDate:        g.cfg.SealDate,
	}

	// Insert household FIRST
	if err := g.insertHousehold(ctx, tx, household); err != nil {
		return err
	}

	resident.HouseholdID = &householdID

	if err := g.insertResident(ctx, tx, resident); err != nil {
		return err
	}

	return nil
}

func (g *Generator) generateResident(surname string, sex models.Sex, age int, parent1ID, parent2ID *string) *models.Resident {
	var givenName string
	if sex == models.SexMale {
		givenName = MaleGivenNames[g.rng.Intn(len(MaleGivenNames))]
	} else {
		givenName = FemaleGivenNames[g.rng.Intn(len(FemaleGivenNames))]
	}

	// Add middle name 60% of the time
	if g.rng.Float32() < 0.6 {
		middle := MiddleNames[g.rng.Intn(len(MiddleNames))]
		givenName = givenName + " " + middle
	}

	// Calculate date of birth
	dob := g.cfg.SealDate.AddDate(-age, -g.rng.Intn(12), -g.rng.Intn(28))

	// Determine entry type
	entryType := models.EntryTypeOriginal
	if parent1ID != nil {
		entryType = models.EntryTypeVaultBorn
	}

	// Random blood type based on distribution
	bloodType := g.randomBloodType()

	// Clearance based on age
	clearance := 1
	if age >= 18 && age < 65 {
		clearance = 1 + g.rng.Intn(3) // 1-3 for most adults
	}

	id := g.idGen.NewID()
	regNum := g.regNumGen.Next()

	return &models.Resident{
		ID:                  id,
		RegistryNumber:      regNum,
		Surname:             surname,
		GivenNames:          givenName,
		DateOfBirth:         dob,
		Sex:                 sex,
		BloodType:           bloodType,
		EntryType:           entryType,
		EntryDate:           g.cfg.SealDate,
		Status:              models.ResidentStatusActive,
		BiologicalParent1ID: parent1ID,
		BiologicalParent2ID: parent2ID,
		ClearanceLevel:      clearance,
	}
}

func (g *Generator) randomBloodType() models.BloodType {
	total := 0
	for _, bt := range BloodTypes {
		total += bt.Weight
	}

	r := g.rng.Intn(total)
	cumulative := 0
	for _, bt := range BloodTypes {
		cumulative += bt.Weight
		if r < cumulative {
			return models.BloodType(bt.Type)
		}
	}

	return models.BloodTypeOPos // Fallback
}

func (g *Generator) insertResident(ctx context.Context, tx *sql.Tx, r *models.Resident) error {
	query := `INSERT INTO residents (
		id, registry_number, surname, given_names, date_of_birth,
		sex, blood_type, entry_type, entry_date, status,
		biological_parent_1_id, biological_parent_2_id,
		household_id, clearance_level, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := tx.ExecContext(ctx, query,
		r.ID, r.RegistryNumber, r.Surname, r.GivenNames,
		r.DateOfBirth.Format(time.DateOnly),
		string(r.Sex), string(r.BloodType), string(r.EntryType),
		r.EntryDate.Format(time.RFC3339), string(r.Status),
		r.BiologicalParent1ID, r.BiologicalParent2ID,
		r.HouseholdID, r.ClearanceLevel, now, now,
	)
	if err != nil {
		return fmt.Errorf("inserting resident %s: %w", r.RegistryNumber, err)
	}

	g.residents = append(g.residents, r)
	g.residentCount++

	return nil
}

func (g *Generator) insertHousehold(ctx context.Context, tx *sql.Tx, h *models.Household) error {
	query := `INSERT INTO households (
		id, designation, household_type, head_of_household_id,
		ration_class, status, formed_date, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := tx.ExecContext(ctx, query,
		h.ID, h.Designation, string(h.HouseholdType), h.HeadOfHouseholdID,
		string(h.RationClass), string(h.Status),
		h.FormedDate.Format(time.DateOnly), now, now,
	)
	if err != nil {
		return fmt.Errorf("inserting household %s: %w", h.Designation, err)
	}

	g.households = append(g.households, h)

	return nil
}

func (g *Generator) generateResources(ctx context.Context, tx *sql.Tx) error {
	slog.Debug("generating resources")

	now := time.Now().UTC().Format(time.RFC3339)

	// Create a map to store category IDs by code
	categoryIDs := make(map[string]string)

	// Generate categories
	catQuery := `INSERT INTO resource_categories (
		id, code, name, description, unit_of_measure,
		is_consumable, is_critical, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	for _, cat := range ResourceCategories {
		id := g.idGen.NewID()
		categoryIDs[cat.Code] = id

		isConsumable := 0
		if cat.IsConsumable {
			isConsumable = 1
		}
		isCritical := 0
		if cat.IsCritical {
			isCritical = 1
		}

		_, err := tx.ExecContext(ctx, catQuery,
			id, cat.Code, cat.Name, cat.Description, cat.UnitOfMeasure,
			isConsumable, isCritical, now,
		)
		if err != nil {
			return fmt.Errorf("inserting category %s: %w", cat.Code, err)
		}
	}

	slog.Debug("categories generated", "count", len(ResourceCategories))

	// Generate items and their initial stocks
	itemQuery := `INSERT INTO resource_items (
		id, category_id, item_code, name, description, unit_of_measure,
		calories_per_unit, shelf_life_days, storage_requirements,
		is_producible, production_rate_per_day, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stockQuery := `INSERT INTO resource_stocks (
		id, item_id, lot_number, quantity, quantity_reserved,
		storage_location, received_date, expiration_date, status,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, item := range ResourceItems {
		categoryID := categoryIDs[item.CategoryCode]
		if categoryID == "" {
			continue
		}

		itemID := g.idGen.NewID()

		// Handle optional fields
		var calories, prodRate interface{}
		if item.CaloriesPerUnit > 0 {
			calories = item.CaloriesPerUnit
		}
		if item.ProdRatePerDay > 0 {
			prodRate = item.ProdRatePerDay
		}

		var shelfLife interface{}
		if item.ShelfLifeDays > 0 {
			shelfLife = item.ShelfLifeDays
		}

		isProducible := 0
		if item.IsProducible {
			isProducible = 1
		}

		_, err := tx.ExecContext(ctx, itemQuery,
			itemID, categoryID, item.ItemCode, item.Name, item.Description,
			item.UnitOfMeasure, calories, shelfLife, nil,
			isProducible, prodRate, now, now,
		)
		if err != nil {
			return fmt.Errorf("inserting item %s: %w", item.ItemCode, err)
		}

		// Create initial stock for this item
		stockID := g.idGen.NewID()
		lotNumber := fmt.Sprintf("LOT-%s-2077", item.ItemCode)

		// Calculate initial quantity based on population needs
		var quantity float64
		switch item.CategoryCode {
		case "FOOD":
			// 90 days of food supply per person
			quantity = float64(g.cfg.TargetPopulation) * 0.5 * 90 // 0.5 kg per person per day
		case "WATER":
			// 30 days of water supply
			quantity = float64(g.cfg.TargetPopulation) * 3.0 * 30 // 3 liters per person per day
		case "MEDICAL":
			// Medical supplies per 100 residents
			quantity = float64(g.cfg.TargetPopulation) * 2.0
		default:
			// Default stockpile
			quantity = float64(g.cfg.TargetPopulation) * 0.5
		}

		// Calculate expiration date if applicable
		var expirationDate interface{}
		if item.ShelfLifeDays > 0 {
			expDate := g.cfg.SealDate.AddDate(0, 0, item.ShelfLifeDays)
			expirationDate = expDate.Format(time.RFC3339)
		}

		storageLocation := fmt.Sprintf("STORAGE-%s-01", item.CategoryCode[:4])

		_, err = tx.ExecContext(ctx, stockQuery,
			stockID, itemID, lotNumber, quantity, 0,
			storageLocation, g.cfg.SealDate.Format(time.RFC3339), expirationDate,
			"AVAILABLE", now, now,
		)
		if err != nil {
			return fmt.Errorf("inserting stock for %s: %w", item.ItemCode, err)
		}
	}

	slog.Debug("items and stocks generated", "count", len(ResourceItems))

	return nil
}
