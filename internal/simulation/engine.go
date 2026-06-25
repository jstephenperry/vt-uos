// Package simulation implements the VT-UOS simulation control core: the
// authoritative engine that advances vault time and drives realistic resource
// consumption, production, facility degradation, scheduled maintenance, and
// operational incidents against the persistent database.
//
// The engine is designed as operational software, not a game. Every interval it
// processes is deterministic given its seed, mutates real domain state through
// the service layer, records occurrences to the persistent event and audit
// logs, and publishes an operational snapshot (protocol.VaultState) to any
// subscribed consumers — the local TUI, the master server's web console, and
// remote client terminals.
package simulation

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/services/facilities"
	"github.com/vtuos/vtuos/internal/services/population"
	"github.com/vtuos/vtuos/internal/services/resources"
	"github.com/vtuos/vtuos/internal/util"
)

const (
	// wakeInterval is how often the engine wakes to advance simulated time.
	// A short real-time cadence keeps telemetry fresh for exhibit displays.
	wakeInterval = 1 * time.Second

	// maxHoursPerWake caps the simulated hours processed per wake so a large
	// time scale (or a long pause/resume gap) cannot stall the engine.
	maxHoursPerWake = 168 // one simulated week

	// eventBufferSize is the number of recent operational events retained in
	// memory for the live feed. The full history lives in simulation_events.
	eventBufferSize = 200
)

// Engine is the simulation control core. It is safe for concurrent use.
type Engine struct {
	db    *database.DB
	cfg   *config.Config
	clock *util.VaultClock
	log   *slog.Logger
	idGen *util.IDGenerator

	// Domain services. The engine owns a single shared set so the TUI and
	// server read and write through the same code paths the engine uses.
	pop *population.Service
	res *resources.Service
	fac *facilities.Service

	// rng is the engine's deterministic source of randomness. It is only
	// touched from the processing goroutine (and Step), so it needs no lock.
	rng  *rand.Rand
	seed int64

	mu       sync.RWMutex
	status   protocol.SimStatus
	lastProc time.Time // last vault time fully processed
	sealDate time.Time
	tickN    uint64

	// dailyUse holds the most recently planned daily consumption per resource
	// category code, used to compute runway in operational snapshots.
	dailyUse map[string]float64

	// births/deaths accumulate over the run for the census board.
	births int
	deaths int

	state  protocol.VaultState
	alerts []protocol.Alert
	events []protocol.EventRecord

	subs   map[int]chan protocol.VaultState
	nextID int

	// lifecycle
	runMu   sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// New constructs a simulation engine bound to the given database, configuration
// and vault clock. The engine creates its own service layer; callers that need
// service access (TUI, server) should reuse the accessors below.
func New(db *database.DB, cfg *config.Config, clock *util.VaultClock) *Engine {
	seed := deriveSeed(cfg)
	e := &Engine{
		db:       db,
		cfg:      cfg,
		clock:    clock,
		log:      slog.Default().With("component", "simulation"),
		idGen:    util.NewIDGenerator(),
		pop:      population.NewService(db.DB, cfg.Vault.Number),
		res:      resources.NewService(db.DB),
		fac:      facilities.NewService(db.DB),
		rng:      rand.New(rand.NewSource(seed)),
		seed:     seed,
		status:   protocol.SimPaused,
		dailyUse: make(map[string]float64),
		subs:     make(map[int]chan protocol.VaultState),
	}
	if sd, err := cfg.Simulation.StartDateTime(); err == nil {
		e.sealDate = sd
	} else if sd, err := cfg.Vault.SealedDateTime(); err == nil {
		e.sealDate = sd
	} else {
		e.sealDate = clock.Now()
	}
	e.lastProc = clock.Now()
	return e
}

// deriveSeed produces a stable seed so identical configurations replay
// identically, satisfying the project's deterministic-build constraint.
func deriveSeed(cfg *config.Config) int64 {
	seed := int64(cfg.Vault.Number) * 1_000_003
	for i, r := range cfg.Simulation.StartDate {
		seed += int64(r) * int64(i+1)
	}
	if seed == 0 {
		seed = 2077
	}
	return seed
}

// Population returns the engine's population service.
func (e *Engine) Population() *population.Service { return e.pop }

// Resources returns the engine's resource service.
func (e *Engine) Resources() *resources.Service { return e.res }

// Facilities returns the engine's facilities service.
func (e *Engine) Facilities() *facilities.Service { return e.fac }

// Clock returns the vault clock driving the engine.
func (e *Engine) Clock() *util.VaultClock { return e.clock }

// Start prepares operational state and, if simulation is enabled, begins
// advancing vault time. It is idempotent: a second call is a no-op.
func (e *Engine) Start(ctx context.Context) error {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	if e.running {
		return nil
	}

	// Restore persisted operational time for power-loss continuity.
	if err := e.loadState(ctx); err != nil {
		e.log.Warn("could not restore simulation state", "error", err)
	}

	// Ensure the vault has infrastructure to operate. Core systems are
	// reference data that any commissioned vault possesses.
	if err := EnsureCoreSystems(ctx, e.db, e.fac, e.sealDate); err != nil {
		e.log.Warn("could not ensure core facility systems", "error", err)
	}

	// Build an initial operational snapshot so consumers have data immediately.
	if err := e.recomputeState(ctx); err != nil {
		e.log.Warn("initial state computation failed", "error", err)
	}

	enabled := e.cfg.Simulation.Enabled && !e.clock.IsPaused()
	e.mu.Lock()
	if enabled {
		e.status = protocol.SimRunning
	} else {
		e.status = protocol.SimPaused
	}
	e.mu.Unlock()

	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})
	e.running = true
	go e.run()

	e.log.Info("simulation control core started",
		"status", e.statusString(),
		"vault_time", e.clock.Now().Format(time.RFC3339),
		"time_scale", e.clock.TimeScale(),
		"seed", e.seed,
	)
	return nil
}

