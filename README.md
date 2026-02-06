<div align="center">

# 🏛️ VT-UOS: Vault-Tec Unified Operating System

**The realistic vault management system that Vault-Tec should have built**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/jstephenperry/vt-uos)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20arm64-lightgrey)](https://github.com/jstephenperry/vt-uos)

[Features](#-features) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [Roadmap](#-roadmap) • [Contributing](#-contributing)

</div>

---

## 📖 Overview

VT-UOS is a **realistic vault population management and operations system** inspired by the Fallout universe. Unlike terminal emulators that replicate the in-game aesthetic, VT-UOS is the **actual operational software** that Vault-Tec would have deployed to manage sealed underground communities for multi-generational survival.

This isn't a game—it's a serious simulation of managing 500-1000 residents across 25+ year timespans in a closed-loop life support environment where every decision matters.

### 🎯 Why VT-UOS?

- **Authentic Operations**: Manage real population dynamics, resource consumption, facility maintenance, and labor scheduling
- **Realistic Constraints**: Designed for resource-constrained hardware (Raspberry Pi, 512MB RAM minimum)
- **Offline-First**: No network dependencies—survives in a sealed vault
- **Power-Loss Resilient**: Embedded SQLite with proper transaction handling
- **Single Static Binary**: No runtime dependencies, no installation headaches
- **Terminal UI**: Green phosphor aesthetic with keyboard-only navigation

## ✨ Features

### 🧑‍🤝‍🧑 Population Management
- Track 500-1000 residents with full genealogy and lineage
- Household management with ration classes
- Vital statistics tracking (births, deaths, aging)
- Inbreeding detection across generations
- Census reports and demographic analysis

### 📦 Resource Management (Current Phase)
- 8 resource categories: Food, Water, Medical, Power, Parts, Clothing, Tools, Chemicals
- FIFO consumption tracking with expiration management
- Production and transaction logging
- Runway forecasting and low-stock alerts
- Household ration allocation
- Inventory auditing

### 🏭 Facility Operations (In Progress)
- Critical infrastructure monitoring (Power, Water Treatment, HVAC)
- Maintenance scheduling and failure prediction
- Efficiency tracking and degradation modeling
- Alert generation for critical systems

### 🔮 Coming Soon
- **Labor Management**: Work assignments, shift scheduling, skill tracking
- **Medical System**: Health records, radiation tracking, epidemiology
- **Security**: Access control, incident management, audit logging
- **Simulation Engine**: Time progression, random events, cascading failures

## 🚀 Quick Start

### Prerequisites

- **Go 1.22+** (for building from source)
- **512MB RAM minimum** (1GB+ recommended)
- **Linux/ARM64** (Raspberry Pi 4/5, Pi Zero 2W) or any Go-supported platform

### Installation

```bash
# Clone the repository
git clone https://github.com/jstephenperry/vt-uos.git
cd vt-uos

# Build the binary
make build

# Run with seed data (first time)
./bin/vtuos --seed

# Run normally
./bin/vtuos
```

### First Launch

On first run, VT-UOS will:
1. Create configuration at `~/.config/vtuos/vault.toml`
2. Initialize database at `~/.local/share/vtuos/vault.db`
3. Run migrations to create schema
4. Generate 500 residents, 200 households, and resource inventory (if `--seed` is used)

### Navigation

| Key | Screen |
|-----|--------|
| `F1` | Help |
| `F2` | Dashboard |
| `F3` | Population Registry |
| `F4` | Resource Management |
| `F5` | Facility Operations (coming soon) |
| `F10` | Quit |

## 🏗️ Architecture

VT-UOS follows a clean layered architecture optimized for embedded systems:

```
┌─────────────────────────────────────┐
│         Terminal UI (Bubble Tea)    │  ← Presentation Layer
├─────────────────────────────────────┤
│          Service Layer              │  ← Business Logic
├─────────────────────────────────────┤
│        Repository Layer             │  ← Data Access
├─────────────────────────────────────┤
│         SQLite Database             │  ← Pure Go (no CGO)
└─────────────────────────────────────┘
```

### Tech Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Language** | Go 1.22+ | Single binary, cross-compilation, low memory |
| **Database** | SQLite ([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)) | Pure Go, embedded, no CGO dependencies |
| **TUI** | Bubble Tea + Lip Gloss | Modern, composable terminal UI framework |
| **Config** | TOML | Human-readable configuration |
| **Logging** | log/slog | Structured logging (stdlib) |

### Design Principles

- ✅ **No CGO**: Pure Go for easy cross-compilation to ARM64
- ✅ **No Global State**: Dependency injection throughout
- ✅ **Transactional**: All multi-step operations are atomic
- ✅ **Tested**: 80%+ code coverage target
- ✅ **Layered**: Clear separation between data/logic/presentation

## 📚 Documentation

Comprehensive documentation is available in the [`docs/`](docs/) directory:

- **[DATABASE.md](docs/DATABASE.md)** - Complete SQLite schema with all tables, indexes, and business rules
- **[MODULES.md](docs/MODULES.md)** - Service specifications, algorithms, and API interfaces
- **[TUI.md](docs/TUI.md)** - UI design, components, navigation, and key bindings
- **[CONFIGURATION.md](docs/CONFIGURATION.md)** - Setup, config files, and environment variables
- **[DEVELOPMENT.md](docs/DEVELOPMENT.md)** - Workflow, testing, build system, and guidelines

## 🗺️ Roadmap

### Current Status: **Phase 3 Complete** 🎉

- [x] **Phase 1**: Foundation (config, database, TUI shell)
- [x] **Phase 2**: Population module MVP (residents, households, CRUD)
- [x] **Phase 3**: Resource management (inventory, consumption, rationing)
- [ ] **Phase 4**: Facility operations (systems, maintenance) ← **In Progress**
- [ ] **Phase 5**: Simulation engine (time, events, degradation)
- [ ] **Phase 6**: Additional modules (labor, medical, security, governance)
- [ ] **Phase 7**: Polish (dashboard, alerts, optimization)

See [PROGRESS.md](PROGRESS.md) for detailed development tracking.

## 🛠️ Development

### Build Commands

```bash
# Build for current platform
make build

# Cross-compile for Raspberry Pi 4/5
make build-pi

# Cross-compile for Pi Zero 2W (minimal)
make build-pi-zero

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint

# Format code
make fmt
```

### Project Structure

```
vtuos/
├── cmd/vtuos/          # Entry point
├── internal/
│   ├── config/         # Configuration loading
│   ├── database/       # SQLite connection & migrations
│   ├── models/         # Domain models
│   ├── repository/     # Data access layer
│   ├── services/       # Business logic
│   ├── simulation/     # Time progression engine
│   ├── tui/            # Terminal UI components
│   └── util/           # Utilities
├── docs/               # Detailed documentation
├── migrations/         # SQL migrations
└── testdata/           # Test fixtures
```

## 🤝 Contributing

We welcome contributions! Before submitting a PR, please:

1. Read [DEVELOPMENT.md](docs/DEVELOPMENT.md) for coding standards
2. Check [PROGRESS.md](PROGRESS.md) to see what's in progress
3. Open an issue to discuss major changes
4. Ensure tests pass (`make test`)
5. Run the linter (`make lint`)

### Development Workflow

1. Fork and clone the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the architecture patterns
4. Write tests (80%+ coverage target)
5. Run `make test lint` to verify
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
- **Discussions**: [GitHub Discussions](https://github.com/jstephenperry/vt-uos/discussions)
- **Documentation**: [docs/](docs/)

---

<div align="center">

**Built with ❤️ for the Fallout community**

*"War. War never changes. But vault management systems? Those need work."*

</div>
