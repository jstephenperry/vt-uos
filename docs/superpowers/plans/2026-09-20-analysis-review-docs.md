# As-Built Docs Rewrite + Adversarial Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce verified as-built documentation for VT-UOS and file every verified adversarial-review finding as a GitHub issue, without changing any Go code.

**Architecture:** An orchestrator installs a Go toolchain and records a build/vet/test baseline, then dispatches six read-only inventory agents whose merged output becomes the single source of truth. Four review agents and six doc agents then run in parallel against that inventory. The orchestrator verifies every finding and every doc citation before anything reaches GitHub or the repo.

**Tech Stack:** Go 1.24.7 (linux/amd64), `gh` CLI, bash/awk/grep, Claude subagents (`Explore` for read-only work, `general-purpose` for doc writing).

**Spec:** `docs/superpowers/specs/2026-09-20-analysis-review-docs-design.md`

## Global Constraints

- No Go source changes, no new tests, no refactors, no golangci-lint. Findings only.
- Go version: **1.24.7**, installed to `$HOME/sdk/go1.24.7` (no passwordless sudo on this machine). Every `go` invocation in this plan assumes `export PATH=$HOME/sdk/go1.24.7/bin:$PATH`.
- Working directory for intermediate artifacts: `SCRATCH=<session scratchpad>/vtuos-review`. If the session scratchpad does not exist, use `$HOME/.cache/vtuos-review`. Never write intermediates into the repo.
- Every behavioral claim in a doc or a finding cites a source location the author read. Published docs cite at `path/file.go` or `path/file.go (FuncName)` granularity; a sidecar `$SCRATCH/citations/<docname>.tsv` holds `file<TAB>line<TAB>claim` rows for the audit.
- Docs describe the code as-is, bugs included. Design intent for unbuilt features lives only in `docs/ROADMAP.md`.
- Two outward-facing gates require explicit user confirmation: (1) before creating any GitHub label or issue; (2) before pushing the docs branch / opening the PR.
- Commits end with `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`. PR bodies end with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
- Repo: `jstephenperry/vt-uos`, default branch `main`. Docs branch: `docs/as-built-rewrite`.

---

## File Structure

**Repo files created:** `docs/ARCHITECTURE.md`, `docs/CLI.md`, `docs/API.md`, `docs/ROADMAP.md`, `docs/TESTING.md`
**Repo files rewritten/edited:** `README.md`, `CLAUDE.md`, `docs/DATABASE.md`, `docs/MODULES.md`, `docs/SIMULATION.md`, `docs/DEPLOYMENT.md`, `docs/TUI.md`, `docs/CONFIGURATION.md`, `docs/DEVELOPMENT.md`
**Repo files deleted:** `PROGRESS.md`

**Scratch layout (`$SCRATCH/`):**
```
baseline.md                 # Task 2–3 output: exact commands + outputs
coverage.out, coverage.txt  # go test coverage artefacts
inventory/inv-*.md          # Task 4: one per slice
inventory.md                # Task 5: merged
suspicious.md               # Task 5: merged "suspicious spots"
findings/<lens>.json        # Task 6 agent output
findings/verified.json      # Task 8 output after the gate
docs/<name>.md              # Task 7 agent drafts
citations/<name>.tsv        # Task 7 sidecars
issues.md                   # Task 9 preview shown to user
issue-map.tsv               # Task 9: finding_id<TAB>issue_number
```

---

### Task 1: Install Go 1.24.7

**Files:**
- Create: `$HOME/sdk/go1.24.7/` (outside repo)

**Interfaces:**
- Produces: a working `go` on `PATH` for all later tasks.

- [ ] **Step 1: Download and verify the official tarball**

```bash
mkdir -p "$HOME/sdk" && cd "$HOME/sdk"
curl -fsSLO https://go.dev/dl/go1.24.7.linux-amd64.tar.gz
curl -fsSL https://go.dev/dl/?mode=json\&include=all | grep -A3 '"go1.24.7.linux-amd64.tar.gz"' | grep sha256 | head -1
sha256sum go1.24.7.linux-amd64.tar.gz
```
Expected: the two SHA-256 values match. If they do not, stop and report.

- [ ] **Step 2: Extract to a versioned directory**

```bash
cd "$HOME/sdk" && rm -rf go1.24.7 && mkdir go1.24.7 && tar -C go1.24.7 --strip-components=1 -xzf go1.24.7.linux-amd64.tar.gz && rm go1.24.7.linux-amd64.tar.gz
```

- [ ] **Step 3: Verify**

```bash
export PATH=$HOME/sdk/go1.24.7/bin:$PATH && go version && go env GOPATH GOMODCACHE
```
Expected: `go version go1.24.7 linux/amd64`.

- [ ] **Step 4: Create the scratch layout**

```bash
mkdir -p "$SCRATCH"/{inventory,findings,docs,citations} && ls "$SCRATCH"
```

No commit (nothing in the repo changed).

---

### Task 2: Static baseline — build, vet, test, coverage, deps

**Files:**
- Create: `$SCRATCH/baseline.md`, `$SCRATCH/coverage.out`, `$SCRATCH/coverage.txt`

**Interfaces:**
- Produces: `baseline.md` with sections `## Build`, `## Static binary`, `## Vet`, `## Tests`, `## Coverage`, `## Dependencies`, `## Cross-compile`. Later tasks quote these verbatim.

- [ ] **Step 1: Build**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
{ echo '# Baseline — '"$(date -u +%Y-%m-%dT%H:%MZ)"' — commit '"$(git rev-parse --short HEAD)"; echo; echo '## Build'; echo '```'; echo '$ go build ./...'; go build ./... 2>&1; echo "exit=$?"; echo '```'; } | tee "$SCRATCH/baseline.md"
```
Expected: `exit=0`. A non-zero exit is a critical finding; record it and continue.

- [ ] **Step 2: Static binary check**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
{ echo; echo '## Static binary'; echo '```'; echo '$ CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/vtuos ./cmd/vtuos && ldd bin/vtuos'; CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/vtuos ./cmd/vtuos 2>&1; ldd bin/vtuos 2>&1; ls -la bin/vtuos; echo '```'; } | tee -a "$SCRATCH/baseline.md"
```
Expected: `not a dynamic executable`. Record binary size.

- [ ] **Step 3: Vet**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
{ echo; echo '## Vet'; echo '```'; echo '$ go vet ./...'; go vet ./... 2>&1; echo "exit=$?"; echo '```'; } | tee -a "$SCRATCH/baseline.md"
```
Every vet message becomes a candidate finding (lens: correctness or architecture).

- [ ] **Step 4: Tests with race detector and coverage**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
{ echo; echo '## Tests'; echo '```'; echo '$ go test -race -count=1 -coverprofile=$SCRATCH/coverage.out ./internal/...'; go test -race -count=1 -coverprofile="$SCRATCH/coverage.out" ./internal/... 2>&1 | tee "$SCRATCH/test-output.txt" | grep -Ev '^(=== RUN|--- PASS|PASS$)'; echo "exit=${PIPESTATUS[0]}"; echo '```'; } | tee -a "$SCRATCH/baseline.md"
```
Expected: record exit code, every `FAIL`, every `DATA RACE`, and any package with `[no test files]`. Full output stays in `test-output.txt`.

- [ ] **Step 5: Coverage table**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
go tool cover -func="$SCRATCH/coverage.out" > "$SCRATCH/coverage.txt"
{ echo; echo '## Coverage'; echo; echo '| Package | Statements covered |'; echo '|---|---|'; go test -count=1 -cover ./internal/... 2>/dev/null | awk '/coverage:/{for(i=1;i<=NF;i++) if($i=="coverage:") print "| "$2" | "$(i+1)" |"} /no test files/{print "| "$2" | 0.0% (no test files) |"}'; echo; echo 'Total: '"$(grep total "$SCRATCH/coverage.txt" | awk '{print $NF}')"; echo; echo 'Documented claim (CLAUDE.md, DEVELOPMENT.md, Makefile test-coverage-check): 80%'; } | tee -a "$SCRATCH/baseline.md"
```

- [ ] **Step 6: Dependencies**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
{ echo; echo '## Dependencies'; echo '```'; echo '$ go mod verify'; go mod verify 2>&1; echo '$ go list -m -u all (direct only)'; go list -m -u -f '{{if not .Indirect}}{{.Path}} {{.Version}}{{if .Update}} -> {{.Update.Version}}{{end}}{{end}}' all 2>&1 | grep -v '^$'; echo '$ go run golang.org/x/vuln/cmd/govulncheck@latest ./...'; go run golang.org/x/vuln/cmd/govulncheck@latest ./... 2>&1 | tail -40; echo '```'; } | tee -a "$SCRATCH/baseline.md"
```
Each govulncheck hit that is reachable becomes a security finding.

