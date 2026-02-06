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

// chromeLines is the number of terminal lines reserved for header, alert, footer, separators.
const chromeLines = 6

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
	alerts     []Alert
	alertIndex int
	alertTick  int

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
		// Update visible rows in views based on new height
		a.updateViewDimensions()
		return a, nil

	case tickMsg:
		// Update vault time in views
		a.censusView.SetVaultTime(a.clock.Now())
		a.inventoryView.SetVaultTime(a.clock.Now())
		// Rotate alerts every 3 ticks
		a.alertTick++
		if a.alertTick >= 3 && len(a.alerts) > 1 {
			a.alertTick = 0
			a.alertIndex = (a.alertIndex + 1) % len(a.alerts)
		}
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

// updateViewDimensions recalculates visible rows for all views based on terminal height.
func (a *App) updateViewDimensions() {
	contentH := ContentHeight(a.height, chromeLines)
	// Census table: subtract 4 lines for title, search info, separator, help line
	censusRows := contentH - 6
	if censusRows < 5 {
		censusRows = 5
	}
	a.censusView.SetVisibleRows(censusRows)

	// Inventory table: subtract 4 lines for title, filter info, separator, help line
	invRows := contentH - 6
	if invRows < 5 {
		invRows = 5
	}
	a.inventoryView.SetVisibleRows(invRows)
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
		return "Initializing VT-UOS..."
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
	contentHeight := ContentHeight(a.height, chromeLines)
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

// renderHeader renders the top header bar, responsive to terminal width.
func (a *App) renderHeader() string {
	w := a.width
	if w < 20 {
		w = 20
	}

	// Left side: title
	title := "VAULT-TEC UNIFIED OPERATING SYSTEM"
	versionStr := fmt.Sprintf("v%s", Version)

	// Right side: vault info
	vaultInfo := fmt.Sprintf("%s │ POP: %d",
		a.config.Vault.Designation,
		a.population,
	)

	bp := GetBreakpoint(w)
	switch bp {
	case BreakpointNarrow:
		// Compact: just vault designation and population
		title = "VT-UOS"
		vaultInfo = fmt.Sprintf("POP:%d", a.population)
	case BreakpointMedium:
		title = "VT-UOS " + versionStr
	default:
		title = title + " " + versionStr
	}

	titleRendered := a.theme.Header.Render(title)
	infoRendered := a.theme.Header.Render(vaultInfo)
	titleWidth := lipgloss.Width(titleRendered)
	infoWidth := lipgloss.Width(infoRendered)

	spacing := w - titleWidth - infoWidth
	if spacing < 1 {
		spacing = 1
	}

	header := titleRendered + strings.Repeat(" ", spacing) + infoRendered

	// Separator line
	separator := a.theme.DrawDoubleLine(w)

	return header + "\n" + separator
}

// renderAlertBar renders the rotating alert display.
func (a *App) renderAlertBar() string {
	w := a.width
	vaultTime := a.clock.Now()

	// Time display adapts to width
	var timeStr string
	bp := GetBreakpoint(w)
	switch bp {
	case BreakpointNarrow:
		timeStr = vaultTime.Format(a.config.Display.TimeFormat)
	default:
		timeStr = vaultTime.Format(a.config.Display.DateFormat + " " + a.config.Display.TimeFormat)
	}

	// Show current time and any active alerts
	var alertText string
	if len(a.alerts) > 0 {
		idx := a.alertIndex % len(a.alerts)
		alert := a.alerts[idx]
		switch alert.Level {
		case AlertCritical:
			alertText = a.theme.AlertCrit.Render("CRITICAL: " + alert.Message)
		case AlertWarning:
			alertText = a.theme.AlertWarn.Render("WARNING: " + alert.Message)
		default:
			alertText = a.theme.Alert.Render("INFO: " + alert.Message)
		}
		// Truncate alert to fit
		maxAlertWidth := w - lipgloss.Width(timeStr) - 5
		if maxAlertWidth > 0 && lipgloss.Width(alertText) > maxAlertWidth {
			alertText = Truncate(alertText, maxAlertWidth)
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

	// Use full terminal width (content views handle their own width constraints)
	style := lipgloss.NewStyle().
		Width(a.width).
		Height(height).
		MaxHeight(height)

	return style.Render(content)
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
	case ModuleFacilities:
		return a.renderFacilities()
	case ModuleLabor:
		return a.renderLabor()
	case ModuleMedical:
		return a.renderMedical()
	case ModuleSecurity:
		return a.renderSecurity()
	case ModuleGovernance:
		return a.renderGovernance()
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
		return a.censusView.RenderDetail(resident, a.width)
	}

	// Show search bar if in search mode
	var searchBar string
	if a.searchMode {
		searchBar = a.theme.Label.Render("SEARCH: ") +
			a.theme.Accent.Render(a.searchInput) +
			a.theme.Accent.Render("_") + "\n\n"
	}

	return searchBar + a.censusView.Render(a.width, a.height-chromeLines)
}

// renderResources renders the resources module.
func (a *App) renderResources() string {
	// Show detail if active
	if a.showDetail {
		stock := a.inventoryView.SelectedStock()
		return a.inventoryView.RenderDetail(stock, a.width)
	}

	return a.inventoryView.Render(a.width, a.height-chromeLines)
}

// renderDashboard renders the main dashboard view with responsive panels.
func (a *App) renderDashboard() string {
	w := a.width
	if w < 40 {
		w = 40
	}

	var b strings.Builder

	// Title
	b.WriteString(a.theme.Title.Render("═══ VAULT STATUS OVERVIEW ═══"))
	b.WriteString("\n\n")

	bp := GetBreakpoint(w)

	// Build panels
	popPanel := a.renderPopulationPanel(w, bp)
	sysPanel := a.renderSystemsPanel(w, bp)
	resPanel := a.renderResourcesPanel(w, bp)
	simPanel := a.renderSimulationPanel(w, bp)

	switch bp {
	case BreakpointNarrow:
		// Stack all panels vertically
		b.WriteString(popPanel)
		b.WriteString("\n")
		b.WriteString(sysPanel)
		b.WriteString("\n")
		b.WriteString(resPanel)
		b.WriteString("\n")
		b.WriteString(simPanel)
	default:
		// Side-by-side: Population + Systems, then Resources + Simulation
		halfWidth := w / 2
		b.WriteString(renderSideBySide(popPanel, sysPanel, halfWidth, w))
		b.WriteString("\n")
		b.WriteString(renderSideBySide(resPanel, simPanel, halfWidth, w))
	}

	return b.String()
}

// renderPopulationPanel renders the population status panel for the dashboard.
func (a *App) renderPopulationPanel(totalWidth int, bp LayoutBreakpoint) string {
	var b strings.Builder
	b.WriteString(a.theme.Subtitle.Render("POPULATION"))
	b.WriteString("\n")

	capacity := a.config.Vault.DesignedCapacity
	ratio := float64(a.population) / float64(capacity)

	b.WriteString(fmt.Sprintf("  Active:   %s\n", a.theme.Value.Render(fmt.Sprintf("%d", a.population))))
	b.WriteString(fmt.Sprintf("  Capacity: %s\n", a.theme.Muted.Render(fmt.Sprintf("%d", capacity))))

	// Population bar
	barWidth := totalWidth/2 - 4
	if bp == BreakpointNarrow {
		barWidth = totalWidth - 4
	}
	if barWidth > 40 {
		barWidth = 40
	}
	if barWidth < 10 {
		barWidth = 10
	}
	b.WriteString("  ")
	b.WriteString(a.theme.ProgressBar(float64(a.population), float64(capacity), barWidth))
	pctStr := fmt.Sprintf(" %.0f%%", ratio*100)
	b.WriteString(a.theme.Muted.Render(pctStr))
	b.WriteString("\n")

	return b.String()
}

// renderSystemsPanel renders critical systems status for the dashboard.
func (a *App) renderSystemsPanel(totalWidth int, bp LayoutBreakpoint) string {
	var b strings.Builder
	b.WriteString(a.theme.Subtitle.Render("CRITICAL SYSTEMS"))
	b.WriteString("\n")

	systems := []struct {
		name   string
		status string
		pct    float64
	}{
		{"Power", "OPERATIONAL", 0.98},
		{"Water", "OPERATIONAL", 0.95},
		{"HVAC", "OPERATIONAL", 0.92},
		{"Security", "OPERATIONAL", 1.0},
	}

	barWidth := 16
	if bp == BreakpointNarrow {
		barWidth = 10
	}

	for _, sys := range systems {
		statusStyle := a.theme.Success
		if sys.pct < 0.8 {
			statusStyle = a.theme.Warning
		}
		if sys.pct < 0.5 {
			statusStyle = a.theme.Error
		}

		line := fmt.Sprintf("  %-10s", sys.name)
		b.WriteString(a.theme.Base.Render(line))
		b.WriteString(a.theme.ProgressBar(sys.pct, 1.0, barWidth))
		b.WriteString(" ")
		b.WriteString(statusStyle.Render(sys.status))
		b.WriteString("\n")
	}

	return b.String()
}

// renderResourcesPanel renders resource status for the dashboard.
func (a *App) renderResourcesPanel(totalWidth int, bp LayoutBreakpoint) string {
	var b strings.Builder
	b.WriteString(a.theme.Subtitle.Render("RESOURCE STATUS"))
	b.WriteString("\n")

	// Placeholder resource data (would come from service in production)
	resourceStats := []struct {
		name    string
		pct     float64
		runway  int
	}{
		{"Food", 0.72, 180},
		{"Water", 0.85, 240},
		{"Medical", 0.60, 120},
		{"Power", 0.90, 365},
	}

	barWidth := 16
	if bp == BreakpointNarrow {
		barWidth = 10
	}

	for _, res := range resourceStats {
		line := fmt.Sprintf("  %-10s", res.name)
		b.WriteString(a.theme.Base.Render(line))
		b.WriteString(a.theme.ProgressBar(res.pct, 1.0, barWidth))
		runway := fmt.Sprintf(" %dd", res.runway)
		b.WriteString(a.theme.Muted.Render(runway))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSimulationPanel renders simulation status for the dashboard.
func (a *App) renderSimulationPanel(totalWidth int, bp LayoutBreakpoint) string {
	var b strings.Builder
	b.WriteString(a.theme.Subtitle.Render("SIMULATION"))
	b.WriteString("\n")

	if !a.config.Simulation.Enabled {
		b.WriteString(a.theme.Muted.Render("  Simulation disabled"))
		b.WriteString("\n")
		return b.String()
	}

	status := "RUNNING"
	statusStyle := a.theme.Success
	if a.clock.IsPaused() {
		status = "PAUSED"
		statusStyle = a.theme.Warning
	}

	vaultTime := a.clock.Now()
	sealDate, err := a.config.Simulation.StartDateTime()
	var years, days int
	if err == nil {
		elapsed := vaultTime.Sub(sealDate)
		years = int(elapsed.Hours() / 8760)
		days = int(elapsed.Hours()/24) % 365
	}

	b.WriteString(fmt.Sprintf("  Status:     %s\n", statusStyle.Render(status)))
	b.WriteString(fmt.Sprintf("  Time Scale: %s\n", a.theme.Value.Render(fmt.Sprintf("%.0fx", a.clock.TimeScale()))))
	b.WriteString(fmt.Sprintf("  Vault Time: %s\n", a.theme.Value.Render(vaultTime.Format("2006-01-02 15:04"))))
	b.WriteString(fmt.Sprintf("  Elapsed:    %s\n", a.theme.Value.Render(fmt.Sprintf("%d years, %d days", years, days))))

	return b.String()
}

// renderSideBySide renders two panels side by side, falling back to vertical stack.
func renderSideBySide(left, right string, halfWidth, totalWidth int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	// Check if both fit
	maxLeftWidth := 0
	for _, l := range leftLines {
		w := lipgloss.Width(l)
		if w > maxLeftWidth {
			maxLeftWidth = w
		}
	}

	if maxLeftWidth+2 > halfWidth {
		// Stack vertically
		return left + "\n" + right
	}

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}

		lw := lipgloss.Width(l)
		pad := halfWidth - lw
		if pad < 1 {
			pad = 1
		}

		b.WriteString(l)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(r)
		if i < maxLines-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderFacilities renders the facilities module placeholder with structure.
func (a *App) renderFacilities() string {
	w := a.width

	var b strings.Builder
	b.WriteString(a.theme.Title.Render("═══ FACILITY OPERATIONS ═══"))
	b.WriteString("\n\n")

	systems := []struct {
		code       string
		name       string
		category   string
		status     string
		efficiency float64
	}{
		{"PWR-REACTOR-01", "Primary Reactor", "POWER", "OPERATIONAL", 0.98},
		{"PWR-GEN-01", "Backup Generator A", "POWER", "STANDBY", 1.00},
		{"WTR-PURIF-01", "Water Purification", "WATER", "OPERATIONAL", 0.95},
		{"WTR-RECYCLE-01", "Water Recycler", "WATER", "OPERATIONAL", 0.88},
		{"HVC-FILT-01", "Air Filtration", "HVAC", "OPERATIONAL", 0.92},
		{"HVC-TEMP-01", "Climate Control", "HVAC", "OPERATIONAL", 0.94},
		{"WST-PROC-01", "Waste Processing", "WASTE", "DEGRADED", 0.72},
		{"SEC-DOOR-MAIN", "Vault Door", "SECURITY", "SEALED", 1.00},
		{"MED-EQUIP-01", "Medical Bay", "MEDICAL", "OPERATIONAL", 0.97},
		{"FPR-HYDRO-01", "Hydroponics Bay A", "FOOD_PROD", "OPERATIONAL", 0.85},
		{"COM-TERM-01", "Terminal Network", "COMMS", "OPERATIONAL", 0.99},
	}

	bp := GetBreakpoint(w)
	barWidth := 12
	if bp == BreakpointNarrow {
		barWidth = 8
	}

	nameWidth := 22
	catWidth := 10
	if bp == BreakpointNarrow {
		nameWidth = 15
		catWidth = 0 // hide category on narrow
	}

	for _, sys := range systems {
		statusStyle := a.theme.Success
		switch sys.status {
		case "DEGRADED":
			statusStyle = a.theme.Warning
		case "OFFLINE", "FAILED":
			statusStyle = a.theme.Error
		case "STANDBY":
			statusStyle = a.theme.Muted
		case "SEALED":
			statusStyle = a.theme.Accent
		}

		name := Truncate(sys.name, nameWidth)
		line := fmt.Sprintf("  %-*s", nameWidth, name)
		b.WriteString(a.theme.Base.Render(line))
		if catWidth > 0 {
			b.WriteString(a.theme.Muted.Render(fmt.Sprintf(" %-*s", catWidth, sys.category)))
		}
		b.WriteString(" ")
		b.WriteString(a.theme.ProgressBar(sys.efficiency, 1.0, barWidth))
		pctStr := fmt.Sprintf(" %3.0f%%", sys.efficiency*100)
		b.WriteString(a.theme.Muted.Render(pctStr))
		b.WriteString(" ")
		b.WriteString(statusStyle.Render(sys.status))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Muted.Render("  Facility management module — monitoring mode"))

	return b.String()
}

// renderLabor renders the labor module placeholder with structure.
func (a *App) renderLabor() string {
	var b strings.Builder
	b.WriteString(a.theme.Title.Render("═══ LABOR ALLOCATION ═══"))
	b.WriteString("\n\n")

	shifts := []struct {
		name     string
		hours    string
		assigned int
		capacity int
	}{
		{"ALPHA", "0600-1400", 165, 180},
		{"BETA", "1400-2200", 152, 180},
		{"GAMMA", "2200-0600", 48, 60},
	}

	bp := GetBreakpoint(a.width)
	barWidth := 20
	if bp == BreakpointNarrow {
		barWidth = 12
	}

	b.WriteString(a.theme.Subtitle.Render("SHIFT ROSTER"))
	b.WriteString("\n")
	for _, shift := range shifts {
		ratio := float64(shift.assigned) / float64(shift.capacity)
		b.WriteString(fmt.Sprintf("  %-8s", shift.name))
		b.WriteString(a.theme.Muted.Render(fmt.Sprintf("%-12s", shift.hours)))
		b.WriteString(a.theme.ProgressBar(ratio, 1.0, barWidth))
		b.WriteString(a.theme.Value.Render(fmt.Sprintf(" %d/%d", shift.assigned, shift.capacity)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Subtitle.Render("DEPARTMENT STAFFING"))
	b.WriteString("\n")

	depts := []struct {
		name     string
		filled   int
		required int
	}{
		{"Engineering", 45, 50},
		{"Security", 30, 35},
		{"Medical", 20, 22},
		{"Hydroponics", 35, 40},
		{"Maintenance", 25, 30},
		{"Administration", 15, 15},
		{"Education", 10, 12},
		{"Science", 12, 15},
	}

	for _, dept := range depts {
		ratio := float64(dept.filled) / float64(dept.required)
		statusStyle := a.theme.Success
		if ratio < 0.9 {
			statusStyle = a.theme.Warning
		}
		if ratio < 0.7 {
			statusStyle = a.theme.Error
		}
		vacancy := dept.required - dept.filled

		b.WriteString(fmt.Sprintf("  %-16s", dept.name))
		b.WriteString(a.theme.ProgressBar(ratio, 1.0, barWidth))
		b.WriteString(statusStyle.Render(fmt.Sprintf(" %d/%d", dept.filled, dept.required)))
		if vacancy > 0 {
			b.WriteString(a.theme.Warning.Render(fmt.Sprintf(" (%d vacant)", vacancy)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Muted.Render("  Labor allocation module — monitoring mode"))

	return b.String()
}

// renderMedical renders the medical module placeholder with structure.
func (a *App) renderMedical() string {
	var b strings.Builder
	b.WriteString(a.theme.Title.Render("═══ MEDICAL RECORDS ═══"))
	b.WriteString("\n\n")

	b.WriteString(a.theme.Subtitle.Render("VAULT HEALTH SUMMARY"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Healthy:         %s\n", a.theme.Success.Render("467")))
	b.WriteString(fmt.Sprintf("  Minor Ailments:  %s\n", a.theme.Warning.Render("23")))
	b.WriteString(fmt.Sprintf("  Serious:         %s\n", a.theme.Error.Render("8")))
	b.WriteString(fmt.Sprintf("  Quarantine:      %s\n", a.theme.AlertCrit.Render("2")))
	b.WriteString("\n")

	b.WriteString(a.theme.Subtitle.Render("RADIATION LEVELS"))
	b.WriteString("\n")

	bp := GetBreakpoint(a.width)
	barWidth := 20
	if bp == BreakpointNarrow {
		barWidth = 12
	}

	zones := []struct {
		name string
		msv  float64
		max  float64
	}{
		{"Reactor Level", 0.8, 5.0},
		{"Residential", 0.05, 5.0},
		{"Hydroponics", 0.02, 5.0},
		{"Vault Door", 1.2, 5.0},
	}

	for _, zone := range zones {
		b.WriteString(fmt.Sprintf("  %-16s", zone.name))
		b.WriteString(a.theme.ProgressBar(zone.msv, zone.max, barWidth))
		b.WriteString(a.theme.Value.Render(fmt.Sprintf(" %.2f mSv", zone.msv)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Subtitle.Render("RECENT ENCOUNTERS"))
	b.WriteString("\n")
	b.WriteString(a.theme.Base.Render("  No recent medical encounters recorded.\n"))

	b.WriteString("\n")
	b.WriteString(a.theme.Muted.Render("  Medical records module — monitoring mode"))

	return b.String()
}

// renderSecurity renders the security module placeholder with structure.
func (a *App) renderSecurity() string {
	var b strings.Builder
	b.WriteString(a.theme.Title.Render("═══ SECURITY ═══"))
	b.WriteString("\n\n")

	b.WriteString(a.theme.Subtitle.Render("SECURITY ZONES"))
	b.WriteString("\n")

	zones := []struct {
		code      string
		name      string
		clearance int
		status    string
	}{
		{"ZONE-A", "Command Center", 8, "SECURE"},
		{"ZONE-B", "Residential", 1, "SECURE"},
		{"ZONE-C", "Engineering", 4, "SECURE"},
		{"ZONE-D", "Armory", 7, "LOCKED"},
		{"ZONE-E", "Vault Door", 10, "SEALED"},
		{"ZONE-F", "Reactor", 6, "RESTRICTED"},
	}

	for _, zone := range zones {
		statusStyle := a.theme.Success
		switch zone.status {
		case "LOCKED":
			statusStyle = a.theme.Warning
		case "SEALED":
			statusStyle = a.theme.Accent
		case "RESTRICTED":
			statusStyle = a.theme.Warning
		case "BREACH":
			statusStyle = a.theme.Error
		}

		b.WriteString(fmt.Sprintf("  %-8s %-18s CLR:%d  %s\n",
			zone.code,
			zone.name,
			zone.clearance,
			statusStyle.Render(zone.status)))
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Subtitle.Render("INCIDENT LOG"))
	b.WriteString("\n")
	b.WriteString(a.theme.Base.Render("  No active security incidents.\n"))

	b.WriteString("\n")
	b.WriteString(a.theme.Muted.Render("  Security module — monitoring mode"))

	return b.String()
}

// renderGovernance renders the governance module placeholder with structure.
func (a *App) renderGovernance() string {
	var b strings.Builder
	b.WriteString(a.theme.Title.Render("═══ GOVERNANCE ═══"))
	b.WriteString("\n\n")

	b.WriteString(a.theme.Subtitle.Render("ACTIVE DIRECTIVES"))
	b.WriteString("\n")

	directives := []struct {
		number string
		title  string
		level  string
		status string
	}{
		{"OD-2077-001", "Vault Sealing Protocol", "OVERSEER", "ACTIVE"},
		{"OD-2077-002", "Resource Rationing Standard", "OVERSEER", "ACTIVE"},
		{"OD-2077-003", "Emergency Power Protocol", "DEPT_HEAD", "ACTIVE"},
		{"OD-2077-004", "Population Census Schedule", "OVERSEER", "ACTIVE"},
		{"OD-2077-005", "Work Assignment Policy", "DEPT_HEAD", "ACTIVE"},
	}

	for _, d := range directives {
		statusStyle := a.theme.Success
		if d.status != "ACTIVE" {
			statusStyle = a.theme.Muted
		}
		b.WriteString(fmt.Sprintf("  %-14s %-28s %-10s %s\n",
			a.theme.Value.Render(d.number),
			d.title,
			a.theme.Muted.Render(d.level),
			statusStyle.Render(d.status)))
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Subtitle.Render("AUDIT LOG"))
	b.WriteString("\n")
	b.WriteString(a.theme.Base.Render("  System initialized. Awaiting overseer input.\n"))

	b.WriteString("\n")
	b.WriteString(a.theme.Muted.Render("  Governance module — monitoring mode"))

	return b.String()
}

// renderHelp renders the help screen, responsive to terminal width.
func (a *App) renderHelp() string {
	w := a.width
	bp := GetBreakpoint(w)

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

	// On wider terminals, render in two columns
	if bp == BreakpointWide && len(navItems) > 5 {
		half := (len(navItems) + 1) / 2
		for i := 0; i < half; i++ {
			left := fmt.Sprintf("    %-8s  %-24s", navItems[i][0], navItems[i][1])
			b.WriteString(a.theme.Primary.Render(left))
			if i+half < len(navItems) {
				right := fmt.Sprintf("    %-8s  %s", navItems[i+half][0], navItems[i+half][1])
				b.WriteString(a.theme.Primary.Render(right))
			}
			b.WriteString("\n")
		}
	} else {
		for _, item := range navItems {
			line := fmt.Sprintf("    %-8s  %s", item[0], item[1])
			b.WriteString(a.theme.Primary.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(a.theme.Subtitle.Render("CONTROLS"))
	b.WriteString("\n\n")

	ctrlItems := [][2]string{
		{"Up/Down", "Navigate lists"},
		{"Enter", "Select / Confirm"},
		{"Esc", "Back / Cancel"},
		{"/", "Search in lists"},
		{"Tab", "Next field in forms"},
		{"PgUp/Dn", "Page navigation"},
		{"a", "Add new record"},
		{"e", "Edit selected"},
		{"d", "Delete / Death record"},
		{"c", "Cycle category filter"},
	}

	if bp == BreakpointWide && len(ctrlItems) > 5 {
		half := (len(ctrlItems) + 1) / 2
		for i := 0; i < half; i++ {
			left := fmt.Sprintf("    %-10s  %-22s", ctrlItems[i][0], ctrlItems[i][1])
			b.WriteString(a.theme.Primary.Render(left))
			if i+half < len(ctrlItems) {
				right := fmt.Sprintf("    %-10s  %s", ctrlItems[i+half][0], ctrlItems[i+half][1])
				b.WriteString(a.theme.Primary.Render(right))
			}
			b.WriteString("\n")
		}
	} else {
		for _, item := range ctrlItems {
			line := fmt.Sprintf("    %-10s  %s", item[0], item[1])
			b.WriteString(a.theme.Primary.Render(line))
			b.WriteString("\n")
		}
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

// renderFooter renders the bottom status bar, responsive to terminal width.
func (a *App) renderFooter() string {
	// Draw separator
	separator := a.theme.DrawHorizontalLine(a.width)

	// Help text adapts to width
	help := a.keys.StatusBarHelpResponsive(a.width)

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

	// Reset alert rotation to show new alert
	a.alertIndex = 0
}

// ClearAlerts removes all alerts.
func (a *App) ClearAlerts() {
	a.alerts = []Alert{}
	a.alertIndex = 0
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
