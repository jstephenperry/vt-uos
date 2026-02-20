// Package facilities provides TUI views for facility management.
package facilities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/services/facilities"
	"github.com/vtuos/vtuos/internal/tui/components"
)

// SystemsView displays the facility systems list.
type SystemsView struct {
	service   *facilities.Service
	table     *components.Table
	systems   []*models.FacilitySystem
	page      models.Pagination
	filter    models.FacilitySystemFilter
	loading   bool
	err       error
	vaultTime time.Time
}

// NewSystemsView creates a new systems view.
func NewSystemsView(service *facilities.Service) *SystemsView {
	columns := []components.Column{
		{Title: "Code", Width: 16, Weight: 0, Priority: 10},
		{Title: "Name", Width: 20, Weight: 2.0, Priority: 9},
		{Title: "Category", Width: 12, Weight: 0, Priority: 5},
		{Title: "Status", Width: 12, Weight: 0, Priority: 8},
		{Title: "Efficiency", Width: 10, Align: lipgloss.Right, Priority: 7},
		{Title: "Sector", Width: 6, Priority: 4},
		{Title: "Level", Width: 5, Align: lipgloss.Right, Priority: 3},
		{Title: "Maint Due", Width: 10, Priority: 6},
	}

	table := components.NewTable(columns)
	table.SetVisibleRows(25)
	table.Focus(true)

	return &SystemsView{
		service: service,
		table:   table,
		page:    models.Pagination{Page: 1, PageSize: 25},
	}
}

// Load fetches facility systems from the database.
func (v *SystemsView) Load(ctx context.Context) error {
	v.loading = true
	v.err = nil

	result, err := v.service.ListSystems(ctx, v.filter, v.page)
	if err != nil {
		v.loading = false
		v.err = err
		return err
	}

	v.systems = result.Systems
	v.loading = false

	rows := make([][]string, len(v.systems))
	for i, s := range v.systems {
		maintDue := "-"
		if s.NextMaintenanceDue != nil {
			maintDue = s.NextMaintenanceDue.Format("2006-01-02")
			if s.IsOverdueForMaintenance(v.vaultTime) {
				maintDue = "OVERDUE"
			}
		}
		rows[i] = []string{
			s.SystemCode,
			s.Name,
			string(s.Category),
			string(s.Status),
			fmt.Sprintf("%.0f%%", s.EfficiencyPercent),
			s.LocationSector,
			fmt.Sprintf("%d", s.LocationLevel),
			maintDue,
		}
	}

	v.table.SetRows(rows)
	v.table.SetPagination(result.Page, result.TotalPages, result.Total)

	return nil
}

// SetVaultTime sets the current vault time for display.
func (v *SystemsView) SetVaultTime(t time.Time) {
	v.vaultTime = t
}

// SetVisibleRows sets the number of visible table rows.
func (v *SystemsView) SetVisibleRows(n int) {
	v.table.SetVisibleRows(n)
}

// SetCategoryFilter sets the category filter.
func (v *SystemsView) SetCategoryFilter(category *models.SystemCategory) {
	v.filter.Category = category
	v.page.Page = 1
}

// GetCategoryFilter returns the current category filter.
func (v *SystemsView) GetCategoryFilter() *models.SystemCategory {
	return v.filter.Category
}

// NextPage moves to the next page.
func (v *SystemsView) NextPage() {
	v.page.Page++
}

// PrevPage moves to the previous page.
func (v *SystemsView) PrevPage() {
	if v.page.Page > 1 {
		v.page.Page--
	}
}

// MoveUp moves the selection up.
func (v *SystemsView) MoveUp() {
	v.table.MoveUp()
}

// MoveDown moves the selection down.
func (v *SystemsView) MoveDown() {
	v.table.MoveDown()
}

// SelectedSystem returns the currently selected system.
func (v *SystemsView) SelectedSystem() *models.FacilitySystem {
	idx := v.table.Selected()
	if idx >= 0 && idx < len(v.systems) {
		return v.systems[idx]
	}
	return nil
}

// Render renders the systems view, responsive to the given terminal dimensions.
func (v *SystemsView) Render(width, height int) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))

	var b strings.Builder

	b.WriteString(titleStyle.Render("═══ FACILITY SYSTEMS ═══"))
	b.WriteString("\n\n")

	// Category filter info
	if v.filter.Category != nil {
		b.WriteString(labelStyle.Render("Category: "))
		b.WriteString(valueStyle.Render(string(*v.filter.Category)))
		b.WriteString("\n\n")
	}

	if v.err != nil {
		b.WriteString(errStyle.Render("Error: " + v.err.Error()))
		b.WriteString("\n\n")
	}

	if v.loading {
		b.WriteString(labelStyle.Render("Loading..."))
		b.WriteString("\n")
	} else if v.table.Empty() {
		b.WriteString(labelStyle.Render("No facility systems found."))
		b.WriteString("\n")
	} else {
		b.WriteString(v.table.RenderResponsive(width))
	}

	b.WriteString("\n")
	if width < 60 {
		b.WriteString(helpStyle.Render("↑↓:Nav  Enter:Details  c:Category"))
	} else {
		b.WriteString(helpStyle.Render("Up/Down:Select  Enter:Details  c:Category  PgUp/Dn:Page"))
	}

	return b.String()
}

