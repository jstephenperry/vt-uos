# Deployment: Local, Master Server, and Client Terminals

VT-UOS ships as a single binary with three run modes, dispatched on the first argument in `cmd/vtuos/main.go`.
The server and client modes use only the Go standard library for transport (`net/http`, `html/template`, `embed`) — no web framework.

```
vtuos                 Local operator console (TUI + in-process control core)
vtuos serve           Master server (authoritative core + JSON/SSE API + web console)
vtuos connect <addr>  Client display/operator terminal driven by a master
```

Flags for all three modes are listed in [CLI.md](CLI.md); the HTTP and client protocol are specified in [API.md](API.md).

## Exhibit topology

```
                        ┌──────────────────────────────┐
                        │        vtuos serve           │
                        │  authoritative control core  │
                        │  JSON API · SSE · web console │
                        └───────────────┬──────────────┘
                                        │ HTTP / SSE
        ┌───────────────────┬───────────┴───────────┬───────────────────┐
        ▼                   ▼                       ▼                   ▼
  vtuos connect       vtuos connect           vtuos connect       Web browser
  (atrium display)    (corridor display)      (operator podium)   (overseer console)
```

In code terms, "exhibit topology" means one process owns the database and the simulation engine, and every other screen is a thin renderer of that process's state:

- `vtuos serve` runs `prepare` (config, logging, optional restore, database recovery, migrations), starts a `simulation.Engine`, and hands it to `server.New`; the engine is the only writer to the vault database.
- `vtuos connect` loads configuration (falling back to `config.Default()` if loading fails) but opens no database and runs no engine; it only holds a `client.Conn` to the master.
- Client terminals receive state over the SSE stream and report telemetry on a heartbeat; operators drive the simulation and the terminals through the token-gated API, either from the embedded web console or with `curl`.
- The local console (`vtuos` with no subcommand) is the same TUI with the engine attached in-process (`tui.RunWithEngine`); it does not talk to a master.

## Master server (`vtuos serve`)

```bash
vtuos serve --config testdata/vault-exhibit.toml
./bin/vtuos serve --listen 127.0.0.1:18081 --token SECRET
```

`serve` accepts `--config`, `--debug`, `--listen`, `--token`, and `--restore`; `--listen` and `--token` override `[server] listen` and `[server] admin_token` after the config file is loaded.

`[server]` configuration (an example with a token set; the defaults from `internal/config/config.go` are in the table below):

```toml
[server]
listen = ":8080"
admin_token = "change-me-overseer-token"
enable_web = true
heartbeat_seconds = 5
client_timeout_seconds = 20
```

| Key | Default | Runtime behaviour |
| --- | --- | --- |
| `listen` | `":8080"` | passed to `http.Server.Addr`; plain HTTP, no TLS |
| `admin_token` | `""` | operator token for the control and client-management routes; empty means those routes accept every request |
| `enable_web` | `true` | mounts the web console at `/` and `/static/` |
| `heartbeat_seconds` | `5` | cadence handed to clients at registration; `<= 0` is treated as 5 |
| `client_timeout_seconds` | `20` | silence after which a client is listed offline; `<= 0` is treated as 20; entries silent for ten times this are deleted once a minute |

