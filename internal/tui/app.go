package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/services/population"
	"github.com/vtuos/vtuos/internal/services/resources"
	popviews "github.com/vtuos/vtuos/internal/tui/views/population"
	resviews "github.com/vtuos/vtuos/internal/tui/views/resources"
	"github.com/vtuos/vtuos/internal/util"
)

// Version information (set at build time)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// MaxContentWidth is the maximum width for content display
const MaxContentWidth = 120

// Module represents a view module in the application.
type Module string

const (
	ModuleDashboard  Module = "dashboard"
	ModulePopulation Module = "population"
	ModuleResources  Module = "resources"
	ModuleFacilities Module = "facilities"
	ModuleLabor      Module = "labor"
	ModuleMedical    Module = "medical"
	ModuleSecurity   Module = "security"
	ModuleGovernance Module = "governance"
	ModuleSettings   Module = "settings"
	ModuleHelp       Module = "help"
)

// App is the main Bubble Tea application model.
type App struct {
	// Dependencies
	db     *database.DB
	config *config.Config
	clock  *util.VaultClock

	// Services
	populationSvc *population.Service
	resourceSvc   *resources.Service

	// Views
	censusView    *popviews.CensusView
	residentForm  *popviews.ResidentForm
	inventoryView *resviews.InventoryView

	// UI state
	theme       *Theme
	keys        KeyMap
	width       int
	height      int
	ready       bool
	quitting    bool
	showConfirm bool

	// Current view
	currentModule  Module
	previousModule Module
	showDetail     bool // Show detail view instead of list
	showForm       bool // Show add/edit form
	searchMode     bool // Search input mode
	searchInput    string

	// Alerts
	alerts []Alert

	// Population count (updated periodically)
	population int
}

// Alert represents a system alert.
type Alert struct {
	Level   AlertLevel
	Message string
	Time    time.Time
}

// AlertLevel indicates the severity of an alert.
type AlertLevel int

const (
	AlertInfo AlertLevel = iota
	AlertWarning
	AlertCritical
)

// tickMsg is sent periodically to update the UI.
type tickMsg time.Time

// New creates a new App instance.
func New(db *database.DB, cfg *config.Config, clock *util.VaultClock) *App {
	// Create population service
	popSvc := population.NewService(db.DB, cfg.Vault.Number)

	// Create resource service
	resSvc := resources.NewService(db.DB)

	// Create census view
	censusView := popviews.NewCensusView(popSvc)
	censusView.SetVaultTime(clock.Now())

	// Create inventory view
	inventoryView := resviews.NewInventoryView(resSvc)
	inventoryView.SetVaultTime(clock.Now())

	return &App{
		db:            db,
		config:        cfg,
		clock:         clock,
		populationSvc: popSvc,
		resourceSvc:   resSvc,
		censusView:    censusView,
		inventoryView: inventoryView,
		theme:         NewTheme(cfg.Display.ColorScheme),
		keys:          DefaultKeyMap(),
		currentModule: ModuleDashboard,
		alerts:        []Alert{},
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
		a.loadPopulation(),
	)
}

// tickCmd returns a command that sends tick messages.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// loadPopulation loads the population count from the database.
func (a *App) loadPopulation() tea.Cmd {
	return func() tea.Msg {
		var count int
		err := a.db.QueryRow(
			"SELECT COUNT(*) FROM residents WHERE status = 'ACTIVE'",
		).Scan(&count)
		if err != nil {
			// Table might not exist yet
			return populationMsg{count: 0}
		}
		return populationMsg{count: count}
	}
}

type populationMsg struct {
	count int
}

type censusLoadedMsg struct {
	err error
}

type inventoryLoadedMsg struct {
	err error
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		return a, nil

	case tickMsg:
		// Update vault time in views
		a.censusView.SetVaultTime(a.clock.Now())
		a.inventoryView.SetVaultTime(a.clock.Now())
		return a, tickCmd()

	case populationMsg:
		a.population = msg.count
		return a, nil

	case censusLoadedMsg:
		if msg.err != nil {
			a.AddAlert(AlertWarning, "Failed to load census: "+msg.err.Error())
		}
		return a, nil

	case inventoryLoadedMsg:
		if msg.err != nil {
			a.AddAlert(AlertWarning, "Failed to load inventory: "+msg.err.Error())
		}
		return a, nil

	case residentSavedMsg:
		a.showForm = false
		a.residentForm = nil
		if msg.err != nil {
			a.AddAlert(AlertWarning, "Failed to save resident: "+msg.err.Error())
		} else {
			a.AddAlert(AlertInfo, "Resident saved successfully")
		}
		return a, tea.Batch(a.loadCensus(), a.loadPopulation())

	case deathRegisteredMsg:
		a.showDetail = false
		if msg.err != nil {
			a.AddAlert(AlertWarning, "Failed to register death: "+msg.err.Error())
		} else {
			a.AddAlert(AlertInfo, "Death registered")
		}
		return a, tea.Batch(a.loadCensus(), a.loadPopulation())
	}

	return a, nil
}

