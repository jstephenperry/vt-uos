package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/util"
)

// newTestApp creates an App instance backed by an in-memory database for testing.
// The App is initialized with a default config, a paused vault clock, and
// migrations applied. The window is set to 120x40 and marked ready.
func newTestApp(t *testing.T) *App {
	t.Helper()

	db, err := database.NewInMemory()
	if err != nil {
		t.Fatalf("creating test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Run migrations
	migrationsDir := filepath.Join("..", "database", "migrations")
	runTestMigrations(t, db, migrationsDir)

	cfg := config.Default()
	vaultTime, _ := time.Parse(time.RFC3339, "2077-10-23T09:47:00Z")
	clock := util.NewVaultClock(vaultTime, 1.0)
	clock.Pause()

	app := New(db, cfg, clock)

	// Simulate a window size message to make the app ready
	app.width = 120
	app.height = 40
	app.ready = true
	app.updateViewDimensions()

	return app
}

// runTestMigrations runs SQL migration files (Up portion only) on the database.
func runTestMigrations(t *testing.T, db *database.DB, migrationsDir string) {
	t.Helper()

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("reading migrations directory: %v", err)
	}

	ctx := context.Background()
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		sqlPath := filepath.Join(migrationsDir, file.Name())
		sqlBytes, err := os.ReadFile(sqlPath)
		if err != nil {
			t.Fatalf("reading migration %s: %v", file.Name(), err)
		}

		sqlStr := string(sqlBytes)
		if idx := strings.Index(sqlStr, "-- +migrate Down"); idx >= 0 {
			sqlStr = sqlStr[:idx]
		}

		if _, err := db.ExecContext(ctx, sqlStr); err != nil {
			t.Fatalf("executing migration %s: %v", file.Name(), err)
		}
	}
}

// keyMsg creates a tea.KeyMsg for a regular character key.
func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// specialKeyMsg creates a tea.KeyMsg for a special key type.
func specialKeyMsg(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}
