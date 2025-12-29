// Package config provides configuration management for VT-UOS.
// Configurations are loaded from TOML files with XDG-compliant paths.
package config

import (
	"errors"
	"fmt"
	"time"
)

// Config holds the complete application configuration.
type Config struct {
	Vault      VaultConfig      `toml:"vault"`
	Overseer   OverseerConfig   `toml:"overseer"`
	Experiment ExperimentConfig `toml:"experiment"`
	Simulation SimulationConfig `toml:"simulation"`
	Display    DisplayConfig    `toml:"display"`
	Logging    LoggingConfig    `toml:"logging"`
	Database   DatabaseConfig   `toml:"database"`
}

// VaultConfig contains vault identity and physical specifications.
type VaultConfig struct {
	Designation      string        `toml:"designation"`
	Number           int           `toml:"number"`
	Region           string        `toml:"region"`
	CommissionedDate string        `toml:"commissioned_date"`
	SealedDate       string        `toml:"sealed_date"`
	DesignedCapacity int           `toml:"designed_capacity"`
	VaultType        VaultType     `toml:"vault_type"`
	Location         VaultLocation `toml:"location"`
}

// VaultLocation specifies the physical location of the vault.
type VaultLocation struct {
	Latitude    float64 `toml:"latitude"`
	Longitude   float64 `toml:"longitude"`
	DepthMeters float64 `toml:"depth_meters"`
}

// VaultType indicates whether the vault is a control or experimental vault.
type VaultType string

const (
	VaultTypeControl      VaultType = "control"
	VaultTypeExperimental VaultType = "experimental"
)

// OverseerConfig contains initial overseer configuration.
type OverseerConfig struct {
	InitialOverseerID string `toml:"initial_overseer_id"`
}

// ExperimentConfig contains experimental protocol settings (if applicable).
type ExperimentConfig struct {
	Enabled        bool                     `toml:"enabled"`
	ProtocolID     string                   `toml:"protocol_id"`
	ProtocolName   string                   `toml:"protocol_name"`
	Classification ExperimentClassification `toml:"classification"`
}

// ExperimentClassification indicates the secrecy level of the experiment.
type ExperimentClassification string

const (
	ClassificationNone         ExperimentClassification = "NONE"
	ClassificationConfidential ExperimentClassification = "CONFIDENTIAL"
	ClassificationSecret       ExperimentClassification = "SECRET"
	ClassificationOverseerOnly ExperimentClassification = "OVERSEER_ONLY"
)

// SimulationConfig controls the time simulation engine.
type SimulationConfig struct {
	Enabled        bool              `toml:"enabled"`
	TimeScale      float64           `toml:"time_scale"`
	AutoEvents     bool              `toml:"auto_events"`
	EventFrequency EventFrequency    `toml:"event_frequency"`
	StartDate      string            `toml:"start_date"`
	Consumption    ConsumptionConfig `toml:"consumption"`
}

// ConsumptionConfig controls resource consumption variance.
type ConsumptionConfig struct {
	CalorieVariance     float64 `toml:"calorie_variance"`
	WaterVariance       float64 `toml:"water_variance"`
	EfficiencyDecayRate float64 `toml:"efficiency_decay_rate"`
}

// EventFrequency controls how often random events occur.
type EventFrequency string

const (
	EventFrequencyMinimal   EventFrequency = "minimal"
	EventFrequencyReduced   EventFrequency = "reduced"
	EventFrequencyNormal    EventFrequency = "normal"
	EventFrequencyIncreased EventFrequency = "increased"
	EventFrequencyChaotic   EventFrequency = "chaotic"
)

// DisplayConfig controls TUI appearance.
type DisplayConfig struct {
	ColorScheme ColorScheme `toml:"color_scheme"`
	ScanLines   bool        `toml:"scan_lines"`
	Flicker     bool        `toml:"flicker"`
	DateFormat  string      `toml:"date_format"`
	TimeFormat  string      `toml:"time_format"`
}

// ColorScheme defines the terminal color palette.
type ColorScheme string

const (
	ColorSchemeGreenPhosphor ColorScheme = "green_phosphor"
	ColorSchemeAmber         ColorScheme = "amber"
	ColorSchemeWhite         ColorScheme = "white"
)

// LoggingConfig controls application logging.
type LoggingConfig struct {
	Level      LogLevel `toml:"level"`
	File       string   `toml:"file"`
	MaxSizeMB  int      `toml:"max_size_mb"`
	MaxBackups int      `toml:"max_backups"`
}

