package tui

// themeStyler adapts the TUI Theme to the minimal Styler interface required by
// the simulation control view, keeping that view free of a dependency on this
// package.
type themeStyler struct{ t *Theme }

func (s themeStyler) Title(x string) string    { return s.t.Title.Render(x) }
func (s themeStyler) Subtitle(x string) string { return s.t.Subtitle.Render(x) }
func (s themeStyler) Label(x string) string    { return s.t.Label.Render(x) }
func (s themeStyler) Value(x string) string    { return s.t.Value.Render(x) }
func (s themeStyler) Muted(x string) string    { return s.t.Muted.Render(x) }
func (s themeStyler) Success(x string) string  { return s.t.Success.Render(x) }
func (s themeStyler) Warning(x string) string  { return s.t.Warning.Render(x) }
func (s themeStyler) Error(x string) string    { return s.t.Error.Render(x) }
func (s themeStyler) Accent(x string) string   { return s.t.Accent.Render(x) }

func (s themeStyler) Bar(value, max float64, width int) string {
	return s.t.ProgressBar(value, max, width)
}