> ⚠ See [#39](https://github.com/jstephenperry/vt-uos/issues/39).

`Config.Validate` checks the vault, simulation, display, logging, and database sections but has no validator for `[server]` or `[client]`, so these keys are never range-checked at load time.

### Token provisioning

The operator token is a shared secret compared with `==`; there are no per-operator accounts.
Set it in `[server] admin_token`, or pass `--token` on the command line, which replaces the configured value for that process.
When the effective token is empty, `Start` logs `no admin_token configured: simulation control and client management endpoints are UNAUTHENTICATED` at `Warn` level and the server keeps running with those routes open.
Requests present the token as `Authorization: Bearer <token>` or `X-Admin-Token: <token>`; the web console uses the latter and keeps the token in the browser's `localStorage` under `vtuos_token`.
Set a token for any networked deployment.

> ⚠ See [#6](https://github.com/jstephenperry/vt-uos/issues/6) and [#8](https://github.com/jstephenperry/vt-uos/issues/8).

Startup and shutdown: `serve` logs `master server listening` with `addr`, `web_console`, and `vault`; on SIGINT/SIGTERM it cancels the context, gives `Shutdown` five seconds, and a ten-second backstop forces exit if shutdown stalls.
Logs go to `[logging] file` as JSON when set, otherwise to stderr as text.

> ⚠ See [#37](https://github.com/jstephenperry/vt-uos/issues/37).

### Web administration console

With `enable_web = true`, browse to `http://<master>:8080/`.
The console (`internal/server/web/index.html.tmpl`, `static/app.js`) provides:

- **OVERVIEW** — population, power, systems summary, resource runway, and active alerts, rendered from `state` events on `EventSource("/api/v1/stream")`.
- **SIMULATION** — RESUME, PAUSE, STEP 1h/1d/1w, SNAPSHOT, and a TIME SCALE selector (Frozen, 1×, 60×, 600×, 1440×, 3600×, 14400×, 86400×) with APPLY SCALE; each button POSTs to the matching `/api/v1/sim/*` route.
  > ⚠ See [#30](https://github.com/jstephenperry/vt-uos/issues/30).
- **SYSTEMS** — the `system_list` table with efficiency bars.
- **TERMINALS** — `GET /api/v1/clients` with VIEW, IDENTIFY, KIOSK, MSG, and REBOOT buttons per terminal; the tab shows "Operator token required to view terminals." when the list call is refused.
- **EVENT LOG** — `GET /api/v1/events?limit=80`.

The OVERSEER TOKEN bar at the top stores the token when SET is clicked; the TERMINALS and EVENT LOG tabs poll every `heartbeat_seconds` while visible.

### HTTP API

Read endpoints are open; the six `/api/v1/sim/*` routes, `GET /api/v1/clients`, and `POST /api/v1/clients/{id}/command` require the operator token; `heartbeat` and `result` require the per-client token from registration.
Full request/response schemas, status codes, and the SSE frame format are in [API.md](API.md).

> ⚠ See [#36](https://github.com/jstephenperry/vt-uos/issues/36) and [#41](https://github.com/jstephenperry/vt-uos/issues/41).

```bash
curl -s 127.0.0.1:18081/api/v1/status
curl -s -X POST 127.0.0.1:18081/api/v1/sim/step -H 'X-Admin-Token: SECRET' -d '{}'
```

## Client terminal (`vtuos connect`)

```bash
./bin/vtuos connect --name 'Corridor Display' --kind display 127.0.0.1:18081
```

`connect` takes the master address as its positional argument (host:port, or a full `http://`/`https://` URL; a bare host:port gets `http://` prepended) and the flags `--config`, `--name`, `--kind`, `--debug`; with no address it prints a usage line and exits 2.
The terminal name is the first non-empty of `--name`, `[client] name`, and `vtuos-terminal`; the kind is `OPERATOR` only when `--kind` or `[client] kind` equals `operator` case-insensitively, otherwise `DISPLAY`.
Client logs go to `[logging] file` when configured and are otherwise discarded so they cannot corrupt the screen.

A terminal:

1. **Registers** with the master (up to 5 attempts, 1–5 s apart) and receives an id, a token, the vault designation, and the heartbeat interval.
2. **Streams** live state over SSE and renders it as a read-only dashboard with Systems, Resources, Events, and Population pages.
3. **Reports telemetry** (uptime, current page, kiosk state, a per-second counter, error count, terminal size) on each heartbeat.
4. **Executes remote operations** returned in the heartbeat response and reports a result for each.

> ⚠ See [#20](https://github.com/jstephenperry/vt-uos/issues/20).

The SSE connection is opened with an `http.Client` whose 15-second timeout also applies to the stream, so the terminal's header flips between LIVE and OFFLINE roughly every 15 seconds as the stream is cut and reopened after a 2-second pause.
If the master restarts, the in-memory registry is empty, every heartbeat returns `401`, and the terminal stays OFFLINE; there is no re-registration path.

> ⚠ See [#18](https://github.com/jstephenperry/vt-uos/issues/18) and [#35](https://github.com/jstephenperry/vt-uos/issues/35).

`[client]` configuration provides defaults:

```toml
[client]
name = "Atrium Display"
kind = "DISPLAY"   # DISPLAY (unattended) or OPERATOR
```

Local keys on a client terminal: `1`–`5` switch between dashboard, systems, resources, events, and population; `q` or `esc` quits immediately with no confirmation; `ctrl+c` quits even in kiosk mode.
The footer reads `[1]Dashboard [2]Systems [3]Resources [4]Events [5]Population   MANAGED TERMINAL`, or `KIOSK — display managed remotely by overseer console` in kiosk mode.
The `[exhibit]` section is not read by the client; the only kiosk switch for a client terminal is the `SET_KIOSK` remote operation, and its state is lost when the process exits.

### Remote operations

Issued from the web console (TERMINALS tab) or `POST /api/v1/clients/{id}/command`; the table gives what `applyCommands` in `internal/client/client.go` actually does.

| Command       | Args                   | Effect on the terminal                                   |
| ------------- | ---------------------- | -------------------------------------------------------- |
| `SWITCH_VIEW` | `view`                 | Change the displayed page; unknown names render the dashboard |
| `SET_KIOSK`   | `enabled` (true/false) | Lock or unlock local keys except `ctrl+c`                |
| `MESSAGE`     | `text`                 | Show `text` as a banner for 10 seconds                   |
| `IDENTIFY`    | —                      | Show a banner with the terminal's id prefix for 6 seconds |
| `REFRESH`     | —                      | Reports `refreshed`; no other effect                     |
| `REBOOT`      | —                      | Shows `RECONNECTING TERMINAL…` for 5 seconds; does not reconnect |
| `SHUTDOWN`    | —                      | Quits the terminal process                                |

The web console has no button for `SHUTDOWN` or `REFRESH`; send them with `curl`.

> ⚠ See [#18](https://github.com/jstephenperry/vt-uos/issues/18), [#42](https://github.com/jstephenperry/vt-uos/issues/42) and [#43](https://github.com/jstephenperry/vt-uos/issues/43).

## Exhibit kiosk presentation

The `[exhibit]` section is read by the local console TUI (`tui.New` and `updateAttract` in `internal/tui/app.go`); it has no effect on `vtuos serve` (no TUI) or `vtuos connect` (see above).

```toml
[exhibit]
boot_sequence = true          # RobCo-style startup
attract_mode = true           # auto-rotate views when idle
attract_idle_seconds = 90
rotate_seconds = 12
kiosk = true                  # blocks q/F10 exit; see below
```

| Key | Default | What it does |
| --- | --- | --- |
| `boot_sequence` | `false` | shows a 12-line boot screen that reveals one line every 220 ms and auto-continues after 3.5 s; any key skips it |
| `attract_mode` | `false` | after `attract_idle_seconds` without a key press, jumps to the Dashboard and then rotates Dashboard → Simulation → Population → Resources → Facilities → Labor → Medical → Security → Governance every `rotate_seconds`; any key exits attract and is swallowed |
| `attract_idle_seconds` | `120` | `<= 0` is treated as 120 |
| `rotate_seconds` | `12` | `<= 0` is treated as 12 |
| `kiosk` | `false` | `q` and `F10` raise the INFO alert `Kiosk mode active — exit is disabled` instead of opening the quit dialog |

What kiosk lockout actually covers: only the quit keys `q` and `F10`.
`ctrl+c` still opens the CONFIRM EXIT dialog in kiosk mode, and `y` there exits the program.
Add (`a`), edit (`e`), register death (`d`), the category filters, and all simulation controls (`space`, `+`, `-`, `s`, `h`, `k`, `a`) remain active in kiosk mode.
Attract rotation switches the module without issuing the Population/Resources/Facilities load commands, so those screens show whatever was last loaded (or their empty state) while rotating.
The footer reads `ATTRACT MODE — press any key to take control` while attract is active.

> ⚠ See [#19](https://github.com/jstephenperry/vt-uos/issues/19).

These are off by default; `testdata/vault-exhibit.toml` is a ready-made exhibit profile that enables all of them, sets `time_scale = 1440.0`, and configures `[server]` with `admin_token = "change-me-overseer-token"`.

## Resetting an exhibit

Snapshot before a demonstration, then restore afterward.

A snapshot is `Engine.CreateSnapshot`: it persists engine state, then `db.Backup` runs `VACUUM INTO` a file named `vault-<YYYYMMDD-HHMMSS>.db` (real time, not vault time) in the backup directory.
The backup directory is `backups/` beside the database when `[database] path` is absolute, otherwise `$XDG_DATA_HOME/vtuos/backups` (defaulting to `~/.local/share/vtuos/backups`).
Three triggers call it: `POST /api/v1/sim/snapshot`, the web console's SNAPSHOT button, and `k` in the TUI Simulation Control view.

```bash
# Snapshot (web SIMULATION tab, TUI `k`, or the API)
curl -s -X POST 127.0.0.1:18081/api/v1/sim/snapshot -H 'X-Admin-Token: SECRET'
# {"snapshot":"<backup_dir>/vault-YYYYMMDD-HHMMSS.db"}   (name from time.Now().Format("20060102-150405"), internal/database/database.go)

# Reset to that baseline (stop the process that uses the database first, then):
vtuos serve --restore /path/to/backups/vault-YYYYMMDD-HHMMSS.db
```

`--restore <file>` exists on both `vtuos` and `vtuos serve` (not on `connect`).
At startup, before the database is opened, `restoreDatabase` removes the `-wal` and `-shm` sidecars of the configured database path, truncates the database file, copies the snapshot over it, and syncs; the normal recovery check and migrations then run against the restored file.
There is no `reset`, `backup`, or `restore` subcommand; the flag is the only reset path.

> ⚠ See [#16](https://github.com/jstephenperry/vt-uos/issues/16) and [#38](https://github.com/jstephenperry/vt-uos/issues/38).
