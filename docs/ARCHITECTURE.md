# Architecture (As Built)

This document describes how the VT-UOS binary is wired today, at commit `34c3398`. It is derived from the code rather than from the earlier design documents (whose unimplemented intent is collected in [ROADMAP.md](ROADMAP.md)); the service internals are in [MODULES.md](MODULES.md).

## Package Layers

Import graph of non-test Go files under `internal/` and `cmd/` (dependency direction is left → right; a package never imports anything to its left):

| Layer | Packages | Imports from `internal/` |
| --- | --- | --- |
| Leaf | `protocol`, `models`, `util`, `config`, `tui/components` | none |
| Persistence | `database` | `config` |
| | `database/seed` | `models`, `util` |
| | `repository` | `models` |
| Services | `services/population`, `services/resources`, `services/facilities` | `models`, `repository`, `util` |
| Engine | `simulation` | `config`, `database`, `models`, `protocol`, `services/*`, `util` |
| Presentation | `tui/views/{population,resources,facilities}` | `models`, `services/<same>`, `tui/components` |
| | `tui/views/simulation` | `protocol` |
| | `tui` | `config`, `database`, `models`, `protocol`, `services/*`, `simulation`, `tui/views/*`, `util` |
| Network | `server` | `config`, `protocol`, `simulation`, `util` |
| | `client` | `config`, `protocol`, `tui` |
| Entry | `cmd/vtuos` | `client`, `config`, `database`, `database/seed`, `protocol`, `server`, `simulation`, `tui`, `util` |

`internal/protocol` is stdlib-only and is the shared vocabulary between the engine, the TUI, the master server, the web console and client terminals (`internal/protocol/protocol.go`).

## Three Run Modes

`cmd/vtuos/main.go` dispatches on `os.Args[1]`: `serve` → `serveMain`, `connect` → `connectMain`, anything else (including no arguments) → `localMain` (`cmd/vtuos/main.go`, `main`). Each mode function returns an exit code that `main` passes to `os.Exit`.

All three modes install the same signal handler: `signalContext` cancels a root context on SIGINT/SIGTERM and then arms a 10-second `time.AfterFunc` that calls `os.Exit(1)` if shutdown has not completed (`cmd/vtuos/main.go`, `signalContext`).

### Shared setup: `prepare`

`localMain` and `serveMain` both call `prepare`, which performs the following in order (`cmd/vtuos/main.go`, `prepare`):

1. `config.Load(configPath, true)` — a load error is fatal.
2. `setupLogging` — JSON `slog` handler to the configured log file, or a text handler to stderr when no log path is configured.
3. `config.EnsureDataDir` — resolves the database path.
4. If `-restore <file>` was given, `restoreDatabase` copies the snapshot over the live database path before anything opens it.
5. `config.BackupDir` — a failure here only logs a warning and disables backups.
6. If the database file already exists, `database.AttemptRecovery` runs; a hard failure aborts startup.
7. `database.Open` — opens SQLite with `SetMaxOpenConns(1)`, applies pragmas, runs an integrity check whose failure is only logged (`internal/database/database.go`, `Open`).
8. `database.NewMigrator` + `MigrateUp` — applies embedded `migrations/*.sql` (`internal/database/migrate.go`).

`prepare` returns a `closeFn` that closes the database and the log file.

### Local console (`vtuos`)

Construction order in `runLocal` (`cmd/vtuos/main.go`, `runLocal`):

1. `prepare` (above).
2. `-migrate-only` → log and return before anything else starts.
3. `-seed` → `runSeed` and return; the TUI never starts. `runSeed` skips when `residents` already has rows, and otherwise runs `seed.Generator` with `FamilyHouseholds: 100`, `SingleHouseholds: 80`, `RandomSeed: 2077` (`cmd/vtuos/main.go`, `runSeed`).
4. `newClock(cfg)` — a `util.VaultClock` started at `simulation.start_date` (falling back to `time.Now()`), paused when `simulation.enabled = false` (`cmd/vtuos/main.go`, `newClock`).
5. `simulation.New(db, cfg, clock)` then `engine.Start(ctx)`; a `Start` error is logged as a warning and startup continues.
6. `defer stopEngine(engine)` — `Stop` with a 5-second timeout.
7. `tui.RunWithEngine(ctx, db, cfg, clock, engine)` blocks until the Bubble Tea program exits.

### Master server (`vtuos serve`)

Construction order in `serveMain` (`cmd/vtuos/main.go`, `serveMain`):

