package facilities

import (
	"strings"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

func TestSystemsView_New(t *testing.T) {
	view := NewSystemsView(nil)
	if view == nil {
		t.Fatal("expected non-nil view")
	}
	if view.table == nil {
		t.Fatal("expected non-nil table")
	}
}

func TestSystemsView_EmptyRender(t *testing.T) {
	view := NewSystemsView(nil)
	output := view.Render(120, 40)

	if !strings.Contains(output, "FACILITY SYSTEMS") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "No facility systems found") {
		t.Error("expected empty state message")
	}
}

func TestSystemsView_RenderHelp_Wide(t *testing.T) {
	view := NewSystemsView(nil)
	output := view.Render(120, 40)

	if !strings.Contains(output, "PgUp/Dn:Page") {
		t.Error("expected full help text on wide terminal")
	}
}

func TestSystemsView_RenderHelp_Narrow(t *testing.T) {
	view := NewSystemsView(nil)
	output := view.Render(50, 40)

	if !strings.Contains(output, "c:Category") {
		t.Error("expected compact help text on narrow terminal")
	}
}

func TestSystemsView_RenderDetail_NilSystem(t *testing.T) {
	view := NewSystemsView(nil)
	output := view.RenderDetail(nil, 120)

	if !strings.Contains(output, "No system selected") {
		t.Error("expected 'No system selected' for nil system")
	}
}

func TestSystemsView_RenderDetail_Operational(t *testing.T) {
	view := NewSystemsView(nil)
	now := time.Now().UTC()
	nextMaint := now.AddDate(0, 0, 30)
	lastMaint := now.AddDate(0, -1, 0)
	mtbf := 5000

	system := &models.FacilitySystem{
		ID:                      "test-id",
		SystemCode:              "PWR-GEN-01",
		Name:                    "Primary Generator",
		Category:                models.SystemCategoryPower,
		LocationSector:          "A",
		LocationLevel:           2,
		Status:                  models.SystemStatusOperational,
		EfficiencyPercent:       95.0,
		TotalRuntimeHours:       17520,
		InstallDate:             now.AddDate(-2, 0, 0),
		LastMaintenanceDate:     &lastMaint,
		NextMaintenanceDue:      &nextMaint,
		MaintenanceIntervalDays: 90,
		MTBFHours:               &mtbf,
		Notes:                   "Primary power generation unit",
	}

	output := view.RenderDetail(system, 120)

	checks := []struct {
		label string
		value string
	}{
		{"title", "SYSTEM DETAILS"},
		{"system code", "PWR-GEN-01"},
		{"name", "Primary Generator"},
		{"category", "POWER"},
		{"status", "OPERATIONAL"},
		{"efficiency", "95.0%"},
		{"location", "Sector A"},
		{"level", "Level 2"},
		{"runtime", "17520 hours"},
		{"interval", "90 days"},
		{"MTBF", "5000 hours"},
		{"notes section", "NOTES"},
		{"notes content", "Primary power generation unit"},
		{"help", "Esc:Back"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.value) {
			t.Errorf("expected %s (%q) in output", check.label, check.value)
		}
	}
}

func TestSystemsView_RenderDetail_DegradedStatus(t *testing.T) {
	view := NewSystemsView(nil)
	now := time.Now().UTC()

	system := &models.FacilitySystem{
		ID:                      "test-id",
		SystemCode:              "WST-PROC-01",
		Name:                    "Waste Processor",
		Category:                models.SystemCategoryWaste,
		LocationSector:          "C",
		LocationLevel:           3,
		Status:                  models.SystemStatusDegraded,
		EfficiencyPercent:       45.0,
		TotalRuntimeHours:       5000,
		InstallDate:             now.AddDate(-1, 0, 0),
		MaintenanceIntervalDays: 60,
	}

	output := view.RenderDetail(system, 120)

	if !strings.Contains(output, "DEGRADED") {
		t.Error("expected DEGRADED status in output")
	}
	if !strings.Contains(output, "45.0%") {
		t.Error("expected low efficiency in output")
	}
}

