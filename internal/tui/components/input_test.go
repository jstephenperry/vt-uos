package components

import (
	"strings"
	"testing"
)

func TestInput_BasicOperations(t *testing.T) {
	input := NewInput("Name")
	input.SetValue("Alice")

	if input.Value() != "Alice" {
		t.Errorf("Expected 'Alice', got %q", input.Value())
	}

	input.SetWidth(30)
	input.SetMaxLength(50)
	input.SetRequired(true)
	input.SetPlaceholder("Enter name")

	if !input.Validate() {
		t.Error("Expected validation to pass with value set")
	}
}

func TestInput_RequiredValidation(t *testing.T) {
	input := NewInput("Name").SetRequired(true)

	// Empty value should fail
	if input.Validate() {
		t.Error("Expected validation to fail for empty required field")
	}

	// With value should pass
	input.SetValue("Alice")
	if !input.Validate() {
		t.Error("Expected validation to pass with value set")
	}

	// Whitespace-only should fail
	input.SetValue("   ")
	if input.Validate() {
		t.Error("Expected validation to fail for whitespace-only required field")
	}
}

func TestInput_Focus(t *testing.T) {
	input := NewInput("Name")

	if input.IsFocused() {
		t.Error("Should not be focused initially")
	}

	input.Focus(true)
	if !input.IsFocused() {
		t.Error("Should be focused after Focus(true)")
	}

	input.Focus(false)
	if input.IsFocused() {
		t.Error("Should not be focused after Focus(false)")
	}
}

func TestInput_HandleKey_TypeCharacter(t *testing.T) {
	input := NewInput("Name")
	input.Focus(true)

	input.HandleKey("A")
	input.HandleKey("B")
	input.HandleKey("C")

	if input.Value() != "ABC" {
		t.Errorf("Expected 'ABC', got %q", input.Value())
	}
}

func TestInput_HandleKey_Backspace(t *testing.T) {
	input := NewInput("Name")
	input.SetValue("Hello")
	input.Focus(true)

	input.HandleKey("backspace")
	if input.Value() != "Hell" {
		t.Errorf("Expected 'Hell', got %q", input.Value())
	}
}

func TestInput_HandleKey_CursorMovement(t *testing.T) {
	input := NewInput("Name")
	input.SetValue("Hello")
	input.Focus(true)

	// Cursor at end (5), move left
	input.HandleKey("left")
	// Now at 4, type a char
	input.HandleKey("X")
	if input.Value() != "HellXo" {
		t.Errorf("Expected 'HellXo', got %q", input.Value())
	}

	// Home
	input.HandleKey("home")
	input.HandleKey("Y")
	if input.Value() != "YHellXo" {
		t.Errorf("Expected 'YHellXo', got %q", input.Value())
	}
}

func TestInput_HandleKey_NotFocused(t *testing.T) {
	input := NewInput("Name")
	input.SetValue("Hello")
	// Not focused

	input.HandleKey("A")
	if input.Value() != "Hello" {
		t.Errorf("Should not handle keys when not focused, got %q", input.Value())
	}
}

func TestInput_Render_ShowsLabel(t *testing.T) {
	input := NewInput("Username")
	input.SetValue("admin")

	output := input.Render()
	if !strings.Contains(output, "Username") {
		t.Error("Expected label 'Username' in output")
	}
	if !strings.Contains(output, "admin") {
		t.Error("Expected value 'admin' in output")
	}
}

func TestInput_RenderWithLabelWidth_ZeroHidesLabel(t *testing.T) {
	input := NewInput("Username")
	input.SetValue("admin")

	output := input.RenderWithLabelWidth(0)
	// With labelWidth=0, the label should be omitted
	if strings.Contains(output, "Username") {
		t.Error("Expected label to be hidden with labelWidth=0")
	}
	if !strings.Contains(output, "admin") {
		t.Error("Expected value 'admin' in output")
	}
}

func TestInput_RenderWithLabelWidth_Custom(t *testing.T) {
	input := NewInput("Name")
	input.SetValue("Alice")

	output := input.RenderWithLabelWidth(12)
	if !strings.Contains(output, "Name") {
		t.Error("Expected label in output")
	}
}

func TestInput_Render_ShowsPlaceholder(t *testing.T) {
	input := NewInput("Name").SetPlaceholder("Enter name")

	output := input.Render()
	if !strings.Contains(output, "Enter name") {
		t.Error("Expected placeholder in output when unfocused and empty")
	}
}

func TestInput_Render_ShowsCursor(t *testing.T) {
	input := NewInput("Name")
	input.SetValue("Hi")
	input.Focus(true)

	output := input.Render()
	if !strings.Contains(output, "_") {
		t.Error("Expected cursor '_' in focused input output")
	}
}

