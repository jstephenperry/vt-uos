// Package population provides population management services for VT-UOS.
package population

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/repository"
	"github.com/vtuos/vtuos/internal/util"
)

// Service provides population management operations.
type Service struct {
	db          *sql.DB
	vaultNumber int
	residents   *repository.ResidentRepository
	households  *repository.HouseholdRepository
	idGenerator *util.IDGenerator
	regNumGen   *util.RegistryNumberGenerator
}

// NewService creates a new population service.
func NewService(db *sql.DB, vaultNumber int) *Service {
	return &Service{
		db:          db,
		vaultNumber: vaultNumber,
		residents:   repository.NewResidentRepository(db),
		households:  repository.NewHouseholdRepository(db),
		idGenerator: util.NewIDGenerator(),
		regNumGen:   util.NewRegistryNumberGenerator(vaultNumber),
	}
}

// CreateResidentInput contains data for creating a new resident.
type CreateResidentInput struct {
	Surname             string
	GivenNames          string
	DateOfBirth         time.Time
	Sex                 models.Sex
	BloodType           models.BloodType
	EntryType           models.EntryType
	EntryDate           time.Time
	BiologicalParent1ID *string
	BiologicalParent2ID *string
	HouseholdID         *string
	ClearanceLevel      int
	Notes               string
}

// CreateResident creates a new resident in the vault.
func (s *Service) CreateResident(ctx context.Context, input CreateResidentInput) (*models.Resident, error) {
	// Generate IDs
	id := s.idGenerator.NewID()
	regNum, err := s.residents.GetNextRegistryNumber(ctx, s.vaultNumber)
	if err != nil {
		return nil, fmt.Errorf("generating registry number: %w", err)
	}

	// Set defaults
	clearance := input.ClearanceLevel
	if clearance < 1 {
		clearance = 1
	}

	resident := &models.Resident{
		ID:                  id,
		RegistryNumber:      regNum,
		Surname:             input.Surname,
		GivenNames:          input.GivenNames,
		DateOfBirth:         input.DateOfBirth,
		Sex:                 input.Sex,
		BloodType:           input.BloodType,
		EntryType:           input.EntryType,
		EntryDate:           input.EntryDate,
		Status:              models.ResidentStatusActive,
		BiologicalParent1ID: input.BiologicalParent1ID,
		BiologicalParent2ID: input.BiologicalParent2ID,
		HouseholdID:         input.HouseholdID,
		ClearanceLevel:      clearance,
		Notes:               input.Notes,
	}

	if err := s.residents.Create(ctx, nil, resident); err != nil {
		return nil, fmt.Errorf("creating resident: %w", err)
	}

	return resident, nil
}

// GetResident retrieves a resident by ID.
func (s *Service) GetResident(ctx context.Context, id string) (*models.Resident, error) {
	return s.residents.GetByID(ctx, id)
}

// GetResidentByRegistryNumber retrieves a resident by registry number.
func (s *Service) GetResidentByRegistryNumber(ctx context.Context, regNum string) (*models.Resident, error) {
	return s.residents.GetByRegistryNumber(ctx, regNum)
}

// UpdateResidentInput contains data for updating a resident.
type UpdateResidentInput struct {
	Surname        *string
	GivenNames     *string
	BloodType      *models.BloodType
	Status         *models.ResidentStatus
	DateOfDeath    *time.Time
	HouseholdID    *string
	QuartersID     *string
	VocationID     *string
	ClearanceLevel *int
	Notes          *string
}