// handleKeyPress processes key press events.
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit confirmation first (modal takes priority)
	if a.showConfirm {
		switch msg.String() {
		case "y", "Y", "enter":
			a.quitting = true
			return a, tea.Quit
		case "n", "N", "esc":
			a.showConfirm = false
			return a, nil
		}
		return a, nil
	}

	// Handle form mode BEFORE global keys - form needs all input
	if a.currentModule == ModulePopulation && a.showForm {
		return a.handleFormKeys(msg)
	}

	// Handle search mode BEFORE global keys - search needs text input
	if a.currentModule == ModulePopulation && a.searchMode {
		return a.handleSearchKeys(msg)
	}

	// Global key bindings (only when not in input mode)
	if a.keys.IsQuit(msg) {
		a.showConfirm = true
		return a, nil
	}

	// Function key navigation (always available)
	if a.keys.IsFunctionKey(msg) {
		module := a.keys.GetFunctionKeyModule(msg)
		switch module {
		case "quit":
			a.showConfirm = true
		case "help":
			a.previousModule = a.currentModule
			a.currentModule = ModuleHelp
		case "dashboard":
			a.currentModule = ModuleDashboard
			a.showDetail = false
		case "population":
			a.currentModule = ModulePopulation
			a.showDetail = false
			return a, a.loadCensus()
		case "resources":
			a.currentModule = ModuleResources
			a.showDetail = false
			return a, a.loadInventory()
		case "facilities":
			a.currentModule = ModuleFacilities
		case "labor":
			a.currentModule = ModuleLabor
		case "medical":
			a.currentModule = ModuleMedical
		case "security":
			a.currentModule = ModuleSecurity
		case "governance":
			a.currentModule = ModuleGovernance
		}
		return a, nil
	}

	// Back navigation (only when not in input mode)
	if a.keys.Back.Matches(msg) {
		if a.showDetail {
			a.showDetail = false
			return a, nil
		}
		if a.currentModule == ModuleHelp && a.previousModule != "" {
			a.currentModule = a.previousModule
			a.previousModule = ""
		}
		return a, nil
	}

	// Module-specific key handling
	if a.currentModule == ModulePopulation {
		return a.handlePopulationKeys(msg)
	}

	if a.currentModule == ModuleResources {
		return a.handleResourceKeys(msg)
	}

	return a, nil
}

// handlePopulationKeys handles key presses in the population module.
// Note: form and search modes are handled in handleKeyPress before this is called
func (a *App) handlePopulationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.showDetail {
		// In detail view
		switch msg.String() {
		case "esc":
			a.showDetail = false
		case "e":
			// Edit resident
			resident := a.censusView.SelectedResident()
			if resident != nil {
				a.residentForm = popviews.NewResidentForm(popviews.FormModeEdit)
				a.residentForm.SetResident(resident)
				a.showForm = true
				a.showDetail = false
			}
		case "d":
			// Register death - show confirmation
			resident := a.censusView.SelectedResident()
			if resident != nil && resident.IsAlive() {
				return a, a.registerDeath(resident)
			}
		}
		return a, nil
	}

	// In list view
	switch msg.String() {
	case "up", "k":
		a.censusView.MoveUp()
	case "down", "j":
		a.censusView.MoveDown()
	case "enter":
		if a.censusView.SelectedResident() != nil {
			a.showDetail = true
		}
	case "pgup":
		a.censusView.PrevPage()
		return a, a.loadCensus()
	case "pgdown":
		a.censusView.NextPage()
		return a, a.loadCensus()
	case "a":
		// Add new resident
		a.residentForm = popviews.NewResidentForm(popviews.FormModeAdd)
		a.showForm = true
	case "/", "s":
		// Enter search mode
		a.searchMode = true
		a.searchInput = ""
	}

	return a, nil
}

// handleFormKeys handles key presses in form mode.
func (a *App) handleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	a.residentForm.HandleKey(key)

	if a.residentForm.IsCancelled() {
		a.showForm = false
		a.residentForm = nil
		return a, nil
	}

	if a.residentForm.IsSubmitted() {
		return a, a.saveResident()
	}

	return a, nil
}

