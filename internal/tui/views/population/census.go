// Package population provides TUI views for population management.
package population

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/services/population"
	"github.com/vtuos/vtuos/internal/tui/components"
)

// CensusView displays the resident census list.
type CensusView struct {
	service   *population.Service
	table     *components.Table
	residents []*models.Resident
	page      models.Pagination
	filter    models.ResidentFilter
	loading   bool
	err       error
	search    string
	vaultTime time.Time
}

// MaxContentWidth is the maximum width for content display
const MaxContentWidth = 120

// NewCensusView creates a new census view.
func NewCensusView(service *population.Service) *CensusView {
	// Columns sized to use ~110 chars of MaxContentWidth (leaving room for separators)
	columns := []components.Column{
		{Title: "Registry #", Width: 12},
		{Title: "Surname", Width: 15},
		{Title: "Given Names", Width: 20},
		{Title: "Age", Width: 4, Align: lipgloss.Right},
		{Title: "Sex", Width: 3},
		{Title: "Blood", Width: 5},
		{Title: "Status", Width: 12},
		{Title: "Entry", Width: 10},
		{Title: "Clr", Width: 3, Align: lipgloss.Right},
	}

	table := components.NewTable(columns)
	table.SetVisibleRows(25) // Show full page without scrolling
	table.Focus(true)

	return &CensusView{
		service: service,
		table:   table,
		page:    models.Pagination{Page: 1, PageSize: 25},
	}
}

// Load fetches residents from the database.
func (v *CensusView) Load(ctx context.Context) error {
	v.loading = true
	v.err = nil

	result, err := v.service.ListResidents(ctx, v.filter, v.page)
	if err != nil {
		v.loading = false
		v.err = err
		return err
	}

	v.residents = result.Residents
	v.loading = false

	// Convert to table rows
	rows := make([][]string, len(v.residents))
	for i, r := range v.residents {
		age := r.Age(v.vaultTime)
		blood := string(r.BloodType)
		if blood == "" {
			blood = "-"
		}
		rows[i] = []string{
			r.RegistryNumber,
			r.Surname,
			r.GivenNames,
			fmt.Sprintf("%d", age),
			string(r.Sex),
			blood,
			string(r.Status),
			string(r.EntryType),
			fmt.Sprintf("%d", r.ClearanceLevel),
		}
	}

	v.table.SetRows(rows)
	v.table.SetPagination(result.Page, result.TotalPages, result.Total)

	return nil
}

// SetVaultTime sets the current vault time for age calculation.
func (v *CensusView) SetVaultTime(t time.Time) {
	v.vaultTime = t
}

// SetSearch sets the search filter.
func (v *CensusView) SetSearch(term string) {
	v.search = term
	v.filter.SearchTerm = term
	v.page.Page = 1
}

// SetStatusFilter sets the status filter.
func (v *CensusView) SetStatusFilter(status *models.ResidentStatus) {
	v.filter.Status = status
	v.page.Page = 1
}

// NextPage moves to the next page.
func (v *CensusView) NextPage() {
	v.page.Page++
}

// PrevPage moves to the previous page.
func (v *CensusView) PrevPage() {
	if v.page.Page > 1 {
		v.page.Page--
	}
}

// MoveUp moves the selection up.
func (v *CensusView) MoveUp() {
	v.table.MoveUp()
}

// MoveDown moves the selection down.
func (v *CensusView) MoveDown() {
	v.table.MoveDown()
}

// SelectedResident returns the currently selected resident.
func (v *CensusView) SelectedResident() *models.Resident {
	idx := v.table.Selected()
	if idx >= 0 && idx < len(v.residents) {
		return v.residents[idx]
	}
	return nil
}

