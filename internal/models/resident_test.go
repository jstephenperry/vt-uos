package models

import (
	"strings"
	"testing"
	"time"
)

func TestSex_Valid(t *testing.T) {
	tests := []struct {
		name string
		sex  Sex
		want bool
	}{
		{"Male is valid", SexMale, true},
		{"Female is valid", SexFemale, true},
		{"Empty string is invalid", Sex(""), false},
		{"Unknown value is invalid", Sex("X"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sex.Valid(); got != tt.want {
				t.Errorf("Sex.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSex_String(t *testing.T) {
	tests := []struct {
		name string
		sex  Sex
		want string
	}{
		{"Male", SexMale, "Male"},
		{"Female", SexFemale, "Female"},
		{"Unknown", Sex("X"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sex.String(); got != tt.want {
				t.Errorf("Sex.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBloodType_Valid(t *testing.T) {
	tests := []struct {
		name      string
		bloodType BloodType
		want      bool
	}{
		{"A+ is valid", BloodTypeAPos, true},
		{"A- is valid", BloodTypeANeg, true},
		{"B+ is valid", BloodTypeBPos, true},
		{"B- is valid", BloodTypeBNeg, true},
		{"AB+ is valid", BloodTypeABPos, true},
		{"AB- is valid", BloodTypeABNeg, true},
		{"O+ is valid", BloodTypeOPos, true},
		{"O- is valid", BloodTypeONeg, true},
		{"Empty string is invalid", BloodType(""), false},
		{"Invalid blood type", BloodType("C+"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bloodType.Valid(); got != tt.want {
				t.Errorf("BloodType.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntryType_Valid(t *testing.T) {
	tests := []struct {
		name      string
		entryType EntryType
		want      bool
	}{
		{"Original is valid", EntryTypeOriginal, true},
		{"Vault-born is valid", EntryTypeVaultBorn, true},
		{"Admitted is valid", EntryTypeAdmitted, true},
		{"Empty string is invalid", EntryType(""), false},
		{"Invalid entry type", EntryType("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entryType.Valid(); got != tt.want {
				t.Errorf("EntryType.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResidentStatus_Valid(t *testing.T) {
	tests := []struct {
		name   string
		status ResidentStatus
		want   bool
	}{
		{"Active is valid", ResidentStatusActive, true},
		{"Deceased is valid", ResidentStatusDeceased, true},
		{"Exiled is valid", ResidentStatusExiled, true},
		{"Surface mission is valid", ResidentStatusSurfaceMission, true},
		{"Quarantine is valid", ResidentStatusQuarantine, true},
		{"Empty string is invalid", ResidentStatus(""), false},
		{"Invalid status", ResidentStatus("RETIRED"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.want {
				t.Errorf("ResidentStatus.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResidentStatus_IsAlive(t *testing.T) {
	tests := []struct {
		name   string
		status ResidentStatus
		want   bool
	}{
		{"Active is alive", ResidentStatusActive, true},
		{"Deceased is not alive", ResidentStatusDeceased, false},
		{"Exiled is alive", ResidentStatusExiled, true},
		{"Surface mission is alive", ResidentStatusSurfaceMission, true},
		{"Quarantine is alive", ResidentStatusQuarantine, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsAlive(); got != tt.want {
				t.Errorf("ResidentStatus.IsAlive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResident_FullName(t *testing.T) {
	resident := &Resident{
		Surname:    "Smith",
		GivenNames: "John David",
	}

	want := "Smith, John David"
	if got := resident.FullName(); got != want {
		t.Errorf("Resident.FullName() = %v, want %v", got, want)
	}
}

func TestResident_Age(t *testing.T) {
	tests := []struct {
		name string
		dob  time.Time
		asOf time.Time
		want int
	}{
		{
			name: "30 years old",
			dob:  time.Date(1994, 1, 15, 0, 0, 0, 0, time.UTC),
			asOf: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			want: 30,
		},
		{
			name: "Not yet birthday this year",
			dob:  time.Date(1994, 6, 15, 0, 0, 0, 0, time.UTC),
			asOf: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			want: 29,
		},
		{
			name: "Birthday today",
			dob:  time.Date(1994, 3, 15, 0, 0, 0, 0, time.UTC),
			asOf: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			want: 30,
		},
		{
			name: "Infant (0 years)",
			dob:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			asOf: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resident := &Resident{DateOfBirth: tt.dob}
			if got := resident.Age(tt.asOf); got != tt.want {
				t.Errorf("Resident.Age() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResident_IsAdult(t *testing.T) {
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		dob  time.Time
		want bool
	}{
		{"17 years old is not adult", now.AddDate(-17, 0, 0), false},
		{"18 years old is adult", now.AddDate(-18, 0, 0), true},
		{"30 years old is adult", now.AddDate(-30, 0, 0), true},
		{"65 years old is adult", now.AddDate(-65, 0, 0), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resident := &Resident{DateOfBirth: tt.dob}
			if got := resident.IsAdult(now); got != tt.want {
				t.Errorf("Resident.IsAdult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResident_IsWorkingAge(t *testing.T) {
	now := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		dob  time.Time
		want bool
	}{
		{"15 years old is not working age", now.AddDate(-15, 0, 0), false},
		{"16 years old is working age", now.AddDate(-16, 0, 0), true},
		{"30 years old is working age", now.AddDate(-30, 0, 0), true},
		{"65 years old is working age", now.AddDate(-65, 0, 0), true},
		{"66 years old is not working age", now.AddDate(-66, 0, 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resident := &Resident{DateOfBirth: tt.dob}
			if got := resident.IsWorkingAge(now); got != tt.want {
				t.Errorf("Resident.IsWorkingAge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResident_IsAlive(t *testing.T) {
	tests := []struct {
		name   string
		status ResidentStatus
		want   bool
	}{
		{"Active resident is alive", ResidentStatusActive, true},
		{"Deceased resident is not alive", ResidentStatusDeceased, false},
		{"Exiled resident is alive", ResidentStatusExiled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resident := &Resident{Status: tt.status}
			if got := resident.IsAlive(); got != tt.want {
				t.Errorf("Resident.IsAlive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResident_Validate(t *testing.T) {
	now := time.Now().UTC()
	deathDate := now.AddDate(0, -1, 0)
	parent1ID := "parent-1"
	parent2ID := "parent-2"

	tests := []struct {
		name    string
		resident *Resident
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid resident",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				BloodType:      BloodTypeOPos,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now.AddDate(-1, 0, 0),
				Status:         ResidentStatusActive,
				ClearanceLevel: 3,
			},
			wantErr: false,
		},
		{
			name: "Missing ID",
			resident: &Resident{
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now,
				Status:         ResidentStatusActive,
				ClearanceLevel: 3,
			},
			wantErr: true,
			errMsg:  "id is required",
		},
		{
			name: "Missing registry number",
			resident: &Resident{
				ID:             "res-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now,
				Status:         ResidentStatusActive,
				ClearanceLevel: 3,
			},
			wantErr: true,
			errMsg:  "registry_number is required",
		},
		{
			name: "Invalid sex",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            Sex("X"),
				EntryType:      EntryTypeOriginal,
				EntryDate:      now,
				Status:         ResidentStatusActive,
				ClearanceLevel: 3,
			},
			wantErr: true,
			errMsg:  "invalid sex",
		},
		{
			name: "Invalid blood type",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				BloodType:      BloodType("C+"),
				EntryType:      EntryTypeOriginal,
				EntryDate:      now,
				Status:         ResidentStatusActive,
				ClearanceLevel: 3,
			},
			wantErr: true,
			errMsg:  "invalid blood_type",
		},
		{
			name: "Clearance level too low",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now,
				Status:         ResidentStatusActive,
				ClearanceLevel: 0,
			},
			wantErr: true,
			errMsg:  "clearance_level must be between 1 and 10",
		},
		{
			name: "Clearance level too high",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now,
				Status:         ResidentStatusActive,
				ClearanceLevel: 11,
			},
			wantErr: true,
			errMsg:  "clearance_level must be between 1 and 10",
		},
		{
			name: "Vault-born without parents",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-5, 0, 0),
				Sex:            SexMale,
				EntryType:      EntryTypeVaultBorn,
				EntryDate:      now.AddDate(-5, 0, 0),
				Status:         ResidentStatusActive,
				ClearanceLevel: 3,
			},
			wantErr: true,
			errMsg:  "vault-born residents must have both biological parents",
		},
		{
			name: "Vault-born with parents is valid",
			resident: &Resident{
				ID:                  "res-001",
				RegistryNumber:      "VT-076-001",
				Surname:             "Smith",
				GivenNames:          "John",
				DateOfBirth:         now.AddDate(-5, 0, 0),
				Sex:                 SexMale,
				EntryType:           EntryTypeVaultBorn,
				EntryDate:           now.AddDate(-5, 0, 0),
				Status:              ResidentStatusActive,
				BiologicalParent1ID: &parent1ID,
				BiologicalParent2ID: &parent2ID,
				ClearanceLevel:      3,
			},
			wantErr: false,
		},
		{
			name: "Deceased without death date",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				Sex:            SexMale,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now.AddDate(-1, 0, 0),
				Status:         ResidentStatusDeceased,
				ClearanceLevel: 3,
			},
			wantErr: true,
			errMsg:  "deceased residents must have date_of_death",
		},
		{
			name: "Deceased with death date is valid",
			resident: &Resident{
				ID:             "res-001",
				RegistryNumber: "VT-076-001",
				Surname:        "Smith",
				GivenNames:     "John",
				DateOfBirth:    now.AddDate(-30, 0, 0),
				DateOfDeath:    &deathDate,
				Sex:            SexMale,
				EntryType:      EntryTypeOriginal,
				EntryDate:      now.AddDate(-1, 0, 0),
				Status:         ResidentStatusDeceased,
				ClearanceLevel: 3,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resident.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Resident.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Resident.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}
