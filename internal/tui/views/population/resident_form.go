package population

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/tui/components"
)

// FormMode indicates the form mode.
type FormMode int

const (
	FormModeAdd FormMode = iota
	FormModeEdit
)

// ResidentForm is a form for adding/editing residents.
type ResidentForm struct {
	mode     FormMode
	resident *models.Resident

	// Form fields
	surname    *components.Input
	givenNames *components.Input
	dobYear    *components.Input
	dobMonth   *components.Input
	dobDay     *components.Input
	sex        *components.Select
	bloodType  *components.Select
	entryType  *components.Select
	clearance  *components.Input
	notes      *components.Input

	// State
	focusIndex int
	fields     []components.FormField
	submitted  bool
	cancelled  bool
	err        string
}

// NewResidentForm creates a new resident form.
func NewResidentForm(mode FormMode) *ResidentForm {
	f := &ResidentForm{
		mode: mode,

		surname:    components.NewInput("Surname").SetRequired(true).SetWidth(25),
		givenNames: components.NewInput("Given Names").SetRequired(true).SetWidth(25),
		dobYear:    components.NewInput("Birth Year").SetRequired(true).SetWidth(6).SetMaxLength(4).SetPlaceholder("YYYY"),
		dobMonth:   components.NewInput("Month").SetRequired(true).SetWidth(4).SetMaxLength(2).SetPlaceholder("MM"),
		dobDay:     components.NewInput("Day").SetRequired(true).SetWidth(4).SetMaxLength(2).SetPlaceholder("DD"),
		sex:        components.NewSelect("Sex", []string{"M", "F"}),
		bloodType:  components.NewSelect("Blood Type", []string{"A+", "A-", "B+", "B-", "AB+", "AB-", "O+", "O-", "-"}),
		entryType:  components.NewSelect("Entry Type", []string{"ORIGINAL", "VAULT_BORN", "ADMITTED"}),
		clearance:  components.NewInput("Clearance").SetWidth(4).SetMaxLength(2).SetValue("1"),
		notes:      components.NewInput("Notes").SetWidth(40),
	}

	// Build fields list
	f.fields = []components.FormField{
		f.surname,
		f.givenNames,
		f.dobYear,
		f.dobMonth,
		f.dobDay,
		f.sex,
		f.bloodType,
		f.entryType,
		f.clearance,
		f.notes,
	}

	// Focus first field
	f.fields[0].Focus(true)

	return f
}

// SetResident populates the form with existing resident data.
func (f *ResidentForm) SetResident(r *models.Resident) {
	f.resident = r
	f.surname.SetValue(r.Surname)
	f.givenNames.SetValue(r.GivenNames)
	f.dobYear.SetValue(fmt.Sprintf("%d", r.DateOfBirth.Year()))
	f.dobMonth.SetValue(fmt.Sprintf("%02d", r.DateOfBirth.Month()))
	f.dobDay.SetValue(fmt.Sprintf("%02d", r.DateOfBirth.Day()))

	switch r.Sex {
	case models.SexMale:
		f.sex.SetSelected(0)
	case models.SexFemale:
		f.sex.SetSelected(1)
	}

	// Set blood type
	bloodTypes := []string{"A+", "A-", "B+", "B-", "AB+", "AB-", "O+", "O-"}
	for i, bt := range bloodTypes {
		if bt == string(r.BloodType) {
			f.bloodType.SetSelected(i)
			break
		}
	}

	// Set entry type
	entryTypes := []string{"ORIGINAL", "VAULT_BORN", "ADMITTED"}
	for i, et := range entryTypes {
		if et == string(r.EntryType) {
			f.entryType.SetSelected(i)
			break
		}
	}

	f.clearance.SetValue(fmt.Sprintf("%d", r.ClearanceLevel))
	f.notes.SetValue(r.Notes)
}

// HandleKey handles key input.
func (f *ResidentForm) HandleKey(key string) {
	switch key {
	case "tab", "down":
		f.nextField()
	case "shift+tab", "up":
		f.prevField()
	case "ctrl+s":
		f.submit()
	case "esc":
		f.cancelled = true
	case "enter":
		// Move to next field, or submit on last field
		if f.focusIndex == len(f.fields)-1 {
			f.submit()
		} else {
			f.nextField()
		}
	default:
		f.fields[f.focusIndex].HandleKey(key)
	}
}

func (f *ResidentForm) nextField() {
	f.fields[f.focusIndex].Focus(false)
	f.focusIndex++
	if f.focusIndex >= len(f.fields) {
		f.focusIndex = 0
	}
	f.fields[f.focusIndex].Focus(true)
}

func (f *ResidentForm) prevField() {
	f.fields[f.focusIndex].Focus(false)
	f.focusIndex--
	if f.focusIndex < 0 {
		f.focusIndex = len(f.fields) - 1
	}
	f.fields[f.focusIndex].Focus(true)
}

