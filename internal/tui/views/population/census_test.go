package population

import (
	"strings"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

func TestCensusView_New(t *testing.T) {
	view := NewCensusView(nil)
	if view == nil {
		t.Fatal("expected non-nil view")
	}
	if view.table == nil {
		t.Fatal("expected non-nil table")
	}
}

func TestCensusView_EmptyRender(t *testing.T) {
	view := NewCensusView(nil)
	output := view.Render(120, 40)

	if !strings.Contains(output, "POPULATION CENSUS") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "No residents found") {
		t.Error("expected empty state message")
	}
}

func TestCensusView_RenderHelp_Wide(t *testing.T) {
	view := NewCensusView(nil)
	output := view.Render(120, 40)

	if !strings.Contains(output, "PgUp/Dn:Page") {
		t.Error("expected full help text on wide terminal")
	}
}

func TestCensusView_RenderHelp_Narrow(t *testing.T) {
	view := NewCensusView(nil)
	output := view.Render(50, 40)

	if !strings.Contains(output, "s:Search") {
		t.Error("expected compact help text on narrow terminal")
	}
}

func TestCensusView_RenderWithSearch(t *testing.T) {
	view := NewCensusView(nil)
	view.SetSearch("John")

	output := view.Render(120, 40)
	if !strings.Contains(output, "Search:") {
		t.Error("expected search label in output")
	}
	if !strings.Contains(output, "John") {
		t.Error("expected search term in output")
	}
}

func TestCensusView_RenderWithStatusFilter(t *testing.T) {
	view := NewCensusView(nil)
	status := models.ResidentStatusActive
	view.SetStatusFilter(&status)

	output := view.Render(120, 40)
	if !strings.Contains(output, "Status:") {
		t.Error("expected status label in output")
	}
	if !strings.Contains(output, "ACTIVE") {
		t.Error("expected status value in output")
	}
}

func TestCensusView_RenderDetail_NilResident(t *testing.T) {
	view := NewCensusView(nil)
	output := view.RenderDetail(nil, 120)

	// The label style has a fixed width that may wrap the text
	if !strings.Contains(output, "No resident") {
		t.Error("expected 'No resident' message for nil resident")
	}
}

func TestCensusView_RenderDetail_ActiveResident(t *testing.T) {
	view := NewCensusView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	dob := now.AddDate(-30, 0, 0)
	entryDate := now.AddDate(-2, 0, 0)
	householdID := "hh-001"

	resident := &models.Resident{
		ID:             "res-001",
		RegistryNumber: "VT-076-0001",
		Surname:        "Smith",
		GivenNames:     "John",
		DateOfBirth:    dob,
		Sex:            models.SexMale,
		BloodType:      models.BloodTypeOPos,
		EntryType:      models.EntryTypeOriginal,
		EntryDate:      entryDate,
		Status:         models.ResidentStatusActive,
		ClearanceLevel: 5,
		HouseholdID:    &householdID,
		Notes:          "Engineering specialist",
	}

	output := view.RenderDetail(resident, 120)

	checks := []struct {
		label string
		value string
	}{
		{"title", "RESIDENT DETAILS"},
		{"registry", "VT-076-0001"},
		{"name", "Smith"},
		{"sex", "Male"},
		{"blood type", "O+"},
		{"status", "ACTIVE"},
		{"clearance", "5"},
		{"entry type", "ORIGINAL"},
		{"age", "30 years"},
		{"household", "hh-001"},
		{"notes", "Engineering specialist"},
		{"help", "Esc:Back"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.value) {
			t.Errorf("expected %s (%q) in detail output", check.label, check.value)
		}
	}
}

