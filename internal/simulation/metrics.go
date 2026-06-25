package simulation

import (
	"context"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/protocol"
)

// nominalDrawKW is the approximate electrical draw of a running system by
// category, used to estimate the vault's power balance for the operations board.
var nominalDrawKW = map[string]float64{
	"POWER":           0,
	"WATER":           40,
	"HVAC":            60,
	"WASTE":           25,
	"SECURITY":        15,
	"MEDICAL":         20,
	"FOOD_PRODUCTION": 35,
	"COMMUNICATIONS":  5,
	"STRUCTURAL":      5,
}

// recomputeState rebuilds the operational snapshot from current database state,
// re-evaluates alerts, stores the result, and notifies subscribers indirectly
// via the caller's broadcast.
func (e *Engine) recomputeState(ctx context.Context) error {
	now := e.clock.Now()

	pop := e.buildPopulation(ctx, now)
	res := e.buildResources(ctx)
	sysSummary, sysList := e.buildSystems(ctx, now)
	power := e.buildPower(sysList)

	e.mu.Lock()
	e.state.SchemaVersion = protocol.Version
	e.state.Generated = time.Now().UTC()
	e.state.VaultDesignation = e.cfg.Vault.Designation
	e.state.VaultNumber = e.cfg.Vault.Number
	e.state.Status = e.status
	e.state.TimeScale = e.clock.TimeScale()
	e.state.VaultTime = now
	e.state.SealDate = e.sealDate
	e.state.TickCount = e.tickN
	if !e.sealDate.IsZero() {
		elapsed := now.Sub(e.sealDate)
		e.state.ElapsedDays = int(elapsed.Hours() / 24)
		e.state.ElapsedYears = int(elapsed.Hours() / 8760)
	}
	pop.Births = e.births
	pop.Deaths = e.deaths
	e.state.Population = pop
	e.state.Resources = res
	e.state.Systems = sysSummary
	e.state.SystemList = sysList
	e.state.Power = power
	e.alerts = e.evaluateAlerts(e.state, e.alerts)
	e.state.Alerts = append([]protocol.Alert(nil), e.alerts...)
	e.mu.Unlock()
	return nil
}

