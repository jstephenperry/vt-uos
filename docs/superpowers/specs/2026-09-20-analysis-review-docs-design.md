# VT-UOS: As-Built Documentation Rewrite + Adversarial Review

**Date:** 2026-09-20
**Status:** Approved
**Type:** Analysis / documentation / review (no code changes)

## Goal

1. Replace the current mix of design-intent specs and post-hoc descriptions with
   documentation that describes what the code actually does today, verified
   against source, with unbuilt design intent moved to a separate roadmap.
2. Perform an adversarial system and code review across four lenses and file
   each verified finding as a GitHub issue.

Both workstreams run from the same commit and consume a shared as-built
functionality inventory.

## Decisions (from brainstorming)

| Question | Decision |
|---|---|
| Review output | Findings only. No fixes, tests, or refactors. |
| Docs stance | As-built primary; unbuilt module specs move to `docs/ROADMAP.md`. |
| Toolchain | Install Go 1.24.7 (matches `go.mod` toolchain). No golangci-lint. |
| Review lenses | Security, Correctness & integrity, Architecture, Resource/resilience — all four, in that priority order. |
| Execution | Parallel subagents; the orchestrator verifies every claim before it reaches an issue or a doc. |
| Findings destination | GitHub issues in `jstephenperry/vt-uos`, one per verified medium+ finding, low/info batched per lens, plus one tracking issue. |
| Approach | Inventory first, then fan out review and docs in parallel. |

## Current State (observed)

- ~25.6k LOC Go; phases 1–5 complete; labor/medical/security/governance unbuilt.
- Docs: `DATABASE.md`, `MODULES.md`, `TUI.md` are pre-implementation specs
  including unbuilt modules; `SIMULATION.md`, `DEPLOYMENT.md` are post-hoc.
- Visible drift: `PROGRESS.md` says "Next Phase: Phase 4"; `README.md` calls
  Resources the "Current Phase"; `DEVELOPMENT.md` embeds an example Makefile.
- No tests for `services/*`, `repository/resource_repo.go`, `client`, `config`,
  `database`, `seed`; one test file each in `simulation` and `server`.
- `internal/tui/app.go` is 2,037 lines.
- No Go toolchain on the development machine.
- `gh` authenticated as repo owner; issues enabled; only default labels exist.

## Section 1: Toolchain & Baseline

Performed by the orchestrator before any agent launches.

1. Install Go 1.24.7 to `/usr/local/go` (sudo) or `~/sdk/go1.24.7`; verify `go version`.
2. Capture to scratchpad `baseline.md`, quoting exact commands and output:
   - `go build ./...`; `CGO_ENABLED=0 go build` + `ldd` check (verify static claim)
   - `go vet ./...`
   - `go test -race -coverprofile=coverage.out ./internal/...`; per-package and total coverage
   - `go mod verify`; note dependency ages and any known advisories
   - `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build` cross-compile smoke
   - Run `./bin/vtuos --seed`, `./bin/vtuos serve`, and `curl` the API to confirm runtime claims are testable
3. Any build/vet/test failure or coverage shortfall against the documented 80% is itself a finding.

## Section 2: Inventory Round

Six read-only agents, run in parallel, each producing a Markdown inventory in
the scratchpad using a fixed schema.

| Agent | Slice | Files |
|---|---|---|
| inv-core | CLI, config, database, migrations, recovery, seed, util | `cmd/`, `internal/config`, `internal/database`, `internal/util` |
| inv-population | population models/repo/service/genetics/demographics + census TUI | `models/{resident,household}`, `repository/{resident,household}_repo`, `services/population`, `tui/views/population` |
| inv-resources | resource models/repo/service + inventory TUI | `models/resource`, `repository/resource_repo`, `services/resources`, `tui/views/resources` |
| inv-facilities | facility models/repo/service + systems TUI | `models/facility`, `repository/facility_repo`, `services/facilities`, `tui/views/facilities` |
| inv-simulation | engine and all tick stages, alerts, persistence, snapshot | `internal/simulation` |
| inv-network | protocol, server, client, TUI app shell + components | `internal/protocol`, `internal/server`, `internal/client`, `internal/tui/{app,keys,layout,simadapter,styles,components}` |

