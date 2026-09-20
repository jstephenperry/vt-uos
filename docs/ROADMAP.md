# VT-UOS Roadmap

**Nothing planned in this document is implemented.** Every item under "Phase 6", "Phase 7" and "Carried-over design intent" is prefixed **Planned:** and is design intent carried over from the pre-rewrite docs; none of it exists in the code. The "Status" table is the only part that describes what the code at commit `34c3398` actually does.

## Status

| Module | State | As built |
| ------ | ----- | -------- |
| Population | Implemented | Resident and household models, repositories, service (`internal/services/population`), census list, resident form and death registration in the TUI (`internal/tui/views/population`, `internal/tui/app.go`). The birth path computes a Wright's-formula inbreeding coefficient and discards it (`internal/services/population/service.go`, `RegisterBirth`). `GetAncestry`, `GetDescendants`, `FindCommonAncestors` and `AssessCOIRisk` have no caller outside `internal/services/population/genetics.go`. |
| Resources | Implemented | Category/item/stock/transaction models, repository, service (`internal/services/resources`), inventory list and detail view (`internal/tui/views/resources`). Consumption draws lots by expiration date then receipt date (`internal/repository/resource_repo.go`, `ListStocks`). The inventory detail help line advertises `a:Adjust` and `u:Audit` but `handleResourceKeys` handles only `esc` in detail mode (`internal/tui/views/resources/inventory.go`, `internal/tui/app.go`). |
| Facilities | Implemented | System and maintenance-record models, repository, service (`internal/services/facilities`), systems list and detail view (`internal/tui/views/facilities`). Degradation and the maintenance sweep live in `internal/simulation/degradation.go` and write `facility_systems` and `maintenance_records` with direct SQL. `ScheduleMaintenance` and `CompleteMaintenance` have no caller outside `internal/services/facilities/service.go`. The systems detail help line advertises `m:Maintenance Log` but `handleFacilityKeys` handles only `esc` in detail mode (`internal/tui/views/facilities/systems.go`, `internal/tui/app.go`). |
| Simulation | Implemented | Engine with a 7-stage daily pipeline, incidents, alerts, event persistence, `vault_metadata` checkpoint and `VACUUM INTO` snapshots (`internal/simulation`). Control surface: TUI Simulation view (F11) and the HTTP API. |
| Master server | Implemented | `vtuos serve`: `net/http` JSON API, SSE stream, token-gated control and client-management routes, embedded web console (`internal/server`). |
| Client terminal | Implemented | `vtuos connect`: registration with retry, SSE consumer, heartbeat/telemetry, remote commands, kiosk lockout (`internal/client`). REFRESH and REBOOT commands only set a result message or a banner (`internal/client/client.go`, `applyCommands`). |
| Labor | Not implemented | `vocations` and `work_assignments` tables exist in `internal/database/migrations/001_initial.sql`, and the seed generator inserts `vocations` rows (`internal/database/seed/generator.go`). No model, repository, service or data-driven view exists; F6 renders fixed shift-roster numbers (`internal/tui/app.go`, `renderLabor`). |
| Medical | Not implemented | `medical_records` and `medical_conditions` tables exist in `internal/database/migrations/001_initial.sql`. The only writer is the simulation's illness incident, which inserts a `medical_conditions` row with direct SQL (`internal/simulation/incidents.go`, `incidentIllness`). F7 renders fixed health-summary numbers (`internal/tui/app.go`, `renderMedical`). |
| Security | Not implemented | `security_zones`, `access_log` and `security_incidents` tables exist in `internal/database/migrations/001_initial.sql`. The only writer is the simulation's security incident, which inserts a `security_incidents` row with direct SQL (`internal/simulation/incidents.go`, `incidentSecurity`). F8 renders a fixed zone list (`internal/tui/app.go`, `renderSecurity`). |
| Governance | Not implemented | `directives` and `audit_log` tables exist in `internal/database/migrations/001_initial.sql`. The simulation writes `audit_log` rows for each event it emits (`internal/simulation/events.go`, `persistEvent`); nothing reads them. F9 renders a fixed directive list (`internal/tui/app.go`, `renderGovernance`). |
| Quarters | Partial | `Quarters` is modeled with validation (`internal/models/household.go`) and seeded (`internal/database/seed/generator.go`, `generateQuarters`); no `QuartersRepository` exists in `internal/repository`. |
| TUI components | Partial | `Table`, `Input`, `Select` and `Form` exist in `internal/tui/components`. There is no `Modal`, `Tabs`, `Tree`, `Menu`, `Header`, `StatusBar`, `AlertBanner` or `Pagination` component; the quit confirmation, header, footer and alert bar are rendered inline by `internal/tui/app.go`. |
| Dashboard | Implemented | F2 renders live engine data when a snapshot is present and falls back to fixed sample rows otherwise (`internal/tui/app.go`, `hasLive`). |

