// VT-UOS: Vault-Tec Unified Operating System
//
// A mission-critical vault population management and operations system
// designed for multi-generational underground survival.
//
// The single binary runs in three modes:
//
//	vtuos                 local operator console (TUI + in-process control core)
//	vtuos serve           master server (authoritative core + API + web console)
//	vtuos connect <addr>  client display/operator terminal for a master
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/vtuos/vtuos/internal/client"
	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/database/seed"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/server"
	"github.com/vtuos/vtuos/internal/simulation"
	"github.com/vtuos/vtuos/internal/tui"
	"github.com/vtuos/vtuos/internal/util"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Dispatch subcommands before global flag parsing.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			os.Exit(serveMain(os.Args[2:]))
		case "connect":
			os.Exit(connectMain(os.Args[2:]))
		}
	}
	os.Exit(localMain(os.Args[1:]))
}

// signalContext returns a context cancelled on SIGINT/SIGTERM with a hard exit
// backstop if shutdown stalls.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		signal.Stop(sigChan) // deregister; the goroutine exits after cancelling
		slog.Info("received shutdown signal", "signal", sig)
		cancel()
		time.AfterFunc(10*time.Second, func() {
			slog.Error("forced shutdown after timeout")
			os.Exit(1)
		})
	}()
	return ctx, cancel
}

// ----- Local operator console -----

func localMain(args []string) int {
	fs := flag.NewFlagSet("vtuos", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to configuration file")
	migrateOnly := fs.Bool("migrate-only", false, "Run migrations and exit")
	seedData := fs.Bool("seed", false, "Generate seed data")
	showVersion := fs.Bool("version", false, "Show version and exit")
	debugMode := fs.Bool("debug", false, "Enable debug logging")
	restorePath := fs.String("restore", "", "Restore the vault database from a snapshot file before starting")
	_ = fs.Parse(args)

	if *showVersion {
		fmt.Printf("VT-UOS version %s (built %s)\n", Version, BuildTime)
		return 0
	}

	ctx, cancel := signalContext()
	defer cancel()

	if err := runLocal(ctx, *configPath, *migrateOnly, *seedData, *debugMode, *restorePath); err != nil {
		slog.Error("application error", "error", err)
		return 1
	}
	return 0
}

func runLocal(ctx context.Context, configPath string, migrateOnly, seedData, debugMode bool, restorePath string) error {
	cfg, db, closeFn, err := prepare(ctx, configPath, debugMode, restorePath)
	if err != nil {
		return err
	}
	defer closeFn()

	if migrateOnly {
		slog.Info("migrations complete, exiting")
		return nil
	}
	if seedData {
		return runSeed(ctx, cfg, db)
	}

	clock := newClock(cfg)
	engine := simulation.New(db, cfg, clock)
	if err := engine.Start(ctx); err != nil {
		slog.Warn("simulation control core failed to start", "error", err)
	}
	defer stopEngine(engine)

	tui.Version = Version
	tui.BuildTime = BuildTime
	slog.Info("starting local console", "vault", cfg.Vault.Designation, "simulation", cfg.Simulation.Enabled)
	if err := tui.RunWithEngine(ctx, db, cfg, clock, engine); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	slog.Info("VT-UOS shutdown complete")
	return nil
}

// ----- Master server -----

func serveMain(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to configuration file")
	debugMode := fs.Bool("debug", false, "Enable debug logging")
	listen := fs.String("listen", "", "Override the server listen address (e.g. :8080)")
	token := fs.String("token", "", "Override the operator admin token")
	restorePath := fs.String("restore", "", "Restore the vault database from a snapshot file before starting")
	_ = fs.Parse(args)

	ctx, cancel := signalContext()
	defer cancel()

	cfg, db, closeFn, err := prepare(ctx, *configPath, *debugMode, *restorePath)
	if err != nil {
		slog.Error("serve setup failed", "error", err)
		return 1
	}
	defer closeFn()

	if *listen != "" {
		cfg.Server.Listen = *listen
	}
	if *token != "" {
		cfg.Server.AdminToken = *token
	}

	clock := newClock(cfg)
	engine := simulation.New(db, cfg, clock)
	if err := engine.Start(ctx); err != nil {
		slog.Warn("simulation control core failed to start", "error", err)
	}
	defer stopEngine(engine)

	srv, err := server.New(engine, cfg)
	if err != nil {
		slog.Error("creating server failed", "error", err)
		return 1
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			slog.Error("server stopped", "error", err)
			return 1
		}
	}

	shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_ = srv.Shutdown(shutCtx)
	slog.Info("master server shutdown complete")
	return 0
}

