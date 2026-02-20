package database

import (
	"database/sql"
	"fmt"

	"github.com/vtuos/vtuos/internal/config"
	_ "modernc.org/sqlite"
)

// NewInMemory creates an in-memory database for testing purposes.
// It enables foreign keys but does not run migrations or enable WAL mode.
func NewInMemory() (*DB, error) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	return &DB{
		DB:        sqlDB,
		path:      ":memory:",
		config:    &config.DatabaseConfig{},
		closeChan: make(chan struct{}),
	}, nil
}
