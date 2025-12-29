package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Input is a simple text input component.
type Input struct {
	label       string
	value       string
	placeholder string
	width       int
	focused     bool
	cursorPos   int
	maxLength   int
	required    bool
	err         string
}

// NewInput creates a new input field.
func NewInput(label string) *Input {
	return &Input{
		label:     label,
		width:     20,
		maxLength: 100,
	}
}

// SetValue sets the input value.
func (i *Input) SetValue(v string) *Input {
	i.value = v
	i.cursorPos = len(v)
	return i
}

// SetPlaceholder sets the placeholder text.
func (i *Input) SetPlaceholder(p string) *Input {
	i.placeholder = p
	return i
}

// SetWidth sets the input width.
func (i *Input) SetWidth(w int) *Input {
	i.width = w
	return i
}

// SetMaxLength sets the maximum input length.
func (i *Input) SetMaxLength(m int) *Input {
	i.maxLength = m
	return i
}

// SetRequired marks the field as required.
func (i *Input) SetRequired(r bool) *Input {
	i.required = r
	return i
}

// SetError sets an error message.
func (i *Input) SetError(e string) *Input {
	i.err = e
	return i
}

// Focus sets the focus state.
func (i *Input) Focus(focused bool) {
	i.focused = focused
	if focused && i.cursorPos > len(i.value) {
		i.cursorPos = len(i.value)
	}
}

// IsFocused returns the focus state.
func (i *Input) IsFocused() bool {
	return i.focused
}

// Value returns the current value.
func (i *Input) Value() string {
	return i.value
}

// HandleKey handles a key press.
func (i *Input) HandleKey(key string) {
	if !i.focused {
		return
	}

	switch key {
	case "backspace":
		if len(i.value) > 0 && i.cursorPos > 0 {
			i.value = i.value[:i.cursorPos-1] + i.value[i.cursorPos:]
			i.cursorPos--
		}
	case "delete":
		if i.cursorPos < len(i.value) {
			i.value = i.value[:i.cursorPos] + i.value[i.cursorPos+1:]
		}
	case "left":
		if i.cursorPos > 0 {
			i.cursorPos--
		}
	case "right":
		if i.cursorPos < len(i.value) {
			i.cursorPos++
		}
	case "home", "ctrl+a":
		i.cursorPos = 0
	case "end", "ctrl+e":
		i.cursorPos = len(i.value)
	default:
		// Insert printable character
		if len(key) == 1 && len(i.value) < i.maxLength {
			i.value = i.value[:i.cursorPos] + key + i.value[i.cursorPos:]
			i.cursorPos++
		}
	}
}

// Validate validates the input.
func (i *Input) Validate() bool {
	if i.required && strings.TrimSpace(i.value) == "" {
		i.err = "Required"
		return false
	}
	i.err = ""
	return true
}

// Render renders the input field.
func (i *Input) Render() string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Width(16)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#006600"))

	// Build label
	label := i.label
	if i.required {
		label += "*"
	}
	label += ":"

	// Build value display
	var display string
	if i.value == "" && i.placeholder != "" && !i.focused {
		display = mutedStyle.Render(i.placeholder)
	} else if i.focused {
		// Show cursor
		before := i.value[:i.cursorPos]
		after := ""
		if i.cursorPos < len(i.value) {
			after = i.value[i.cursorPos:]
		}
		display = focusStyle.Render(before + "_" + after)
	} else {
		display = valueStyle.Render(i.value)
	}

	// Pad display to width
	displayLen := len(i.value)
	if i.focused {
		displayLen++ // cursor
	}
	if displayLen < i.width {
		display += strings.Repeat(" ", i.width-displayLen)
	}

	result := labelStyle.Render(label) + " " + display

	// Show error if present
	if i.err != "" {
		result += " " + errStyle.Render(i.err)
	}

	return result
}

// Select is a selection input component.
type Select struct {
	label    string
	options  []string
	selected int
	focused  bool
}

// NewSelect creates a new select input.
func NewSelect(label string, options []string) *Select {
	return &Select{
		label:   label,
		options: options,
	}
}

// SetSelected sets the selected index.
func (s *Select) SetSelected(idx int) *Select {
	if idx >= 0 && idx < len(s.options) {
		s.selected = idx
	}
	return s
}

