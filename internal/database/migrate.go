package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migration represents a database migration.
type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
	Applied     bool
	AppliedAt   time.Time
}

// MigrationResult contains the result of running migrations.
type MigrationResult struct {
	Applied        []Migration
	CurrentVersion int
	TargetVersion  int
	Error          error
}

// Migrator handles database schema migrations.
type Migrator struct {
	db         *DB
	migrations []Migration
}

// NewMigrator creates a new Migrator for the given database.
func NewMigrator(db *DB) (*Migrator, error) {
	m := &Migrator{db: db}

	// Load migrations from embedded filesystem
	if err := m.loadMigrations(); err != nil {
		return nil, fmt.Errorf("loading migrations: %w", err)
	}

	// Ensure migrations table exists
	if err := m.ensureMigrationsTable(); err != nil {
		return nil, fmt.Errorf("creating migrations table: %w", err)
	}

	return m, nil
}

// loadMigrations reads all migration files from the embedded filesystem.
func (m *Migrator) loadMigrations() error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	// Pattern: NNN_description.sql
	pattern := regexp.MustCompile(`^(\d{3})_(.+)\.sql$`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			slog.Warn("skipping invalid migration filename", "name", entry.Name())
			continue
		}

		version, _ := strconv.Atoi(matches[1])
		description := strings.ReplaceAll(matches[2], "_", " ")

		content, err := fs.ReadFile(migrationsFS, filepath.Join("migrations", entry.Name()))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}

		upSQL, downSQL := parseMigration(string(content))

		m.migrations = append(m.migrations, Migration{
			Version:     version,
			Description: description,
			UpSQL:       upSQL,
			DownSQL:     downSQL,
		})
	}

	// Sort by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	return nil
}