- [ ] **Step 7: Cross-compile smoke**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
{ echo; echo '## Cross-compile'; echo '```'; echo '$ GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /dev/null ./cmd/vtuos'; GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /dev/null ./cmd/vtuos 2>&1; echo "exit=$?"; echo '$ make build-pi-zero flags: -tags sqlite_omit_load_extension'; GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags sqlite_omit_load_extension -o /dev/null ./cmd/vtuos 2>&1; echo "exit=$?"; echo '```'; } | tee -a "$SCRATCH/baseline.md"
```

- [ ] **Step 8: Verify baseline.md is complete**

```bash
grep -c '^## ' "$SCRATCH/baseline.md"
```
Expected: `7`. No commit.

---

### Task 3: Runtime baseline — seed, serve, API smoke

**Files:**
- Modify: `$SCRATCH/baseline.md` (append `## Runtime`)
- Create: `$SCRATCH/runtime/` (temp DB, logs)

**Interfaces:**
- Consumes: `bin/vtuos` from Task 2.
- Produces: confirmed CLI flags, working data dir override, live API route list with auth status. Used by doc cluster (c) and (f) and by the security lens.

- [ ] **Step 1: Discover flags without reading memory**

```bash
cd /home/stephen-perry/Development/vt-uos
{ echo; echo '## Runtime'; echo '```'; for sub in "" serve connect; do echo "\$ ./bin/vtuos $sub --help"; ./bin/vtuos $sub --help 2>&1; done; echo '```'; } | tee -a "$SCRATCH/baseline.md"
```

- [ ] **Step 2: Seed into an isolated data dir**

Identify the flag or env var that sets the database path from the `--help` output above (check `internal/config/loader.go` for env var names: `grep -n 'Getenv\|VTUOS_' internal/config/loader.go internal/config/config.go`). Then:

```bash
mkdir -p "$SCRATCH/runtime" && cd /home/stephen-perry/Development/vt-uos
# Replace <DBFLAG> with the discovered flag/env, e.g. --db "$SCRATCH/runtime/vault.db" or VTUOS_DB_PATH=...
{ echo '```'; echo '$ ./bin/vtuos <DBFLAG> --seed'; timeout 120 ./bin/vtuos <DBFLAG> --seed 2>&1 | tail -20; echo "exit=$?"; echo '```'; } | tee -a "$SCRATCH/baseline.md"
sqlite3 "$SCRATCH/runtime/vault.db" '.tables' 2>/dev/null || echo "sqlite3 CLI not installed — use: go run modernc.org/sqlite/... not needed; skip table dump"
```
Record which tables exist and row counts for `residents`, `households`, `resource_stocks`, `facilities` if `sqlite3` is available.

- [ ] **Step 3: Start the server in the background and probe every route**

```bash
cd /home/stephen-perry/Development/vt-uos
# Discover the token/port flags from serve --help; substitute below.
(./bin/vtuos serve <DBFLAG> --addr 127.0.0.1:18080 <TOKENFLAG> testtoken > "$SCRATCH/runtime/serve.log" 2>&1 &) ; sleep 3
probe() { printf '%-6s %-32s ' "$1" "$2"; curl -s -o /dev/null -w '%{http_code}' -X "$1" ${3:+-H "$3"} "http://127.0.0.1:18080$2"; echo; }
{ echo '```'; echo '# route probes — no auth / with auth'
for r in /healthz /api/v1/health /api/v1/state /api/v1/status /api/v1/events /api/v1/alerts /api/v1/systems /api/v1/population /api/v1/resources /api/v1/clients / /static/app.js; do probe GET $r; done
for r in /api/v1/sim/pause /api/v1/sim/resume /api/v1/sim/step /api/v1/sim/scale /api/v1/sim/snapshot /api/v1/sim/ack /api/v1/clients/register /api/v1/clients/x/heartbeat /api/v1/clients/x/result /api/v1/clients/x/command; do probe POST $r; probe POST $r "Authorization: Bearer testtoken"; done
echo '# SSE first 5 lines'; timeout 3 curl -sN http://127.0.0.1:18080/api/v1/stream | head -5
echo '```'; } | tee -a "$SCRATCH/baseline.md"
```
Adjust the auth header form to whatever `internal/server/auth.go` actually reads (`grep -n 'Header.Get\|Query().Get\|Cookie' internal/server/auth.go`). Record which POST routes return 2xx **without** a token — each is a security finding candidate.

- [ ] **Step 4: Stop the server, keep the DB for later repro**

```bash
pkill -f 'bin/vtuos serve' ; sleep 1; tail -20 "$SCRATCH/runtime/serve.log" >> "$SCRATCH/baseline.md"
```

- [ ] **Step 5: Confirm section count**

```bash
grep -c '^## ' "$SCRATCH/baseline.md"
```
Expected: `8`. No commit.

---

### Task 4: Inventory round — dispatch six read-only agents

**Files:**
- Create: `$SCRATCH/inventory/inv-core.md`, `inv-population.md`, `inv-resources.md`, `inv-facilities.md`, `inv-simulation.md`, `inv-network.md`

**Interfaces:**
- Produces: six inventories in the exact schema below. Task 5 merges them; Tasks 6–7 consume the merge.

- [ ] **Step 1: Write the shared brief to a file so every agent gets identical instructions**

```bash
cat > "$SCRATCH/inventory/BRIEF.md" <<'EOF'
You are producing an AS-BUILT FUNCTIONALITY INVENTORY for one slice of the Go project at /home/stephen-perry/Development/vt-uos. You are READ-ONLY: do not modify any file in the repo. Write your output ONLY to the path given at the end.

