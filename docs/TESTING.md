# Testing (As Built)

What the test suite actually contains at commit `34c3398`, how it is put together, and where it is empty. Per-package figures come from `go test -count=1 -cover ./internal/...`; the total comes from `go tool cover -func` over the profile written by `go test -race -count=1 -coverprofile=coverage.out ./internal/...`. See [DEVELOPMENT.md](DEVELOPMENT.md) for the Makefile wrappers.

## Summary

- 19 `_test.go` files across 10 packages; 0 failures, 0 data races under `-race`.
- Total statement coverage: **41.8%**. `make test-coverage-check` requires 80% and fails.
  > ⚠ See [#7](https://github.com/jstephenperry/vt-uos/issues/7).
- No build tags, no golden files, no property-based tests, no benchmarks. `github.com/charmbracelet/x/exp/golden` appears in `go.mod` only as an indirect dependency of `teatest`.
- Every test that needs a database uses in-memory SQLite (`:memory:`) with the real migration SQL applied; there are no fixture databases on disk.

## Coverage by Package

Approach key: **unit** = direct function calls with hand-written assertions; **table** = `[]struct{...}` cases with `t.Run`; **subtests** = `t.Run` per scenario against a shared fixture; **httptest** = `httptest.NewRequest`/`NewRecorder` against the real mux; **teatest** = the real Bubble Tea program in a headless terminal.

| Package | Test files | Approach | Statements covered |
| --- | --- | --- | --- |
| `internal/client` | none | — | 0.0% (no test files) |
| `internal/config` | none | — | 0.0% (no test files) |
| `internal/database` | none | — | 0.0% (no test files) |
| `internal/database/seed` | none | — | 0.0% (no test files) |
| `internal/models` | `facility_test.go`, `household_test.go`, `resident_test.go`, `resource_test.go` | table (38 test funcs, 33 case tables) | 83.5% |
| `internal/protocol` | none | — | 0.0% (no test files) |
| `internal/repository` | `facility_repo_test.go`, `household_repo_test.go`, `resident_repo_test.go` | subtests over a migrated in-memory DB via `testutil` (27 test funcs, 49 subtests) | 44.0% |
| `internal/server` | `server_test.go` | httptest over a seeded engine (5 test funcs) | 47.2% |
| `internal/services/facilities` | none | — | 0.0% (no test files) |
| `internal/services/population` | none | — | 0.0% (no test files) |
| `internal/services/resources` | none | — | 0.0% (no test files) |
| `internal/simulation` | `engine_test.go` | unit over a seeded in-memory engine driven by `Step` (7 test funcs) | 72.5% (72.7% under `-race`; the package has timing-dependent test paths) |
| `internal/testutil` | none | — | 0.0% (no test files) |
| `internal/tui` | `app_test.go`, `e2e_test.go`, `layout_test.go`, `testhelper_test.go` | unit on `App.Update`/`View` (47 funcs) + teatest e2e (20 funcs) + table/unit layout helpers (13 funcs) | 68.6% |
| `internal/tui/components` | `input_test.go`, `table_test.go` | unit (38 test funcs) | 82.5% |
| `internal/tui/views/facilities` | `systems_test.go` | unit, views constructed with a `nil` service (16 funcs) | 79.2% |
| `internal/tui/views/population` | `census_test.go` | unit, `nil` service (18 funcs) | 36.4% |
| `internal/tui/views/resources` | `inventory_test.go` | unit, `nil` service (16 funcs) | 60.7% |
| `internal/tui/views/simulation` | `control_test.go` | unit, renders a hand-built `protocol.VaultState` through a plain styler (2 funcs) | 85.7% |
| `internal/util` | none | — | 0.0% (no test files) |
| `cmd/vtuos` | none | — | not in the `./internal/...` run |

Total (`go tool cover -func`): **41.8%**.

## Harnesses in Use

Each package that touches a database builds its own fixture. There are four different ways the schema gets applied:

| Package | Database | Migrations applied by | Seed data |
| --- | --- | --- | --- |
| `internal/repository` | `testutil.NewTestDB(t)` (`*sql.DB`, `:memory:`, `PRAGMA foreign_keys = ON`) | `TestDB.RunMigrations(t, "../../internal/database/migrations")` — reads `*.sql` from disk, truncates at `-- +migrate Down`, `ExecContext`s each file | `testutil.Fixture*` structs inserted through the repository under test |
| `internal/simulation` | `database.NewInMemory()` (`*database.DB`) | `database.NewMigrator(db).MigrateUp(ctx)` — the production path | `seed.NewGenerator` with 80 residents, 12 family + 10 single households, `RandomSeed: 2077`; `Simulation.Enabled = false`; clock paused at 2077-10-23T09:47Z; `Engine.Start` |
| `internal/server` | `database.NewInMemory()` | `database.NewMigrator(db).MigrateUp(ctx)` | `seed.NewGenerator` with 60 residents, `RandomSeed: 7`; `AdminToken = "secret"`; `EnableWeb = true`; started engine with paused clock |
| `internal/tui` | `database.NewInMemory()` | `runTestMigrations` in `testhelper_test.go` — same disk-read-and-split approach as `testutil`, duplicated locally | none; views start empty |

The `internal/tui` and `internal/testutil` runners do not go through `database.Migrator`, so a regression in the embedded-FS loader or statement splitter would only be caught by the simulation and server suites.

### `internal/database/testhelper.go`

```go
func NewInMemory() (*DB, error)
```

Opens `sqlite` at `:memory:`, sets `SetMaxOpenConns(1)`/`SetMaxIdleConns(1)`, runs `PRAGMA foreign_keys = ON`, and returns a `*database.DB` with an empty `config.DatabaseConfig` and no backup directory. It does not run migrations, does not enable WAL, and starts no backup scheduler. Used by `server_test.go`, `engine_test.go`, `e2e_test.go` and `testhelper_test.go`.

### `internal/testutil` (imported only by `internal/repository` tests)

`database.go`:

```go
type TestDB struct { *sql.DB; path string }

func NewTestDB(t *testing.T) *TestDB                          // :memory:, foreign keys on
func NewTestDBWithFile(t *testing.T) *TestDB                  // t.TempDir()/test.db
func (tdb *TestDB) RunMigrations(t *testing.T, migrationsDir string)
func (tdb *TestDB) RunSchema(t *testing.T, schemaPath string)
func (tdb *TestDB) Close(t *testing.T)
func (tdb *TestDB) Truncate(t *testing.T, tables ...string)   // FK off, DELETE FROM each, FK on, in one tx
func (tdb *TestDB) AssertRowCount(t *testing.T, table string, expected int)
func (tdb *TestDB) ExecSQL(t *testing.T, sql string, args ...any)
```

`fixtures.go` — every fixture returns a pointer with sensible defaults and applies functional overrides in order:

```go
func FixtureResident(overrides ...func(*models.Resident)) *models.Resident
func FixtureFemaleResident(overrides ...func(*models.Resident)) *models.Resident
func FixtureVaultBornResident(parent1ID, parent2ID string, overrides ...func(*models.Resident)) *models.Resident
func FixtureDeceasedResident(overrides ...func(*models.Resident)) *models.Resident
func FixtureHousehold(overrides ...func(*models.Household)) *models.Household
func FixtureIndividualHousehold(overrides ...func(*models.Household)) *models.Household
func FixtureDissolvedHousehold(overrides ...func(*models.Household)) *models.Household
func FixtureQuarters(overrides ...func(*models.Quarters)) *models.Quarters
func FixtureResourceCategory(overrides ...func(*models.ResourceCategory)) *models.ResourceCategory
func FixtureResourceItem(categoryID string, overrides ...func(*models.ResourceItem)) *models.ResourceItem
func FixtureResourceStock(itemID string, overrides ...func(*models.ResourceStock)) *models.ResourceStock
func FixtureResourceTransaction(itemID string, overrides ...func(*models.ResourceTransaction)) *models.ResourceTransaction
func FixtureFacilitySystem(overrides ...func(*models.FacilitySystem)) *models.FacilitySystem
func FixtureDegradedSystem(overrides ...func(*models.FacilitySystem)) *models.FacilitySystem
func FixtureMaintenanceRecord(systemID string, overrides ...func(*models.MaintenanceRecord)) *models.MaintenanceRecord
func StringPtr(s string) *string
func IntPtr(i int) *int
func Float64Ptr(f float64) *float64
func TimePtr(t time.Time) *time.Time
```

Fixture IDs are `uuid.New().String()` (random v4), not the `util.NewID()` time-ordered IDs production uses. `FixtureResident` sets `RegistryNumber` to `"VT-076-" + id[:8]`, which is not the `V076-NNNNN` shape `util.RegistryNumberGenerator` produces.

Typical use (`internal/repository/resident_repo_test.go`):

```go
func setupTestDB(t *testing.T) *testutil.TestDB {
    t.Helper()
    db := testutil.NewTestDB(t)
    migrationsDir := filepath.Join("..", "..", "internal", "database", "migrations")
    db.RunMigrations(t, migrationsDir)
    return db
}

db := setupTestDB(t)
defer db.Close(t)
repo := NewResidentRepository(db.DB)
resident := testutil.FixtureResident()
err := repo.Create(ctx, nil, resident)
```

Helpers that no test currently calls: `NewTestDBWithFile`, `RunSchema`, `Truncate`, `AssertRowCount`, `ExecSQL`, `FixtureQuarters`, all four `FixtureResource*` fixtures, and the four pointer helpers.

### `internal/tui` helpers (`testhelper_test.go`, `e2e_test.go`)

```go
func newTestApp(t *testing.T) *App        // migrated in-memory DB, config.Default(), paused clock at 2077-10-23T09:47Z, width 120 × height 40, ready=true
func runTestMigrations(t *testing.T, db *database.DB, migrationsDir string)
func keyMsg(key string) tea.KeyMsg        // KeyRunes
func specialKeyMsg(keyType tea.KeyType) tea.KeyMsg
func newE2EApp(t *testing.T) *App         // same DB/config/clock, no size (teatest sends WindowSizeMsg)
func waitFor(t *testing.T, tm *teatest.TestModel, text string)  // teatest.WaitFor on tm.Output(), 5 s
```

Unit tests call `app.Update(keyMsg("q"))` and inspect fields or `app.View()`. E2E tests do `teatest.NewTestModel(t, newE2EApp(t), teatest.WithInitialTermSize(120, 40))`, send keys with `tm.Send`, and assert on rendered bytes. No TUI test attaches an engine, so the F11 simulation view, dashboard live panels, kiosk, boot and attract paths are exercised only where they render without one.

### `internal/simulation/engine_test.go`

`newTestEngine(t)` returns a started engine over a seeded in-memory DB with the clock paused; every test advances time with `Step`, so the automatic `run` loop never advances in any test. `TestEngineDeterministic` builds two engines over identically seeded databases, steps 720 hours, and compares tick count, births, deaths and `simulation_events` count. `TestEngineConcurrentSubscribeBroadcast` runs subscribers and steps under a `sync.WaitGroup`.

### `internal/server/server_test.go`

`newTestServer(t)` returns a `*Server` and its engine; `do(t, h, method, path, body, headers)` drives `srv.routes()` through `httptest`. Tests cover `/healthz`, `/api/v1/state`, the 401/200 split on `X-Admin-Token`, `/api/v1/sim/step` advancing `TickCount`, the client register → command → heartbeat → list lifecycle including a bad-token 401, and that `GET /` and `GET /static/app.js` serve.

## How to Run

Commands as they appear in `Makefile`:

```bash
make test                      # go test -v -race -cover ./internal/...
make test-coverage             # + -coverprofile=coverage.out, coverage.html, prints total
make test-pkg PKG=./internal/models
make test-coverage-check       # exits 1 below 80% total (currently fails)
make test-integration          # -tags=integration; no files carry the tag, so identical to make test
make test-all                  # test + test-integration
```

Invocations that produced the numbers above:

```bash
go test -count=1 -cover ./internal/...                            # per-package column
go test -race -count=1 -coverprofile=coverage.out ./internal/...  # total, via go tool cover -func
```

`cmd/vtuos` is outside `./internal/...` and is never compiled by the test targets.

## Known Gaps

Stated as facts about the current tree.

### Packages with no test files

`internal/client`, `internal/config`, `internal/database`, `internal/database/seed`, `internal/protocol`, `internal/services/facilities`, `internal/services/population`, `internal/services/resources`, `internal/testutil`, `internal/util`, and `cmd/vtuos`.

Consequences worth knowing:

- All three service packages — the business-logic layer — have zero direct tests. `population.Service` (birth/death registration, households, demographics, COI), `resources.Service` (FIFO/FEFO consumption, production, expiry, runway, allocation) and `facilities.Service` (create/update/schedule/complete maintenance, stats) are exercised only indirectly through the engine, TUI and server suites.
- `internal/database`'s `Open`, pragmas, `Backup`, `cleanOldBackups`, `Close`, `WithTransaction`, `AttemptRecovery`, and `Migrator.MigrateDown`/`MigrateTo`/`DryRun`/`Status` have no test. `NewMigrator` + `MigrateUp` are proven only by being fixture setup in other packages.
- `config.Load`, `Save`, every `Validate`, and the XDG path resolution have no test.
- `util.NewID`, `RegistryNumberGenerator`, `VaultClock` and every date helper have no direct test.
- The whole client terminal (`Conn`, reconnect loop, remote commands, kiosk lockout, rendering) has no test.
- `internal/repository/resource_repo.go` has no test file; only the resident, household and facility repositories are tested.

### Untested behaviour inside tested packages

- `internal/simulation`: the automatic run loop advancing time; `lightweightRefresh`; production, expirations, degradation thresholds, maintenance sweep, every incident handler, alert evaluation, `Events`/`Alerts`/`AcknowledgeAlert`, `CreateSnapshot`, `Stop` persisting state.
- `internal/server`: the SSE stream, `Authorization: Bearer` parsing, open mode with an empty token, `/clients/{id}/result`, registry cleanup/online computation, `decodeJSON` size limit, protocol-mismatch 409, `Shutdown`.
- `internal/tui`: anything requiring an attached engine, kiosk gating, boot sequence, attract mode, `saveResident`/`registerDeath`, the resource `c` category cycle.
- `internal/tui/views/*`: the `Apply*`/`SetLoadError`/`QueryParams` data-path methods — every view test constructs the view with a `nil` service and never loads rows. `ResidentForm` has no test file.
- `internal/repository`: `GetByDesignation`, `GetNextDesignation`, `GetByRationClass` on households.

### Drift from the documented testing plan

The earlier DEVELOPMENT.md described integration tests for "simulation engine tick processing" and "database migration up/down", property-based tests with `gopter`, and seed fixture directories under `testdata/seed/`. None of these exist; `gopter` is not in `go.mod`, `testdata/` holds two TOML files, and the engine tests drive `Step` only. That text now lives in [ROADMAP.md](ROADMAP.md).
