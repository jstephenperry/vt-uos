package testutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/vtuos/vtuos/internal/models"
)

// FixtureResident creates a test resident with sensible defaults.
func FixtureResident(overrides ...func(*models.Resident)) *models.Resident {
	id := uuid.New().String()
	now := time.Now().UTC()
	dob := now.AddDate(-30, 0, 0) // 30 years old

	resident := &models.Resident{
		ID:             id,
		RegistryNumber: "VT-076-" + id[:8],
		Surname:        "Doe",
		GivenNames:     "John",
		DateOfBirth:    dob,
		Sex:            models.SexMale,
		BloodType:      models.BloodTypeOPos,
		EntryType:      models.EntryTypeOriginal,
		EntryDate:      now.AddDate(-1, 0, 0), // 1 year ago
		Status:         models.ResidentStatusActive,
		ClearanceLevel: 3,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	for _, override := range overrides {
		override(resident)
	}

	return resident
}

// FixtureFemaleResident creates a female test resident.
func FixtureFemaleResident(overrides ...func(*models.Resident)) *models.Resident {
	return FixtureResident(append([]func(*models.Resident){
		func(r *models.Resident) {
			r.Sex = models.SexFemale
			r.GivenNames = "Jane"
		},
	}, overrides...)...)
}

// FixtureVaultBornResident creates a vault-born resident with parents.
func FixtureVaultBornResident(parent1ID, parent2ID string, overrides ...func(*models.Resident)) *models.Resident {
	return FixtureResident(append([]func(*models.Resident){
		func(r *models.Resident) {
			r.EntryType = models.EntryTypeVaultBorn
			r.BiologicalParent1ID = &parent1ID
			r.BiologicalParent2ID = &parent2ID
			r.DateOfBirth = time.Now().UTC().AddDate(-5, 0, 0) // 5 years old
		},
	}, overrides...)...)
}

// FixtureDeceasedResident creates a deceased resident.
func FixtureDeceasedResident(overrides ...func(*models.Resident)) *models.Resident {
	deathDate := time.Now().UTC().AddDate(0, -1, 0) // Died 1 month ago
	return FixtureResident(append([]func(*models.Resident){
		func(r *models.Resident) {
			r.Status = models.ResidentStatusDeceased
			r.DateOfDeath = &deathDate
		},
	}, overrides...)...)
}

// FixtureHousehold creates a test household with sensible defaults.
func FixtureHousehold(overrides ...func(*models.Household)) *models.Household {
	id := uuid.New().String()
	now := time.Now().UTC()

	household := &models.Household{
		ID:            id,
		Designation:   "HH-" + id[:8],
		HouseholdType: models.HouseholdTypeFamily,
		RationClass:   models.RationClassStandard,
		Status:        models.HouseholdStatusActive,
		FormedDate:    now.AddDate(-1, 0, 0), // Formed 1 year ago
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	for _, override := range overrides {
		override(household)
	}

	return household
}

// FixtureIndividualHousehold creates an individual household.
func FixtureIndividualHousehold(overrides ...func(*models.Household)) *models.Household {
	return FixtureHousehold(append([]func(*models.Household){
		func(h *models.Household) {
			h.HouseholdType = models.HouseholdTypeIndividual
		},
	}, overrides...)...)
}

// FixtureDissolvedHousehold creates a dissolved household.
func FixtureDissolvedHousehold(overrides ...func(*models.Household)) *models.Household {
	dissolvedDate := time.Now().UTC().AddDate(0, -2, 0) // Dissolved 2 months ago
	return FixtureHousehold(append([]func(*models.Household){
		func(h *models.Household) {
			h.Status = models.HouseholdStatusDissolved
			h.DissolvedDate = &dissolvedDate
		},
	}, overrides...)...)
}

// FixtureQuarters creates test quarters with sensible defaults.
func FixtureQuarters(overrides ...func(*models.Quarters)) *models.Quarters {
	id := uuid.New().String()
	now := time.Now().UTC()

	quarters := &models.Quarters{
		ID:           id,
		UnitCode:     "Q-" + id[:8],
		Sector:       "A",
		Level:        1,
		UnitType:     models.QuartersTypeFamily,
		Capacity:     5,
		SquareMeters: 50.0,
		Status:       models.QuartersStatusAvailable,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	for _, override := range overrides {
		override(quarters)
	}

	return quarters
}

// FixtureResourceCategory creates a test resource category.
func FixtureResourceCategory(overrides ...func(*models.ResourceCategory)) *models.ResourceCategory {
	id := uuid.New().String()
	now := time.Now().UTC()

	category := &models.ResourceCategory{
		ID:            id,
		Code:          "FOOD",
		Name:          "Food",
		Description:   "Food and nutrition",
		UnitOfMeasure: "kg",
		IsConsumable:  true,
		IsCritical:    true,
		CreatedAt:     now,
	}

	for _, override := range overrides {
		override(category)
	}

	return category
}

// FixtureResourceItem creates a test resource item.
func FixtureResourceItem(categoryID string, overrides ...func(*models.ResourceItem)) *models.ResourceItem {
	id := uuid.New().String()
	now := time.Now().UTC()
	calories := 250.0
	shelfLife := 365

	item := &models.ResourceItem{
		ID:              id,
		CategoryID:      categoryID,
		ItemCode:        "FOOD-PROTEIN-001",
		Name:            "Protein Ration",
		Description:     "High-protein meal ration",
		UnitOfMeasure:   "unit",
		CaloriesPerUnit: &calories,
		ShelfLifeDays:   &shelfLife,
		IsProducible:    false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	for _, override := range overrides {
		override(item)
	}

	return item
}

// FixtureResourceStock creates test resource stock.
func FixtureResourceStock(itemID string, overrides ...func(*models.ResourceStock)) *models.ResourceStock {
	id := uuid.New().String()
	now := time.Now().UTC()
	expiration := now.AddDate(1, 0, 0) // Expires in 1 year

	stock := &models.ResourceStock{
		ID:              id,
		ItemID:          itemID,
		Quantity:        100.0,
		StorageLocation: "STORAGE-A-12",
		ReceivedDate:    now.AddDate(-1, 0, 0),
		ExpirationDate:  &expiration,
		Status:          models.StockStatusAvailable,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	for _, override := range overrides {
		override(stock)
	}

	return stock
}

// FixtureResourceTransaction creates a test resource transaction.
func FixtureResourceTransaction(itemID string, overrides ...func(*models.ResourceTransaction)) *models.ResourceTransaction {
	id := uuid.New().String()
	now := time.Now().UTC()

	transaction := &models.ResourceTransaction{
		ID:              id,
		ItemID:          itemID,
		TransactionType: models.TransactionTypeConsumption,
		Quantity:        -10.0, // Consumption is negative
		BalanceAfter:    90.0,
		Reason:          "Daily ration distribution",
		Timestamp:       now,
		CreatedAt:       now,
	}

	for _, override := range overrides {
		override(transaction)
	}

	return transaction
}

// FixtureFacilitySystem creates a test facility system with sensible defaults.
func FixtureFacilitySystem(overrides ...func(*models.FacilitySystem)) *models.FacilitySystem {
	id := uuid.New().String()
	now := time.Now().UTC()
	installDate := now.AddDate(-2, 0, 0)
	nextMaint := now.AddDate(0, 1, 0)

	system := &models.FacilitySystem{
		ID:                      id,
		SystemCode:              "PWR-GEN-" + id[:4],
		Name:                    "Generator Unit " + id[:4],
		Category:                models.SystemCategoryPower,
		LocationSector:          "A",
		LocationLevel:           2,
		Status:                  models.SystemStatusOperational,
		EfficiencyPercent:       95.0,
		InstallDate:             installDate,
		NextMaintenanceDue:      &nextMaint,
		MaintenanceIntervalDays: 90,
		TotalRuntimeHours:       17520,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	for _, override := range overrides {
		override(system)
	}

	return system
}

// FixtureDegradedSystem creates a degraded facility system.
func FixtureDegradedSystem(overrides ...func(*models.FacilitySystem)) *models.FacilitySystem {
	return FixtureFacilitySystem(append([]func(*models.FacilitySystem){
		func(s *models.FacilitySystem) {
			s.Status = models.SystemStatusDegraded
			s.EfficiencyPercent = 72.0
			s.Category = models.SystemCategoryWaste
			s.Name = "Waste Processing"
			s.SystemCode = "WST-PROC-01"
		},
	}, overrides...)...)
}

// FixtureMaintenanceRecord creates a test maintenance record.
func FixtureMaintenanceRecord(systemID string, overrides ...func(*models.MaintenanceRecord)) *models.MaintenanceRecord {
	id := uuid.New().String()
	now := time.Now().UTC()
	scheduled := now.AddDate(0, 0, 7)

	record := &models.MaintenanceRecord{
		ID:              id,
		SystemID:        systemID,
		MaintenanceType: models.MaintenanceTypePreventive,
		Description:     "Routine preventive maintenance",
		ScheduledDate:   &scheduled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	for _, override := range overrides {
		override(record)
	}

	return record
}

// StringPtr returns a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to an int value.
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr returns a pointer to a float64 value.
func Float64Ptr(f float64) *float64 {
	return &f
}

// TimePtr returns a pointer to a time value.
func TimePtr(t time.Time) *time.Time {
	return &t
}
