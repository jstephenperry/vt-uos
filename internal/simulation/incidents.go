package simulation

import (
	"context"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/protocol"
)

// dailyIncidentProbability maps the configured event frequency to the daily
// probability that an unplanned operational incident occurs.
func dailyIncidentProbability(freq config.EventFrequency) float64 {
	switch freq {
	case config.EventFrequencyMinimal:
		return 0.05
	case config.EventFrequencyReduced:
		return 0.12
	case config.EventFrequencyIncreased:
		return 0.45
	case config.EventFrequencyChaotic:
		return 0.80
	default: // normal or unset
		return 0.25
	}
}

// rollIncidents determines whether an unplanned operational incident occurs on
// the given day and, if so, generates one. Incident weighting is modified by the
// vault's current condition: failing systems make equipment faults more likely,
// scarce resources make security incidents more likely, and crowding raises the
// chance of conflict.
func (e *Engine) rollIncidents(ctx context.Context, asOf time.Time) {
	if !e.cfg.Simulation.AutoEvents {
		return
	}
	if e.rng.Float64() >= dailyIncidentProbability(e.cfg.Simulation.EventFrequency) {
		return
	}

	e.mu.RLock()
	avgEff := e.state.Systems.AvgEfficiency
	loadPct := e.state.Population.LoadPct
	scarce := false
	for _, r := range e.state.Resources {
		if r.Status == "CRITICAL" || r.Status == "WARNING" {
			scarce = true
			break
		}
	}
	e.mu.RUnlock()

	// Build a weighted incident table reflecting current conditions.
	type weighted struct {
		fn func(context.Context, time.Time)
		w  int
	}
	table := []weighted{
		{e.incidentEquipmentFault, 30 + int((100 - avgEff))},
		{e.incidentContamination, 15},
		{e.incidentIllness, 20},
		{e.incidentDiscovery, 12},
	}
	secWeight := 10
	if scarce {
		secWeight += 20
	}
	if loadPct > 95 {
		secWeight += 15
	}
	table = append(table, weighted{e.incidentSecurity, secWeight})

	total := 0
	for _, t := range table {
		total += t.w
	}
	if total <= 0 {
		return
	}
	pick := e.rng.Intn(total)
	for _, t := range table {
		if pick < t.w {
			t.fn(ctx, asOf)
			return
		}
		pick -= t.w
	}
}

// incidentEquipmentFault applies a sudden efficiency loss to a random
// operational system, modelling an unplanned fault.
func (e *Engine) incidentEquipmentFault(ctx context.Context, asOf time.Time) {
	sys := e.randomSystem(ctx, "WHERE status IN ('OPERATIONAL','DEGRADED')")
	if sys == nil {
		return
	}
	drop := 15.0 + e.rng.Float64()*35.0
	newEff := sys.EfficiencyPercent - drop
	if newEff < 0 {
		newEff = 0
	}
	newStatus := models.SystemStatusDegraded
	severity := protocol.AlertWarning
	if newEff < efficiencyFailedBelow {
		newStatus = models.SystemStatusFailed
		severity = protocol.AlertCritical
	}
	if err := e.updateSystemDaily(ctx, sys.ID, newEff, newStatus, sys.TotalRuntimeHours, outputFor(sys, newEff)); err != nil {
		return
	}
	e.emitEvent(ctx, asOf, catFacility, "EQUIPMENT_FAULT", severity,
		fmt.Sprintf("Unplanned fault: %s", sys.Name),
		fmt.Sprintf("%s (%s) lost %.0f points of efficiency to an unplanned fault; now at %.0f%%.",
			sys.Name, sys.SystemCode, drop, newEff))
}

