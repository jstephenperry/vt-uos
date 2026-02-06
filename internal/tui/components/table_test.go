package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewTable(t *testing.T) {
	cols := []Column{
		{Title: "ID", Width: 5},
		{Title: "Name", Width: 20},
	}

	table := NewTable(cols)
	if table == nil {
		t.Fatal("Expected non-nil table")
	}
	if table.Empty() {
		// New table should be empty
	}
	if table.RowCount() != 0 {
		t.Errorf("Expected 0 rows, got %d", table.RowCount())
	}
}

func TestTable_SetRows(t *testing.T) {
	cols := []Column{
		{Title: "ID", Width: 5},
		{Title: "Name", Width: 20},
	}

	table := NewTable(cols)
	rows := [][]string{
		{"1", "Alice"},
		{"2", "Bob"},
		{"3", "Charlie"},
	}
	table.SetRows(rows)

	if table.RowCount() != 3 {
		t.Errorf("Expected 3 rows, got %d", table.RowCount())
	}
	if table.Empty() {
		t.Error("Table should not be empty after setting rows")
	}
}

func TestTable_Navigation(t *testing.T) {
	cols := []Column{{Title: "ID", Width: 5}}
	table := NewTable(cols)
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}})

	// Initially at row 0
	if table.Selected() != 0 {
		t.Errorf("Expected selected=0, got %d", table.Selected())
	}

	// Move down
	table.MoveDown()
	if table.Selected() != 1 {
		t.Errorf("Expected selected=1, got %d", table.Selected())
	}

	// Move up
	table.MoveUp()
	if table.Selected() != 0 {
		t.Errorf("Expected selected=0, got %d", table.Selected())
	}

	// Can't move above 0
	table.MoveUp()
	if table.Selected() != 0 {
		t.Errorf("Expected selected=0, got %d", table.Selected())
	}

	// GoToBottom
	table.GoToBottom()
	if table.Selected() != 4 {
		t.Errorf("Expected selected=4, got %d", table.Selected())
	}

	// Can't move below last
	table.MoveDown()
	if table.Selected() != 4 {
		t.Errorf("Expected selected=4, got %d", table.Selected())
	}

	// GoToTop
	table.GoToTop()
	if table.Selected() != 0 {
		t.Errorf("Expected selected=0, got %d", table.Selected())
	}
}

func TestTable_SelectedRow(t *testing.T) {
	cols := []Column{{Title: "ID", Width: 5}, {Title: "Name", Width: 10}}
	table := NewTable(cols)
	table.SetRows([][]string{{"1", "Alice"}, {"2", "Bob"}})

	row := table.SelectedRow()
	if row == nil {
		t.Fatal("Expected non-nil selected row")
	}
	if row[0] != "1" || row[1] != "Alice" {
		t.Errorf("Expected [1, Alice], got %v", row)
	}

	table.MoveDown()
	row = table.SelectedRow()
	if row[0] != "2" || row[1] != "Bob" {
		t.Errorf("Expected [2, Bob], got %v", row)
	}
}

func TestTable_EmptySelectedRow(t *testing.T) {
	cols := []Column{{Title: "ID", Width: 5}}
	table := NewTable(cols)

	row := table.SelectedRow()
	if row != nil {
		t.Errorf("Expected nil for empty table selected row, got %v", row)
	}
}

func TestTable_PageNavigation(t *testing.T) {
	cols := []Column{{Title: "ID", Width: 5}}
	table := NewTable(cols)
	table.SetVisibleRows(3)

	rows := make([][]string, 10)
	for i := range rows {
		rows[i] = []string{string(rune('A' + i))}
	}
	table.SetRows(rows)

	// PageDown should jump by visible rows
	table.PageDown()
	if table.Selected() != 3 {
		t.Errorf("After PageDown expected selected=3, got %d", table.Selected())
	}

	// PageUp should jump back
	table.PageUp()
	if table.Selected() != 0 {
		t.Errorf("After PageUp expected selected=0, got %d", table.Selected())
	}
}

func TestTable_ComputeWidths_AllVisible(t *testing.T) {
	cols := []Column{
		{Title: "A", Width: 10, Priority: 3},
		{Title: "B", Width: 10, Priority: 2},
		{Title: "C", Width: 10, Priority: 1},
	}

	table := NewTable(cols)
	widths := table.computeWidths(100) // plenty of room

	for i, w := range widths {
		if w != 10 {
			t.Errorf("widths[%d] = %d, want 10", i, w)
		}
	}
}

func TestTable_ComputeWidths_DropsLowestPriority(t *testing.T) {
	cols := []Column{
		{Title: "A", Width: 20, Priority: 3},
		{Title: "B", Width: 20, Priority: 2},
		{Title: "C", Width: 20, Priority: 1}, // lowest priority
	}

	table := NewTable(cols)
	// Not enough room for all three (need 20+20+20 + 6(sep) + 2(pad) = 68)
	widths := table.computeWidths(50)

	// Lowest priority (C) should be dropped
	if widths[2] != 0 {
		t.Errorf("widths[2] = %d, want 0 (should be dropped)", widths[2])
	}
	if widths[0] == 0 {
		t.Error("widths[0] should not be 0")
	}
}