func TestSystemsView_RenderDetail_WithCapacity(t *testing.T) {
	view := NewSystemsView(nil)
	now := time.Now().UTC()
	capacity := 100.0
	output := 85.0

	system := &models.FacilitySystem{
		ID:                      "test-id",
		SystemCode:              "PWR-GEN-01",
		Name:                    "Generator",
		Category:                models.SystemCategoryPower,
		Status:                  models.SystemStatusOperational,
		EfficiencyPercent:       95.0,
		CapacityRating:          &capacity,
		CurrentOutput:           &output,
		CapacityUnit:            "kW",
		InstallDate:             now,
		MaintenanceIntervalDays: 90,
	}

	detail := view.RenderDetail(system, 120)

	if !strings.Contains(detail, "85.0") {
		t.Error("expected current output in detail")
	}
	if !strings.Contains(detail, "100.0") {
		t.Error("expected capacity rating in detail")
	}
	if !strings.Contains(detail, "kW") {
		t.Error("expected capacity unit in detail")
	}
}

func TestSystemsView_RenderDetail_Responsive(t *testing.T) {
	view := NewSystemsView(nil)
	now := time.Now().UTC()

	system := &models.FacilitySystem{
		ID:                      "test-id",
		SystemCode:              "PWR-GEN-01",
		Name:                    "Generator",
		Category:                models.SystemCategoryPower,
		Status:                  models.SystemStatusOperational,
		EfficiencyPercent:       95.0,
		InstallDate:             now,
		MaintenanceIntervalDays: 90,
	}

	// Wide and narrow should both contain system code
	wide := view.RenderDetail(system, 120)
	narrow := view.RenderDetail(system, 50)

	if !strings.Contains(wide, "PWR-GEN-01") {
		t.Error("expected system code in wide output")
	}
	if !strings.Contains(narrow, "PWR-GEN-01") {
		t.Error("expected system code in narrow output")
	}
}

func TestSystemsView_RenderDetail_OverdueMaintenance(t *testing.T) {
	view := NewSystemsView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	pastDue := now.AddDate(0, 0, -7)
	system := &models.FacilitySystem{
		ID:                      "test-id",
		SystemCode:              "SYS-01",
		Name:                    "Test System",
		Category:                models.SystemCategoryPower,
		Status:                  models.SystemStatusOperational,
		EfficiencyPercent:       90.0,
		InstallDate:             now.AddDate(-1, 0, 0),
		NextMaintenanceDue:      &pastDue,
		MaintenanceIntervalDays: 90,
	}

	output := view.RenderDetail(system, 120)
	// The overdue date should still be displayed (with error styling)
	if !strings.Contains(output, pastDue.Format("2006-01-02")) {
		t.Error("expected overdue date in output")
	}
}

func TestSystemsView_CategoryFilter(t *testing.T) {
	view := NewSystemsView(nil)

	if view.GetCategoryFilter() != nil {
		t.Error("expected nil initial category filter")
	}

	cat := models.SystemCategoryPower
	view.SetCategoryFilter(&cat)

	if view.GetCategoryFilter() == nil {
		t.Error("expected non-nil category filter")
	}
	if *view.GetCategoryFilter() != models.SystemCategoryPower {
		t.Errorf("expected POWER, got %s", *view.GetCategoryFilter())
	}

	view.SetCategoryFilter(nil)
	if view.GetCategoryFilter() != nil {
		t.Error("expected nil after clearing filter")
	}
}

func TestSystemsView_RenderWithFilter(t *testing.T) {
	view := NewSystemsView(nil)
	cat := models.SystemCategoryPower
	view.SetCategoryFilter(&cat)

	output := view.Render(120, 40)
	if !strings.Contains(output, "POWER") {
		t.Error("expected category filter in render output")
	}
}

func TestSystemsView_Navigation_Empty(t *testing.T) {
	view := NewSystemsView(nil)

	// Should be safe with no data
	view.MoveUp()
	view.MoveDown()

	if view.SelectedSystem() != nil {
		t.Error("expected nil selected system with no data")
	}
}

func TestSystemsView_Pagination(t *testing.T) {
	view := NewSystemsView(nil)

	view.NextPage()
	view.PrevPage()
	view.PrevPage() // Should not go below 1
}

func TestSystemsView_SetVaultTime(t *testing.T) {
	view := NewSystemsView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	if view.vaultTime.IsZero() {
		t.Error("expected non-zero vault time")
	}
}

func TestSystemsView_SetVisibleRows(t *testing.T) {
	view := NewSystemsView(nil)
	view.SetVisibleRows(10)
	// Should not panic
}