// handleSearchKeys handles key presses in search mode.
func (a *App) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		a.searchMode = false
		a.searchInput = ""
		a.censusView.SetSearch("")
		return a, a.loadCensus()
	case "enter":
		a.searchMode = false
		a.censusView.SetSearch(a.searchInput)
		return a, a.loadCensus()
	case "backspace":
		if len(a.searchInput) > 0 {
			a.searchInput = a.searchInput[:len(a.searchInput)-1]
		}
	default:
		if len(key) == 1 {
			a.searchInput += key
		}
	}

	return a, nil
}

type residentSavedMsg struct {
	err error
}

type deathRegisteredMsg struct {
	err error
}

// saveResident saves the resident from the form.
func (a *App) saveResident() tea.Cmd {
	return func() tea.Msg {
		resident, err := a.residentForm.GetData()
		if err != nil {
			return residentSavedMsg{err: err}
		}

		ctx := context.Background()
		if resident.ID == "" {
			// New resident - use CreateResidentInput
			input := population.CreateResidentInput{
				Surname:        resident.Surname,
				GivenNames:     resident.GivenNames,
				DateOfBirth:    resident.DateOfBirth,
				Sex:            resident.Sex,
				BloodType:      resident.BloodType,
				EntryType:      resident.EntryType,
				EntryDate:      a.clock.Now(),
				ClearanceLevel: resident.ClearanceLevel,
				Notes:          resident.Notes,
			}
			_, err = a.populationSvc.CreateResident(ctx, input)
		} else {
			// Update existing - use UpdateResidentInput
			input := population.UpdateResidentInput{
				Surname:        &resident.Surname,
				GivenNames:     &resident.GivenNames,
				BloodType:      &resident.BloodType,
				ClearanceLevel: &resident.ClearanceLevel,
				Notes:          &resident.Notes,
			}
			_, err = a.populationSvc.UpdateResident(ctx, resident.ID, input)
		}

		return residentSavedMsg{err: err}
	}
}

// registerDeath registers a death for the resident.
func (a *App) registerDeath(resident *models.Resident) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		input := population.DeathRegistration{
			DateOfDeath: a.clock.Now(),
			Cause:       "Cause pending investigation",
		}
		err := a.populationSvc.RegisterDeath(ctx, resident.ID, input)
		return deathRegisteredMsg{err: err}
	}
}

// loadCensus loads the census data.
func (a *App) loadCensus() tea.Cmd {
	return func() tea.Msg {
		err := a.censusView.Load(context.Background())
		return censusLoadedMsg{err: err}
	}
}

// handleResourceKeys handles key presses in the resources module.
func (a *App) handleResourceKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.showDetail {
		// In detail view
		switch msg.String() {
		case "esc":
			a.showDetail = false
		}
		return a, nil
	}

	// In list view
	switch msg.String() {
	case "up", "k":
		a.inventoryView.MoveUp()
	case "down", "j":
		a.inventoryView.MoveDown()
	case "enter":
		if a.inventoryView.SelectedStock() != nil {
			a.showDetail = true
		}
	case "pgup":
		a.inventoryView.PrevPage()
		return a, a.loadInventory()
	case "pgdown":
		a.inventoryView.NextPage()
		return a, a.loadInventory()
	case "c":
		// Cycle through category filter
		categories := a.inventoryView.GetCategories()
		if len(categories) > 0 {
			// Get current category
			current := a.inventoryView.SelectedStock()
			var nextCat *string
			if current == nil || current.Item == nil {
				// Start with first category
				nextCat = &categories[0].ID
			} else {
				// Find next category
				for i, cat := range categories {
					if current.Item.CategoryID == cat.ID {
						if i+1 < len(categories) {
							nextCat = &categories[i+1].ID
						}
						// If at end, nextCat stays nil (all categories)
						break
					}
				}
			}
			a.inventoryView.SetCategoryFilter(nextCat)
			return a, a.loadInventory()
		}
	}

	return a, nil
}

// loadInventory loads the inventory data.
func (a *App) loadInventory() tea.Cmd {
	return func() tea.Msg {
		err := a.inventoryView.Load(context.Background())
		return inventoryLoadedMsg{err: err}
	}
}

// View implements tea.Model.
func (a *App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	if a.quitting {
		return a.theme.Title.Render("Vault-Tec Unified Operating System shutting down...")
	}

	var b strings.Builder

	// Header
	b.WriteString(a.renderHeader())
	b.WriteString("\n")

	// Alert bar
	b.WriteString(a.renderAlertBar())
	b.WriteString("\n")

	// Main content area
	contentHeight := a.height - 6 // header, alert, footer
	if a.showConfirm {
		b.WriteString(a.renderConfirmDialog(contentHeight))
	} else {
		b.WriteString(a.renderContent(contentHeight))
	}

	// Footer/status bar
	b.WriteString("\n")
	b.WriteString(a.renderFooter())

	return b.String()
}

