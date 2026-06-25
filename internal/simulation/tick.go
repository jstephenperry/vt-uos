package simulation

import (
	"context"
	"time"

	"github.com/vtuos/vtuos/internal/util"
)

// advanceTo brings the simulation up to the given vault time. Whole simulated
// hours are advanced one at a time; the heavy operational accounting (resource
// consumption, degradation, maintenance, incidents, demographics) is performed
// once per simulated day, which bounds database writes regardless of time scale.
func (e *Engine) advanceTo(ctx context.Context, now time.Time) error {
	e.mu.Lock()
	last := e.lastProc
	e.mu.Unlock()

	hours := int(now.Sub(last).Hours())
	if hours <= 0 {
		// No whole simulated hour elapsed; refresh the clock-derived fields so
		// displays keep ticking without touching the database.
		e.lightweightRefresh()
		return nil
	}
	if hours > maxHoursPerWake {
		hours = maxHoursPerWake // catch up over subsequent wakes
	}

	cursor := last
	daysProcessed := 0
	for i := 0; i < hours; i++ {
		prev := cursor
		cursor = cursor.Add(time.Hour)

		e.mu.Lock()
		e.tickN++
		e.mu.Unlock()

		// Cross a calendar day boundary: run the daily operational cycle for
		// the day that just completed.
		if !util.IsSameDay(prev, cursor) {
			if err := e.processDay(ctx, util.StartOfDay(cursor)); err != nil {
				e.log.Error("daily operational cycle failed",
					"day", cursor.Format(util.DateFormat), "error", err)
			}
			daysProcessed++
		}
	}

	e.mu.Lock()
	e.lastProc = cursor
	e.mu.Unlock()

	if daysProcessed > 0 {
		if err := e.saveState(ctx); err != nil {
			e.log.Warn("could not persist simulation state", "error", err)
		}
	}

	if err := e.recomputeState(ctx); err != nil {
		return err
	}
	e.broadcast()
	return nil
}

// processDay runs one simulated day of vault operations. Each stage is isolated:
// a failure in one is logged and the cycle continues, mirroring how an
// operational system degrades gracefully rather than halting.
func (e *Engine) processDay(ctx context.Context, asOf time.Time) error {
	e.consumeDailyResources(ctx, asOf)
	e.produceDailyResources(ctx, asOf)
	e.processExpirations(ctx, asOf)
	e.degradeSystems(ctx, asOf)
	e.maintenanceSweep(ctx, asOf)
	e.rollIncidents(ctx, asOf)
	e.processDemographics(ctx, asOf)
	return nil
}

// lightweightRefresh updates only the clock-derived fields of the operational
// snapshot and notifies subscribers. It performs no database access.
func (e *Engine) lightweightRefresh() {
	now := e.clock.Now()
	e.mu.Lock()
	e.state.VaultTime = now
	e.state.Generated = time.Now().UTC()
	e.state.Status = e.status
	e.state.TimeScale = e.clock.TimeScale()
	e.state.TickCount = e.tickN
	if !e.sealDate.IsZero() {
		elapsed := now.Sub(e.sealDate)
		e.state.ElapsedDays = int(elapsed.Hours() / 24)
		e.state.ElapsedYears = int(elapsed.Hours() / 8760)
	}
	e.mu.Unlock()
	e.broadcast()
}
