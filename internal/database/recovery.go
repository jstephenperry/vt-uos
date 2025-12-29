package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// RecoveryResult indicates the outcome of a recovery attempt.
type RecoveryResult int

const (
	// RecoverySuccess means the database was healthy or recovered in place.
	RecoverySuccess RecoveryResult = iota
	// RecoveryFromBackup means the database was restored from a backup.
	RecoveryFromBackup
	// RecoveryFailed means all recovery attempts failed.
	RecoveryFailed
)

func (r RecoveryResult) String() string {
	switch r {
	case RecoverySuccess:
		return "success"
	case RecoveryFromBackup:
		return "restored_from_backup"
	case RecoveryFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// RecoveryReport contains details about a recovery attempt.
type RecoveryReport struct {
	Result         RecoveryResult
	DatabasePath   string
	BackupUsed     string
	IntegrityCheck string
	WALRecovered   bool
	Error          error
	Steps          []RecoveryStep
}

// RecoveryStep represents a single step in the recovery process.
type RecoveryStep struct {
	Name      string
	Succeeded bool
	Message   string
	Duration  time.Duration
}

// AttemptRecovery performs a phased recovery of a potentially corrupted database.
// Phase 1: Integrity check
// Phase 2: WAL recovery
// Phase 3: Backup restoration
// Phase 4: Failure with diagnostics
func AttemptRecovery(dbPath string, backupDir string) (*RecoveryReport, error) {
	report := &RecoveryReport{
		DatabasePath: dbPath,
	}

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// No database file - this is fine for first run
		report.Result = RecoverySuccess
		report.Steps = append(report.Steps, RecoveryStep{
			Name:      "check_exists",
			Succeeded: true,
			Message:   "database does not exist (first run)",
		})
		return report, nil
	}

	// Phase 1: Integrity Check
	step1 := runRecoveryStep("integrity_check", func() (string, error) {
		return checkDatabaseIntegrity(dbPath)
	})
	report.Steps = append(report.Steps, step1)

	if step1.Succeeded {
		report.Result = RecoverySuccess
		report.IntegrityCheck = step1.Message
		slog.Info("database integrity check passed", "path", dbPath)
		return report, nil
	}

	slog.Warn("database integrity check failed", "path", dbPath, "error", step1.Message)

	// Phase 2: WAL Recovery
	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); err == nil {
		step2 := runRecoveryStep("wal_recovery", func() (string, error) {
			return attemptWALRecovery(dbPath)
		})
		report.Steps = append(report.Steps, step2)

		if step2.Succeeded {
			// Re-check integrity after WAL recovery
			step2b := runRecoveryStep("post_wal_integrity", func() (string, error) {
				return checkDatabaseIntegrity(dbPath)
			})
			report.Steps = append(report.Steps, step2b)

			if step2b.Succeeded {
				report.Result = RecoverySuccess
				report.WALRecovered = true
				slog.Info("database recovered via WAL replay", "path", dbPath)
				return report, nil
			}
		}
	}

	// Phase 3: Backup Restoration
	if backupDir != "" {
		step3 := runRecoveryStep("backup_restoration", func() (string, error) {
			return restoreFromBackup(dbPath, backupDir)
		})
		report.Steps = append(report.Steps, step3)

		if step3.Succeeded {
			report.Result = RecoveryFromBackup
			report.BackupUsed = step3.Message
			slog.Info("database restored from backup",
				"path", dbPath,
				"backup", step3.Message,
			)
			return report, nil
		}
	}

	// Phase 4: Failed
	report.Result = RecoveryFailed
	report.Error = errors.New("all recovery attempts failed")

	slog.Error("database recovery failed",
		"path", dbPath,
		"steps", len(report.Steps),
	)

	return report, report.Error
}

// runRecoveryStep executes a recovery step and records the result.
func runRecoveryStep(name string, fn func() (string, error)) RecoveryStep {
	start := time.Now()
	msg, err := fn()
	duration := time.Since(start)

	step := RecoveryStep{
		Name:     name,
		Duration: duration,
	}

	if err != nil {
		step.Succeeded = false
		step.Message = err.Error()
	} else {
		step.Succeeded = true
		step.Message = msg
	}

	return step
}

// checkDatabaseIntegrity runs SQLite's integrity check.
func checkDatabaseIntegrity(dbPath string) (string, error) {
	connStr := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return "", fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return "", fmt.Errorf("running integrity check: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return "", fmt.Errorf("scanning result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterating results: %w", err)
	}

	if len(results) == 1 && results[0] == "ok" {
		return "ok", nil
	}

	return "", fmt.Errorf("integrity check failed: %s", strings.Join(results, "; "))
}