1. `prepare`.
2. `-listen` and `-token` flag values are written into `cfg.Server` after `config.Load` has already validated the config; the overrides themselves are never validated.
   > ⚠ See [#8](https://github.com/jstephenperry/vt-uos/issues/8).
3. `newClock`, `simulation.New`, `engine.Start` (warning on error), `defer stopEngine` — identical to local mode.
4. `server.New(engine, cfg)` — builds the client registry and an `http.Server` with only `ReadHeaderTimeout: 10s` set (`internal/server/server.go`, `New`).
5. `srv.Start()` runs in its own goroutine and reports into a one-slot `errCh`.
6. `select` on `ctx.Done()` versus `errCh`. On a server error the function logs and returns 1 without calling `srv.Shutdown`; on context cancellation it calls `srv.Shutdown` with a 5-second timeout and discards the error.

There is no TUI in serve mode; the engine is observed only through HTTP.

### Client terminal (`vtuos connect <addr>`)

Construction order in `connectMain` (`cmd/vtuos/main.go`, `connectMain`):

1. Positional master address is required; missing → usage on stderr, exit 2.
2. `config.Load`; on error the client silently falls back to `config.Default()` instead of failing.
3. `setupClientLogging` — logs to the configured file if possible, otherwise to `io.Discard` so nothing prints over the TUI.
4. Terminal kind is `OPERATOR` only when `-kind` or `client.kind` equals `operator` case-insensitively; any other value becomes `DISPLAY`.
5. `client.NewConn(addr, name, kind, Version)` then `client.Run(ctx, conn, cfg)`.

No database, engine, or migrations are touched in connect mode.

## Process Model

### Goroutines by mode

| Mode | Goroutine | Started by | Wakes on | Talks to |
| --- | --- | --- | --- | --- |
| all | signal watcher | `signalContext` (`cmd/vtuos/main.go`) | SIGINT/SIGTERM | cancels root ctx; 10 s forced exit |
| local, serve | backup scheduler | `database.Open` when `backup_interval_hours > 0` and a backup dir exists (`internal/database/database.go`, `startBackupScheduler`) | ticker every `backup_interval_hours` | `db.Backup` with a 5-minute timeout; errors logged only |
| local, serve | engine run loop | `Engine.Start` → `go e.run()` (`internal/simulation/engine.go`, `run`) | `time.Ticker(1s)` | `advanceTo(ctx, clock.Now())` under `procMu` when status is `RUNNING`; 30 s timeout per wake |
| local | Bubble Tea program | `tui.RunWithEngine` (`internal/tui/app.go`) | messages | `engine.Snapshot()` every 1 s tick; `Step`/`CreateSnapshot` in `tea.Cmd` goroutines |
| local | ctx → `p.Quit()` watcher | `tui.RunWithEngine` | root ctx done | Bubble Tea program |
| serve | HTTP listener | `serveMain` → `go srv.Start()` | `ListenAndServe` | one goroutine per request (`net/http`) |
| serve | registry cleanup | `Server.Start` → `go s.cleanupLoop()` (`internal/server/server.go`) | ticker every minute | `ClientRegistry.CleanupStale(10 × client_timeout)` |
| serve | SSE stream (per client) | `handleStream` request goroutine (`internal/server/sse.go`) | subscriber channel, 15 s keep-alive ticker, `r.Context()` | `engine.Subscribe()` / `Unsubscribe()` |
| connect | SSE reader | `client.Run` (`internal/client/client.go`, `Run`) | HTTP body lines | `Conn.StreamStates` → `stateCh`; on error sends `false` on `connCh` and sleeps 2 s before reconnecting |
| connect | Bubble Tea program | `client.Run` | messages | `listenState`/`listenConn` commands drain `stateCh`/`connCh`; heartbeat `tea.Tick` at the master's `heartbeat_seconds`; 1 s `secondCmd` |
| connect | ctx → `p.Quit()` watcher | `client.Run` | root ctx done | Bubble Tea program |

### How the pieces communicate

- **Engine → consumers (push).** `Engine.Subscribe()` allocates a `chan protocol.VaultState` with capacity 4, registers it under the write lock, and primes it with one clone of the current state (`internal/simulation/engine.go`, `Subscribe`). `broadcast()` clones once under the read lock and does a non-blocking send to every subscriber; a full channel drops that frame (`internal/simulation/engine.go`, `broadcast`). `Unsubscribe` deletes and closes the channel under the write lock. The only production subscriber is the SSE handler (`internal/server/sse.go`, `handleStream`).
- **Engine → consumers (pull).** `Engine.Snapshot()` returns `cloneState()` under the read lock (`internal/simulation/engine.go`, `Snapshot`). The local TUI calls it on every 1-second `tickMsg` and after each step or snapshot (`internal/tui/app.go`, `Update`); the HTTP read handlers call it per request (`internal/server/api.go`).
- **Engine lifecycle.** `stopCh` (closed by `Stop`) and `doneCh` (closed when `run` returns) are the only lifecycle channels (`internal/simulation/engine.go`, `Start`/`Stop`).
- **Control into the engine.** Method calls only — `Pause`, `Resume`, `SetTimeScale`, `Step`, `CreateSnapshot`, `AcknowledgeAlert` — from TUI key handlers (`internal/tui/app.go`, `handleSimulationKeys`) and HTTP handlers (`internal/server/api.go`). `Step` and `CreateSnapshot` run in Bubble Tea command goroutines with `context.Background()` (`internal/tui/app.go`, `stepSim`, `snapshotSim`); the HTTP step handler runs synchronously on the request goroutine with `r.Context()`.
- **Master ↔ web console.** Browser opens `EventSource("/api/v1/stream")` for state and POSTs control requests with the token from `localStorage["vtuos_token"]` in `X-Admin-Token` (`internal/server/web/static/app.js`).
- **Master ↔ client terminal.** Plain HTTP: `POST /api/v1/clients/register` (5 attempts, linear 1–5 s backoff, `internal/client/client.go`, `registerWithRetry`), `GET /api/v1/stream` parsed line-by-line for `data:` frames (`internal/client/transport.go`, `StreamStates`), `POST /api/v1/clients/{id}/heartbeat` carrying `protocol.Telemetry` and returning queued `protocol.Command`s, and `POST /api/v1/clients/{id}/result`. All calls share one `http.Client{Timeout: 15s}`, including the long-lived stream (`internal/client/transport.go`, `NewConn`).
  > ⚠ See [#35](https://github.com/jstephenperry/vt-uos/issues/35).
- **No callbacks.** Nothing in the engine, server, or client registers function callbacks; communication is channels, method calls, and HTTP.

### Locks

| Lock | Guards | Where |
| --- | --- | --- |
| `Engine.mu` (RWMutex) | `status`, `lastProc`, `tickN`, `dailyUse`, `births`, `deaths`, `state`, `alerts`, `events`, `subs` | `internal/simulation/engine.go` |
| `Engine.procMu` | serialises `advanceTo` between the run loop and `Step` | `internal/simulation/tick.go`, `advanceTo` |
| `Engine.runMu` | `running`, `stopCh`, `doneCh` | `internal/simulation/engine.go`, `Start`/`Stop` |
| `ClientRegistry.mu` (RWMutex) | in-memory client map | `internal/server/clients.go` |
| `DB.mu` (RWMutex) | `closed` flag | `internal/database/database.go` |
| SQLite pool | `SetMaxOpenConns(1)` — every statement from engine, TUI and HTTP handlers serialises through one connection | `internal/database/database.go`, `Open` |

`util.VaultClock` has no mutex; its fields are read by the engine run loop (`clock.Now()` each wake) and written by `Pause`/`Resume`/`SetTimeScale`/`Step` from TUI and HTTP goroutines, and read by the TUI on every tick (`internal/util/time.go`, `VaultClock`; `internal/simulation/engine.go`; `internal/tui/app.go`).

> ⚠ See [#12](https://github.com/jstephenperry/vt-uos/issues/12).

## Data Flow

```plaintext
                         cmd/vtuos/main.go (prepare)
                                   │
                    config.Load ─► database.Open ─► Migrator.MigrateUp
                                   │
                                   ▼
        ┌──────────────────── SQLite (vault.db, WAL, 1 conn) ───────────────────┐
        │  residents · households · resource_* · facility_systems ·             │
        │  maintenance_records · simulation_events · audit_log · vault_metadata │
        └───────────────┬──────────────────────────────────▲─────────────────────┘
                        │ reads (services + raw SQL)       │ writes (services + raw SQL)
                        ▼                                  │
             ┌──────────────────────────────────────────────────────┐
             │ simulation.Engine                                    │
             │   run() ── 1 s ──► advanceTo(clock.Now())            │
             │     └─ per simulated day: processDay()               │
             │          consume ► produce ► expire ► degrade ►      │
             │          maintain ► incidents ► demographics         │
             │   recomputeState()  ──► e.state (protocol.VaultState)│
             │   broadcast()       ──► subs[i] <- cloneState()      │
             │   Snapshot()        ◄── pull                         │
             └───────┬───────────────────────┬──────────────────────┘
                     │ Snapshot() (1 s tick) │ Subscribe() (chan cap 4)
                     ▼                       ▼
        ┌────────────────────┐     ┌─────────────────────────────┐
        │ internal/tui (App) │     │ internal/server             │
        │  a.live            │     │  GET /api/v1/state  (pull)  │
        │  dashboard, F11    │     │  GET /api/v1/stream (SSE)   │
        │  simulation view   │     │  POST /api/v1/sim/* (token) │
        └────────────────────┘     └──────┬───────────────┬──────┘
                                          │ SSE           │ SSE + heartbeat/commands
                                          ▼               ▼
                              ┌───────────────┐   ┌─────────────────────┐
                              │ web console   │   │ internal/client     │
                              │ static/app.js │   │ vtuos connect <addr>│
                              └───────────────┘   └─────────────────────┘
```

`internal/tui` also reads the database directly (one `SELECT COUNT(*) FROM residents`) and through the three services it constructs itself; those paths are not drawn (`internal/tui/app.go`, `New`, `loadPopulation`).

## `protocol.VaultState`

Defined in `internal/protocol/protocol.go`. JSON field names are what the API and SSE stream emit.

| Field | Go type | JSON | Source |
| --- | --- | --- | --- |
| `SchemaVersion` | `string` | `schema_version` | `protocol.Version` (`"1.0"`) |
| `Generated` | `time.Time` | `generated` | wall clock at rebuild |
| `VaultDesignation` | `string` | `vault_designation` | `vault.designation` |
| `VaultNumber` | `int` | `vault_number` | `vault.number` |
| `Status` | `SimStatus` | `status` | `RUNNING` / `PAUSED` (`STOPPED` is declared but never assigned) |
| `TimeScale` | `float64` | `time_scale` | `clock.TimeScale()` |
| `VaultTime` | `time.Time` | `vault_time` | `clock.Now()` |
| `SealDate` | `time.Time` | `seal_date` | `simulation.start_date`, else `vault.sealed_date`, else clock at construction |
| `ElapsedDays` | `int` | `elapsed_days` | `(VaultTime − SealDate) / 24 h` |
| `ElapsedYears` | `int` | `elapsed_years` | `(VaultTime − SealDate) / 8760 h` |
| `TickCount` | `uint64` | `tick_count` | simulated hours processed |
| `Population` | `PopulationStatus` | `population` | `active`, `capacity`, `deceased`, `births`, `deaths`, `quarantined`, `average_age`, `load_pct` |
| `Resources` | `[]ResourceStatus` | `resources` | per category: `category`, `label`, `on_hand`, `unit`, `daily_use`, `runway_days` (−1 = unlimited), `status` (`OK`/`WATCH`/`WARNING`/`CRITICAL`), `fill_fraction` |
| `Systems` | `SystemsSummary` | `systems` | `total`, `operational`, `degraded`, `offline`, `failed`, `avg_efficiency`, `overdue_maintenance` |
| `SystemList` | `[]SystemStatus` | `system_list` | per system: `code`, `name`, `category`, `status`, `efficiency`, `critical` |
| `Power` | `PowerStatus` | `power` | `generation_kw`, `consumption_kw`, `balance_kw`, `reserve_pct` |
| `Alerts` | `[]Alert` | `alerts` | `code`, `level`, `message`, `subsystem`, `raised`, `acknowledged` |
| `RecentEvents` | `[]EventRecord` | `recent_events` | newest 25 of the in-memory ring of 200: `id`, `vault_time`, `category`, `type`, `severity`, `summary`, `detail` |

### Update cadence

- **Full rebuild** (`recomputeState`, `internal/simulation/metrics.go`): runs at the end of `Engine.Start`, and at the end of every `advanceTo` that processed at least one whole simulated hour (`internal/simulation/tick.go`, `advanceTo`). It queries population, resources, systems and power from the database outside the lock, then fills every field and re-evaluates alerts under `mu`.
- **Lightweight refresh** (`lightweightRefresh`, `internal/simulation/tick.go`): when a wake finds no whole simulated hour elapsed, only `VaultTime`, `Generated`, `Status`, `TimeScale`, `TickCount`, `ElapsedDays` and `ElapsedYears` are updated, with no database access, and a broadcast follows.
- **Broadcast triggers**: end of every `advanceTo`, every `lightweightRefresh`, and each `Pause`, `Resume`, `SetTimeScale` (`internal/simulation/engine.go`). While the engine is `PAUSED`, the run loop skips its work entirely, so no broadcast originates from the loop until the next control call (`internal/simulation/engine.go`, `run`).
- **Wall-clock rate**: the run loop wakes every 1 s (`wakeInterval`), so in `RUNNING` state consumers see at most one frame per second from the loop. A single wake processes at most 168 simulated hours (`maxHoursPerWake`); any backlog beyond that is caught up on subsequent wakes.
- **Snapshot readers** (TUI tick, `GET /api/v1/state`) always see the latest rebuilt state regardless of broadcast timing.

## Where the Layering Rules Are Not Followed

CLAUDE.md states "Views call services, never repositories directly" and describes a models ← repository ← services ← TUI stack. The code departs from that in the following places. Stated as facts; no fixes proposed here.

1. **TUI runs raw SQL.** `App.loadPopulation` executes `SELECT COUNT(*) FROM residents WHERE status = 'ACTIVE'` against `*database.DB` from the presentation layer, bypassing both `population.Service` and `ResidentRepository` (`internal/tui/app.go`, `loadPopulation`).
   > ⚠ See [#45](https://github.com/jstephenperry/vt-uos/issues/45).
2. **Simulation package runs raw SQL against domain tables.** The package doc says the engine "mutates real domain state through the service layer" (`internal/simulation/engine.go`), but `e.db.ExecContext`/`QueryRowContext`/`QueryContext` are used directly in `consumption.go` (available stock), `degradation.go` (efficiency/status updates, maintenance record inserts, per-category efficiency), `incidents.go` (random picks, quarantine updates, `medical_conditions` and `security_incidents` inserts, recovered-lot inserts), `demographics.go` (census counts, random resident/mother/father selection), `metrics.go` (population, resource and power aggregates), `persistence.go` (`vault_metadata`), `events.go` (`simulation_events`, `audit_log`) and `systems_seed.go` (count guard). Mutations that do go through services are `RecordConsumption`/`RecordProduction`/`ProcessExpiredItems`, `RegisterBirth`/`RegisterDeath`, and `CreateSystem`.
   > ⚠ See [#21](https://github.com/jstephenperry/vt-uos/issues/21).
3. **Services reach across domains at the repository level.** `resources.Service` constructs and uses `HouseholdRepository` and `ResidentRepository` directly for allocation and daily-requirement calculations rather than calling `population.Service` (`internal/services/resources/service.go`, `NewService`).
4. **Three independent service instances.** `simulation.New` builds its own `population`, `resources` and `facilities` services (`internal/simulation/engine.go`, `New`), and `tui.New` builds a second, separate set over the same `*sql.DB` (`internal/tui/app.go`, `New`); the engine's `Population()`/`Resources()`/`Facilities()` accessors have no callers outside the package. Each service in turn constructs its own repositories and `util.IDGenerator`.
5. **`client` depends on `tui`.** `internal/client` imports `internal/tui` for `Theme`, `NewTheme`, `DrawDoubleLine`, `DrawHorizontalLine` and `ProgressBar` (`internal/client/client.go`), which transitively pulls the database, services and simulation packages into the client binary path.
6. **Dead injection in `population.Service`.** `regNumGen` (`*util.RegistryNumberGenerator`) is constructed and stored but never called; registry numbers come from `ResidentRepository.GetNextRegistryNumber` (`internal/services/population/service.go`, `NewService`, `CreateResident`, `RegisterBirth`).
7. **Test-only migration runners bypass `Migrator`.** `testutil.TestDB.RunMigrations` (`internal/testutil/database.go`) and `runTestMigrations` (`internal/tui/testhelper_test.go`) each re-implement discovery of `internal/database/migrations/*.sql` from disk and split on the `-- +migrate Down` marker, instead of calling `database.NewMigrator`/`MigrateUp` against the embedded FS. Only `internal/server/server_test.go` and `internal/simulation/engine_test.go` exercise the real migrator.

Dependency direction otherwise holds: `config`, `database`, `util`, `models`, `repository`, and `protocol` import nothing above themselves.