// renderHeader renders the top header bar.
func (a *App) renderHeader() string {
	// Left side: title and version
	title := fmt.Sprintf("VAULT-TEC UNIFIED OPERATING SYSTEM v%s", Version)

	// Right side: vault info
	vaultInfo := fmt.Sprintf("%s | POP: %d",
		a.config.Vault.Designation,
		a.population,
	)

	// Calculate spacing
	spacing := a.width - lipgloss.Width(title) - lipgloss.Width(vaultInfo) - 2
	if spacing < 1 {
		spacing = 1
	}

	header := a.theme.Header.Render(title) +
		strings.Repeat(" ", spacing) +
		a.theme.Header.Render(vaultInfo)

	// Separator line
	separator := a.theme.DrawDoubleLine(a.width)

	return header + "\n" + separator
}

// renderAlertBar renders the rotating alert display.
func (a *App) renderAlertBar() string {
	vaultTime := a.clock.Now()
	timeStr := vaultTime.Format(a.config.Display.DateFormat + " " + a.config.Display.TimeFormat)

	// Show current time and any active alerts
	var alertText string
	if len(a.alerts) > 0 {
		alert := a.alerts[0]
		switch alert.Level {
		case AlertCritical:
			alertText = a.theme.AlertCrit.Render("CRITICAL: " + alert.Message)
		case AlertWarning:
			alertText = a.theme.AlertWarn.Render("WARNING: " + alert.Message)
		default:
			alertText = a.theme.Alert.Render("INFO: " + alert.Message)
		}
	} else {
		alertText = a.theme.Muted.Render("All systems operational")
	}

	timeDisplay := a.theme.Value.Render(timeStr)
	divider := a.theme.StatusDivider.Render()

	return timeDisplay + divider + alertText
}

// renderContent renders the main content area based on current module.
func (a *App) renderContent(height int) string {
	content := a.getModuleContent()

	// Constrain content width to MaxContentWidth
	contentWidth := a.width
	if contentWidth > MaxContentWidth {
		contentWidth = MaxContentWidth
	}

	// Center the content container within the terminal
	style := lipgloss.NewStyle().
		Width(a.width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Top)

	contentStyle := lipgloss.NewStyle().
		Width(contentWidth)

	return style.Render(contentStyle.Render(content))
}

// getModuleContent returns the content for the current module.
func (a *App) getModuleContent() string {
	switch a.currentModule {
	case ModuleDashboard:
		return a.renderDashboard()
	case ModulePopulation:
		return a.renderPopulation()
	case ModuleResources:
		return a.renderResources()
	case ModuleHelp:
		return a.renderHelp()
	default:
		return a.renderPlaceholder(string(a.currentModule))
	}
}

// renderPopulation renders the population module.
func (a *App) renderPopulation() string {
	// Show form if active
	if a.showForm && a.residentForm != nil {
		return a.residentForm.Render()
	}

	// Show detail if active
	if a.showDetail {
		resident := a.censusView.SelectedResident()
		return a.censusView.RenderDetail(resident)
	}

	// Show search bar if in search mode
	var searchBar string
	if a.searchMode {
		searchBar = a.theme.Label.Render("SEARCH: ") +
			a.theme.Accent.Render(a.searchInput) +
			a.theme.Accent.Render("_") + "\n\n"
	}

	return searchBar + a.censusView.Render(a.width, a.height-6)
}

// renderResources renders the resources module.
func (a *App) renderResources() string {
	// Show detail if active
	if a.showDetail {
		stock := a.inventoryView.SelectedStock()
		return a.inventoryView.RenderDetail(stock)
	}

	return a.inventoryView.Render(a.width, a.height-6)
}

