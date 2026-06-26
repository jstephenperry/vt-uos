package client

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/protocol"
)

func (m *model) View() string {
	if m.width == 0 {
		return "Connecting to master server…"
	}
	if m.quitting {
		return m.theme.Title.Render("Terminal disconnecting…")
	}

	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(m.theme.DrawDoubleLine(m.width))
	b.WriteString("\n")

	if m.banner != "" {
		banner := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(m.theme.Accent.Bold(true).Render(m.banner))
		b.WriteString(banner)
		b.WriteString("\n\n")
	}

	if !m.haveState {
		b.WriteString(m.theme.Muted.Render("  Awaiting operational state from master…"))
	} else {
		switch m.page {
		case "systems":
			b.WriteString(m.renderSystems())
		case "resources":
			b.WriteString(m.renderResources())
		case "events":
			b.WriteString(m.renderEvents())
		case "population":
			b.WriteString(m.renderPopulation())
		default:
			b.WriteString(m.renderDashboard())
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

func (m *model) renderHeader() string {
	conn := m.theme.Error.Render("● OFFLINE")
	if m.connected {
		conn = m.theme.Success.Render("● LIVE")
	}
	st := m.statusBadge()
	left := m.theme.Header.Render("VT-UOS DISPLAY · " + m.vault)
	right := fmt.Sprintf("%s  %s  %s", st, m.theme.Value.Render(vaultTime(m.state)), conn)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *model) statusBadge() string {
	switch m.state.Status {
	case protocol.SimRunning:
		return m.theme.Success.Render("RUNNING")
	case protocol.SimPaused:
		return m.theme.Warning.Render("PAUSED")
	default:
		return m.theme.Muted.Render("STOPPED")
	}
}

func vaultTime(st protocol.VaultState) string {
	return st.VaultTime.Format("2006-01-02 15:04")
}

func (m *model) renderDashboard() string {
	pop := m.panel("POPULATION", m.popBody(), m.colWidth(3))
	pwr := m.panel("POWER", m.powerBody(), m.colWidth(3))
	sys := m.panel("SYSTEMS", m.sysSummaryBody(), m.colWidth(3))
	top := lipgloss.JoinHorizontal(lipgloss.Top, pop, " ", pwr, " ", sys)

	res := m.panel("RESOURCE RUNWAY", m.resourceBody(), m.colWidth(2))
	alr := m.panel("ACTIVE ALERTS", m.alertsBody(), m.colWidth(2))
	mid := lipgloss.JoinHorizontal(lipgloss.Top, res, " ", alr)

	ev := m.panel("OPERATIONAL EVENT LOG", m.eventsBody(6), m.width-4)

	return top + "\n" + mid + "\n" + ev
}

func (m *model) colWidth(cols int) int {
	w := (m.width - (cols+1)*1) / cols
	if w < 20 {
		w = 20
	}
	return w
}

func (m *model) panel(title, body string, width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.SecondaryColor).
		Width(width-2).
		Padding(0, 1)
	header := m.theme.Accent.Bold(true).Render(title) + "\n"
	return style.Render(header + body)
}

func (m *model) popBody() string {
	p := m.state.Population
	return fmt.Sprintf("Active:  %s / %s\nLoad:    %s\nBirths:  %s   Deaths: %s\nAvg age: %s",
		m.theme.Value.Render(itoa(p.Active)), m.theme.Muted.Render(itoa(p.Capacity)),
		m.theme.Value.Render(fmt.Sprintf("%.0f%%", p.LoadPct)),
		m.theme.Success.Render(itoa(p.Births)), m.theme.Error.Render(itoa(p.Deaths)),
		m.theme.Value.Render(fmt.Sprintf("%.1f", p.AverageAge)))
}

func (m *model) powerBody() string {
	pw := m.state.Power
	balStyle := m.theme.Success
	if pw.BalanceKW < 0 {
		balStyle = m.theme.Error
	} else if pw.ReservePct < 10 {
		balStyle = m.theme.Warning
	}
	return fmt.Sprintf("Gen:     %s kW\nLoad:    %s kW\nBalance: %s\nReserve: %s",
		m.theme.Value.Render(fmt.Sprintf("%.0f", pw.GenerationKW)),
		m.theme.Value.Render(fmt.Sprintf("%.0f", pw.ConsumptionKW)),
		balStyle.Render(fmt.Sprintf("%+.0f kW", pw.BalanceKW)),
		balStyle.Render(fmt.Sprintf("%.0f%%", pw.ReservePct)))
}

func (m *model) sysSummaryBody() string {
	s := m.state.Systems
	return fmt.Sprintf("Operational: %s\nDegraded:    %s\nFailed:      %s\nAvg eff:     %s\nOverdue PM:  %s",
		m.theme.Success.Render(itoa(s.Operational)),
		m.theme.Warning.Render(itoa(s.Degraded)),
		m.theme.Error.Render(itoa(s.Failed)),
		m.theme.Value.Render(fmt.Sprintf("%.0f%%", s.AvgEfficiency)),
		m.theme.Warning.Render(itoa(s.OverdueMaint)))
}

func (m *model) resourceBody() string {
	var b strings.Builder
	any := false
	for _, r := range m.state.Resources {
		if r.DailyUse <= 0 {
			continue
		}
		any = true
		runway := "∞"
		if r.RunwayDays >= 0 {
			runway = itoa(r.RunwayDays) + "d"
		}
		b.WriteString(fmt.Sprintf("%-10s %s\n", r.Label, m.statusStyle(r.Status).Render(runway+" ("+r.Status+")")))
	}
	if !any {
		return m.theme.Muted.Render("No metered consumption yet.")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) alertsBody() string {
	if len(m.state.Alerts) == 0 {
		return m.theme.Success.Render("✓ All systems nominal")
	}
	var b strings.Builder
	for i, a := range m.state.Alerts {
		if i >= 4 {
			break
		}
		style := m.theme.Warning
		if a.Level == protocol.AlertCritical {
			style = m.theme.Error
		}
		b.WriteString(style.Render("["+string(a.Level)+"] ") + a.Message + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) eventsBody(n int) string {
	if len(m.state.RecentEvents) == 0 {
		return m.theme.Muted.Render("No events recorded.")
	}
	var b strings.Builder
	for i, e := range m.state.RecentEvents {
		if i >= n {
			break
		}
		ts := e.VaultTime.Format("01-02 15:04")
		line := fmt.Sprintf("%s  %-10s %s", m.theme.Muted.Render(ts), e.Category, e.Summary)
		if e.Severity == protocol.AlertCritical {
			line = m.theme.Error.Render(line)
		} else if e.Severity == protocol.AlertWarning {
			line = m.theme.Warning.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) renderSystems() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("FACILITY SYSTEMS"))
	b.WriteString("\n\n")
	for _, s := range m.state.SystemList {
		style := m.theme.Success
		if s.Efficiency < 50 {
			style = m.theme.Error
		} else if s.Efficiency < 80 {
			style = m.theme.Warning
		}
		b.WriteString(fmt.Sprintf("  %-14s %-22s %s %s\n",
			s.Code, s.Name, m.theme.ProgressBar(s.Efficiency, 100, 16),
			style.Render(fmt.Sprintf("%3.0f%% %s", s.Efficiency, s.Status))))
	}
	return b.String()
}

func (m *model) renderResources() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("RESOURCE RUNWAY"))
	b.WriteString("\n\n")
	for _, r := range m.state.Resources {
		runway := "∞"
		if r.RunwayDays >= 0 {
			runway = itoa(r.RunwayDays) + " days"
		}
		b.WriteString(fmt.Sprintf("  %-12s on hand %-12s use/day %-10s %s\n",
			r.Label, fmt.Sprintf("%.0f %s", r.OnHand, r.Unit), fmt.Sprintf("%.0f", r.DailyUse),
			m.statusStyle(r.Status).Render(runway)))
	}
	return b.String()
}

func (m *model) renderEvents() string {
	return m.theme.Title.Render("OPERATIONAL EVENT LOG") + "\n\n  " +
		strings.ReplaceAll(m.eventsBody(20), "\n", "\n  ")
}

func (m *model) renderPopulation() string {
	p := m.state.Population
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("POPULATION"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Active:        %s\n", m.theme.Value.Render(itoa(p.Active))))
	b.WriteString(fmt.Sprintf("  Capacity:      %s\n", m.theme.Muted.Render(itoa(p.Capacity))))
	b.WriteString(fmt.Sprintf("  Load:          %s\n", m.theme.Value.Render(fmt.Sprintf("%.0f%%", p.LoadPct))))
	b.WriteString(fmt.Sprintf("  Deceased:      %s\n", m.theme.Muted.Render(itoa(p.Deceased))))
	b.WriteString(fmt.Sprintf("  Births:        %s\n", m.theme.Success.Render(itoa(p.Births))))
	b.WriteString(fmt.Sprintf("  Deaths:        %s\n", m.theme.Error.Render(itoa(p.Deaths))))
	b.WriteString(fmt.Sprintf("  Quarantined:   %s\n", m.theme.Warning.Render(itoa(p.Quarantined))))
	b.WriteString(fmt.Sprintf("  Average age:   %s\n", m.theme.Value.Render(fmt.Sprintf("%.1f", p.AverageAge))))
	return b.String()
}

func (m *model) statusStyle(status string) lipgloss.Style {
	switch status {
	case "CRITICAL":
		return m.theme.Error
	case "WARNING", "WATCH":
		return m.theme.Warning
	default:
		return m.theme.Success
	}
}

func (m *model) renderFooter() string {
	sep := m.theme.DrawHorizontalLine(m.width)
	hint := "[1]Dashboard [2]Systems [3]Resources [4]Events [5]Population   MANAGED TERMINAL"
	if m.kiosk {
		hint = "KIOSK — display managed remotely by overseer console"
	}
	return sep + "\n" + m.theme.Footer.Render(hint)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
