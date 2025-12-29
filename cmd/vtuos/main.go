// VT-UOS: Vault-Tec Unified Operating System
//
// A mission-critical vault population management and operations system
// designed for multi-generational underground survival.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/database/seed"
	"github.com/vtuos/vtuos/internal/tui"
	"github.com/vtuos/vtuos/internal/util"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		migrateOnly = flag.Bool("migrate-only", false, "Run migrations and exit")
		seedData    = flag.Bool("seed", false, "Generate seed data")
		showVersion = flag.Bool("version", false, "Show version and exit")
		debugMode   = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("VT-UOS version %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("received shutdown signal", "signal", sig)
		cancel()

		// Force exit after timeout
		time.AfterFunc(10*time.Second, func() {
			slog.Error("forced shutdown after timeout")
			os.Exit(1)
		})
	}()

	// Run the application
	if err := run(ctx, *configPath, *migrateOnly, *seedData, *debugMode); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string, migrateOnly, seedData, debugMode bool) error {
	// Load configuration
	cfg, cfgPath, err := config.Load(configPath, true)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Setup logging
	logLevel := slog.LevelInfo
	if debugMode {
		logLevel = slog.LevelDebug
	} else {
		switch cfg.Logging.Level {
		case config.LogLevelDebug:
			logLevel = slog.LevelDebug
		case config.LogLevelWarn:
			logLevel = slog.LevelWarn
		case config.LogLevelError:
			logLevel = slog.LevelError
		}
	}

	// Create log file if configured
	var logHandler slog.Handler
	logPath, err := config.EnsureLogDir(cfg)
	if err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer logFile.Close()

		logHandler = slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: logLevel,
		})
	} else {
		logHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	slog.Info("VT-UOS starting",
		"version", Version,
		"build_time", BuildTime,
		"config_path", cfgPath,
	)

	// Get database path
	dbPath, err := config.EnsureDataDir(cfg)
	if err != nil {
		return fmt.Errorf("ensuring data directory: %w", err)
	}

	// Get backup directory
	backupDir, err := config.BackupDir(cfg)
	if err != nil {
		slog.Warn("failed to create backup directory", "error", err)
		backupDir = ""
	}

	// Attempt database recovery if needed
	if _, err := os.Stat(dbPath); err == nil {
		report, err := database.AttemptRecovery(dbPath, backupDir)
		if err != nil {
			slog.Error("database recovery failed",
				"path", dbPath,
				"steps", len(report.Steps),
			)
			return fmt.Errorf("database recovery failed: %w", err)
		}

		switch report.Result {
		case database.RecoveryFromBackup:
			slog.Warn("database restored from backup",
				"backup", report.BackupUsed,
			)
		case database.RecoverySuccess:
			slog.Debug("database integrity verified")
		}
	}

	// Open database
	db, err := database.Open(dbPath, &cfg.Database, backupDir)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() {
		slog.Info("closing database")
		if err := db.Close(); err != nil {
			slog.Error("error closing database", "error", err)
		}
	}()

	// Run migrations
	migrator, err := database.NewMigrator(db)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	result, err := migrator.MigrateUp(ctx)
	if err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	if len(result.Applied) > 0 {
		slog.Info("applied migrations",
			"count", len(result.Applied),
			"to_version", result.TargetVersion,
		)
	}

	// Exit early if migrate-only mode
	if migrateOnly {
		slog.Info("migrations complete, exiting")
		return nil
	}

	// Generate seed data if requested
	if seedData {
		slog.Info("generating seed data", "vault", cfg.Vault.Number)

		// Check if data already exists
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM residents").Scan(&count); err == nil && count > 0 {
			slog.Warn("database already contains residents, skipping seed generation", "count", count)
			return nil
		}

		// Get seal date from config
		sealDate, err := cfg.Simulation.StartDateTime()
		if err != nil {
			sealDate = time.Date(2077, 10, 23, 9, 47, 0, 0, time.UTC)
		}

		seedCfg := seed.Config{
			VaultNumber:      cfg.Vault.Number,
			SealDate:         sealDate,
			TargetPopulation: cfg.Vault.DesignedCapacity,
			FamilyHouseholds: 100,
			SingleHouseholds: 80,
			RandomSeed:       2077,
		}

		generator := seed.NewGenerator(db.DB, seedCfg)
		if err := generator.Generate(ctx); err != nil {
			return fmt.Errorf("generating seed data: %w", err)
		}

		slog.Info("seed data generation complete")
		return nil
	}

	// Initialize vault clock
	startTime, err := cfg.Simulation.StartDateTime()
	if err != nil {
		startTime = time.Now()
	}
	clock := util.NewVaultClock(startTime, cfg.Simulation.TimeScale)

	if !cfg.Simulation.Enabled {
		clock.Pause()
	}

	// Set version info for TUI
	tui.Version = Version
	tui.BuildTime = BuildTime

	// Run TUI
	slog.Info("starting TUI",
		"vault", cfg.Vault.Designation,
		"simulation", cfg.Simulation.Enabled,
	)

	if err := tui.Run(ctx, db, cfg, clock); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	slog.Info("VT-UOS shutdown complete")
	return nil
}