// UpdateResident updates an existing resident.
func (s *Service) UpdateResident(ctx context.Context, id string, input UpdateResidentInput) (*models.Resident, error) {
	resident, err := s.residents.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if input.Surname != nil {
		resident.Surname = *input.Surname
	}
	if input.GivenNames != nil {
		resident.GivenNames = *input.GivenNames
	}
	if input.BloodType != nil {
		resident.BloodType = *input.BloodType
	}
	if input.Status != nil {
		resident.Status = *input.Status
	}
	if input.DateOfDeath != nil {
		resident.DateOfDeath = input.DateOfDeath
	}
	if input.HouseholdID != nil {
		resident.HouseholdID = input.HouseholdID
	}
	if input.QuartersID != nil {
		resident.QuartersID = input.QuartersID
	}
	if input.VocationID != nil {
		resident.PrimaryVocationID = input.VocationID
	}
	if input.ClearanceLevel != nil {
		resident.ClearanceLevel = *input.ClearanceLevel
	}
	if input.Notes != nil {
		resident.Notes = *input.Notes
	}

	if err := s.residents.Update(ctx, nil, resident); err != nil {
		return nil, fmt.Errorf("updating resident: %w", err)
	}

	return resident, nil
}

// ListResidents retrieves residents with filtering and pagination.
func (s *Service) ListResidents(ctx context.Context, filter models.ResidentFilter, page models.Pagination) (*models.ResidentList, error) {
	return s.residents.List(ctx, filter, page)
}

// BirthRegistration contains data for registering a birth.
type BirthRegistration struct {
	Surname     string
	GivenNames  string
	DateOfBirth time.Time
	Sex         models.Sex
	BloodType   models.BloodType
	Parent1ID   string
	Parent2ID   string
	HouseholdID string
	Notes       string
}