// Render renders the census view.
func (v *CensusView) Render(width, height int) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("=== POPULATION CENSUS ==="))
	b.WriteString("\n\n")

	// Search/filter info
	if v.search != "" {
		b.WriteString(labelStyle.Render("Search: "))
		b.WriteString(valueStyle.Render(v.search))
		b.WriteString("\n")
	}

	if v.filter.Status != nil {
		b.WriteString(labelStyle.Render("Status: "))
		b.WriteString(valueStyle.Render(string(*v.filter.Status)))
		b.WriteString("\n")
	}

	if v.search != "" || v.filter.Status != nil {
		b.WriteString("\n")
	}

	// Error display
	if v.err != nil {
		b.WriteString(errStyle.Render("Error: " + v.err.Error()))
		b.WriteString("\n\n")
	}

	// Loading indicator
	if v.loading {
		b.WriteString(labelStyle.Render("Loading..."))
		b.WriteString("\n")
	} else if v.table.Empty() {
		b.WriteString(labelStyle.Render("No residents found."))
		b.WriteString("\n")
	} else {
		// Table
		b.WriteString(v.table.Render())
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Up/Down:Select  Enter:Details  s:Search  a:Add  PgUp/Dn:Page"))

	return b.String()
}

// RenderDetail renders the detail view for the selected resident.
func (v *CensusView) RenderDetail(resident *models.Resident) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Width(18)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))

	if resident == nil {
		return labelStyle.Render("No resident selected")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("=== RESIDENT DETAILS ==="))
	b.WriteString("\n\n")

	// Identity
	b.WriteString(sectionStyle.Render("IDENTITY"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Registry #:") + " " + valueStyle.Render(resident.RegistryNumber) + "\n")
	b.WriteString(labelStyle.Render("Name:") + " " + valueStyle.Render(resident.FullName()) + "\n")
	b.WriteString(labelStyle.Render("Sex:") + " " + valueStyle.Render(resident.Sex.String()) + "\n")
	if resident.BloodType != "" {
		b.WriteString(labelStyle.Render("Blood Type:") + " " + valueStyle.Render(string(resident.BloodType)) + "\n")
	}
	b.WriteString("\n")

	// Dates
	b.WriteString(sectionStyle.Render("DATES"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Date of Birth:") + " " + valueStyle.Render(resident.DateOfBirth.Format("2006-01-02")) + "\n")
	b.WriteString(labelStyle.Render("Age:") + " " + valueStyle.Render(fmt.Sprintf("%d years", resident.Age(v.vaultTime))) + "\n")
	b.WriteString(labelStyle.Render("Entry Type:") + " " + valueStyle.Render(string(resident.EntryType)) + "\n")
	b.WriteString(labelStyle.Render("Entry Date:") + " " + valueStyle.Render(resident.EntryDate.Format("2006-01-02")) + "\n")
	if resident.DateOfDeath != nil {
		b.WriteString(labelStyle.Render("Date of Death:") + " " + valueStyle.Render(resident.DateOfDeath.Format("2006-01-02")) + "\n")
	}
	b.WriteString("\n")

	// Status
	b.WriteString(sectionStyle.Render("STATUS"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Status:") + " " + valueStyle.Render(string(resident.Status)) + "\n")
	b.WriteString(labelStyle.Render("Clearance:") + " " + valueStyle.Render(fmt.Sprintf("%d", resident.ClearanceLevel)) + "\n")
	if resident.HouseholdID != nil {
		b.WriteString(labelStyle.Render("Household:") + " " + valueStyle.Render(*resident.HouseholdID) + "\n")
	}
	b.WriteString("\n")

	// Notes
	if resident.Notes != "" {
		b.WriteString(sectionStyle.Render("NOTES"))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("") + resident.Notes)
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("Esc:Back  e:Edit  d:Death Record"))

	return b.String()
}

func formatHouseholdID(id *string) string {
	if id == nil {
		return "-"
	}
	// Truncate UUID to last 8 chars
	if len(*id) > 8 {
		return (*id)[len(*id)-8:]
	}
	return *id
}