func TestCensusView_RenderDetail_DeceasedResident(t *testing.T) {
	view := NewCensusView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	dob := now.AddDate(-50, 0, 0)
	deathDate := now.AddDate(0, -1, 0)

	resident := &models.Resident{
		ID:             "res-002",
		RegistryNumber: "VT-076-0002",
		Surname:        "Jones",
		GivenNames:     "Mary",
		DateOfBirth:    dob,
		Sex:            models.SexFemale,
		EntryType:      models.EntryTypeOriginal,
		EntryDate:      now.AddDate(-5, 0, 0),
		Status:         models.ResidentStatusDeceased,
		ClearanceLevel: 3,
		DateOfDeath:    &deathDate,
	}

	output := view.RenderDetail(resident, 120)

	if !strings.Contains(output, "DECEASED") {
		t.Error("expected DECEASED status in output")
	}
	if !strings.Contains(output, "Date of Death:") {
		t.Error("expected death date label in output")
	}
}

func TestCensusView_RenderDetail_Responsive(t *testing.T) {
	view := NewCensusView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	resident := &models.Resident{
		ID:             "res-001",
		RegistryNumber: "VT-076-0001",
		Surname:        "Smith",
		GivenNames:     "John",
		DateOfBirth:    now.AddDate(-25, 0, 0),
		Sex:            models.SexMale,
		EntryType:      models.EntryTypeOriginal,
		EntryDate:      now.AddDate(-1, 0, 0),
		Status:         models.ResidentStatusActive,
		ClearanceLevel: 1,
	}

	wide := view.RenderDetail(resident, 120)
	narrow := view.RenderDetail(resident, 50)

	// Both should show resident info
	if !strings.Contains(wide, "VT-076-0001") {
		t.Error("expected registry in wide output")
	}
	if !strings.Contains(narrow, "VT-076-0001") {
		t.Error("expected registry in narrow output")
	}

	// Wide should show full help, narrow should show compact
	if !strings.Contains(wide, "d:Death Record") {
		t.Error("expected full help in wide output")
	}
	if !strings.Contains(narrow, "d:Death") {
		t.Error("expected compact help in narrow output")
	}
}

func TestCensusView_SetSearch(t *testing.T) {
	view := NewCensusView(nil)
	view.SetSearch("test")

	if view.search != "test" {
		t.Errorf("expected search 'test', got %q", view.search)
	}
	if view.filter.SearchTerm != "test" {
		t.Errorf("expected filter search term 'test', got %q", view.filter.SearchTerm)
	}
}

func TestCensusView_SetSearch_ResetsPage(t *testing.T) {
	view := NewCensusView(nil)
	view.page.Page = 5

	view.SetSearch("test")
	if view.page.Page != 1 {
		t.Errorf("expected page 1 after search, got %d", view.page.Page)
	}
}

func TestCensusView_SetStatusFilter(t *testing.T) {
	view := NewCensusView(nil)
	status := models.ResidentStatusActive
	view.SetStatusFilter(&status)

	if view.filter.Status == nil {
		t.Fatal("expected status filter to be set")
	}
	if *view.filter.Status != models.ResidentStatusActive {
		t.Errorf("expected ACTIVE, got %s", *view.filter.Status)
	}
}

func TestCensusView_SetStatusFilter_ResetsPage(t *testing.T) {
	view := NewCensusView(nil)
	view.page.Page = 3

	status := models.ResidentStatusDeceased
	view.SetStatusFilter(&status)
	if view.page.Page != 1 {
		t.Errorf("expected page 1 after filter, got %d", view.page.Page)
	}
}

func TestCensusView_Navigation_Empty(t *testing.T) {
	view := NewCensusView(nil)

	view.MoveUp()
	view.MoveDown()

	if view.SelectedResident() != nil {
		t.Error("expected nil selected resident with no data")
	}
}

func TestCensusView_Pagination(t *testing.T) {
	view := NewCensusView(nil)

	view.NextPage()
	view.PrevPage()
	view.PrevPage() // Should not go below 1
}

func TestCensusView_SetVaultTime(t *testing.T) {
	view := NewCensusView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	if view.vaultTime.IsZero() {
		t.Error("expected non-zero vault time")
	}
}

func TestCensusView_SetVisibleRows(t *testing.T) {
	view := NewCensusView(nil)
	view.SetVisibleRows(15)
	// Should not panic
}
