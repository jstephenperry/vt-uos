# CLI Reference

`vtuos` is a single binary with three run modes, dispatched on `os.Args[1]`: `serve` and `connect` select the corresponding mode; any other value, including no arguments at all, falls through to the local operator console (`cmd/vtuos/main.go`).

> ⚠ See [#25](https://github.com/jstephenperry/vt-uos/issues/25).

```
vtuos [flags]                 local operator console
vtuos serve [flags]           master server
vtuos connect <addr> [flags]  client display/operator terminal
```

## `vtuos [flags]` — local operator console

Runs the TUI plus an in-process simulation control core against a local SQLite database.

```
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
```

| Flag | Type | Default | Effect | Read by |
| --- | --- | --- | --- | --- |
| `-config` | string | `""` | Path passed to `config.Load` inside `prepare` | `cmd/vtuos/main.go` (`runLocal`, `prepare`) |
| `-migrate-only` | bool | `false` | Run pending migrations (via `prepare`) then return without starting the TUI or simulation engine | `cmd/vtuos/main.go` (`runLocal`) |
| `-seed` | bool | `false` | Run seed data generation then return without starting the TUI | `cmd/vtuos/main.go` (`runLocal`, `runSeed`) |
| `-version` | bool | `false` | Print `VT-UOS version <Version> (built <BuildTime>)` and exit 0 immediately, before any config is loaded | `cmd/vtuos/main.go` (`localMain`) |
| `-debug` | bool | `false` | Force the effective log level to debug regardless of `logging.level` | `cmd/vtuos/main.go` (`setupLogging`, `logLevel`) |
| `-restore` | string | `""` | Copy the named snapshot file over the live database path before recovery/migration run | `cmd/vtuos/main.go` (`prepare`, `restoreDatabase`) |

If none of `-migrate-only`/`-seed` is set, `runLocal` builds a `util.VaultClock`, starts the simulation engine (a start failure is only logged as a warning, not fatal), and blocks in the TUI until it exits (`cmd/vtuos/main.go`, `runLocal`).

## `vtuos serve [flags]` — master server

Runs the authoritative simulation engine plus the JSON/SSE API and (if `server.enable_web` is true) the embedded web console.

```
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
```

| Flag | Type | Default | Effect | Read by |
| --- | --- | --- | --- | --- |
| `-config` | string | `""` | Path passed to `config.Load` inside `prepare` | `cmd/vtuos/main.go` (`serveMain`, `prepare`) |
| `-debug` | bool | `false` | Force the effective log level to debug | `cmd/vtuos/main.go` (`setupLogging`, `logLevel`) |
| `-listen` | string | `""` | If non-empty, overwrites `cfg.Server.Listen` after config load | `cmd/vtuos/main.go` (`serveMain`) |
| `-token` | string | `""` | If non-empty, overwrites `cfg.Server.AdminToken` after config load | `cmd/vtuos/main.go` (`serveMain`) |
| `-restore` | string | `""` | Copy the named snapshot file over the live database path before recovery/migration run | `cmd/vtuos/main.go` (`prepare`, `restoreDatabase`) |

`-listen` and `-token` are applied to `cfg.Server` **after** `config.Load` has already run `Config.Validate()` inside `loadFromFile`, so an override value supplied on the command line is never validated by any code path.

> ⚠ See [#8](https://github.com/jstephenperry/vt-uos/issues/8).

## `vtuos connect <addr> [flags]` — client terminal

Requires a positional master address argument (`<addr>`); if it is empty, `connectMain` prints `usage: vtuos connect [flags] <master-address>` to stderr and returns exit code 2 without loading configuration (`cmd/vtuos/main.go`).

```
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

| Flag | Type | Default | Effect | Read by |
| --- | --- | --- | --- | --- |
| `-config` | string | `""` | Path passed to `config.Load`; unlike local/serve mode, a load failure here is **not** fatal — `connectMain` silently substitutes `config.Default()` | `cmd/vtuos/main.go` (`connectMain`) |
| `-name` | string | `""` | Terminal display name; resolved as `-name`, else `client.name` from config, else the literal fallback `"vtuos-terminal"` | `cmd/vtuos/main.go` (`connectMain`, `firstNonEmpty`) |
| `-kind` | string | `""` | Terminal kind is `OPERATOR` only when `-kind` **or** `client.kind` from config equals `operator` case-insensitively (a boolean OR, not a fallback: `-kind display` cannot override a config value of `operator`); any other value, including a typo, silently becomes `protocol.ClientDisplay` with no validation or warning | `cmd/vtuos/main.go` (`connectMain`) |
| `-debug` | bool | `false` | Force the effective log level to debug | `cmd/vtuos/main.go` (`setupClientLogging`, `logLevel`) |

`connectMain` treating a `config.Load` failure as non-fatal is inconsistent with `prepare()` (used by local and serve mode), which treats the identical error as fatal.

Client logging is routed through `setupClientLogging`, which logs to the configured `logging.file` path when present and otherwise discards all log output (`io.Discard`) specifically to keep the TUI screen clean — it never falls back to stderr the way `setupLogging` does for local/serve mode (`cmd/vtuos/main.go`).

## Startup sequence

Local and serve mode share a single setup routine, `prepare()`; connect mode does not call it and never opens a database.

Shared `prepare()` sequence, in order:

1. `config.Load` — resolve and read the TOML config (see [CONFIGURATION.md](CONFIGURATION.md) for search order and write-on-first-run behavior).
2. `setupLogging` — configure `slog`; opens the configured log file if one is set, otherwise logs as text to stderr.
3. `config.EnsureDataDir` — resolve the database directory and create it if missing.
4. If `-restore` was given, `restoreDatabase` copies the snapshot file over the live database path (removing stale `-wal`/`-shm` sidecars first) — this happens before any integrity check or migration, so a corrupt or wrong-vault snapshot is installed unconditionally.
   > ⚠ See [#38](https://github.com/jstephenperry/vt-uos/issues/38).
5. `config.BackupDir` — resolve and create the backup directory; failure here is non-fatal and only disables backups for the rest of the run.
6. If the database file already exists on disk, `database.AttemptRecovery` runs a phased integrity check, then WAL recovery, then backup restoration as needed.
7. `database.Open` — opens the SQLite connection, sets all pragmas, runs an initial integrity check (warning-only on failure), and optionally starts a background backup ticker.
8. `database.NewMigrator` followed by `MigrateUp` — applies any pending embedded migrations, one file per transaction.

(`cmd/vtuos/main.go`, `prepare`)

Mode-specific continuation after step 8:

- **local** (`runLocal`): if `-migrate-only`, log and return immediately; else if `-seed`, run seed generation (`runSeed`) and return; otherwise build the vault clock, start the simulation engine, and block in the TUI until it exits.
- **serve** (`serveMain`): apply `-listen`/`-token` overrides, start the simulation clock and engine, construct the HTTP server (`server.New`), start it in a goroutine, and block on either context cancellation or a server error.
- **connect** (`connectMain`) skips `prepare()` entirely: it loads config (falling back to `config.Default()` on error), sets up client-only logging, resolves terminal name/kind, then opens a connection to the master address and runs the client loop.

## Signals

All three modes install identical signal handling via `signalContext()`: a goroutine waits for `SIGINT` or `SIGTERM`, deregisters the signal channel, cancels the run context, and arms a 10-second `time.AfterFunc` that force-exits the process (`os.Exit(1)`) if shutdown has not completed by then — a backstop against a hung shutdown path (`cmd/vtuos/main.go`, `signalContext`).

> ⚠ See [#37](https://github.com/jstephenperry/vt-uos/issues/37).

## Exit codes

| Mode | Code | Condition | Returned by |
| --- | --- | --- | --- |
| local | `0` | Success, or `-version` was given | `cmd/vtuos/main.go` (`localMain`) |
| local | `1` | `runLocal` returned a non-nil error (config/DB/migration/TUI failure) | `cmd/vtuos/main.go` (`localMain`) |
| serve | `1` | `prepare` failed | `cmd/vtuos/main.go` (`serveMain`) |
| serve | `1` | `server.New` failed | `cmd/vtuos/main.go` (`serveMain`) |
| serve | `1` | The HTTP server reported an error on its result channel | `cmd/vtuos/main.go` (`serveMain`) |
| serve | `0` | Graceful shutdown (run context cancelled, e.g. by a signal) | `cmd/vtuos/main.go` (`serveMain`) |
| connect | `2` | Positional master-address argument was missing | `cmd/vtuos/main.go` (`connectMain`) |
| connect | `1` | `client.Run` returned a non-nil error | `cmd/vtuos/main.go` (`connectMain`) |
| connect | `0` | Success | `cmd/vtuos/main.go` (`connectMain`) |

On the error path, `serveMain`'s server-error branch returns `1` without calling `srv.Shutdown`, unlike the graceful-cancellation branch a few lines below it, which calls `srv.Shutdown` with a 5-second timeout — an error from `srv.Start()` skips cleanup of the HTTP server and any resources it holds.

## Environment variables

Only two environment variables are read anywhere in the codebase, both inside `internal/config/loader.go`, and both affect only *where* files are resolved — neither one overrides a configuration value directly.

| Variable | Effect | Read by |
| --- | --- | --- |
| `XDG_CONFIG_HOME` | Base directory for the config file search path: `$XDG_CONFIG_HOME/vtuos/vault.toml` | `internal/config/loader.go` (`xdgConfigPath`) |
| `XDG_DATA_HOME` | Base directory for the resolved database directory (falls back to `~/.local/share` if unset) | `internal/config/loader.go` (`EnsureDataDir`) |
| `XDG_DATA_HOME` | Base directory for the resolved backup directory (falls back to `~/.local/share` if unset) | `internal/config/loader.go` (`BackupDir`) |

No `VTUOS_*` environment variables exist anywhere in this repository.
