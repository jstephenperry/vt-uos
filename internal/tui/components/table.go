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
	Width    int
	Align    lipgloss.Position
	Sortable bool
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

// Render renders the table.
func (t *Table) Render() string {
	var b strings.Builder

	// Calculate total width
	totalWidth := 0
	for _, col := range t.columns {
		totalWidth += col.Width + 3 // +3 for padding and separator
	}

	// Render header
	b.WriteString(t.renderRow(t.getHeaders(), t.headerStyle, false))
	b.WriteString("\n")

	// Render separator
	b.WriteString(t.borderStyle.Render(strings.Repeat("-", totalWidth)))
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

		b.WriteString(t.renderRow(t.rows[i], style, isSelected))
		b.WriteString("\n")
	}

	// Show pagination info
	if t.totalPages > 0 {
		b.WriteString(t.borderStyle.Render(strings.Repeat("-", totalWidth)))
		b.WriteString("\n")
		b.WriteString(t.borderStyle.Render(fmt.Sprintf("Page %d/%d | %d total", t.currentPage, t.totalPages, t.totalRows)))
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
	var parts []string

	for i, col := range t.columns {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}

		// Truncate if too long
		if len(cell) > col.Width {
			cell = cell[:col.Width-1] + "…"
		}

		// Pad to width
		switch col.Align {
		case lipgloss.Right:
			cell = fmt.Sprintf("%*s", col.Width, cell)
		case lipgloss.Center:
			padding := col.Width - len(cell)
			leftPad := padding / 2
			rightPad := padding - leftPad
			cell = strings.Repeat(" ", leftPad) + cell + strings.Repeat(" ", rightPad)
		default: // Left
			cell = fmt.Sprintf("%-*s", col.Width, cell)
		}

		if isSelected {
			parts = append(parts, style.Render(cell))
		} else {
			parts = append(parts, style.Render(cell))
		}
	}

	return " " + strings.Join(parts, " | ") + " "
}

// Empty returns true if the table has no rows.
func (t *Table) Empty() bool {
	return len(t.rows) == 0
}

// RowCount returns the number of rows.
func (t *Table) RowCount() int {
	return len(t.rows)
}