// LogLevel defines logging verbosity.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// DatabaseConfig controls SQLite database settings.
type DatabaseConfig struct {
	Path                string `toml:"path"`
	BackupIntervalHours int    `toml:"backup_interval_hours"`
	BackupRetentionDays int    `toml:"backup_retention_days"`
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	var errs []error

	if err := c.Vault.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("vault: %w", err))
	}

	if err := c.Simulation.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("simulation: %w", err))
	}

	if err := c.Display.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("display: %w", err))
	}

	if err := c.Logging.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("logging: %w", err))
	}

	if err := c.Database.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("database: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Validate checks that the vault configuration is valid.
func (v *VaultConfig) Validate() error {
	var errs []error

	if v.Designation == "" {
		errs = append(errs, errors.New("designation is required"))
	}

	if v.Number < 1 || v.Number > 999 {
		errs = append(errs, errors.New("number must be between 1 and 999"))
	}

	if v.DesignedCapacity < 1 {
		errs = append(errs, errors.New("designed_capacity must be positive"))
	}

	if v.VaultType != VaultTypeControl && v.VaultType != VaultTypeExperimental {
		errs = append(errs, fmt.Errorf("invalid vault_type: %s", v.VaultType))
	}

	if v.SealedDate != "" {
		if _, err := time.Parse(time.RFC3339, v.SealedDate); err != nil {
			errs = append(errs, fmt.Errorf("invalid sealed_date format (expected RFC3339): %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Validate checks that the simulation configuration is valid.
func (s *SimulationConfig) Validate() error {
	var errs []error

	if s.TimeScale < 0 {
		errs = append(errs, errors.New("time_scale must be non-negative"))
	}

	validFrequencies := map[EventFrequency]bool{
		EventFrequencyMinimal:   true,
		EventFrequencyReduced:   true,
		EventFrequencyNormal:    true,
		EventFrequencyIncreased: true,
		EventFrequencyChaotic:   true,
	}

	if !validFrequencies[s.EventFrequency] && s.EventFrequency != "" {
		errs = append(errs, fmt.Errorf("invalid event_frequency: %s", s.EventFrequency))
	}

	if s.StartDate != "" {
		if _, err := time.Parse(time.RFC3339, s.StartDate); err != nil {
			errs = append(errs, fmt.Errorf("invalid start_date format (expected RFC3339): %w", err))
		}
	}

	if s.Consumption.CalorieVariance < 0 || s.Consumption.CalorieVariance > 1 {
		errs = append(errs, errors.New("calorie_variance must be between 0 and 1"))
	}

	if s.Consumption.WaterVariance < 0 || s.Consumption.WaterVariance > 1 {
		errs = append(errs, errors.New("water_variance must be between 0 and 1"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Validate checks that the display configuration is valid.
func (d *DisplayConfig) Validate() error {
	var errs []error

	validSchemes := map[ColorScheme]bool{
		ColorSchemeGreenPhosphor: true,
		ColorSchemeAmber:         true,
		ColorSchemeWhite:         true,
	}

	if !validSchemes[d.ColorScheme] && d.ColorScheme != "" {
		errs = append(errs, fmt.Errorf("invalid color_scheme: %s", d.ColorScheme))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Validate checks that the logging configuration is valid.
func (l *LoggingConfig) Validate() error {
	var errs []error

	validLevels := map[LogLevel]bool{
		LogLevelDebug: true,
		LogLevelInfo:  true,
		LogLevelWarn:  true,
		LogLevelError: true,
	}

	if !validLevels[l.Level] && l.Level != "" {
		errs = append(errs, fmt.Errorf("invalid log level: %s", l.Level))
	}

	if l.MaxSizeMB < 0 {
		errs = append(errs, errors.New("max_size_mb must be non-negative"))
	}

	if l.MaxBackups < 0 {
		errs = append(errs, errors.New("max_backups must be non-negative"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Validate checks that the database configuration is valid.
func (d *DatabaseConfig) Validate() error {
	var errs []error

	if d.Path == "" {
		errs = append(errs, errors.New("path is required"))
	}

	if d.BackupIntervalHours < 0 {
		errs = append(errs, errors.New("backup_interval_hours must be non-negative"))
	}

	if d.BackupRetentionDays < 0 {
		errs = append(errs, errors.New("backup_retention_days must be non-negative"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Default returns a configuration with sensible default values.
func Default() *Config {
	return &Config{
		Vault: VaultConfig{
			Designation:      "Vault 076",
			Number:           76,
			Region:           "Appalachia",
			CommissionedDate: "2076-10-23",
			SealedDate:       "2077-10-23T09:47:00Z",
			DesignedCapacity: 500,
			VaultType:        VaultTypeControl,
			Location: VaultLocation{
				Latitude:    39.6295,
				Longitude:   -79.9559,
				DepthMeters: 100,
			},
		},
		Overseer: OverseerConfig{
			InitialOverseerID: "",
		},
		Experiment: ExperimentConfig{
			Enabled:        false,
			ProtocolID:     "",
			ProtocolName:   "",
			Classification: ClassificationNone,
		},
		Simulation: SimulationConfig{
			Enabled:        true,
			TimeScale:      60.0,
			AutoEvents:     true,
			EventFrequency: EventFrequencyNormal,
			StartDate:      "2077-10-23T09:47:00Z",
			Consumption: ConsumptionConfig{
				CalorieVariance:     0.1,
				WaterVariance:       0.1,
				EfficiencyDecayRate: 0.001,
			},
		},
		Display: DisplayConfig{
			ColorScheme: ColorSchemeGreenPhosphor,
			ScanLines:   true,
			Flicker:     false,
			DateFormat:  "2006-01-02",
			TimeFormat:  "15:04:05",
		},
		Logging: LoggingConfig{
			Level:      LogLevelInfo,
			File:       "logs/vtuos.log",
			MaxSizeMB:  10,
			MaxBackups: 5,
		},
		Database: DatabaseConfig{
			Path:                "vault.db",
			BackupIntervalHours: 24,
			BackupRetentionDays: 30,
		},
	}
}

// SealedDateTime returns the vault's seal date as a time.Time.
func (v *VaultConfig) SealedDateTime() (time.Time, error) {
	if v.SealedDate == "" {
		return time.Time{}, errors.New("sealed_date is not set")
	}
	return time.Parse(time.RFC3339, v.SealedDate)
}

// StartDateTime returns the simulation start date as a time.Time.
func (s *SimulationConfig) StartDateTime() (time.Time, error) {
	if s.StartDate == "" {
		return time.Time{}, errors.New("start_date is not set")
	}
	return time.Parse(time.RFC3339, s.StartDate)
}
