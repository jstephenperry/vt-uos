// Package testutil provides utilities for testing.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // SQLite driver
)

// TestDB wraps a test database connection.
type TestDB struct {
	*sql.DB
	path string
}

// NewTestDB creates a new in-memory SQLite database for testing.
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Use in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	return &TestDB{DB: db, path: ":memory:"}
}

// NewTestDBWithFile creates a test database backed by a temporary file.
// Useful for debugging tests.
func NewTestDBWithFile(t *testing.T) *TestDB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	return &TestDB{DB: db, path: dbPath}
}

// RunMigrations executes SQL migration files in order.
// Only executes the "Up" portion of each migration (before "-- +migrate Down").
func (tdb *TestDB) RunMigrations(t *testing.T, migrationsDir string) {
	t.Helper()

	// Read all .sql files from migrations directory
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	ctx := context.Background()

	// Execute each migration file individually (SQLite in-memory needs
	// tables to exist before indexes can reference them across files)
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		sqlPath := filepath.Join(migrationsDir, file.Name())
		sqlBytes, err := os.ReadFile(sqlPath)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", file.Name(), err)
		}

		// Extract only the "Up" portion (before "-- +migrate Down")
		sqlStr := string(sqlBytes)
		if idx := strings.Index(sqlStr, "-- +migrate Down"); idx >= 0 {
			sqlStr = sqlStr[:idx]
		}

		if _, err := tdb.ExecContext(ctx, sqlStr); err != nil {
			t.Fatalf("failed to execute migration %s: %v", file.Name(), err)
		}
	}
}

// RunSchema executes a SQL schema file directly.
func (tdb *TestDB) RunSchema(t *testing.T, schemaPath string) {
	t.Helper()

	sqlBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema file: %v", err)
	}

	ctx := context.Background()
	if _, err := tdb.ExecContext(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("failed to execute schema: %v", err)
	}
}

// Close closes the test database and cleans up resources.
func (tdb *TestDB) Close(t *testing.T) {
	t.Helper()

	if err := tdb.DB.Close(); err != nil {
		t.Errorf("failed to close test database: %v", err)
	}
}

// Truncate removes all data from specified tables while maintaining schema.
func (tdb *TestDB) Truncate(t *testing.T, tables ...string) {
	t.Helper()

	ctx := context.Background()
	tx, err := tdb.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Disable foreign keys temporarily
	if _, err := tx.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("failed to disable foreign keys: %v", err)
	}

	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}

	// Re-enable foreign keys
	if _, err := tx.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit truncate: %v", err)
	}
}

// AssertRowCount asserts the row count for a table.
func (tdb *TestDB) AssertRowCount(t *testing.T, table string, expected int) {
	t.Helper()

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if err := tdb.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("failed to count rows in %s: %v", table, err)
	}

	if count != expected {
		t.Errorf("expected %d rows in %s, got %d", expected, table, count)
	}
}

// ExecSQL executes arbitrary SQL (useful for test setup).
func (tdb *TestDB) ExecSQL(t *testing.T, sql string, args ...any) {
	t.Helper()

	if _, err := tdb.Exec(sql, args...); err != nil {
		t.Fatalf("failed to execute SQL: %v\nSQL: %s", err, sql)
	}
}
