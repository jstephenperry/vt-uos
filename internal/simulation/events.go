package simulation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/util"
)

// Event categories used in the operational log.
const (
	catResource   = "RESOURCE"
	catFacility   = "FACILITY"
	catPopulation = "POPULATION"
	catSecurity   = "SECURITY"
	catMedical    = "MEDICAL"
	catDiscovery  = "DISCOVERY"
	catSystem     = "SYSTEM"
)

// emitEvent records an operational occurrence to the in-memory feed, the
// persistent simulation_events table, and the immutable audit log. It is
// best-effort with respect to persistence: a database error is logged but does
// not abort the operational cycle.
func (e *Engine) emitEvent(ctx context.Context, asOf time.Time, category, evType string, severity protocol.AlertLevel, summary, detail string) {
	rec := protocol.EventRecord{
		ID:        e.idGen.NewID(),
		VaultTime: asOf,
		Category:  category,
		Type:      evType,
		Severity:  severity,
		Summary:   summary,
		Detail:    detail,
	}

	e.mu.Lock()
	e.events = append(e.events, rec)
	if len(e.events) > eventBufferSize {
		e.events = e.events[len(e.events)-eventBufferSize:]
	}
	e.mu.Unlock()

	e.persistEvent(ctx, rec)
	e.log.Info("operational event",
		"category", category, "type", evType, "severity", string(severity), "summary", summary)
}

// persistEvent writes the event to simulation_events and the audit log.
func (e *Engine) persistEvent(ctx context.Context, rec protocol.EventRecord) {
	payload, _ := json.Marshal(map[string]string{
		"category": rec.Category,
		"severity": string(rec.Severity),
		"summary":  rec.Summary,
		"detail":   rec.Detail,
	})

	const evInsert = `INSERT INTO simulation_events
		(id, event_type, scheduled_time, processed_at, status, priority, payload, result, created_at)
		VALUES (?, ?, ?, ?, 'COMPLETED', ?, ?, ?, ?)`
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := e.db.ExecContext(ctx, evInsert,
		rec.ID, rec.Type, rec.VaultTime.UTC().Format(time.RFC3339), now,
		severityPriority(rec.Severity), string(payload), rec.Summary, now,
	); err != nil {
		e.log.Debug("persisting simulation event failed", "error", err)
	}

	const auditInsert = `INSERT INTO audit_log
		(id, timestamp, actor_type, actor_id, action, entity_type, entity_id, new_values, terminal_id)
		VALUES (?, ?, 'SIMULATION', 'control-core', ?, ?, ?, ?, 'SIM')`
	if _, err := e.db.ExecContext(ctx, auditInsert,
		e.idGen.NewID(), rec.VaultTime.UTC().Format(util.DateTimeFormat),
		rec.Type, rec.Category, rec.ID, rec.Summary,
	); err != nil {
		e.log.Debug("persisting audit entry failed", "error", err)
	}
}

func severityPriority(level protocol.AlertLevel) int {
	switch level {
	case protocol.AlertCritical:
		return 100
	case protocol.AlertWarning:
		return 50
	default:
		return 10
	}
}
