# VT-UOS: Vault-Tec Unified Operating System

## Project Vision

VT-UOS is a realistic vault population management and operations system inspired by the Fallout universe. Unlike terminal emulators that replicate the in-game aesthetic, this project builds the **actual operational software** that Vault-Tec would have deployed to manage sealed underground communities for multi-generational survival.

**Core Capabilities:**

- Manage 500-1000 residents across 25+ year timespans
- Track resources in a closed-loop life support environment
- Schedule labor in a command economy
- Monitor facility systems critical to survival
- Simulate time progression with realistic dynamics

## Quick Reference

- **[Architecture](docs/ARCHITECTURE.md)** - Layers, process model, data flow as wired
- **[CLI](docs/CLI.md)** - `vtuos`, `vtuos serve`, `vtuos connect`: flags, exit codes, startup
- **[Configuration](docs/CONFIGURATION.md)** - Every TOML key, defaults, validation, search order
- **[Database Schema](docs/DATABASE.md)** - Schema from the migrations, PRAGMAs, recovery
- **[Module Specifications](docs/MODULES.md)** - Population, Resources, Facilities services
- **[Simulation](docs/SIMULATION.md)** - Tick pipeline, incidents, alerts, persistence, control API
- **[API](docs/API.md)** - HTTP routes, auth, SSE stream, client protocol
- **[Deployment](docs/DEPLOYMENT.md)** - Local / master / client modes, tokens, kiosk
- **[TUI Design](docs/TUI.md)** - Views, key bindings, breakpoints, components
- **[Development Guide](docs/DEVELOPMENT.md)** - Branching, Makefile targets, guidelines
- **[Testing](docs/TESTING.md)** - What is tested, how to run it, coverage, gaps
- **[Roadmap](docs/ROADMAP.md)** - As-built status and the Phase 6–7 plan (nothing in it is implemented)

## Technical Stack

| Component | Choice | Rationale |
| --------- | ------ | --------- |
| Language | Go 1.24+ | Single binary, low memory, cross-compilation |
| Database | SQLite (modernc.org/sqlite) | Pure Go, embedded, no CGO |
| TUI | Bubble Tea + Lip Gloss | Modern, composable, terminal UI |
| Config | TOML | Human-readable |
| Logging | log/slog | Structured, stdlib |

## Technical Constraints

**Target Platform:**

- Primary: Raspberry Pi 4/5 (ARM64, 1-8GB RAM)
- Secondary: Pi Zero 2W (ARM64, 512MB RAM)
- Development: Any Go 1.24+ system (`go.mod`)

**Non-Negotiable:**

1. Single static binary (no runtime dependencies)
2. Embedded database (SQLite only)
3. Run in 512MB RAM
4. Offline-first
5. Power-loss resilient
6. Deterministic builds