**Inventory schema (per slice):**

1. Public surface — types, exported functions, CLI flags, HTTP routes, key bindings, config keys, SQL tables touched.
2. Behavior per capability — what it does, inputs, side effects, DB writes, error paths.
3. Cross-module dependencies.
4. Test coverage present — which functions have tests.
5. Observed doc drift — each place existing docs contradict code, with `file:line` on both sides.
6. Suspicious spots — anything that looks wrong, flagged not judged; feeds the review round.

Every claim cites `file:line`. The orchestrator merges the six into
`inventory.md` and spot-checks ~10 citations per slice before round 3.

## Section 3: Review Round

Four agents, one per lens, run in parallel with the docs round. Each receives
the merged inventory, `baseline.md`, and the suspicious-spots list. Threat
framing: attacker on the exhibit LAN, Pi losing power mid-tick, 25-year
simulation run, hostile client terminal.

| Lens | Primary targets | Must-answer questions |
|---|---|---|
| Security | `server/{auth,api,sse,web,clients}.go`, `web/static/app.js`, `index.html.tmpl`, `client/transport.go`, snapshot/restore paths | Timing-safe token compare? Which routes are unauthenticated and what can they do? Can a rogue client register and receive/inject commands? XSS via resident names in web console? Path traversal on `--restore`/snapshot? CSRF on POST endpoints? Kiosk lockout escapable? |
| Correctness & integrity | `simulation/*`, `repository/*`, `database/{migrate,recovery}.go`, `services/*` | Multi-step ops in transactions? FIFO math at boundaries (partial lots, zero stock)? Tick idempotent after crash mid-tick? Seeded RNG or `time.Now()` leaking in? Races between engine goroutine, HTTP handlers, TUI? Migration rollback safety? |
| Architecture | `tui/app.go`, all layers | TUI touching repositories directly? Global state? Error-wrapping gaps? Dead code? Real per-package coverage vs. the 80% claim? |
| Resource / resilience | `database.go` pragmas, `simulation/persistence.go`, `server/sse.go`, `client/*` | Unbounded `simulation_events`/`audit_log` growth over 25 years? SSE slow-consumer handling / buffer growth? Goroutine leaks on client disconnect? Disk-full / `SQLITE_BUSY` behavior? Memory under 512MB with 1000 residents? |

**Finding record (JSON from agents):**
`title, lens, severity ∈ {critical, high, medium, low, info}, file:line, claim, failure_scenario, evidence, proposed_fix, confidence`.

**Verification gate:** the orchestrator re-reads the cited code for every
medium+ finding and attempts a repro where feasible (curl against the running
server, a throwaway Go test in the scratchpad, or a crafted DB). Unconfirmed
findings are downgraded and marked *unverified*. Duplicates across lenses are
merged.

**GitHub issues:**

- Labels to create: `severity:critical`, `severity:high`, `severity:medium`, `severity:low`, `lens:security`, `lens:correctness`, `lens:architecture`, `lens:resilience`, `adversarial-review`.
- One issue per verified medium+ finding. Title prefix `[SEC]`, `[COR]`, `[ARCH]`, `[RES]`. Body = finding record fields.
- Low/info findings batched into one "Minor findings" issue per lens.
- One tracking issue indexing all issues with a severity summary table, baseline numbers, and a link to the docs PR.
- The full issue list is shown to the user for explicit confirmation before any label or issue is created.

## Section 4: Docs Round

Six agents, one per doc cluster, run in parallel with the review round. Each
receives the merged inventory, `baseline.md`, and its relevant source files.

**Hard rule in every brief:** no sentence about behavior without a `file:line`
the agent read. If a current doc claims something the code does not do, cut it
or move it to `ROADMAP.md` — never keep it. Agents write to the scratchpad; the
orchestrator diffs against current docs and spot-checks citations before
moving files into the repo.