// incidentContamination quarantines a random available stock lot.
func (e *Engine) incidentContamination(ctx context.Context, asOf time.Time) {
	var stockID, location string
	var itemName string
	const q = `SELECT rs.id, rs.storage_location, ri.name
		FROM resource_stocks rs JOIN resource_items ri ON ri.id = rs.item_id
		WHERE rs.status = 'AVAILABLE' AND rs.quantity > 0
		LIMIT 1 OFFSET ?`
	offset := e.randomOffset(ctx, "SELECT COUNT(*) FROM resource_stocks WHERE status='AVAILABLE' AND quantity > 0")
	if offset < 0 {
		return
	}
	if err := e.db.QueryRowContext(ctx, q, offset).Scan(&stockID, &location, &itemName); err != nil {
		return
	}
	if _, err := e.db.ExecContext(ctx,
		`UPDATE resource_stocks SET status = 'QUARANTINE', updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), stockID); err != nil {
		return
	}
	e.emitEvent(ctx, asOf, catResource, "CONTAMINATION", protocol.AlertWarning,
		fmt.Sprintf("Contamination: %s", itemName),
		fmt.Sprintf("A lot of %s in %s was flagged and moved to quarantine pending assay.", itemName, location))
}

// incidentIllness records a contagious medical condition for a random resident
// and, for severe cases, moves them to quarantine.
func (e *Engine) incidentIllness(ctx context.Context, asOf time.Time) {
	var residentID, name string
	const q = `SELECT id, surname || ', ' || given_names FROM residents
		WHERE status = 'ACTIVE' LIMIT 1 OFFSET ?`
	offset := e.randomOffset(ctx, "SELECT COUNT(*) FROM residents WHERE status='ACTIVE'")
	if offset < 0 {
		return
	}
	if err := e.db.QueryRowContext(ctx, q, offset).Scan(&residentID, &name); err != nil {
		return
	}

	severe := e.rng.Float64() < 0.3
	severity := "MODERATE"
	if severe {
		severity = "SEVERE"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	const cond = `INSERT INTO medical_conditions
		(id, resident_id, condition_code, condition_name, onset_date, severity,
		 is_chronic, is_genetic, is_contagious, created_at, updated_at)
		VALUES (?, ?, 'INF-001', 'Communicable infection', ?, ?, 0, 0, 1, ?, ?)`
	if _, err := e.db.ExecContext(ctx, cond,
		e.idGen.NewID(), residentID, asOf.UTC().Format(time.RFC3339), severity, now, now); err != nil {
		return
	}

	level := protocol.AlertWarning
	detail := fmt.Sprintf("%s presented with a communicable infection (%s). Contact tracing initiated.", name, severity)
	if severe {
		level = protocol.AlertCritical
		if _, err := e.db.ExecContext(ctx,
			`UPDATE residents SET status = 'QUARANTINE', updated_at = ? WHERE id = ?`,
			now, residentID); err != nil {
			e.log.Debug("quarantine update failed", "error", err)
		}
		detail = fmt.Sprintf("%s placed in medical quarantine with a severe communicable infection.", name)
	}
	e.emitEvent(ctx, asOf, catMedical, "ILLNESS", level, "Communicable infection reported", detail)
}

// incidentSecurity records a security incident.
func (e *Engine) incidentSecurity(ctx context.Context, asOf time.Time) {
	kinds := []struct {
		typ, sev, desc string
	}{
		{"ALTERCATION", "MINOR", "A verbal altercation between residents was reported and de-escalated."},
		{"THEFT", "MODERATE", "Ration pilferage was reported from a storage area; investigation opened."},
		{"INSUBORDINATION", "MINOR", "A work-shift refusal was logged and referred to the department head."},
		{"UNAUTHORIZED_ACCESS", "MAJOR", "An unauthorised access attempt was detected at a restricted door."},
	}
	k := kinds[e.rng.Intn(len(kinds))]
	now := time.Now().UTC().Format(time.RFC3339)
	incidentNum := fmt.Sprintf("INC-%s-%04d", asOf.Format("20060102"), e.rng.Intn(10000))
	const ins = `INSERT INTO security_incidents
		(id, incident_number, incident_type, severity, description, status,
		 occurred_at, reported_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'OPEN', ?, ?, ?, ?)`
	if _, err := e.db.ExecContext(ctx, ins,
		e.idGen.NewID(), incidentNum, k.typ, k.sev, k.desc,
		asOf.UTC().Format(time.RFC3339), asOf.UTC().Format(time.RFC3339), now, now); err != nil {
		e.log.Debug("inserting security incident failed", "error", err)
		return
	}
	level := protocol.AlertInfo
	if k.sev == "MAJOR" {
		level = protocol.AlertWarning
	}
	e.emitEvent(ctx, asOf, catSecurity, "SECURITY_INCIDENT", level,
		fmt.Sprintf("Security: %s (%s)", k.typ, incidentNum), k.desc)
}

// incidentDiscovery is a positive incident: a recovered cache adds stock to a
// random producible-or-stored resource.
func (e *Engine) incidentDiscovery(ctx context.Context, asOf time.Time) {
	var itemID, itemName, unit string
	const q = `SELECT id, name, unit_of_measure FROM resource_items LIMIT 1 OFFSET ?`
	offset := e.randomOffset(ctx, "SELECT COUNT(*) FROM resource_items")
	if offset < 0 {
		return
	}
	if err := e.db.QueryRowContext(ctx, q, offset).Scan(&itemID, &itemName, &unit); err != nil {
		return
	}
	qty := 20.0 + e.rng.Float64()*180.0
	const stock = `INSERT INTO resource_stocks
		(id, item_id, lot_number, quantity, quantity_reserved, storage_location,
		 received_date, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 'STORAGE-RECOVERED', ?, 'AVAILABLE', ?, ?)`
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := e.db.ExecContext(ctx, stock,
		e.idGen.NewID(), itemID, fmt.Sprintf("LOT-RCV-%s", asOf.Format("20060102")),
		qty, asOf.UTC().Format(time.RFC3339), now, now); err != nil {
		return
	}
	e.emitEvent(ctx, asOf, catDiscovery, "CACHE_RECOVERED", protocol.AlertInfo,
		fmt.Sprintf("Cache recovered: %s", itemName),
		fmt.Sprintf("A sealed storage cache was located and inventoried: %.0f %s of %s added to stock.",
			qty, unit, itemName))
}

// randomSystem returns a random facility system matching the given WHERE clause,
// or nil if none match.
func (e *Engine) randomSystem(ctx context.Context, where string) *models.FacilitySystem {
	offset := e.randomOffset(ctx, "SELECT COUNT(*) FROM facility_systems "+where)
	if offset < 0 {
		return nil
	}
	var id string
	q := "SELECT id FROM facility_systems " + where + " LIMIT 1 OFFSET ?"
	if err := e.db.QueryRowContext(ctx, q, offset).Scan(&id); err != nil {
		return nil
	}
	sys, err := e.fac.GetSystem(ctx, id)
	if err != nil {
		return nil
	}
	return sys
}

// randomOffset returns a deterministic random offset within the row count
// produced by the given COUNT(*) query, or -1 if there are no rows.
func (e *Engine) randomOffset(ctx context.Context, countQuery string) int {
	var n int
	if err := e.db.QueryRowContext(ctx, countQuery).Scan(&n); err != nil || n <= 0 {
		return -1
	}
	return e.rng.Intn(n)
}