## Phase 6 — Labor / Medical / Security / Governance

Tables for all four modules are already created by `internal/database/migrations/001_initial.sql`; Phase 6 is the Go layers on top of them. Column-level schema for the existing tables is documented in [DATABASE.md](DATABASE.md).

### Labor Allocation

**Purpose (planned):** Manage work assignments, shift scheduling, and workforce planning.

- **Planned:** Vocation Management - Define positions, requirements, headcounts
- **Planned:** Assignment Management - Assign residents to vocations
- **Planned:** Shift Scheduling - Three 8-hour shifts (Alpha, Beta, Gamma)
- **Planned:** Skill Matching - Match resident skills to vocation requirements
- **Planned:** Vacancy Tracking - Identify understaffed positions
- **Planned:** Succession Planning - Track aging workforce, training needs

**Planned:** Shift schedule:

| Shift | Hours | Typical Roles |
| ----- | ----- | ------------- |
| ALPHA | 0600-1400 | Administration, education, most services |
| BETA | 1400-2200 | Maintenance, production |
| GAMMA | 2200-0600 | Security, essential systems monitoring |

**Planned:** Assignment rules:

- Residents age 16-18: TRAINING assignments only
- Residents age 18-65: Full assignments
- Residents age 65+: Reduced hours, advisory roles
- Minimum rest: 8 hours between shifts
- Maximum: 1 PRIMARY + 1 SECONDARY assignment

**Planned:** Service interface (from the pre-rewrite `docs/MODULES.md`; no such type exists in the code):

```go
type LaborService interface {
    // Vocations
    CreateVocation(ctx context.Context, input CreateVocationInput) (*Vocation, error)
    GetVocation(ctx context.Context, id string) (*Vocation, error)
    ListVocations(ctx context.Context, filter VocationFilter) ([]Vocation, error)
    GetVocationStaffing(ctx context.Context, vocationID string) (*StaffingStatus, error)

    // Assignments
    AssignResident(ctx context.Context, input AssignmentInput) (*WorkAssignment, error)
    EndAssignment(ctx context.Context, assignmentID string, reason string) error
    GetResidentAssignments(ctx context.Context, residentID string) ([]WorkAssignment, error)

    // Scheduling
    GetShiftRoster(ctx context.Context, date string, shift string) (*ShiftRoster, error)
    GetResidentSchedule(ctx context.Context, residentID string, startDate, endDate string) (*Schedule, error)

    // Analysis
    GetStaffingReport(ctx context.Context) (*StaffingReport, error)
    GetVacancies(ctx context.Context) ([]Vacancy, error)
    FindQualifiedResidents(ctx context.Context, vocationID string) ([]Resident, error)
}
```

**Planned:** Storage already in place: `vocations` (code, title, department, required clearance/skills, authorized/minimum headcount, shift pattern, hazard level) and `work_assignments` (resident, vocation, assignment type PRIMARY/SECONDARY/TEMPORARY/TRAINING, shift ALPHA/BETA/GAMMA, status, performance rating).

**Planned:** TUI (F6): Assignments, Shift Roster, Vacancies, Vocations views.

### Medical

**Purpose (planned):** Track resident health, manage medical records, and monitor epidemiological trends.

