# Master Server API

The master server (`vtuos serve`) exposes the simulation control core over a plain-HTTP JSON API, a Server-Sent Events stream, and a client-terminal protocol, all served from one `net/http` mux built in `(*Server) routes()` (`internal/server/server.go`).
There is no TLS configuration anywhere in the server package; the `http.Server` sets only `Addr`, `Handler`, and a 10-second `ReadHeaderTimeout`.
Wire types live in `internal/protocol/protocol.go` and are standard-library only.
The protocol revision constant is `"1.0"` (`protocol.Version`).

> ⚠ See [#36](https://github.com/jstephenperry/vt-uos/issues/36) and [#39](https://github.com/jstephenperry/vt-uos/issues/39).

All examples below were run against `./bin/vtuos serve --listen 127.0.0.1:18081 --token SECRET` with a config based on `testdata/vault-076.toml` (Vault 076, the default vault number); the timestamps, IDs and paths in the sample output are from that run and will differ on yours.

## Conventions

- Every JSON response is written by `writeJSON` (`internal/server/auth.go`), which sets `Content-Type: application/json` and `Access-Control-Allow-Origin: *` on every response, including authenticated ones.
  > ⚠ See [#41](https://github.com/jstephenperry/vt-uos/issues/41).
- Errors use the envelope `{"error": "<message>"}` (`protocol.APIError`).
- Request bodies are read through `decodeJSON`, which wraps the body in a 1 MiB `MaxBytesReader` and decodes one JSON value; unknown fields are not rejected.
- Every handler maps any decode error, including an empty body and an oversized body, to `400` with a fixed message; the code comment promising a `413` for oversized bodies does not match the handlers.
- Routes are registered with Go 1.22 method patterns, so a wrong method returns `405` and an unknown path `404` from the mux; there is no `OPTIONS` handler.
- The only request logging is a `Debug` line when `/api/v1/stream` is opened; other requests are not access-logged (`logRequests`).
- Timestamps are Go `time.Time` values marshalled as RFC 3339; those produced by `time.Now().UTC()` carry `Z`, while registry timestamps (`server_time` on register, `issued`, `connected`, `last_seen`) come from `time.Now()` and carry the host's local offset.

## Authentication

### Operator token

The operator token is `cfg.Server.AdminToken` (`[server] admin_token` in TOML), overridable with `vtuos serve --token`.
`adminToken` reads it from the `Authorization` header when that header starts with case-insensitive `bearer ` (the remainder is trimmed), otherwise from the `X-Admin-Token` header.
`adminAuthorized` returns `true` unconditionally when the configured token is the empty string; otherwise it compares the presented token to the configured one with `==`.
The default `admin_token` is `""`, so a server started without a token answers every operator route without credentials; `Start` logs a `Warn` line saying the control and client-management endpoints are UNAUTHENTICATED.
`requireAdmin` wraps a handler and answers `401 {"error":"operator token required"}` when `adminAuthorized` is false.
There is no rate limiting and no per-operator identity; control-endpoint log lines attribute actions to `r.RemoteAddr`.

> ⚠ See [#6](https://github.com/jstephenperry/vt-uos/issues/6), [#8](https://github.com/jstephenperry/vt-uos/issues/8) and [#43](https://github.com/jstephenperry/vt-uos/issues/43).

Both header forms observed against a server started with `--token SECRET`:

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/sim/pause -H 'Authorization: Bearer SECRET'
curl -s -X POST 127.0.0.1:18081/api/v1/sim/resume -H 'X-Admin-Token: SECRET'
curl -s -w ' %{http_code}\n' -X POST 127.0.0.1:18081/api/v1/sim/resume -H 'X-Admin-Token: nope'
# {"error":"operator token required"}
#  401
```

### Client token

Client terminals authenticate with a per-client token issued at registration and sent as `X-Client-Token` on heartbeat and result calls; the registry compares it with `==` against the stored token.
The client id and token are two consecutive UUIDv7 values from the same `util.IDGenerator`.
The heartbeat and result handlers decode the request body before checking the token, so a bad token with a malformed body yields `400`, and a bad token with a valid body yields `401`.

> ⚠ See [#40](https://github.com/jstephenperry/vt-uos/issues/40).

### Route table

Auth column: `open` = no check; `operator` = wrapped in `requireAdmin`; `client` = per-client `X-Client-Token` checked inside the registry.

| Method | Path | Auth | Handler |
| --- | --- | --- | --- |
| GET | `/healthz` | open | inline, writes `ok` |
| GET | `/api/v1/health` | open | `handleHealth` |
| GET | `/api/v1/state` | open | `handleState` |
| GET | `/api/v1/status` | open | `handleStatus` |
| GET | `/api/v1/events` | open | `handleEvents` |
| GET | `/api/v1/alerts` | open | `handleAlerts` |
| GET | `/api/v1/systems` | open | `handleSystems` |
| GET | `/api/v1/population` | open | `handlePopulation` |
| GET | `/api/v1/resources` | open | `handleResources` |
| GET | `/api/v1/stream` | open | `handleStream` (SSE) |
| POST | `/api/v1/sim/pause` | operator | `handleSimPause` |
| POST | `/api/v1/sim/resume` | operator | `handleSimResume` |
| POST | `/api/v1/sim/scale` | operator | `handleSimScale` |
| POST | `/api/v1/sim/step` | operator | `handleSimStep` |
| POST | `/api/v1/sim/snapshot` | operator | `handleSimSnapshot` |
| POST | `/api/v1/sim/ack` | operator | `handleAckAlert` |
| POST | `/api/v1/clients/register` | open | `handleClientRegister` |
| POST | `/api/v1/clients/{id}/heartbeat` | client | `handleClientHeartbeat` |
| POST | `/api/v1/clients/{id}/result` | client | `handleClientResult` |
| GET | `/api/v1/clients` | operator | `handleClientList` |
| POST | `/api/v1/clients/{id}/command` | operator | `handleClientCommand` |
| GET | `/static/` | open | embedded `http.FileServer`; only when `enable_web = true` |
| GET | `/{$}` | open | web console template; only when `enable_web = true` |

Status codes for empty-body probes, first without a token and then with `X-Admin-Token` (the `400` rows are request-body rejections by the handler, not auth failures):

```
GET    /api/v1/clients                   401
POST   /api/v1/sim/pause                 401
POST   /api/v1/sim/pause                 200
POST   /api/v1/sim/step                  401
POST   /api/v1/sim/step                  400
POST   /api/v1/clients/register          400
POST   /api/v1/clients/register          400
POST   /api/v1/clients/x/command         401
POST   /api/v1/clients/x/command         400
```

## Read endpoints

None of the read handlers touch the database; they return copies of the engine's in-memory snapshot taken under its read lock (`Engine.Snapshot`, `Engine.Alerts`, `Engine.Events`).

### GET /healthz

Auth: open. Writes `200` with the plain-text body `ok`.

### GET /api/v1/health

Auth: open. Returns a health summary built from the engine snapshot and the client registry's `Count()`.

| Field | Type | Source |
| --- | --- | --- |
| `status` | string | literal `"ok"` |
| `protocol` | string | `protocol.Version` (`"1.0"`) |
| `vault` | string | snapshot `vault_designation` |
| `vault_number` | int | snapshot `vault_number` |
| `sim_status` | string | snapshot `status` (`RUNNING`/`PAUSED`/`STOPPED`) |
| `vault_time` | RFC 3339 | snapshot `vault_time` |
| `server_time` | RFC 3339 (UTC) | `time.Now().UTC()` |
| `clients_total` | int | registry entries |
| `clients_online` | int | entries whose `last_seen` is within `client_timeout_seconds` |

```bash
curl -s 127.0.0.1:18081/api/v1/health
# {"clients_online":0,"clients_total":0,"protocol":"1.0","server_time":"2026-09-20T15:14:59.442452174Z","sim_status":"RUNNING","status":"ok","vault":"Vault 076","vault_number":76,"vault_time":"2077-10-23T10:11:02.1738947Z"}
```

### GET /api/v1/state

Auth: open. Returns the full `protocol.VaultState` from `engine.Snapshot()`.

`VaultState` fields (`internal/protocol/protocol.go`):

| Field | Type | Notes |
| --- | --- | --- |
| `schema_version` | string | `"1.0"` once the engine has computed state; empty before |
| `generated` | RFC 3339 | time the snapshot was rebuilt |
| `vault_designation`, `vault_number` | string, int | from `[vault]` config |
| `status` | string | `SimStatus`: `STOPPED`, `RUNNING`, `PAUSED` |
| `time_scale` | float | vault seconds per real second |
| `vault_time`, `seal_date` | RFC 3339 | |
| `elapsed_days`, `elapsed_years` | int | since seal |
| `tick_count` | uint64 | simulated hours processed |
| `population` | `PopulationStatus` | `active`, `capacity`, `deceased`, `births`, `deaths`, `quarantined`, `average_age`, `load_pct` |
| `resources` | `[]ResourceStatus` | `category`, `label`, `on_hand`, `unit`, `daily_use`, `runway_days` (-1 = unlimited), `status` (`OK`/`WATCH`/`WARNING`/`CRITICAL`), `fill_fraction` |
| `systems` | `SystemsSummary` | `total`, `operational`, `degraded`, `offline`, `failed`, `avg_efficiency`, `overdue_maintenance` |
| `system_list` | `[]SystemStatus` | `code`, `name`, `category`, `status`, `efficiency`, `critical` |
| `power` | `PowerStatus` | `generation_kw`, `consumption_kw`, `balance_kw`, `reserve_pct` |
| `alerts` | `[]Alert` | `code`, `level` (`INFO`/`WARNING`/`CRITICAL`), `message`, `subsystem`, `raised`, `acknowledged`; `null` when there are none |
| `recent_events` | `[]EventRecord` | newest first, at most 25; `id`, `vault_time`, `category`, `type`, `severity`, `summary`, `detail` (omitted when empty) |

The `status`, `time_scale`, `vault_time`, and `tick_count` fields of the snapshot are rewritten by the engine's own refresh paths (`lightweightRefresh` in `internal/simulation/tick.go`, `recomputeState` in `internal/simulation/metrics.go`); `Pause`, `Resume`, and `SetTimeScale` update the engine's status/clock and broadcast the existing snapshot without rewriting those fields.
Because the run loop skips refreshes while the status is not `RUNNING`, a paused engine keeps reporting `"status":"RUNNING"` in `/state`, `/status`, `/health`, and the SSE stream until the next `Step` or `Resume` triggers a refresh; observed: `POST /sim/pause` returned `"status":"RUNNING"` and `/status` still said `RUNNING` four seconds later with a frozen `vault_time`.

### GET /api/v1/status

Auth: open. Returns six fields copied from the snapshot: `status`, `time_scale`, `vault_time`, `elapsed_days`, `elapsed_years`, `tick_count`.

```bash
curl -s 127.0.0.1:18081/api/v1/status
# {"elapsed_days":0,"elapsed_years":0,"status":"RUNNING","tick_count":0,"time_scale":60,"vault_time":"2077-10-23T10:20:02.17417196Z"}
```

### GET /api/v1/events?limit=N

Auth: open. Returns `engine.Events(limit)` — a JSON array of `EventRecord`, newest first.
`limit` defaults to 50; only a positive integer is accepted, and any other value (including `abc` or `0`) silently falls back to 50.
There is no upper bound in the handler; the engine clamps the limit to the number of events it holds.
An empty log serialises as `[]`.

### GET /api/v1/alerts

Auth: open. Returns `engine.Alerts()` — a JSON array of `Alert` including acknowledged ones (an empty list serialises as `[]`).

### GET /api/v1/systems

Auth: open. Returns the snapshot's `system_list` array.

### GET /api/v1/population

Auth: open. Returns the snapshot's `population` object.

### GET /api/v1/resources

Auth: open. Returns the snapshot's `resources` array.

## Simulation control

All six routes are wrapped in `requireAdmin`; none open a transaction or spawn a goroutine, and each logs `by=<RemoteAddr>` on success.

### POST /api/v1/sim/pause

Auth: operator. Body: ignored. Calls `engine.Pause()` (pauses the vault clock and sets the engine status to `PAUSED`), then returns `200` with `engine.Snapshot()` as the body.
See the note under `/api/v1/state`: the returned snapshot's `status` field may still read `RUNNING`.

### POST /api/v1/sim/resume

Auth: operator. Body: ignored. Calls `engine.Resume()` (resumes the clock, resets the engine's last-processed marker, sets `RUNNING`) and returns `200` with `engine.Snapshot()`.

### POST /api/v1/sim/scale

Auth: operator. Body: `protocol.ControlRequest`.

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `time_scale` | float | yes in practice | omitted or `0` freezes the clock; negative values are rejected |

Responses: `400 {"error":"invalid request body"}` on decode failure; `400` with the engine's error text (`time scale must be non-negative, got -1.00`) for negative scales; `200` with `engine.Snapshot()` otherwise.
The engine rejects only negative values, so there is no upper bound on the accepted scale.
The `time_scale` field in the `200` body is the pre-call value until the engine's next refresh (observed: request `1440`, response `"time_scale":60`).

> ⚠ See [#30](https://github.com/jstephenperry/vt-uos/issues/30).

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/sim/scale -H 'X-Admin-Token: SECRET' -d '{"time_scale":1440}'
curl -s -w ' %{http_code}\n' -X POST 127.0.0.1:18081/api/v1/sim/scale -H 'X-Admin-Token: SECRET' -d '{"time_scale":-1}'
# {"error":"time scale must be non-negative, got -1.00"}
#  400
```

### POST /api/v1/sim/step

Auth: operator. Body: `protocol.ControlRequest`; an empty body is a decode error (`400`), so send at least `{}`.

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `hours` | int | no | `<= 0` or absent becomes 24; values above 8760 are clamped to 8760 |

Side effects: `engine.Step` pauses the clock if it is running, advances it by `hours`, and processes the interval synchronously on the request goroutine using `r.Context()`, so a client disconnect mid-step cancels the step partway.
Responses: `500 {"error":"simulation step failed"}` if the engine returns an error; `200` with `engine.Snapshot()` otherwise.
Nothing in this layer serialises concurrent step requests.

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/sim/step -H 'X-Admin-Token: SECRET' -d '{}'
# ... "status":"PAUSED","tick_count":24 ...
curl -s -w ' %{http_code}\n' -X POST 127.0.0.1:18081/api/v1/sim/step -H 'X-Admin-Token: SECRET'
# {"error":"invalid request body"}
#  400
```

### POST /api/v1/sim/snapshot

Auth: operator. Body: ignored. Calls `engine.CreateSnapshot(r.Context())`, which persists engine state and then runs `db.Backup` (`VACUUM INTO` a file named `vault-YYYYMMDD-HHMMSS.db` in the backup directory).
Responses: `200 {"snapshot": "<absolute path>"}`; `500 {"error":"snapshot creation failed"}` on error (including when no backup directory is configured).

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/sim/snapshot -H 'X-Admin-Token: SECRET'
# {"snapshot":"<backup_dir>/vault-YYYYMMDD-HHMMSS.db"}   (name from time.Now().Format("20060102-150405"), internal/database/database.go)
```

### POST /api/v1/sim/ack

Auth: operator. Body: `protocol.ControlRequest` with `code` (string).
Calls `engine.AcknowledgeAlert(code)`, which sets `acknowledged = true` on the first alert with that code, and returns `200 {"acknowledged": <bool>}`; `false` means no alert with that code exists.

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/sim/ack -H 'X-Admin-Token: SECRET' -d '{"code":"POWER_DEFICIT"}'
# {"acknowledged":false}
```

## Client terminal endpoints

The registry (`internal/server/clients.go`) is in-memory only; a server restart forgets every client, after which their heartbeats fail with `401` and the client terminal never re-registers.

> ⚠ See [#18](https://github.com/jstephenperry/vt-uos/issues/18).

### POST /api/v1/clients/register

Auth: open; there is no limit on registrations per address.
Body: `protocol.RegisterRequest`.

> ⚠ See [#20](https://github.com/jstephenperry/vt-uos/issues/20).

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `protocol` | string | no | if non-empty and not `"1.0"`, `409 {"error":"protocol version mismatch"}`; empty passes the check |
| `name` | string | no | empty becomes `terminal-<first 8 chars of id>` |
| `kind` | string | no | anything other than `OPERATOR` is stored as `DISPLAY` |
| `version` | string | no | stored verbatim and shown in the client list |

Response `200`, `protocol.RegisterResponse`:

| Field | Type | Notes |
| --- | --- | --- |
| `client_id` | string | UUIDv7 |
| `token` | string | UUIDv7 generated immediately after the id |
| `protocol` | string | `"1.0"` |
| `vault_designation` | string | `[vault] designation` |
| `server_time` | RFC 3339 | registry `nowFn()` (local offset) |
| `heartbeat_seconds` | int | `[server] heartbeat_seconds`, or 5 when that is `<= 0` |

The registry stores `Address = r.RemoteAddr`, marks the client online, and logs `client registered`.

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/clients/register -H 'Content-Type: application/json' -d '{"protocol":"1.0","name":"Atrium Display","kind":"DISPLAY","version":"dev"}'
# {"client_id":"<uuidv7>","token":"<uuidv7-issued-immediately-after-client_id>","protocol":"1.0","vault_designation":"Vault 076","server_time":"<rfc3339>","heartbeat_seconds":5}
curl -s -w ' %{http_code}\n' -X POST 127.0.0.1:18081/api/v1/clients/register -d '{"protocol":"0.9"}'
# {"error":"protocol version mismatch"}
#  409
```

### POST /api/v1/clients/{id}/heartbeat

Auth: client (`X-Client-Token`).
Body: `protocol.Telemetry` — `uptime_seconds` (int64), `current_view` (string), `kiosk_mode` (bool), `frames_rendered` (uint64), `errors` (uint64), `width_cols` (int), `height_rows` (int), `note` (string, omitted when empty); none are required and the whole object replaces the stored telemetry.
Registry effect: updates `last_seen`, sets `online`, stores the telemetry, and returns-and-clears the pending command queue in one locked step, so delivery is at-most-once — if the response is lost, so are the commands.
Responses: `400 {"error":"invalid telemetry body"}`; `401 {"error":"unknown client or bad token"}`; `200` `protocol.HeartbeatResponse` with `acknowledged` (always `true`), `server_time` (UTC), and `commands` (array of `Command`, or `null` when the queue was empty).

> ⚠ See [#43](https://github.com/jstephenperry/vt-uos/issues/43).

```bash
curl -s -X POST 127.0.0.1:18081/api/v1/clients/$ID/heartbeat -H "X-Client-Token: $TOK" -d '{"uptime_seconds":12,"current_view":"dashboard","kiosk_mode":false,"frames_rendered":12,"errors":0,"width_cols":120,"height_rows":40}'
# {"acknowledged":true,"server_time":"2026-09-20T15:16:10.429694888Z","commands":[{"id":"01a0bf63-9818-7000-ac8c-a98f9a4179a7","type":"MESSAGE","args":{"text":"Hello from the overseer"},"issued":"2026-09-20T10:16:10.392279945-05:00"}]}
curl -s -X POST 127.0.0.1:18081/api/v1/clients/$ID/heartbeat -H "X-Client-Token: $TOK" -d '{}'
# {"acknowledged":true,"server_time":"2026-09-20T15:16:10.447841547Z","commands":null}
```

### POST /api/v1/clients/{id}/result

Auth: client (`X-Client-Token`).
Body: `protocol.CommandResult` — `command_id` (string), `ok` (bool), `message` (string, omitted when empty), `completed` (RFC 3339).
Registry effect: appends to the client's `results` slice, trimmed to the last 20 entries; nothing reads that slice — `ClientInfo`, `List`, and `Get` never expose results.
Responses: `400 {"error":"invalid result body"}`; `401 {"error":"unknown client or bad token"}`; `200 {"ok": true}`.

> ⚠ See [#43](https://github.com/jstephenperry/vt-uos/issues/43).

## Client management

### GET /api/v1/clients

Auth: operator. Returns `Registry.List()`: a JSON array of `protocol.ClientInfo` in map iteration order.

| Field | Type | Notes |
| --- | --- | --- |
| `id`, `name`, `kind`, `address`, `version` | string | as registered |
| `connected`, `last_seen` | RFC 3339 | registration time; last heartbeat time |
| `online` | bool | recomputed at list time as `now - last_seen < client_timeout_seconds` |
| `telemetry` | `Telemetry` | last heartbeat body (zero values before the first heartbeat) |
| `pending_commands` | int | current queue length |

Entries whose `last_seen` is older than ten times the client timeout (200 s at defaults) are deleted by `CleanupStale`, which runs once a minute from `cleanupLoop`.

```bash
curl -s 127.0.0.1:18081/api/v1/clients -H 'X-Admin-Token: SECRET' | jq -c '.[] | {id:(.id|.[0:8]),name,kind,online,pending_commands,address,version}'
# {"id":"01a0bf63","name":"Atrium Display","kind":"DISPLAY","online":true,"pending_commands":0,"address":"127.0.0.1:50680","version":"dev"}
```

### POST /api/v1/clients/{id}/command

Auth: operator. Body: `protocol.Command`.

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `type` | string | yes | empty → `400 {"error":"command type required"}`; any non-empty string is accepted, not only the seven `CommandType` values |
| `args` | object of string→string | no | passed through unbounded |
| `id` | string | no | assigned a UUIDv7 when empty |
| `issued` | RFC 3339 | no | set to now when zero |

Registry effect: appends to the client's pending queue, dropping the oldest entries beyond 64.
Responses: `404 {"error":"unknown client"}`; `202 {"queued": "<type>"}` and an Info log line `remote operation queued`.

```bash
curl -s -w ' %{http_code}\n' -X POST 127.0.0.1:18081/api/v1/clients/$ID/command -H 'X-Admin-Token: SECRET' -d '{"type":"MESSAGE","args":{"text":"Hello from the overseer"}}'
# {"queued":"MESSAGE"}
#  202
curl -s -w ' %{http_code}\n' -X POST 127.0.0.1:18081/api/v1/clients/$ID/command -H 'X-Admin-Token: SECRET' -d '{"type":"FROBNICATE"}'
# {"queued":"FROBNICATE"}
#  202
```

## SSE stream: GET /api/v1/stream

Auth: open.
`handleStream` requires the `ResponseWriter` to implement `http.Flusher`, else `500 streaming unsupported`.
Response headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `Access-Control-Allow-Origin: *`.
Each connection calls `engine.Subscribe()` and defers `Unsubscribe`; the subscription channel has a buffer of 4 and is primed with the current state, so the first frame arrives immediately.
Frames are written as `event: state` followed by `data: <VaultState JSON>` and a blank line; there is no `id:` field and no `retry:` directive.
A 15-second ticker makes the handler write the comment `: keep-alive` followed by a blank line, independent of frame traffic.
The engine broadcasts with a non-blocking send, so a subscriber that has not drained its 4-slot buffer misses frames rather than stalling the engine.
The loop ends when the request context is cancelled or the engine closes the channel; write errors are not checked, so disconnects are only noticed through the context.
There is no cap on the number of concurrent subscribers.

> ⚠ See [#36](https://github.com/jstephenperry/vt-uos/issues/36).

```bash
curl -s -N -m 3 -D - 127.0.0.1:18081/api/v1/stream | head -c 600
# HTTP/1.1 200 OK
# Access-Control-Allow-Origin: *
# Cache-Control: no-cache
# Connection: keep-alive
# Content-Type: text/event-stream
#
# event: state
# data: {"schema_version":"1.0","generated":"2026-09-20T15:16:20.474659337Z","vault_designation":"Vault 076", ...
```

## Client terminal protocol (`vtuos connect`)

The lifecycle implemented in `internal/client/transport.go` and `internal/client/client.go` is register → stream + heartbeat → command → result.

### 1. Register

`NewConn` prefixes `http://` when the address has no scheme and builds one `http.Client{Timeout: 15s}` that is shared by every call, including the SSE stream.
`Conn.Register` POSTs to `/api/v1/clients/register` with `protocol` fixed to `"1.0"`, plus the configured `name`, `kind`, and `version` (the binary's `Version`), and stores `client_id`, `token`, and `heartbeat_seconds` (if `> 0`) on the connection.
`client.Run` retries registration up to 5 times with 1, 2, 3, 4, 5 second waits and then fails with `could not reach master server: <last error>`.

```json
{"protocol":"1.0","name":"Atrium Display","kind":"DISPLAY","version":"dev"}
```

### 2. Stream

A background goroutine loops `Conn.StreamStates`, which GETs `/api/v1/stream`, accumulates `data:` lines (ignoring `event:` and comment lines), and on each blank line unmarshals the payload into `VaultState` and hands it to the UI; unmarshal errors drop the frame silently.
On any stream error the goroutine marks the terminal OFFLINE and reopens the stream after 2 seconds.
Because the shared client timeout is 15 seconds, every stream is force-closed by the client after 15 seconds, so the terminal flips to OFFLINE, waits 2 seconds, and reconnects continuously.

> ⚠ See [#35](https://github.com/jstephenperry/vt-uos/issues/35).

### 3. Heartbeat

Every `heartbeat_seconds` the model snapshots its telemetry and POSTs it to `/api/v1/clients/{id}/heartbeat` with `X-Client-Token`.
Telemetry sent: `uptime_seconds` (since process start), `current_view` (page name), `kiosk_mode`, `frames_rendered` (incremented once per second by the tick, not per render), `errors`, `width_cols`, `height_rows`.
A non-`200` response or transport error marks the terminal OFFLINE and increments `errors`; the client never re-registers on `401`.

```json
{"uptime_seconds":12,"current_view":"dashboard","kiosk_mode":false,"frames_rendered":12,"errors":0,"width_cols":120,"height_rows":40}
```

### 4. Command execution

Commands returned in `HeartbeatResponse.commands` are applied in order by `applyCommands`:

| `type` | `args` | What the terminal actually does |
| --- | --- | --- |
| `SWITCH_VIEW` | `view` | sets the page to the given string if non-empty; pages are `dashboard`, `systems`, `resources`, `events`, `population`, and any other value renders the dashboard |
| `SET_KIOSK` | `enabled` | `kiosk = (enabled == "true")`; kiosk state lives only in memory |
| `MESSAGE` | `text` | shows `text` verbatim as a centred banner for 10 s |
| `IDENTIFY` | — | shows a banner with the first 8 characters of the client id for 6 s |
| `REFRESH` | — | replies `refreshed`; performs no other action |
| `REBOOT` | — | shows a `RECONNECTING TERMINAL…` banner for 5 s; does not reconnect or re-register |
| `SHUTDOWN` | — | sets `quitting` and quits the program |
| anything else | — | replies `ok=false`, `unsupported command`, and increments `errors` |

> ⚠ See [#42](https://github.com/jstephenperry/vt-uos/issues/42).

### 5. Result

After each command the terminal POSTs a `CommandResult` to `/api/v1/clients/{id}/result` with `X-Client-Token`; the transport closes the body without checking the status code, and the caller discards the error.

```json
{"command_id":"01a0bf63-9818-7000-ac8c-a98f9a4179a7","ok":true,"message":"applied","completed":"2026-09-20T15:20:00Z"}
```

## Web console

`registerWebRoutes` is called only when `[server] enable_web` is true (default `true`).
It parses the embedded `web/index.html.tmpl` with `html/template` (`template.Must`, so a parse error panics at startup), serves `web/static/*` at `/static/` through `http.FileServer` (which also renders a directory listing at `/static/`), and serves `GET /{$}` by executing the template with `Vault`, `VaultNumber`, `Protocol` (hard-coded `"1.0"`), and `Heartbeat`.
The template exposes those values to the page as `window.VTUOS = { protocol, heartbeat, vaultNumber }`.
The page has five tabs: OVERVIEW, SIMULATION, SYSTEMS, TERMINALS, EVENT LOG.
`app.js` reads the operator token from `localStorage["vtuos_token"]`, writes it there when SET is clicked, and sends it as `X-Admin-Token` on every POST and on `GET /api/v1/clients`.
Live data comes from `EventSource("/api/v1/stream")` listening for `state` events; the SIMULATION tab POSTs to `/sim/pause`, `/sim/resume`, `/sim/step` (1, 24, or 168 hours), `/sim/snapshot`, and `/sim/scale` with a fixed list of scales (0, 1, 60, 600, 1440, 3600, 14400, 86400).
The TERMINALS tab lists `GET /api/v1/clients` and offers VIEW, IDENTIFY, KIOSK, MSG, and REBOOT buttons that POST `{type, args}` to `/api/v1/clients/{id}/command`, using `prompt()`/`confirm()` for arguments; there is no SHUTDOWN or REFRESH button.
The TERMINALS and EVENT LOG tabs re-fetch every `heartbeat` seconds while active.
No Content-Security-Policy or other security headers are set on the console responses.

> ⚠ See [#43](https://github.com/jstephenperry/vt-uos/issues/43).