func (f *ResidentForm) submit() {
	f.err = ""

	// Validate required fields
	valid := true
	if !f.surname.Validate() {
		valid = false
	}
	if !f.givenNames.Validate() {
		valid = false
	}

	// Validate date
	year := f.dobYear.Value()
	month := f.dobMonth.Value()
	day := f.dobDay.Value()
	dateStr := fmt.Sprintf("%s-%s-%s", year, month, day)
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		f.err = "Invalid date of birth"
		valid = false
	}

	if !valid {
		if f.err == "" {
			f.err = "Please fill in all required fields"
		}
		return
	}

	f.submitted = true
}

// IsSubmitted returns true if the form was submitted.
func (f *ResidentForm) IsSubmitted() bool {
	return f.submitted
}

// IsCancelled returns true if the form was cancelled.
func (f *ResidentForm) IsCancelled() bool {
	return f.cancelled
}

// GetData returns the form data as a resident struct.
func (f *ResidentForm) GetData() (*models.Resident, error) {
	// Parse date
	dateStr := fmt.Sprintf("%s-%s-%s",
		f.dobYear.Value(),
		f.dobMonth.Value(),
		f.dobDay.Value())
	dob, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %w", err)
	}

	// Parse clearance
	var clearance int
	fmt.Sscanf(f.clearance.Value(), "%d", &clearance)
	if clearance < 1 {
		clearance = 1
	}
	if clearance > 10 {
		clearance = 10
	}

	// Get sex
	sex := models.SexMale
	if f.sex.SelectedIndex() == 1 {
		sex = models.SexFemale
	}

	// Get blood type
	bloodType := models.BloodType(f.bloodType.Value())
	if bloodType == "-" {
		bloodType = ""
	}

	// Get entry type
	entryType := models.EntryType(f.entryType.Value())

	r := &models.Resident{
		Surname:        f.surname.Value(),
		GivenNames:     f.givenNames.Value(),
		DateOfBirth:    dob,
		Sex:            sex,
		BloodType:      bloodType,
		EntryType:      entryType,
		ClearanceLevel: clearance,
		Notes:          f.notes.Value(),
	}

	// Copy ID if editing
	if f.resident != nil {
		r.ID = f.resident.ID
		r.RegistryNumber = f.resident.RegistryNumber
		r.EntryDate = f.resident.EntryDate
		r.Status = f.resident.Status
		r.BiologicalParent1ID = f.resident.BiologicalParent1ID
		r.BiologicalParent2ID = f.resident.BiologicalParent2ID
		r.HouseholdID = f.resident.HouseholdID
		r.QuartersID = f.resident.QuartersID
		r.PrimaryVocationID = f.resident.PrimaryVocationID
	}

	return r, nil
}

// Render renders the form with default width.
func (f *ResidentForm) Render() string {
	return f.RenderResponsive(0)
}

// RenderResponsive renders the form adapted to the given terminal width.
func (f *ResidentForm) RenderResponsive(width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))

	// Adapt label width to terminal
	labelWidth := 16
	if width > 0 && width < 60 {
		labelWidth = 10
	}

	var b strings.Builder

	// Title
	title := "ADD RESIDENT"
	if f.mode == FormModeEdit {
		title = "EDIT RESIDENT"
	}
	b.WriteString(titleStyle.Render("═══ " + title + " ═══"))
	b.WriteString("\n\n")

	// Name fields
	b.WriteString(f.surname.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n")
	b.WriteString(f.givenNames.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n\n")

	// Date of birth - adapt layout for narrow terminals
	dobLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Width(labelWidth)
	if width > 0 && width < 60 {
		b.WriteString(dobLabel.Render("DOB:"))
	} else {
		b.WriteString(dobLabel.Render("Date of Birth:"))
	}
	b.WriteString(" ")
	b.WriteString(f.dobYear.RenderWithLabelWidth(0))
	b.WriteString(" - ")
	b.WriteString(f.dobMonth.RenderWithLabelWidth(0))
	b.WriteString(" - ")
	b.WriteString(f.dobDay.RenderWithLabelWidth(0))
	b.WriteString("\n\n")

	// Selects
	b.WriteString(f.sex.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n")
	b.WriteString(f.bloodType.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n")
	b.WriteString(f.entryType.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n\n")

	// Other fields
	b.WriteString(f.clearance.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n")
	b.WriteString(f.notes.RenderWithLabelWidth(labelWidth))
	b.WriteString("\n")

	// Error message
	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("Error: " + f.err))
	}

	// Help - adapt to width
	b.WriteString("\n\n")
	_ = labelStyle
	if width > 0 && width < 60 {
		b.WriteString(helpStyle.Render("Tab:Next  Ctrl+S:Save  Esc:Cancel"))
	} else {
		b.WriteString(helpStyle.Render("Tab/Down:Next  Shift+Tab/Up:Prev  Ctrl+S:Save  Esc:Cancel"))
	}

	return b.String()
}
