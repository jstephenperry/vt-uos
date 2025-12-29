package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultConfigFileName is the standard configuration file name.
	DefaultConfigFileName = "vault.toml"

	// XDGConfigSubdir is the subdirectory under XDG_CONFIG_HOME for vtuos.
	XDGConfigSubdir = "vtuos"
)

// LoadError represents an error that occurred while loading configuration.
type LoadError struct {
	Path string
	Err  error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("loading config from %s: %v", e.Path, e.Err)
}

func (e *LoadError) Unwrap() error {
	return e.Err
}

// Load attempts to load configuration from multiple sources in order of precedence:
// 1. Explicit path (if provided)
// 2. XDG config path (~/.config/vtuos/vault.toml)
// 3. Current working directory (./vault.toml)
// 4. Default configuration (if createDefault is true)
//
// Returns the loaded configuration and the path it was loaded from.
func Load(explicitPath string, createDefault bool) (*Config, string, error) {
	// If explicit path provided, use only that
	if explicitPath != "" {
		cfg, err := loadFromFile(explicitPath)
		if err != nil {
			return nil, "", &LoadError{Path: explicitPath, Err: err}
		}
		return cfg, explicitPath, nil
	}

	// Try XDG config path first
	xdgPath := xdgConfigPath()
	if xdgPath != "" {
		if fileExists(xdgPath) {
			cfg, err := loadFromFile(xdgPath)
			if err != nil {
				return nil, "", &LoadError{Path: xdgPath, Err: err}
			}
			return cfg, xdgPath, nil
		}
	}

	// Try current working directory
	cwdPath := filepath.Join(".", DefaultConfigFileName)
	if fileExists(cwdPath) {
		cfg, err := loadFromFile(cwdPath)
		if err != nil {
			return nil, "", &LoadError{Path: cwdPath, Err: err}
		}
		return cfg, cwdPath, nil
	}

	// No config file found
	if !createDefault {
		return nil, "", errors.New("no configuration file found; searched: " + xdgPath + ", " + cwdPath)
	}

	// Create default configuration
	cfg := Default()

	// Determine where to write the default config
	defaultPath := cwdPath
	if xdgPath != "" {
		// Prefer XDG path if we can create the directory
		if err := os.MkdirAll(filepath.Dir(xdgPath), 0750); err == nil {
			defaultPath = xdgPath
		}
	}

	// Write default configuration
	if err := Save(cfg, defaultPath); err != nil {
		// Continue with in-memory default if we can't write
		return cfg, "", nil
	}

	return cfg, defaultPath, nil
}

// loadFromFile reads and parses a TOML configuration file.
func loadFromFile(path string) (*Config, error) {
	// Start with defaults so missing values get sensible defaults
	cfg := Default()

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Parse TOML
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parsing TOML: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// Save writes a configuration to a TOML file.
func Save(cfg *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
	}

	// Create or truncate file
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// Write header comment
	header := `# VT-UOS Configuration File
# Vault-Tec Unified Operating System
#
# This file was auto-generated. Edit as needed.
# See CLAUDE.md for full configuration documentation.

`
	if _, err := f.WriteString(header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Encode TOML
	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encoding TOML: %w", err)
	}

	return nil
}

// xdgConfigPath returns the XDG-compliant config file path.
// Returns empty string if XDG_CONFIG_HOME is not set and HOME is not available.
func xdgConfigPath() string {
	// Check XDG_CONFIG_HOME first
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		return filepath.Join(xdgConfig, XDGConfigSubdir, DefaultConfigFileName)
	}

	// Fall back to ~/.config
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", XDGConfigSubdir, DefaultConfigFileName)
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ConfigPath returns the configuration file path that would be used.
// Useful for displaying to users.
func ConfigPath(explicitPath string) string {
	if explicitPath != "" {
		return explicitPath
	}

	xdgPath := xdgConfigPath()
	if xdgPath != "" && fileExists(xdgPath) {
		return xdgPath
	}

	cwdPath := filepath.Join(".", DefaultConfigFileName)
	if fileExists(cwdPath) {
		return cwdPath
	}

	// Return XDG path as the preferred location for new configs
	if xdgPath != "" {
		return xdgPath
	}

	return cwdPath
}

// EnsureDataDir creates the data directory for the database if needed.
// Returns the absolute path to the database file.
func EnsureDataDir(cfg *Config) (string, error) {
	dbPath := cfg.Database.Path

	// If absolute path, use as-is
	if filepath.IsAbs(dbPath) {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return "", fmt.Errorf("creating database directory: %w", err)
		}
		return dbPath, nil
	}

	// For relative paths, check if we should use XDG data directory
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			xdgData = filepath.Join(home, ".local", "share")
		}
	}

	if xdgData != "" {
		dataDir := filepath.Join(xdgData, XDGConfigSubdir)
		if err := os.MkdirAll(dataDir, 0750); err != nil {
			// Fall back to current directory
			return dbPath, nil
		}
		return filepath.Join(dataDir, dbPath), nil
	}

	// Use relative path in current directory
	return dbPath, nil
}

// EnsureLogDir creates the log directory if needed.
// Returns the absolute path to the log file.
func EnsureLogDir(cfg *Config) (string, error) {
	logPath := cfg.Logging.File

	// If empty, disable file logging
	if logPath == "" {
		return "", nil
	}

	// If absolute path, use as-is
	if filepath.IsAbs(logPath) {
		dir := filepath.Dir(logPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return "", fmt.Errorf("creating log directory: %w", err)
		}
		return logPath, nil
	}

	// Ensure relative log directory exists
	dir := filepath.Dir(logPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return "", fmt.Errorf("creating log directory: %w", err)
		}
	}

	return logPath, nil
}

// BackupDir returns the directory for database backups.
func BackupDir(cfg *Config) (string, error) {
	dbPath := cfg.Database.Path

	// Put backups next to the database
	var backupDir string
	if filepath.IsAbs(dbPath) {
		backupDir = filepath.Join(filepath.Dir(dbPath), "backups")
	} else {
		// Check XDG data directory
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				xdgData = filepath.Join(home, ".local", "share")
			}
		}

		if xdgData != "" {
			backupDir = filepath.Join(xdgData, XDGConfigSubdir, "backups")
		} else {
			backupDir = "backups"
		}
	}

	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	return backupDir, nil
}
