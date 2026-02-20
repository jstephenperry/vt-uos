package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestApp_InitialState(t *testing.T) {
	app := newTestApp(t)

	if app.currentModule != ModuleDashboard {
		t.Errorf("expected initial module Dashboard, got %s", app.currentModule)
	}
	if !app.ready {
		t.Error("expected app to be ready")
	}
	if app.quitting {
		t.Error("expected app not to be quitting")
	}
	if app.showDetail {
		t.Error("expected no detail shown initially")
	}
	if app.showForm {
		t.Error("expected no form shown initially")
	}
	if app.searchMode {
		t.Error("expected search mode off initially")
	}
}

func TestApp_View_NotReady(t *testing.T) {
	app := newTestApp(t)
	app.ready = false

	output := app.View()
	if !strings.Contains(output, "Initializing") {
		t.Error("expected initialization message when not ready")
	}
}

func TestApp_View_Quitting(t *testing.T) {
	app := newTestApp(t)
	app.quitting = true

	output := app.View()
	if !strings.Contains(output, "shutting down") {
		t.Error("expected shutdown message when quitting")
	}
}

func TestApp_View_Dashboard(t *testing.T) {
	app := newTestApp(t)
	output := app.View()

	if !strings.Contains(output, "VAULT STATUS OVERVIEW") {
		t.Error("expected dashboard title in view output")
	}
}

func TestApp_ModuleNavigation_FKeys(t *testing.T) {
	tests := []struct {
		key      tea.KeyType
		expected Module
	}{
		{tea.KeyF3, ModulePopulation},
		{tea.KeyF4, ModuleResources},
		{tea.KeyF5, ModuleFacilities},
		{tea.KeyF6, ModuleLabor},
		{tea.KeyF7, ModuleMedical},
		{tea.KeyF8, ModuleSecurity},
		{tea.KeyF9, ModuleGovernance},
		{tea.KeyF2, ModuleDashboard},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			app := newTestApp(t)
			app.Update(specialKeyMsg(tt.key))

			if app.currentModule != tt.expected {
				t.Errorf("expected module %s, got %s", tt.expected, app.currentModule)
			}
		})
	}
}

func TestApp_ModuleNavigation_HelpKey(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF1))

	if app.currentModule != ModuleHelp {
		t.Errorf("expected Help module, got %s", app.currentModule)
	}
}

func TestApp_ModuleNavigation_ClearsDetail(t *testing.T) {
	app := newTestApp(t)
	app.showDetail = true

	app.Update(specialKeyMsg(tea.KeyF5))

	if app.showDetail {
		t.Error("expected detail to be cleared on module switch")
	}
}

func TestApp_QuitConfirmation_Show(t *testing.T) {
	app := newTestApp(t)
	app.Update(keyMsg("q"))

	if !app.showConfirm {
		t.Error("expected quit confirmation to show")
	}
}

func TestApp_QuitConfirmation_Cancel(t *testing.T) {
	app := newTestApp(t)
	app.Update(keyMsg("q"))
	app.Update(keyMsg("n"))

	if app.showConfirm {
		t.Error("expected quit confirmation to be dismissed")
	}
	if app.quitting {
		t.Error("expected app not to be quitting after cancel")
	}
}

func TestApp_QuitConfirmation_Confirm(t *testing.T) {
	app := newTestApp(t)
	app.Update(keyMsg("q"))
	_, cmd := app.Update(keyMsg("y"))

	if !app.quitting {
		t.Error("expected app to be quitting after confirm")
	}
	// The returned command should be tea.Quit
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestApp_QuitConfirmation_F10(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF10))

	if !app.showConfirm {
		t.Error("expected quit confirmation from F10")
	}
}

func TestApp_QuitConfirmation_EscCancels(t *testing.T) {
	app := newTestApp(t)
	app.Update(keyMsg("q"))
	app.Update(specialKeyMsg(tea.KeyEscape))

	if app.showConfirm {
		t.Error("expected Esc to dismiss confirmation")
	}
}

func TestApp_QuitConfirmation_IgnoresOtherKeys(t *testing.T) {
	app := newTestApp(t)
	app.Update(keyMsg("q"))
	app.Update(keyMsg("x"))

	if !app.showConfirm {
		t.Error("expected confirmation to stay open on unrelated key")
	}
}

func TestApp_ConfirmDialog_Render(t *testing.T) {
	app := newTestApp(t)
	app.showConfirm = true

	output := app.View()
	if !strings.Contains(output, "CONFIRM EXIT") {
		t.Error("expected confirm dialog in output")
	}
}