- **Planned:** Patient Records - Medical history, encounters, treatments
- **Planned:** Condition Tracking - Chronic conditions, contagious diseases
- **Planned:** Radiation Monitoring - Individual and population exposure tracking
- **Planned:** Epidemiology - Disease spread, outbreak detection

**Planned:** Storage already in place: `medical_records` (record type EXAMINATION/TREATMENT/VACCINATION/INCIDENT/PSYCHOLOGICAL/RADIATION/CHRONIC_CONDITION/LAB_RESULT, diagnosis, treatment, vitals JSON, radiation dose and cumulative mSv, provider, encounter/follow-up dates, status, confidentiality level) and `medical_conditions` (condition code/name, onset/resolution, severity MILD/MODERATE/SEVERE/CRITICAL, chronic/genetic/contagious flags, treatment plan).

**Planned:** A path that returns a resident from `QUARANTINE` to `ACTIVE`; today the illness incident sets `QUARANTINE` and nothing clears it (`internal/simulation/incidents.go`).

**Planned:** TUI (F7): Patient Records, Active Conditions, Radiation Tracking, Epidemiology views.

### Security

**Purpose (planned):** Monitor access control, incident tracking, and vault security.

- **Planned:** Access Control - Zone restrictions, clearance enforcement
- **Planned:** Access Logging - Track all zone entry/exit
- **Planned:** Incident Management - Report, investigate, resolve security events
- **Planned:** Threat Assessment - Analyze security trends

**Planned:** Storage already in place: `security_zones` (zone code, sector, required clearance, restricted flag, access schedule JSON), `access_log` (resident, zone, access point, ENTRY/EXIT, GRANTED/DENIED/OVERRIDE/EMERGENCY, denial reason, override authoriser) and `security_incidents` (incident number, type ALTERCATION/THEFT/VANDALISM/UNAUTHORIZED_ACCESS/CONTRABAND/INSUBORDINATION/ASSAULT/OTHER, severity, parties as JSON arrays, status OPEN → CLOSED, resolution, disciplinary action).

**Planned:** TUI (F8): Access Log; Incidents (Open, All, Report Incident); Zone Management views.

### Governance

**Purpose (planned):** Manage vault directives, policies, and administrative decisions.

- **Planned:** Directive Management - Issue, track, supersede official directives
- **Planned:** Policy Enforcement - Link directives to system behaviors
- **Planned:** Audit Trail - Immutable log of all system changes
- **Planned:** Classification Control - Manage document access levels

**Planned:** Storage already in place: `directives` (directive number, type POLICY/EMERGENCY/OPERATIONAL/PERSONNEL/RESOURCE/SECURITY, summary and full text, issuer and authority level, affected departments/clearances as JSON, lifecycle DRAFT/ACTIVE/SUSPENDED/SUPERSEDED/RESCINDED with supersession links, classification GENERAL/RESTRICTED/CONFIDENTIAL/OVERSEER_ONLY) and `audit_log` (actor type USER/SYSTEM/SIMULATION, action, entity type/id, old/new values JSON, session, IP, terminal).

**Planned:** Audit rows for operator actions; today only the simulation writes `audit_log`, with `actor_type='SIMULATION'` and `terminal_id='SIM'` (`internal/simulation/events.go`).

**Planned:** TUI (F9): Directives (Active, All, Issue Directive); Audit Log views.

### Cross-module (Phase 6)

- **Planned:** Vocation and quarters management on top of the existing `vocations`, `quarters` tables and the `Quarters` model: a `QuartersRepository`, vocation/quarters assignment in the resident form.
- **Planned:** Household views (F3 → Households), Vital Records (Register Birth / Register Death / View Records), Demographics and Lineage Browser screens, from the pre-rewrite `docs/TUI.md` navigation structure. The service methods behind Demographics (`GetAgeDistribution`, `GetSexDistribution`, `ProjectPopulation`, `GetWorkforceStats`) and Lineage (`GetAncestry`, `GetDescendants`) already exist without a view.
- **Planned:** Resource screens for Transactions, Rationing (Allocation Table, Adjust Ration Class), Expiring Items and Forecasting. `GetTransactionHistory`, `GetExpiringItems`, `GetResourceRunway` and `CalculateHouseholdAllocation` exist in `internal/services/resources/service.go` without a view.
- **Planned:** Facility screens for Maintenance (Schedule, Overdue, Create Work Order) and Telemetry. `ListMaintenanceRecords`, `ScheduleMaintenance` and `CompleteMaintenance` exist without a view; `facility_systems.telemetry_json` / `telemetry_updated_at` exist in the schema and are not mapped by `internal/models/facility.go`.

