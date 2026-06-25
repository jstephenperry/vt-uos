package simulation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/database/seed"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/util"
)

// newTestEngine builds an in-memory, migrated, seeded vault and returns a
// started engine with the vault clock paused (so progression is driven by Step).
func newTestEngine(t *testing.T) (*Engine, *database.DB) {
	t.Helper()
	ctx := context.Background()

	db, err := database.NewInMemory()
	if err != nil {
		t.Fatalf("creating in-memory db: %v", err)
	}

	migrator, err := database.NewMigrator(db)
	if err != nil {
		t.Fatalf("creating migrator: %v", err)
	}
	if _, err := migrator.MigrateUp(ctx); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	cfg := config.Default()
	cfg.Vault.Number = 76
	cfg.Simulation.Enabled = false // hold the run loop; we drive with Step

	sealDate := time.Date(2077, 10, 23, 9, 47, 0, 0, time.UTC)
	seedCfg := seed.Config{
		VaultNumber:      cfg.Vault.Number,
		SealDate:         sealDate,
		TargetPopulation: 80,
		FamilyHouseholds: 12,
		SingleHouseholds: 10,
		RandomSeed:       2077,
	}
	if err := seed.NewGenerator(db.DB, seedCfg).Generate(ctx); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	clock := util.NewVaultClock(sealDate, cfg.Simulation.TimeScale)
	clock.Pause()

	eng := New(db, cfg, clock)
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("starting engine: %v", err)
	}
	return eng, db
}

func tableCount(t *testing.T, db *database.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}

func TestEngineStartEnsuresCoreSystems(t *testing.T) {
	eng, db := newTestEngine(t)
	defer eng.Stop(context.Background())

	if got := tableCount(t, db, "facility_systems"); got != len(coreSystems) {
		t.Fatalf("expected %d core systems, got %d", len(coreSystems), got)
	}

	st := eng.Snapshot()
	if st.Systems.Total != len(coreSystems) {
		t.Errorf("snapshot systems total = %d, want %d", st.Systems.Total, len(coreSystems))
	}
	if st.Population.Active == 0 {
		t.Error("expected a non-zero active population in snapshot")
	}
	if len(st.Resources) == 0 {
		t.Error("expected resource categories in snapshot")
	}
	if st.Power.GenerationKW <= 0 {
		t.Errorf("expected positive power generation, got %.1f", st.Power.GenerationKW)
	}
}

func TestEngineStepConsumesResources(t *testing.T) {
	ctx := context.Background()
	eng, db := newTestEngine(t)
	defer eng.Stop(ctx)

	consumptionBefore := consumptionTxns(t, db)

	// Advance ten simulated days.
	if err := eng.Step(ctx, 24*10); err != nil {
		t.Fatalf("stepping: %v", err)
	}

	consumptionAfter := consumptionTxns(t, db)
	if consumptionAfter <= consumptionBefore {
		t.Errorf("expected consumption transactions to increase (before=%d after=%d)",
			consumptionBefore, consumptionAfter)
	}

	st := eng.Snapshot()
	if st.ElapsedDays < 10 {
		t.Errorf("expected at least 10 elapsed days, got %d", st.ElapsedDays)
	}
	if st.TickCount < 240 {
		t.Errorf("expected at least 240 ticks, got %d", st.TickCount)
	}
	// Daily-use figures should be populated after processing.
	foundUse := false
	for _, r := range st.Resources {
		if r.DailyUse > 0 {
			foundUse = true
		}
	}
	if !foundUse {
		t.Error("expected at least one resource with a non-zero daily use")
	}
}

func consumptionTxns(t *testing.T, db *database.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM resource_transactions WHERE transaction_type='CONSUMPTION'").Scan(&n); err != nil {
		t.Fatalf("counting consumption: %v", err)
	}
	return n
}

func TestEngineControls(t *testing.T) {
	ctx := context.Background()
	eng, _ := newTestEngine(t)
	defer eng.Stop(ctx)

	eng.Resume()
	if eng.Status() != protocol.SimRunning {
		t.Errorf("after Resume status = %s, want RUNNING", eng.Status())
	}
	eng.Pause()
	if eng.Status() != protocol.SimPaused {
		t.Errorf("after Pause status = %s, want PAUSED", eng.Status())
	}
	if err := eng.SetTimeScale(120); err != nil {
		t.Fatalf("SetTimeScale: %v", err)
	}
	if eng.Clock().TimeScale() != 120 {
		t.Errorf("time scale = %.0f, want 120", eng.Clock().TimeScale())
	}
	if err := eng.SetTimeScale(-1); err == nil {
		t.Error("expected error for negative time scale")
	}
}