// Stop halts the processing goroutine and persists operational state.
func (e *Engine) Stop(ctx context.Context) error {
	e.runMu.Lock()
	if !e.running {
		e.runMu.Unlock()
		return nil
	}
	e.running = false
	close(e.stopCh)
	done := e.doneCh
	e.runMu.Unlock()

	<-done

	if err := e.saveState(ctx); err != nil {
		e.log.Warn("could not persist simulation state on stop", "error", err)
	}
	e.log.Info("simulation control core stopped")
	return nil
}

// run is the processing loop. It wakes on a fixed real-time cadence and advances
// the simulation to match the wall-clock-scaled vault time.
func (e *Engine) run() {
	defer close(e.doneCh)
	ticker := time.NewTicker(wakeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if e.Status() != protocol.SimRunning {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := e.advanceTo(ctx, e.clock.Now()); err != nil {
				e.log.Error("simulation tick failed", "error", err)
			}
			cancel()
		}
	}
}

// Pause holds vault time steady without unloading the engine.
func (e *Engine) Pause() {
	e.clock.Pause()
	e.mu.Lock()
	e.status = protocol.SimPaused
	e.mu.Unlock()
	e.broadcast()
	e.log.Info("simulation paused", "vault_time", e.clock.Now().Format(time.RFC3339))
}

// Resume restarts automatic time progression.
func (e *Engine) Resume() {
	e.clock.Resume()
	e.mu.Lock()
	e.lastProc = e.clock.Now()
	e.status = protocol.SimRunning
	e.mu.Unlock()
	e.broadcast()
	e.log.Info("simulation resumed", "vault_time", e.clock.Now().Format(time.RFC3339))
}

// SetTimeScale changes the ratio of vault time to real time. Valid scales are
// non-negative; a scale of zero effectively freezes progression.
func (e *Engine) SetTimeScale(scale float64) error {
	if scale < 0 {
		return fmt.Errorf("time scale must be non-negative, got %.2f", scale)
	}
	e.clock.SetTimeScale(scale)
	e.mu.Lock()
	e.lastProc = e.clock.Now()
	e.mu.Unlock()
	e.broadcast()
	e.log.Info("time scale changed", "scale", scale)
	return nil
}