## Phase 7 — Polish

- **Planned:** Dashboard driven entirely by live data; remove the fixed fallback rows in `internal/tui/app.go` (Power 0.98, Water 0.95, HVAC 0.92, Security 1.0).
- **Planned:** Inbreeding warnings (the `RegisterBirth` comment in `internal/services/population/service.go` says "Log warning but don't prevent birth"; the formatted warning is assigned to `_`) and low-stock alerts from `GetResourceRunway` (the pre-rewrite README listed "low-stock alerts"; today resource alerts come from the same-day shortfall check in `internal/simulation/consumption.go` and the runway thresholds in `internal/simulation/metrics.go`).
- **Planned:** Death-registration confirmation and a cause-of-death input, per the "Register death - show confirmation" comment in `internal/tui/app.go`; today `d` in the resident detail view fires `registerDeath` immediately with the fixed cause "Cause pending investigation".
- **Planned:** Handlers for the keys the UI already advertises: `a`/`u` on the inventory detail (`internal/tui/views/resources/inventory.go`), `m` on the systems detail (`internal/tui/views/facilities/systems.go`), and the `KeyMap` entries `Help` (`?`), `Left`, `Right`, `Home`, `End` in `internal/tui/keys.go`, none of which `handleKeyPress` consults.
- **Planned:** List-view keys from the pre-rewrite `docs/TUI.md`: `n`/`N` next/previous search result, `s` sort options, `r` refresh (today `s` enters search mode in the census list).
- **Planned:** Components from the pre-rewrite `docs/TUI.md` component library that do not exist: `Modal` (confirm/alert/info with buttons), `Tabs`, `Tree` (family trees, org charts), `Menu`, `Header`, `StatusBar`, `AlertBanner`, `Pagination` as reusable types.
- **Planned:** Settings screen (Vault Configuration, Simulation Controls, User Preferences, About) from the pre-rewrite `docs/TUI.md` navigation structure.
- **Planned:** Kiosk mode that disables "quit and destructive actions" as the `exhibit.kiosk` field comment in `internal/config/config.go` states; today `handleKeyPress` blocks only `q` and `F10`, while `Ctrl+C` opens the quit dialog and `a`/`e`/`d` and the simulation controls remain live (`internal/tui/app.go`).
- **Planned:** REFRESH as "force a state refresh" and REBOOT as "reconnect/restart the client loop", as the `CommandType` comments in `internal/protocol/protocol.go` describe; today `applyCommands` replies "refreshed" for REFRESH and shows a 5-second banner for REBOOT, and a heartbeat failure sets `connected=false` without re-registering (`internal/client/client.go`).
- **Planned:** Test coverage toward the 80% threshold in the Makefile's `test-coverage-check` target; total is 41.8% with no test files in `config`, `database`, `database/seed`, `util`, `protocol`, `client`, `testutil` or any `services/*` package.
- **Planned:** Migration rollback and status from the CLI (the pre-rewrite `docs/CONFIGURATION.md` described `migrate status|up|down` subcommands); `Migrator.MigrateDown`, `MigrateTo`, `DryRun` and `Status` exist in `internal/database/migrate.go` with no caller outside that file, and `cmd/vtuos/main.go` calls only `MigrateUp`.
- **Planned:** Optimization pass for the Pi Zero 2W target (512MB RAM).

## Carried-over design intent from the other rewritten docs

Design intent removed from the pre-rewrite `docs/DEVELOPMENT.md`, `docs/CONFIGURATION.md`, `docs/MODULES.md`, `docs/SIMULATION.md`, `docs/API.md`, `docs/DEPLOYMENT.md` and `docs/TUI.md` because the code does not implement it. Illustrative code samples, the example Makefile, screen mockups and the colour-constant block from the old docs were not carried over; they were examples, not plans.

