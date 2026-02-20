package models

import (
	"fmt"
	"time"
)

// SystemCategory represents the category of a facility system.
type SystemCategory string

const (
	SystemCategoryPower          SystemCategory = "POWER"
	SystemCategoryWater          SystemCategory = "WATER"
	SystemCategoryHVAC           SystemCategory = "HVAC"
	SystemCategorySecurity       SystemCategory = "SECURITY"
	SystemCategoryMedical        SystemCategory = "MEDICAL"
	SystemCategoryFoodProduction SystemCategory = "FOOD_PRODUCTION"
	SystemCategoryWaste          SystemCategory = "WASTE"
	SystemCategoryCommunications SystemCategory = "COMMUNICATIONS"
	SystemCategoryStructural     SystemCategory = "STRUCTURAL"
)

// Valid returns true if the system category is valid.
func (c SystemCategory) Valid() bool {
	switch c {
	case SystemCategoryPower, SystemCategoryWater, SystemCategoryHVAC,
		SystemCategorySecurity, SystemCategoryMedical, SystemCategoryFoodProduction,
		SystemCategoryWaste, SystemCategoryCommunications, SystemCategoryStructural:
		return true
	default:
		return false
	}
}

// SystemStatus represents the operational status of a facility system.
type SystemStatus string

const (
	SystemStatusOperational SystemStatus = "OPERATIONAL"
	SystemStatusDegraded    SystemStatus = "DEGRADED"
	SystemStatusMaintenance SystemStatus = "MAINTENANCE"
	SystemStatusOffline     SystemStatus = "OFFLINE"
	SystemStatusFailed      SystemStatus = "FAILED"
	SystemStatusDestroyed   SystemStatus = "DESTROYED"
)

// Valid returns true if the system status is valid.
func (s SystemStatus) Valid() bool {
	switch s {
	case SystemStatusOperational, SystemStatusDegraded, SystemStatusMaintenance,
		SystemStatusOffline, SystemStatusFailed, SystemStatusDestroyed:
		return true
	default:
		return false
	}
}

// IsOperational returns true if the system is running (possibly degraded).
func (s SystemStatus) IsOperational() bool {
	return s == SystemStatusOperational || s == SystemStatusDegraded
}