// RegisterBirth registers a new vault-born resident.
func (s *Service) RegisterBirth(ctx context.Context, input BirthRegistration) (*models.Resident, error) {
	// Validate parents exist and are alive
	parent1, err := s.residents.GetByID(ctx, input.Parent1ID)
	if err != nil {
		return nil, fmt.Errorf("parent 1 not found: %w", err)
	}
	if !parent1.IsAlive() {
		return nil, fmt.Errorf("parent 1 is deceased")
	}

	parent2, err := s.residents.GetByID(ctx, input.Parent2ID)
	if err != nil {
		return nil, fmt.Errorf("parent 2 not found: %w", err)
	}
	if !parent2.IsAlive() {
		return nil, fmt.Errorf("parent 2 is deceased")
	}

	// Calculate and warn on COI
	coi, err := s.CalculateCOI(ctx, input.Parent1ID, input.Parent2ID)
	if err == nil && coi > 0.0625 {
		// Log warning but don't prevent birth
		// In a real system, this would be recorded
		_ = fmt.Sprintf("WARNING: High coefficient of inbreeding: %.4f", coi)
	}

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Generate IDs
	id := s.idGenerator.NewID()
	regNum, err := s.residents.GetNextRegistryNumber(ctx, s.vaultNumber)
	if err != nil {
		return nil, fmt.Errorf("generating registry number: %w", err)
	}

	resident := &models.Resident{
		ID:                  id,
		RegistryNumber:      regNum,
		Surname:             input.Surname,
		GivenNames:          input.GivenNames,
		DateOfBirth:         input.DateOfBirth,
		Sex:                 input.Sex,
		BloodType:           input.BloodType,
		EntryType:           models.EntryTypeVaultBorn,
		EntryDate:           input.DateOfBirth,
		Status:              models.ResidentStatusActive,
		BiologicalParent1ID: &input.Parent1ID,
		BiologicalParent2ID: &input.Parent2ID,
		HouseholdID:         &input.HouseholdID,
		ClearanceLevel:      1,
		Notes:               input.Notes,
	}

	if err := s.residents.Create(ctx, tx, resident); err != nil {
		return nil, fmt.Errorf("creating resident: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return resident, nil
}

// DeathRegistration contains data for registering a death.
type DeathRegistration struct {
	DateOfDeath time.Time
	Cause       string // Stored in notes
}

// RegisterDeath records the death of a resident.
func (s *Service) RegisterDeath(ctx context.Context, residentID string, input DeathRegistration) error {
	resident, err := s.residents.GetByID(ctx, residentID)
	if err != nil {
		return err
	}

	if !resident.IsAlive() {
		return fmt.Errorf("resident is already deceased")
	}

	resident.Status = models.ResidentStatusDeceased
	resident.DateOfDeath = &input.DateOfDeath
	if input.Cause != "" {
		if resident.Notes != "" {
			resident.Notes += "\n"
		}
		resident.Notes += fmt.Sprintf("Cause of death: %s", input.Cause)
	}

	return s.residents.Update(ctx, nil, resident)
}

// CreateHouseholdInput contains data for creating a household.
type CreateHouseholdInput struct {
	HouseholdType     models.HouseholdType
	HeadOfHouseholdID *string
	QuartersID        *string
	RationClass       models.RationClass
	FormedDate        time.Time
}

// CreateHousehold creates a new household.
func (s *Service) CreateHousehold(ctx context.Context, input CreateHouseholdInput) (*models.Household, error) {
	id := s.idGenerator.NewID()
	designation, err := s.households.GetNextDesignation(ctx)
	if err != nil {
		return nil, fmt.Errorf("generating designation: %w", err)
	}

	household := &models.Household{
		ID:                id,
		Designation:       designation,
		HouseholdType:     input.HouseholdType,
		HeadOfHouseholdID: input.HeadOfHouseholdID,
		QuartersID:        input.QuartersID,
		RationClass:       input.RationClass,
		Status:            models.HouseholdStatusActive,
		FormedDate:        input.FormedDate,
	}

	if err := s.households.Create(ctx, nil, household); err != nil {
		return nil, fmt.Errorf("creating household: %w", err)
	}

	return household, nil
}

// GetHousehold retrieves a household by ID.
func (s *Service) GetHousehold(ctx context.Context, id string) (*models.Household, error) {
	return s.households.GetByID(ctx, id)
}

// ListHouseholds retrieves households with filtering and pagination.
func (s *Service) ListHouseholds(ctx context.Context, filter models.HouseholdFilter, page models.Pagination) (*models.HouseholdList, error) {
	return s.households.List(ctx, filter, page)
}

// GetHouseholdMembers retrieves all members of a household.
func (s *Service) GetHouseholdMembers(ctx context.Context, householdID string) ([]*models.Resident, error) {
	return s.residents.GetByHousehold(ctx, householdID)
}

// AssignToHousehold assigns a resident to a household.
func (s *Service) AssignToHousehold(ctx context.Context, residentID, householdID string) error {
	resident, err := s.residents.GetByID(ctx, residentID)
	if err != nil {
		return err
	}

	// Verify household exists
	_, err = s.households.GetByID(ctx, householdID)
	if err != nil {
		return fmt.Errorf("household not found: %w", err)
	}

	resident.HouseholdID = &householdID
	return s.residents.Update(ctx, nil, resident)
}

// GetChildren retrieves biological children of a resident.
func (s *Service) GetChildren(ctx context.Context, residentID string) ([]*models.Resident, error) {
	return s.residents.GetChildren(ctx, residentID)
}

// GetParents retrieves biological parents of a resident.
func (s *Service) GetParents(ctx context.Context, residentID string) ([]*models.Resident, error) {
	return s.residents.GetParents(ctx, residentID)
}

// GetPopulationStats returns current population statistics.
func (s *Service) GetPopulationStats(ctx context.Context) (*PopulationStats, error) {
	statusCounts, err := s.residents.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	stats := &PopulationStats{
		TotalActive:   statusCounts[models.ResidentStatusActive],
		TotalDeceased: statusCounts[models.ResidentStatusDeceased],
		TotalExiled:   statusCounts[models.ResidentStatusExiled],
		OnMission:     statusCounts[models.ResidentStatusSurfaceMission],
		Quarantined:   statusCounts[models.ResidentStatusQuarantine],
	}

	stats.Total = stats.TotalActive + stats.TotalDeceased + stats.TotalExiled + stats.OnMission + stats.Quarantined

	return stats, nil
}

// PopulationStats contains population statistics.
type PopulationStats struct {
	Total         int
	TotalActive   int
	TotalDeceased int
	TotalExiled   int
	OnMission     int
	Quarantined   int
}