### Population (from the old MODULES.md)

- **Planned:** Vital Records - Birth registration, death registration, marriage/union records (no union/marriage concept exists in `internal/models` or the schema)
- **Planned:** Search & Filter - Find residents by name, status, vocation, household, etc.; today `ResidentFilter` in `internal/models/resident.go` carries status, household, vocation, sex, age and entry-type fields, but the census view sets only `SearchTerm` (surname/given names) and `Status` (`internal/tui/views/population/census.go`, `SetSearch`, `SetStatusFilter`)
- **Planned:** Population projection at 5, 10, 25, 50 year intervals with an alert if the trajectory threatens viability; today `ProjectPopulation` returns one point per year for the requested span and `assessViability` fills a struct that nothing displays (`internal/services/population/demographics.go`)

### Resources (from the old MODULES.md)

- **Planned:** Daily consumption estimate as `SUM(member_calorie_needs based on age, sex, activity) × ration_class_modifier` and `SUM(member_water_needs)`; today the formula is ration-class target × member count (`internal/services/resources/service.go`, `CalculateHouseholdAllocation`, `GetVaultDailyRequirements`)
- **Planned:** Rationing - Calculate and enforce allocation by household/ration class (the calculation exists; nothing enforces it)
- **Planned:** Expiration alerts on items expiring within 30, 7, 1 days; today the only expiration-related event is `SPOILAGE`, emitted when three or more lots expire in one day (`internal/simulation/consumption.go`, `processExpirations`)
- **Planned:** Consumption Tracking - Record all resource usage with attribution; today engine consumption is attributed to `RelatedEntityType: "VAULT"` (`internal/simulation/consumption.go`, `consumeByQuantity`)

### Facilities (from the old MODULES.md)

- **Planned:** Work Orders - Create, assign, track maintenance work
- **Planned:** Parts Management - Track parts consumption from resource inventory; today the maintenance sweep draws a flat 1.0 unit of `PARTS` per service (`internal/simulation/degradation.go`, `performMaintenance`)
- **Planned:** Failure Prediction - MTBF-based alerts; `mtbf_hours` is stored (`internal/models/facility.go`) and not read by any alert or incident rule
- **Planned:** Dependency Mapping - Understand system interdependencies
- **Planned:** Telemetry - `RecordTelemetry(ctx, systemID, telemetry map[string]any)` over the existing `telemetry_json` / `telemetry_updated_at` columns
- **Planned:** Efficiency impact: systems below 50% efficiency reported CRITICAL; POWER degradation cascades to dependent systems; HVAC failure triggers population health events. Today each system degrades independently and the power balance is only reported (`internal/simulation/metrics.go`, `buildPower`)
- **Planned:** Service methods from the old design-time `FacilityService` interface that do not exist: `UpdateSystemStatus`, `RecordTelemetry`, `GetMaintenanceSchedule`, `GetOverdueMaintenance`, `CompleteMaintenanceRecord(ctx, id, outcome)`, `GetSystemHealth`, `GetFailurePredictions`, `GetDependencyGraph`

### Simulation (from the old MODULES.md and SIMULATION.md)

- **Planned:** Scheduled-event queue with `GetPendingEvents`, `QueueEvent` and `GetEventHistory(filter, page)`; today there is no queue — incidents are rolled once per day and events are an in-memory ring of 200 plus `simulation_events` rows (`internal/simulation/events.go`)
- **Planned:** Event probability modifiers: population density affects conflict probability; time since seal affects external threat probability. Today the incident weights use average system efficiency, resource scarcity and load percentage only (`internal/simulation/incidents.go`, `rollIncidents`)
- **Planned:** Event categories not yet generated: accidents, conflicts (as population events), breaches and external threats (security), recovered data and innovations (discovery)
- **Planned:** Design goals stated in the old SIMULATION.md that the built engine does not meet: all writes through the service layer (several stages use direct SQL); a seed derived from the vault number and seal date (`deriveSeed` uses `simulation.start_date`); replay-identical live runs (incident weights read a snapshot recomputed once per batch, and row IDs use `crypto/rand`); resume where processing left off (`saveState` checkpoints `clock.Now()`, not the processed position) (`internal/simulation/engine.go`, `internal/simulation/persistence.go`)
- **Planned:** A `SimulationEngine` interface with `Pause(ctx) error`, `Resume(ctx) error`, `GetVaultTime`, `AdvanceTime`; today `Pause`/`Resume` take no context and return nothing