// Focus sets the focus state.
func (s *Select) Focus(focused bool) {
	s.focused = focused
}

// IsFocused returns the focus state.
func (s *Select) IsFocused() bool {
	return s.focused
}

// Value returns the selected value.
func (s *Select) Value() string {
	if s.selected >= 0 && s.selected < len(s.options) {
		return s.options[s.selected]
	}
	return ""
}

// SelectedIndex returns the selected index.
func (s *Select) SelectedIndex() int {
	return s.selected
}

// HandleKey handles a key press.
func (s *Select) HandleKey(key string) {
	if !s.focused {
		return
	}

	switch key {
	case "left", "h":
		if s.selected > 0 {
			s.selected--
		}
	case "right", "l":
		if s.selected < len(s.options)-1 {
			s.selected++
		}
	}
}

// Render renders the select.
func (s *Select) Render() string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Width(16)
	optStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)

	var b strings.Builder
	b.WriteString(labelStyle.Render(s.label + ":"))
	b.WriteString(" ")

	for i, opt := range s.options {
		if i > 0 {
			b.WriteString(" ")
		}

		if i == s.selected {
			if s.focused {
				b.WriteString(selStyle.Render("[" + opt + "]"))
			} else {
				b.WriteString(selStyle.Render("(" + opt + ")"))
			}
		} else {
			b.WriteString(optStyle.Render(" " + opt + " "))
		}
	}

	return b.String()
}

// formField interface for form components
type formField interface {
	Focus(bool)
	IsFocused() bool
	HandleKey(string)
	Render() string
}

// Ensure Input and Select implement formField
var (
	_ formField = (*Input)(nil)
	_ formField = (*Select)(nil)
)

// FormField wraps components to satisfy formField interface
type FormField interface {
	Focus(bool)
	IsFocused() bool
	HandleKey(string)
	Render() string
}

// Form is a simple form container.
type Form struct {
	title      string
	fields     []FormField
	focusIndex int
	submitted  bool
	cancelled  bool
	err        string
}

// NewForm creates a new form.
func NewForm(title string) *Form {
	return &Form{
		title: title,
	}
}

// AddField adds a field to the form.
func (f *Form) AddField(field FormField) *Form {
	f.fields = append(f.fields, field)
	if len(f.fields) == 1 {
		field.Focus(true)
	}
	return f
}

// HandleKey handles form navigation.
func (f *Form) HandleKey(key string) {
	switch key {
	case "tab", "down":
		f.nextField()
	case "shift+tab", "up":
		f.prevField()
	case "ctrl+s":
		f.submitted = true
	case "esc":
		f.cancelled = true
	case "enter":
		// Move to next field on enter, or submit if on last field
		if f.focusIndex == len(f.fields)-1 {
			f.submitted = true
		} else {
			f.nextField()
		}
	default:
		if f.focusIndex < len(f.fields) {
			f.fields[f.focusIndex].HandleKey(key)
		}
	}
}

func (f *Form) nextField() {
	if len(f.fields) == 0 {
		return
	}
	f.fields[f.focusIndex].Focus(false)
	f.focusIndex = (f.focusIndex + 1) % len(f.fields)
	f.fields[f.focusIndex].Focus(true)
}

func (f *Form) prevField() {
	if len(f.fields) == 0 {
		return
	}
	f.fields[f.focusIndex].Focus(false)
	f.focusIndex--
	if f.focusIndex < 0 {
		f.focusIndex = len(f.fields) - 1
	}
	f.fields[f.focusIndex].Focus(true)
}

// IsSubmitted returns true if form was submitted.
func (f *Form) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if form was cancelled.
func (f *Form) IsCancelled() bool {
	return f.cancelled
}

// SetError sets an error message.
func (f *Form) SetError(err string) {
	f.err = err
}

// Render renders the form.
func (f *Form) Render() string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(fmt.Sprintf("=== %s ===", f.title)))
	b.WriteString("\n\n")

	// Fields
	for _, field := range f.fields {
		b.WriteString(field.Render())
		b.WriteString("\n")
	}

	// Error
	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("Error: " + f.err))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Tab/Down:Next  Shift+Tab/Up:Prev  Ctrl+S:Save  Esc:Cancel"))

	return b.String()
}
