# Development Guide

Companion documents: [ARCHITECTURE.md](ARCHITECTURE.md) (how the binary is wired), [TESTING.md](TESTING.md) (what tests exist and how to run them), [ROADMAP.md](ROADMAP.md) (unbuilt work).

## Development Workflow

### Branch Strategy

- `main` - Stable, release-ready; the only branch on the remote (`git branch -r` lists `origin/main` alone)
- `feature/*` - Feature development
- `fix/*` - Bug fixes

There is no `develop` integration branch; feature and fix branches merge to `main` by pull request.

### Commit Messages

Follow Conventional Commits:

```plaintext
feat(population): add COI calculation for lineage tracking
fix(resources): correct expiration date comparison
docs(readme): update installation instructions
refactor(tui): extract table component
test(simulation): add property tests for consumption
```

### Code Style

- Run `gofmt` on all code. `make fmt` runs `go fmt ./...` followed by `goimports -w .`; the Makefile expects `goimports` on `PATH` but it is not required for `make build` or `make test`.
- `make lint` runs `golangci-lint run ./...` and exits 1 with an install hint if the binary is missing. The Makefile expects `golangci-lint`; it is not required to build. No `.golangci.yml` configuration file exists in the repository, so the linter runs with its defaults.
- No exported functions without documentation comments
- Errors must be wrapped with context: `fmt.Errorf("loading resident %s: %w", id, err)`

### Pull Request Checklist

- [ ] `make test` passes
- [ ] Documentation updated
- [ ] Migration added under `internal/database/migrations/` (if schema change)

There is no `CHANGELOG` file and no CI configuration (`.github/`) in the repository; the checklist above is what a reviewer can actually verify.

## Testing Requirements

Measured at commit `34c3398` with `go test -race -count=1 -coverprofile=coverage.out ./internal/...`:

- Total statement coverage: **41.8%** (`go tool cover -func`).
- Ten packages have no `_test.go` files at all: `internal/client`, `internal/config`, `internal/database`, `internal/database/seed`, `internal/protocol`, `internal/services/facilities`, `internal/services/population`, `internal/services/resources`, `internal/testutil`, `internal/util`.
- `make test-coverage-check` enforces an 80% total threshold and therefore fails on the current tree.