// parseMigration extracts UP and DOWN SQL from migration content.
// Format:
//
//	-- +migrate Up
//	SQL statements...
//	-- +migrate Down
//	SQL statements...
func parseMigration(content string) (upSQL, downSQL string) {
	upMarker := "-- +migrate Up"
	downMarker := "-- +migrate Down"

	upIdx := strings.Index(content, upMarker)
	downIdx := strings.Index(content, downMarker)

	if upIdx == -1 {
		// No markers, treat entire content as UP
		return strings.TrimSpace(content), ""
	}

	if downIdx == -1 {
		// Only UP section
		upSQL = strings.TrimSpace(content[upIdx+len(upMarker):])
		return upSQL, ""
	}

	if upIdx < downIdx {
		upSQL = strings.TrimSpace(content[upIdx+len(upMarker) : downIdx])
		downSQL = strings.TrimSpace(content[downIdx+len(downMarker):])
	} else {
		downSQL = strings.TrimSpace(content[downIdx+len(downMarker) : upIdx])
		upSQL = strings.TrimSpace(content[upIdx+len(upMarker):])
	}

	return upSQL, downSQL
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist.
func (m *Migrator) ensureMigrationsTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now')),
			checksum TEXT
		)
	`)
	return err
}

// CurrentVersion returns the current schema version.
func (m *Migrator) CurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("querying current version: %w", err)
	}
	return version, nil
}

// PendingMigrations returns migrations that haven't been applied yet.
func (m *Migrator) PendingMigrations(ctx context.Context) ([]Migration, error) {
	current, err := m.CurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, mig := range m.migrations {
		if mig.Version > current {
			pending = append(pending, mig)
		}
	}

	return pending, nil
}

// MigrateUp runs all pending migrations.
func (m *Migrator) MigrateUp(ctx context.Context) (*MigrationResult, error) {
	current, err := m.CurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	result := &MigrationResult{
		CurrentVersion: current,
	}

	pending, err := m.PendingMigrations(ctx)
	if err != nil {
		return nil, err
	}

	if len(pending) == 0 {
		result.TargetVersion = current
		slog.Info("database is up to date", "version", current)
		return result, nil
	}

	result.TargetVersion = pending[len(pending)-1].Version

	for _, mig := range pending {
		slog.Info("applying migration",
			"version", mig.Version,
			"description", mig.Description,
		)

		if err := m.applyMigration(ctx, mig); err != nil {
			result.Error = fmt.Errorf("migration %d failed: %w", mig.Version, err)
			return result, result.Error
		}

		mig.Applied = true
		mig.AppliedAt = time.Now()
		result.Applied = append(result.Applied, mig)
	}

	slog.Info("migrations complete",
		"from", current,
		"to", result.TargetVersion,
		"applied", len(result.Applied),
	)

	return result, nil
}

// applyMigration applies a single migration within a transaction.
func (m *Migrator) applyMigration(ctx context.Context, mig Migration) error {
	return m.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Execute the migration SQL
		// Split by semicolon to handle multiple statements
		statements := splitStatements(mig.UpSQL)
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
			}
		}

		// Record the migration
		_, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
			mig.Version, mig.Description,
		)
		if err != nil {
			return fmt.Errorf("recording migration: %w", err)
		}

		return nil
	})
}

// MigrateDown rolls back the last migration.
func (m *Migrator) MigrateDown(ctx context.Context) (*MigrationResult, error) {
	current, err := m.CurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	result := &MigrationResult{
		CurrentVersion: current,
	}

	if current == 0 {
		return result, errors.New("no migrations to roll back")
	}

	// Find the migration to roll back
	var mig *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == current {
			mig = &m.migrations[i]
			break
		}
	}

	if mig == nil {
		return result, fmt.Errorf("migration %d not found", current)
	}

	if mig.DownSQL == "" {
		return result, fmt.Errorf("migration %d has no rollback SQL", current)
	}

	slog.Info("rolling back migration",
		"version", mig.Version,
		"description", mig.Description,
	)

	if err := m.rollbackMigration(ctx, *mig); err != nil {
		result.Error = fmt.Errorf("rollback %d failed: %w", mig.Version, err)
		return result, result.Error
	}

	result.TargetVersion = current - 1

	return result, nil
}

// rollbackMigration rolls back a single migration within a transaction.
func (m *Migrator) rollbackMigration(ctx context.Context, mig Migration) error {
	return m.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Execute the rollback SQL
		statements := splitStatements(mig.DownSQL)
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("executing statement: %w\nSQL: %s", err, stmt)
			}
		}

		// Remove the migration record
		_, err := tx.ExecContext(ctx,
			"DELETE FROM schema_migrations WHERE version = ?",
			mig.Version,
		)
		if err != nil {
			return fmt.Errorf("removing migration record: %w", err)
		}

		return nil
	})
}

// MigrateTo migrates to a specific version (up or down).
func (m *Migrator) MigrateTo(ctx context.Context, targetVersion int) (*MigrationResult, error) {
	current, err := m.CurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	result := &MigrationResult{
		CurrentVersion: current,
		TargetVersion:  targetVersion,
	}

	if targetVersion == current {
		return result, nil
	}

	if targetVersion > current {
		// Migrate up
		for _, mig := range m.migrations {
			if mig.Version > current && mig.Version <= targetVersion {
				if err := m.applyMigration(ctx, mig); err != nil {
					result.Error = err
					return result, err
				}
				mig.Applied = true
				mig.AppliedAt = time.Now()
				result.Applied = append(result.Applied, mig)
			}
		}
	} else {
		// Migrate down
		for i := len(m.migrations) - 1; i >= 0; i-- {
			mig := m.migrations[i]
			if mig.Version <= current && mig.Version > targetVersion {
				if mig.DownSQL == "" {
					result.Error = fmt.Errorf("migration %d has no rollback SQL", mig.Version)
					return result, result.Error
				}
				if err := m.rollbackMigration(ctx, mig); err != nil {
					result.Error = err
					return result, err
				}
				result.Applied = append(result.Applied, mig)
			}
		}
	}

	return result, nil
}

// DryRun shows what migrations would be applied without applying them.
func (m *Migrator) DryRun(ctx context.Context) ([]Migration, error) {
	return m.PendingMigrations(ctx)
}

// Status returns the status of all migrations.
func (m *Migrator) Status(ctx context.Context) ([]Migration, error) {
	current, err := m.CurrentVersion(ctx)
	if err != nil {
		return nil, err
	}

	// Get applied migrations from database
	rows, err := m.db.QueryContext(ctx,
		"SELECT version, applied_at FROM schema_migrations ORDER BY version",
	)
	if err != nil {
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var appliedAt string
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		t, _ := time.Parse("2006-01-02 15:04:05", appliedAt)
		applied[version] = t
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	// Build status for all migrations
	result := make([]Migration, len(m.migrations))
	for i, mig := range m.migrations {
		result[i] = mig
		if t, ok := applied[mig.Version]; ok {
			result[i].Applied = true
			result[i].AppliedAt = t
		}
	}

	_ = current // Used for logging if needed

	return result, nil
}

// splitStatements splits SQL content into individual statements.
// Handles semicolons properly (not those inside strings).
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for i, ch := range sql {
		if inString {
			current.WriteRune(ch)
			if ch == stringChar && (i == 0 || sql[i-1] != '\\') {
				inString = false
			}
		} else {
			if ch == '\'' || ch == '"' {
				inString = true
				stringChar = ch
				current.WriteRune(ch)
			} else if ch == ';' {
				stmt := strings.TrimSpace(current.String())
				if stmt != "" {
					statements = append(statements, stmt)
				}
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		}
	}

	// Handle last statement without semicolon
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}
