// Package database provides SQLite database management with mission-critical
// fault tolerance features including WAL mode, power-loss resilience, and
// automatic backup scheduling.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vtuos/vtuos/internal/config"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB with additional functionality for mission-critical operations.
type DB struct {
	*sql.DB
	path      string
	config    *config.DatabaseConfig
	backupDir string

	// Shutdown coordination
	mu        sync.RWMutex
	closed    bool
	closeChan chan struct{}

	// Backup scheduling
	backupTicker *time.Ticker
	backupDone   chan struct{}
}

// Open creates a new database connection with WAL mode enabled for power-loss resilience.
// It performs integrity checks and enables all safety pragmas.
func Open(dbPath string, cfg *config.DatabaseConfig, backupDir string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	// Build connection string with parameters
	connStr := fmt.Sprintf("file:%s?_txlock=immediate&_timeout=5000&_fk=true", dbPath)

	// Open database connection
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(1) // SQLite only supports one writer
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0) // Keep connection open

	db := &DB{
		DB:        sqlDB,
		path:      dbPath,
		config:    cfg,
		backupDir: backupDir,
		closeChan: make(chan struct{}),
	}

	// Initialize with safety pragmas
	if err := db.initPragmas(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("initializing pragmas: %w", err)
	}

	// Perform initial integrity check
	if err := db.CheckIntegrity(context.Background()); err != nil {
		slog.Warn("database integrity check failed", "error", err)
		// Don't fail here - recovery will be attempted by caller
	}

	// Start backup scheduler if configured
	if cfg.BackupIntervalHours > 0 && backupDir != "" {
		db.startBackupScheduler()
	}

	return db, nil
}

// initPragmas sets all critical SQLite pragmas for mission-critical operation.
func (db *DB) initPragmas() error {
	pragmas := []struct {
		name   string
		pragma string
	}{
		// WAL mode for power-loss resilience
		{"journal_mode", "PRAGMA journal_mode=WAL"},
		// Synchronous NORMAL balances safety and performance
		{"synchronous", "PRAGMA synchronous=NORMAL"},
		// 5 second busy timeout for concurrent access
		{"busy_timeout", "PRAGMA busy_timeout=5000"},
		// Enable foreign key constraints
		{"foreign_keys", "PRAGMA foreign_keys=ON"},
		// Use 4KB page size (matches typical filesystem block size)
		{"page_size", "PRAGMA page_size=4096"},
		// 16MB cache for good performance
		{"cache_size", "PRAGMA cache_size=-16000"},
		// Enable memory-mapped I/O for reads (256MB)
		{"mmap_size", "PRAGMA mmap_size=268435456"},
		// Secure delete for sensitive data
		{"secure_delete", "PRAGMA secure_delete=ON"},
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p.pragma); err != nil {
			return fmt.Errorf("setting %s: %w", p.name, err)
		}
	}

	return nil
}

// CheckIntegrity performs a database integrity check.
func (db *DB) CheckIntegrity(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("running integrity check: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("scanning result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating results: %w", err)
	}

	// "ok" is the expected result for a healthy database
	if len(results) == 1 && results[0] == "ok" {
		return nil
	}

	return fmt.Errorf("integrity check failed: %v", results)
}

// Checkpoint forces a WAL checkpoint to sync all changes to the main database file.
func (db *DB) Checkpoint(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("WAL checkpoint: %w", err)
	}
	return nil
}

// Backup creates a backup of the database to the backup directory.
func (db *DB) Backup(ctx context.Context) (string, error) {
	if db.backupDir == "" {
		return "", errors.New("backup directory not configured")
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("vault-%s.db", timestamp)
	backupPath := filepath.Join(db.backupDir, backupName)

	// Checkpoint first to ensure WAL is flushed
	if err := db.Checkpoint(ctx); err != nil {
		slog.Warn("checkpoint before backup failed", "error", err)
	}

	// Use SQLite backup API via VACUUM INTO
	_, err := db.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}

	slog.Info("database backup created", "path", backupPath)

	// Clean up old backups
	if db.config.BackupRetentionDays > 0 {
		go db.cleanOldBackups()
	}

	return backupPath, nil
}

