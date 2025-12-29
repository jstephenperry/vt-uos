// Package models defines the domain models for VT-UOS.
package models

import (
	"fmt"
	"time"
)

// Sex represents biological sex.
type Sex string

const (
	SexMale   Sex = "M"
	SexFemale Sex = "F"
)

// Valid returns true if the sex is a valid value.
func (s Sex) Valid() bool {
	return s == SexMale || s == SexFemale
}

// String returns the display string for the sex.
func (s Sex) String() string {
	switch s {
	case SexMale:
		return "Male"
	case SexFemale:
		return "Female"
	default:
		return "Unknown"
	}
}

// BloodType represents a blood type.
type BloodType string

const (
	BloodTypeAPos  BloodType = "A+"
	BloodTypeANeg  BloodType = "A-"
	BloodTypeBPos  BloodType = "B+"
	BloodTypeBNeg  BloodType = "B-"
	BloodTypeABPos BloodType = "AB+"
	BloodTypeABNeg BloodType = "AB-"
	BloodTypeOPos  BloodType = "O+"
	BloodTypeONeg  BloodType = "O-"
)

// Valid returns true if the blood type is valid.
func (b BloodType) Valid() bool {
	switch b {
	case BloodTypeAPos, BloodTypeANeg, BloodTypeBPos, BloodTypeBNeg,
		BloodTypeABPos, BloodTypeABNeg, BloodTypeOPos, BloodTypeONeg:
		return true
	default:
		return false
	}
}

// EntryType represents how a resident entered the vault.
type EntryType string

const (
	EntryTypeOriginal  EntryType = "ORIGINAL"
	EntryTypeVaultBorn EntryType = "VAULT_BORN"
	EntryTypeAdmitted  EntryType = "ADMITTED"
)

// Valid returns true if the entry type is valid.
func (e EntryType) Valid() bool {
	return e == EntryTypeOriginal || e == EntryTypeVaultBorn || e == EntryTypeAdmitted
}

// ResidentStatus represents the current status of a resident.
type ResidentStatus string

const (
	ResidentStatusActive         ResidentStatus = "ACTIVE"
	ResidentStatusDeceased       ResidentStatus = "DECEASED"
	ResidentStatusExiled         ResidentStatus = "EXILED"
	ResidentStatusSurfaceMission ResidentStatus = "SURFACE_MISSION"
	ResidentStatusQuarantine     ResidentStatus = "QUARANTINE"
)

// Valid returns true if the status is valid.
func (s ResidentStatus) Valid() bool {
	switch s {
	case ResidentStatusActive, ResidentStatusDeceased, ResidentStatusExiled,
		ResidentStatusSurfaceMission, ResidentStatusQuarantine:
		return true
	default:
		return false
	}
}

// IsAlive returns true if the status represents a living resident.
func (s ResidentStatus) IsAlive() bool {
	return s != ResidentStatusDeceased
}

// Resident represents a vault dweller.
type Resident struct {
	// Identity
	ID             string `json:"id"`
	RegistryNumber string `json:"registry_number"`

	// Biographic Data
	Surname     string     `json:"surname"`
	GivenNames  string     `json:"given_names"`
	DateOfBirth time.Time  `json:"date_of_birth"`
	DateOfDeath *time.Time `json:"date_of_death,omitempty"`
	Sex         Sex        `json:"sex"`
	BloodType   BloodType  `json:"blood_type,omitempty"`

	// Origin & Status
	EntryType EntryType      `json:"entry_type"`
	EntryDate time.Time      `json:"entry_date"`
	Status    ResidentStatus `json:"status"`

	// Lineage
	BiologicalParent1ID *string `json:"biological_parent_1_id,omitempty"`
	BiologicalParent2ID *string `json:"biological_parent_2_id,omitempty"`

	// Current Assignments
	HouseholdID       *string `json:"household_id,omitempty"`
	QuartersID        *string `json:"quarters_id,omitempty"`
	PrimaryVocationID *string `json:"primary_vocation_id,omitempty"`
	ClearanceLevel    int     `json:"clearance_level"`

	// Metadata
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FullName returns the resident's full name.
func (r *Resident) FullName() string {
	return fmt.Sprintf("%s, %s", r.Surname, r.GivenNames)
}

// Age calculates the resident's age as of the given date.
func (r *Resident) Age(asOf time.Time) int {
	years := asOf.Year() - r.DateOfBirth.Year()
	if asOf.YearDay() < r.DateOfBirth.YearDay() {
		years--
	}
	return years
}

// IsAdult returns true if the resident is 18 or older as of the given date.
func (r *Resident) IsAdult(asOf time.Time) bool {
	return r.Age(asOf) >= 18
}

// IsWorkingAge returns true if the resident is between 16 and 65 as of the given date.
func (r *Resident) IsWorkingAge(asOf time.Time) bool {
	age := r.Age(asOf)
	return age >= 16 && age <= 65
}

// IsAlive returns true if the resident is not deceased.
func (r *Resident) IsAlive() bool {
	return r.Status.IsAlive()
}

// Validate checks if the resident data is valid.
func (r *Resident) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("id is required")
	}
	if r.RegistryNumber == "" {
		return fmt.Errorf("registry_number is required")
	}
	if r.Surname == "" {
		return fmt.Errorf("surname is required")
	}
	if r.GivenNames == "" {
		return fmt.Errorf("given_names is required")
	}
	if r.DateOfBirth.IsZero() {
		return fmt.Errorf("date_of_birth is required")
	}
	if !r.Sex.Valid() {
		return fmt.Errorf("invalid sex: %s", r.Sex)
	}
	if r.BloodType != "" && !r.BloodType.Valid() {
		return fmt.Errorf("invalid blood_type: %s", r.BloodType)
	}
	if !r.EntryType.Valid() {
		return fmt.Errorf("invalid entry_type: %s", r.EntryType)
	}
	if r.EntryDate.IsZero() {
		return fmt.Errorf("entry_date is required")
	}
	if !r.Status.Valid() {
		return fmt.Errorf("invalid status: %s", r.Status)
	}
	if r.ClearanceLevel < 1 || r.ClearanceLevel > 10 {
		return fmt.Errorf("clearance_level must be between 1 and 10")
	}

	// Vault-born residents must have parents
	if r.EntryType == EntryTypeVaultBorn {
		if r.BiologicalParent1ID == nil || r.BiologicalParent2ID == nil {
			return fmt.Errorf("vault-born residents must have both biological parents")
		}
	}

	// Deceased residents must have death date
	if r.Status == ResidentStatusDeceased && r.DateOfDeath == nil {
		return fmt.Errorf("deceased residents must have date_of_death")
	}

	return nil
}

// ResidentFilter defines filtering options for resident queries.
type ResidentFilter struct {
	Status      *ResidentStatus
	HouseholdID *string
	VocationID  *string
	Sex         *Sex
	MinAge      *int
	MaxAge      *int
	SearchTerm  string // Searches surname and given_names
	EntryType   *EntryType
}

// ResidentList represents a paginated list of residents.
type ResidentList struct {
	Residents  []*Resident
	Total      int
	Page       int
	PageSize   int
	TotalPages int
}
