// Package simulation provides the operator control surface for the simulation
// control core within the terminal UI. It is a render-only component: the host
// application owns the engine and routes key presses to it, while this view
// presents live operational telemetry, the event feed, and active alerts.
package simulation

import (
	"fmt"
	"strings"

	"github.com/vtuos/vtuos/internal/protocol"
)

// Styler is the minimal styling surface this view needs from the host theme.
// It is satisfied by the TUI theme, avoiding an import cycle.
type Styler interface {
	Title(string) string
	Subtitle(string) string
	Label(string) string
	Value(string) string
	Muted(string) string
	Success(string) string
	Warning(string) string
	Error(string) string
	Accent(string) string
	Bar(value, max float64, width int) string
}

// ControlView renders the simulation control console.
type ControlView struct{}

// NewControlView constructs the control view.
func NewControlView() *ControlView { return &ControlView{} }

// Render draws the control console from the supplied operational snapshot,
// recent events and active alerts.
func (v *ControlView) Render(s Styler, st protocol.VaultState, events []protocol.EventRecord, alerts []protocol.Alert, width, height int) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder

	b.WriteString(s.Title("═══ SIMULATION CONTROL CORE ═══"))
	b.WriteString("\n\n")

	// Status line.
	statusStr := renderStatus(s, st.Status)
	b.WriteString(fmt.Sprintf("  %s   %s   %s\n",
		s.Label("STATUS:")+" "+statusStr,
		s.Label("SCALE:")+" "+s.Value(formatScale(st.TimeScale)),
		s.Label("TICK:")+" "+s.Value(fmt.Sprintf("%d", st.TickCount)),
	))
	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		s.Label("VAULT TIME:"), s.Value(st.VaultTime.Format("2077-01-02 15:04")),
		s.Label("ELAPSED:"), s.Value(fmt.Sprintf("%d years, %d days", st.ElapsedYears, st.ElapsedDays%365)),
	))
	b.WriteString("\n")

	// Controls legend (operator actions handled by the host).
	b.WriteString(s.Muted("  [Space] play/pause   [+/-] time scale   [s] step 1 day   [h] step 1 hour   [k] snapshot   [a] ack alerts"))
	b.WriteString("\n\n")

	// Two-column operational summary.
	left := renderOpsSummary(s, st)
	right := renderPower(s, st) + "\n" + renderResourceRunways(s, st)
	b.WriteString(joinColumns(left, right, width))
	b.WriteString("\n")

	// Active alerts.
	b.WriteString(s.Subtitle("ACTIVE ALERTS"))
	b.WriteString("\n")
	if len(alerts) == 0 {
		b.WriteString(s.Success("  ✓ No active alerts — all systems nominal"))
		b.WriteString("\n")
	} else {
		shown := alerts
		if len(shown) > 5 {
			shown = shown[:5]
		}
		for _, a := range shown {
			b.WriteString("  " + renderAlertLine(s, a) + "\n")
		}
	}
	b.WriteString("\n")

	// Operational event feed.
	b.WriteString(s.Subtitle("OPERATIONAL EVENT LOG"))
	b.WriteString("\n")
	if len(events) == 0 {
		b.WriteString(s.Muted("  No events recorded yet. Advance time to begin operations."))
		b.WriteString("\n")
	} else {
		max := 8
		if max > len(events) {
			max = len(events)
		}
		for i := 0; i < max; i++ {
			b.WriteString("  " + renderEventLine(s, events[i], width-4) + "\n")
		}
	}

	return b.String()
}

func renderStatus(s Styler, st protocol.SimStatus) string {
	switch st {
	case protocol.SimRunning:
		return s.Success("● RUNNING")
	case protocol.SimPaused:
		return s.Warning("‖ PAUSED")
	default:
		return s.Muted("■ STOPPED")
	}
}