// cleanOldBackups removes backups older than the retention period.
func (db *DB) cleanOldBackups() {
	if db.backupDir == "" {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -db.config.BackupRetentionDays)

	entries, err := os.ReadDir(db.backupDir)
	if err != nil {
		slog.Warn("reading backup directory", "error", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(db.backupDir, entry.Name())
			if err := os.Remove(path); err != nil {
				slog.Warn("removing old backup", "path", path, "error", err)
			} else {
				slog.Debug("removed old backup", "path", path)
			}
		}
	}
}

// startBackupScheduler starts the background backup scheduler.
func (db *DB) startBackupScheduler() {
	interval := time.Duration(db.config.BackupIntervalHours) * time.Hour
	db.backupTicker = time.NewTicker(interval)
	db.backupDone = make(chan struct{})

	go func() {
		for {
			select {
			case <-db.backupTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if _, err := db.Backup(ctx); err != nil {
					slog.Error("scheduled backup failed", "error", err)
				}
				cancel()
			case <-db.backupDone:
				return
			}
		}
	}()
}

// Close gracefully closes the database connection.
// It ensures all pending transactions are complete and performs a final WAL checkpoint.
func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil
	}
	db.closed = true
	close(db.closeChan)
	db.mu.Unlock()

	// Stop backup scheduler
	if db.backupTicker != nil {
		db.backupTicker.Stop()
		close(db.backupDone)
	}

	// Final WAL checkpoint
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Checkpoint(ctx); err != nil {
		slog.Warn("final checkpoint failed", "error", err)
	}

	// Close the database
	if err := db.DB.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}

	slog.Info("database closed gracefully")
	return nil
}

// IsClosed returns true if the database has been closed.
func (db *DB) IsClosed() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.closed
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// BeginTx starts a transaction with the given options.
// If the database is closed, it returns an error.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return nil, errors.New("database is closed")
	}
	db.mu.RUnlock()

	return db.DB.BeginTx(ctx, opts)
}

// WithTransaction executes a function within a transaction.
// The transaction is committed if the function returns nil, otherwise rolled back.
func (db *DB) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rolling back after error %v: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// HealthCheck performs a basic health check on the database.
func (db *DB) HealthCheck(ctx context.Context) error {
	if db.IsClosed() {
		return errors.New("database is closed")
	}

	// Simple query to verify connection
	var result int
	err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check query: %w", err)
	}

	if result != 1 {
		return errors.New("unexpected health check result")
	}

	return nil
}

// GetStats returns database statistics.
type Stats struct {
	Path          string
	SizeBytes     int64
	WALSizeBytes  int64
	PageCount     int64
	FreePageCount int64
	PageSize      int64
	SchemaVersion int64
	JournalMode   string
}

// GetStats retrieves current database statistics.
func (db *DB) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{Path: db.path}

	// Get file sizes
	if info, err := os.Stat(db.path); err == nil {
		stats.SizeBytes = info.Size()
	}

	walPath := db.path + "-wal"
	if info, err := os.Stat(walPath); err == nil {
		stats.WALSizeBytes = info.Size()
	}

	// Get page statistics
	queries := []struct {
		pragma string
		dest   *int64
	}{
		{"PRAGMA page_count", &stats.PageCount},
		{"PRAGMA freelist_count", &stats.FreePageCount},
		{"PRAGMA page_size", &stats.PageSize},
		{"PRAGMA schema_version", &stats.SchemaVersion},
	}

	for _, q := range queries {
		if err := db.QueryRowContext(ctx, q.pragma).Scan(q.dest); err != nil {
			slog.Warn("getting stat", "pragma", q.pragma, "error", err)
		}
	}

	// Get journal mode
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&stats.JournalMode); err != nil {
		slog.Warn("getting journal mode", "error", err)
	}

	return stats, nil
}