> ⚠ See [#15](https://github.com/jstephenperry/vt-uos/issues/15) and [#36](https://github.com/jstephenperry/vt-uos/issues/36).

**Explicitly Avoided:**

- CGO, external databases, web frameworks, heavy ORMs, global state

## Project Structure

```plaintext
vtuos/
├── cmd/vtuos/main.go           # Entry point: local console, serve, connect
├── internal/
│   ├── client/                 # Client terminal (connect mode)
│   ├── config/                 # Configuration loading
│   ├── database/               # SQLite connection, migrations, recovery
│   │   ├── migrations/         # Embedded SQL migrations (001, 002)
│   │   └── seed/               # Seed data generator
│   ├── models/                 # Domain models
│   ├── protocol/               # Wire types shared by server, web console, clients
│   ├── repository/             # Data access layer
│   ├── server/                 # Master server: HTTP API, SSE, web console
│   │   └── web/                # Embedded console template
│   │       └── static/         # Embedded console JS/CSS
│   ├── services/               # Business logic
│   │   ├── facilities/         # System monitoring, maintenance records
│   │   ├── population/         # Census, lineage, demographics
│   │   └── resources/          # Inventory, rationing, forecasting
│   ├── simulation/             # Simulation control core
│   ├── testutil/               # Test DB helper and fixtures
│   ├── tui/                    # Terminal UI shell
│   │   ├── components/         # Table, Input, Select, Form
│   │   └── views/              # Screen implementations
│   │       ├── facilities/
│   │       ├── population/
│   │       ├── resources/
│   │       └── simulation/
│   └── util/                   # ID generation, vault clock, date helpers
├── docs/                       # Detailed documentation
│   └── superpowers/            # Design specs and plans
│       ├── plans/
│       └── specs/
├── testdata/                   # Sample TOML configs
├── .vscode/                    # Editor settings
└── Makefile
```

## Core Domain Concepts

**Residents** - Vault dwellers with registry numbers, biological-parent links, clearance levels and status (`internal/models/resident.go`)  
**Households** - Family units with a ration class that sets daily calorie and water targets (`internal/models/household.go`)  
**Resources** - Categories, items and stock lots tracked through a transaction ledger with expiration (`internal/models/resource.go`)  
**Facilities** - Infrastructure systems with efficiency, status and maintenance intervals (`internal/models/facility.go`)  
**Vocations / Quarters** - Tables exist and are seeded; `Quarters` is modeled but neither has a repository or service (`internal/database/migrations/001_initial.sql`, `internal/database/seed/generator.go`)  
**Simulation** - Daily pipeline: consumption, production, expiration, degradation, maintenance, incidents, births/deaths (`internal/simulation/tick.go`)

**→ Full database schema:** [docs/DATABASE.md](docs/DATABASE.md)

## System Modules

Implemented (service + repository + TUI view):

1. **Population** - Census, resident forms, birth/death registration, demographics; inbreeding coefficient computed but not surfaced (`internal/services/population`)
2. **Resources** - Inventory, consumption ledger, expiration, runway forecasting, allocation calculation (`internal/services/resources`)
3. **Facilities** - Systems list/detail, stats, maintenance records (`internal/services/facilities`)
4. **Simulation** - Time engine, daily pipeline, incidents, alerts, persistence (`internal/simulation`)

Planned (tables only in `internal/database/migrations/001_initial.sql`; F6–F9 render static text in `internal/tui/app.go`):

5. **Labor** - Work assignments, shift scheduling, vacancy tracking
6. **Medical** - Health records, radiation tracking, epidemiology
7. **Security** - Access control, incident management
8. **Governance** - Directives, policy enforcement, classification

**→ Implemented services:** [docs/MODULES.md](docs/MODULES.md) · **→ Planned:** [docs/ROADMAP.md](docs/ROADMAP.md)

## User Interface

**Terminal UI** with Bubble Tea framework:

- Green phosphor, amber or white theme (`internal/tui/styles.go`)
- Keyboard-only navigation: F-keys switch modules, vi-style `j`/`k` in lists (`internal/tui/keys.go`, `internal/tui/app.go`)
- Flat module switching (F1–F11) with list → detail drill-down; no hierarchical menus
- Reusable components: `Table`, `Input`, `Select`, `Form` (`internal/tui/components`); the quit confirmation is drawn inline in `app.go`, and there is no modal, tabs or tree component
- Responsive layout at 60 and 100 columns (`internal/tui/layout.go`)

**→ Full TUI design guide:** [docs/TUI.md](docs/TUI.md)

## Quick Start

```bash
# Clone and build
git clone https://github.com/jstephenperry/vt-uos.git
cd vt-uos
make build

# Seed, then run the local console
./bin/vtuos -seed
./bin/vtuos

# Master server + client terminal
./bin/vtuos serve -token overseer-secret
./bin/vtuos connect 127.0.0.1:8080

# Cross-compile for Raspberry Pi
make build-pi
```

**→ Flags and exit codes:** [docs/CLI.md](docs/CLI.md) · **→ Config keys:** [docs/CONFIGURATION.md](docs/CONFIGURATION.md)

## Development

**Architecture:** Layered, with constructor injection from `cmd/vtuos/main.go`

- **Models** - Domain entities
- **Repository** - Data access (SQLite)
- **Services** - Business logic
- **Simulation** - Engine over the services (also issues some direct SQL)
- **TUI / Server / Client** - Presentation layers

**Testing:** `go test -race` unit and teatest end-to-end tests; 41.8% total statement coverage; no test files in `config`, `database`, `seed`, `util`, `protocol`, `client`, `testutil` or any `services/*` package  
**Workflow:** Feature branches → PR → merge to `main` (single branch; no CI config in the repo)

> ⚠ See [#7](https://github.com/jstephenperry/vt-uos/issues/7).

**→ Developer guide:** [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) · **→ Testing:** [docs/TESTING.md](docs/TESTING.md)

## Implementation Phases

- [x] **Phase 1:** Foundation (config, database, TUI shell)
- [x] **Phase 2:** Population module MVP (residents, households, CRUD)
- [x] **Phase 3:** Resource management (inventory, consumption, rationing)
- [x] **Phase 4:** Facility operations (systems, maintenance)
- [x] **Phase 5:** Simulation control core (time, consumption, degradation, incidents, demographics) + master/client deployment + web console
- [ ] **Phase 6:** Remaining modules (labor, medical, security, governance)
- [ ] **Phase 7:** Polish (dashboard, alerts, optimization)

See [docs/ROADMAP.md](docs/ROADMAP.md) for per-module status and the Phase 6–7 plan, and [docs/SIMULATION.md](docs/SIMULATION.md) / [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for the control core and client/master-server deployment.

## Key Design Decisions

**Why SQLite?** Pure Go (no CGO), embedded, sub-512MB RAM, power-loss resilient  
**Why TUI first?** Authentic aesthetic, low resource overhead, offline-first; the web console exists only in `serve` mode  
**Why Go?** Single binary, cross-compilation, excellent concurrency, low memory  
**Why No CGO?** Simplifies cross-compilation for ARM targets (Pi)

> ⚠ See [#9](https://github.com/jstephenperry/vt-uos/issues/9), [#21](https://github.com/jstephenperry/vt-uos/issues/21), [#22](https://github.com/jstephenperry/vt-uos/issues/22) and [#45](https://github.com/jstephenperry/vt-uos/issues/45).

## AI Assistant Guidelines

When working on this project:

1. **Start with data model** - Check [DATABASE.md](docs/DATABASE.md) for schema
2. **Repository layer first** - Pure data access, no business logic
3. **Service layer with tests** - Business logic with 80%+ coverage
4. **TUI last** - Views call services, never repositories directly
5. **Keep functions small** - <50 lines preferred
6. **Wrap all errors** - `fmt.Errorf("context: %w", err)`
7. **Use transactions** - Multi-step DB ops must be atomic
8. **Log structurally** - `slog` with key-value pairs
9. **Follow patterns** - Look at existing code for conventions
10. **Ask when unsure** - Don't assume ambiguous requirements

## Reference Files

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - Layers, process model, data flow, layering exceptions
- [CLI.md](docs/CLI.md) - Invocation forms, flags, exit codes, startup sequence, signals
- [CONFIGURATION.md](docs/CONFIGURATION.md) - TOML keys, defaults, validation, file search order
- [DATABASE.md](docs/DATABASE.md) - Schema from migrations, indexes, PRAGMAs, migration and recovery mechanics
- [MODULES.md](docs/MODULES.md) - Population, Resources, Facilities: methods, algorithms, repositories
- [SIMULATION.md](docs/SIMULATION.md) - Tick pipeline, time model, incidents, alerts, persistence, control API
- [API.md](docs/API.md) - HTTP routes, authentication, SSE, client protocol
- [DEPLOYMENT.md](docs/DEPLOYMENT.md) - Modes, tokens, kiosk, reset procedure
- [TUI.md](docs/TUI.md) - Views, key bindings, breakpoints, components, style tokens
- [DEVELOPMENT.md](docs/DEVELOPMENT.md) - Branching, commit style, Makefile targets, guidelines
- [TESTING.md](docs/TESTING.md) - Test inventory, commands, coverage, gaps
- [ROADMAP.md](docs/ROADMAP.md) - As-built status; Phase 6–7 plan (nothing in it is implemented)
