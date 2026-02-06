package tui

import (
	"strings"
	"testing"
)

func TestGetBreakpoint(t *testing.T) {
	tests := []struct {
		width    int
		expected LayoutBreakpoint
	}{
		{40, BreakpointNarrow},
		{59, BreakpointNarrow},
		{60, BreakpointMedium},
		{80, BreakpointMedium},
		{99, BreakpointMedium},
		{100, BreakpointWide},
		{140, BreakpointWide},
		{200, BreakpointWide},
	}

	for _, tt := range tests {
		result := GetBreakpoint(tt.width)
		if result != tt.expected {
			t.Errorf("GetBreakpoint(%d) = %d, want %d", tt.width, result, tt.expected)
		}
	}
}

func TestCalculateColumnWidths_AllFixed(t *testing.T) {
	specs := []ColumnSpec{
		{Fixed: 10, Priority: 3},
		{Fixed: 15, Priority: 2},
		{Fixed: 20, Priority: 1},
	}

	widths := CalculateColumnWidths(specs, 100, 3)

	// All should be their fixed widths with plenty of room
	if widths[0] != 10 {
		t.Errorf("widths[0] = %d, want 10", widths[0])
	}
	if widths[1] != 15 {
		t.Errorf("widths[1] = %d, want 15", widths[1])
	}
	if widths[2] != 20 {
		t.Errorf("widths[2] = %d, want 20", widths[2])
	}
}

func TestCalculateColumnWidths_ProportionalDistribution(t *testing.T) {
	specs := []ColumnSpec{
		{Fixed: 10, Priority: 3},
		{Weight: 1.0, MinWidth: 5, Priority: 2},
		{Weight: 2.0, MinWidth: 5, Priority: 1},
	}

	widths := CalculateColumnWidths(specs, 100, 3)

	// Fixed column should be 10
	if widths[0] != 10 {
		t.Errorf("widths[0] = %d, want 10", widths[0])
	}
	// Weighted columns should have roughly 1:2 ratio
	// Available: 100 - 10 (fixed) - 6 (separators) - 2 (padding) = 82
	// Weight col 1: 82 * 1/3 ≈ 27
	// Weight col 2: 82 * 2/3 ≈ 54
	if widths[1] < 20 || widths[1] > 35 {
		t.Errorf("widths[1] = %d, want ~27", widths[1])
	}
	if widths[2] < 45 || widths[2] > 60 {
		t.Errorf("widths[2] = %d, want ~54", widths[2])
	}
	// Weight ratio should be approximately 1:2
	ratio := float64(widths[2]) / float64(widths[1])
	if ratio < 1.5 || ratio > 2.5 {
		t.Errorf("ratio widths[2]/widths[1] = %.2f, want ~2.0", ratio)
	}
}

func TestCalculateColumnWidths_DropsLowPriority(t *testing.T) {
	specs := []ColumnSpec{
		{Fixed: 30, Priority: 3},
		{Fixed: 30, Priority: 2},
		{Fixed: 30, Priority: 1}, // lowest priority, should be dropped first
	}

	// Width only fits ~2 columns (30+30+sep+pad = 67, third needs 96+)
	widths := CalculateColumnWidths(specs, 70, 3)

	// Lowest priority column should be dropped (width = 0)
	if widths[2] != 0 {
		t.Errorf("widths[2] = %d, want 0 (should be dropped)", widths[2])
	}
	// Higher priority columns should be preserved
	if widths[0] != 30 {
		t.Errorf("widths[0] = %d, want 30", widths[0])
	}
	if widths[1] != 30 {
		t.Errorf("widths[1] = %d, want 30", widths[1])
	}
}