// Step manually advances vault time by the given number of hours and processes
// the interval. It is intended for use while paused (single-stepping the core).
func (e *Engine) Step(ctx context.Context, hours int) error {
	if hours <= 0 {
		hours = 1
	}
	if !e.clock.IsPaused() {
		e.clock.Pause()
		e.mu.Lock()
		e.status = protocol.SimPaused
		e.mu.Unlock()
	}
	if err := e.clock.Advance(time.Duration(hours) * time.Hour); err != nil {
		return fmt.Errorf("advancing clock: %w", err)
	}

	// A single advanceTo processes at most maxHoursPerWake (runaway protection
	// for the automatic loop). An explicit step is an operator action, so drain
	// the full requested span in capped chunks.
	target := e.clock.Now()
	for i := 0; i <= hours/maxHoursPerWake+1; i++ {
		if err := e.advanceTo(ctx, target); err != nil {
			return err
		}
		e.mu.RLock()
		caughtUp := !e.lastProc.Before(target.Add(-time.Hour))
		e.mu.RUnlock()
		if caughtUp {
			break
		}
	}
	return nil
}

// Status returns the current run state of the control core.
func (e *Engine) Status() protocol.SimStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

func (e *Engine) statusString() string { return string(e.Status()) }

// Snapshot returns the latest operational snapshot. The returned value is a
// copy safe for the caller to read and marshal.
func (e *Engine) Snapshot() protocol.VaultState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cloneState()
}

// Alerts returns the currently active operational alerts.
func (e *Engine) Alerts() []protocol.Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]protocol.Alert, len(e.alerts))
	copy(out, e.alerts)
	return out
}

// Events returns up to limit of the most recent operational events, newest first.
func (e *Engine) Events(limit int) []protocol.EventRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.events) {
		limit = len(e.events)
	}
	out := make([]protocol.EventRecord, 0, limit)
	for i := len(e.events) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, e.events[i])
	}
	return out
}

// AcknowledgeAlert clears the acknowledged flag for the alert with the given code.
func (e *Engine) AcknowledgeAlert(code string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.alerts {
		if e.alerts[i].Code == code {
			e.alerts[i].Acknowledged = true
			return true
		}
	}
	return false
}

// Subscribe registers a consumer for operational snapshots. It returns the
// subscriber id and a buffered channel that receives a copy of the state after
// every recomputation. Call Unsubscribe with the id to release it.
func (e *Engine) Subscribe() (int, <-chan protocol.VaultState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	id := e.nextID
	e.nextID++
	ch := make(chan protocol.VaultState, 4)
	e.subs[id] = ch
	// Prime the subscriber with the current state.
	ch <- e.cloneState()
	return id, ch
}

// Unsubscribe releases a previously registered subscriber.
func (e *Engine) Unsubscribe(id int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ch, ok := e.subs[id]; ok {
		delete(e.subs, id)
		close(ch)
	}
}

// broadcast pushes the current state to all subscribers without blocking.
func (e *Engine) broadcast() {
	e.mu.RLock()
	st := e.cloneState()
	subs := make([]chan protocol.VaultState, 0, len(e.subs))
	for _, ch := range e.subs {
		subs = append(subs, ch)
	}
	e.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- st:
		default:
			// Slow consumer: drop this frame rather than stall the engine.
		}
	}
}

// cloneState returns a deep-enough copy of the state for safe external reads.
// Callers must hold at least a read lock.
func (e *Engine) cloneState() protocol.VaultState {
	st := e.state
	st.Resources = append([]protocol.ResourceStatus(nil), e.state.Resources...)
	st.SystemList = append([]protocol.SystemStatus(nil), e.state.SystemList...)
	st.Alerts = append([]protocol.Alert(nil), e.alerts...)
	// newest-first, capped for transport
	limit := 25
	if limit > len(e.events) {
		limit = len(e.events)
	}
	recent := make([]protocol.EventRecord, 0, limit)
	for i := len(e.events) - 1; i >= 0 && len(recent) < limit; i-- {
		recent = append(recent, e.events[i])
	}
	st.RecentEvents = recent
	return st
}
