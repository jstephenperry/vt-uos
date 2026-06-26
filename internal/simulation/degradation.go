package simulation

import (
	"context"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/protocol"
)

const (
	// efficiencyDegradedBelow is the threshold under which a system is reported
	// as degraded rather than fully operational.
	efficiencyDegradedBelow = 80.0
	// efficiencyFailedBelow is the threshold under which an unmaintained system
	// is considered failed.
	efficiencyFailedBelow = 15.0

	// maintenanceChancePerDay is the daily probability that maintenance crews
	// service a given overdue system, restoring its condition.
	maintenanceChancePerDay = 0.34
	// maintenanceRestore is the efficiency restored by a preventive service.
	maintenanceRestore = 35.0
)

// degradeSystems applies one day of wear to every operational facility system,
// accrues runtime, transitions status across thresholds, and reports notable
// changes to the operational log.
func (e *Engine) degradeSystems(ctx context.Context, asOf time.Time) {
	list, err := e.fac.ListSystems(ctx, models.FacilitySystemFilter{}, models.Pagination{Page: 1, PageSize: 500})
	if err != nil {
		e.log.Warn("listing systems for degradation failed", "error", err)
		return
	}

	baseDecay := e.cfg.Simulation.Consumption.EfficiencyDecayRate * 100 // percent/day
	if baseDecay <= 0 {
		baseDecay = 0.05
	}

	for _, sys := range list.Systems {
		if !sys.Status.IsOperational() {
			continue // offline, failed, destroyed or in maintenance: no wear
		}

		// Overdue maintenance accelerates wear.
		wear := 1.0
		if sys.IsOverdueForMaintenance(asOf) {
			wear = 2.2
		}
		jitter := 0.5 + e.rng.Float64() // 0.5..1.5
		decay := baseDecay * wear * jitter

		newEff := sys.EfficiencyPercent - decay
		if newEff < 0 {
			newEff = 0
		}

		newStatus := models.SystemStatusOperational
		switch {
		case newEff < efficiencyFailedBelow:
			newStatus = models.SystemStatusFailed
		case newEff < efficiencyDegradedBelow:
			newStatus = models.SystemStatusDegraded
		}

		runtimeAdd := 24.0 * (newEff / 100.0)
		output := outputFor(sys, newEff)

		if err := e.updateSystemDaily(ctx, sys.ID, newEff, newStatus, sys.TotalRuntimeHours+runtimeAdd, output); err != nil {
			e.log.Debug("updating system condition failed", "system", sys.SystemCode, "error", err)
			continue
		}

		if newStatus != sys.Status {
			e.reportStatusChange(ctx, asOf, sys, newStatus, newEff)
		}
	}
}

// reportStatusChange emits an operational event when a system crosses a status
// threshold downward.
func (e *Engine) reportStatusChange(ctx context.Context, asOf time.Time, sys *models.FacilitySystem, newStatus models.SystemStatus, eff float64) {
	switch newStatus {
	case models.SystemStatusFailed:
		e.emitEvent(ctx, asOf, catFacility, "SYSTEM_FAILURE", protocol.AlertCritical,
			fmt.Sprintf("%s failed", sys.Name),
			fmt.Sprintf("%s (%s) dropped to %.0f%% efficiency and is offline pending corrective maintenance.",
				sys.Name, sys.SystemCode, eff))
	case models.SystemStatusDegraded:
		e.emitEvent(ctx, asOf, catFacility, "SYSTEM_DEGRADED", protocol.AlertWarning,
			fmt.Sprintf("%s degraded", sys.Name),
			fmt.Sprintf("%s (%s) efficiency fell to %.0f%%; preventive maintenance advised.",
				sys.Name, sys.SystemCode, eff))
	}
}

// maintenanceSweep services overdue systems with some daily probability,
// restoring condition, consuming spare parts, and recording the work.
func (e *Engine) maintenanceSweep(ctx context.Context, asOf time.Time) {
	list, err := e.fac.ListSystems(ctx, models.FacilitySystemFilter{}, models.Pagination{Page: 1, PageSize: 500})
	if err != nil {
		return
	}
	for _, sys := range list.Systems {
		needsService := sys.IsOverdueForMaintenance(asOf) ||
			sys.Status == models.SystemStatusFailed ||
			sys.EfficiencyPercent < efficiencyDegradedBelow
		if !needsService {
			continue
		}
		// Failed systems are always prioritised; others are scheduled.
		chance := maintenanceChancePerDay
		if sys.Status == models.SystemStatusFailed {
			chance = 0.6
		}
		if e.rng.Float64() > chance {
			continue
		}
		e.performMaintenance(ctx, asOf, sys)
	}
}

