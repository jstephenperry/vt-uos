package models

import (
	"strings"
	"testing"
	"time"
)

func TestHouseholdType_Valid(t *testing.T) {
	tests := []struct {
		name          string
		householdType HouseholdType
		want          bool
	}{
		{"Family is valid", HouseholdTypeFamily, true},
		{"Individual is valid", HouseholdTypeIndividual, true},
		{"Communal is valid", HouseholdTypeCommunal, true},
		{"Temporary is valid", HouseholdTypeTemporary, true},
		{"Empty string is invalid", HouseholdType(""), false},
		{"Invalid type", HouseholdType("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.householdType.Valid(); got != tt.want {
				t.Errorf("HouseholdType.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRationClass_Valid(t *testing.T) {
	tests := []struct {
		name        string
		rationClass RationClass
		want        bool
	}{
		{"Minimal is valid", RationClassMinimal, true},
		{"Standard is valid", RationClassStandard, true},
		{"Enhanced is valid", RationClassEnhanced, true},
		{"Medical is valid", RationClassMedical, true},
		{"Labor intensive is valid", RationClassLaborIntensive, true},
		{"Empty string is invalid", RationClass(""), false},
		{"Invalid class", RationClass("LUXURY"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rationClass.Valid(); got != tt.want {
				t.Errorf("RationClass.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRationClass_CalorieTarget(t *testing.T) {
	tests := []struct {
		name        string
		rationClass RationClass
		want        int
	}{
		{"Minimal", RationClassMinimal, 1500},
		{"Standard", RationClassStandard, 2000},
		{"Enhanced", RationClassEnhanced, 2500},
		{"Labor intensive", RationClassLaborIntensive, 3000},
		{"Medical", RationClassMedical, 2000},
		{"Unknown defaults to standard", RationClass("UNKNOWN"), 2000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rationClass.CalorieTarget(); got != tt.want {
				t.Errorf("RationClass.CalorieTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRationClass_WaterTarget(t *testing.T) {
	tests := []struct {
		name        string
		rationClass RationClass
		want        float64
	}{
		{"Minimal", RationClassMinimal, 2.0},
		{"Standard", RationClassStandard, 3.0},
		{"Enhanced", RationClassEnhanced, 3.5},
		{"Labor intensive", RationClassLaborIntensive, 4.0},
		{"Medical", RationClassMedical, 3.0},
		{"Unknown defaults to standard", RationClass("UNKNOWN"), 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rationClass.WaterTarget(); got != tt.want {
				t.Errorf("RationClass.WaterTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHouseholdStatus_Valid(t *testing.T) {
	tests := []struct {
		name   string
		status HouseholdStatus
		want   bool
	}{
		{"Active is valid", HouseholdStatusActive, true},
		{"Dissolved is valid", HouseholdStatusDissolved, true},
		{"Merged is valid", HouseholdStatusMerged, true},
		{"Empty string is invalid", HouseholdStatus(""), false},
		{"Invalid status", HouseholdStatus("SUSPENDED"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.want {
				t.Errorf("HouseholdStatus.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHousehold_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status HouseholdStatus
		want   bool
	}{
		{"Active household", HouseholdStatusActive, true},
		{"Dissolved household", HouseholdStatusDissolved, false},
		{"Merged household", HouseholdStatusMerged, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			household := &Household{Status: tt.status}
			if got := household.IsActive(); got != tt.want {
				t.Errorf("Household.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHousehold_Validate(t *testing.T) {
	now := time.Now().UTC()
	dissolvedDate := now.AddDate(0, -1, 0)

	tests := []struct {
		name      string
		household *Household
		wantErr   bool
		errMsg    string
	}{
		{
			name: "Valid household",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusActive,
				FormedDate:    now.AddDate(-1, 0, 0),
			},
			wantErr: false,
		},
		{
			name: "Missing ID",
			household: &Household{
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusActive,
				FormedDate:    now,
			},
			wantErr: true,
			errMsg:  "id is required",
		},
		{
			name: "Missing designation",
			household: &Household{
				ID:            "hh-001",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusActive,
				FormedDate:    now,
			},
			wantErr: true,
			errMsg:  "designation is required",
		},
		{
			name: "Invalid household type",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdType("UNKNOWN"),
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusActive,
				FormedDate:    now,
			},
			wantErr: true,
			errMsg:  "invalid household_type",
		},
		{
			name: "Invalid ration class",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClass("LUXURY"),
				Status:        HouseholdStatusActive,
				FormedDate:    now,
			},
			wantErr: true,
			errMsg:  "invalid ration_class",
		},
		{
			name: "Invalid status",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatus("UNKNOWN"),
				FormedDate:    now,
			},
			wantErr: true,
			errMsg:  "invalid status",
		},
		{
			name: "Missing formed date",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusActive,
			},
			wantErr: true,
			errMsg:  "formed_date is required",
		},
		{
			name: "Dissolved without dissolved date",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusDissolved,
				FormedDate:    now.AddDate(-1, 0, 0),
			},
			wantErr: true,
			errMsg:  "dissolved households must have dissolved_date",
		},
		{
			name: "Dissolved with dissolved date is valid",
			household: &Household{
				ID:            "hh-001",
				Designation:   "Smith Family",
				HouseholdType: HouseholdTypeFamily,
				RationClass:   RationClassStandard,
				Status:        HouseholdStatusDissolved,
				FormedDate:    now.AddDate(-1, 0, 0),
				DissolvedDate: &dissolvedDate,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.household.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Household.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Household.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestQuartersType_Valid(t *testing.T) {
	tests := []struct {
		name         string
		quartersType QuartersType
		want         bool
	}{
		{"Single is valid", QuartersTypeSingle, true},
		{"Double is valid", QuartersTypeDouble, true},
		{"Family is valid", QuartersTypeFamily, true},
		{"Dormitory is valid", QuartersTypeDormitory, true},
		{"Executive is valid", QuartersTypeExecutive, true},
		{"Empty string is invalid", QuartersType(""), false},
		{"Invalid type", QuartersType("LUXURY"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quartersType.Valid(); got != tt.want {
				t.Errorf("QuartersType.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuartersType_DefaultCapacity(t *testing.T) {
	tests := []struct {
		name         string
		quartersType QuartersType
		want         int
	}{
		{"Single", QuartersTypeSingle, 1},
		{"Double", QuartersTypeDouble, 2},
		{"Family", QuartersTypeFamily, 5},
		{"Dormitory", QuartersTypeDormitory, 12},
		{"Executive", QuartersTypeExecutive, 3},
		{"Unknown defaults to 1", QuartersType("UNKNOWN"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quartersType.DefaultCapacity(); got != tt.want {
				t.Errorf("QuartersType.DefaultCapacity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuartersStatus_Valid(t *testing.T) {
	tests := []struct {
		name   string
		status QuartersStatus
		want   bool
	}{
		{"Available is valid", QuartersStatusAvailable, true},
		{"Occupied is valid", QuartersStatusOccupied, true},
		{"Maintenance is valid", QuartersStatusMaintenance, true},
		{"Condemned is valid", QuartersStatusCondemned, true},
		{"Empty string is invalid", QuartersStatus(""), false},
		{"Invalid status", QuartersStatus("RENOVATING"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.want {
				t.Errorf("QuartersStatus.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuarters_IsAvailable(t *testing.T) {
	tests := []struct {
		name   string
		status QuartersStatus
		want   bool
	}{
		{"Available quarters", QuartersStatusAvailable, true},
		{"Occupied quarters", QuartersStatusOccupied, false},
		{"Maintenance quarters", QuartersStatusMaintenance, false},
		{"Condemned quarters", QuartersStatusCondemned, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quarters := &Quarters{Status: tt.status}
			if got := quarters.IsAvailable(); got != tt.want {
				t.Errorf("Quarters.IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuarters_Validate(t *testing.T) {
	tests := []struct {
		name     string
		quarters *Quarters
		wantErr  bool
		errMsg   string
	}{
		{
			name: "Valid quarters",
			quarters: &Quarters{
				ID:       "q-001",
				UnitCode: "A-101",
				Sector:   "A",
				UnitType: QuartersTypeFamily,
				Capacity: 5,
				Status:   QuartersStatusAvailable,
			},
			wantErr: false,
		},
		{
			name: "Missing ID",
			quarters: &Quarters{
				UnitCode: "A-101",
				Sector:   "A",
				UnitType: QuartersTypeFamily,
				Capacity: 5,
				Status:   QuartersStatusAvailable,
			},
			wantErr: true,
			errMsg:  "id is required",
		},
		{
			name: "Missing unit code",
			quarters: &Quarters{
				ID:       "q-001",
				Sector:   "A",
				UnitType: QuartersTypeFamily,
				Capacity: 5,
				Status:   QuartersStatusAvailable,
			},
			wantErr: true,
			errMsg:  "unit_code is required",
		},
		{
			name: "Missing sector",
			quarters: &Quarters{
				ID:       "q-001",
				UnitCode: "A-101",
				UnitType: QuartersTypeFamily,
				Capacity: 5,
				Status:   QuartersStatusAvailable,
			},
			wantErr: true,
			errMsg:  "sector is required",
		},
		{
			name: "Invalid unit type",
			quarters: &Quarters{
				ID:       "q-001",
				UnitCode: "A-101",
				Sector:   "A",
				UnitType: QuartersType("LUXURY"),
				Capacity: 5,
				Status:   QuartersStatusAvailable,
			},
			wantErr: true,
			errMsg:  "invalid unit_type",
		},
		{
			name: "Invalid capacity",
			quarters: &Quarters{
				ID:       "q-001",
				UnitCode: "A-101",
				Sector:   "A",
				UnitType: QuartersTypeFamily,
				Capacity: 0,
				Status:   QuartersStatusAvailable,
			},
			wantErr: true,
			errMsg:  "capacity must be at least 1",
		},
		{
			name: "Invalid status",
			quarters: &Quarters{
				ID:       "q-001",
				UnitCode: "A-101",
				Sector:   "A",
				UnitType: QuartersTypeFamily,
				Capacity: 5,
				Status:   QuartersStatus("UNKNOWN"),
			},
			wantErr: true,
			errMsg:  "invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.quarters.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Quarters.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Quarters.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}
