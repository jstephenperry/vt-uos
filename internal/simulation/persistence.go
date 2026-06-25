package simulation

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// metadata keys used to persist operational continuity across restarts.
const (
	mdVaultTime = "vault_time"
	mdBirths    = "sim_births"
	mdDeaths    = "sim_deaths"
	mdTickCount = "sim_tick_count"
)

// saveState persists the simulation's operational position so the vault resumes
// where it left off after a restart or power loss.
func (e *Engine) saveState(ctx context.Context) error {
	e.mu.RLock()
	vaultTime := e.clock.Now().UTC().Format(time.RFC3339)
	births := e.births
	deaths := e.deaths
	tick := e.tickN
	e.mu.RUnlock()

	pairs := map[string]string{
		mdVaultTime: vaultTime,
		mdBirths:    strconv.Itoa(births),
		mdDeaths:    strconv.Itoa(deaths),
		mdTickCount: strconv.FormatUint(tick, 10),
	}
	for k, v := range pairs {
		if err := e.putMetadata(ctx, k, v); err != nil {
			return fmt.Errorf("persisting %s: %w", k, err)
		}
	}
	return nil
}

// loadState restores operational continuity from the metadata table. The vault
// clock is advanced to the persisted time so progression continues seamlessly.
func (e *Engine) loadState(ctx context.Context) error {
	vt, err := e.getMetadata(ctx, mdVaultTime)
	if err != nil {
		return err
	}
	if vt != "" {
		if t, perr := time.Parse(time.RFC3339, vt); perr == nil && !t.Before(e.sealDate) {
			wasPaused := e.clock.IsPaused()
			e.clock.Pause()
			if serr := e.clock.SetTime(t); serr == nil {
				e.lastProc = t
			}
			if !wasPaused {
				e.clock.Resume()
			}
		}
	}
	if b, err := e.getMetadata(ctx, mdBirths); err == nil && b != "" {
		e.births, _ = strconv.Atoi(b)
	}
	if d, err := e.getMetadata(ctx, mdDeaths); err == nil && d != "" {
		e.deaths, _ = strconv.Atoi(d)
	}
	if t, err := e.getMetadata(ctx, mdTickCount); err == nil && t != "" {
		if n, perr := strconv.ParseUint(t, 10, 64); perr == nil {
			e.tickN = n
		}
	}
	return nil
}

func (e *Engine) putMetadata(ctx context.Context, key, value string) error {
	const q = `INSERT INTO vault_metadata (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := e.db.ExecContext(ctx, q, key, value, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (e *Engine) getMetadata(ctx context.Context, key string) (string, error) {
	var v string
	err := e.db.QueryRowContext(ctx, `SELECT value FROM vault_metadata WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}
