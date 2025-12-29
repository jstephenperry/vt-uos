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

- **[Database Schema](docs/DATABASE.md)** - Complete SQLite schema definitions
- **[Module Specifications](docs/MODULES.md)** - Service APIs and business logic
- **[TUI Design](docs/TUI.md)** - Interface design, components, navigation
- **[Configuration](docs/CONFIGURATION.md)** - Setup, config files, environment
- **[Development Guide](docs/DEVELOPMENT.md)** - Workflow, testing, implementation

## Technical Stack

| Component | Choice | Rationale |
| --------- | ------ | --------- |
| Language | Go 1.22+ | Single binary, low memory, cross-compilation |
| Database | SQLite (modernc.org/sqlite) | Pure Go, embedded, no CGO |
| TUI | Bubble Tea + Lip Gloss | Modern, composable, terminal UI |
| Config | TOML | Human-readable |
| Logging | log/slog | Structured, stdlib |

## Technical Constraints

**Target Platform:**

- Primary: Raspberry Pi 4/5 (ARM64, 1-8GB RAM)
- Secondary: Pi Zero 2W (ARM64, 512MB RAM)
- Development: Any Go 1.22+ system

**Non-Negotiable:**

1. Single static binary (no runtime dependencies)
2. Embedded database (SQLite only)
3. Run in 512MB RAM
4. Offline-first
5. Power-loss resilient
6. Deterministic builds

**Explicitly Avoided:**

- CGO, external databases, web frameworks, heavy ORMs, global state

## Project Structure

```plaintext
vtuos/
├── cmd/vtuos/main.go           # Entry point
├── internal/
│   ├── config/                 # Configuration loading
│   ├── database/               # SQLite connection & migrations
│   ├── models/                 # Domain models
│   ├── repository/             # Data access layer
│   ├── services/               # Business logic
│   │   ├── population/         # Census, lineage, demographics
│   │   ├── resources/          # Inventory, rationing, forecasting
│   │   ├── labor/              # Work assignments, scheduling
│   │   ├── facilities/         # System monitoring, maintenance
│   │   ├── medical/            # Health records, epidemiology
│   │   ├── security/           # Access control, incidents
│   │   └── governance/         # Directives, audit log
│   ├── simulation/             # Time progression, events
│   ├── tui/                    # Terminal UI
│   │   ├── components/         # Reusable UI components
│   │   └── views/              # Screen implementations
│   └── util/                   # Helpers
├── docs/                       # Detailed documentation
│   ├── DATABASE.md             # Complete schema
│   ├── MODULES.md              # Service specifications
│   ├── TUI.md                  # UI design
│   ├── CONFIGURATION.md        # Setup & config
│   └── DEVELOPMENT.md          # Dev workflow
├── migrations/                 # SQL migrations
├── testdata/                   # Test fixtures
└── Makefile
```

## Core Domain Concepts

**Residents** - Vault dwellers with lineage tracking, clearance levels, and life history  
**Households** - Family units with ration classes and quarters assignments  
**Resources** - Consumables tracked via stocks, transactions, and expiration  
**Facilities** - Infrastructure systems with efficiency, maintenance schedules  
**Vocations** - Jobs with shift patterns, skill requirements, headcount limits  
**Simulation** - Time progression engine with consumption, aging, random events

**→ Full database schema:** [docs/DATABASE.md](docs/DATABASE.md)

## System Modules

1. **Population** - Census, vital records, lineage, inbreeding detection, demographics
2. **Resources** - Inventory, rationing, expiration tracking, runway forecasting
3. **Labor** - Work assignments, shift scheduling, vacancy tracking
4. **Facilities** - System monitoring, maintenance scheduling, failure prediction
5. **Medical** - Health records, radiation tracking, epidemiology
6. **Security** - Access control, incident management, audit logging
7. **Governance** - Directives, policy enforcement, classification
8. **Simulation** - Time engine, consumption processing, event generation

**→ Complete API specifications:** [docs/MODULES.md](docs/MODULES.md)

## User Interface

**Terminal UI** with Bubble Tea framework:

- Green phosphor aesthetic (Fallout-style)
- Keyboard-only navigation (F-keys, vi-style)
- Hierarchical menus (Dashboard → Modules → Views)
- Reusable components (tables, forms, modals, trees)
- Dense information display maximizing screen real estate

**→ Full TUI design guide:** [docs/TUI.md](docs/TUI.md)

## Quick Start

```bash
# Clone and build
git clone https://github.com/yourusername/vt-uos.git
cd vt-uos
make build

# Run
./bin/vtuos

# Cross-compile for Raspberry Pi
make build-linux-arm64
```

**→ Complete setup guide:** [docs/CONFIGURATION.md](docs/CONFIGURATION.md)

## Development

**Architecture:** Clean layered architecture with dependency injection

- **Models** - Domain entities
- **Repository** - Data access (SQLite)
- **Services** - Business logic
- **TUI** - Presentation layer

**Testing:** Unit tests (80% coverage), integration tests, property-based tests  
**Workflow:** Feature branches → PR → CI → Merge to develop → Release from main

**→ Developer guide:** [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)

## Implementation Phases

- [x] **Phase 1:** Foundation (config, database, TUI shell)
- [ ] **Phase 2:** Population module MVP (residents, households, CRUD)
- [ ] **Phase 3:** Resource management (inventory, consumption, rationing)
- [ ] **Phase 4:** Facility operations (systems, maintenance)
- [ ] **Phase 5:** Simulation engine (time, events, degradation)
- [ ] **Phase 6:** Remaining modules (labor, medical, security, governance)
- [ ] **Phase 7:** Polish (dashboard, alerts, optimization)

## Key Design Decisions

**Why SQLite?** Pure Go (no CGO), embedded, sub-512MB RAM, power-loss resilient  
**Why TUI not Web?** Authentic aesthetic, low resource overhead, offline-first  
**Why Go?** Single binary, cross-compilation, excellent concurrency, low memory  
**Why No CGO?** Simplifies cross-compilation for ARM targets (Pi)

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

- [DATABASE.md](docs/DATABASE.md) - Complete SQLite schema with all tables, indexes, business rules
- [MODULES.md](docs/MODULES.md) - Service specifications, algorithms, API interfaces
- [TUI.md](docs/TUI.md) - UI design, components, navigation, key bindings, examples
- [CONFIGURATION.md](docs/CONFIGURATION.md) - Setup, config files, environment variables
- [DEVELOPMENT.md](docs/DEVELOPMENT.md) - Workflow, testing, build system, guidelines