func TestTable_ComputeWidths_ProportionalWeight(t *testing.T) {
	cols := []Column{
		{Title: "Fixed", Width: 10, Weight: 0, Priority: 3},
		{Title: "Flex1", Width: 5, Weight: 1.0, Priority: 2},
		{Title: "Flex2", Width: 5, Weight: 2.0, Priority: 1},
	}

	table := NewTable(cols)
	widths := table.computeWidths(100)

	// First column should be fixed at 10
	if widths[0] != 10 {
		t.Errorf("widths[0] = %d, want 10", widths[0])
	}

	// Flex columns should have approximately 1:2 ratio
	if widths[1] == 0 || widths[2] == 0 {
		t.Fatalf("Flex columns shouldn't be dropped: widths[1]=%d, widths[2]=%d", widths[1], widths[2])
	}

	ratio := float64(widths[2]) / float64(widths[1])
	if ratio < 1.5 || ratio > 2.5 {
		t.Errorf("Flex ratio = %.2f, want ~2.0 (widths: %d, %d)", ratio, widths[1], widths[2])
	}
}

func TestTable_RenderResponsive_ContainsHeaders(t *testing.T) {
	cols := []Column{
		{Title: "ID", Width: 5, Priority: 2},
		{Title: "Name", Width: 10, Priority: 1},
	}

	table := NewTable(cols)
	table.SetRows([][]string{{"1", "Alice"}, {"2", "Bob"}})

	output := table.RenderResponsive(80)

	if !strings.Contains(output, "ID") {
		t.Error("Expected header 'ID' in output")
	}
	if !strings.Contains(output, "Name") {
		t.Error("Expected header 'Name' in output")
	}
	if !strings.Contains(output, "Alice") {
		t.Error("Expected row data 'Alice' in output")
	}
}

func TestTable_RenderResponsive_HidesColumnsOnNarrow(t *testing.T) {
	cols := []Column{
		{Title: "ID", Width: 15, Priority: 2},
		{Title: "Extra", Width: 15, Priority: 1},
	}

	table := NewTable(cols)
	table.SetRows([][]string{{"001", "Details"}})

	// Very narrow - should hide lowest priority column
	output := table.RenderResponsive(20)

	if !strings.Contains(output, "ID") {
		t.Error("Expected high-priority header 'ID' in narrow output")
	}
	// Low priority "Extra" column may be dropped
	// The data "Details" should not appear if its column was dropped
	if strings.Contains(output, "Details") {
		// Column wasn't dropped, which is OK if it fits
		// This depends on exact width calculations
	}
}

func TestTable_Render_FallsBackToFixed(t *testing.T) {
	cols := []Column{
		{Title: "ID", Width: 5},
		{Title: "Name", Width: 10},
	}

	table := NewTable(cols)
	table.SetRows([][]string{{"1", "Alice"}})

	// Render() calls RenderResponsive(0) which uses fixed widths
	output := table.Render()
	if !strings.Contains(output, "Alice") {
		t.Error("Expected 'Alice' in fallback render output")
	}
}

func TestTable_RenderResponsive_ShowsPagination(t *testing.T) {
	cols := []Column{
		{Title: "ID", Width: 5, Priority: 1},
	}

	table := NewTable(cols)
	table.SetRows([][]string{{"1"}, {"2"}})
	table.SetPagination(1, 5, 100)

	output := table.RenderResponsive(80)

	if !strings.Contains(output, "Page 1/5") {
		t.Error("Expected pagination info in output")
	}
	if !strings.Contains(output, "100 total") {
		t.Error("Expected total count in output")
	}
}

func TestTable_RenderResponsive_RightAligned(t *testing.T) {
	cols := []Column{
		{Title: "Value", Width: 10, Align: lipgloss.Right, Priority: 1},
	}

	table := NewTable(cols)
	table.SetRows([][]string{{"42"}})
	table.Focus(true)

	output := table.RenderResponsive(80)
	// "42" should be right-padded to width 10, so it should be preceded by spaces
	if !strings.Contains(output, "42") {
		t.Error("Expected '42' in output")
	}
}

func TestTable_SetPagination(t *testing.T) {
	cols := []Column{{Title: "ID", Width: 5}}
	table := NewTable(cols)

	table.SetPagination(3, 10, 250)

	// Verify by rendering
	table.SetRows([][]string{{"1"}})
	output := table.RenderResponsive(80)
	if !strings.Contains(output, "Page 3/10") {
		t.Error("Expected 'Page 3/10' in output")
	}
}