**Target doc set:**

| File | Action | Content |
|---|---|---|
| `README.md` | rewrite | Accurate feature list, quick start verified against the binary, doc index |
| `CLAUDE.md` | targeted edit | Fix phase checklist, module list, project tree (add `client/`, `server/`, `protocol/`, `testutil/`; drop unbuilt dirs) |
| `docs/ARCHITECTURE.md` | new | Actual layering, process/goroutine model (engine, server, TUI, client), text data-flow diagram, `VaultState` contract |
| `docs/CLI.md` | new | Every subcommand/flag of `vtuos`, `serve`, `connect`; exit codes; env vars — from `main.go` |
| `docs/DATABASE.md` | rewrite | Tables as defined in `001_initial.sql` + `002_performance_hardening.sql`, pragmas, migration mechanism, recovery behavior; unbuilt tables → ROADMAP |
| `docs/MODULES.md` | rewrite | Population, Resources, Facilities, Simulation as-built: service APIs, implemented algorithms, error behavior; Labor/Medical/Security/Governance → ROADMAP |
| `docs/API.md` | new | Every HTTP route: method, path, auth, request/response JSON, SSE event format, client register/heartbeat/command protocol |
| `docs/SIMULATION.md` | expand | Tick pipeline in order, incident weights, alert rules, persistence/continuity, snapshot/restore — verified |
| `docs/DEPLOYMENT.md` | verify/expand | Check every claim; add kiosk mode and token setup only where code supports them |
| `docs/TUI.md` | rewrite | Actual key bindings (`keys.go`), views, components, responsive breakpoints (`layout.go`); unbuilt components → ROADMAP |
| `docs/CONFIGURATION.md` | rewrite | Every TOML key from `config.go` with type/default/validation; drop sections with no code behind them |
| `docs/DEVELOPMENT.md` | edit | Real Makefile targets (remove embedded example), real test layout, real coverage numbers |
| `docs/ROADMAP.md` | new | Phase 6/7 design intent salvaged from current specs, labeled "not implemented" |
| `docs/TESTING.md` | new | What is tested, how (teatest e2e, testutil fixtures), per-package coverage table |
| `PROGRESS.md` | delete | Superseded by README roadmap + ROADMAP.md |

**Clusters:** (a) README + CLAUDE.md + PROGRESS removal + ROADMAP; (b)
ARCHITECTURE + DEVELOPMENT + TESTING; (c) CLI + CONFIGURATION; (d) DATABASE;
(e) MODULES + SIMULATION; (f) API + DEPLOYMENT + TUI.

Docs describe the code as-is, bugs included. Where a review finding contradicts
documented behavior, the doc gets a `> ⚠ See issue #N` callout.

## Section 5: Verification & Delivery

**Verification:**

- Citation audit on every doc: each `file:line`/symbol reference is checked to exist. Broken citations block the doc.
- Every command shown in docs is executed once against the built binary; output is included only if it matches.
- Every GitHub issue has had its cited code personally confirmed (Section 3 gate).
- Cross-links between docs resolve; `CLAUDE.md` Quick Reference matches the final file set.

**Delivery:**

- Branch `docs/as-built-rewrite` off `main`; one commit per doc cluster; PR to `main` with a per-file change table and baseline numbers.
- Issues created against the repo directly; tracking issue links the docs PR.
- Two explicit user confirmations before outward-facing actions: (1) final issue list before labels/issues are created; (2) docs diff summary before push/PR.

**Ordering:**

1. Toolchain + baseline (orchestrator)
2. Inventory round (6 agents)
3. Merge + spot-check (orchestrator)
4. Review round (4 agents) and docs round (6 agents) together
5. Verify findings → user confirms → create labels + issues
6. Verify docs, add `⚠ issue #N` callouts → user confirms → commit, push, PR

## Out of Scope

No code fixes, no new tests, no refactors, no golangci-lint, no Phase 6 design
work beyond salvaging existing spec text into `ROADMAP.md`.