// FacilitySystem represents an infrastructure system within the vault.
type FacilitySystem struct {
	ID                    string         `json:"id"`
	SystemCode            string         `json:"system_code"`
	Name                  string         `json:"name"`
	Category              SystemCategory `json:"category"`
	LocationSector        string         `json:"location_sector"`
	LocationLevel         int            `json:"location_level"`
	Status                SystemStatus   `json:"status"`
	EfficiencyPercent     float64        `json:"efficiency_percent"`
	CapacityRating        *float64       `json:"capacity_rating,omitempty"`
	CapacityUnit          string         `json:"capacity_unit,omitempty"`
	CurrentOutput         *float64       `json:"current_output,omitempty"`
	InstallDate           time.Time      `json:"install_date"`
	LastMaintenanceDate   *time.Time     `json:"last_maintenance_date,omitempty"`
	NextMaintenanceDue    *time.Time     `json:"next_maintenance_due,omitempty"`
	MaintenanceIntervalDays int          `json:"maintenance_interval_days"`
	MTBFHours             *int           `json:"mtbf_hours,omitempty"`
	TotalRuntimeHours     float64        `json:"total_runtime_hours"`
	Notes                 string         `json:"notes,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

// Validate checks if the facility system data is valid.
func (f *FacilitySystem) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("id is required")
	}
	if f.SystemCode == "" {
		return fmt.Errorf("system_code is required")
	}
	if f.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !f.Category.Valid() {
		return fmt.Errorf("invalid category: %s", f.Category)
	}
	if f.LocationSector == "" {
		return fmt.Errorf("location_sector is required")
	}
	if !f.Status.Valid() {
		return fmt.Errorf("invalid status: %s", f.Status)
	}
	if f.EfficiencyPercent < 0 || f.EfficiencyPercent > 100 {
		return fmt.Errorf("efficiency_percent must be between 0 and 100")
	}
	if f.InstallDate.IsZero() {
		return fmt.Errorf("install_date is required")
	}
	if f.MaintenanceIntervalDays < 1 {
		return fmt.Errorf("maintenance_interval_days must be at least 1")
	}
	return nil
}

// IsOverdueForMaintenance returns true if maintenance is past due.
func (f *FacilitySystem) IsOverdueForMaintenance(asOf time.Time) bool {
	if f.NextMaintenanceDue == nil {
		return false
	}
	return asOf.After(*f.NextMaintenanceDue)
}

// MaintenanceType represents the type of maintenance performed.
type MaintenanceType string

const (
	MaintenanceTypePreventive  MaintenanceType = "PREVENTIVE"
	MaintenanceTypeCorrective  MaintenanceType = "CORRECTIVE"
	MaintenanceTypeEmergency   MaintenanceType = "EMERGENCY"
	MaintenanceTypeInspection  MaintenanceType = "INSPECTION"
	MaintenanceTypeUpgrade     MaintenanceType = "UPGRADE"
)

// Valid returns true if the maintenance type is valid.
func (m MaintenanceType) Valid() bool {
	switch m {
	case MaintenanceTypePreventive, MaintenanceTypeCorrective, MaintenanceTypeEmergency,
		MaintenanceTypeInspection, MaintenanceTypeUpgrade:
		return true
	default:
		return false
	}
}

// MaintenanceOutcome represents the outcome of a maintenance operation.
type MaintenanceOutcome string

const (
	MaintenanceOutcomeCompleted MaintenanceOutcome = "COMPLETED"
	MaintenanceOutcomePartial   MaintenanceOutcome = "PARTIAL"
	MaintenanceOutcomeFailed    MaintenanceOutcome = "FAILED"
	MaintenanceOutcomeDeferred  MaintenanceOutcome = "DEFERRED"
	MaintenanceOutcomeCancelled MaintenanceOutcome = "CANCELLED"
)

// Valid returns true if the maintenance outcome is valid.
func (o MaintenanceOutcome) Valid() bool {
	switch o {
	case MaintenanceOutcomeCompleted, MaintenanceOutcomePartial, MaintenanceOutcomeFailed,
		MaintenanceOutcomeDeferred, MaintenanceOutcomeCancelled:
		return true
	default:
		return false
	}
}

// MaintenanceRecord represents a maintenance event on a facility system.
type MaintenanceRecord struct {
	ID                 string              `json:"id"`
	SystemID           string              `json:"system_id"`
	MaintenanceType    MaintenanceType     `json:"maintenance_type"`
	Description        string              `json:"description"`
	WorkPerformed      string              `json:"work_performed,omitempty"`
	PartsConsumed      string              `json:"parts_consumed,omitempty"`
	LeadTechnicianID   *string             `json:"lead_technician_id,omitempty"`
	CrewMemberIDs      string              `json:"crew_member_ids,omitempty"`
	ScheduledDate      *time.Time          `json:"scheduled_date,omitempty"`
	StartedAt          *time.Time          `json:"started_at,omitempty"`
	CompletedAt        *time.Time          `json:"completed_at,omitempty"`
	EstimatedHours     *float64            `json:"estimated_hours,omitempty"`
	ActualHours        *float64            `json:"actual_hours,omitempty"`
	Outcome            *MaintenanceOutcome `json:"outcome,omitempty"`
	SystemStatusBefore string              `json:"system_status_before,omitempty"`
	SystemStatusAfter  string              `json:"system_status_after,omitempty"`
	EfficiencyBefore   *float64            `json:"efficiency_before,omitempty"`
	EfficiencyAfter    *float64            `json:"efficiency_after,omitempty"`
	Notes              string              `json:"notes,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// Validate checks if the maintenance record data is valid.
func (m *MaintenanceRecord) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("id is required")
	}
	if m.SystemID == "" {
		return fmt.Errorf("system_id is required")
	}
	if !m.MaintenanceType.Valid() {
		return fmt.Errorf("invalid maintenance_type: %s", m.MaintenanceType)
	}
	if m.Description == "" {
		return fmt.Errorf("description is required")
	}
	if m.Outcome != nil && !m.Outcome.Valid() {
		return fmt.Errorf("invalid outcome: %s", *m.Outcome)
	}
	return nil
}

// IsComplete returns true if the maintenance has been completed.
func (m *MaintenanceRecord) IsComplete() bool {
	return m.Outcome != nil && *m.Outcome == MaintenanceOutcomeCompleted
}

// FacilitySystemFilter defines filtering options for facility system queries.
type FacilitySystemFilter struct {
	Category   *SystemCategory
	Status     *SystemStatus
	Sector     string
	SearchTerm string
	OverdueOnly bool
}

// FacilitySystemList represents a paginated list of facility systems.
type FacilitySystemList struct {
	Systems    []*FacilitySystem
	Total      int
	Page       int
	PageSize   int
	TotalPages int
}

// MaintenanceRecordFilter defines filtering options for maintenance record queries.
type MaintenanceRecordFilter struct {
	SystemID        *string
	MaintenanceType *MaintenanceType
	Outcome         *MaintenanceOutcome
	SearchTerm      string
}

// MaintenanceRecordList represents a paginated list of maintenance records.
type MaintenanceRecordList struct {
	Records    []*MaintenanceRecord
	Total      int
	Page       int
	PageSize   int
	TotalPages int
}

// FacilityStats contains aggregate statistics about facility systems.
type FacilityStats struct {
	TotalSystems       int
	Operational        int
	Degraded           int
	InMaintenance      int
	Offline            int
	Failed             int
	AvgEfficiency      float64
	OverdueMaintenance int
}
