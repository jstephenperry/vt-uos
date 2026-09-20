# Configuration & Setup

Configuration is a single TOML file, `vault.toml`, decoded onto `config.Default()` so any key the file omits keeps its default value (`internal/config/loader.go`, `internal/config/config.go`). The struct groups below match the Go struct layout exactly.

Unknown or misspelled keys are accepted silently: `loadFromFile` discards the `toml.MetaData` returned by `toml.Decode` and never calls `Undecoded()`, so a typo such as `admin_tokn` leaves the real key at its default with no error or warning (`internal/config/loader.go`).

> ⚠ See [#8](https://github.com/jstephenperry/vt-uos/issues/8).

## Config file search order

`config.Load` (called with `createDefault=true` by `prepare()` for local/serve mode, and by `connectMain` for client mode) resolves the file in this order:

1. The explicit `-config <path>` flag value, if non-empty — used as-is, with no further search.
2. `$XDG_CONFIG_HOME/vtuos/vault.toml`, or `~/.config/vtuos/vault.toml` if `XDG_CONFIG_HOME` is unset.
3. `./vault.toml` in the current working directory.
4. If none of the above exist, an in-memory `config.Default()` is built and an attempt is made to write it to disk (preferring the XDG path, falling back to the CWD path). If that write fails, `Load` silently returns the in-memory default with an **empty** path and a `nil` error — a caller cannot distinguish "wrote a default to disk" from "running on an ephemeral default" except by checking whether the returned path is empty.

(`internal/config/loader.go`)

There is no environment-variable override step for configuration *values* — the two environment variables below only affect where files are searched for or written, not what a decoded value is.

| Variable | Effect |
| --- | --- |
| `XDG_CONFIG_HOME` | Base directory for the config file search path (`$XDG_CONFIG_HOME/vtuos/vault.toml`) |
| `XDG_DATA_HOME` | Base directory used to resolve the database directory and the backups directory when `database.path` is relative (falls back to `~/.local/share`) |

(`internal/config/loader.go`)

A generated `vault.toml` is written with a header comment pointing readers at "CLAUDE.md for full configuration documentation" — CLAUDE.md does not document configuration keys; this file is the reference. (`internal/config/loader.go`)

`config.Save` never calls `Sync()`/`fsync` on the written config file (unlike `database/recovery.go`'s backup-file copy, which does sync), so an unsynced config write could be lost or truncated on power loss. (`internal/config/loader.go`)

`Config.Validate()` only calls the sub-validators for `Vault`, `Simulation`, `Display`, `Logging`, and `Database` — `Overseer`, `Experiment`, `Exhibit`, `Server`, and `Client` are never validated, no matter what values they hold. (`internal/config/config.go`)

> ⚠ See [#8](https://github.com/jstephenperry/vt-uos/issues/8).

## `[vault]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `designation` | string | `"Vault 076"` | required, non-empty | Vault name; used for log lines, seed data, and TUI/web display |
| `number` | int | `76` | 1–999 | Vault number; feeds registry-number generation and display |
| `region` | string | `"Appalachia"` | none | Descriptive location text only |
| `commissioned_date` | string | `"2076-10-23"` | none | Descriptive text only; not parsed to a time value anywhere |
| `sealed_date` | string | `"2077-10-23T09:47:00Z"` | RFC3339 if non-empty | Parsed by `VaultConfig.SealedDateTime()` into the vault's seal timestamp |
| `designed_capacity` | int | `500` | must be positive | Target population used by seed generation (`runSeed` passes it as `seed.Config.TargetPopulation`) |
| `vault_type` | enum: `control`, `experimental` | `"control"` | must be one of the two values | Classifies the vault (no other behavior gated on it in this slice) |

### `[vault.location]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `latitude` | float64 | `39.6295` | none | Descriptive text only |
| `longitude` | float64 | `-79.9559` | none | Descriptive text only |
| `depth_meters` | float64 | `100` | none | Descriptive text only |

(`internal/config/config.go`)

## `[overseer]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `initial_overseer_id` | string | `""` | none (`OverseerConfig` has no `Validate` method and is never validated) | Not read anywhere outside `internal/config/config.go`'s own definition and default — currently has no effect |

(`internal/config/config.go`)

## `[experiment]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `enabled` | bool | `false` | none (`ExperimentConfig` has no `Validate` method) | Not read anywhere outside `internal/config/config.go` — currently has no effect |
| `protocol_id` | string | `""` | none | Not read anywhere outside `internal/config/config.go` — currently has no effect |
| `protocol_name` | string | `""` | none | Not read anywhere outside `internal/config/config.go` — currently has no effect |
| `classification` | enum: `NONE`, `CONFIDENTIAL`, `SECRET`, `OVERSEER_ONLY` | `"NONE"` | none — accepted values are declared as constants but never checked in `Validate` | Not read anywhere outside `internal/config/config.go` — currently has no effect |

> ⚠ See [#24](https://github.com/jstephenperry/vt-uos/issues/24).

(`internal/config/config.go`)

## `[simulation]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `enabled` | bool | `true` | none | Gates whether the vault clock starts paused (`newClock`) and whether the simulation engine advances time at all (`internal/simulation/engine.go`) |
| `time_scale` | float64 | `60.0` | must be ≥ 0 | Ratio of real seconds to simulated seconds for the vault clock (`util.VaultClock`) |
| `auto_events` | bool | `true` | none | Gates whether the engine rolls for random incidents each simulated day (`internal/simulation/incidents.go`) |
| `event_frequency` | enum: `minimal`, `reduced`, `normal`, `increased`, `chaotic` | `"normal"` | must be one of the five values, or empty | Feeds the daily incident probability table (`internal/simulation/incidents.go`) |
| `start_date` | string | `"2077-10-23T09:47:00Z"` | RFC3339 if non-empty | Parsed by `SimulationConfig.StartDateTime()` as the vault clock's starting simulated time; if unset or unparseable, `newClock` falls back to the real current time |

### `[simulation.consumption]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `calorie_variance` | float64 | `0.1` | 0–1 | Random ± variance applied to per-resident calorie consumption (`internal/simulation/consumption.go`) |
| `water_variance` | float64 | `0.1` | 0–1 | Random ± variance applied to per-resident water consumption (`internal/simulation/consumption.go`) |
| `efficiency_decay_rate` | float64 | `0.001` | none | Base facility wear per simulated day; multiplied by 100 to get a percent-per-day figure, and if the resulting value is ≤ 0 the code substitutes a hard-coded `0.05`%/day instead (`internal/simulation/degradation.go`) |

(`internal/config/config.go`)

## `[display]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `color_scheme` | enum: `green_phosphor`, `amber`, `white` | `"green_phosphor"` | must be one of the three values, or empty | Selects the TUI color theme (`internal/tui/app.go` passes it to `NewTheme`) |
| `scan_lines` | bool | `true` | none | Not read anywhere outside `internal/config/config.go` — currently has no effect |
| `flicker` | bool | `false` | none | Not read anywhere outside `internal/config/config.go` — currently has no effect |
| `date_format` | string | `"2006-01-02"` | none | Go time-layout string used to format the vault clock's date in the TUI |
| `time_format` | string | `"15:04:05"` | none | Go time-layout string used to format the vault clock's time in the TUI |

> ⚠ See [#24](https://github.com/jstephenperry/vt-uos/issues/24).

(`internal/config/config.go`, `internal/tui/app.go`)

## `[logging]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `level` | enum: `debug`, `info`, `warn`, `error` | `"info"` | must be one of the four values, or empty | Sets the `slog` handler level; overridden by `-debug` on any subcommand |
| `file` | string | `"logs/vtuos.log"` | none | Log destination path; if empty, local/serve mode logs text to stderr and client mode discards logs entirely |
| `max_size_mb` | int | `10` | must be non-negative | Validated but never used for log rotation anywhere in the codebase — currently has no runtime effect beyond the validation check |
| `max_backups` | int | `5` | must be non-negative | Validated but never used for log rotation anywhere in the codebase — currently has no runtime effect beyond the validation check |

> ⚠ See [#24](https://github.com/jstephenperry/vt-uos/issues/24).

(`internal/config/config.go`, `cmd/vtuos/main.go`)

## `[database]`

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `path` | string | `"vault.db"` | required, non-empty | Database file location; resolved by `config.EnsureDataDir` (absolute path used as-is; relative path placed under `$XDG_DATA_HOME/vtuos` when creatable, otherwise left relative to the current working directory with no warning if that directory creation fails) |
| `backup_interval_hours` | int | `24` | must be non-negative | If > 0 (and a backup directory was resolved), `database.Open` starts a background ticker that calls `Backup` on this interval |
| `backup_retention_days` | int | `30` | must be non-negative | If > 0, each successful `Backup` spawns an untracked goroutine that deletes backup files older than this many days |

> ⚠ See [#16](https://github.com/jstephenperry/vt-uos/issues/16) and [#17](https://github.com/jstephenperry/vt-uos/issues/17).

(`internal/config/config.go`, `internal/config/loader.go`, `internal/database/database.go`)

## `[exhibit]`

Presentation behavior for unattended kiosk/exhibit deployments; see `testdata/vault-exhibit.toml` for an example enabling all of it. `ExhibitConfig` has no `Validate` method at all, so none of these keys are validated.

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `boot_sequence` | bool | `false` | none | Shows a boot screen at TUI startup (`internal/tui/app.go`) |
| `attract_mode` | bool | `false` | none | Enables auto-rotating views after operator idle time (`internal/tui/app.go`) |
| `attract_idle_seconds` | int | `120` | none | Idle time before attract mode engages; a value ≤ 0 falls back to a hard-coded 120s |
| `rotate_seconds` | int | `12` | none | Seconds spent on each view while in attract mode; a value ≤ 0 falls back to a hard-coded 12s |
| `kiosk` | bool | `false` | none | The struct comment says "disable quit and destructive actions"; as built, `handleKeyPress` blocks `q` and `F10` with an info alert, while `Ctrl+C` still opens the quit confirmation and the add/edit/death and simulation keys stay live (`internal/tui/app.go`) |

(`internal/config/config.go`, `internal/tui/app.go`)

## `[server]`

Only read by master-server mode (`vtuos serve`). `ServerConfig` has no `Validate` method, so none of these keys are validated — including a value supplied via the `-listen`/`-token` flag overrides, which are applied after `Config.Validate()` has already run.

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `listen` | string | `":8080"` | none | HTTP listen address for the master server; overridable with `-listen` |
| `admin_token` | string | `""` | none | Operator token required for admin-gated endpoints, presented as `Authorization: Bearer <token>` or `X-Admin-Token: <token>` (`internal/server/auth.go`, `adminToken`); when empty, the admin control surface is open to any caller — overridable with `-token` |
| `enable_web` | bool | `true` | none | Serves the embedded web administration console alongside the JSON/SSE API |
| `heartbeat_seconds` | int | `5` | none | Heartbeat cadence advertised to clients; a value ≤ 0 falls back to a hard-coded 5 |
| `client_timeout_seconds` | int | `20` | none | Silence duration after which a connected client is marked offline |

> ⚠ See [#6](https://github.com/jstephenperry/vt-uos/issues/6), [#8](https://github.com/jstephenperry/vt-uos/issues/8) and [#39](https://github.com/jstephenperry/vt-uos/issues/39).

(`internal/config/config.go`, `internal/server/server.go`, `internal/server/auth.go`, `cmd/vtuos/main.go`)

## `[client]`

Only read by client-terminal mode (`vtuos connect`). `ClientConfig` has no `Validate` method.

| Key | Type | Default | Validation | Effect |
| --- | --- | --- | --- | --- |
| `name` | string | `""` | none | Terminal display name reported to the master; overridable with `-name`, falling back to the literal `"vtuos-terminal"` if both are empty |
| `kind` | string | `"DISPLAY"` | none — any string is accepted | The terminal is `OPERATOR` only when this key **or** the `-kind` flag equals `operator` case-insensitively (a boolean OR: `-kind display` cannot override `kind = "operator"`); any other value (including a typo) silently becomes a display terminal, with no validation or warning |

(`internal/config/config.go`, `cmd/vtuos/main.go`)

## Glossary

| Term | Definition |
| --- | --- |
| Sealed Date | The vault's seal timestamp, parsed from `vault.sealed_date` (`internal/config/config.go`) |
| Vault Time | The simulated clock time reported by the engine and shown in the TUI/API as `VAULT TIME`/`vault_time` (`internal/tui/views/simulation/control.go`, `internal/protocol/protocol.go`) |
| Time Scale | Ratio of real time to vault time, set by `simulation.time_scale` and shown in the TUI as `Time Scale` (`internal/tui/app.go`) |
| Runway | Projected days until a resource category is depleted at its current consumption rate, shown as `RESOURCE RUNWAY` (`internal/models/resource.go`, `internal/client/view.go`) |
| MTBF | Mean Time Between Failures for a facility system, shown in the facilities view (`internal/models/facility.go`, `internal/tui/views/facilities/systems.go`) |
| COI | Coefficient of Inbreeding, calculated for potential offspring pairings (`internal/services/population/genetics.go`) |