RULES
- Every claim about behavior cites `path/file.go:LINE` (the line you actually read). No citation, no claim.
- Describe what the code DOES, not what comments or docs say it does. If a comment and the code disagree, say so under "Doc drift".
- Do not judge or fix. Flag oddities under "Suspicious spots" with one sentence each and a citation.
- Read whole files in your slice with `cat -n`. Use grep across the repo only to trace callers/callees.
- Existing docs to compare against for drift: README.md, CLAUDE.md, docs/*.md, PROGRESS.md.

OUTPUT SCHEMA (use these exact H2 headings, in this order)
## 1. Public surface
Tables (one per kind that applies): exported types; exported funcs/methods with signatures; CLI subcommands & flags (name, type, default, help text); HTTP routes (method, path, handler, auth wrapper yes/no); TUI key bindings (key → action → view); TOML config keys (path, type, default, validation); SQL tables/columns actually read or written (table → ops → file:line).
## 2. Behavior by capability
One H3 per capability (e.g. "### Register death", "### FIFO consumption"). For each: what it does, inputs, side effects, DB writes (tables), transaction used yes/no, error paths and how errors are wrapped/returned, concurrency (goroutines, mutexes, channels touched). Cite every sentence.
## 3. Cross-module dependencies
Bullets: "this slice imports X for Y (file:line)"; "this slice is called by X (file:line)". Note any layering violation (e.g., tui importing repository) explicitly.
## 4. Test coverage present
Table: function/method → test file:line that exercises it, or "none". List behaviors in §2 with no test at all.
## 5. Doc drift
Table: doc file:line → what it claims → code file:line → what code does. Include claims about things that do not exist at all.
## 6. Suspicious spots
Bullets: one sentence + file:line + which lens it likely belongs to (security / correctness / architecture / resilience). Be generous; the review round will triage.
EOF
```

- [ ] **Step 2: Dispatch all six agents in ONE message (parallel)**

Use the `Agent` tool with `subagent_type: "Explore"` six times in a single response. Each prompt is: the full contents of `$SCRATCH/inventory/BRIEF.md`, then the slice block below, then `Write your inventory to: $SCRATCH/inventory/<name>.md` (substitute the real path).

Slice blocks:

```
SLICE inv-core — CLI, config, database, migrations, recovery, seed, util
Files: cmd/vtuos/main.go, internal/config/*.go, internal/database/*.go, internal/database/migrations/*.sql, internal/database/seed/*.go, internal/util/*.go, Makefile, testdata/*.toml
Also record: exact PRAGMAs set; migration table name and versioning; what recovery.go does on corrupt/locked DB; every env var read.
```
```
SLICE inv-population — population domain end to end
Files: internal/models/{resident,household,common}.go, internal/repository/{resident,household}_repo.go, internal/services/population/*.go, internal/tui/views/population/*.go, plus matching *_test.go for §4
Also record: the genetics/inbreeding algorithm as implemented (coefficient formula, generations searched); demographics stats computed; what "register death" writes.
```
```
SLICE inv-resources — resources domain end to end
Files: internal/models/resource.go, internal/repository/resource_repo.go, internal/services/resources/{service,types}.go, internal/tui/views/resources/*.go, plus matching *_test.go for §4
Also record: FIFO consumption algorithm step by step; runway forecast formula; ration allocation rules; expiration handling; every transaction boundary.
```
```
SLICE inv-facilities — facilities domain end to end
Files: internal/models/facility.go, internal/repository/facility_repo.go, internal/services/facilities/service.go, internal/tui/views/facilities/*.go, plus matching *_test.go for §4
Also record: efficiency/degradation math owned here vs. in internal/simulation; maintenance scheduling rules; alert generation.
```
```
SLICE inv-simulation — simulation control core
Files: internal/simulation/*.go (all), plus engine_test.go for §4
Also record: exact tick pipeline order with file:line per stage; RNG source and seeding; how simulated time advances; every table written per tick; what is persisted for power-loss continuity and when; snapshot/restore mechanism; incident weight table; alert thresholds; how VaultState is built and published (channels/callbacks, buffering).
```
```
SLICE inv-network — protocol, master server, client terminal, TUI shell
Files: internal/protocol/protocol.go, internal/server/*.go, internal/server/web/**, internal/client/*.go, internal/tui/{app,keys,layout,simadapter,styles}.go, internal/tui/components/*.go, plus *_test.go for §4
Also record: full route table with auth wrapper; token source and comparison method; SSE implementation (fan-out, buffers, disconnect handling); client registration/heartbeat/command/result lifecycle and how the server trusts client identity; kiosk/lockout implementation; web console template escaping and JS that injects data; every TUI key binding and view transition; responsive breakpoints.
```

- [ ] **Step 3: Wait for all six; confirm files exist and are non-trivial**

```bash
wc -l "$SCRATCH"/inventory/inv-*.md && for f in "$SCRATCH"/inventory/inv-*.md; do echo "$f: $(grep -c '^## ' "$f") sections"; done
```
Expected: six files, each with 6 `##` sections. Re-dispatch any agent whose file is missing a section.

No commit.

---

### Task 5: Merge inventories and spot-check citations

**Files:**
- Create: `$SCRATCH/inventory.md`, `$SCRATCH/suspicious.md`, `$SCRATCH/drift.md`

**Interfaces:**
- Produces: `inventory.md` (all six, each under an H1 with its slice name), `suspicious.md` (all §6 bullets, tagged by slice), `drift.md` (all §5 tables). Tasks 6 and 7 receive these paths.

- [ ] **Step 1: Merge**

```bash
cd "$SCRATCH" && : > inventory.md && : > suspicious.md && : > drift.md
for s in core population resources facilities simulation network; do
  f="inventory/inv-$s.md"
  { echo "# SLICE: $s"; echo; cat "$f"; echo; } >> inventory.md
  { echo "## $s"; awk '/^## 6\. Suspicious/{p=1;next} /^## /{p=0} p' "$f"; echo; } >> suspicious.md
  { echo "## $s"; awk '/^## 5\. Doc drift/{p=1;next} /^## /{p=0} p' "$f"; echo; } >> drift.md
done
wc -l inventory.md suspicious.md drift.md
```

- [ ] **Step 2: Automated citation check across the merged inventory**

```bash
cd /home/stephen-perry/Development/vt-uos
grep -oE '(cmd|internal|docs|testdata)/[A-Za-z0-9_./-]+\.(go|sql|md|toml|tmpl|js|css|html):[0-9]+' "$SCRATCH/inventory.md" | sort -u | while IFS=: read f l; do
  if [ ! -f "$f" ]; then echo "MISSING FILE $f:$l"; elif [ "$l" -gt "$(wc -l < "$f")" ]; then echo "LINE OUT OF RANGE $f:$l (max $(wc -l < "$f"))"; fi
done | tee "$SCRATCH/inventory-citation-errors.txt"; echo "errors: $(wc -l < "$SCRATCH/inventory-citation-errors.txt")"
```
Expected: 0 errors. If any, fix by re-reading the code yourself and correcting the inventory line (do not re-dispatch for a handful).

- [ ] **Step 3: Manual spot-check — 10 random citations per slice**

```bash
cd /home/stephen-perry/Development/vt-uos
for s in core population resources facilities simulation network; do
  echo "=== $s ==="
  grep -oE '(cmd|internal)/[A-Za-z0-9_./-]+\.(go|sql):[0-9]+' "$SCRATCH/inventory/inv-$s.md" | sort -u | shuf -n 10 --random-source=<(yes) | while IFS=: read f l; do
    echo "--- $f:$l"; sed -n "$((l>2?l-2:1)),$((l+2))p" "$f"
  done
done > "$SCRATCH/spotcheck.txt"; wc -l "$SCRATCH/spotcheck.txt"
```
Read `spotcheck.txt` alongside the inventory sentence for each. For any citation that does not support its claim, correct the inventory in place and note the slice as low-trust in a `# TRUST NOTES` block at the top of `inventory.md`.

- [ ] **Step 4: Fold baseline facts into the inventory header**

```bash
cd "$SCRATCH" && { echo '# BASELINE SUMMARY (see baseline.md for full output)'; echo; grep -A2 '^## Build' baseline.md | tail -1; echo "Total coverage: $(grep total coverage.txt | awk '{print $NF}')"; grep -c 'FAIL' test-output.txt | xargs echo 'FAIL lines in test output:'; grep -c 'DATA RACE' test-output.txt | xargs echo 'DATA RACE lines:'; echo; cat inventory.md; } > inventory.tmp && mv inventory.tmp inventory.md && head -12 inventory.md
```

No commit.

---

### Task 6: Review round — dispatch four lens agents

**Files:**
- Create: `$SCRATCH/findings/security.json`, `correctness.json`, `architecture.json`, `resilience.json`

**Interfaces:**
- Consumes: `$SCRATCH/inventory.md`, `$SCRATCH/suspicious.md`, `$SCRATCH/baseline.md`, `$SCRATCH/runtime/vault.db`.
- Produces: JSON arrays of finding records with this exact shape (Task 8 parses it):

```json
[{
  "id": "SEC-01",
  "title": "short imperative title",
  "lens": "security|correctness|architecture|resilience",
  "severity": "critical|high|medium|low|info",
  "location": "internal/server/auth.go:42",
  "claim": "one sentence: what is wrong",
  "failure_scenario": "concrete steps/inputs → observable wrong outcome",
  "evidence": "code excerpt (<=15 lines) or command + output",
  "proposed_fix": "one paragraph",
  "confidence": "high|medium|low",
  "related_docs": ["docs/DEPLOYMENT.md#http-api"]
}]
```

- [ ] **Step 1: Write the shared review brief**

```bash
cat > "$SCRATCH/findings/BRIEF.md" <<'EOF'
You are performing an ADVERSARIAL REVIEW of one lens over the Go project at /home/stephen-perry/Development/vt-uos. You are READ-ONLY in the repo. You MAY write throwaway repro tests or scripts under $SCRATCH/repro/<lens>/ and run them with `export PATH=$HOME/sdk/go1.24.7/bin:$PATH`; never add files inside the repo.

INPUTS (read them first): $SCRATCH/inventory.md (as-built map, cited), $SCRATCH/suspicious.md (pre-flagged oddities — triage every one in your lens), $SCRATCH/baseline.md (build/vet/test/coverage/route-probe results). A seeded DB is at $SCRATCH/runtime/vault.db; the binary is at bin/vtuos.

THREAT FRAMING: assume an attacker with network access to the exhibit LAN, a Raspberry Pi that loses power at any instruction, a simulation that runs 25 simulated years, and a client terminal whose binary has been replaced by a hostile one.

RULES
- A finding needs: exact location (file:line you read), a concrete failure scenario (specific inputs/state → specific wrong outcome), and evidence (code excerpt or repro output). "Might be a problem" is not a finding — put those in a final `"id": "<LENS>-NOTES"` record with severity "info".
- Severity: critical = remote compromise / data loss / sim corruption with no operator action; high = same with a simple precondition, or a documented guarantee that is false; medium = incorrect behavior in realistic use; low = quality/maintainability with plausible future impact; info = observation.
- Prefer breadth over depth once you have >= 8 findings; stop at 25.
- Do NOT duplicate a finding already stated in baseline.md (test failures, vet output) — reference it instead in related evidence.
- Output: a single JSON array (schema given below) written to the path at the end. Valid JSON only, no prose outside it.
EOF
```

- [ ] **Step 2: Dispatch all four in ONE message (parallel with Task 7's six)**

Use `Agent` with `subagent_type: "general-purpose"` (they need Bash to run repro tests). Each prompt = `BRIEF.md` contents + the JSON schema block from Interfaces above + the lens block below + `Write to: $SCRATCH/findings/<lens>.json`.

```
LENS security (id prefix SEC)
Primary files: internal/server/{auth,api,sse,web,clients,server}.go, internal/server/web/index.html.tmpl, internal/server/web/static/app.js, internal/client/{transport,client}.go, internal/protocol/protocol.go, cmd/vtuos/main.go (serve/connect/--restore), internal/simulation/snapshot.go.
Must answer, each with a finding or an explicit "verified OK (file:line)" in the NOTES record:
1. Is the admin token compared with crypto/subtle.ConstantTimeCompare? Where does the token come from and is an empty token accepted?
2. List every route without requireAdmin and what state it can change or leak (baseline.md has probe results).
3. Can a client register with an arbitrary ID / impersonate another client / receive commands meant for another / post results for another?
4. Do resident names, item names, alert messages, or client-supplied telemetry reach the web console unescaped (html/template context, and app.js innerHTML/insertAdjacentHTML)?
5. Is there path traversal or overwrite in snapshot path, --restore, or any file operation influenced by config/HTTP input?
6. Are state-changing POST endpoints CSRF-able from a browser on the same LAN (no CSRF token, cookie auth, or bearer-only)?
7. Can the kiosk lockout be escaped (key bindings, signals, ctrl-c/ctrl-z handling)?
8. Any SQL built with fmt.Sprintf/string concat in repository/*.go? (grep for it.)
9. Request size limits, timeouts (ReadHeaderTimeout etc.), and slowloris/SSE exhaustion.
10. govulncheck results from baseline.md — reachable?
```
```
LENS correctness (id prefix COR)
Primary files: internal/simulation/*.go, internal/repository/*.go, internal/services/**, internal/database/{migrate,recovery,database}.go, internal/util/time.go.
Must answer:
1. Every multi-row write path: is it inside one transaction? List the ones that are not.
2. FIFO consumption: partial lot, exactly-zero remainder, insufficient stock, expired lots interleaved, negative/zero quantity request. Write a throwaway test in $SCRATCH/repro/correctness if you need to.
3. Tick idempotency: if the process dies after stage k of the tick pipeline, what happens on restart — replay, skip, double-apply? Identify the persistence point.
4. Determinism: find every math/rand, time.Now, map iteration feeding decisions; is the RNG seeded deterministically and single-owner?
5. Data races: shared state between engine goroutine, HTTP handlers, TUI, SSE fan-out. Run `go test -race` per package already in baseline; add a targeted race repro if you suspect one.
6. Migration mechanism: version table, ordering, partial-apply on crash, rollback claims in docs vs. reality.
7. Genetics/inbreeding coefficient: correct formula and search depth; cycles in lineage graph; self-parent; missing parents.
8. Time: vault clock vs. wall clock, day boundaries, leap handling, integer overflow of tick counters over 25 years.
9. Every vet message in baseline.md — real bug or noise?
```
```
LENS architecture (id prefix ARCH)
Primary files: all of internal/, with focus on internal/tui/app.go (2,037 lines), cmd/vtuos/main.go, internal/tui/simadapter.go.
Must answer:
1. Layering: does any tui/ or server/ file import internal/repository or internal/database directly? Does any service import tui? Cite each.
2. Global/package-level mutable state (var declarations holding state, sync.Once singletons, init() side effects).
3. Error handling: unwrapped errors (return err without fmt.Errorf %w) in services/repository; swallowed errors (`_ =`); panics in library code.
4. app.go: list its distinct responsibilities and propose a split boundary (this is advice, severity low, but include the concrete file list).
5. Dead code: exported symbols with no callers (grep-based), unused config keys, unused SQL tables/columns.
6. Test coverage reality: from baseline.md coverage table, list packages under 50% and the untested critical paths in them; state the true total vs. the 80% claim as a finding (severity: high, because it is a documented guarantee that is false — if it is).
7. Doc/code drift with architectural impact from $SCRATCH/drift.md (e.g., features claimed that do not exist).
8. Dependency hygiene: pinned versions vs. latest (baseline.md), replace directives, unused requires (`go mod tidy` diff — run it with -diff or in a copy, never modify go.mod).
```
```
LENS resilience (id prefix RES)
Primary files: internal/database/database.go (pragmas), internal/simulation/{persistence,snapshot,engine,events}.go, internal/server/{sse,clients}.go, internal/client/*.go, internal/config/config.go.
Must answer:
1. PRAGMAs: journal_mode, synchronous, busy_timeout, foreign_keys, cache_size, mmap — values and whether they honor "power-loss resilient" on SD-card storage.
2. Unbounded growth: simulation_events, audit_log, alerts, client telemetry — any retention, pruning, or index to keep 25 years queryable? Estimate rows/day from the tick code and project size at 25 years.
3. SSE: per-client buffering, slow-consumer policy (drop/block/disconnect), goroutine and channel cleanup on disconnect, max clients.
4. Goroutine leaks: every `go func` — how does it exit on shutdown/context cancel? Client reconnect loop backoff and jitter.
5. Disk full / SQLITE_BUSY / SQLITE_LOCKED handling: what does the engine do when a tick write fails — retry, halt, silently skip?
6. Memory: rough working-set estimate for 1000 residents (VaultState size × subscribers, in-memory caches, seed generator). Anything O(residents²) per tick (genetics scans)?
7. Startup with a corrupt or zero-length DB: recovery.go behavior; can it destroy a good DB?
8. Config validation: what invalid values (0 tick interval, negative scale, empty addr) are accepted and what happens.
```

- [ ] **Step 3: Validate the JSON when they return**

```bash
cd "$SCRATCH/findings" && for l in security correctness architecture resilience; do python3 -c "import json,sys; d=json.load(open('$l.json')); print('$l', len(d), 'findings;', sum(1 for f in d if f['severity'] in ('critical','high')), 'crit/high')" || echo "INVALID JSON: $l.json"; done
```
Fix invalid JSON by hand (usually trailing prose) rather than re-dispatching.

No commit.

---

### Task 7: Docs round — dispatch six cluster agents

**Files:**
- Create: `$SCRATCH/docs/{README,CLAUDE,ROADMAP,ARCHITECTURE,DEVELOPMENT,TESTING,CLI,CONFIGURATION,DATABASE,MODULES,SIMULATION,API,DEPLOYMENT,TUI}.md`
- Create: `$SCRATCH/citations/<same names>.tsv`

**Interfaces:**
- Consumes: `$SCRATCH/inventory.md`, `$SCRATCH/drift.md`, `$SCRATCH/baseline.md`, current docs.
- Produces: one draft per target doc + one citations sidecar per doc (`file<TAB>line<TAB>claim`). Task 10 audits the sidecars; Task 11 moves the drafts into the repo.

- [ ] **Step 1: Write the shared docs brief**

```bash
cat > "$SCRATCH/docs/BRIEF.md" <<'EOF'
You are rewriting documentation for the Go project at /home/stephen-perry/Development/vt-uos so that it describes the code AS BUILT. Do not modify anything inside the repo; write drafts only to the paths given at the end.

INPUTS: $SCRATCH/inventory.md (cited as-built map — trust it but verify anything you quote by opening the file), $SCRATCH/drift.md (known doc errors — every one that falls in your docs must be fixed), $SCRATCH/baseline.md (real build/test/coverage/route numbers — quote these, never the old numbers), and the current version of each doc you own.

HARD RULES
1. No sentence about behavior unless you read the code that implements it. For each such sentence append a row to your citations sidecar: `<path/file.go>\t<line>\t<first 60 chars of the sentence>`. The sidecar is audited mechanically; a doc with unverifiable claims is rejected.
2. If the current doc claims something the code does not do: delete it, or (if it is clearly design intent for phases 6–7) move it verbatim under a heading in $SCRATCH/docs/ROADMAP-fragments-<yourcluster>.md so the ROADMAP author can collect it. Never leave it in place.
3. In the published text, cite at file granularity in backticks, e.g. "(`internal/server/auth.go`)" or "(`internal/simulation/tick.go`, `runTick`)". Do not put line numbers in the published doc.
4. Describe bugs as they are. Do not editorialize or propose fixes; the review round handles that. If behavior looks wrong, describe it neutrally and add `<!-- REVIEW: possible issue -->` after the sentence so the orchestrator can attach an issue link later.
5. Every shell command you show must be one you ran (or that appears verbatim in baseline.md). Every TOML key, flag, route, key binding, table, and column must appear in inventory.md §1 for its slice.
6. Keep the project voice: terse, technical, Fallout-flavored headings are fine, no marketing.
7. Match existing formatting: GitHub-flavored Markdown, tables for reference data, fenced code with language tags, `##` top-level sections under a single `#` title.
EOF
```

- [ ] **Step 2: Dispatch all six in the SAME message as Task 6's four**

`Agent` with `subagent_type: "general-purpose"`. Each prompt = `BRIEF.md` + the cluster block + explicit output paths.

```
CLUSTER a — README.md, CLAUDE.md, ROADMAP.md (and PROGRESS.md removal)
Outputs: $SCRATCH/docs/README.md, $SCRATCH/docs/CLAUDE.md, $SCRATCH/docs/ROADMAP.md, sidecars for each.
README: keep the existing tone and emoji section style; fix Features so Population/Resources/Facilities/Simulation/Server+Client are "implemented" and labor/medical/security/governance are "planned"; Quick Start commands must match `bin/vtuos --help` output in baseline.md; Documentation section lists exactly: ARCHITECTURE, CLI, CONFIGURATION, DATABASE, MODULES, SIMULATION, API, DEPLOYMENT, TUI, DEVELOPMENT, TESTING, ROADMAP; Roadmap section: phases 1–5 done, 6–7 pending, link ROADMAP.md; delete any "Current Phase"/"In Progress" labels that are stale.
CLAUDE.md: edit surgically — Project Structure tree must match `find . -type d -not -path './.git*'` (add client/, server/, protocol/, testutil/, database/seed, database/migrations; remove services/{labor,medical,security,governance} and simulation subdirs that do not exist); Quick Reference table lists the new doc set; Implementation Phases checkboxes correct; keep AI Assistant Guidelines unchanged.
ROADMAP.md: new. Sections: "Status" (what is built, one line per module), "Phase 6 — Labor / Medical / Security / Governance" and "Phase 7 — Polish". Populate from: the Labor/Medical/Security/Governance sections of current docs/MODULES.md, the Medical/Security/Governance table specs in current docs/DATABASE.md, unbuilt components in current docs/TUI.md (Modal/Tabs/Tree if absent per inventory), and any $SCRATCH/docs/ROADMAP-fragments-*.md that other clusters produce (check for them at the end; if none yet, leave a `<!-- ROADMAP-fragments merge point -->` marker). Every item is prefixed "**Planned:**" and the top of the file states in bold that nothing in it is implemented.
PROGRESS.md: do not write a replacement; confirm in your final message that everything factual in it is covered by README+ROADMAP.
```
```
CLUSTER b — ARCHITECTURE.md (new), DEVELOPMENT.md, TESTING.md (new)
Outputs: $SCRATCH/docs/ARCHITECTURE.md, DEVELOPMENT.md, TESTING.md + sidecars.
ARCHITECTURE: layers as actually wired (who constructs what in cmd/vtuos/main.go for each of the three modes); process model — goroutines started in local, serve, and connect modes and how they communicate (channels, callbacks, HTTP/SSE); an ASCII data-flow diagram from DB → engine → protocol.VaultState → {TUI, web console, clients}; the VaultState struct fields (from internal/protocol/protocol.go) and update cadence; where layering rules are violated today (state neutrally, from inventory §3).
DEVELOPMENT: keep Branch Strategy/Commit Messages/Code Style; replace "Example Makefile" with the real target list from Makefile (name + one-line purpose); "Implementation Order" section becomes a 6-line summary pointing to ROADMAP.md; Testing Requirements quotes the real total coverage from baseline.md and links TESTING.md; remove any tool (golangci-lint, goimports) claims that are not verified — say "Makefile expects X; not required for build".
TESTING: what exists — per-package table (package, test files, approach: unit/table-driven/teatest e2e/golden, coverage % from baseline.md); the testutil and database/testhelper helpers and how to use them (signatures); how to run (exact commands from Makefile); known gaps (packages with no test files) stated as facts.
```
```
CLUSTER c — CLI.md (new), CONFIGURATION.md
Outputs: $SCRATCH/docs/CLI.md, CONFIGURATION.md + sidecars.
CLI: one section per invocation form — `vtuos [flags]`, `vtuos serve [flags]`, `vtuos connect <addr> [flags]` — with a table per flag (flag, type, default, effect, file:func that reads it); exit codes as returned by localMain/serveMain/connectMain; environment variables read anywhere (grep Getenv); startup sequence (config → db open → migrate → seed? → mode) as numbered steps; signals handled.
CONFIGURATION: every TOML key from internal/config/config.go struct tags, grouped as the struct groups them, table columns: key, type, default (from the defaults function), validation rule (from the validate function or "none"), effect. Config file search order and env overrides from loader.go. Delete the current sections that describe things with no code behind them (verify each: migration status/force/rollback CLI, performance tuning knobs, security considerations) — move anything that is clearly a plan to ROADMAP-fragments-c.md. Keep a short Glossary only if terms are used in the code/UI.
```
```
CLUSTER d — DATABASE.md
Outputs: $SCRATCH/docs/DATABASE.md + sidecar.
Rewrite from internal/database/migrations/001_initial.sql and 002_performance_hardening.sql ONLY. For each table: purpose (one line), full column table (name, type, constraints, default), indexes, triggers, foreign keys. Then: PRAGMAs actually set in database.go with values and rationale where a comment gives one; migration mechanism (tracking table, how versions are applied, what happens on partial failure — from migrate.go); recovery behavior (recovery.go); snapshot mechanism (VACUUM INTO, from simulation/snapshot.go); data directory/default path. Tables in the current DATABASE.md that are not in the migrations (medical, security, governance, quarters, vocations…) go to ROADMAP-fragments-d.md, not here. Include the seed data shape (counts per table from seed/generator.go).
```
```
CLUSTER e — MODULES.md, SIMULATION.md
Outputs: $SCRATCH/docs/MODULES.md, SIMULATION.md + sidecars.
MODULES: one H2 per implemented module (Population, Resources, Facilities) and a short H2 "Simulation" that links SIMULATION.md. Per module: responsibilities; service constructor and every exported method with signature, what it does, what it writes, and errors it returns (from inventory §1–2); implemented algorithms written out (inbreeding coefficient, demographics stats, FIFO consumption, runway forecast, ration allocation, facility efficiency/degradation/maintenance/alerts); repository methods list; TUI views that call it. Labor/Medical/Security/Governance sections → ROADMAP-fragments-e.md.
SIMULATION: expand the current doc. Tick pipeline as an ordered list with the function per stage; time model (tick size, scale, how sim time maps to wall time); RNG and determinism as implemented; consumption/production/expiration rules; degradation and maintenance sweep; incident table with weights and effects; demographics (birth/death rules, rates); alert rules and thresholds; persistence per tick and continuity on restart; snapshot & --restore; control API (pause/resume/scale/step/snapshot/ack) as exposed to TUI and HTTP; VaultState publication.
```
```
CLUSTER f — API.md (new), DEPLOYMENT.md, TUI.md
Outputs: $SCRATCH/docs/API.md, DEPLOYMENT.md, TUI.md + sidecars.
API: authentication (token source, header/query form, which routes require it — from auth.go and the route table in server.go; cross-check with the probe results in baseline.md); one H3 per route: method, path, auth, request body JSON (field, type, required), response JSON (field, type), status codes, side effects; SSE stream: event names, payload shape, heartbeat/retry; client protocol lifecycle: register → heartbeat → command → result with the JSON of each (from protocol.go); health endpoints; static/web console route.
DEPLOYMENT: verify every existing sentence against code and baseline; keep what is true; add: exact serve/connect flags (link CLI.md), token provisioning, kiosk mode flags and what lockout actually does, reset procedure using real flags, what "exhibit topology" means in code terms. Remove claims about systemd/units/TLS/etc. unless code or Makefile provides them.
TUI: rewrite. Views that exist (Dashboard, Census, Resident form, Inventory, Systems, Simulation Control, Help) — one H3 each with purpose, key bindings, and the service methods it calls; global key bindings table from keys.go and app.go; responsive breakpoints and what changes at each (layout.go); components that exist (table, input from components/) with their public API; style tokens from styles.go. Components described in the current TUI.md that do not exist in components/ (Modal, Tabs, Tree, Pagination, AlertBanner… verify each) → ROADMAP-fragments-f.md. Keep the "Example Screens" only if you regenerate them from the running binary or drop them.
```

- [ ] **Step 3: When all six return, confirm outputs**

```bash
cd "$SCRATCH" && ls -la docs/*.md citations/*.tsv && for f in docs/*.md; do n=$(basename "$f" .md); [ -s "citations/$n.tsv" ] || echo "NO SIDECAR for $n"; done
```
Expected: 14 doc drafts (plus `ROADMAP-fragments-*.md`), 14 non-empty sidecars.

No commit.

---

### Task 8: Verify findings (the gate) and deduplicate

**Files:**
- Create: `$SCRATCH/findings/verified.json`, `$SCRATCH/findings/verification-log.md`

**Interfaces:**
- Consumes: the four lens JSON files.
- Produces: `verified.json` — same schema plus `"verdict": "CONFIRMED|DOWNGRADED|REJECTED|UNVERIFIED"`, `"verified_by": "orchestrator"`, `"repro": "<what was run>"`. Task 9 creates issues only from CONFIRMED and UNVERIFIED (the latter labelled as such).

- [ ] **Step 1: Concatenate and list medium+ for review**

```bash
cd "$SCRATCH/findings" && python3 - <<'EOF'
import json
allf=[]
for l in ["security","correctness","architecture","resilience"]:
    allf+=json.load(open(f"{l}.json"))
json.dump(allf,open("all.json","w"),indent=2)
order={"critical":0,"high":1,"medium":2,"low":3,"info":4}
for f in sorted(allf,key=lambda f:order[f["severity"]]):
    print(f'{f["id"]:8} {f["severity"]:8} {f["location"]:45} {f["title"]}')
print(len(allf),"total")
EOF
```

- [ ] **Step 2: For every critical/high/medium finding, re-read the cited code yourself**

For each, run and record in `verification-log.md`:
```bash
# template — substitute location
f=internal/server/auth.go; l=42; sed -n "$((l-15)),$((l+15))p" $f
```
Then decide: does the code do what the claim says? Is the failure scenario reachable? Write one line per finding: `ID | verdict | reason`.

- [ ] **Step 3: Attempt a repro for each critical/high**

Security: re-run the relevant curl against a fresh `bin/vtuos serve` on `127.0.0.1:18080` (same as Task 3 Step 3) and paste the response. Correctness: write a throwaway `_test.go` under `$SCRATCH/repro/verify/` that imports the package (`go test` there with `-run` naming it; use `GOFLAGS=-mod=mod` and a `go.mod` `replace github.com/vtuos/vtuos => /home/stephen-perry/Development/vt-uos`), or an `sqlite3`/Go script against `$SCRATCH/runtime/vault.db`. Resilience: `grep -n PRAGMA`, count `go func`, inspect channel buffer sizes. Architecture: `grep -rn '"github.com/vtuos/vtuos/internal/repository"' internal/tui internal/server` etc.

A finding whose repro fails is REJECTED (with the reason) or DOWNGRADED to the severity the evidence supports. A finding that cannot be repro'd in reasonable time but whose code reading holds is UNVERIFIED — kept, labelled.

- [ ] **Step 4: Deduplicate across lenses**

Same location + same claim → keep the one with better evidence, add `"merged_from": ["ID",...]`. Two findings that are symptoms of one root cause (e.g., "no transaction in X" and "no transaction in Y") stay separate issues but cross-link in `related`.

- [ ] **Step 5: Write verified.json**

```bash
cd "$SCRATCH/findings" && python3 -c "
import json; d=json.load(open('verified.json'))
from collections import Counter
print(Counter(f['verdict'] for f in d)); print(Counter((f['severity'],f['verdict']) for f in d if f['verdict']!='REJECTED'))"
```
Expected: every record has a `verdict`; no CONFIRMED record lacks a `repro` string.

No commit.

---

### Task 9: Gate 1 — user confirms, then create labels and issues

**Files:**
- Create: `$SCRATCH/issues.md`, `$SCRATCH/issue-map.tsv`

**Interfaces:**
- Consumes: `verified.json`.
- Produces: GitHub issues; `issue-map.tsv` (`finding_id<TAB>issue_number`) consumed by Task 11 for doc callouts; tracking issue number.

- [ ] **Step 1: Render the preview**

```bash
cd "$SCRATCH/findings" && python3 - <<'EOF' > ../issues.md
import json
d=[f for f in json.load(open("verified.json")) if f["verdict"]!="REJECTED"]
order={"critical":0,"high":1,"medium":2,"low":3,"info":4}
pref={"security":"SEC","correctness":"COR","architecture":"ARCH","resilience":"RES"}
print("# Issues to be created\n")
print("## Individual issues (severity >= medium)\n")
for f in sorted([x for x in d if order[x["severity"]]<=2],key=lambda f:order[f["severity"]]):
    tag=" (unverified)" if f["verdict"]=="UNVERIFIED" else ""
    print(f'- **[{pref[f["lens"]]}] {f["title"]}**{tag} — `{f["severity"]}` — `{f["location"]}`')
print("\n## Batched 'Minor findings' issues (one per lens, low/info)\n")
for lens in pref:
    minor=[x for x in d if x["lens"]==lens and order[x["severity"]]>=3]
    if minor: print(f'- [{pref[lens]}] Minor findings — {len(minor)} items')
print("\n## Tracking issue\n\n- Adversarial review 2026-09-20 — index of all of the above + baseline numbers")
print("\n## Labels to create\n\nseverity:critical severity:high severity:medium severity:low lens:security lens:correctness lens:architecture lens:resilience adversarial-review unverified")
EOF
cat "$SCRATCH/issues.md"
```

- [ ] **Step 2: STOP. Show `issues.md` to the user and ask for explicit confirmation.**

Use `AskUserQuestion`: "Create these N issues + M batched + 1 tracking issue and 10 labels on jstephenperry/vt-uos?" Options: Yes, create all / Let me edit the list first / No. Do not proceed on anything but an explicit yes. If they edit, regenerate the preview and ask again.

- [ ] **Step 3: Create labels (idempotent)**

```bash
cd /home/stephen-perry/Development/vt-uos
mk(){ gh label create "$1" --color "$2" --description "$3" --force >/dev/null && echo "label $1"; }
mk severity:critical B60205 "Remote compromise, data loss, or sim corruption without operator action"
mk severity:high D93F0B "Serious defect or a documented guarantee that is false"
mk severity:medium FBCA04 "Incorrect behavior in realistic use"
mk severity:low 0E8A16 "Quality/maintainability with plausible future impact"
mk lens:security 5319E7 "Adversarial review: security lens"
mk lens:correctness 1D76DB "Adversarial review: correctness & data integrity lens"
mk lens:architecture 0052CC "Adversarial review: architecture & maintainability lens"
mk lens:resilience 006B75 "Adversarial review: resource & resilience lens"
mk adversarial-review C5DEF5 "Filed by the 2026-09-20 adversarial review"
mk unverified E4E669 "Finding held on code reading; repro not achieved"
```

- [ ] **Step 4: Create individual issues and record the map**

```bash
cd /home/stephen-perry/Development/vt-uos && : > "$SCRATCH/issue-map.tsv"
python3 - <<'EOF' | while IFS=$'\t' read id title labels bodyfile; do n=$(gh issue create --title "$title" --label "$labels" --body-file "$bodyfile" | grep -oE '[0-9]+$'); printf '%s\t%s\n' "$id" "$n" >> "$SCRATCH/issue-map.tsv"; echo "#$n $title"; done
import json,os
S=os.environ["SCRATCH"]; os.makedirs(f"{S}/issue-bodies",exist_ok=True)
d=[f for f in json.load(open(f"{S}/findings/verified.json")) if f["verdict"] in ("CONFIRMED","UNVERIFIED")]
order={"critical":0,"high":1,"medium":2}
pref={"security":"SEC","correctness":"COR","architecture":"ARCH","resilience":"RES"}
for f in sorted([x for x in d if x["severity"] in order],key=lambda f:order[f["severity"]]):
    body=f"""**Severity:** {f['severity']}  **Lens:** {f['lens']}  **Verdict:** {f['verdict']}  **Finding ID:** {f['id']}
**Location:** `{f['location']}`

## Claim
{f['claim']}

## Failure scenario
{f['failure_scenario']}

## Evidence
```
{f['evidence']}
```

## Verification
{f.get('repro','(code reading only)')}

## Proposed fix
{f['proposed_fix']}

## Related docs
{chr(10).join('- '+x for x in f.get('related_docs',[])) or '- (none)'}

---
Filed by the 2026-09-20 adversarial review (findings only; no fix included). Spec: `docs/superpowers/specs/2026-09-20-analysis-review-docs-design.md`.
"""
    p=f"{S}/issue-bodies/{f['id']}.md"; open(p,"w").write(body)
    labels=",".join([f"severity:{f['severity']}",f"lens:{f['lens']}","adversarial-review"]+(["unverified"] if f["verdict"]=="UNVERIFIED" else []))
    print(f"{f['id']}\t[{pref[f['lens']]}] {f['title']}\t{labels}\t{p}")
EOF
wc -l "$SCRATCH/issue-map.tsv"
```
Expected: one line per medium+ finding.

- [ ] **Step 5: Create the four batched minor-findings issues**

```bash
cd /home/stephen-perry/Development/vt-uos
python3 - <<'EOF' | while IFS=$'\t' read lens title labels bodyfile; do n=$(gh issue create --title "$title" --label "$labels" --body-file "$bodyfile" | grep -oE '[0-9]+$'); printf '%s-MINOR\t%s\n' "$lens" "$n" >> "$SCRATCH/issue-map.tsv"; echo "#$n $title"; done
import json,os
S=os.environ["SCRATCH"]
d=[f for f in json.load(open(f"{S}/findings/verified.json")) if f["verdict"]!="REJECTED" and f["severity"] in ("low","info")]
pref={"security":"SEC","correctness":"COR","architecture":"ARCH","resilience":"RES"}
for lens in pref:
    items=[f for f in d if f["lens"]==lens]
    if not items: continue
    body="Low/info observations from the 2026-09-20 adversarial review, batched to keep the tracker readable.\n\n"
    for f in items:
        body+=f"### {f['id']} — {f['title']} (`{f['severity']}`)\n`{f['location']}`\n\n{f['claim']}\n\n*Scenario:* {f['failure_scenario']}\n\n*Suggested:* {f['proposed_fix']}\n\n"
    p=f"{S}/issue-bodies/{lens}-minor.md"; open(p,"w").write(body)
    print(f"{lens}\t[{pref[lens]}] Minor findings ({len(items)})\tseverity:low,lens:{lens},adversarial-review\t{p}")
EOF
```

- [ ] **Step 6: Create the tracking issue**

```bash
cd /home/stephen-perry/Development/vt-uos
python3 - <<'EOF' > "$SCRATCH/issue-bodies/tracking.md"
import json,os,collections
S=os.environ["SCRATCH"]
m=dict(l.rstrip("\n").split("\t") for l in open(f"{S}/issue-map.tsv"))
d=[f for f in json.load(open(f"{S}/findings/verified.json")) if f["verdict"]!="REJECTED"]
c=collections.Counter(f["severity"] for f in d)
print("# Adversarial review — 2026-09-20\n")
print("Findings-only review across four lenses (security, correctness & integrity, architecture, resource/resilience). Every issue below was re-verified by the orchestrator; `unverified` marks findings held on code reading without a repro.\n")
print("| Severity | Count |\n|---|---|"); [print(f"| {s} | {c.get(s,0)} |") for s in ("critical","high","medium","low","info")]
print("\n## Baseline at commit\n\n```"); print(open(f"{S}/baseline.md").read().split("## Coverage")[1].split("## Dependencies")[0].strip()); print("```")
print("\n## Issues\n")
for lens in ("security","correctness","architecture","resilience"):
    print(f"### {lens}\n")
    for f in sorted([x for x in d if x["lens"]==lens and x["severity"] in ("critical","high","medium")],key=lambda f:("critical","high","medium").index(f["severity"])):
        print(f"- #{m[f['id']]} `{f['severity']}` {f['title']}")
    k=f"{lens}-MINOR"
    if k in m: print(f"- #{m[k]} minor findings")
    print()
print("## Documentation\n\nAs-built docs rewrite PR: _(link added when opened)_\n\nSpec: `docs/superpowers/specs/2026-09-20-analysis-review-docs-design.md`")
EOF
n=$(gh issue create --title "Adversarial review 2026-09-20 — tracking" --label "adversarial-review" --body-file "$SCRATCH/issue-bodies/tracking.md" | grep -oE '[0-9]+$'); printf 'TRACKING\t%s\n' "$n" >> "$SCRATCH/issue-map.tsv"; echo "tracking issue #$n"
```

- [ ] **Step 7: Verify on GitHub**

```bash
gh issue list --label adversarial-review --limit 100 --json number,title,labels -q '.[] | "\(.number)\t\(.title)"' | sort -n | tee -a "$SCRATCH/issues-created.txt" | wc -l
```
Expected: count equals lines in `issue-map.tsv`.

No repo commit.

---

### Task 10: Verify the doc drafts — citation audit, command execution, cross-links

**Files:**
- Create: `$SCRATCH/docs-audit.md`

**Interfaces:**
- Consumes: `$SCRATCH/docs/*.md`, `$SCRATCH/citations/*.tsv`.
- Produces: an audit log with zero blocking errors; drafts corrected in place.

- [ ] **Step 1: Mechanical sidecar audit**

```bash
cd /home/stephen-perry/Development/vt-uos && : > "$SCRATCH/docs-audit.md"
for t in "$SCRATCH"/citations/*.tsv; do
  n=$(basename "$t" .tsv); err=0
  while IFS=$'\t' read f l claim; do
    [ -z "$f" ] && continue
    if [ ! -f "$f" ]; then echo "$n: MISSING $f ($claim)"; err=$((err+1));
    elif ! [[ "$l" =~ ^[0-9]+$ ]] || [ "$l" -gt "$(wc -l < "$f")" ]; then echo "$n: BAD LINE $f:$l ($claim)"; err=$((err+1)); fi
  done < "$t"
  echo "$n: $(wc -l < "$t") citations, $err errors"
done | tee -a "$SCRATCH/docs-audit.md"
```
Expected: 0 errors per doc. Fix by opening the code and correcting the draft or sidecar.

- [ ] **Step 2: Backticked identifiers exist**

```bash
cd /home/stephen-perry/Development/vt-uos
for d in "$SCRATCH"/docs/*.md; do
  n=$(basename "$d")
  grep -oE '`(cmd|internal|docs|testdata)/[A-Za-z0-9_./-]+`' "$d" | tr -d '`' | sort -u | while read p; do [ -e "$p" ] || echo "$n: path does not exist: $p"; done
  grep -oE '`[A-Z][A-Za-z0-9]+(\.[A-Za-z0-9]+)?`' "$d" | tr -d '`' | sort -u | while read sym; do base=${sym##*.}; grep -rqE "func (\([^)]*\) )?$base\b|type $base\b|^\s*$base\s" --include='*.go' internal cmd || echo "$n: symbol not found in Go source: $sym"; done
done | tee -a "$SCRATCH/docs-audit.md" | head -80
```
Review each hit: acronyms and prose words in backticks are fine; a documented function that does not exist is a blocker.

- [ ] **Step 3: Run every documented command**

```bash
cd /home/stephen-perry/Development/vt-uos && export PATH=$HOME/sdk/go1.24.7/bin:$PATH
grep -hoE '^\$ .*|^make [a-z-]+.*|^\./bin/vtuos.*' "$SCRATCH"/docs/{README,CLI,CONFIGURATION,DEPLOYMENT,DEVELOPMENT,TESTING}.md | sed 's/^\$ //' | sort -u | tee "$SCRATCH/doc-commands.txt"
```
Run each one that is safe (build/test/help/list forms; skip `rm`, `serve` foreground, `connect`) and note `OK` / `FAIL: <output>` in `docs-audit.md`. A failing documented command is a blocker: fix the doc.

- [ ] **Step 4: Cross-link audit across the new doc set**

```bash
cd "$SCRATCH/docs" && for d in *.md; do grep -oE '\]\((docs/)?[A-Za-z_.-]+\.md(#[a-z0-9-]+)?\)' "$d" | tr -d '()]' | sed 's#^docs/##; s/#.*//' | sort -u | while read tgt; do [ -f "$tgt" ] || [ -f "../docs/$tgt" ] || echo "$d -> $tgt (missing)"; done; done | tee -a "$SCRATCH/docs-audit.md"
```
Expected: no missing targets. (README/CLAUDE link as `docs/X.md`; docs link each other as `X.md`.)

- [ ] **Step 5: Merge ROADMAP fragments and confirm nothing aspirational remains elsewhere**

```bash
cd "$SCRATCH/docs" && ls ROADMAP-fragments-*.md 2>/dev/null
grep -nE 'TODO|TBD|coming soon|will be|planned|not yet implemented|in progress' README.md ARCHITECTURE.md CLI.md CONFIGURATION.md DATABASE.md MODULES.md SIMULATION.md API.md DEPLOYMENT.md TUI.md DEVELOPMENT.md TESTING.md | tee -a "$SCRATCH/docs-audit.md"
```
Append each fragment file's content under the matching phase heading in `ROADMAP.md`, remove the `<!-- ROADMAP-fragments merge point -->` marker, then review each grep hit: allowed only where it links to ROADMAP.md.

- [ ] **Step 6: Read every draft end to end**

No script. Read each file. Check tone, remove agent artefacts ("As requested…", "I verified…"), ensure tables render, and that each doc has exactly one `#` title. Record "read: <name> OK" in `docs-audit.md`.

