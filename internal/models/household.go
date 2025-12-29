package models

import (
	"fmt"
	"time"
)

// HouseholdType represents the type of household.
type HouseholdType string

const (
	HouseholdTypeFamily     HouseholdType = "FAMILY"
	HouseholdTypeIndividual HouseholdType = "INDIVIDUAL"
	HouseholdTypeCommunal   HouseholdType = "COMMUNAL"
	HouseholdTypeTemporary  HouseholdType = "TEMPORARY"
)

// Valid returns true if the household type is valid.
func (h HouseholdType) Valid() bool {
	switch h {
	case HouseholdTypeFamily, HouseholdTypeIndividual, HouseholdTypeCommunal, HouseholdTypeTemporary:
		return true
	default:
		return false
	}
}

// RationClass represents the ration allocation class for a household.
type RationClass string

const (
	RationClassMinimal        RationClass = "MINIMAL"
	RationClassStandard       RationClass = "STANDARD"
	RationClassEnhanced       RationClass = "ENHANCED"
	RationClassMedical        RationClass = "MEDICAL"
	RationClassLaborIntensive RationClass = "LABOR_INTENSIVE"
)

// Valid returns true if the ration class is valid.
func (r RationClass) Valid() bool {
	switch r {
	case RationClassMinimal, RationClassStandard, RationClassEnhanced,
		RationClassMedical, RationClassLaborIntensive:
		return true
	default:
		return false
	}
}

// CalorieTarget returns the daily calorie target for this ration class.
func (r RationClass) CalorieTarget() int {
	switch r {
	case RationClassMinimal:
		return 1500
	case RationClassStandard:
		return 2000
	case RationClassEnhanced:
		return 2500
	case RationClassLaborIntensive:
		return 3000
	case RationClassMedical:
		return 2000 // Variable, but use standard as baseline
	default:
		return 2000
	}
}

// WaterTarget returns the daily water allocation in liters for this ration class.
func (r RationClass) WaterTarget() float64 {
	switch r {
	case RationClassMinimal:
		return 2.0
	case RationClassStandard:
		return 3.0
	case RationClassEnhanced:
		return 3.5
	case RationClassLaborIntensive:
		return 4.0
	case RationClassMedical:
		return 3.0 // Variable, but use standard as baseline
	default:
		return 3.0
	}
}

// HouseholdStatus represents the status of a household.
type HouseholdStatus string

const (
	HouseholdStatusActive    HouseholdStatus = "ACTIVE"
	HouseholdStatusDissolved HouseholdStatus = "DISSOLVED"
	HouseholdStatusMerged    HouseholdStatus = "MERGED"
)

// Valid returns true if the status is valid.
func (s HouseholdStatus) Valid() bool {
	return s == HouseholdStatusActive || s == HouseholdStatusDissolved || s == HouseholdStatusMerged
}

// Household represents a group of residents sharing living quarters.
type Household struct {
	ID                string          `json:"id"`
	Designation       string          `json:"designation"`
	HouseholdType     HouseholdType   `json:"household_type"`
	HeadOfHouseholdID *string         `json:"head_of_household_id,omitempty"`
	QuartersID        *string         `json:"quarters_id,omitempty"`
	RationClass       RationClass     `json:"ration_class"`
	Status            HouseholdStatus `json:"status"`
	FormedDate        time.Time       `json:"formed_date"`
	DissolvedDate     *time.Time      `json:"dissolved_date,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`

	// Computed fields (not stored in DB)
	MemberCount int         `json:"member_count,omitempty"`
	Members     []*Resident `json:"members,omitempty"`
}

// Validate checks if the household data is valid.
func (h *Household) Validate() error {
	if h.ID == "" {
		return fmt.Errorf("id is required")
	}
	if h.Designation == "" {
		return fmt.Errorf("designation is required")
	}
	if !h.HouseholdType.Valid() {
		return fmt.Errorf("invalid household_type: %s", h.HouseholdType)
	}
	if !h.RationClass.Valid() {
		return fmt.Errorf("invalid ration_class: %s", h.RationClass)
	}
	if !h.Status.Valid() {
		return fmt.Errorf("invalid status: %s", h.Status)
	}
	if h.FormedDate.IsZero() {
		return fmt.Errorf("formed_date is required")
	}

	// Dissolved households must have dissolved date
	if h.Status == HouseholdStatusDissolved && h.DissolvedDate == nil {
		return fmt.Errorf("dissolved households must have dissolved_date")
	}

	return nil
}