func TestApp_WindowResize(t *testing.T) {
	app := newTestApp(t)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if app.width != 80 {
		t.Errorf("expected width 80, got %d", app.width)
	}
	if app.height != 24 {
		t.Errorf("expected height 24, got %d", app.height)
	}
	if !app.ready {
		t.Error("expected app ready after window size")
	}
}

func TestApp_PopulationNavigation(t *testing.T) {
	app := newTestApp(t)

	// Navigate to population
	app.Update(specialKeyMsg(tea.KeyF3))
	if app.currentModule != ModulePopulation {
		t.Fatalf("expected Population, got %s", app.currentModule)
	}

	// Process the census load message
	app.Update(censusLoadedMsg{})

	// Navigate down/up (no data, should not crash)
	app.Update(specialKeyMsg(tea.KeyDown))
	app.Update(specialKeyMsg(tea.KeyUp))

	output := app.View()
	if !strings.Contains(output, "POPULATION CENSUS") {
		t.Error("expected census view in output")
	}
}

func TestApp_PopulationNavigation_ViKeys(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	// j/k navigation should work
	app.Update(keyMsg("j"))
	app.Update(keyMsg("k"))
}

func TestApp_PopulationSearchMode_Enter(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	// Enter search mode with '/'
	app.Update(keyMsg("/"))
	if !app.searchMode {
		t.Error("expected search mode to be active")
	}

	// Type search term
	app.Update(keyMsg("J"))
	app.Update(keyMsg("o"))
	app.Update(keyMsg("h"))
	app.Update(keyMsg("n"))
	if app.searchInput != "John" {
		t.Errorf("expected search 'John', got %q", app.searchInput)
	}

	// View should show search bar
	output := app.View()
	if !strings.Contains(output, "SEARCH") {
		t.Error("expected SEARCH bar in output during search mode")
	}
}

func TestApp_PopulationSearchMode_Backspace(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	app.Update(keyMsg("s"))
	app.Update(keyMsg("A"))
	app.Update(keyMsg("B"))
	app.Update(specialKeyMsg(tea.KeyBackspace))

	if app.searchInput != "A" {
		t.Errorf("expected 'A' after backspace, got %q", app.searchInput)
	}
}

func TestApp_PopulationSearchMode_Cancel(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	app.Update(keyMsg("/"))
	app.Update(keyMsg("T"))
	app.Update(keyMsg("e"))

	// Cancel with Esc
	app.Update(specialKeyMsg(tea.KeyEscape))
	if app.searchMode {
		t.Error("expected search mode off after Esc")
	}
	if app.searchInput != "" {
		t.Errorf("expected empty search after cancel, got %q", app.searchInput)
	}
}

func TestApp_PopulationSearchMode_Submit(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	app.Update(keyMsg("s"))
	app.Update(keyMsg("T"))
	app.Update(specialKeyMsg(tea.KeyEnter))

	if app.searchMode {
		t.Error("expected search mode off after submit")
	}
}

func TestApp_PopulationAddResident(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	app.Update(keyMsg("a"))

	if !app.showForm {
		t.Error("expected form to be shown after 'a'")
	}
	if app.residentForm == nil {
		t.Error("expected resident form to be created")
	}
}

func TestApp_PopulationPagination(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	// Page navigation shouldn't crash with empty data
	app.Update(specialKeyMsg(tea.KeyPgDown))
	app.Update(specialKeyMsg(tea.KeyPgUp))
}

func TestApp_FacilitiesNavigation(t *testing.T) {
	app := newTestApp(t)

	app.Update(specialKeyMsg(tea.KeyF5))
	if app.currentModule != ModuleFacilities {
		t.Fatalf("expected Facilities, got %s", app.currentModule)
	}

	app.Update(systemsLoadedMsg{})

	// Navigate
	app.Update(specialKeyMsg(tea.KeyDown))
	app.Update(specialKeyMsg(tea.KeyUp))
	app.Update(keyMsg("j"))
	app.Update(keyMsg("k"))

	output := app.View()
	if !strings.Contains(output, "FACILITY SYSTEMS") {
		t.Error("expected facilities view in output")
	}
}

func TestApp_FacilitiesCategoryFilter(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF5))
	app.Update(systemsLoadedMsg{})

	// Press 'c' to cycle category
	app.Update(keyMsg("c"))
	if app.systemsView.GetCategoryFilter() == nil {
		t.Error("expected category filter to be set after 'c'")
	}

	// Cycle through all categories and back to nil
	for i := 0; i < 9; i++ {
		app.Update(keyMsg("c"))
		app.Update(systemsLoadedMsg{})
	}

	// Should be back to nil (all categories)
	if app.systemsView.GetCategoryFilter() != nil {
		t.Error("expected category filter to be nil after full cycle")
	}
}