func renderOpsSummary(s Styler, st protocol.VaultState) string {
	var b strings.Builder
	b.WriteString(s.Subtitle("POPULATION & SYSTEMS"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Active:      %s / %s  (%.0f%%)\n",
		s.Value(fmt.Sprintf("%d", st.Population.Active)),
		s.Muted(fmt.Sprintf("%d", st.Population.Capacity)),
		st.Population.LoadPct))
	b.WriteString(fmt.Sprintf("  Births/Deaths: %s / %s\n",
		s.Success(fmt.Sprintf("%d", st.Population.Births)),
		s.Error(fmt.Sprintf("%d", st.Population.Deaths))))
	if st.Population.Quarantined > 0 {
		b.WriteString(fmt.Sprintf("  Quarantine:  %s\n", s.Warning(fmt.Sprintf("%d", st.Population.Quarantined))))
	}
	b.WriteString(fmt.Sprintf("  Avg Age:     %s\n", s.Value(fmt.Sprintf("%.1f", st.Population.AverageAge))))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Systems:     %s op  %s deg  %s fail\n",
		s.Success(fmt.Sprintf("%d", st.Systems.Operational)),
		s.Warning(fmt.Sprintf("%d", st.Systems.Degraded)),
		s.Error(fmt.Sprintf("%d", st.Systems.Failed))))
	b.WriteString(fmt.Sprintf("  Avg Eff:     %s  ",
		s.Value(fmt.Sprintf("%.0f%%", st.Systems.AvgEfficiency))))
	b.WriteString(s.Bar(st.Systems.AvgEfficiency, 100, 16))
	b.WriteString("\n")
	if st.Systems.OverdueMaint > 0 {
		b.WriteString(fmt.Sprintf("  Overdue PM:  %s\n", s.Warning(fmt.Sprintf("%d", st.Systems.OverdueMaint))))
	}
	return b.String()
}

func renderPower(s Styler, st protocol.VaultState) string {
	var b strings.Builder
	b.WriteString(s.Subtitle("POWER BALANCE"))
	b.WriteString("\n")
	balStyle := s.Success
	if st.Power.BalanceKW < 0 {
		balStyle = s.Error
	} else if st.Power.ReservePct < 10 {
		balStyle = s.Warning
	}
	b.WriteString(fmt.Sprintf("  Gen: %s kW   Load: %s kW\n",
		s.Value(fmt.Sprintf("%.0f", st.Power.GenerationKW)),
		s.Value(fmt.Sprintf("%.0f", st.Power.ConsumptionKW))))
	b.WriteString(fmt.Sprintf("  Balance: %s kW  (reserve %s)\n",
		balStyle(fmt.Sprintf("%+.0f", st.Power.BalanceKW)),
		balStyle(fmt.Sprintf("%.0f%%", st.Power.ReservePct))))
	return b.String()
}

func renderResourceRunways(s Styler, st protocol.VaultState) string {
	var b strings.Builder
	b.WriteString(s.Subtitle("RESOURCE RUNWAY"))
	b.WriteString("\n")
	for _, r := range st.Resources {
		if r.DailyUse <= 0 {
			continue
		}
		runway := "∞"
		if r.RunwayDays >= 0 {
			runway = fmt.Sprintf("%dd", r.RunwayDays)
		}
		st := statusStyle(s, r.Status)
		b.WriteString(fmt.Sprintf("  %-10s %s\n", r.Label, st(runway+"  ("+r.Status+")")))
	}
	return b.String()
}

func renderAlertLine(s Styler, a protocol.Alert) string {
	tag := statusStyleByLevel(s, a.Level)
	ack := ""
	if a.Acknowledged {
		ack = s.Muted(" [ack]")
	}
	return tag("["+string(a.Level)+"]") + " " + a.Message + ack
}

func renderEventLine(s Styler, e protocol.EventRecord, width int) string {
	ts := e.VaultTime.Format("01-02 15:04")
	line := fmt.Sprintf("%s  %-10s %s", s.Muted(ts), e.Category, e.Summary)
	style := statusStyleByLevel(s, e.Severity)
	if e.Severity == protocol.AlertInfo {
		return line
	}
	return style(line)
}

func statusStyle(s Styler, status string) func(string) string {
	switch status {
	case "CRITICAL":
		return s.Error
	case "WARNING":
		return s.Warning
	case "WATCH":
		return s.Accent
	default:
		return s.Success
	}
}

func statusStyleByLevel(s Styler, level protocol.AlertLevel) func(string) string {
	switch level {
	case protocol.AlertCritical:
		return s.Error
	case protocol.AlertWarning:
		return s.Warning
	default:
		return s.Accent
	}
}

func formatScale(scale float64) string {
	if scale == 0 {
		return "frozen"
	}
	return fmt.Sprintf("%.0fx", scale)
}

// joinColumns places two multi-line blocks side by side within width.
func joinColumns(left, right string, width int) string {
	half := width / 2
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	n := len(leftLines)
	if len(rightLines) > n {
		n = len(rightLines)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		pad := half - displayWidth(l)
		if pad < 1 {
			pad = 1
		}
		b.WriteString(l)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(r)
		b.WriteString("\n")
	}
	return b.String()
}

// displayWidth approximates rendered width, ignoring ANSI escape sequences.
func displayWidth(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case inEscape:
			// skip
		default:
			n++
		}
	}
	return n
}