// IsActive returns true if the household is active.
func (h *Household) IsActive() bool {
	return h.Status == HouseholdStatusActive
}

// HouseholdFilter defines filtering options for household queries.
type HouseholdFilter struct {
	Status        *HouseholdStatus
	HouseholdType *HouseholdType
	RationClass   *RationClass
	HasQuarters   *bool
	SearchTerm    string // Searches designation
}

// HouseholdList represents a paginated list of households.
type HouseholdList struct {
	Households []*Household
	Total      int
	Page       int
	PageSize   int
	TotalPages int
}

// Quarters represents a physical living space within the vault.
type Quarters struct {
	ID                  string         `json:"id"`
	UnitCode            string         `json:"unit_code"`
	Sector              string         `json:"sector"`
	Level               int            `json:"level"`
	UnitType            QuartersType   `json:"unit_type"`
	Capacity            int            `json:"capacity"`
	SquareMeters        float64        `json:"square_meters"`
	Amenities           []string       `json:"amenities,omitempty"`
	Status              QuartersStatus `json:"status"`
	AssignedHouseholdID *string        `json:"assigned_household_id,omitempty"`
	Notes               string         `json:"notes,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// QuartersType represents the type of living quarters.
type QuartersType string

const (
	QuartersTypeSingle    QuartersType = "SINGLE"
	QuartersTypeDouble    QuartersType = "DOUBLE"
	QuartersTypeFamily    QuartersType = "FAMILY"
	QuartersTypeDormitory QuartersType = "DORMITORY"
	QuartersTypeExecutive QuartersType = "EXECUTIVE"
)

// Valid returns true if the quarters type is valid.
func (q QuartersType) Valid() bool {
	switch q {
	case QuartersTypeSingle, QuartersTypeDouble, QuartersTypeFamily,
		QuartersTypeDormitory, QuartersTypeExecutive:
		return true
	default:
		return false
	}
}

// DefaultCapacity returns the default capacity for this quarters type.
func (q QuartersType) DefaultCapacity() int {
	switch q {
	case QuartersTypeSingle:
		return 1
	case QuartersTypeDouble:
		return 2
	case QuartersTypeFamily:
		return 5
	case QuartersTypeDormitory:
		return 12
	case QuartersTypeExecutive:
		return 3
	default:
		return 1
	}
}

// QuartersStatus represents the status of quarters.
type QuartersStatus string

const (
	QuartersStatusAvailable   QuartersStatus = "AVAILABLE"
	QuartersStatusOccupied    QuartersStatus = "OCCUPIED"
	QuartersStatusMaintenance QuartersStatus = "MAINTENANCE"
	QuartersStatusCondemned   QuartersStatus = "CONDEMNED"
)

// Valid returns true if the status is valid.
func (s QuartersStatus) Valid() bool {
	switch s {
	case QuartersStatusAvailable, QuartersStatusOccupied,
		QuartersStatusMaintenance, QuartersStatusCondemned:
		return true
	default:
		return false
	}
}

// Validate checks if the quarters data is valid.
func (q *Quarters) Validate() error {
	if q.ID == "" {
		return fmt.Errorf("id is required")
	}
	if q.UnitCode == "" {
		return fmt.Errorf("unit_code is required")
	}
	if q.Sector == "" {
		return fmt.Errorf("sector is required")
	}
	if !q.UnitType.Valid() {
		return fmt.Errorf("invalid unit_type: %s", q.UnitType)
	}
	if q.Capacity < 1 {
		return fmt.Errorf("capacity must be at least 1")
	}
	if !q.Status.Valid() {
		return fmt.Errorf("invalid status: %s", q.Status)
	}
	return nil
}

// IsAvailable returns true if the quarters can be assigned.
func (q *Quarters) IsAvailable() bool {
	return q.Status == QuartersStatusAvailable
}