### CLI and configuration (from the old CONFIGURATION.md)

- **Planned:** Interactive first-run setup (vault designation, starting population, initial resource stocks, facility configuration) and creation of an initial Overseer account; today first run writes `config.Default()` to disk silently, and `overseer.initial_overseer_id` is never read outside `internal/config/config.go`
- **Planned:** `vtuos backup`, `vtuos restore <file>` and `vtuos reset --confirm` subcommands alongside the `migrate status|up|down` item above; today backup runs only from the scheduler and the snapshot endpoint (`internal/database/database.go`, `Backup`) and restore only via the `-restore` flag
- **Planned:** Configurable memory knobs for the Pi Zero 2W target: `database.cache_size_mb`, `database.max_connections`, `simulation.batch_size`, `simulation.event_buffer`; today `cache_size` is a fixed `-16000` pragma, the pool is fixed at one connection, and the event buffer is a fixed 200 (`internal/database/database.go`, `internal/simulation/engine.go`)
- **Planned:** Multi-user hardening: an authentication layer, database encryption (SQLCipher), role-based access control; today there is one shared `server.admin_token` with no per-user identity (`internal/server/auth.go`)

### TUI (from the old TUI.md)

- **Planned:** Minimal animation - cursor blink, occasional scan lines; `display.scan_lines` and `display.flicker` are parsed (`internal/config/config.go`) and read nowhere else
- **Planned:** Shell-level `Tab` / `Shift+Tab` element navigation (today only `components.Form` handles them) and `Ctrl+C` as a force quit (today it opens the confirm dialog)
- **Planned:** Dashboard sub-panels as designed: Population Summary, Resource Status, System Status, Active Alerts
- **Planned:** `Table` with sortable columns (`SortColumn`, `SortDir`, `Column.Sortable`) and a typed `Form.Field` with `FieldType` and `Validator`; today `components.Table` has no sort state, the `FormField` interface has no validate method, and the only validation is `Input.Validate`'s `required` check (`internal/tui/components/table.go`, `internal/tui/components/input.go`)
- **Planned:** Resources › Inventory › Search and Facilities › System Status › Critical Only views

### Testing and build (from the old DEVELOPMENT.md)

- **Planned:** Unit tests for every service method; repository methods against in-memory SQLite; simulation calculations with known inputs/outputs; minimum 80% coverage for `internal/services/` (today 0.0%: no test files in any `internal/services/*` package)
- **Planned:** Integration tests behind `make test-integration` (`-tags=integration`) covering full CRUD workflows per module, engine tick processing and migration up/down; today no Go file carries an `integration` build tag, so the target runs the same tests as `make test`
- **Planned:** Property-based tests (COI for random family trees, consumption calculations, projection accuracy); `gopter` is not in `go.mod`
- **Planned:** Test fixtures `testdata/seed/day1/`, `testdata/seed/year10/`, `testdata/seed/crisis/`; today `testdata/` holds two TOML files
- **Planned:** PR checklist items "Lint passes" and "CHANGELOG updated"; there is no `CHANGELOG` file in the repository
- **Planned:** Makefile targets `build-linux-arm64`, `vet`, `migrate-up`, `migrate-down`, `migrate-status`, `deps` (today: `build-pi`, a single `migrate` target that runs `--migrate-only`, and no vet/deps targets)
- **Planned:** Phase 7 items from the old implementation order: reporting and exports; performance optimization (indexes, prepared statements, batched inserts, `EXPLAIN QUERY PLAN`; streamed result sets, `pprof`; debounced TUI updates, partial re-render, lazy loading, cached values)

