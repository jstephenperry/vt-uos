# Deployment: Local, Master Server, and Client Terminals

VT-UOS ships as a **single static binary** with three run modes. The server and
client modes use only the Go standard library for transport (`net/http`,
`html/template`, `go:embed`) — no web framework — so the single-binary,
no-CGO, offline-first constraints hold.

```
vtuos                 Local operator console (TUI + in-process control core)
vtuos serve           Master server (authoritative core + JSON/SSE API + web console)
vtuos connect <addr>  Client display/operator terminal driven by a master
```

## Interactive-exhibit topology

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

One master runs the simulation; any number of client terminals render the live
state it streams and report their telemetry back. An operator drives everything
— the simulation and the terminals — from the embedded web console or the API.

## Master server (`vtuos serve`)

```bash
vtuos serve                              # uses [server] config
vtuos serve --listen :8080 --token SECRET
vtuos serve --config testdata/vault-exhibit.toml
```

`[server]` configuration:

```toml
[server]
listen = ":8080"
admin_token = "change-me-overseer-token"  # required for control/admin endpoints
enable_web = true                           # serve the web console
heartbeat_seconds = 5                       # client heartbeat cadence
client_timeout_seconds = 20                 # mark a client offline after silence
```

> If `admin_token` is empty, control and client-management endpoints are
> **unauthenticated** and a warning is logged at startup. Always set a token for
> any networked deployment.

### Web administration console

With `enable_web = true`, browse to `http://<master>:8080/`. The green-phosphor
console provides:

- **Overview** — live population, power, systems, resource runway, and alerts (via SSE).
- **Simulation** — pause/resume, time-scale selector, single-step (1h/1d/1w),
  snapshot. Enter the operator token once (stored locally) to authorise actions.
- **Systems** — live facility table with efficiency.
- **Terminals** — connected clients with state/telemetry and remote-operation buttons.
- **Event log** — the operational event feed.

### HTTP API

Read endpoints are open (so displays can poll freely); control and
client-management endpoints require the operator token via
`Authorization: Bearer <token>` or `X-Admin-Token: <token>`.

| Method & path                          | Auth      | Purpose                              |
| -------------------------------------- | --------- | ------------------------------------ |
| `GET /healthz`                         | open      | Liveness                             |
| `GET /api/v1/health`                   | open      | Machine-readable health summary      |
| `GET /api/v1/state`                    | open      | Full operational snapshot            |
| `GET /api/v1/status`                   | open      | Sim status summary                   |
| `GET /api/v1/events?limit=N`           | open      | Operational event log                |
| `GET /api/v1/alerts`                   | open      | Active alerts                        |
| `GET /api/v1/systems` / `population` / `resources` | open | Section snapshots          |
| `GET /api/v1/stream`                   | open      | Live state via Server-Sent Events    |
| `POST /api/v1/sim/pause` / `resume`    | operator  | Control progression                  |
| `POST /api/v1/sim/scale`               | operator  | `{"time_scale": 1440}`               |
| `POST /api/v1/sim/step`                | operator  | `{"hours": 24}`                      |
| `POST /api/v1/sim/snapshot`            | operator  | Snapshot the database                |
| `POST /api/v1/sim/ack`                 | operator  | `{"code": "POWER_DEFICIT"}`          |
| `POST /api/v1/clients/register`        | open      | Terminal registration                |
| `POST /api/v1/clients/{id}/heartbeat`  | client    | Telemetry + receive remote ops       |
| `POST /api/v1/clients/{id}/result`     | client    | Report remote-op outcome             |
| `GET  /api/v1/clients`                 | operator  | List connected terminals             |
| `POST /api/v1/clients/{id}/command`    | operator  | Queue a remote operation             |

Example:

```bash
curl -s localhost:8080/api/v1/state | jq .population
curl -s -X POST localhost:8080/api/v1/sim/step \
     -H 'X-Admin-Token: SECRET' -d '{"hours":24}'
```

## Client terminal (`vtuos connect`)

```bash
vtuos connect 127.0.0.1:8080
vtuos connect --name "Atrium Display" --kind display master.local:8080
vtuos connect --kind operator 10.0.0.5:8080
```

A terminal:

1. **Registers** with the master and receives an id + token.
2. **Streams** live state over SSE and renders it (read-only dashboard with
   Systems / Resources / Events / Population pages).
3. **Reports telemetry** (current view, kiosk state, uptime, dimensions) on each
   heartbeat — the "report back to the server" channel.
4. **Executes remote operations** the master returns on a heartbeat.

`[client]` configuration provides defaults:

```toml
[client]
name = "Atrium Display"
kind = "DISPLAY"   # DISPLAY (unattended) or OPERATOR
```

### Remote operations

Issued from the web console (Terminals tab) or `POST /api/v1/clients/{id}/command`:

| Command       | Args                | Effect on the terminal                    |
| ------------- | ------------------- | ----------------------------------------- |
| `SWITCH_VIEW` | `view`              | Change the displayed page                 |
| `SET_KIOSK`   | `enabled` (true/false) | Lock/unlock local input                 |
| `MESSAGE`     | `text`              | Show an overseer message banner           |
| `IDENTIFY`    | —                   | Flash an identifying banner               |
| `REFRESH`     | —                   | Force a state refresh                     |
| `REBOOT`      | —                   | Reconnect the terminal                    |
| `SHUTDOWN`    | —                   | Gracefully terminate the terminal         |

## Exhibit kiosk presentation

The `[exhibit]` section adds unattended presentation, useful for displays:

```toml
[exhibit]
boot_sequence = true          # RobCo-style startup
attract_mode = true           # auto-rotate views when idle
attract_idle_seconds = 90
rotate_seconds = 12
kiosk = true                  # disable quit so visitors can't close the display
```

These are **off by default** so the console stays a utilitarian operator tool;
`testdata/vault-exhibit.toml` is a ready-made exhibit profile.

## Resetting an exhibit

Snapshot before a demonstration, then restore afterward:

```bash
# Snapshot (operator console, web Simulation tab, or API)
curl -X POST localhost:8080/api/v1/sim/snapshot -H 'X-Admin-Token: SECRET'

# Reset to a clean baseline (stop the process first, then):
vtuos serve --restore ~/.local/share/vtuos/backups/vault-20771023-094700.db
```