func TestApp_FacilitiesDetailView(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF5))
	app.Update(systemsLoadedMsg{})

	// Manually set detail mode
	app.showDetail = true

	output := app.View()
	// With no data, should show "No system selected"
	if !strings.Contains(output, "No system selected") {
		t.Error("expected 'No system selected' in detail with no data")
	}

	// Esc should go back
	app.Update(specialKeyMsg(tea.KeyEscape))
	if app.showDetail {
		t.Error("expected detail hidden after Esc")
	}
}

func TestApp_FacilitiesPagination(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF5))
	app.Update(systemsLoadedMsg{})

	app.Update(specialKeyMsg(tea.KeyPgDown))
	app.Update(specialKeyMsg(tea.KeyPgUp))
}

func TestApp_ResourcesNavigation(t *testing.T) {
	app := newTestApp(t)

	app.Update(specialKeyMsg(tea.KeyF4))
	if app.currentModule != ModuleResources {
		t.Fatalf("expected Resources, got %s", app.currentModule)
	}

	app.Update(inventoryLoadedMsg{})

	app.Update(specialKeyMsg(tea.KeyDown))
	app.Update(specialKeyMsg(tea.KeyUp))

	output := app.View()
	if !strings.Contains(output, "RESOURCE INVENTORY") {
		t.Error("expected inventory view in output")
	}
}

func TestApp_ResourcesDetailView(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF4))
	app.Update(inventoryLoadedMsg{})

	app.showDetail = true

	output := app.View()
	if !strings.Contains(output, "No stock selected") {
		t.Error("expected 'No stock selected' in detail with no data")
	}

	app.Update(specialKeyMsg(tea.KeyEscape))
	if app.showDetail {
		t.Error("expected detail hidden after Esc")
	}
}

func TestApp_ResourcesPagination(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF4))
	app.Update(inventoryLoadedMsg{})

	app.Update(specialKeyMsg(tea.KeyPgDown))
	app.Update(specialKeyMsg(tea.KeyPgUp))
}

func TestApp_BackNavigation_HelpToOriginal(t *testing.T) {
	app := newTestApp(t)

	// Go to facilities first
	app.Update(specialKeyMsg(tea.KeyF5))
	app.Update(systemsLoadedMsg{})

	// Go to help
	app.Update(specialKeyMsg(tea.KeyF1))
	if app.currentModule != ModuleHelp {
		t.Fatalf("expected Help, got %s", app.currentModule)
	}
	if app.previousModule != ModuleFacilities {
		t.Errorf("expected previous module Facilities, got %s", app.previousModule)
	}

	// Go back
	app.Update(specialKeyMsg(tea.KeyEscape))
	if app.currentModule != ModuleFacilities {
		t.Errorf("expected to return to Facilities, got %s", app.currentModule)
	}
}

func TestApp_BackNavigation_DetailToList(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF5))
	app.Update(systemsLoadedMsg{})

	app.showDetail = true

	// Esc hides detail via back handler (before module-specific handling)
	app.Update(specialKeyMsg(tea.KeyEscape))
	if app.showDetail {
		t.Error("expected detail to be hidden after back")
	}
}

func TestApp_AlertManagement(t *testing.T) {
	app := newTestApp(t)

	app.AddAlert(AlertInfo, "Test info")
	app.AddAlert(AlertWarning, "Test warning")
	app.AddAlert(AlertCritical, "Test critical")

	if len(app.alerts) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(app.alerts))
	}

	// Newest alert should be first
	if app.alerts[0].Message != "Test critical" {
		t.Errorf("expected newest alert first, got %q", app.alerts[0].Message)
	}

	output := app.View()
	if !strings.Contains(output, "Test critical") {
		t.Error("expected critical alert in view output")
	}

	// Clear
	app.ClearAlerts()
	if len(app.alerts) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(app.alerts))
	}
}

func TestApp_AlertLimit(t *testing.T) {
	app := newTestApp(t)

	for i := 0; i < 15; i++ {
		app.AddAlert(AlertInfo, fmt.Sprintf("Alert %d", i))
	}

	if len(app.alerts) != 10 {
		t.Errorf("expected max 10 alerts, got %d", len(app.alerts))
	}
}

func TestApp_AlertBar_NoAlerts(t *testing.T) {
	app := newTestApp(t)
	output := app.renderAlertBar()

	if !strings.Contains(output, "All systems operational") {
		t.Error("expected 'All systems operational' with no alerts")
	}
}

func TestApp_TickMessage(t *testing.T) {
	app := newTestApp(t)
	_, cmd := app.Update(tickMsg(time.Now()))

	// Tick should return a new tick command
	if cmd == nil {
		t.Error("expected tick to return a new command")
	}
}