// renderDashboard renders the main dashboard view.
func (a *App) renderDashboard() string {
	var b strings.Builder

	b.WriteString(a.theme.Title.Render("═══ VAULT STATUS OVERVIEW ═══"))
	b.WriteString("\n\n")

	// Population summary
	b.WriteString(a.theme.Subtitle.Render("POPULATION"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Active Residents: %d\n", a.population))
	b.WriteString(fmt.Sprintf("  Vault Capacity:   %d\n", a.config.Vault.DesignedCapacity))
	b.WriteString("\n")

	// System status
	b.WriteString(a.theme.Subtitle.Render("CRITICAL SYSTEMS"))
	b.WriteString("\n")
	b.WriteString("  Power:        " + a.theme.Success.Render("OPERATIONAL") + "\n")
	b.WriteString("  Water:        " + a.theme.Success.Render("OPERATIONAL") + "\n")
	b.WriteString("  HVAC:         " + a.theme.Success.Render("OPERATIONAL") + "\n")
	b.WriteString("  Security:     " + a.theme.Success.Render("OPERATIONAL") + "\n")
	b.WriteString("\n")

	// Simulation status
	if a.config.Simulation.Enabled {
		status := "RUNNING"
		if a.clock.IsPaused() {
			status = "PAUSED"
		}
		b.WriteString(a.theme.Subtitle.Render("SIMULATION"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Status:     %s\n", status))
		b.WriteString(fmt.Sprintf("  Time Scale: %.0fx\n", a.clock.TimeScale()))
	}

	return b.String()
}

// renderHelp renders the help screen.
func (a *App) renderHelp() string {
	var b strings.Builder

	b.WriteString(a.theme.Title.Render("═══ HELP ═══"))
	b.WriteString("\n\n")

	b.WriteString(a.theme.Subtitle.Render("NAVIGATION"))
	b.WriteString("\n\n")

	navItems := [][2]string{
		{"F1", "Help"},
		{"F2", "Dashboard"},
		{"F3", "Population Registry"},
		{"F4", "Resource Management"},
		{"F5", "Facility Operations"},
		{"F6", "Labor Allocation"},
		{"F7", "Medical Records"},
		{"F8", "Security"},
		{"F9", "Governance"},
		{"F10", "Quit"},
	}

	for _, item := range navItems {
		line := fmt.Sprintf("    %-8s  %s", item[0], item[1])
		b.WriteString(a.theme.Primary.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Subtitle.Render("CONTROLS"))
	b.WriteString("\n\n")

	ctrlItems := [][2]string{
		{"Up/Down", "Navigate"},
		{"Enter", "Select"},
		{"Esc", "Back/Cancel"},
		{"/", "Search"},
		{"Tab", "Next field"},
		{"PgUp/Dn", "Page navigation"},
	}

	for _, item := range ctrlItems {
		line := fmt.Sprintf("    %-8s  %s", item[0], item[1])
		b.WriteString(a.theme.Primary.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Muted.Render("Press Esc to return"))

	return b.String()
}

// renderPlaceholder renders a placeholder for unimplemented modules.
func (a *App) renderPlaceholder(name string) string {
	var b strings.Builder

	title := fmt.Sprintf("═══ %s ═══", strings.ToUpper(name))
	b.WriteString(a.theme.Title.Render(title))
	b.WriteString("\n\n")

	b.WriteString(a.theme.Muted.Render("This module is not yet implemented."))
	b.WriteString("\n\n")

	b.WriteString(a.theme.Label.Render("Press F2 to return to Dashboard"))

	return b.String()
}

// renderConfirmDialog renders the quit confirmation dialog.
func (a *App) renderConfirmDialog(height int) string {
	dialog := a.theme.Box.Render(
		a.theme.Title.Render("CONFIRM EXIT") + "\n\n" +
			a.theme.Base.Render("Are you sure you want to exit?") + "\n\n" +
			a.theme.Label.Render("[Y]es  [N]o"),
	)

	// Center the dialog
	style := lipgloss.NewStyle().
		Width(a.width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(dialog)
}

// renderFooter renders the bottom status bar.
func (a *App) renderFooter() string {
	// Draw separator
	separator := a.theme.DrawHorizontalLine(a.width)

	// Help text
	help := a.keys.StatusBarHelp()

	return separator + "\n" + a.theme.Footer.Render(help)
}

// AddAlert adds a new alert to the display.
func (a *App) AddAlert(level AlertLevel, message string) {
	a.alerts = append([]Alert{{
		Level:   level,
		Message: message,
		Time:    time.Now(),
	}}, a.alerts...)

	// Keep only last 10 alerts
	if len(a.alerts) > 10 {
		a.alerts = a.alerts[:10]
	}
}

// ClearAlerts removes all alerts.
func (a *App) ClearAlerts() {
	a.alerts = []Alert{}
}

// Run starts the TUI application.
func Run(ctx context.Context, db *database.DB, cfg *config.Config, clock *util.VaultClock) error {
	app := New(db, cfg, clock)

	p := tea.NewProgram(app, tea.WithAltScreen())

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		p.Quit()
	}()

	_, err := p.Run()
	return err
}
