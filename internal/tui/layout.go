package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LayoutBreakpoint defines terminal width thresholds for responsive layout.
type LayoutBreakpoint int

const (
	// BreakpointNarrow is for terminals under 60 columns (e.g., Pi Zero).
	BreakpointNarrow LayoutBreakpoint = 60
	// BreakpointMedium is for terminals between 60-100 columns.
	BreakpointMedium LayoutBreakpoint = 100
	// BreakpointWide is for terminals over 100 columns.
	BreakpointWide LayoutBreakpoint = 140
)

// GetBreakpoint returns the current layout breakpoint for the given width.
func GetBreakpoint(width int) LayoutBreakpoint {
	switch {
	case width < int(BreakpointNarrow):
		return BreakpointNarrow
	case width < int(BreakpointMedium):
		return BreakpointMedium
	default:
		return BreakpointWide
	}
}

// ColumnSpec defines a column with proportional or fixed width.
type ColumnSpec struct {
	// MinWidth is the absolute minimum width; column is hidden below this.
	MinWidth int
	// Weight is the proportional share of remaining width.
	Weight float64
	// Fixed is a fixed width (overrides Weight if > 0).
	Fixed int
	// Priority determines drop order when terminal is narrow (lower = dropped first).
	Priority int
}

// CalculateColumnWidths distributes available width among columns proportionally.
// Columns with priority below minPriority are hidden (returned as 0 width).
// The separator parameter is the width consumed per column gap (e.g., " | " = 3).
func CalculateColumnWidths(specs []ColumnSpec, availableWidth int, separator int) []int {
	widths := make([]int, len(specs))

	// Determine which columns are visible
	visible := make([]bool, len(specs))
	totalFixed := 0
	totalWeight := 0.0
	visibleCount := 0

	for i, spec := range specs {
		visible[i] = true
		visibleCount++
		if spec.Fixed > 0 {
			totalFixed += spec.Fixed
		} else {
			totalWeight += spec.Weight
		}
	}

	// Account for separators between visible columns
	separatorWidth := 0
	if visibleCount > 1 {
		separatorWidth = (visibleCount - 1) * separator
	}
	remaining := availableWidth - totalFixed - separatorWidth - 2 // -2 for row padding

	// If too narrow, progressively drop low-priority columns
	for remaining < 0 && visibleCount > 1 {
		// Find lowest priority visible column
		lowestPri := -1
		lowestIdx := -1
		for i, spec := range specs {
			if !visible[i] {
				continue
			}
			if lowestPri == -1 || spec.Priority < lowestPri {
				lowestPri = spec.Priority
				lowestIdx = i
			}
		}
		if lowestIdx < 0 {
			break
		}
		visible[lowestIdx] = false
		visibleCount--
		if specs[lowestIdx].Fixed > 0 {
			totalFixed -= specs[lowestIdx].Fixed
		} else {
			totalWeight -= specs[lowestIdx].Weight
		}
		separatorWidth = 0
		if visibleCount > 1 {
			separatorWidth = (visibleCount - 1) * separator
		}
		remaining = availableWidth - totalFixed - separatorWidth - 2
	}

	if remaining < 0 {
		remaining = 0
	}

	// Distribute remaining space by weight
	for i, spec := range specs {
		if !visible[i] {
			widths[i] = 0
			continue
		}
		if spec.Fixed > 0 {
			widths[i] = spec.Fixed
		} else if totalWeight > 0 {
			w := int(float64(remaining) * spec.Weight / totalWeight)
			if w < spec.MinWidth {
				w = spec.MinWidth
			}
			widths[i] = w
		} else {
			widths[i] = spec.MinWidth
		}
	}

	return widths
}

// Panel renders a bordered panel with a title.
func (t *Theme) Panel(title, content string, width int) string {
	titleStr := t.Subtitle.Render(" " + title + " ")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.SecondaryColor).
		Width(width - 2). // -2 for border chars
		Padding(0, 1)

	rendered := style.Render(content)

	// Replace top border segment with title
	lines := strings.Split(rendered, "\n")
	if len(lines) > 0 && len(title) > 0 {
		topLine := lines[0]
		titleRendered := t.Accent.Bold(true).Render(" " + title + " ")
		titleWidth := lipgloss.Width(titleRendered)
		topLineWidth := lipgloss.Width(topLine)
		if titleWidth+4 < topLineWidth {
			// Insert title after the first corner character
			lines[0] = string([]rune(topLine)[:2]) + titleRendered + string([]rune(topLine)[2+titleWidth:])
		}
		rendered = strings.Join(lines, "\n")
	}
	_ = titleStr

	return rendered
}

// SideBySide renders two strings side by side, collapsing to vertical on narrow terminals.
func SideBySide(left, right string, totalWidth, gap int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	// If both fit side by side, render horizontally
	if leftWidth+rightWidth+gap <= totalWidth {
		spacing := totalWidth - leftWidth - rightWidth
		if spacing < gap {
			spacing = gap
		}
		leftLines := strings.Split(left, "\n")
		rightLines := strings.Split(right, "\n")
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
			pad := (totalWidth / 2) - lw
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

	// Otherwise stack vertically
	return left + "\n\n" + right
}

// ProgressBar renders a text-based progress bar.
func (t *Theme) ProgressBar(value, max float64, width int) string {
	if max <= 0 {
		max = 1
	}
	ratio := value / max
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0 {
		ratio = 0
	}

	barWidth := width - 2 // for [ and ]
	if barWidth < 4 {
		barWidth = 4
	}

	filled := int(ratio * float64(barWidth))
	empty := barWidth - filled

	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"

	// Color based on ratio
	switch {
	case ratio > 0.6:
		return t.Success.Render(bar)
	case ratio > 0.3:
		return t.Warning.Render(bar)
	default:
		return t.Error.Render(bar)
	}
}

// Truncate shortens a string to fit within maxWidth, adding ellipsis if needed.
func Truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return s[:maxWidth]
	}
	runes := []rune(s)
	if len(runes) > maxWidth-1 {
		runes = runes[:maxWidth-1]
	}
	return string(runes) + "…"
}

// PadRight pads a string to the given width with spaces.
func PadRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft pads a string to the given width with spaces on the left.
func PadLeft(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// ContentWidth returns the usable content width, capped between min and max.
func ContentWidth(termWidth, minWidth, maxWidth int) int {
	w := termWidth
	if w < minWidth {
		w = minWidth
	}
	if maxWidth > 0 && w > maxWidth {
		w = maxWidth
	}
	return w
}

// ContentHeight returns the usable content height after subtracting chrome.
// chromeLines is the total lines used by header, footer, alert bar, separators.
func ContentHeight(termHeight, chromeLines int) int {
	h := termHeight - chromeLines
	if h < 5 {
		h = 5
	}
	return h
}
