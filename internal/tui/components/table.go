// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Column defines a table column.
type Column struct {
	Title    string
	Width    int // Used as base/minimum width for proportional sizing
	Align    lipgloss.Position
	Sortable bool
	// Weight controls proportional sizing (0 = use fixed Width).
	Weight float64
	// Priority controls which columns are hidden first on narrow terminals.
	// Higher priority columns are kept. 0 means always visible.
	Priority int
}

// Table is a simple table component.
type Table struct {
	columns     []Column
	rows        [][]string
	selected    int
	offset      int
	visibleRows int
	focused     bool

	// Styles
	headerStyle   lipgloss.Style
	rowStyle      lipgloss.Style
	rowAltStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	borderStyle   lipgloss.Style

	// Pagination
	currentPage int
	totalPages  int
	totalRows   int
	pageSize    int
}

// NewTable creates a new table with the given columns.
func NewTable(columns []Column) *Table {
	return &Table{
		columns:       columns,
		rows:          [][]string{},
		selected:      0,
		offset:        0,
		visibleRows:   10,
		headerStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#66FF66")),
		rowStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")),
		rowAltStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")),
		selectedStyle: lipgloss.NewStyle().Background(lipgloss.Color("#00FF00")).Foreground(lipgloss.Color("#000000")),
		borderStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")),
		pageSize:      25,
	}
}

// SetRows sets the table data.
func (t *Table) SetRows(rows [][]string) {
	t.rows = rows
}

// SetPagination sets pagination info.
func (t *Table) SetPagination(page, totalPages, totalRows int) {
	t.currentPage = page
	t.totalPages = totalPages
	t.totalRows = totalRows
}

// SetVisibleRows sets the number of visible rows.
func (t *Table) SetVisibleRows(n int) {
	t.visibleRows = n
}

// SetStyles sets the table styles.
func (t *Table) SetStyles(header, row, rowAlt, selected, border lipgloss.Style) {
	t.headerStyle = header
	t.rowStyle = row
	t.rowAltStyle = rowAlt
	t.selectedStyle = selected
	t.borderStyle = border
}

// Focus sets the table focus state.
func (t *Table) Focus(focused bool) {
	t.focused = focused
}

// Selected returns the currently selected row index.
func (t *Table) Selected() int {
	return t.selected
}

// SelectedRow returns the currently selected row data.
func (t *Table) SelectedRow() []string {
	if t.selected >= 0 && t.selected < len(t.rows) {
		return t.rows[t.selected]
	}
	return nil
}

// MoveUp moves the selection up.
func (t *Table) MoveUp() {
	if t.selected > 0 {
		t.selected--
		if t.selected < t.offset {
			t.offset = t.selected
		}
	}
}

// MoveDown moves the selection down.
func (t *Table) MoveDown() {
	if t.selected < len(t.rows)-1 {
		t.selected++
		if t.selected >= t.offset+t.visibleRows {
			t.offset = t.selected - t.visibleRows + 1
		}
	}
}

// PageUp moves up one page.
func (t *Table) PageUp() {
	t.selected -= t.visibleRows
	if t.selected < 0 {
		t.selected = 0
	}
	t.offset = t.selected
}

// PageDown moves down one page.
func (t *Table) PageDown() {
	t.selected += t.visibleRows
	if t.selected >= len(t.rows) {
		t.selected = len(t.rows) - 1
	}
	if t.selected < 0 {
		t.selected = 0
	}
	t.offset = t.selected - t.visibleRows + 1
	if t.offset < 0 {
		t.offset = 0
	}
}

// GoToTop goes to the first row.
func (t *Table) GoToTop() {
	t.selected = 0
	t.offset = 0
}

// GoToBottom goes to the last row.
func (t *Table) GoToBottom() {
	if len(t.rows) > 0 {
		t.selected = len(t.rows) - 1
		t.offset = t.selected - t.visibleRows + 1
		if t.offset < 0 {
			t.offset = 0
		}
	}
}