func TestCalculateColumnWidths_VeryNarrow(t *testing.T) {
	specs := []ColumnSpec{
		{Fixed: 10, Priority: 3},
		{Fixed: 10, Priority: 2},
		{Fixed: 10, Priority: 1},
	}

	// Very narrow - should drop two columns
	widths := CalculateColumnWidths(specs, 15, 3)

	visibleCount := 0
	for _, w := range widths {
		if w > 0 {
			visibleCount++
		}
	}

	// At minimum, the highest priority column should survive
	if widths[0] == 0 {
		t.Error("Highest priority column should not be dropped")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxWidth int
		expected string
	}{
		{"hello", 10, "hello"},       // fits
		{"hello", 5, "hello"},        // exact fit
		{"hello world", 5, "hell…"},  // truncated
		{"hi", 0, ""},                // zero width
		{"hello world", 3, "hel"},    // very short (<=3)
		{"hello world", 1, "h"},      // single char
	}

	for _, tt := range tests {
		result := Truncate(tt.input, tt.maxWidth)
		if result != tt.expected {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxWidth, result, tt.expected)
		}
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},     // exact fit
		{"hello!", 5, "hello!"},   // already wider
	}

	for _, tt := range tests {
		result := PadRight(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("PadRight(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
		}
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hi", 5, "   hi"},
		{"hello", 5, "hello"},     // exact fit
		{"hello!", 5, "hello!"},   // already wider
	}

	for _, tt := range tests {
		result := PadLeft(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("PadLeft(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
		}
	}
}

func TestContentWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		minWidth  int
		maxWidth  int
		expected  int
	}{
		{80, 40, 120, 80},   // normal
		{30, 40, 120, 40},   // below min, clamp up
		{200, 40, 120, 120}, // above max, clamp down
		{80, 40, 0, 80},     // no max
	}

	for _, tt := range tests {
		result := ContentWidth(tt.termWidth, tt.minWidth, tt.maxWidth)
		if result != tt.expected {
			t.Errorf("ContentWidth(%d, %d, %d) = %d, want %d",
				tt.termWidth, tt.minWidth, tt.maxWidth, result, tt.expected)
		}
	}
}

func TestContentHeight(t *testing.T) {
	tests := []struct {
		termHeight  int
		chromeLines int
		expected    int
	}{
		{24, 6, 18},  // normal
		{40, 6, 34},  // tall terminal
		{8, 6, 5},    // very short, clamps to 5
		{5, 6, 5},    // shorter than chrome, clamps to 5
	}

	for _, tt := range tests {
		result := ContentHeight(tt.termHeight, tt.chromeLines)
		if result != tt.expected {
			t.Errorf("ContentHeight(%d, %d) = %d, want %d",
				tt.termHeight, tt.chromeLines, result, tt.expected)
		}
	}
}

func TestSideBySide_Horizontal(t *testing.T) {
	left := "AAA"
	right := "BBB"
	result := SideBySide(left, right, 80, 4)

	// Should contain both strings on the same line (no double newline between them)
	if strings.Contains(result, "\n\n") {
		t.Error("Expected horizontal layout, got vertical (double newline found)")
	}
	if !strings.Contains(result, "AAA") || !strings.Contains(result, "BBB") {
		t.Error("Expected both strings in output")
	}
}

func TestSideBySide_Vertical(t *testing.T) {
	left := strings.Repeat("A", 50)
	right := strings.Repeat("B", 50)
	result := SideBySide(left, right, 60, 4)

	// Should stack vertically since both strings won't fit
	if !strings.Contains(result, "\n\n") {
		t.Error("Expected vertical layout (double newline) when content doesn't fit")
	}
}

func TestProgressBar(t *testing.T) {
	theme := NewTheme("green")

	// Basic progress bar should contain filled and empty characters
	bar := theme.ProgressBar(0.5, 1.0, 20)
	if bar == "" {
		t.Error("Expected non-empty progress bar")
	}
	if !strings.Contains(bar, "█") {
		t.Error("Expected filled characters in progress bar")
	}
	if !strings.Contains(bar, "░") {
		t.Error("Expected empty characters in progress bar")
	}

	// Full bar
	full := theme.ProgressBar(1.0, 1.0, 20)
	if strings.Contains(full, "░") {
		t.Error("Full progress bar should not contain empty characters")
	}

	// Empty bar
	empty := theme.ProgressBar(0, 1.0, 20)
	if strings.Contains(empty, "█") {
		t.Error("Empty progress bar should not contain filled characters")
	}
}