// attemptWALRecovery tries to recover the database by replaying the WAL file.
func attemptWALRecovery(dbPath string) (string, error) {
	// Open in normal mode to trigger WAL recovery
	connStr := fmt.Sprintf("file:%s?_txlock=immediate", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return "", fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Force WAL checkpoint
	_, err = db.ExecContext(ctx, "PRAGMA wal_checkpoint(RESTART)")
	if err != nil {
		return "", fmt.Errorf("WAL checkpoint: %w", err)
	}

	return "WAL checkpoint complete", nil
}

// restoreFromBackup finds the most recent valid backup and restores it.
func restoreFromBackup(dbPath string, backupDir string) (string, error) {
	// Find all backup files
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", fmt.Errorf("reading backup directory: %w", err)
	}

	// Filter and sort backup files by modification time (newest first)
	type backupFile struct {
		path    string
		modTime time.Time
	}

	var backups []backupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, backupFile{
			path:    filepath.Join(backupDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	if len(backups) == 0 {
		return "", errors.New("no backup files found")
	}

	// Sort by modification time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.After(backups[j].modTime)
	})

	// Try each backup until we find a valid one
	for _, backup := range backups {
		slog.Debug("trying backup", "path", backup.path)

		// Check backup integrity
		result, err := checkDatabaseIntegrity(backup.path)
		if err != nil {
			slog.Debug("backup failed integrity check",
				"path", backup.path,
				"error", err,
			)
			continue
		}

		if result != "ok" {
			continue
		}

		// Backup the corrupted database before replacing
		corruptedPath := dbPath + ".corrupted." + time.Now().Format("20060102-150405")
		if err := moveFile(dbPath, corruptedPath); err != nil {
			slog.Warn("failed to preserve corrupted database",
				"path", dbPath,
				"error", err,
			)
		}

		// Remove WAL and SHM files if they exist
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")

		// Copy backup to database path
		if err := copyFile(backup.path, dbPath); err != nil {
			return "", fmt.Errorf("copying backup: %w", err)
		}

		return backup.path, nil
	}

	return "", errors.New("no valid backup found")
}

// moveFile moves a file from src to dst.
func moveFile(src, dst string) error {
	// Try rename first (fastest, same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + delete
	if err := copyFile(src, dst); err != nil {
		return err
	}

	return os.Remove(src)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}

	// Sync to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("syncing destination: %w", err)
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	return nil
}

// DiagnoseDatabase provides detailed diagnostics about a database file.
func DiagnoseDatabase(dbPath string) (*DatabaseDiagnostics, error) {
	diag := &DatabaseDiagnostics{
		Path: dbPath,
	}

	// Check main database file
	if info, err := os.Stat(dbPath); err == nil {
		diag.Exists = true
		diag.SizeBytes = info.Size()
		diag.ModTime = info.ModTime()
		diag.Permissions = info.Mode().String()
	} else if os.IsNotExist(err) {
		diag.Exists = false
		return diag, nil
	} else {
		return nil, fmt.Errorf("stating database: %w", err)
	}

	// Check WAL file
	walPath := dbPath + "-wal"
	if info, err := os.Stat(walPath); err == nil {
		diag.WALExists = true
		diag.WALSizeBytes = info.Size()
	}

	// Check SHM file
	shmPath := dbPath + "-shm"
	if _, err := os.Stat(shmPath); err == nil {
		diag.SHMExists = true
	}

	// Try to open and get header info
	connStr := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		diag.OpenError = err.Error()
		return diag, nil
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get SQLite version
	db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&diag.SQLiteVersion)

	// Get schema version
	db.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&diag.SchemaVersion)

	// Get user version
	db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&diag.UserVersion)

	// Get journal mode
	db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&diag.JournalMode)

	// Get page size
	db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&diag.PageSize)

	// Get page count
	db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&diag.PageCount)

	// Get free pages
	db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&diag.FreelistCount)

	// Quick integrity check
	var integrityResult string
	db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrityResult)
	diag.QuickCheck = integrityResult

	return diag, nil
}

// DatabaseDiagnostics contains detailed information about a database file.
type DatabaseDiagnostics struct {
	Path          string
	Exists        bool
	SizeBytes     int64
	ModTime       time.Time
	Permissions   string
	WALExists     bool
	WALSizeBytes  int64
	SHMExists     bool
	OpenError     string
	SQLiteVersion string
	SchemaVersion int
	UserVersion   int
	JournalMode   string
	PageSize      int
	PageCount     int
	FreelistCount int
	QuickCheck    string
}