// buildPopulation queries the census for the operations board.
func (e *Engine) buildPopulation(ctx context.Context, asOf time.Time) protocol.PopulationStatus {
	var p protocol.PopulationStatus
	p.Capacity = e.cfg.Vault.DesignedCapacity

	rows := e.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN status='ACTIVE' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status='DECEASED' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status='QUARANTINE' THEN 1 ELSE 0 END),0)
		FROM residents`)
	_ = rows.Scan(&p.Active, &p.Deceased, &p.Quarantined)

	_ = e.db.QueryRowContext(ctx,
		`SELECT COALESCE(AVG((julianday(?) - julianday(date_of_birth))/365.25),0)
		 FROM residents WHERE status='ACTIVE'`,
		asOf.UTC().Format(time.RFC3339)).Scan(&p.AverageAge)

	if p.Capacity > 0 {
		p.LoadPct = float64(p.Active) / float64(p.Capacity) * 100
	}
	return p
}

// buildResources summarises on-hand inventory by category with runway forecasts.
func (e *Engine) buildResources(ctx context.Context) []protocol.ResourceStatus {
	const q = `SELECT rc.code, rc.name, rc.unit_of_measure,
		COALESCE(SUM(CASE WHEN rs.status='AVAILABLE' THEN rs.quantity ELSE 0 END),0)
		FROM resource_categories rc
		LEFT JOIN resource_items ri ON ri.category_id = rc.id
		LEFT JOIN resource_stocks rs ON rs.item_id = ri.id
		GROUP BY rc.id ORDER BY rc.code`
	rows, err := e.db.QueryContext(ctx, q)
	if err != nil {
		return nil
	}
	defer rows.Close()

	e.mu.RLock()
	use := make(map[string]float64, len(e.dailyUse))
	for k, v := range e.dailyUse {
		use[k] = v
	}
	e.mu.RUnlock()

	var out []protocol.ResourceStatus
	for rows.Next() {
		var code, name, unit string
		var onHand float64
		if err := rows.Scan(&code, &name, &unit, &onHand); err != nil {
			continue
		}
		rs := protocol.ResourceStatus{
			Category: code, Label: name, Unit: unit, OnHand: onHand,
			DailyUse: use[code], RunwayDays: -1, Status: "OK",
		}
		if rs.DailyUse > 0 {
			rs.RunwayDays = int(onHand / rs.DailyUse)
			switch {
			case rs.RunwayDays < 7:
				rs.Status = "CRITICAL"
			case rs.RunwayDays < 30:
				rs.Status = "WARNING"
			case rs.RunwayDays < 90:
				rs.Status = "WATCH"
			}
			rs.FillFraction = clamp(float64(rs.RunwayDays)/365.0, 0, 1)
		} else {
			rs.FillFraction = 1
		}
		out = append(out, rs)
	}
	return out
}

// buildSystems returns the facility health summary and a per-system list.
func (e *Engine) buildSystems(ctx context.Context, asOf time.Time) (protocol.SystemsSummary, []protocol.SystemStatus) {
	var summary protocol.SystemsSummary
	if stats, err := e.fac.GetFacilityStats(ctx, asOf); err == nil {
		summary = protocol.SystemsSummary{
			Total:         stats.TotalSystems,
			Operational:   stats.Operational,
			Degraded:      stats.Degraded,
			Offline:       stats.Offline,
			Failed:        stats.Failed,
			AvgEfficiency: stats.AvgEfficiency,
			OverdueMaint:  stats.OverdueMaintenance,
		}
	}

	list, err := e.fac.ListSystems(ctx, models.FacilitySystemFilter{}, models.Pagination{Page: 1, PageSize: 500})
	if err != nil {
		return summary, nil
	}
	out := make([]protocol.SystemStatus, 0, len(list.Systems))
	for _, s := range list.Systems {
		out = append(out, protocol.SystemStatus{
			Code:       s.SystemCode,
			Name:       s.Name,
			Category:   string(s.Category),
			Status:     string(s.Status),
			Efficiency: s.EfficiencyPercent,
			Critical:   isCriticalCategory(s.Category),
		})
	}
	return summary, out
}

// buildPower estimates the vault's electrical balance from current systems.
func (e *Engine) buildPower(systems []protocol.SystemStatus) protocol.PowerStatus {
	var gen, cons float64
	for _, s := range systems {
		operational := s.Status == string(models.SystemStatusOperational) ||
			s.Status == string(models.SystemStatusDegraded)
		if !operational {
			continue
		}
		if s.Category == string(models.SystemCategoryPower) {
			// Generation handled below via DB for capacity accuracy.
			continue
		}
		cons += nominalDrawKW[s.Category]
	}
	// Generation from rated capacity scaled by efficiency.
	_ = e.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(SUM(COALESCE(capacity_rating,0)*efficiency_percent/100.0),0)
		 FROM facility_systems
		 WHERE category='POWER' AND status IN ('OPERATIONAL','DEGRADED')`).Scan(&gen)

	bal := gen - cons
	reserve := 0.0
	if gen > 0 {
		reserve = bal / gen * 100
	}
	return protocol.PowerStatus{
		GenerationKW:  round1(gen),
		ConsumptionKW: round1(cons),
		BalanceKW:     round1(bal),
		ReservePct:    round1(reserve),
	}
}

func isCriticalCategory(c models.SystemCategory) bool {
	switch c {
	case models.SystemCategoryPower, models.SystemCategoryWater,
		models.SystemCategoryHVAC, models.SystemCategoryWaste,
		models.SystemCategorySecurity:
		return true
	default:
		return false
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
