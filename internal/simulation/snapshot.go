package simulation

import (
	"context"
	"fmt"
)

// Snapshot captures a consistent copy of the entire vault database to the
// configured backup directory and returns its path. Operators use snapshots to
// checkpoint an exhibit before a demonstration and to restore a clean baseline
// afterward (via `vtuos --restore <path>`), which is the safe way to reset a
// running vault.
func (e *Engine) CreateSnapshot(ctx context.Context) (string, error) {
	if err := e.saveState(ctx); err != nil {
		e.log.Warn("persisting state before snapshot failed", "error", err)
	}
	path, err := e.db.Backup(ctx)
	if err != nil {
		return "", fmt.Errorf("creating snapshot: %w", err)
	}
	e.log.Info("vault snapshot created", "path", path)
	return path, nil
}

// Births returns the cumulative number of births recorded by the engine.
func (e *Engine) Births() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.births
}

// Deaths returns the cumulative number of deaths recorded by the engine.
func (e *Engine) Deaths() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.deaths
}

// TickCount returns the number of simulated hours processed.
func (e *Engine) TickCount() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tickN
}
