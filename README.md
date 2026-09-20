<div align="center">

# 🏛️ VT-UOS: Vault-Tec Unified Operating System

**The realistic vault management system that Vault-Tec should have built**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20arm64-lightgrey)](https://github.com/jstephenperry/vt-uos)

[Features](#-features) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [Roadmap](#-roadmap) • [Contributing](#-contributing)

</div>

---

## 📖 Overview

VT-UOS is a **realistic vault population management and operations system** inspired by the Fallout universe. Unlike terminal emulators that replicate the in-game aesthetic, VT-UOS is the **actual operational software** that Vault-Tec would have deployed to manage sealed underground communities for multi-generational survival.

This isn't a game—it's a simulation of managing a sealed population in a closed-loop life support environment, built to run on a Raspberry Pi.

### 🎯 Why VT-UOS?

- **Authentic Operations**: Real population records, resource ledgers and facility maintenance, driven by a simulation core that consumes, produces, degrades and breaks things every vault day
- **Realistic Constraints**: Built for resource-constrained hardware (Raspberry Pi, 512MB RAM target)
- **Offline-First**: The local console needs no network; master/client mode is plain HTTP on the LAN (`internal/server/server.go`)
- **Power-Loss Aware**: SQLite in WAL mode with a startup integrity check, WAL recovery and restore-from-backup (`internal/database/database.go`, `internal/database/recovery.go`)
- **Single Static Binary**: One 15 MB executable, `CGO_ENABLED=0`, no runtime dependencies
- **Terminal UI**: Green phosphor, amber or white theme with keyboard-only navigation (`internal/tui/styles.go`)

## ✨ Features

### 🧑‍🤝‍🧑 Population Management (implemented)

- Residents carry a registry number (`V076-00001` style), sex, blood type, entry type, status, two biological-parent links, household/quarters/vocation references and a 1–10 clearance level (`internal/models/resident.go`, `internal/repository/resident_repo.go`)
- Households have a type, status and one of five ration classes, each with a fixed daily calorie and water target (`internal/models/household.go`)
- Census list (F3) with search (`/`), pagination, an add/edit resident form (`a`, `e`) and death registration from the detail view (`d`) (`internal/tui/views/population`, `internal/tui/app.go`)
- Birth and death registration, household creation and assignment, and population status counts in the service layer (`internal/services/population/service.go`)
- Age and sex distribution, workforce stats and a year-by-year population projection (`internal/services/population/demographics.go`)
- Wright's-formula coefficient of inbreeding is computed during birth registration (`internal/services/population/genetics.go`); the result is assigned to `_` and no view, alert or log reports it
  > ⚠ See [#22](https://github.com/jstephenperry/vt-uos/issues/22) and [#26](https://github.com/jstephenperry/vt-uos/issues/26).

### 📦 Resource Management (implemented)

- 8 seeded resource categories: FOOD, WATER, MEDICAL, POWER, PARTS, CLOTHING, TOOLS, CHEMICALS (`internal/database/seed/names.go`)
- Stock lots with expiration dates; consumption draws lots in expiration-date order, then receipt date (`internal/repository/resource_repo.go`, `internal/services/resources/service.go`, `RecordConsumption`)
- A transaction ledger with CONSUMPTION, PRODUCTION, ADJUSTMENT, SPOILAGE, TRANSFER and AUDIT_CORRECTION entries (`internal/models/resource.go`)
- Runway projection per item from the last 30 days of consumption (`internal/services/resources/service.go`, `GetResourceRunway`)
- Household and vault-wide daily requirements computed as ration-class target × member count (`CalculateHouseholdAllocation`, `GetVaultDailyRequirements`); nothing enforces the allocation against consumption
- Inventory audit that records the counted difference as an `AUDIT_CORRECTION` transaction (`PerformInventoryAudit`)
- Inventory view (F4): paginated stock list, category filter (`c`), detail view (`internal/tui/views/resources/inventory.go`)

### 🏭 Facility Operations (implemented)

- 15 core systems (reactor, generators, distribution, water, HVAC, waste, security, medical, hydroponics, comms, structural) are created on first engine start if the table is empty (`internal/simulation/systems_seed.go`, `EnsureCoreSystems`)
- Systems have a category, status, efficiency, capacity/output, maintenance interval and next-due date (`internal/models/facility.go`)
- Daily efficiency decay, DEGRADED below 80 % and FAILED below 15 %, and a probabilistic maintenance sweep that restores efficiency and writes `maintenance_records` (`internal/simulation/degradation.go`)
- Systems view (F5): paginated list with category filter (`c`) and a detail view (`internal/tui/views/facilities/systems.go`)
- `ScheduleMaintenance` and `CompleteMaintenance` exist in `internal/services/facilities/service.go` but have no caller in the TUI, CLI or engine

### ⏱️ Simulation Control Core (implemented)

- A vault clock runs at a configurable ratio to real time, 60× by default (`internal/util/time.go`, `internal/config/config.go`)
- Once per simulated day the engine runs, in order: consume resources → produce → expire stock → degrade systems → maintenance sweep → roll incidents → demographics (`internal/simulation/tick.go`, `processDay`)
  > ⚠ See [#33](https://github.com/jstephenperry/vt-uos/issues/33).
- Weighted daily incidents: equipment fault, contamination, illness, discovery, security (`internal/simulation/incidents.go`)
- Coded alerts (power deficit, system failures, resource runway, population viability, overcrowding, outbreak) recomputed from the vault state (`internal/simulation/alerts.go`)
- Events are appended to `simulation_events` and `audit_log` (`internal/simulation/events.go`)
- Vault time and counters checkpoint to `vault_metadata`; snapshots are SQLite `VACUUM INTO` backups; `-restore <file>` installs a snapshot before start (`internal/simulation/persistence.go`, `internal/simulation/snapshot.go`, `cmd/vtuos/main.go`)
- Simulation Control view (F11): pause/resume, time-scale ladder, step an hour or a day, snapshot, acknowledge alerts (`internal/tui/app.go`, `handleSimulationKeys`)

### 🌐 Master Server & Client Terminals (implemented)

- `vtuos serve` exposes a stdlib `net/http` JSON API, an SSE state stream and an embedded web console (`internal/server/server.go`, `internal/server/sse.go`, `internal/server/web.go`)
- Simulation-control and client-management routes require `server.admin_token` as `Authorization: Bearer <token>` or `X-Admin-Token`; with no token configured those routes are open and a warning is logged at startup (`internal/server/auth.go`, `internal/server/server.go`)
- In-memory client registry with heartbeats, telemetry and queued remote commands: SWITCH_VIEW, SET_KIOSK, MESSAGE, IDENTIFY, REFRESH, REBOOT, SHUTDOWN (`internal/server/clients.go`, `internal/protocol/protocol.go`)
- `vtuos connect <addr>` registers with retry, streams state over SSE, renders five pages on keys `1`–`5` and applies remote commands (`internal/client/client.go`); REFRESH only reports "refreshed" and REBOOT only shows a banner
- Exhibit options: boot sequence, attract-mode rotation and kiosk mode; kiosk blocks `q` and `F10` but `Ctrl+C` still opens the quit dialog (`internal/tui/app.go`)

### 🔮 Planned (Phase 6)

- **Labor**, **Medical**, **Security** and **Governance** have tables in `internal/database/migrations/001_initial.sql` but no models, repositories, services or data-driven views; F6–F9 render fixed sample text (`internal/tui/app.go`, `renderLabor`, `renderMedical`, `renderSecurity`, `renderGovernance`). See [docs/ROADMAP.md](docs/ROADMAP.md).
  > ⚠ See [#23](https://github.com/jstephenperry/vt-uos/issues/23).

## 🚀 Quick Start

### Prerequisites

- **Go 1.24+** (`go.mod`)
- **Linux/ARM64** (Raspberry Pi 4/5, Pi Zero 2W) or any Go-supported platform
- A terminal at least 100 columns wide to see the full function-key footer; narrower terminals get an abbreviated one (`internal/tui/keys.go`, `StatusBarHelpResponsive`)

### Installation

```bash
# Clone the repository
git clone https://github.com/jstephenperry/vt-uos.git
cd vt-uos

# Build the binary
make build

# Generate seed data (first time; exits when done)
./bin/vtuos -seed

# Run the local operator console
./bin/vtuos

# Or run as a master server (API + web console on :8080)
./bin/vtuos serve -token overseer-secret

# …and connect display terminals to it
./bin/vtuos connect 127.0.0.1:8080
```

The three invocation forms and their flags:

```text
$ ./bin/vtuos --help
Usage of vtuos:
  -config string
    	Path to configuration file
  -debug
    	Enable debug logging
  -migrate-only
    	Run migrations and exit
  -restore string
    	Restore the vault database from a snapshot file before starting
  -seed
    	Generate seed data
  -version
    	Show version and exit
$ ./bin/vtuos serve --help
Usage of serve:
  -config string
    	Path to configuration file
  -debug
    	Enable debug logging
  -listen string
    	Override the server listen address (e.g. :8080)
  -restore string
    	Restore the vault database from a snapshot file before starting
  -token string
    	Override the operator admin token
$ ./bin/vtuos connect --help
Usage of connect:
  -config string
    	Path to configuration file
  -debug
    	Enable debug logging
  -kind string
    	Terminal kind: display or operator
  -name string
    	Terminal display name reported to the master
```

See **[docs/CLI.md](docs/CLI.md)** for flag semantics and exit codes, **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)** for the master/client topology and web console, and **[docs/SIMULATION.md](docs/SIMULATION.md)** for the simulation control core.

### First Launch

On first run (`./bin/vtuos` or `./bin/vtuos -seed`), VT-UOS will:

1. Write a default configuration to `$XDG_CONFIG_HOME/vtuos/vault.toml`, normally `~/.config/vtuos/vault.toml` (`internal/config/loader.go`, `Load`)
2. Create the database at `$XDG_DATA_HOME/vtuos/vault.db`, normally `~/.local/share/vtuos/vault.db`, with a `backups/` directory beside it (`internal/config/loader.go`, `EnsureDataDir`, `BackupDir`)
3. Write JSON logs to `logs/vtuos.log` relative to the working directory (`internal/config/config.go`, `internal/config/loader.go`, `EnsureLogDir`)
4. Apply the two embedded migrations (`cmd/vtuos/main.go`, `prepare`)
5. With `-seed`: generate residents up to `vault.designed_capacity` (500 by default) in 100 family households plus single households, 8 resource categories with 28 items and one stock lot per item, then exit; seeding is skipped if any resident already exists (`cmd/vtuos/main.go`, `runSeed`; `internal/database/seed/generator.go`, `internal/database/seed/names.go`)
6. Without `-seed`: start the simulation engine, create the 15 core facility systems if none exist, and open the TUI (`cmd/vtuos/main.go`, `runLocal`; `internal/simulation/systems_seed.go`)

### Navigation

| Key | Screen |
|-----|--------|
| `F1` | Help |
| `F2` | Dashboard |
| `F3` | Population Registry |
| `F4` | Resource Management |
| `F5` | Facility Operations |
| `F6`–`F9` | Labor / Medical / Security / Governance (static placeholders) |
| `F11` | Simulation Control Core |
| `F10` | Quit (with confirmation) |

Function keys are mapped in `internal/tui/keys.go` (`GetFunctionKeyModule`). `q` and `Ctrl+C` also open the quit confirmation (`internal/tui/app.go`).

In the **Simulation Control** view: `Space` or `p` play/pause, `+`/`-` time scale (`=`/`_` are aliases), `s` step a day, `h` step an hour, `k` snapshot, `a` acknowledge alerts (`internal/tui/app.go`, `handleSimulationKeys`).

## 🏗️ Architecture

VT-UOS is a layered Go application. In `serve` mode the same engine is fronted by an HTTP server; `connect` mode is a thin terminal that renders the published state.

```
┌─────────────────────────────────────┐   ┌─────────────────────────┐
│  Terminal UI (Bubble Tea)           │   │  HTTP API / SSE / web   │  ← serve mode only
├─────────────────────────────────────┤   ├─────────────────────────┤
│  Simulation control core            │◄──┤  protocol.VaultState    │──► connect clients
├─────────────────────────────────────┤   └─────────────────────────┘
│  Services (population/resources/    │
│            facilities)              │
├─────────────────────────────────────┤
│  Repository layer                   │
├─────────────────────────────────────┤
│  SQLite (modernc.org/sqlite, WAL)   │  ← Pure Go (no CGO)
└─────────────────────────────────────┘
```

The engine and the TUI also issue some SQL directly against the database rather than through repositories; see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for where.

### Tech Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Language** | Go 1.24+ | Single binary, cross-compilation, low memory |
| **Database** | SQLite ([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)) | Pure Go, embedded, no CGO dependencies |
| **TUI** | Bubble Tea + Lip Gloss | Composable terminal UI framework |
| **HTTP** | `net/http`, `html/template` (stdlib) | No web framework |
| **Config** | TOML (BurntSushi/toml) | Human-readable configuration |
| **Logging** | log/slog | Structured JSON logging (stdlib) |

### Design Principles

- ✅ **No CGO**: `ldd bin/vtuos` reports "not a dynamic executable"
- ✅ **Layered**: models → repository → services → TUI, with the exceptions listed in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- ✅ **Embedded migrations**: SQL files compiled into the binary and applied on start, one transaction per migration file (`internal/database/migrate.go`)
- ⚠️ **Coverage**: 41.8% of statements total (`go test -race -coverprofile`); the Makefile's `test-coverage-check` target expects 80%
  > ⚠ See [#7](https://github.com/jstephenperry/vt-uos/issues/7).
- ⚠️ **Transactions**: the resources service performs its stock-plus-ledger writes as separate statements with no enclosing transaction (`internal/services/resources/service.go`)
  > ⚠ See [#9](https://github.com/jstephenperry/vt-uos/issues/9).

## 📚 Documentation

Documentation lives in the [`docs/`](docs/) directory:

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Layers, process model and data flow as wired in `cmd/vtuos/main.go`
- **[CLI.md](docs/CLI.md)** - `vtuos`, `vtuos serve`, `vtuos connect`: flags, exit codes, startup sequence
- **[CONFIGURATION.md](docs/CONFIGURATION.md)** - Every TOML key, defaults, validation, file search order
- **[DATABASE.md](docs/DATABASE.md)** - Schema from the migrations, PRAGMAs, migration and recovery mechanics
- **[MODULES.md](docs/MODULES.md)** - Population, Resources and Facilities services: methods, algorithms, repositories
- **[SIMULATION.md](docs/SIMULATION.md)** - Tick pipeline, time model, incidents, alerts, persistence, control API
- **[API.md](docs/API.md)** - HTTP routes, authentication, SSE stream, client protocol
- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** - Local / master-server / client modes, tokens, kiosk, reset procedure
- **[TUI.md](docs/TUI.md)** - Views, key bindings, breakpoints, components, style tokens
- **[DEVELOPMENT.md](docs/DEVELOPMENT.md)** - Branching, commit style, Makefile targets, coding guidelines
- **[TESTING.md](docs/TESTING.md)** - What is tested, how to run it, per-package coverage, known gaps
- **[ROADMAP.md](docs/ROADMAP.md)** - What is built, and the Phase 6–7 plan (nothing in it is implemented)

## 🗺️ Roadmap

- [x] **Phase 1**: Foundation (config, database, TUI shell)
- [x] **Phase 2**: Population module (residents, households, census, forms)
- [x] **Phase 3**: Resource management (inventory, consumption, ledger, runway)
- [x] **Phase 4**: Facility operations (systems, degradation, maintenance sweep)
- [x] **Phase 5**: Simulation control core + master/client deployment + web console
- [ ] **Phase 6**: Labor, Medical, Security, Governance
- [ ] **Phase 7**: Polish (dashboard, alerts, optimization)

See [docs/ROADMAP.md](docs/ROADMAP.md) for the as-built status of each module and the Phase 6–7 plan.

## 🛠️ Development

### Build Commands

```bash
# Build for current platform (CGO_ENABLED=0)
make build

# Cross-compile for Raspberry Pi 4/5 (linux/arm64)
make build-pi

# Cross-compile for Pi Zero 2W (linux/arm64, -tags sqlite_omit_load_extension)
make build-pi-zero

# Run unit tests with -race and per-package coverage
make test

# Run golangci-lint (fails with an install hint if golangci-lint is not on PATH)
make lint

# List every target
make help
```

`make lint` and `make fmt` need `golangci-lint` and `goimports`; neither is required to build (`Makefile`).

### Project Structure

```
vt-uos/
├── cmd/vtuos/                  # Entry point: local console, serve, connect
├── internal/
│   ├── client/                 # Client terminal (connect mode)
│   ├── config/                 # TOML configuration loading
│   ├── database/               # SQLite connection, migrations, recovery
│   │   ├── migrations/         # Embedded SQL migrations
│   │   └── seed/               # Seed data generator
│   ├── models/                 # Domain models
│   ├── protocol/               # Wire types shared by server, web console, clients
│   ├── repository/             # Data access layer
│   ├── server/                 # Master server: HTTP API, SSE, web console
│   │   └── web/static/         # Embedded web console assets
│   ├── services/               # Business logic
│   │   ├── facilities/
│   │   ├── population/
│   │   └── resources/
│   ├── simulation/             # Simulation control core
│   ├── testutil/               # Test fixtures and helpers
│   ├── tui/                    # Terminal UI shell
│   │   ├── components/         # Table, Input, Select, Form
│   │   └── views/              # facilities, population, resources, simulation
│   └── util/                   # ID generation, vault clock, date helpers
├── docs/                       # Documentation
├── testdata/                   # Sample TOML configs
└── Makefile
```

## 🤝 Contributing

Before submitting a PR, please:

1. Read [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for coding standards
2. Check [docs/ROADMAP.md](docs/ROADMAP.md) to see what is planned
3. Open an issue to discuss major changes
4. Ensure tests pass (`make test`)

### Development Workflow

1. Fork and clone the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the architecture patterns
4. Write tests alongside the change (see [docs/TESTING.md](docs/TESTING.md))
5. Run `make test` to verify
6. Commit with clear messages
7. Push and open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by the **Fallout** universe created by Interplay Entertainment and Bethesda Game Studios
- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework by Charm
- Uses [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) for pure-Go SQLite

## 🐛 Support

- **Issues**: [GitHub Issues](https://github.com/jstephenperry/vt-uos/issues)
- **Documentation**: [docs/](docs/)

---

<div align="center">

**Built with ❤️ for the Fallout community**

*"War. War never changes. But vault management systems? Those need work."*

</div>
