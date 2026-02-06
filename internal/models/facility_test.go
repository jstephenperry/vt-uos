package models

import (
	"testing"
	"time"
)

func TestSystemCategory_Valid(t *testing.T) {
	tests := []struct {
		cat   SystemCategory
		valid bool
	}{
		{SystemCategoryPower, true},
		{SystemCategoryWater, true},
		{SystemCategoryHVAC, true},
		{SystemCategorySecurity, true},
		{SystemCategoryMedical, true},
		{SystemCategoryFoodProduction, true},
		{SystemCategoryWaste, true},
		{SystemCategoryCommunications, true},
		{SystemCategoryStructural, true},
		{SystemCategory("INVALID"), false},
	}

	for _, tt := range tests {
		if got := tt.cat.Valid(); got != tt.valid {
			t.Errorf("SystemCategory(%q).Valid() = %v, want %v", tt.cat, got, tt.valid)
		}
	}
}

func TestSystemStatus_Valid(t *testing.T) {
	tests := []struct {
		status SystemStatus
		valid  bool
	}{
		{SystemStatusOperational, true},
		{SystemStatusDegraded, true},
		{SystemStatusMaintenance, true},
		{SystemStatusOffline, true},
		{SystemStatusFailed, true},
		{SystemStatusDestroyed, true},
		{SystemStatus("INVALID"), false},
	}

	for _, tt := range tests {
		if got := tt.status.Valid(); got != tt.valid {
			t.Errorf("SystemStatus(%q).Valid() = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestSystemStatus_IsOperational(t *testing.T) {
	tests := []struct {
		status      SystemStatus
		operational bool
	}{
		{SystemStatusOperational, true},
		{SystemStatusDegraded, true},
		{SystemStatusMaintenance, false},
		{SystemStatusOffline, false},
		{SystemStatusFailed, false},
		{SystemStatusDestroyed, false},
	}

	for _, tt := range tests {
		if got := tt.status.IsOperational(); got != tt.operational {
			t.Errorf("SystemStatus(%q).IsOperational() = %v, want %v", tt.status, got, tt.operational)
		}
	}
}

func TestFacilitySystem_Validate(t *testing.T) {
	now := time.Now().UTC()

	validSystem := func() *FacilitySystem {
		return &FacilitySystem{
			ID:                      "test-id",
			SystemCode:              "PWR-GEN-01",
			Name:                    "Primary Generator",
			Category:                SystemCategoryPower,
			LocationSector:          "A",
			LocationLevel:           2,
			Status:                  SystemStatusOperational,
			EfficiencyPercent:       95.0,
			InstallDate:             now,
			MaintenanceIntervalDays: 90,
		}
	}

	t.Run("Valid system passes", func(t *testing.T) {
		if err := validSystem().Validate(); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("Missing ID", func(t *testing.T) {
		s := validSystem()
		s.ID = ""
		if err := s.Validate(); err == nil {
			t.Error("expected error for missing ID")
		}
	})

	t.Run("Missing system_code", func(t *testing.T) {
		s := validSystem()
		s.SystemCode = ""
		if err := s.Validate(); err == nil {
			t.Error("expected error for missing system_code")
		}
	})

	t.Run("Invalid category", func(t *testing.T) {
		s := validSystem()
		s.Category = "INVALID"
		if err := s.Validate(); err == nil {
			t.Error("expected error for invalid category")
		}
	})

	t.Run("Efficiency out of range", func(t *testing.T) {
		s := validSystem()
		s.EfficiencyPercent = 101
		if err := s.Validate(); err == nil {
			t.Error("expected error for efficiency > 100")
		}
	})
}

func TestFacilitySystem_IsOverdueForMaintenance(t *testing.T) {
	now := time.Now().UTC()
	pastDue := now.AddDate(0, 0, -7)
	futureDue := now.AddDate(0, 0, 7)

	t.Run("Overdue system", func(t *testing.T) {
		s := &FacilitySystem{NextMaintenanceDue: &pastDue}
		if !s.IsOverdueForMaintenance(now) {
			t.Error("expected overdue system to be detected")
		}
	})

	t.Run("Not overdue", func(t *testing.T) {
		s := &FacilitySystem{NextMaintenanceDue: &futureDue}
		if s.IsOverdueForMaintenance(now) {
			t.Error("expected system to not be overdue")
		}
	})

	t.Run("No maintenance date", func(t *testing.T) {
		s := &FacilitySystem{}
		if s.IsOverdueForMaintenance(now) {
			t.Error("expected system without date to not be overdue")
		}
	})
}

func TestMaintenanceType_Valid(t *testing.T) {
	tests := []struct {
		mt    MaintenanceType
		valid bool
	}{
		{MaintenanceTypePreventive, true},
		{MaintenanceTypeCorrective, true},
		{MaintenanceTypeEmergency, true},
		{MaintenanceTypeInspection, true},
		{MaintenanceTypeUpgrade, true},
		{MaintenanceType("INVALID"), false},
	}

	for _, tt := range tests {
		if got := tt.mt.Valid(); got != tt.valid {
			t.Errorf("MaintenanceType(%q).Valid() = %v, want %v", tt.mt, got, tt.valid)
		}
	}
}

func TestMaintenanceOutcome_Valid(t *testing.T) {
	tests := []struct {
		outcome MaintenanceOutcome
		valid   bool
	}{
		{MaintenanceOutcomeCompleted, true},
		{MaintenanceOutcomePartial, true},
		{MaintenanceOutcomeFailed, true},
		{MaintenanceOutcomeDeferred, true},
		{MaintenanceOutcomeCancelled, true},
		{MaintenanceOutcome("INVALID"), false},
	}

	for _, tt := range tests {
		if got := tt.outcome.Valid(); got != tt.valid {
			t.Errorf("MaintenanceOutcome(%q).Valid() = %v, want %v", tt.outcome, got, tt.valid)
		}
	}
}

func TestMaintenanceRecord_Validate(t *testing.T) {
	t.Run("Valid record", func(t *testing.T) {
		r := &MaintenanceRecord{
			ID:              "test-id",
			SystemID:        "sys-id",
			MaintenanceType: MaintenanceTypePreventive,
			Description:     "Routine check",
		}
		if err := r.Validate(); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("Missing system_id", func(t *testing.T) {
		r := &MaintenanceRecord{
			ID:              "test-id",
			MaintenanceType: MaintenanceTypePreventive,
			Description:     "Routine check",
		}
		if err := r.Validate(); err == nil {
			t.Error("expected error for missing system_id")
		}
	})

	t.Run("Invalid maintenance type", func(t *testing.T) {
		r := &MaintenanceRecord{
			ID:              "test-id",
			SystemID:        "sys-id",
			MaintenanceType: "INVALID",
			Description:     "Routine check",
		}
		if err := r.Validate(); err == nil {
			t.Error("expected error for invalid maintenance type")
		}
	})
}

func TestMaintenanceRecord_IsComplete(t *testing.T) {
	completed := MaintenanceOutcomeCompleted
	partial := MaintenanceOutcomePartial

	t.Run("Completed record", func(t *testing.T) {
		r := &MaintenanceRecord{Outcome: &completed}
		if !r.IsComplete() {
			t.Error("expected IsComplete to return true")
		}
	})

	t.Run("Partial record", func(t *testing.T) {
		r := &MaintenanceRecord{Outcome: &partial}
		if r.IsComplete() {
			t.Error("expected IsComplete to return false for partial")
		}
	})

	t.Run("No outcome", func(t *testing.T) {
		r := &MaintenanceRecord{}
		if r.IsComplete() {
			t.Error("expected IsComplete to return false when no outcome")
		}
	})
}
