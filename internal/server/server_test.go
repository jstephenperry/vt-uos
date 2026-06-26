package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/database/seed"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/simulation"
	"github.com/vtuos/vtuos/internal/util"
)

func newTestServer(t *testing.T) (*Server, *simulation.Engine) {
	t.Helper()
	ctx := context.Background()

	db, err := database.NewInMemory()
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mig, err := database.NewMigrator(db)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if _, err := mig.MigrateUp(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := config.Default()
	cfg.Simulation.Enabled = false
	cfg.Server.AdminToken = "secret"
	cfg.Server.EnableWeb = true

	sealDate := time.Date(2077, 10, 23, 9, 47, 0, 0, time.UTC)
	if err := seed.NewGenerator(db.DB, seed.Config{
		VaultNumber: 76, SealDate: sealDate, TargetPopulation: 60,
		FamilyHouseholds: 8, SingleHouseholds: 8, RandomSeed: 7,
	}).Generate(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	clock := util.NewVaultClock(sealDate, 60)
	clock.Pause()
	eng := simulation.New(db, cfg, clock)
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("engine: %v", err)
	}
	t.Cleanup(func() { eng.Stop(context.Background()) })

	srv, err := New(eng, cfg)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	return srv, eng
}

func do(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServerHealthAndState(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()

	if rec := do(t, h, "GET", "/healthz", nil, nil); rec.Code != 200 {
		t.Fatalf("healthz code = %d", rec.Code)
	}

	rec := do(t, h, "GET", "/api/v1/state", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("state code = %d", rec.Code)
	}
	var st protocol.VaultState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if st.SchemaVersion != protocol.Version {
		t.Errorf("schema = %q", st.SchemaVersion)
	}
	if st.Population.Active == 0 {
		t.Error("expected active population in state")
	}
	if st.Systems.Total == 0 {
		t.Error("expected systems in state")
	}
}

func TestServerControlRequiresToken(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()

	// Without the operator token, control is rejected.
	if rec := do(t, h, "POST", "/api/v1/sim/pause", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
	// With the token, control succeeds.
	rec := do(t, h, "POST", "/api/v1/sim/pause", nil, map[string]string{"X-Admin-Token": "secret"})
	if rec.Code != 200 {
		t.Fatalf("expected 200 with token, got %d", rec.Code)
	}
}

func TestServerSimStep(t *testing.T) {
	srv, eng := newTestServer(t)
	h := srv.routes()
	before := eng.TickCount()
	rec := do(t, h, "POST", "/api/v1/sim/step", protocol.ControlRequest{Hours: 48},
		map[string]string{"X-Admin-Token": "secret"})
	if rec.Code != 200 {
		t.Fatalf("step code = %d body=%s", rec.Code, rec.Body.String())
	}
	if eng.TickCount() < before+48 {
		t.Errorf("expected tick count to advance by 48 (before=%d after=%d)", before, eng.TickCount())
	}
}

func TestServerClientLifecycleAndRemoteOps(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()
	admin := map[string]string{"X-Admin-Token": "secret"}

	// Register a client terminal.
	rec := do(t, h, "POST", "/api/v1/clients/register",
		protocol.RegisterRequest{Protocol: protocol.Version, Name: "Atrium", Kind: protocol.ClientDisplay}, nil)
	if rec.Code != 200 {
		t.Fatalf("register code = %d", rec.Code)
	}
	var reg protocol.RegisterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if reg.ClientID == "" || reg.Token == "" {
		t.Fatal("expected client id and token")
	}

	// Operator issues a remote operation.
	rec = do(t, h, "POST", "/api/v1/clients/"+reg.ClientID+"/command",
		protocol.Command{Type: protocol.CmdIdentify}, admin)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("command code = %d", rec.Code)
	}

	// Client heartbeats and receives the queued command.
	rec = do(t, h, "POST", "/api/v1/clients/"+reg.ClientID+"/heartbeat",
		protocol.Telemetry{CurrentView: "dashboard"},
		map[string]string{"X-Client-Token": reg.Token})
	if rec.Code != 200 {
		t.Fatalf("heartbeat code = %d", rec.Code)
	}
	var hb protocol.HeartbeatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &hb); err != nil {
		t.Fatalf("decode heartbeat: %v", err)
	}
	if len(hb.Commands) != 1 || hb.Commands[0].Type != protocol.CmdIdentify {
		t.Fatalf("expected one IDENTIFY command, got %+v", hb.Commands)
	}

	// A bad client token is rejected.
	rec = do(t, h, "POST", "/api/v1/clients/"+reg.ClientID+"/heartbeat",
		protocol.Telemetry{}, map[string]string{"X-Client-Token": "wrong"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad client token, got %d", rec.Code)
	}

	// Operator lists terminals and sees the registered client.
	rec = do(t, h, "GET", "/api/v1/clients", nil, admin)
	if rec.Code != 200 {
		t.Fatalf("clients list code = %d", rec.Code)
	}
	var list []protocol.ClientInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode clients: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Atrium" {
		t.Fatalf("expected one client 'Atrium', got %+v", list)
	}
}

func TestServerWebConsoleServes(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()
	rec := do(t, h, "GET", "/", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("console code = %d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("UNIFIED OPERATING SYSTEM")) {
		t.Error("expected console HTML")
	}
	if rec := do(t, h, "GET", "/static/app.js", nil, nil); rec.Code != 200 {
		t.Errorf("static asset code = %d", rec.Code)
	}
}
