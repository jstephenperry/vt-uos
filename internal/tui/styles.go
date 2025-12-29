// Package tui provides the terminal user interface for VT-UOS.
package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/config"
)

// Theme contains all style definitions for the TUI.
type Theme struct {
	// Colors (raw values for reference)
	PrimaryColor    lipgloss.Color
	SecondaryColor  lipgloss.Color
	AccentColor     lipgloss.Color
	BackgroundColor lipgloss.Color
	ForegroundColor lipgloss.Color
	ErrorColor      lipgloss.Color
	WarningColor    lipgloss.Color
	SuccessColor    lipgloss.Color
	MutedColor      lipgloss.Color

	// Base styles
	Base      lipgloss.Style
	Bold      lipgloss.Style
	Italic    lipgloss.Style
	Underline lipgloss.Style

	// Color styles (for direct use)
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Accent    lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Success   lipgloss.Style
	Muted     lipgloss.Style

	// Component styles
	Header    lipgloss.Style
	Footer    lipgloss.Style
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Label     lipgloss.Style
	Value     lipgloss.Style
	Box       lipgloss.Style
	Border    lipgloss.Style
	Selected  lipgloss.Style
	Focused   lipgloss.Style
	Disabled  lipgloss.Style
	Alert     lipgloss.Style
	AlertWarn lipgloss.Style
	AlertCrit lipgloss.Style

	// Table styles
	TableHeader lipgloss.Style
	TableRow    lipgloss.Style
	TableRowAlt lipgloss.Style

	// Menu styles
	MenuItem         lipgloss.Style
	MenuItemSelected lipgloss.Style
	MenuItemDisabled lipgloss.Style

	// Form styles
	FormLabel lipgloss.Style
	FormInput lipgloss.Style
	FormError lipgloss.Style

	// Status bar
	StatusBar     lipgloss.Style
	StatusKey     lipgloss.Style
	StatusValue   lipgloss.Style
	StatusDivider lipgloss.Style
}

// NewTheme creates a new theme based on the color scheme configuration.
func NewTheme(scheme config.ColorScheme) *Theme {
	switch scheme {
	case config.ColorSchemeAmber:
		return newAmberTheme()
	case config.ColorSchemeWhite:
		return newWhiteTheme()
	default:
		return newGreenPhosphorTheme()
	}
}

// newGreenPhosphorTheme creates the classic green phosphor terminal theme.
func newGreenPhosphorTheme() *Theme {
	primary := lipgloss.Color("#00FF00")
	secondary := lipgloss.Color("#00AA00")
	accent := lipgloss.Color("#66FF66")
	background := lipgloss.Color("#000000")
	foreground := lipgloss.Color("#00FF00")
	muted := lipgloss.Color("#006600")
	errorColor := lipgloss.Color("#FF4444")
	warningColor := lipgloss.Color("#FFAA00")
	successColor := lipgloss.Color("#00FF00")

	return buildTheme(primary, secondary, accent, background, foreground, muted, errorColor, warningColor, successColor)
}

// newAmberTheme creates an amber/orange phosphor terminal theme.
func newAmberTheme() *Theme {
	primary := lipgloss.Color("#FFAA00")
	secondary := lipgloss.Color("#AA7700")
	accent := lipgloss.Color("#FFCC66")
	background := lipgloss.Color("#000000")
	foreground := lipgloss.Color("#FFAA00")
	muted := lipgloss.Color("#664400")
	errorColor := lipgloss.Color("#FF4444")
	warningColor := lipgloss.Color("#FFFF00")
	successColor := lipgloss.Color("#FFAA00")

	return buildTheme(primary, secondary, accent, background, foreground, muted, errorColor, warningColor, successColor)
}

// newWhiteTheme creates a white/monochrome terminal theme.
func newWhiteTheme() *Theme {
	primary := lipgloss.Color("#FFFFFF")
	secondary := lipgloss.Color("#AAAAAA")
	accent := lipgloss.Color("#FFFFFF")
	background := lipgloss.Color("#000000")
	foreground := lipgloss.Color("#FFFFFF")
	muted := lipgloss.Color("#666666")
	errorColor := lipgloss.Color("#FF4444")
	warningColor := lipgloss.Color("#FFAA00")
	successColor := lipgloss.Color("#00FF00")

	return buildTheme(primary, secondary, accent, background, foreground, muted, errorColor, warningColor, successColor)
}

