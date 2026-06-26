# VT-UOS Development Progress

**Last Updated:** 2026-06-25

## Current Status

**Phase 5: Simulation Control Core - COMPLETE**

The vault now runs as authoritative operations software: a deterministic engine
advances time and drives real consumption, production, facility degradation,
maintenance, operational incidents, and demographics against the database. It is
deployable as a master server with an embedded web administration console and
managed client display/operator terminals.

- **Simulation control core** (`internal/simulation`) — see [docs/SIMULATION.md](docs/SIMULATION.md)
- **Master server + web console** (`internal/server`) — see [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- **Client terminals** (`internal/client`) — `vtuos connect`
- **Operator surfaces** — Simulation Control view (F11), live-wired dashboard,
  optional boot/attract/kiosk exhibit mode

---

## Completed Phases

### Phase 5: Simulation Control Core ✅
- Deterministic tick engine: consumption (FIFO), production, expiration,
  facility degradation, maintenance sweep, weighted operational incidents,
  demographics (births/deaths)
- Persistence to `simulation_events` + `audit_log`; power-loss continuity via
  `vault_metadata`; snapshot (`VACUUM INTO`) + `--restore` reset
- Self-healing core facility systems (`EnsureCoreSystems`)
- `protocol.VaultState` snapshot published to TUI, web console, and clients
- Master server: stdlib `net/http` JSON API, SSE stream, health endpoint,
  token-gated control, client registry with telemetry + remote operations,
  embedded `html/template` web console
- Client terminal: registers, streams/renders live state, reports telemetry,
  executes remote ops; kiosk lockout + auto-reconnect
- TUI: Simulation Control view, live dashboard, boot/attract/kiosk exhibit mode
- Subcommands: `serve`, `connect`, default local console

### Phase 1: Foundation ✅
- Configuration loading (TOML)
- Database connection and migrations
- TUI shell with Bubble Tea
- Vault clock for time simulation

### Phase 2: Population Module MVP ✅
- Resident and Household models
- Repository layer for data access
- Population service with business logic
- Census TUI view with table display
- Resident forms (add/edit)
- Search functionality
- Death registration
- Seed data generation (500 residents, 200 households)

### Phase 3: Resource Management ✅
- Resource models (category, item, stock, transaction)
- Repository layer with CRUD operations
- Resource service with business logic:
  - FIFO consumption tracking
  - Production recording
  - Expiration management
  - Runway forecasting
  - Ration allocation by household
  - Inventory auditing
- Seed data (8 categories, 28 items, 28 stocks)
- Inventory TUI view with:
  - Table display (Item Code, Name, Category, Quantity, Unit, Status, Expires)
  - Detail view with color-coded expiration warnings
  - Category filtering ('c' key)
  - Pagination support
  - F4 navigation

---

## Files Created in Phase 3

```
internal/models/resource.go
internal/repository/resource_repo.go
internal/services/resources/service.go
internal/services/resources/types.go
internal/tui/views/resources/inventory.go
```

## Files Modified in Phase 3

```
internal/database/seed/names.go      - Added ResourceCategories, ResourceItems
internal/database/seed/generator.go  - Added generateResources()
internal/tui/app.go                  - Integrated resource views, F4 nav
```

---

## Database State

- **500 residents** (seeded)
- **200 households** (seeded)
- **8 resource categories**: FOOD, WATER, MEDICAL, POWER, PARTS, CLOTHING, TOOLS, CHEMICALS
- **28 resource items**: Various consumables and equipment
- **28 stock records**: Initial inventory based on population

Database location: `~/.local/share/vtuos/vault.db`

---

## Next Phase: Phase 4 - Facility Operations

Per `docs/MODULES.md` and `docs/DATABASE.md`, implement:

1. **Models** (`internal/models/facility.go`):
   - Facility (id, code, name, type, status, efficiency, power_consumption, etc.)
   - MaintenanceSchedule (facility_id, task_type, interval_days, last_performed, next_due)
   - MaintenanceLog (facility_id, performed_date, task_type, performed_by, notes)
   - FacilityAlert (facility_id, alert_type, severity, message, acknowledged)

2. **Repository** (`internal/repository/facility_repo.go`):
   - CRUD for facilities
   - Maintenance schedule management
   - Alert tracking

3. **Service** (`internal/services/facilities/`):
   - System monitoring
   - Maintenance scheduling
   - Failure prediction
   - Efficiency calculations
   - Alert generation

4. **TUI Views** (`internal/tui/views/facilities/`):
   - Facility list view
   - Facility detail view
   - Maintenance schedule view
   - F5 navigation integration

5. **Seed Data**:
   - Core vault facilities (Power Plant, Water Treatment, HVAC, etc.)
   - Initial maintenance schedules

---

## Key Commands

```bash
# Build
make build

# Run
./bin/vtuos

# Regenerate database with seed data
rm ~/.local/share/vtuos/vault.db && ./bin/vtuos --seed

# Check database
sqlite3 ~/.local/share/vtuos/vault.db ".tables"
```

---

## TUI Navigation Reference

| Key | Action |
|-----|--------|
| F1 | Help |
| F2 | Dashboard |
| F3 | Population Registry |
| F4 | Resource Management |
| F5 | Facility Operations (not yet implemented) |
| F6-F9 | Other modules (not yet implemented) |
| F10 | Quit |

---

## Architecture Notes

- **Layered architecture**: Models → Repository → Service → TUI
- **No CGO**: Pure Go SQLite driver (modernc.org/sqlite)
- **Target**: Raspberry Pi (512MB RAM minimum)
- **Patterns**: Follow existing code in Phase 2/3 implementations