> ⚠ See [#7](https://github.com/jstephenperry/vt-uos/issues/7).

The per-package table, the test helpers, and the known gaps are in [TESTING.md](TESTING.md).

```bash
# Run all unit tests (what the Makefile runs)
make test

# Coverage report (coverage.html) plus the total line
make test-coverage

# One package
make test-pkg PKG=./internal/models
```

`make test-integration` passes `-tags=integration`, but no file in the repository carries that build tag, so it runs exactly the same tests as `make test`.

### Test Data

`testdata/` contains two files, both TOML configs: `vault-076.toml` and `vault-exhibit.toml`. There are no seed fixture directories. Test databases are created in memory by each test package (see [TESTING.md](TESTING.md)).

## Implementation Order

Phases 1–5 (foundation, population, resources, facilities, simulation core + master/client modes + web console) are implemented and described as built in [ARCHITECTURE.md](ARCHITECTURE.md), [MODULES.md](MODULES.md) and [SIMULATION.md](SIMULATION.md).
Phase 6 (labor, medical, security, governance) and Phase 7 (dashboard polish, alerts, reporting, optimisation) are not started; the F6–F9 modules render fixed placeholder text.
The phase plan, deliverables and week estimates that used to live here are collected in [ROADMAP.md](ROADMAP.md).
When adding a module, follow the order used by the existing ones: model → repository → service → TUI view → `internal/tui/app.go` wiring.
Schema changes go in a new `NNN_description.sql` file under `internal/database/migrations/` with `-- +migrate Up` / `-- +migrate Down` markers; the file is embedded at build time.
Nothing in this section is a commitment to dates.

> ⚠ See [#44](https://github.com/jstephenperry/vt-uos/issues/44).

## Architecture Guidelines

These are the conventions the existing code follows. See [ARCHITECTURE.md](ARCHITECTURE.md) for where they are currently broken.

### Dependency Injection

Services take a `*sql.DB` and construct their own repositories; nothing is global. This is the shape used by all three services (`internal/services/population/service.go`, `internal/services/resources/service.go`, `internal/services/facilities/service.go`):

```go
// internal/services/facilities/service.go (as built)
type Service struct {
    db          *sql.DB
    facilities  *repository.FacilityRepository
    idGenerator *util.IDGenerator
}

func NewService(db *sql.DB) *Service {
    return &Service{
        db:          db,
        facilities:  repository.NewFacilityRepository(db),
        idGenerator: util.NewIDGenerator(),
    }
}
```

Loggers are not injected; packages use `slog.Default()`, optionally tagged with `With("component", ...)` (`internal/simulation/engine.go`, `internal/server/server.go`).

### Error Handling

Always wrap errors with context:

```go
resident, err := s.repo.GetResident(ctx, id)
if err != nil {
    return nil, fmt.Errorf("population service: getting resident %s: %w", id, err)
}
```

### Transaction Management

Repository methods accept an optional `*sql.Tx`; when it is `nil` they run against the pool. Services open the transaction. `population.Service.RegisterBirth` is the reference example (`internal/services/population/service.go`, `RegisterBirth`): it calls `s.db.BeginTx`, defers `tx.Rollback()`, creates the resident inside the transaction, then commits. `database.DB.WithTransaction` (`internal/database/database.go`) offers the same pattern with panic-safe rollback and is used by the migrator.

### Logging

Use structured logging with key-value pairs:

```go
s.logger.Info("resident created",
    "resident_id", resident.ID,
    "registry_number", resident.RegistryNumber,
    "entry_type", resident.EntryType,
)
```

`slog` output goes to the file named by `logging.file` as JSON, or to stderr as text when no file is configured (`cmd/vtuos/main.go`, `setupLogging`). In `connect` mode logs are discarded when no file is configured so they never overwrite the terminal (`cmd/vtuos/main.go`, `setupClientLogging`).

## Build System

Go toolchain: `go.mod` declares `go 1.24.0`; the figures in this document were produced with go1.24.7. `CGO_ENABLED=0` is set for every build target and the resulting binary is static (`ldd` reports "not a dynamic executable").

### Makefile Targets

Every target in `Makefile`, in file order:

| Target | Purpose |
| --- | --- |
| `all` | Default target: `lint`, then `test`, then `build` (requires `golangci-lint`). |
| `build` | `CGO_ENABLED=0 go build` with `-s -w` and `-X main.Version`/`-X main.BuildTime` → `bin/vtuos`. |
| `build-pi` | Same, `GOOS=linux GOARCH=arm64` → `bin/vtuos-linux-arm64`. |
| `build-pi-zero` | arm64 with `-tags sqlite_omit_load_extension` → `bin/vtuos-linux-arm64-minimal`. |
| `build-all` | `build` + `build-pi` + `build-pi-zero`. |
| `test` | `go test -v -race -cover ./internal/...`. |
| `test-coverage` | Same with `-coverprofile=coverage.out`, writes `coverage.html`, prints the `total` line. |
| `test-pkg` | `go test -v -race -coverprofile` for `PKG=<path>`; errors if `PKG` is unset. |
| `test-coverage-check` | Runs tests and exits 1 if total coverage is below 80% (uses `bc`). |
| `test-integration` | `go test -v -race -tags=integration ./internal/...` (no files carry the tag). |
| `test-all` | `test` + `test-integration`. |
| `lint` | `golangci-lint run ./...`; fails with an install hint if not on `PATH`. |
| `fmt` | `go fmt ./...` then `goimports -w .`. |
| `run` | `build`, then `./bin/vtuos` (local console). |
| `serve` | `build`, then `./bin/vtuos serve`. |
| `connect` | `build`, then `./bin/vtuos connect $(ADDR)`; errors if `ADDR` is unset. |
| `migrate` | `./bin/vtuos --migrate-only` (does not build first). |
| `seed` | `./bin/vtuos --seed` (does not build first). |
| `clean` | Removes `bin/`, `coverage.out`, `coverage.html`, and `*.db`, `*.db-wal`, `*.db-shm` in the repo root. |
| `verify-static` | Builds to `/tmp/vtuos-check` and checks `ldd` reports a static binary. |
| `dev-tools` | `go install` of `golangci-lint` and `goimports`. |
| `backup` | Copies `./vault.db` to `backups/vault-<timestamp>.db` if it exists (plain `cp`, not `VACUUM INTO`). |
| `help` | Prints the target list. |

There are no `vet`, `deps`, `migrate-up`, `migrate-down`, or `migrate-status` targets. `go vet ./...` reports nothing on the current tree.

> ⚠ See [#25](https://github.com/jstephenperry/vt-uos/issues/25).

### Version stamping

`-X main.Version` and `-X main.BuildTime` populate `Version`/`BuildTime` in `cmd/vtuos/main.go`; `runLocal` copies them into `tui.Version`/`tui.BuildTime` for the header, and `connectMain` passes `Version` to `client.NewConn` so the master records it in `ClientInfo.Version`. `vtuos -version` prints them and exits 0.

## AI Assistant Guidelines

When implementing features for this project:

1. **Start with the data model** - Ensure the SQLite schema supports the feature (see [DATABASE.md](DATABASE.md))
2. **Write the repository layer first** - Pure data access, no business logic
3. **Implement service layer with tests** - Business logic lives here (see [MODULES.md](MODULES.md)); note that none of the three existing services has a test file yet
4. **Build TUI last** - UI should call services, not repositories (see [TUI.md](TUI.md))
5. **Keep functions small** - Under 50 lines preferred
6. **Handle all errors** - No ignored errors, wrap with context
7. **Use transactions** - Multi-step database operations must be atomic
8. **Log meaningfully** - Debug for development, Info for operations, Warn/Error for problems
9. **Follow existing patterns** - Look at implemented modules for conventions
10. **Ask clarifying questions** - If a requirement is ambiguous, ask before assuming

## References

### Fallout Lore (for authenticity)

- Vault-Tec terminal entries from Fallout 3, 4, 76
- Vault Dweller's Survival Guide (in-game documents)
- Overseer logs and protocols

### Real-World Analogs

- HMIS (Homeless Management Information System) - population tracking
- Cold War civil defense shelter protocols
- Submarine crew management systems
- Space station life support monitoring
- Hospital information systems (for medical module)

### Technical References

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss Styling](https://github.com/charmbracelet/lipgloss)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [Go Database Patterns](https://www.alexedwards.net/blog/organising-database-access)