// ----- Client terminal -----

func connectMain(args []string) int {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to configuration file")
	name := fs.String("name", "", "Terminal display name reported to the master")
	kind := fs.String("kind", "", "Terminal kind: display or operator")
	debugMode := fs.Bool("debug", false, "Enable debug logging")
	_ = fs.Parse(args)

	addr := fs.Arg(0)
	if addr == "" {
		fmt.Fprintln(os.Stderr, "usage: vtuos connect [flags] <master-address>")
		return 2
	}

	cfg, _, err := config.Load(*configPath, true)
	if err != nil {
		cfg = config.Default()
	}
	closeLog := setupClientLogging(cfg, *debugMode)
	defer closeLog()

	ctx, cancel := signalContext()
	defer cancel()

	terminalName := firstNonEmpty(*name, cfg.Client.Name, "vtuos-terminal")
	terminalKind := protocol.ClientDisplay
	if strings.EqualFold(*kind, "operator") || strings.EqualFold(cfg.Client.Kind, "operator") {
		terminalKind = protocol.ClientOperator
	}

	conn := client.NewConn(addr, terminalName, terminalKind, Version)
	if err := client.Run(ctx, conn, cfg); err != nil {
		slog.Error("client terminal error", "error", err)
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// ----- Shared setup -----

// prepare loads configuration, configures logging, opens (optionally restoring
// and recovering) the database, and applies migrations. The returned closeFn
// releases the database and any log file.
func prepare(ctx context.Context, configPath string, debug bool, restorePath string) (*config.Config, *database.DB, func(), error) {
	cfg, cfgPath, err := config.Load(configPath, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading configuration: %w", err)
	}

	logCloser, err := setupLogging(cfg, debug)
	if err != nil {
		return nil, nil, nil, err
	}

	slog.Info("VT-UOS starting", "version", Version, "build_time", BuildTime, "config_path", cfgPath)

	dbPath, err := config.EnsureDataDir(cfg)
	if err != nil {
		logCloser()
		return nil, nil, nil, fmt.Errorf("ensuring data directory: %w", err)
	}

	if restorePath != "" {
		if err := restoreDatabase(restorePath, dbPath); err != nil {
			logCloser()
			return nil, nil, nil, fmt.Errorf("restoring database from snapshot: %w", err)
		}
		slog.Info("vault database restored from snapshot", "from", restorePath, "to", dbPath)
	}

	backupDir, err := config.BackupDir(cfg)
	if err != nil {
		slog.Warn("failed to create backup directory", "error", err)
		backupDir = ""
	}

	if _, err := os.Stat(dbPath); err == nil {
		report, err := database.AttemptRecovery(dbPath, backupDir)
		if err != nil {
			logCloser()
			return nil, nil, nil, fmt.Errorf("database recovery failed: %w", err)
		}
		switch report.Result {
		case database.RecoveryFromBackup:
			slog.Warn("database restored from backup", "backup", report.BackupUsed)
		case database.RecoverySuccess:
			slog.Debug("database integrity verified")
		}
	}

	db, err := database.Open(dbPath, &cfg.Database, backupDir)
	if err != nil {
		logCloser()
		return nil, nil, nil, fmt.Errorf("opening database: %w", err)
	}

	migrator, err := database.NewMigrator(db)
	if err != nil {
		db.Close()
		logCloser()
		return nil, nil, nil, fmt.Errorf("creating migrator: %w", err)
	}
	result, err := migrator.MigrateUp(ctx)
	if err != nil {
		db.Close()
		logCloser()
		return nil, nil, nil, fmt.Errorf("running migrations: %w", err)
	}
	if len(result.Applied) > 0 {
		slog.Info("applied migrations", "count", len(result.Applied), "to_version", result.TargetVersion)
	}

	closeFn := func() {
		slog.Info("closing database")
		if err := db.Close(); err != nil {
			slog.Error("error closing database", "error", err)
		}
		logCloser()
	}
	return cfg, db, closeFn, nil
}

// setupLogging configures slog and returns a closer for any opened log file.
func setupLogging(cfg *config.Config, debug bool) (func(), error) {
	level := logLevel(cfg, debug)
	logPath, err := config.EnsureLogDir(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			return nil, fmt.Errorf("opening log file: %w", err)
		}
		slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})))
		return func() { f.Close() }, nil
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	return func() {}, nil
}