No commit.

---

### Task 11: Attach issue callouts and land the docs on a branch

**Files:**
- Modify (repo): `README.md`, `CLAUDE.md`, all `docs/*.md` listed in File Structure; delete `PROGRESS.md`
- Create (repo): `docs/ARCHITECTURE.md`, `docs/CLI.md`, `docs/API.md`, `docs/ROADMAP.md`, `docs/TESTING.md`

**Interfaces:**
- Consumes: audited drafts, `issue-map.tsv`, `verified.json` (`related_docs`).
- Produces: branch `docs/as-built-rewrite` with one commit per cluster.

- [ ] **Step 1: Replace `<!-- REVIEW: possible issue -->` markers with issue callouts**

```bash
cd "$SCRATCH" && grep -n 'REVIEW: possible issue' docs/*.md
```
For each marker: find the matching finding in `verified.json` by location/topic, look up its issue number in `issue-map.tsv`, and replace the marker with `> ⚠ See [#N](https://github.com/jstephenperry/vt-uos/issues/N).` on its own line after the paragraph. If no finding matches, delete the marker (the review didn't confirm it). Also, for each CONFIRMED finding whose `related_docs` names a doc/section, add the same callout there if not already present.

```bash
grep -c 'REVIEW: possible issue' "$SCRATCH"/docs/*.md | grep -v ':0$' ; echo "(should print nothing)"
```

- [ ] **Step 2: Create the branch**

```bash
cd /home/stephen-perry/Development/vt-uos && git status --porcelain && git checkout -b docs/as-built-rewrite
```
Expected: clean tree before branching.

- [ ] **Step 3: Cluster (a) — README, CLAUDE, ROADMAP, PROGRESS removal**

```bash
cd /home/stephen-perry/Development/vt-uos
cp "$SCRATCH/docs/README.md" README.md && cp "$SCRATCH/docs/CLAUDE.md" CLAUDE.md && cp "$SCRATCH/docs/ROADMAP.md" docs/ROADMAP.md && git rm -q PROGRESS.md
git add README.md CLAUDE.md docs/ROADMAP.md && git commit -q -m "docs: rewrite README and CLAUDE.md as-built, add ROADMAP, remove stale PROGRESS

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" && git log --oneline -1
```

- [ ] **Step 4: Cluster (b) — ARCHITECTURE, DEVELOPMENT, TESTING**

```bash
cd /home/stephen-perry/Development/vt-uos
cp "$SCRATCH/docs/ARCHITECTURE.md" docs/ && cp "$SCRATCH/docs/DEVELOPMENT.md" docs/ && cp "$SCRATCH/docs/TESTING.md" docs/
git add docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md && git commit -q -m "docs: add ARCHITECTURE and TESTING, align DEVELOPMENT with real Makefile and coverage

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" && git log --oneline -1
```

- [ ] **Step 5: Cluster (c) — CLI, CONFIGURATION**

```bash
cd /home/stephen-perry/Development/vt-uos
cp "$SCRATCH/docs/CLI.md" docs/ && cp "$SCRATCH/docs/CONFIGURATION.md" docs/
git add docs/CLI.md docs/CONFIGURATION.md && git commit -q -m "docs: add CLI reference, rewrite CONFIGURATION from config.go

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" && git log --oneline -1
```

- [ ] **Step 6: Cluster (d) — DATABASE**

```bash
cd /home/stephen-perry/Development/vt-uos
cp "$SCRATCH/docs/DATABASE.md" docs/ && git add docs/DATABASE.md && git commit -q -m "docs: rewrite DATABASE from the actual migrations

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" && git log --oneline -1
```

- [ ] **Step 7: Cluster (e) — MODULES, SIMULATION**

```bash
cd /home/stephen-perry/Development/vt-uos
cp "$SCRATCH/docs/MODULES.md" docs/ && cp "$SCRATCH/docs/SIMULATION.md" docs/
git add docs/MODULES.md docs/SIMULATION.md && git commit -q -m "docs: rewrite MODULES as-built, expand SIMULATION tick pipeline

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" && git log --oneline -1
```

- [ ] **Step 8: Cluster (f) — API, DEPLOYMENT, TUI**

```bash
cd /home/stephen-perry/Development/vt-uos
cp "$SCRATCH/docs/API.md" docs/ && cp "$SCRATCH/docs/DEPLOYMENT.md" docs/ && cp "$SCRATCH/docs/TUI.md" docs/
git add docs/API.md docs/DEPLOYMENT.md docs/TUI.md && git commit -q -m "docs: add API reference, verify DEPLOYMENT, rewrite TUI from keys.go and views

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>" && git log --oneline -1
```

- [ ] **Step 9: Final in-repo checks**

```bash
cd /home/stephen-perry/Development/vt-uos
git diff --stat main..HEAD
# CLAUDE.md Quick Reference must name every doc that exists and nothing else
for f in docs/*.md; do grep -q "$(basename $f)" CLAUDE.md README.md || echo "not indexed: $f"; done
# links from repo root resolve
grep -ohE '\]\(docs/[A-Za-z_.-]+\.md' README.md CLAUDE.md | tr -d '](' | sort -u | while read p; do [ -f "$p" ] || echo "broken: $p"; done
ls PROGRESS.md 2>&1 | grep -q 'No such' && echo "PROGRESS.md removed"
```
Expected: no `not indexed`, no `broken`, `PROGRESS.md removed`. (`docs/superpowers/*` is exempt from the index check — ignore those lines.)

---

### Task 12: Gate 2 — user confirms, then push, open PR, link from tracking issue

**Files:**
- None in repo beyond Task 11 commits.

**Interfaces:**
- Consumes: branch `docs/as-built-rewrite`, `issue-map.tsv` (TRACKING number).
- Produces: pushed branch, PR URL, tracking issue updated.

- [ ] **Step 1: Prepare the PR body**

```bash
cd /home/stephen-perry/Development/vt-uos && T=$(awk -F'\t' '$1=="TRACKING"{print $2}' "$SCRATCH/issue-map.tsv")
{ cat <<EOF
## Summary

Rewrites the documentation to describe VT-UOS **as built**, verified against source at $(git rev-parse --short main). Design intent for unbuilt phases moves to \`docs/ROADMAP.md\`. Companion adversarial review is tracked in #$T; docs carry \`⚠ See #N\` callouts where documented behavior is affected by a confirmed finding.

## Per-file changes

| File | Change |
|---|---|
EOF
git diff --stat main..HEAD --name-status | awk '{printf "| `%s` | %s |\n", $2, ($1=="A"?"new":($1=="D"?"deleted":"rewritten/edited"))}'
cat <<EOF

## Baseline used for all numbers

\`\`\`
$(sed -n '/^## Coverage/,/^## Dependencies/p' "$SCRATCH/baseline.md" | sed '$d')
\`\`\`

## Verification

- Every behavioral sentence has a \`file:line\` citation in an audit sidecar; sidecars audited mechanically (0 missing files / out-of-range lines).
- Every documented shell command was executed against the built binary.
- Cross-links and the CLAUDE.md / README doc index verified.
- No Go source changed in this PR.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
} > "$SCRATCH/pr-body.md"; cat "$SCRATCH/pr-body.md"
```

- [ ] **Step 2: STOP. Show `git diff --stat main..HEAD` and `pr-body.md` to the user; ask for explicit confirmation to push and open the PR.**

Use `AskUserQuestion`: "Push `docs/as-built-rewrite` and open the PR against `main`?" Options: Yes / Let me review the branch locally first / No. Proceed only on yes.

- [ ] **Step 3: Push and open the PR**

```bash
cd /home/stephen-perry/Development/vt-uos
git push -u origin docs/as-built-rewrite
gh pr create --base main --head docs/as-built-rewrite --title "docs: as-built documentation rewrite" --body-file "$SCRATCH/pr-body.md" | tee "$SCRATCH/pr-url.txt"
```

- [ ] **Step 4: Link the PR from the tracking issue**

```bash
cd /home/stephen-perry/Development/vt-uos && T=$(awk -F'\t' '$1=="TRACKING"{print $2}' "$SCRATCH/issue-map.tsv") && URL=$(cat "$SCRATCH/pr-url.txt")
gh issue view "$T" --json body -q .body | sed "s#_(link added when opened)_#$URL#" > "$SCRATCH/tracking-updated.md" && gh issue edit "$T" --body-file "$SCRATCH/tracking-updated.md" && gh issue view "$T" --json body -q .body | grep -c "$URL"
```
Expected: `1`.

- [ ] **Step 5: Final report to the user**

State plainly: PR URL; tracking issue URL; counts by severity; the true coverage number vs. the 80% claim; any test failures or vet output from baseline; the list of anything skipped or left UNVERIFIED and why. Return to `main` locally:

```bash
cd /home/stephen-perry/Development/vt-uos && git checkout main
```

---

## Self-Review Notes

- **Spec coverage:** §1 → Tasks 1–3; §2 → Tasks 4–5; §3 → Tasks 6, 8, 9; §4 → Tasks 7, 10, 11; §5 → Tasks 10–12; Out-of-scope honored (no Go edits anywhere; `go mod tidy` only in diff mode).
- **Gates:** Task 9 Step 2 and Task 12 Step 2 are the two explicit user confirmations required by the spec.
- **Naming consistency:** `$SCRATCH` layout in File Structure matches every task; finding JSON schema (Task 6) is what Tasks 8–9 parse; `issue-map.tsv` format (`id<TAB>number`, plus `<lens>-MINOR` and `TRACKING` rows) is what Tasks 9, 11, 12 read; sidecar TSV format (`file<TAB>line<TAB>claim`) is shared by Tasks 7 and 10.
