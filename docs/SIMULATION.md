# Simulation Control Core

The simulation control core (`internal/simulation`) is the single engine that advances vault time and applies daily operational accounting to the persistent database: consumption, production, expiration, facility wear, maintenance, unplanned incidents and vital statistics. It records occurrences to `simulation_events` and `audit_log`, derives alerts, checkpoints its position to `vault_metadata`, and publishes a `protocol.VaultState` snapshot that the TUI, master server and web console all read (`internal/simulation/engine.go`, `internal/simulation/tick.go`, `internal/simulation/metrics.go`).

The package writes to the database both through the domain services and with raw SQL against the shared handle; which path each stage uses is stated below.

> ⚠ See [#21](https://github.com/jstephenperry/vt-uos/issues/21).

## Engine surface

```go
func New(db *database.DB, cfg *config.Config, clock *util.VaultClock) *Engine

func (e *Engine) Start(ctx context.Context) error   // always returns nil
func (e *Engine) Stop(ctx context.Context) error    // always returns nil
func (e *Engine) Pause()
func (e *Engine) Resume()
func (e *Engine) SetTimeScale(scale float64) error  // scale < 0 rejected
func (e *Engine) Step(ctx context.Context, hours int) error
func (e *Engine) Status() protocol.SimStatus
func (e *Engine) Snapshot() protocol.VaultState
func (e *Engine) Alerts() []protocol.Alert
func (e *Engine) Events(limit int) []protocol.EventRecord
func (e *Engine) AcknowledgeAlert(code string) bool
func (e *Engine) Subscribe() (int, <-chan protocol.VaultState)
func (e *Engine) Unsubscribe(id int)
func (e *Engine) CreateSnapshot(ctx context.Context) (string, error)
func (e *Engine) Births() int / Deaths() int / TickCount() uint64
func (e *Engine) Population() / Resources() / Facilities() / Clock()
func EnsureCoreSystems(ctx, db *database.DB, fac *facilities.Service, installDate time.Time) error
```

`Engine` is a struct; no interface is declared (`internal/simulation/engine.go`). `New` builds its own `population`, `resources` and `facilities` services over `db.DB`, seeds `math/rand` from `deriveSeed(cfg)`, starts in `SimPaused`, and picks `sealDate` from `simulation.start_date`, else `vault.sealed_date`, else `clock.Now()`; `lastProc` starts at `clock.Now()`.

Constants (`engine.go`): `wakeInterval = 1s`, `maxHoursPerWake = 168`, `eventBufferSize = 200`.

## Time model

Vault time is owned by `util.VaultClock` (`internal/util/time.go`). `Now()` returns `startVaultTime + realElapsed × timeScale`, or the frozen `pausedAt` while paused. `timeScale` is a unitless ratio of vault duration to real duration: 1 is real time, 60 is one vault hour per real minute. `Advance` and `SetTime` only work while paused. `VaultClock` has no mutex; the engine reads it from its run goroutine and mutates it from `Pause`/`Resume`/`SetTimeScale`/`Step` on caller goroutines (`engine.go`).

> ⚠ See [#12](https://github.com/jstephenperry/vt-uos/issues/12) and [#30](https://github.com/jstephenperry/vt-uos/issues/30).

`cmd/vtuos/main.go` (`newClock`) builds the clock from `simulation.start_date` (falling back to `time.Now()`) with `simulation.time_scale`, and pauses it when `simulation.enabled = false`. The config default for `time_scale` is 60.0 (`internal/config/config.go`).

The engine never advances the clock in automatic mode. Its run goroutine (`engine.go`, `run`) wakes every `wakeInterval` (1 s); if `Status() != RUNNING` it does nothing, otherwise it calls `advanceTo(ctx, clock.Now())` under a 30 s context and logs any returned error at Error level.

`advanceTo` (`tick.go`):

1. Takes `procMu`, so the run loop and `Step` never process concurrently.
2. `hours = int(now − lastProc)` in whole hours. If `hours <= 0` it runs `lightweightRefresh` (clock-derived fields only, then broadcast) and returns.
3. Caps `hours` at `maxHoursPerWake` (168); the remainder is picked up on later wakes.
4. Walks a cursor forward one hour at a time, incrementing `tickN` per hour. Whenever the cursor crosses a calendar-day boundary (`util.IsSameDay`), it calls `processDay(ctx, util.StartOfDay(cursor))` — the `asOf` passed to every stage is midnight of the day just entered.
5. Sets `lastProc = cursor`, so a sub-hour remainder carries over.
6. If at least one day was processed, `saveState`.
7. `recomputeState` (its error is the only one `advanceTo` returns), then `broadcast`.

`processDay` always returns `nil` (`tick.go`), so the error branch that logs "daily operational cycle failed" is unreachable. Each stage handles its own errors; most are logged at Debug or dropped.

> ⚠ See [#22](https://github.com/jstephenperry/vt-uos/issues/22).

Because `tickN` is the number of processed hours, `TickCount` in the snapshot lags the clock by whatever `advanceTo` has not yet caught up. At a time scale above 168 hours per second (604 800×) the loop falls behind permanently.

> ⚠ See [#14](https://github.com/jstephenperry/vt-uos/issues/14).

## Tick pipeline (per simulated day)

`processDay(ctx, asOf)` (`tick.go`) runs these in order, unconditionally:

1. `consumeDailyResources` — `internal/simulation/consumption.go`
2. `produceDailyResources` — `consumption.go`
3. `processExpirations` — `consumption.go`
4. `degradeSystems` — `internal/simulation/degradation.go`
5. `maintenanceSweep` — `degradation.go`
6. `rollIncidents` — `internal/simulation/incidents.go`
7. `processDemographics` — `internal/simulation/demographics.go`

After all days in the batch (not per day): `saveState`, `recomputeState`, `broadcast` (`tick.go`, `advanceTo`). No stage runs inside a database transaction at the engine level.

### 1. Consumption

`consumeDailyResources` (`consumption.go`):

- `reqs = res.GetVaultDailyRequirements()` (per-household ration class × headcount, see [MODULES.md](MODULES.md)); on error it logs a warning and returns.
- `waterDemand = TotalWaterL × variance(water_variance)`, `calorieDemand = TotalCalories × variance(calorie_variance)`, where `variance(v) = 1 + (rng.Float64()×2 − 1)×v`, or exactly 1.0 when `v <= 0` (no RNG draw).
- Water: `consumeByQuantity("WATER", waterDemand, "Daily potable water draw")`.
- Food: `consumeFoodByCalories(calorieDemand, "Daily ration distribution")`.
- Medical: `consumeByQuantity("MEDICAL", state.Population.Active × 0.02, "Routine clinical consumption")`; `e.state` is read here without the lock.
- Stores `dailyUse["WATER"] = waterDemand` (demand, not the amount consumed), `dailyUse["FOOD"] = kg actually consumed`, `dailyUse["MEDICAL"] = units actually consumed` under `mu`.
  > ⚠ See [#29](https://github.com/jstephenperry/vt-uos/issues/29).
- Emits `WATER_SHORTFALL` (CRITICAL) when the unmet water exceeds 0.5 L and `FOOD_SHORTFALL` (CRITICAL) when the unmet calories exceed 1000 kcal.

`consumeByQuantity(categoryCode, demand, reason)`: resolves the category by code, lists its first 100 items (ordered by `item_code`, `internal/repository/resource_repo.go`), and for each item reads `availableForItem` (raw SQL `SUM(quantity − quantity_reserved)` over AVAILABLE lots), takes `min(remaining, avail)`, and calls `res.RecordConsumption` with `RelatedEntityType: "VAULT"`; a per-item error is skipped with `continue`. Returns `(consumed, shortfall)`.

`consumeFoodByCalories(calorieDemand, reason)`: same walk over the `FOOD` category, converting with `density = item.CaloriesPerUnit` when `> 0`, else `defaultFoodCaloriesPerUnit = 1500` kcal per unit; `take = min(remainingCal / density, avail)`, `remainingCal −= take × density`. Returns `(kg, calorieShortfall)`.

Within one item, `RecordConsumption` draws lots earliest-expiry first (`ORDER BY expiration_date ASC NULLS LAST, received_date ASC`, `internal/repository/resource_repo.go`) with one `UpdateStock` and one `CreateTransaction` per lot, no transaction (`internal/services/resources/service.go`). Across items the order is `item_code`.

### 2. Production

`produceDailyResources` (`consumption.go`): lists categories (silent return on error), computes `systemEfficiencyByCategory` — raw SQL `AVG(efficiency_percent)/100` over systems in status OPERATIONAL or DEGRADED grouped by category (`degradation.go`) — then for each category's first 100 items with `IsProducible && ProductionRatePerDay > 0`:

```plaintext
factor = eff[FOOD_PRODUCTION] for FOOD, eff[WATER] for WATER, eff[POWER] for POWER,
         1.0 for any other category or when the key is missing
qty    = ProductionRatePerDay × factor        (skipped when <= 0)
expiration = asOf + ShelfLifeDays             (nil when ShelfLifeDays is unset or <= 0)
```

Each item goes through `res.RecordProduction` with `StorageLocation = "PROD-" + first four characters of the category code` and reason `"Daily internal production"`; failures are logged at Debug. `RecordProduction` stamps the new lot's `ReceivedDate` with wall-clock `time.Now()` (`internal/services/resources/service.go`).

> ⚠ See [#32](https://github.com/jstephenperry/vt-uos/issues/32) and [#33](https://github.com/jstephenperry/vt-uos/issues/33).

### 3. Expiration

`processExpirations` (`consumption.go`) calls `res.ProcessExpiredItems(ctx, asOf)`, Debug-logs a failure, and emits `SPOILAGE` (WARNING) only when three or more lots were written off. The service selects candidates with `expiration_date <= date('now', '+0 days')` — SQLite's real-clock date — and then applies the vault-time `asOf` check only to that pre-filtered set (`internal/services/resources/service.go`, `internal/repository/resource_repo.go`).

> ⚠ See [#13](https://github.com/jstephenperry/vt-uos/issues/13).

### 4. Degradation

`degradeSystems` (`degradation.go`) lists up to 500 systems and, for each in status OPERATIONAL or DEGRADED:

```plaintext
baseDecay = simulation.consumption.efficiency_decay_rate × 100   (%/day; 0.05 if <= 0)
wear      = 2.2 if IsOverdueForMaintenance(asOf) else 1.0
jitter    = 0.5 + rng.Float64()
newEff    = max(0, efficiency − baseDecay × wear × jitter)
newStatus = FAILED   if newEff < 15
            DEGRADED if newEff < 80
            OPERATIONAL otherwise
runtime  += 24 × newEff / 100
output    = capacity_rating × newEff / 100   (0 when capacity is nil)
```

With the default `efficiency_decay_rate = 0.001` (`internal/config/config.go`), `baseDecay` is 0.1 %/day. The row is written by `updateSystemDaily`, a raw `UPDATE facility_systems SET efficiency_percent, status, total_runtime_hours, current_output, updated_at` with `updated_at = time.Now()`. A status change emits `SYSTEM_FAILURE` (CRITICAL) or `SYSTEM_DEGRADED` (WARNING); upward changes emit nothing (`reportStatusChange`).

`IsOverdueForMaintenance` is `asOf.After(*NextMaintenanceDue)`, false when nil (`internal/models/facility.go`). The repository parses that column with `time.Parse(time.DateOnly, ...)` and ignores the error (`internal/repository/facility_repo.go`, `scanSystem`), while the maintenance sweep writes it as RFC3339 (`degradation.go`, `updateSystemServiced`); after the first service the parsed value is the zero time and the system reads as permanently overdue.

> ⚠ See [#11](https://github.com/jstephenperry/vt-uos/issues/11).

### 5. Maintenance sweep

`maintenanceSweep` (`degradation.go`) lists up to 500 systems; a system needs service when `IsOverdueForMaintenance(asOf) || Status == FAILED || EfficiencyPercent < 80`. Chance of service that day: `maintenanceChancePerDay = 0.34`, or `0.6` when FAILED; skipped when `rng.Float64() > chance`. `performMaintenance`:

1. `newEff = min(100, efficiency + maintenanceRestore(35))`.
2. `updateSystemServiced`: raw `UPDATE` setting `efficiency_percent`, `status = 'OPERATIONAL'`, `current_output`, `last_maintenance_date = asOf`, `next_maintenance_due = asOf + MaintenanceIntervalDays` (both RFC3339), `updated_at = time.Now()`; on error, Debug-log and return.
3. `consumeByQuantity("PARTS", 1.0, "Maintenance of <code>")`, both return values discarded.
4. `insertMaintenanceRecord`: raw `INSERT INTO maintenance_records` with `maintenance_type` CORRECTIVE if the system was FAILED else PREVENTIVE, `completed_at = asOf`, `outcome = 'COMPLETED'`, `system_status_after = 'OPERATIONAL'`, before/after efficiency; error Debug-logged.
5. Emits `MAINTENANCE_COMPLETED` (INFO).

Status is not part of the eligibility test, so OFFLINE, MAINTENANCE or DESTROYED systems that are overdue or below 80% are set OPERATIONAL.

### 6. Incidents

`rollIncidents` (`incidents.go`) returns immediately when `simulation.auto_events = false`. Otherwise one `rng.Float64()` is compared with the daily probability for `simulation.event_frequency`:

| `event_frequency` | p(incident per day) |
| --- | --- |
| `minimal` | 0.05 |
| `reduced` | 0.12 |
| `normal`, `""`, anything else | 0.25 |
| `increased` | 0.45 |
| `chaotic` | 0.80 |

When an incident fires, the weights are built from the current snapshot (`e.state`, read under `mu.RLock`), which is the one produced by the last `recomputeState` — i.e. from the end of the previous `advanceTo` batch, not the current day.

> ⚠ See [#32](https://github.com/jstephenperry/vt-uos/issues/32).

| Incident | Weight | Effect (persisted) |
| --- | --- | --- |
| `incidentEquipmentFault` | `30 + int(100 − Systems.AvgEfficiency)` | Random system in OPERATIONAL/DEGRADED loses `15 + rng×35` efficiency points; status DEGRADED (WARNING), or FAILED (CRITICAL) if `< 15`; written via `updateSystemDaily` with runtime unchanged; event `EQUIPMENT_FAULT`. |
| `incidentContamination` | 15 | Random AVAILABLE lot with `quantity > 0` set to `status = 'QUARANTINE'` by raw UPDATE; event `CONTAMINATION` (WARNING). |
| `incidentIllness` | 20 | Random ACTIVE resident gets a `medical_conditions` row (`INF-001`, "Communicable infection", `is_contagious = 1`, severity SEVERE with p = 0.3 else MODERATE); a severe case also sets `residents.status = 'QUARANTINE'`; event `ILLNESS` (CRITICAL if severe, else WARNING). |
| `incidentDiscovery` | 12 | Random `resource_items` row gains a new `resource_stocks` lot of `20 + rng×180` units, lot `LOT-RCV-YYYYMMDD`, location `STORAGE-RECOVERED`, `received_date = asOf`; no `resource_transactions` row; event `CACHE_RECOVERED` (INFO). |
| `incidentSecurity` | `10`, `+20` if any resource status is CRITICAL or WARNING, `+15` if `Population.LoadPct > 95` | Inserts a `security_incidents` row (one of ALTERCATION/MINOR, THEFT/MODERATE, INSUBORDINATION/MINOR, UNAUTHORIZED_ACCESS/MAJOR; `incident_number = INC-YYYYMMDD-<rng 0..9999>`, status OPEN); event `SECURITY_INCIDENT` (WARNING for MAJOR, else INFO). |

The pick is `rng.Intn(total)` walked linearly through the table in the order above. Random rows are chosen with `LIMIT 1 OFFSET rng.Intn(COUNT(*))` (`randomOffset`, `randomSystem`). Every handler returns silently or Debug-logs on a database error. Nothing in the package ever returns a QUARANTINE resident to ACTIVE or a QUARANTINE lot to AVAILABLE. Elapsed time since seal does not enter any weight.

> ⚠ See [#31](https://github.com/jstephenperry/vt-uos/issues/31).

### 7. Demographics

`processDemographics` (`demographics.go`): `active = COUNT(*) FROM residents WHERE status = 'ACTIVE'` (raw SQL; returns when zero). Constants `annualDeathRate = 0.009`, `annualBirthRate = 0.014`.

- Death when `rng.Float64() < active × 0.009 / 365`: a random ACTIVE resident is selected by offset, a cause is drawn from five fixed strings, and `pop.RegisterDeath(id, {DateOfDeath: asOf, Cause})` is called (a single `residents` UPDATE, see [MODULES.md](MODULES.md)); on success `deaths++` and event `DEATH` (INFO). Household, vocation and quarters are untouched.
- Birth when `rng.Float64() < active × 0.014 / 365`: the mother is a random ACTIVE female with a non-null `household_id` and `date_of_birth` between `asOf − 45y` and `asOf − 18y` (compared as `YYYY-MM-DD` strings); the father is the first ACTIVE male in the same household aged 18+ (`LIMIT 1`, not random). Sex is 50/50, the given name comes from a 16-name list, the surname is the mother's. `pop.RegisterBirth` is called with `Parent1ID = father`, `Parent2ID = mother`, `HouseholdID`, notes `"Registered by simulation control core"`; on success `births++` and event `BIRTH` (INFO). No eligible pairing means the opportunity is skipped.
  > ⚠ See [#27](https://github.com/jstephenperry/vt-uos/issues/27).

`RegisterBirth` opens a transaction and then reads `GetNextRegistryNumber` on the pool connection while the pool is limited to one open connection (`internal/services/population/service.go`, `internal/repository/resident_repo.go`, `internal/database/database.go`).

> ⚠ See [#5](https://github.com/jstephenperry/vt-uos/issues/5).

## RNG and determinism

The RNG is one `math/rand.Rand` seeded by `deriveSeed(cfg)` (`engine.go`):

```plaintext
seed = vault.number × 1_000_003 + Σ_i rune_i(simulation.start_date) × (i+1)
if seed == 0: seed = 2077
```

With the defaults (`vault.number = 76`, `simulation.start_date = 2077-10-23T09:47:00Z`, `internal/config/config.go`) this evaluates to `76012209`; `Start` logs the value as `seed=<n>` when the core starts (`engine.go`). `vault.sealed_date`, database contents and wall time do not enter the seed. The RNG is used only inside `processDay` stages, which run under `procMu`.

Draws per day, in order: water variance and calorie variance (one `Float64` each, only when the variance is `> 0`); one `Float64` jitter per operational system; one `Float64` per system needing service; the incident gate, then `Intn(total)` and handler-specific draws if it fires; the death gate, then offset and cause; the birth gate, then offset, sex and name (`consumption.go`, `degradation.go`, `incidents.go`, `demographics.go`). The number of draws therefore depends on row counts and on the previous snapshot.

Determinism holds for `Step`-driven runs over identical databases: `TestEngineDeterministic` steps two fresh engines 720 hours and compares tick, birth, death and event counts (`internal/simulation/engine_test.go`). In automatic mode the number of days batched per wake depends on wall time, and the incident weights read the previous batch's snapshot, so two live runs can diverge. Row IDs use `crypto/rand` (`internal/util/id.go`) and are never reproducible; `updated_at`, `created_at`, `processed_at`, `Generated` and `Alert.Raised` are wall-clock timestamps.

> ⚠ See [#32](https://github.com/jstephenperry/vt-uos/issues/32).

## Alerts

`evaluateAlerts(state, prev)` (`internal/simulation/alerts.go`) is called only from `recomputeState` under `mu.Lock` (`metrics.go`). It rebuilds the list from scratch each time; for a code that was present in `prev`, `Raised` and `Acknowledged` are copied forward. A code absent for one cycle loses its acknowledgement and is raised fresh if it recurs. `Raised` is `time.Now().UTC()`, not vault time.

| Code | Level | Subsystem | Condition |
| --- | --- | --- | --- |
| `POWER_DEFICIT` | CRITICAL | POWER | `Power.BalanceKW < 0` |
| `POWER_RESERVE_LOW` | WARNING | POWER | else `Power.ReservePct < 10 && GenerationKW > 0` |
| `SYSTEM_FAILURES` | CRITICAL | FACILITIES | `Systems.Failed > 0` |
| `CRIT_SYS_<code>` | CRITICAL | system category | per system: `Critical && (FAILED or OFFLINE)` |
| `SYSTEMS_DEGRADED` | WARNING | FACILITIES | `Systems.Degraded > 0` |
| `MAINTENANCE_OVERDUE` | WARNING | FACILITIES | `Systems.OverdueMaint > 0` |
| `RES_<category>_CRIT` | CRITICAL | RESOURCES | resource `Status == "CRITICAL"` |
| `RES_<category>_WARN` | WARNING | RESOURCES | resource `Status == "WARNING"` |
| `POP_VIABILITY` | CRITICAL | POPULATION | `0 < Population.Active < 160` (`minimumViablePopulation`) |
| `POP_OVERCROWDING` | WARNING | POPULATION | `Population.LoadPct > 100` |
| `MED_OUTBREAK` | WARNING | MEDICAL | `Population.Quarantined >= 5` |

Resource status is computed in `buildResources` (`metrics.go`): when `DailyUse > 0`, `RunwayDays = int(onHand / DailyUse)` with `< 7` CRITICAL, `< 30` WARNING, `< 90` WATCH, else OK, and `FillFraction = clamp(RunwayDays/365, 0, 1)`; when `DailyUse == 0`, `RunwayDays = -1`, status OK, `FillFraction = 1`. `DailyUse` is the engine's `dailyUse` map, so only WATER, FOOD and MEDICAL ever have a runway; the other categories always report `-1`.

`AcknowledgeAlert(code)` sets `Acknowledged = true` on the first alert with that code and returns whether one was found (`engine.go`). Alerts are not persisted.

## Events

`emitEvent` (`internal/simulation/events.go`) appends an `EventRecord` (fresh ID, `VaultTime = asOf`, category, type, severity, summary, detail) to an in-memory ring trimmed to 200, then `persistEvent`, then an Info log. `persistEvent` writes:

- `simulation_events`: `id, event_type, scheduled_time = vault time (RFC3339), processed_at = time.Now(), status = 'COMPLETED', priority (100 CRITICAL / 50 WARNING / 10 otherwise), payload (JSON of category/severity/summary/detail), result = summary, created_at = time.Now()`.
- `audit_log`: `id, timestamp = vault time ("2006-01-02 15:04:05"), actor_type = 'SIMULATION', actor_id = 'control-core', action = event type, entity_type = category, entity_id = event id, new_values = summary, terminal_id = 'SIM'`.

Both inserts are independent and Debug-logged on failure. Categories in use: `RESOURCE`, `FACILITY`, `POPULATION`, `SECURITY`, `MEDICAL`, `DISCOVERY` (`SYSTEM` is declared but unused).

Event types emitted: `WATER_SHORTFALL`, `FOOD_SHORTFALL`, `SPOILAGE`, `SYSTEM_FAILURE`, `SYSTEM_DEGRADED`, `MAINTENANCE_COMPLETED`, `EQUIPMENT_FAULT`, `CONTAMINATION`, `ILLNESS`, `SECURITY_INCIDENT`, `CACHE_RECOVERED`, `DEATH`, `BIRTH`.

## Persistence and continuity

`saveState` (`internal/simulation/persistence.go`) upserts four `vault_metadata` keys — `vault_time` (`clock.Now()` in RFC3339, not `lastProc`), `sim_births`, `sim_deaths`, `sim_tick_count` — as four separate `INSERT ... ON CONFLICT(key) DO UPDATE` statements in Go map order, no transaction; the first failure returns and leaves earlier keys written. It runs at the end of any `advanceTo` that processed at least one day (`tick.go`), in `Stop` after the run goroutine exits (`engine.go`), and at the start of `CreateSnapshot` (`internal/simulation/snapshot.go`). It does not run after hour-only batches, `Pause`, `Resume` or `SetTimeScale`.

> ⚠ See [#15](https://github.com/jstephenperry/vt-uos/issues/15).

`loadState` runs first in `Start`: it reads `vault_time`, and if it parses as RFC3339 and is not before `sealDate`, pauses the clock, `SetTime(t)`, sets `lastProc = t`, and resumes the clock if it was running before. `sim_births`, `sim_deaths` and `sim_tick_count` are restored regardless of whether the time was accepted. The migration pre-seeds `vault_time = '2077-10-23T09:47:00Z'` (`internal/database/migrations/001_initial.sql`), so the first `Start` on a fresh database restores that value; lowering `simulation.start_date` afterwards has no effect on the clock.

Because the checkpoint stores the clock rather than the processed position, a restart after a checkpoint taken while the engine was behind sets `lastProc` to the clock value and the unprocessed hours are never simulated. `Resume` and `SetTimeScale` also reset `lastProc = clock.Now()` (`engine.go`), discarding any backlog at that moment.

> ⚠ See [#10](https://github.com/jstephenperry/vt-uos/issues/10) and [#15](https://github.com/jstephenperry/vt-uos/issues/15).

Domain rows written during a day (stocks, transactions, systems, residents, events) are committed statement by statement as they happen; the checkpoint follows them.

## Start, Stop, Step

`Start` (`engine.go`): idempotent under `runMu`; `loadState` → `EnsureCoreSystems(ctx, db, fac, sealDate)` → `recomputeState` (each warn-and-continue) → status `RUNNING` iff `simulation.enabled && !clock.IsPaused()`, else `PAUSED` → `go run()`. Logs status, vault time, time scale and seed at Info.

`Stop`: closes `stopCh`, waits on `doneCh` with no timeout, then `saveState`. `status` is left unchanged; `protocol.SimStopped` is never assigned anywhere. `cmd/vtuos/main.go` (`stopEngine`) gives `Stop` a 5 s context and logs, but does not act on, an error.

`Step(ctx, hours)`: `hours <= 0` becomes 1; if the clock is running it is paused and status set to `PAUSED` — and not resumed afterwards. Then `clock.Advance(hours × 1h)` and `advanceTo(target)` in a loop of at most `hours/168 + 2` iterations, stopping once `lastProc >= target − 1h`. The first `advanceTo` error is returned unwrapped.

`EnsureCoreSystems` (`internal/simulation/systems_seed.go`): if `COUNT(*) FROM facility_systems > 0` it returns; otherwise it creates 15 fixed systems through `facilities.Service.CreateSystem` with `installDate = sealDate` (or `time.Now()` if zero), warning and continuing on each failure. The gate is a row count, so a database with any user-created system receives no core systems.

| Code | Name | Category | Capacity | Interval (days) |
| --- | --- | --- | --- | --- |
| PWR-RX-01 | Fusion Reactor Core | POWER | 480 kW | 180 |
| PWR-GEN-01 | Backup Generator Bank | POWER | 160 kW | 120 |
| PWR-DIST-01 | Power Distribution Grid | POWER | 600 kW | 90 |
| WTR-PUR-01 | Water Purification Plant | WATER | 12000 L/day | 60 |
| WTR-REC-01 | Greywater Recycling Unit | WATER | 8000 L/day | 60 |
| HVAC-FILT-01 | Primary Air Filtration | HVAC | 100 % | 45 |
| HVAC-CLIM-01 | Climate Control System | HVAC | 100 % | 90 |
| WST-PROC-01 | Sewage Processing Plant | WASTE | 100 % | 60 |
| SEC-DOOR-01 | Vault Door Control | SECURITY | 100 % | 30 |
| SEC-SURV-01 | Surveillance Network | SECURITY | 100 % | 90 |
| MED-CLIN-01 | Medical Bay Equipment | MEDICAL | 100 % | 90 |
| FOOD-HYDRO-01 | Hydroponics Bay A | FOOD_PRODUCTION | 600 kg/day | 75 |
| FOOD-HYDRO-02 | Hydroponics Bay B | FOOD_PRODUCTION | 600 kg/day | 75 |
| COM-INT-01 | Internal Communications | COMMUNICATIONS | 100 % | 180 |
| STR-INT-01 | Structural Integrity Monitor | STRUCTURAL | 100 % | 365 |

## Snapshot and `--restore`

`CreateSnapshot` (`snapshot.go`) calls `saveState` (warn on failure) then `db.Backup(ctx)`, which runs `PRAGMA wal_checkpoint(TRUNCATE)` and `VACUUM INTO '<backup_dir>/vault-YYYYMMDD-HHMMSS.db'` and returns the path (`internal/database/database.go`, `Backup`, `Checkpoint`). `Backup` returns `"backup directory not configured"` when no backup directory is set. No engine lock is held while the backup runs. The filename comes from `time.Now().Format("20060102-150405")`, so it is wall-clock, not vault time.

There is no engine-level restore. `vtuos -restore <file>` and `vtuos serve -restore <file>` (`cmd/vtuos/main.go`, `prepare`) copy the file over the database path before it is opened, first removing any `-wal`/`-shm` sidecars (`restoreDatabase`); the recovery check and migrations then run on the restored file.

## Control API

TUI (`internal/tui/app.go`, `handleSimulationKeys`; active in the simulation view, `F11`):

| Key | Engine call |
| --- | --- |
| `space`, `p` | `Pause()` if `Status() == RUNNING`, else `Resume()` |
| `+`, `=` / `-`, `_` | `SetTimeScale` to the next/previous step of `1, 60, 600, 1440, 3600, 14400, 86400` |
| `s` / `h` | `Step(ctx, 24)` / `Step(ctx, 1)` in a background command with `context.Background()` |
| `k` | `CreateSnapshot` in a background command |
| `a` | `AcknowledgeAlert(code)` for every alert in the current snapshot |

HTTP (`internal/server/server.go`, `internal/server/api.go`); control routes require the admin token via `requireAdmin`:

| Route | Engine call | Notes |
| --- | --- | --- |
| `GET /api/v1/state` | `Snapshot()` | full `VaultState` |
| `GET /api/v1/status` | `Snapshot()` | status, time_scale, vault_time, elapsed_days, elapsed_years, tick_count |
| `GET /api/v1/events?limit=N` | `Events(N)` | default 50 |
| `GET /api/v1/alerts` | `Alerts()` | |
| `GET /api/v1/systems`, `/population`, `/resources` | `Snapshot()` sub-fields | |
| `GET /api/v1/stream` | `Subscribe()`/`Unsubscribe()` | SSE, `event: state` frames, 15 s keep-alive comments |
| `POST /api/v1/sim/pause` | `Pause()` | returns the snapshot |
| `POST /api/v1/sim/resume` | `Resume()` | returns the snapshot |
| `POST /api/v1/sim/scale` `{"time_scale": f}` | `SetTimeScale(f)` | 400 on negative |
| `POST /api/v1/sim/step` `{"hours": n}` | `Step(r.Context(), n)` | `n <= 0` → 24, `n > 8760` → 8760; 500 on error |
| `POST /api/v1/sim/snapshot` | `CreateSnapshot()` | `{"snapshot": path}` |
| `POST /api/v1/sim/ack` `{"code": c}` | `AcknowledgeAlert(c)` | `{"acknowledged": bool}` |

Without the token every control route answers 401. `pause`, `resume` and `snapshot` take no body; `step`, `scale` and `ack` answer 400 `invalid request body` when the body is missing or not JSON (`internal/server/api.go`).

> ⚠ See [#30](https://github.com/jstephenperry/vt-uos/issues/30).

## VaultState publication

`recomputeState` (`metrics.go`) runs its queries outside the lock, then under `mu.Lock` fills identity and clock fields (`SchemaVersion`, `Generated = time.Now()`, designation, number, status, time scale, `VaultTime`, `SealDate`, `TickCount`, `ElapsedDays/Years` from `sealDate`), overlays the engine's `births`/`deaths` counters onto the population block, stores `Resources`, `Systems`, `SystemList`, `Power`, and re-evaluates alerts.

- `buildPopulation`: `Active`/`Deceased`/`Quarantined` counts and `AverageAge` (`julianday` against the vault date) by raw SQL; `Capacity = vault.designed_capacity`; `LoadPct = Active / Capacity × 100`.
- `buildResources`: per-category on-hand over AVAILABLE lots, joined to `dailyUse` (see Alerts for the runway rule).
- `buildSystems`: `fac.GetFacilityStats(asOf)` for the summary and `fac.ListSystems` (up to 500) for the list; `Critical` is true for POWER, WATER, HVAC, WASTE, SECURITY.
- `buildPower`: `ConsumptionKW = Σ nominalDrawKW[category]` over OPERATIONAL/DEGRADED non-POWER systems (WATER 40, HVAC 60, WASTE 25, SECURITY 15, MEDICAL 20, FOOD_PRODUCTION 35, COMMUNICATIONS 5, STRUCTURAL 5); `GenerationKW = Σ capacity_rating × efficiency/100` over OPERATIONAL/DEGRADED POWER systems by raw SQL; `BalanceKW = gen − cons`; `ReservePct = bal / gen × 100` when `gen > 0`; all rounded to 0.1. Power balance is reported only; nothing feeds it back into other systems.

With the 15 core systems (table above) all OPERATIONAL at 100% this evaluates to `generation_kw` 1240 (480 + 160 + 600), `consumption_kw` 355 (2×40 + 2×60 + 25 + 2×15 + 20 + 2×35 + 5 + 5), `balance_kw` 885 and `reserve_pct` 71.4.

Consumers get the state two ways:

- Pull: `Snapshot()` returns `cloneState()` under `RLock` — a struct copy with fresh slices for `Resources`, `SystemList`, `Alerts` and the newest 25 events as `RecentEvents` (`engine.go`). The TUI polls it every UI tick and after steps (`internal/tui/app.go`); the HTTP handlers call it per request.
- Push: `Subscribe()` registers a channel of capacity 4 and primes it with the current state; `broadcast()` clones once under `RLock` and does a non-blocking send to each subscriber, dropping the frame when a channel is full; `Unsubscribe` closes the channel under `Lock`. Broadcasts happen at the end of every `advanceTo` (including the hourless `lightweightRefresh` path, once per second while running) and on `Pause`, `Resume` and `SetTimeScale`. The SSE handler is the only production subscriber (`internal/server/sse.go`).

## Configuration

Keys consumed by this package (`internal/config/config.go` defaults):

```toml
[simulation]
enabled = true                  # false → clock paused, engine starts PAUSED
time_scale = 60.0               # ratio of vault time to real time
auto_events = true              # false → stage 6 is skipped
event_frequency = "normal"      # minimal | reduced | normal | increased | chaotic
start_date = "2077-10-23T09:47:00Z"   # clock start, sealDate, and part of the RNG seed

[simulation.consumption]
calorie_variance = 0.1          # daily demand × (1 ± 0.1)
water_variance = 0.1
efficiency_decay_rate = 0.001   # × 100 → 0.1 % efficiency per day; <= 0 → 0.05 %/day
```

Also read: `vault.number` (seed, services), `vault.designation` and `vault.designed_capacity` (snapshot), `vault.sealed_date` (fallback `sealDate`).

## Concurrency

- One `run` goroutine per engine; `Step` and `CreateSnapshot` are invoked from Bubble Tea command goroutines and HTTP handler goroutines (`internal/tui/app.go`, `internal/server/api.go`).
- `procMu` serialises `advanceTo` (`tick.go`); `Step` holds it only inside each `advanceTo` call.
- `mu` guards published state, counters, `dailyUse`, the event ring and `tickN`; `loadState` writes `lastProc`, `births`, `deaths`, `tickN` without it, before the goroutine starts (`persistence.go`, `engine.go`).
- `runMu` guards `running`, `stopCh`, `doneCh`.
- `VaultClock` is unsynchronised (see Time model).
- The run loop's 30 s context can expire inside `processDay`; the remaining statements of that day fail, `lastProc` still advances, and the day is not retried.
  > ⚠ See [#14](https://github.com/jstephenperry/vt-uos/issues/14).

## Test coverage

`internal/simulation` reports 72.5% statement coverage in the baseline run (`go test -count=1 -cover ./internal/...`; a `-race` run of the same suite reported 72.7%, so the package has timing-dependent test paths). `internal/simulation/engine_test.go` drives the engine only through `Step` with `simulation.enabled = false`; the automatic run loop, alert evaluation, `AcknowledgeAlert`, `CreateSnapshot`, expiration, production, degradation thresholds, the maintenance sweep and the incident handlers have no direct assertions.