// setupClientLogging keeps client logs off the terminal screen: it logs to the
// configured file when present, and otherwise discards logs so the TUI is clean.
func setupClientLogging(cfg *config.Config, debug bool) func() {
	level := logLevel(cfg, debug)
	if logPath, err := config.EnsureLogDir(cfg); err == nil && logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640); err == nil {
			slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})))
			return func() { f.Close() }
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: level})))
	return func() {}
}

func logLevel(cfg *config.Config, debug bool) slog.Level {
	if debug {
		return slog.LevelDebug
	}
	switch cfg.Logging.Level {
	case config.LogLevelDebug:
		return slog.LevelDebug
	case config.LogLevelWarn:
		return slog.LevelWarn
	case config.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newClock(cfg *config.Config) *util.VaultClock {
	startTime, err := cfg.Simulation.StartDateTime()
	if err != nil {
		startTime = time.Now()
	}
	clock := util.NewVaultClock(startTime, cfg.Simulation.TimeScale)
	if !cfg.Simulation.Enabled {
		clock.Pause()
	}
	return clock
}

func stopEngine(engine *simulation.Engine) {
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := engine.Stop(stopCtx); err != nil {
		slog.Warn("error stopping simulation control core", "error", err)
	}
}

func runSeed(ctx context.Context, cfg *config.Config, db *database.DB) error {
	slog.Info("generating seed data", "vault", cfg.Vault.Number)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM residents").Scan(&count); err == nil && count > 0 {
		slog.Warn("database already contains residents, skipping seed generation", "count", count)
		return nil
	}

	sealDate, err := cfg.Simulation.StartDateTime()
	if err != nil {
		sealDate = time.Date(2077, 10, 23, 9, 47, 0, 0, time.UTC)
	}

	generator := seed.NewGenerator(db.DB, seed.Config{
		VaultNumber:      cfg.Vault.Number,
		SealDate:         sealDate,
		TargetPopulation: cfg.Vault.DesignedCapacity,
		FamilyHouseholds: 100,
		SingleHouseholds: 80,
		RandomSeed:       2077,
	})
	if err := generator.Generate(ctx); err != nil {
		return fmt.Errorf("generating seed data: %w", err)
	}
	slog.Info("seed data generation complete")
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// restoreDatabase copies a snapshot file over the live database path, removing
// any stale WAL/SHM sidecar files so the restored database opens cleanly.
func restoreDatabase(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening snapshot: %w", err)
	}
	defer in.Close()

	for _, sidecar := range []string{dst + "-wal", dst + "-shm"} {
		_ = os.Remove(sidecar)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("opening destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying snapshot: %w", err)
	}
	return out.Sync()
}