// performMaintenance restores a system, consumes parts, and records the work in
// the maintenance log and operational event feed.
func (e *Engine) performMaintenance(ctx context.Context, asOf time.Time, sys *models.FacilitySystem) {
	newEff := sys.EfficiencyPercent + maintenanceRestore
	if newEff > 100 {
		newEff = 100
	}
	output := outputFor(sys, newEff)

	if err := e.updateSystemServiced(ctx, sys.ID, newEff, output, asOf, sys.MaintenanceIntervalDays); err != nil {
		e.log.Debug("recording maintenance failed", "system", sys.SystemCode, "error", err)
		return
	}

	// Spare parts are drawn from inventory to reflect real material cost.
	e.consumeByQuantity(ctx, "PARTS", 1.0, fmt.Sprintf("Maintenance of %s", sys.SystemCode))

	mtype := models.MaintenanceTypePreventive
	if sys.Status == models.SystemStatusFailed {
		mtype = models.MaintenanceTypeCorrective
	}
	e.insertMaintenanceRecord(ctx, asOf, sys, mtype, newEff)

	e.emitEvent(ctx, asOf, catFacility, "MAINTENANCE_COMPLETED", protocol.AlertInfo,
		fmt.Sprintf("%s serviced", sys.Name),
		fmt.Sprintf("%s maintenance on %s (%s) restored efficiency to %.0f%%.",
			mtype, sys.Name, sys.SystemCode, newEff))
}

// updateSystemDaily writes a system's condition after one day of wear.
func (e *Engine) updateSystemDaily(ctx context.Context, id string, eff float64, status models.SystemStatus, runtime, output float64) error {
	const q = `UPDATE facility_systems
		SET efficiency_percent = ?, status = ?, total_runtime_hours = ?, current_output = ?, updated_at = ?
		WHERE id = ?`
	_, err := e.db.ExecContext(ctx, q, eff, string(status), runtime, output,
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// updateSystemServiced writes a system's condition after maintenance.
func (e *Engine) updateSystemServiced(ctx context.Context, id string, eff, output float64, asOf time.Time, intervalDays int) error {
	next := asOf.AddDate(0, 0, intervalDays)
	const q = `UPDATE facility_systems
		SET efficiency_percent = ?, status = 'OPERATIONAL', current_output = ?,
		    last_maintenance_date = ?, next_maintenance_due = ?, updated_at = ?
		WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := e.db.ExecContext(ctx, q, eff, output,
		asOf.UTC().Format(time.RFC3339), next.UTC().Format(time.RFC3339), now, id)
	return err
}

// insertMaintenanceRecord records completed maintenance work.
func (e *Engine) insertMaintenanceRecord(ctx context.Context, asOf time.Time, sys *models.FacilitySystem, mtype models.MaintenanceType, effAfter float64) {
	const q = `INSERT INTO maintenance_records
		(id, system_id, maintenance_type, description, completed_at, outcome,
		 system_status_before, system_status_after, efficiency_before, efficiency_after,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'COMPLETED', ?, 'OPERATIONAL', ?, ?, ?, ?)`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := e.db.ExecContext(ctx, q,
		e.idGen.NewID(), sys.ID, string(mtype),
		fmt.Sprintf("Scheduled %s maintenance", mtype),
		asOf.UTC().Format(time.RFC3339),
		string(sys.Status), sys.EfficiencyPercent, effAfter, now, now)
	if err != nil {
		e.log.Debug("inserting maintenance record failed", "error", err)
	}
}

// outputFor estimates a system's current output from its rated capacity and
// present efficiency.
func outputFor(sys *models.FacilitySystem, eff float64) float64 {
	if sys.CapacityRating == nil {
		return 0
	}
	return *sys.CapacityRating * (eff / 100.0)
}

// systemEfficiencyByCategory returns the average efficiency (0..1) of
// operational systems grouped by facility category.
func (e *Engine) systemEfficiencyByCategory(ctx context.Context) map[string]float64 {
	out := make(map[string]float64)
	const q = `SELECT category, AVG(efficiency_percent)
		FROM facility_systems
		WHERE status IN ('OPERATIONAL','DEGRADED')
		GROUP BY category`
	rows, err := e.db.QueryContext(ctx, q)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var cat string
		var avg float64
		if err := rows.Scan(&cat, &avg); err == nil {
			out[cat] = avg / 100.0
		}
	}
	return out
}