func TestEngineDeterministic(t *testing.T) {
	ctx := context.Background()

	run := func() (uint64, int, int, int) {
		eng, db := newTestEngine(t)
		defer eng.Stop(ctx)
		if err := eng.Step(ctx, 24*30); err != nil {
			t.Fatalf("stepping: %v", err)
		}
		events := tableCount(t, db, "simulation_events")
		return eng.TickCount(), eng.Births(), eng.Deaths(), events
	}

	t1, b1, d1, e1 := run()
	t2, b2, d2, e2 := run()

	if t1 != t2 || b1 != b2 || d1 != d2 || e1 != e2 {
		t.Errorf("non-deterministic run:\n  run1: ticks=%d births=%d deaths=%d events=%d\n  run2: ticks=%d births=%d deaths=%d events=%d",
			t1, b1, d1, e1, t2, b2, d2, e2)
	}
}

func TestEnginePersistenceRoundTrip(t *testing.T) {
	ctx := context.Background()
	eng, db := newTestEngine(t)

	if err := eng.Step(ctx, 24*5); err != nil {
		t.Fatalf("stepping: %v", err)
	}
	births, deaths, tick := eng.Births(), eng.Deaths(), eng.TickCount()
	if err := eng.saveState(ctx); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	eng.Stop(ctx)

	// A fresh engine over the same database should restore continuity.
	cfg := config.Default()
	cfg.Vault.Number = 76
	cfg.Simulation.Enabled = false
	clock := util.NewVaultClock(time.Date(2077, 10, 23, 9, 47, 0, 0, time.UTC), 60)
	clock.Pause()
	eng2 := New(db, cfg, clock)
	if err := eng2.Start(ctx); err != nil {
		t.Fatalf("starting second engine: %v", err)
	}
	defer eng2.Stop(ctx)

	if eng2.Births() != births || eng2.Deaths() != deaths || eng2.TickCount() != tick {
		t.Errorf("continuity not restored: got births=%d deaths=%d tick=%d, want births=%d deaths=%d tick=%d",
			eng2.Births(), eng2.Deaths(), eng2.TickCount(), births, deaths, tick)
	}
	// The restored clock should be at or after the persisted vault time.
	if eng2.Clock().Now().Before(clock.Now().Add(-time.Hour)) {
		t.Error("restored clock did not advance to persisted vault time")
	}
}

func TestEngineConcurrentSubscribeBroadcast(t *testing.T) {
	ctx := context.Background()
	eng, _ := newTestEngine(t)
	defer eng.Stop(ctx)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Continuously subscribe/unsubscribe while broadcasts occur — this is the
	// interleaving that previously sent on a closed channel.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				id, ch := eng.Subscribe()
				// Drain a little so the buffer can fill, then release.
				select {
				case <-ch:
				default:
				}
				eng.Unsubscribe(id)
			}
		}()
	}

	// Drive broadcasts via stepping and direct control changes.
	for i := 0; i < 30; i++ {
		eng.Pause()
		eng.Resume()
		if err := eng.Step(ctx, 6); err != nil {
			t.Fatalf("step: %v", err)
		}
	}
	close(stop)
	wg.Wait()
}

func TestEngineSubscribe(t *testing.T) {
	ctx := context.Background()
	eng, _ := newTestEngine(t)
	defer eng.Stop(ctx)

	id, ch := eng.Subscribe()
	defer eng.Unsubscribe(id)

	// The subscription is primed with the current state immediately.
	select {
	case st := <-ch:
		if st.SchemaVersion != protocol.Version {
			t.Errorf("primed state schema = %q, want %q", st.SchemaVersion, protocol.Version)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive primed state")
	}

	// A step should push a fresh frame.
	if err := eng.Step(ctx, 24); err != nil {
		t.Fatalf("stepping: %v", err)
	}
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive state after step")
	}
}