func TestSelect_BasicOperations(t *testing.T) {
	sel := NewSelect("Color", []string{"Red", "Green", "Blue"})

	if sel.Value() != "Red" {
		t.Errorf("Expected 'Red', got %q", sel.Value())
	}
	if sel.SelectedIndex() != 0 {
		t.Errorf("Expected index 0, got %d", sel.SelectedIndex())
	}

	sel.SetSelected(2)
	if sel.Value() != "Blue" {
		t.Errorf("Expected 'Blue', got %q", sel.Value())
	}
}

func TestSelect_HandleKey(t *testing.T) {
	sel := NewSelect("Color", []string{"Red", "Green", "Blue"})
	sel.Focus(true)

	// Move right
	sel.HandleKey("right")
	if sel.Value() != "Green" {
		t.Errorf("Expected 'Green', got %q", sel.Value())
	}

	sel.HandleKey("right")
	if sel.Value() != "Blue" {
		t.Errorf("Expected 'Blue', got %q", sel.Value())
	}

	// Can't move beyond last
	sel.HandleKey("right")
	if sel.Value() != "Blue" {
		t.Errorf("Expected 'Blue', got %q", sel.Value())
	}

	// Move left
	sel.HandleKey("left")
	if sel.Value() != "Green" {
		t.Errorf("Expected 'Green', got %q", sel.Value())
	}
}

func TestSelect_HandleKey_NotFocused(t *testing.T) {
	sel := NewSelect("Color", []string{"Red", "Green", "Blue"})
	// Not focused

	sel.HandleKey("right")
	if sel.Value() != "Red" {
		t.Errorf("Should not handle keys when not focused, got %q", sel.Value())
	}
}

func TestSelect_Render(t *testing.T) {
	sel := NewSelect("Color", []string{"Red", "Green", "Blue"})
	sel.SetSelected(1)

	output := sel.Render()
	if !strings.Contains(output, "Color") {
		t.Error("Expected label 'Color' in output")
	}
	if !strings.Contains(output, "Green") {
		t.Error("Expected selected option 'Green' in output")
	}
}

func TestSelect_RenderWithLabelWidth(t *testing.T) {
	sel := NewSelect("Color", []string{"Red", "Green"})

	output := sel.RenderWithLabelWidth(10)
	if !strings.Contains(output, "Color") {
		t.Error("Expected label in output")
	}
}

func TestSelect_SetSelected_OutOfBounds(t *testing.T) {
	sel := NewSelect("Color", []string{"Red", "Green"})

	sel.SetSelected(-1)
	if sel.SelectedIndex() != 0 {
		t.Errorf("Expected index 0 after invalid SetSelected(-1), got %d", sel.SelectedIndex())
	}

	sel.SetSelected(99)
	if sel.SelectedIndex() != 0 {
		t.Errorf("Expected index 0 after invalid SetSelected(99), got %d", sel.SelectedIndex())
	}
}

func TestForm_BasicFlow(t *testing.T) {
	form := NewForm("Test Form")

	input1 := NewInput("Field1")
	input2 := NewInput("Field2")
	form.AddField(input1)
	form.AddField(input2)

	if form.IsSubmitted() {
		t.Error("Should not be submitted initially")
	}
	if form.IsCancelled() {
		t.Error("Should not be cancelled initially")
	}

	// First field should be focused
	if !input1.IsFocused() {
		t.Error("First field should be focused")
	}

	// Tab to next
	form.HandleKey("tab")
	if !input2.IsFocused() {
		t.Error("Second field should be focused after tab")
	}
	if input1.IsFocused() {
		t.Error("First field should not be focused after tab")
	}

	// Submit
	form.HandleKey("ctrl+s")
	if !form.IsSubmitted() {
		t.Error("Form should be submitted after Ctrl+S")
	}
}

func TestForm_Cancel(t *testing.T) {
	form := NewForm("Test")
	form.AddField(NewInput("Field"))

	form.HandleKey("esc")
	if !form.IsCancelled() {
		t.Error("Form should be cancelled after Esc")
	}
}

func TestForm_Render(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField(NewInput("Name").SetValue("Alice"))

	output := form.Render()
	if !strings.Contains(output, "Test Form") {
		t.Error("Expected title in form output")
	}
	if !strings.Contains(output, "Name") {
		t.Error("Expected field label in form output")
	}
}

func TestForm_RenderResponsive(t *testing.T) {
	form := NewForm("Test Form")
	form.AddField(NewInput("Name").SetValue("Alice"))

	// Wide
	wide := form.RenderResponsive(120)
	if !strings.Contains(wide, "Shift+Tab") {
		t.Error("Expected full help text on wide terminal")
	}

	// Narrow
	narrow := form.RenderResponsive(50)
	if strings.Contains(narrow, "Shift+Tab") {
		t.Error("Expected compact help text on narrow terminal")
	}
}

func TestForm_SetError(t *testing.T) {
	form := NewForm("Test")
	form.AddField(NewInput("Field"))
	form.SetError("Something went wrong")

	output := form.Render()
	if !strings.Contains(output, "Something went wrong") {
		t.Error("Expected error message in form output")
	}
}
