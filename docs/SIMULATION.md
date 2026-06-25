# Simulation Control Core

The simulation control core (`internal/simulation`) is the authoritative engine
that advances vault time and drives realistic operations against the persistent
database. It is **operational software, not a game**: every interval it processes
is deterministic given its seed, mutates real domain state through the service
layer, records occurrences to the persistent event and audit logs, and publishes
an operational snapshot to subscribers.

## Design goals

- **Authoritative** — one engine owns vault progression; the TUI, web console,
  and client terminals all read the same `protocol.VaultState` it produces.
- **Persistent** — consumption, production, degradation, maintenance, incidents,
  and demographics are written to the database through the existing services and
  mirrored to `simulation_events` and the immutable `audit_log`.
- **Deterministic** — a stable seed derived from the vault number and seal date
  drives a single `math/rand` source, so identical configurations replay
  identically (supporting the project's reproducible-build constraint).
- **Power-loss resilient** — operational position (vault time, birth/death
  counters, tick count) is checkpointed to `vault_metadata` so a restart resumes
  where it left off.

## Processing model

The engine wakes on a fixed real-time cadence and advances simulated time to
match the wall-clock-scaled vault time. Whole simulated **hours** are advanced
one at a time; the heavy operational accounting runs once per simulated **day**,
which bounds database writes regardless of time scale.

```
Each simulated day (processDay):
  1. Resource consumption   — water (L) and food (kcal→kg) drawn FIFO from stock,
                              plus routine medical/consumable draw; shortfalls
                              are reported as critical events.
  2. Resource production     — producible items, scaled by the efficiency of the
                              governing facility (hydroponics→food, treatment→water).
  3. Expiration              — perishables past shelf life written off as spoilage.
  4. Facility degradation    — per-day efficiency wear (accelerated when overdue),
                              status transitions (operational→degraded→failed),
                              runtime accrual.
  5. Maintenance sweep       — overdue/failed systems serviced with some daily
                              probability, restoring condition and consuming parts.
  6. Operational incidents   — one weighted unplanned incident may occur (see below).
  7. Demographics            — low-probability births/deaths via the population
                              service (registries, lineage and counts stay consistent).
  8. Persist                 — checkpoint operational position; recompute and
                              broadcast the snapshot.
```

A single automatic wake processes at most one simulated week (`maxHoursPerWake`)
as runaway protection; an explicit operator **step** drains the full requested
span in capped chunks.

## Operational incidents

When `auto_events` is enabled, each day has a configurable probability (by
`event_frequency`) of one unplanned incident. The incident table is weighted by
current conditions — failing systems make equipment faults more likely, scarce
resources raise security incidents, crowding raises conflict. Incidents are
framed and recorded as real operational events:

| Incident          | Effect (persisted)                                              |
| ----------------- | --------------------------------------------------------------- |
| Equipment fault   | Sudden efficiency loss on a random operational system           |
| Contamination     | A stock lot moved to `QUARANTINE` pending assay                 |
| Illness / outbreak| Contagious `medical_conditions` record; severe cases quarantined|
| Security incident | A `security_incidents` record (altercation, theft, …)           |
| Cache recovered   | Recovered stock added to inventory (a positive incident)        |

Every incident is written to `simulation_events` and `audit_log` with the
`SIMULATION` actor, so the governance audit trail reflects them.

## Alerts

Alerts are derived from the current snapshot each cycle (not ad-hoc), so they
always reflect present conditions: power deficit/low reserve, failed/degraded/
overdue systems, critical/low resource runway, sub-viable or over-capacity
population, and medical quarantine. Acknowledgement state is preserved across
recomputes by alert code.

## Control API

```go
engine := simulation.New(db, cfg, clock)
engine.Start(ctx)                 // restore continuity, ensure core systems, begin
engine.Pause(); engine.Resume()
engine.SetTimeScale(1440)         // vault-seconds per real-second
engine.Step(ctx, 24)             // advance & process N hours (operator action)
state := engine.Snapshot()        // latest protocol.VaultState (copy)
alerts := engine.Alerts()
events := engine.Events(50)
engine.AcknowledgeAlert(code)
path, _ := engine.CreateSnapshot(ctx) // VACUUM INTO backup
id, ch := engine.Subscribe()      // live snapshots (SSE / TUI)
engine.Stop(ctx)                  // checkpoint and halt
```

## Configuration

```toml
[simulation]
enabled = true
time_scale = 1440.0          # 1 real minute ≈ 1 vault day
auto_events = true
event_frequency = "normal"   # minimal | reduced | normal | increased | chaotic
start_date = "2077-10-23T09:47:00Z"

[simulation.consumption]
calorie_variance = 0.1       # ±10% daily variance
water_variance = 0.1
efficiency_decay_rate = 0.001 # base facility wear per day
```

## Snapshot & reset

`engine.CreateSnapshot` captures a consistent copy of the whole database to the
backup directory (`VACUUM INTO`). To reset an exhibit to a clean baseline, stop
the process and relaunch with `vtuos --restore <snapshot.db>` (or `serve
--restore`), which safely swaps the database file before reopening.