// computeWidths calculates the actual display width for each column based on
// available terminal width. Columns with Weight > 0 get proportional space;
// columns with Weight == 0 use their fixed Width. Low-priority columns are
// dropped if the terminal is too narrow.
func (t *Table) computeWidths(availableWidth int) []int {
	widths := make([]int, len(t.columns))
	visible := make([]bool, len(t.columns))

	for i := range t.columns {
		visible[i] = true
	}

	for {
		visibleCount := 0
		totalFixed := 0
		totalWeight := 0.0
		for i, col := range t.columns {
			if !visible[i] {
				continue
			}
			visibleCount++
			if col.Weight > 0 {
				totalWeight += col.Weight
			} else {
				totalFixed += col.Width
			}
		}

		separatorWidth := 0
		if visibleCount > 1 {
			separatorWidth = (visibleCount - 1) * 3 // " | "
		}
		remaining := availableWidth - totalFixed - separatorWidth - 2 // -2 for row padding

		if remaining >= 0 || visibleCount <= 1 {
			// Distribute remaining space by weight
			for i, col := range t.columns {
				if !visible[i] {
					widths[i] = 0
					continue
				}
				if col.Weight > 0 && totalWeight > 0 {
					w := int(float64(remaining) * col.Weight / totalWeight)
					if w < col.Width {
						w = col.Width
					}
					widths[i] = w
				} else {
					widths[i] = col.Width
				}
			}
			break
		}

		// Drop lowest priority visible column
		lowestPri := -1
		lowestIdx := -1
		for i, col := range t.columns {
			if !visible[i] {
				continue
			}
			if lowestPri == -1 || col.Priority < lowestPri {
				lowestPri = col.Priority
				lowestIdx = i
			}
		}
		if lowestIdx < 0 {
			break
		}
		visible[lowestIdx] = false
	}

	return widths
}

// Render renders the table using fixed column widths.
func (t *Table) Render() string {
	return t.RenderResponsive(0)
}

// RenderResponsive renders the table adapted to the given terminal width.
// If width <= 0, it falls back to fixed column widths.
func (t *Table) RenderResponsive(width int) string {
	var colWidths []int
	if width > 0 {
		colWidths = t.computeWidths(width)
	} else {
		colWidths = make([]int, len(t.columns))
		for i, col := range t.columns {
			colWidths[i] = col.Width
		}
	}

	var b strings.Builder

	totalWidth := 0
	visibleCount := 0
	for _, w := range colWidths {
		if w > 0 {
			totalWidth += w + 3
			visibleCount++
		}
	}
	if visibleCount > 0 {
		totalWidth -= 3 // Last column has no trailing separator
		totalWidth += 2 // Row padding
	}

	// Render header
	b.WriteString(t.renderRowResponsive(t.getHeaders(), t.headerStyle, false, colWidths))
	b.WriteString("\n")

	// Render separator
	sepWidth := totalWidth
	if sepWidth < 1 {
		sepWidth = 1
	}
	b.WriteString(t.borderStyle.Render(strings.Repeat("─", sepWidth)))
	b.WriteString("\n")

	// Render visible rows
	endIdx := t.offset + t.visibleRows
	if endIdx > len(t.rows) {
		endIdx = len(t.rows)
	}

	for i := t.offset; i < endIdx; i++ {
		isSelected := i == t.selected && t.focused
		isAlt := (i-t.offset)%2 == 1

		var style lipgloss.Style
		if isSelected {
			style = t.selectedStyle
		} else if isAlt {
			style = t.rowAltStyle
		} else {
			style = t.rowStyle
		}

		b.WriteString(t.renderRowResponsive(t.rows[i], style, isSelected, colWidths))
		b.WriteString("\n")
	}

	// Show pagination info
	if t.totalPages > 0 {
		b.WriteString(t.borderStyle.Render(strings.Repeat("─", sepWidth)))
		b.WriteString("\n")
		pageInfo := fmt.Sprintf("Page %d/%d │ %d total", t.currentPage, t.totalPages, t.totalRows)
		b.WriteString(t.borderStyle.Render(pageInfo))
	}

	return b.String()
}

func (t *Table) getHeaders() []string {
	headers := make([]string, len(t.columns))
	for i, col := range t.columns {
		headers[i] = col.Title
	}
	return headers
}

func (t *Table) renderRow(cells []string, style lipgloss.Style, isSelected bool) string {
	widths := make([]int, len(t.columns))
	for i, col := range t.columns {
		widths[i] = col.Width
	}
	return t.renderRowResponsive(cells, style, isSelected, widths)
}

func (t *Table) renderRowResponsive(cells []string, style lipgloss.Style, isSelected bool, colWidths []int) string {
	var parts []string

	for i, col := range t.columns {
		w := colWidths[i]
		if w <= 0 {
			continue // Column hidden
		}

		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}

		// Truncate if too long
		if len(cell) > w {
			if w > 1 {
				cell = cell[:w-1] + "…"
			} else {
				cell = cell[:w]
			}
		}

		// Pad to width
		switch col.Align {
		case lipgloss.Right:
			cell = fmt.Sprintf("%*s", w, cell)
		case lipgloss.Center:
			padding := w - len(cell)
			leftPad := padding / 2
			rightPad := padding - leftPad
			cell = strings.Repeat(" ", leftPad) + cell + strings.Repeat(" ", rightPad)
		default: // Left
			cell = fmt.Sprintf("%-*s", w, cell)
		}

		parts = append(parts, style.Render(cell))
	}

	return " " + strings.Join(parts, " │ ") + " "
}

// Empty returns true if the table has no rows.
func (t *Table) Empty() bool {
	return len(t.rows) == 0
}

// RowCount returns the number of rows.
func (t *Table) RowCount() int {
	return len(t.rows)
}
