package simulation

import (
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/protocol"
)

// minimumViablePopulation is the genetic-diversity floor below which long-term
// vault viability is at risk.
const minimumViablePopulation = 160

// evaluateAlerts derives the current set of actionable operational alerts from
// the snapshot. Acknowledgement state and original raise times are preserved for
// conditions that persist across cycles.
func (e *Engine) evaluateAlerts(st protocol.VaultState, prev []protocol.Alert) []protocol.Alert {
	prevByCode := make(map[string]protocol.Alert, len(prev))
	for _, a := range prev {
		prevByCode[a.Code] = a
	}

	var alerts []protocol.Alert
	add := func(code string, level protocol.AlertLevel, subsystem, msg string) {
		a := protocol.Alert{
			Code: code, Level: level, Subsystem: subsystem, Message: msg,
			Raised: time.Now().UTC(),
		}
		if old, ok := prevByCode[code]; ok {
			a.Raised = old.Raised
			a.Acknowledged = old.Acknowledged
		}
		alerts = append(alerts, a)
	}

	// Power.
	if st.Power.BalanceKW < 0 {
		add("POWER_DEFICIT", protocol.AlertCritical, "POWER",
			fmt.Sprintf("Power deficit: drawing %.0f kW against %.0f kW generation",
				st.Power.ConsumptionKW, st.Power.GenerationKW))
	} else if st.Power.ReservePct < 10 && st.Power.GenerationKW > 0 {
		add("POWER_RESERVE_LOW", protocol.AlertWarning, "POWER",
			fmt.Sprintf("Power reserve low: %.0f%% margin", st.Power.ReservePct))
	}

	// Facility systems.
	if st.Systems.Failed > 0 {
		add("SYSTEM_FAILURES", protocol.AlertCritical, "FACILITIES",
			fmt.Sprintf("%d system(s) failed and awaiting corrective maintenance", st.Systems.Failed))
	}
	for _, s := range st.SystemList {
		if s.Critical && (s.Status == string(models.SystemStatusFailed) || s.Status == string(models.SystemStatusOffline)) {
			add("CRIT_SYS_"+s.Code, protocol.AlertCritical, s.Category,
				fmt.Sprintf("Critical system %s (%s) is %s", s.Name, s.Code, s.Status))
		}
	}
	if st.Systems.Degraded > 0 {
		add("SYSTEMS_DEGRADED", protocol.AlertWarning, "FACILITIES",
			fmt.Sprintf("%d system(s) operating in a degraded state", st.Systems.Degraded))
	}
	if st.Systems.OverdueMaint > 0 {
		add("MAINTENANCE_OVERDUE", protocol.AlertWarning, "FACILITIES",
			fmt.Sprintf("%d system(s) overdue for preventive maintenance", st.Systems.OverdueMaint))
	}

	// Resources.
	for _, r := range st.Resources {
		switch r.Status {
		case "CRITICAL":
			add("RES_"+r.Category+"_CRIT", protocol.AlertCritical, "RESOURCES",
				fmt.Sprintf("%s critical: %d day(s) of runway remaining", r.Label, r.RunwayDays))
		case "WARNING":
			add("RES_"+r.Category+"_WARN", protocol.AlertWarning, "RESOURCES",
				fmt.Sprintf("%s low: %d day(s) of runway remaining", r.Label, r.RunwayDays))
		}
	}

	// Population.
	if st.Population.Active > 0 && st.Population.Active < minimumViablePopulation {
		add("POP_VIABILITY", protocol.AlertCritical, "POPULATION",
			fmt.Sprintf("Population %d below minimum viable threshold of %d",
				st.Population.Active, minimumViablePopulation))
	}
	if st.Population.LoadPct > 100 {
		add("POP_OVERCROWDING", protocol.AlertWarning, "POPULATION",
			fmt.Sprintf("Population at %.0f%% of designed capacity", st.Population.LoadPct))
	}
	if st.Population.Quarantined >= 5 {
		add("MED_OUTBREAK", protocol.AlertWarning, "MEDICAL",
			fmt.Sprintf("%d resident(s) in medical quarantine", st.Population.Quarantined))
	}

	return alerts
}
