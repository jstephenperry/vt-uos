# VT-UOS: Vault-Tec Unified Operating System

VT-UOS is a realistic vault population management and operations system designed for multi-generational underground survival. It ships as a single static Go binary with an offline-first terminal UI in the classic Vault-Tec aesthetic.

## Architecture at a Glance

- **Language/Runtime:** Go 1.22+, pure Go (no CGO) with SQLite via `modernc.org/sqlite` for an embedded, power-loss-resilient database.
- **Layered layout:** `cmd/` (entrypoint) → `internal/` packages for configuration, database, models, repositories, services, simulation, and TUI presentation. Services encapsulate business rules; TUI layers never talk directly to repositories.
- **Modules:** Population, Resources, Facilities, Medical, Security, Governance, Labor, and Simulation. Each module has models, repository accessors, and service logic, with Bubble Tea/Lip Gloss TUI views under `internal/tui/views/`.
- **Configuration & logging:** TOML config (`vault.toml`) loaded from XDG config (`~/.config/vtuos/`) or the working directory; defaults are generated if missing. Structured logging via `log/slog` with optional file output.
- **Migrations & backups:** SQLite migrations live in `migrations/`; database recovery and backup helpers are built into the runtime.

See `docs/` for deep dives: `DATABASE.md` (schema), `MODULES.md` (service specs), `TUI.md` (interface), and `CONFIGURATION.md` (all config knobs).

## Usage

### Prerequisites
- Go 1.22+

### Build & Run
```bash
make build              # builds ./bin/vtuos (static)
./bin/vtuos --config vault.toml   # start with an explicit config
```

If no config is provided, VT-UOS searches `~/.config/vtuos/vault.toml` then `./vault.toml`; a default file is created when absent. Key flags:

- `--migrate-only` — run migrations and exit
- `--seed` — generate seed data after migrations
- `--debug` — enable debug logging
- `--version` — print build metadata

### Common Tasks
- `make test` / `make test-integration` — run unit or integration suites
- `make lint` — run `golangci-lint`
- `make build-pi` / `make build-pi-zero` — cross-compile static ARM64 binaries for Raspberry Pi targets

For configuration fields and module behaviors, refer to `docs/CONFIGURATION.md` and `docs/MODULES.md`.