func buildTheme(primary, secondary, accent, background, foreground, muted, errorColor, warningColor, successColor lipgloss.Color) *Theme {
	t := &Theme{
		PrimaryColor:    primary,
		SecondaryColor:  secondary,
		AccentColor:     accent,
		BackgroundColor: background,
		ForegroundColor: foreground,
		MutedColor:      muted,
		ErrorColor:      errorColor,
		WarningColor:    warningColor,
		SuccessColor:    successColor,
	}

	// Base styles
	t.Base = lipgloss.NewStyle().
		Foreground(foreground)

	t.Bold = t.Base.Bold(true)

	t.Italic = t.Base.Italic(true)

	t.Underline = t.Base.Underline(true)

	// Color styles for direct use
	t.Primary = lipgloss.NewStyle().Foreground(primary)
	t.Secondary = lipgloss.NewStyle().Foreground(secondary)
	t.Accent = lipgloss.NewStyle().Foreground(accent)
	t.Error = lipgloss.NewStyle().Foreground(errorColor)
	t.Warning = lipgloss.NewStyle().Foreground(warningColor)
	t.Success = lipgloss.NewStyle().Foreground(successColor)
	t.Muted = lipgloss.NewStyle().Foreground(muted)

	// Header - top bar with vault info
	t.Header = lipgloss.NewStyle().
		Foreground(primary).
		Bold(true).
		Padding(0, 1)

	// Footer - bottom status bar
	t.Footer = lipgloss.NewStyle().
		Foreground(secondary).
		Padding(0, 1)

	// Title - main headings
	t.Title = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true).
		Padding(0, 1)

	// Subtitle - secondary headings
	t.Subtitle = lipgloss.NewStyle().
		Foreground(primary).
		Padding(0, 1)

	// Label - field labels
	t.Label = lipgloss.NewStyle().
		Foreground(secondary)

	// Value - field values
	t.Value = lipgloss.NewStyle().
		Foreground(primary)

	// Box - bordered container
	t.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(secondary).
		Padding(0, 1)

	// Border - simple border style
	t.Border = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(secondary)

	// Selected - highlighted item
	t.Selected = lipgloss.NewStyle().
		Foreground(background).
		Background(primary).
		Bold(true)

	// Focused - focused input
	t.Focused = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	// Disabled - inactive elements
	t.Disabled = lipgloss.NewStyle().
		Foreground(muted)

	// Alerts
	t.Alert = lipgloss.NewStyle().
		Foreground(primary).
		Bold(true)

	t.AlertWarn = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)

	t.AlertCrit = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true).
		Blink(true)

	// Table styles
	t.TableHeader = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(secondary).
		BorderBottom(true).
		Padding(0, 1)

	t.TableRow = lipgloss.NewStyle().
		Foreground(primary).
		Padding(0, 1)

	t.TableRowAlt = lipgloss.NewStyle().
		Foreground(secondary).
		Padding(0, 1)

	// Menu styles
	t.MenuItem = lipgloss.NewStyle().
		Foreground(primary).
		Padding(0, 2)

	t.MenuItemSelected = lipgloss.NewStyle().
		Foreground(background).
		Background(primary).
		Bold(true).
		Padding(0, 2)

	t.MenuItemDisabled = lipgloss.NewStyle().
		Foreground(muted).
		Padding(0, 2)

	// Form styles
	t.FormLabel = lipgloss.NewStyle().
		Foreground(secondary).
		Width(20)

	t.FormInput = lipgloss.NewStyle().
		Foreground(primary).
		Border(lipgloss.NormalBorder()).
		BorderForeground(secondary).
		Padding(0, 1)

	t.FormError = lipgloss.NewStyle().
		Foreground(errorColor)

	// Status bar
	t.StatusBar = lipgloss.NewStyle().
		Foreground(secondary).
		Background(lipgloss.Color("#001100")).
		Padding(0, 1)

	t.StatusKey = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)

	t.StatusValue = lipgloss.NewStyle().
		Foreground(primary)

	t.StatusDivider = lipgloss.NewStyle().
		Foreground(muted).
		SetString(" │ ")

	return t
}

// Box characters for drawing
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxTeeLeft     = "├"
	BoxTeeRight    = "┤"
	BoxTeeTop      = "┬"
	BoxTeeBottom   = "┴"
	BoxCross       = "┼"

	// Double box characters
	BoxDoubleHorizontal = "═"
	BoxDoubleVertical   = "║"
)

// DrawBox draws a box with the given content.
func (t *Theme) DrawBox(content string, width int) string {
	return t.Box.Width(width).Render(content)
}

// DrawHorizontalLine draws a horizontal line.
func (t *Theme) DrawHorizontalLine(width int) string {
	line := ""
	for i := 0; i < width; i++ {
		line += BoxHorizontal
	}
	return t.Secondary.Render(line)
}

// DrawDoubleLine draws a double horizontal line.
func (t *Theme) DrawDoubleLine(width int) string {
	line := ""
	for i := 0; i < width; i++ {
		line += BoxDoubleHorizontal
	}
	return t.Primary.Render(line)
}
