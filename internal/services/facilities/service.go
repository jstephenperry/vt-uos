// Package facilities provides facility management services for VT-UOS.
package facilities

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/repository"
	"github.com/vtuos/vtuos/internal/util"
)

// Service provides facility management operations.
type Service struct {
	db          *sql.DB
	facilities  *repository.FacilityRepository
	idGenerator *util.IDGenerator
}

// NewService creates a new facilities service.
func NewService(db *sql.DB) *Service {
	return &Service{
		db:          db,
		facilities:  repository.NewFacilityRepository(db),
		idGenerator: util.NewIDGenerator(),
	}
}

// CreateSystemInput contains data for creating a facility system.
type CreateSystemInput struct {
	SystemCode              string
	Name                    string
	Category                models.SystemCategory
	LocationSector          string
	LocationLevel           int
	CapacityRating          *float64
	CapacityUnit            string
	InstallDate             time.Time
	MaintenanceIntervalDays int
	MTBFHours               *int
	Notes                   string
}

// CreateSystem creates a new facility system.
func (s *Service) CreateSystem(ctx context.Context, input CreateSystemInput) (*models.FacilitySystem, error) {
	id := s.idGenerator.NewID()

	intervalDays := input.MaintenanceIntervalDays
	if intervalDays < 1 {
		intervalDays = 90
	}

	// Calculate first maintenance due date
	nextMaint := input.InstallDate.AddDate(0, 0, intervalDays)

	system := &models.FacilitySystem{
		ID:                      id,
		SystemCode:              input.SystemCode,
		Name:                    input.Name,
		Category:                input.Category,
		LocationSector:          input.LocationSector,
		LocationLevel:           input.LocationLevel,
		Status:                  models.SystemStatusOperational,
		EfficiencyPercent:       100.0,
		CapacityRating:          input.CapacityRating,
		CapacityUnit:            input.CapacityUnit,
		InstallDate:             input.InstallDate,
		NextMaintenanceDue:      &nextMaint,
		MaintenanceIntervalDays: intervalDays,
		MTBFHours:               input.MTBFHours,
		TotalRuntimeHours:       0,
		Notes:                   input.Notes,
	}

	if err := s.facilities.Create(ctx, nil, system); err != nil {
		return nil, fmt.Errorf("creating facility system: %w", err)
	}

	return system, nil
}

// GetSystem retrieves a facility system by ID.
func (s *Service) GetSystem(ctx context.Context, id string) (*models.FacilitySystem, error) {
	return s.facilities.GetByID(ctx, id)
}

// GetSystemByCode retrieves a facility system by system code.
func (s *Service) GetSystemByCode(ctx context.Context, code string) (*models.FacilitySystem, error) {
	return s.facilities.GetByCode(ctx, code)
}

// ListSystems retrieves facility systems with filtering and pagination.
func (s *Service) ListSystems(ctx context.Context, filter models.FacilitySystemFilter, page models.Pagination) (*models.FacilitySystemList, error) {
	return s.facilities.List(ctx, filter, page)
}

// UpdateSystemInput contains data for updating a facility system.
type UpdateSystemInput struct {
	Name              *string
	Status            *models.SystemStatus
	EfficiencyPercent *float64
	CurrentOutput     *float64
	Notes             *string
}

// UpdateSystem updates an existing facility system.
func (s *Service) UpdateSystem(ctx context.Context, id string, input UpdateSystemInput) (*models.FacilitySystem, error) {
	system, err := s.facilities.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		system.Name = *input.Name
	}
	if input.Status != nil {
		system.Status = *input.Status
	}
	if input.EfficiencyPercent != nil {
		system.EfficiencyPercent = *input.EfficiencyPercent
	}
	if input.CurrentOutput != nil {
		system.CurrentOutput = input.CurrentOutput
	}
	if input.Notes != nil {
		system.Notes = *input.Notes
	}

	if err := s.facilities.Update(ctx, nil, system); err != nil {
		return nil, fmt.Errorf("updating facility system: %w", err)
	}

	return system, nil
}

// ScheduleMaintenanceInput contains data for scheduling maintenance.
type ScheduleMaintenanceInput struct {
	SystemID        string
	MaintenanceType models.MaintenanceType
	Description     string
	ScheduledDate   time.Time
	EstimatedHours  *float64
	LeadTechID      *string
	Notes           string
}

