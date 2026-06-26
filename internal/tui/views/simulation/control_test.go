package simulation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/protocol"
)

// plainStyler renders without any styling so tests can assert on content.
type plainStyler struct{}

func (plainStyler) Title(s string) string    { return s }
func (plainStyler) Subtitle(s string) string { return s }
func (plainStyler) Label(s string) string    { return s }
func (plainStyler) Value(s string) string    { return s }
func (plainStyler) Muted(s string) string    { return s }
func (plainStyler) Success(s string) string  { return s }
func (plainStyler) Warning(s string) string  { return s }
func (plainStyler) Error(s string) string    { return s }
func (plainStyler) Accent(s string) string   { return s }
func (plainStyler) Bar(v, m float64, w int) string {
	return fmt.Sprintf("[%.0f/%.0f]", v, m)
}

func sampleState() protocol.VaultState {
	now := time.Date(2080, 5, 1, 12, 0, 0, 0, time.UTC)
	return protocol.VaultState{
		SchemaVersion: protocol.Version,
		Status:        protocol.SimRunning,
		TimeScale:     60,
		VaultTime:     now,
		TickCount:     1234,
		ElapsedYears:  2,
		ElapsedDays:   190,
		Population: protocol.PopulationStatus{
			Active: 480, Capacity: 500, Births: 12, Deaths: 7, AverageAge: 31.4, LoadPct: 96,
		},
		Systems: protocol.SystemsSummary{
			Total: 15, Operational: 12, Degraded: 2, Failed: 1, AvgEfficiency: 84.2, OverdueMaint: 1,
		},
		Power: protocol.PowerStatus{GenerationKW: 500, ConsumptionKW: 320, BalanceKW: 180, ReservePct: 36},
		Resources: []protocol.ResourceStatus{
			{Category: "FOOD", Label: "Food", DailyUse: 120, RunwayDays: 95, Status: "WATCH"},
			{Category: "WATER", Label: "Water", DailyUse: 1440, RunwayDays: 25, Status: "WARNING"},
		},
	}
}

func TestControlViewRender(t *testing.T) {
	v := NewControlView()
	st := sampleState()
	alerts := []protocol.Alert{
		{Code: "POWER_RESERVE_LOW", Level: protocol.AlertWarning, Message: "Power reserve low"},
	}
	events := []protocol.EventRecord{
		{VaultTime: st.VaultTime, Category: "FACILITY", Type: "SYSTEM_DEGRADED", Severity: protocol.AlertWarning, Summary: "Hydroponics Bay A degraded"},
	}

	out := v.Render(plainStyler{}, st, events, alerts, 100, 40)

	for _, want := range []string{
		"SIMULATION CONTROL CORE",
		"RUNNING",
		"POWER BALANCE",
		"RESOURCE RUNWAY",
		"ACTIVE ALERTS",
		"OPERATIONAL EVENT LOG",
		"Power reserve low",
		"Hydroponics Bay A degraded",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("control view output missing %q", want)
		}
	}
}

func TestControlViewNoAlerts(t *testing.T) {
	v := NewControlView()
	out := v.Render(plainStyler{}, sampleState(), nil, nil, 100, 40)
	if !strings.Contains(out, "No active alerts") {
		t.Error("expected nominal message when there are no alerts")
	}
}