func TestApp_PopulationMessage(t *testing.T) {
	app := newTestApp(t)
	app.Update(populationMsg{count: 42})

	if app.population != 42 {
		t.Errorf("expected population 42, got %d", app.population)
	}
}

func TestApp_CensusLoadError(t *testing.T) {
	app := newTestApp(t)
	app.Update(censusLoadedMsg{err: fmt.Errorf("test error")})

	if len(app.alerts) == 0 {
		t.Error("expected alert on census load error")
	}
}

func TestApp_InventoryLoadError(t *testing.T) {
	app := newTestApp(t)
	app.Update(inventoryLoadedMsg{err: fmt.Errorf("test error")})

	if len(app.alerts) == 0 {
		t.Error("expected alert on inventory load error")
	}
}

func TestApp_SystemsLoadError(t *testing.T) {
	app := newTestApp(t)
	app.Update(systemsLoadedMsg{err: fmt.Errorf("test error")})

	if len(app.alerts) == 0 {
		t.Error("expected alert on systems load error")
	}
}

func TestApp_ModuleRendering(t *testing.T) {
	tests := []struct {
		module   Module
		contains string
	}{
		{ModuleDashboard, "VAULT STATUS OVERVIEW"},
		{ModuleLabor, "LABOR ALLOCATION"},
		{ModuleMedical, "MEDICAL RECORDS"},
		{ModuleSecurity, "SECURITY"},
		{ModuleGovernance, "GOVERNANCE"},
		{ModuleHelp, "HELP"},
	}

	for _, tt := range tests {
		t.Run(string(tt.module), func(t *testing.T) {
			app := newTestApp(t)
			app.currentModule = tt.module

			output := app.View()
			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected %q in %s module output", tt.contains, tt.module)
			}
		})
	}
}

func TestApp_ResponsiveHeader(t *testing.T) {
	app := newTestApp(t)

	// Narrow
	app.width = 50
	output := app.renderHeader()
	if !strings.Contains(output, "VT-UOS") {
		t.Error("expected compact header on narrow terminal")
	}

	// Wide
	app.width = 120
	output = app.renderHeader()
	if !strings.Contains(output, "VAULT-TEC") {
		t.Error("expected full header on wide terminal")
	}
}

func TestApp_ResponsiveFooter(t *testing.T) {
	app := newTestApp(t)
	output := app.renderFooter()

	if !strings.Contains(output, "Help") {
		t.Error("expected help info in footer")
	}
	if !strings.Contains(output, "Quit") {
		t.Error("expected quit info in footer")
	}
}

func TestApp_DashboardPanels(t *testing.T) {
	app := newTestApp(t)
	output := app.renderDashboard()

	if !strings.Contains(output, "POPULATION") {
		t.Error("expected POPULATION panel in dashboard")
	}
	if !strings.Contains(output, "CRITICAL SYSTEMS") {
		t.Error("expected CRITICAL SYSTEMS panel in dashboard")
	}
	if !strings.Contains(output, "RESOURCE STATUS") {
		t.Error("expected RESOURCE STATUS panel in dashboard")
	}
	if !strings.Contains(output, "SIMULATION") {
		t.Error("expected SIMULATION panel in dashboard")
	}
}

func TestApp_DashboardNarrow(t *testing.T) {
	app := newTestApp(t)
	app.width = 50
	output := app.renderDashboard()

	// Should still contain panels (stacked vertically)
	if !strings.Contains(output, "POPULATION") {
		t.Error("expected POPULATION in narrow dashboard")
	}
}

func TestApp_FormMode_Cancel(t *testing.T) {
	app := newTestApp(t)
	app.Update(specialKeyMsg(tea.KeyF3))
	app.Update(censusLoadedMsg{})

	// Enter add form
	app.Update(keyMsg("a"))
	if !app.showForm {
		t.Fatal("expected form to be shown")
	}

	// Cancel form
	app.Update(specialKeyMsg(tea.KeyEscape))
	if app.showForm {
		t.Error("expected form to be hidden after cancel")
	}
}

func TestApp_AlertBarRotation(t *testing.T) {
	app := newTestApp(t)
	app.AddAlert(AlertInfo, "First")
	app.AddAlert(AlertInfo, "Second")

	// Alert index starts at 0 (newest)
	if app.alertIndex != 0 {
		t.Errorf("expected alertIndex 0, got %d", app.alertIndex)
	}

	// Simulate 3 ticks to trigger rotation
	for i := 0; i < 3; i++ {
		app.Update(tickMsg(time.Now()))
	}

	if app.alertIndex == 0 {
		t.Error("expected alert to rotate after 3 ticks")
	}
}