// ScheduleMaintenance schedules a maintenance operation on a system.
func (s *Service) ScheduleMaintenance(ctx context.Context, input ScheduleMaintenanceInput) (*models.MaintenanceRecord, error) {
	// Verify system exists
	system, err := s.facilities.GetByID(ctx, input.SystemID)
	if err != nil {
		return nil, fmt.Errorf("system not found: %w", err)
	}

	id := s.idGenerator.NewID()

	record := &models.MaintenanceRecord{
		ID:                 id,
		SystemID:           input.SystemID,
		MaintenanceType:    input.MaintenanceType,
		Description:        input.Description,
		ScheduledDate:      &input.ScheduledDate,
		EstimatedHours:     input.EstimatedHours,
		LeadTechnicianID:   input.LeadTechID,
		SystemStatusBefore: string(system.Status),
		EfficiencyBefore:   &system.EfficiencyPercent,
		Notes:              input.Notes,
	}

	if err := s.facilities.CreateMaintenanceRecord(ctx, nil, record); err != nil {
		return nil, fmt.Errorf("creating maintenance record: %w", err)
	}

	return record, nil
}

// CompleteMaintenanceInput contains data for completing a maintenance operation.
type CompleteMaintenanceInput struct {
	RecordID       string
	WorkPerformed  string
	PartsConsumed  string
	ActualHours    float64
	Outcome        models.MaintenanceOutcome
	NewEfficiency  float64
	NewStatus      models.SystemStatus
	Notes          string
}

// CompleteMaintenance completes a maintenance operation and updates the system.
func (s *Service) CompleteMaintenance(ctx context.Context, input CompleteMaintenanceInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the maintenance record to find the system
	records, err := s.facilities.ListMaintenanceRecords(ctx, models.MaintenanceRecordFilter{}, models.Pagination{Page: 1, PageSize: 1000})
	if err != nil {
		return fmt.Errorf("listing maintenance records: %w", err)
	}

	var record *models.MaintenanceRecord
	for _, r := range records.Records {
		if r.ID == input.RecordID {
			record = r
			break
		}
	}
	if record == nil {
		return fmt.Errorf("maintenance record not found: %s", input.RecordID)
	}

	// Get the system
	system, err := s.facilities.GetByID(ctx, record.SystemID)
	if err != nil {
		return fmt.Errorf("system not found: %w", err)
	}

	// Update the system
	now := time.Now().UTC()
	system.Status = input.NewStatus
	system.EfficiencyPercent = input.NewEfficiency
	system.LastMaintenanceDate = &now

	// Calculate next maintenance due
	nextDue := now.AddDate(0, 0, system.MaintenanceIntervalDays)
	system.NextMaintenanceDue = &nextDue

	if err := s.facilities.Update(ctx, tx, system); err != nil {
		return fmt.Errorf("updating system: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetFacilityStats returns aggregate statistics about all facility systems.
func (s *Service) GetFacilityStats(ctx context.Context, asOf time.Time) (*models.FacilityStats, error) {
	statusCounts, err := s.facilities.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	avgEfficiency, err := s.facilities.GetAverageEfficiency(ctx)
	if err != nil {
		return nil, err
	}

	overdueCount, err := s.facilities.GetOverdueCount(ctx, asOf)
	if err != nil {
		return nil, err
	}

	stats := &models.FacilityStats{
		Operational:        statusCounts[models.SystemStatusOperational],
		Degraded:           statusCounts[models.SystemStatusDegraded],
		InMaintenance:      statusCounts[models.SystemStatusMaintenance],
		Offline:            statusCounts[models.SystemStatusOffline],
		Failed:             statusCounts[models.SystemStatusFailed],
		AvgEfficiency:      avgEfficiency,
		OverdueMaintenance: overdueCount,
	}

	stats.TotalSystems = stats.Operational + stats.Degraded + stats.InMaintenance +
		stats.Offline + stats.Failed +
		statusCounts[models.SystemStatusDestroyed]

	return stats, nil
}

// ListMaintenanceRecords retrieves maintenance records with filtering and pagination.
func (s *Service) ListMaintenanceRecords(ctx context.Context, filter models.MaintenanceRecordFilter, page models.Pagination) (*models.MaintenanceRecordList, error) {
	return s.facilities.ListMaintenanceRecords(ctx, filter, page)
}