// RenderDetail renders the detail view for the selected system.
func (v *SystemsView) RenderDetail(system *models.FacilitySystem, width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))

	labelWidth := 20
	if width < 60 {
		labelWidth = 16
	}
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Width(labelWidth)

	if system == nil {
		return labelStyle.Render("No system selected")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("═══ SYSTEM DETAILS ═══"))
	b.WriteString("\n\n")

	// Identification
	b.WriteString(sectionStyle.Render("IDENTIFICATION"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("System Code:") + " " + valueStyle.Render(system.SystemCode) + "\n")
	b.WriteString(labelStyle.Render("Name:") + " " + valueStyle.Render(system.Name) + "\n")
	b.WriteString(labelStyle.Render("Category:") + " " + valueStyle.Render(string(system.Category)) + "\n")
	b.WriteString(labelStyle.Render("Location:") + " " + valueStyle.Render(fmt.Sprintf("Sector %s, Level %d", system.LocationSector, system.LocationLevel)) + "\n")
	b.WriteString("\n")

	// Status
	b.WriteString(sectionStyle.Render("STATUS"))
	b.WriteString("\n")

	statusStyle := valueStyle
	switch system.Status {
	case models.SystemStatusDegraded:
		statusStyle = warnStyle
	case models.SystemStatusFailed, models.SystemStatusDestroyed:
		statusStyle = errStyle
	case models.SystemStatusOffline, models.SystemStatusMaintenance:
		statusStyle = warnStyle
	}
	b.WriteString(labelStyle.Render("Status:") + " " + statusStyle.Render(string(system.Status)) + "\n")

	effStyle := valueStyle
	if system.EfficiencyPercent < 80 {
		effStyle = warnStyle
	}
	if system.EfficiencyPercent < 50 {
		effStyle = errStyle
	}
	b.WriteString(labelStyle.Render("Efficiency:") + " " + effStyle.Render(fmt.Sprintf("%.1f%%", system.EfficiencyPercent)) + "\n")

	if system.CurrentOutput != nil && system.CapacityRating != nil {
		outputStr := fmt.Sprintf("%.1f / %.1f %s", *system.CurrentOutput, *system.CapacityRating, system.CapacityUnit)
		b.WriteString(labelStyle.Render("Output:") + " " + valueStyle.Render(outputStr) + "\n")
	}
	b.WriteString(labelStyle.Render("Runtime:") + " " + valueStyle.Render(fmt.Sprintf("%.0f hours", system.TotalRuntimeHours)) + "\n")
	b.WriteString("\n")

	// Maintenance
	b.WriteString(sectionStyle.Render("MAINTENANCE"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Install Date:") + " " + valueStyle.Render(system.InstallDate.Format("2006-01-02")) + "\n")
	b.WriteString(labelStyle.Render("Interval:") + " " + valueStyle.Render(fmt.Sprintf("%d days", system.MaintenanceIntervalDays)) + "\n")
	if system.LastMaintenanceDate != nil {
		b.WriteString(labelStyle.Render("Last Maintenance:") + " " + valueStyle.Render(system.LastMaintenanceDate.Format("2006-01-02")) + "\n")
	}
	if system.NextMaintenanceDue != nil {
		dueStyle := valueStyle
		if system.IsOverdueForMaintenance(v.vaultTime) {
			dueStyle = errStyle
		}
		b.WriteString(labelStyle.Render("Next Due:") + " " + dueStyle.Render(system.NextMaintenanceDue.Format("2006-01-02")) + "\n")
	}
	if system.MTBFHours != nil {
		b.WriteString(labelStyle.Render("MTBF:") + " " + valueStyle.Render(fmt.Sprintf("%d hours", *system.MTBFHours)) + "\n")
	}
	b.WriteString("\n")

	// Notes
	if system.Notes != "" {
		b.WriteString(sectionStyle.Render("NOTES"))
		b.WriteString("\n")
		b.WriteString(system.Notes + "\n\n")
	}

	b.WriteString(helpStyle.Render("Esc:Back  m:Maintenance Log"))

	return b.String()
}
