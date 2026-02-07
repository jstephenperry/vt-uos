package tui

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/util"
)

// newE2EApp creates an App for end-to-end testing via teatest.
// Unlike newTestApp, this does NOT pre-configure width/height/ready
// since teatest sends WindowSizeMsg via WithInitialTermSize.
func newE2EApp(t *testing.T) *App {
	t.Helper()

	db, err := database.NewInMemory()
	if err != nil {
		t.Fatalf("creating test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	migrationsDir := filepath.Join("..", "database", "migrations")
	runTestMigrations(t, db, migrationsDir)

	cfg := config.Default()
	vaultTime, _ := time.Parse(time.RFC3339, "2077-10-23T09:47:00Z")
	clock := util.NewVaultClock(vaultTime, 1.0)
	clock.Pause()

	return New(db, cfg, clock)
}

// waitFor is a convenience wrapper around teatest.WaitFor with a standard timeout.
func waitFor(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(text))
	}, teatest.WithDuration(5*time.Second))
}

// --- End-to-end tests ---
// These launch the real Bubble Tea program in a headless virtual terminal,
// send actual keystrokes, and assert on the rendered screen output.

func TestE2E_DashboardOnStartup(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	waitFor(t, tm, "VAULT STATUS OVERVIEW")
}

func TestE2E_NavigateToPopulation(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")
}

func TestE2E_NavigateToResources(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF4})
	waitFor(t, tm, "RESOURCE INVENTORY")
}

func TestE2E_NavigateToFacilities(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF5})
	waitFor(t, tm, "FACILITY SYSTEMS")
}

func TestE2E_HelpScreenAndBack(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	// F1 → Help
	tm.Send(tea.KeyMsg{Type: tea.KeyF1})
	waitFor(t, tm, "HELP")

	// Esc → Back to dashboard
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	waitFor(t, tm, "VAULT STATUS OVERVIEW")
}

func TestE2E_QuitFlow(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))

	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	// Press q → confirm dialog
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	waitFor(t, tm, "CONFIRM EXIT")

	// Press y → quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})

	// Program should terminate; verify final model state
	m := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
	app, ok := m.(*App)
	if !ok {
		t.Fatal("expected *App final model")
	}
	if !app.quitting {
		t.Error("expected app to be quitting")
	}
}

func TestE2E_QuitCancel(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	// Press q → confirm dialog
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	waitFor(t, tm, "CONFIRM EXIT")

	// Press n → cancel
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	// Verify app is still responsive by navigating to another module
	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")
}

func TestE2E_PopulationEmptyList(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF3})

	// Both the title and empty state appear in the same frame
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("POPULATION CENSUS")) &&
			bytes.Contains(bts, []byte("No residents found"))
	}, teatest.WithDuration(5*time.Second))
}

func TestE2E_FacilitiesEmptyList(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF5})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("FACILITY SYSTEMS")) &&
			bytes.Contains(bts, []byte("No facility systems found"))
	}, teatest.WithDuration(5*time.Second))
}

func TestE2E_ResourcesEmptyList(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF4})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("RESOURCE INVENTORY")) &&
			bytes.Contains(bts, []byte("No inventory found"))
	}, teatest.WithDuration(5*time.Second))
}

func TestE2E_SearchFlow(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// Navigate to population
	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")

	// Enter search mode with '/'
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	waitFor(t, tm, "SEARCH")

	// Type search term
	tm.Type("Smith")
	waitFor(t, tm, "Smith")

	// Submit search with Enter
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify app is still responsive
	tm.Send(tea.KeyMsg{Type: tea.KeyF2})
	waitFor(t, tm, "VAULT STATUS OVERVIEW")
}

func TestE2E_SearchCancel(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")

	// Enter search mode
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	waitFor(t, tm, "SEARCH")

	// Type then cancel
	tm.Type("test")
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})

	// Verify app is still responsive after cancel
	tm.Send(tea.KeyMsg{Type: tea.KeyF5})
	waitFor(t, tm, "FACILITY SYSTEMS")
}

func TestE2E_FacilitiesCategoryFilter(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF5})
	waitFor(t, tm, "FACILITY SYSTEMS")

	// Press 'c' to cycle category filter
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	waitFor(t, tm, "POWER")
}

func TestE2E_FullNavigationRoundTrip(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// Dashboard
	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	// Population
	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")

	// Resources
	tm.Send(tea.KeyMsg{Type: tea.KeyF4})
	waitFor(t, tm, "RESOURCE INVENTORY")

	// Facilities
	tm.Send(tea.KeyMsg{Type: tea.KeyF5})
	waitFor(t, tm, "FACILITY SYSTEMS")

	// Help
	tm.Send(tea.KeyMsg{Type: tea.KeyF1})
	waitFor(t, tm, "HELP")

	// Esc → Back to Facilities
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	waitFor(t, tm, "FACILITY SYSTEMS")

	// F2 → Back to Dashboard
	tm.Send(tea.KeyMsg{Type: tea.KeyF2})
	waitFor(t, tm, "VAULT STATUS OVERVIEW")
}

func TestE2E_NarrowTerminal(t *testing.T) {
	// Test responsive layout with a narrow terminal (like Pi Zero)
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(50, 24))
	t.Cleanup(func() { tm.Quit() })

	// Should still render the dashboard
	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	// Navigate to population - compact layout
	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")
}

func TestE2E_WideTerminal(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(200, 50))
	t.Cleanup(func() { tm.Quit() })

	waitFor(t, tm, "VAULT STATUS OVERVIEW")

	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")
}

func TestE2E_PlaceholderModules(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// Labor
	tm.Send(tea.KeyMsg{Type: tea.KeyF6})
	waitFor(t, tm, "LABOR ALLOCATION")

	// Medical
	tm.Send(tea.KeyMsg{Type: tea.KeyF7})
	waitFor(t, tm, "MEDICAL RECORDS")

	// Security
	tm.Send(tea.KeyMsg{Type: tea.KeyF8})
	waitFor(t, tm, "SECURITY")

	// Governance
	tm.Send(tea.KeyMsg{Type: tea.KeyF9})
	waitFor(t, tm, "GOVERNANCE")
}

func TestE2E_AddResidentFormOpen(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	tm.Send(tea.KeyMsg{Type: tea.KeyF3})
	waitFor(t, tm, "POPULATION CENSUS")

	// Press 'a' to open add resident form
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	waitFor(t, tm, "Surname")

	// Cancel form with Esc
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})

	// Should return to population list - verify it's still responsive
	tm.Send(tea.KeyMsg{Type: tea.KeyF2})
	waitFor(t, tm, "VAULT STATUS OVERVIEW")
}

func TestE2E_DashboardShowsVaultInfo(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// All dashboard panels should render in the same frame
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Vault 076")) &&
			bytes.Contains(bts, []byte("POPULATION")) &&
			bytes.Contains(bts, []byte("RESOURCE STATUS")) &&
			bytes.Contains(bts, []byte("SIMULATION"))
	}, teatest.WithDuration(5*time.Second))
}

func TestE2E_StatusBarShowsKeyBindings(t *testing.T) {
	tm := teatest.NewTestModel(t, newE2EApp(t),
		teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Quit() })

	// Footer key bindings should be in the rendered output
	// Note: [F10]Quit may be truncated at 120 columns, so check visible keys
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("[F1]Help")) &&
			bytes.Contains(bts, []byte("[F3]Population")) &&
			bytes.Contains(bts, []byte("[F5]Facilities"))
	}, teatest.WithDuration(5*time.Second))
}
