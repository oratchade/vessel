package db

import (
	"database/sql"
	"fmt"
	"time"

	// Import the SQLLite driver
	_ "github.com/mattn/go-sqlite3"
)

// DBConfig holds configuration for connecting to a SQLLite database.
type SQLLiteConfig struct {
	FilePath        string        // Path to the SQLLite database file
	CacheMode       string        // Cache mode (shared, private)
	Mode            string        // Access mode (ro, rw, rwc, memory)
	ForeignKeys     bool          // Enable foreign key constraints
	BusyTimeout     time.Duration // Busy timeout duration
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
}

// Driver returns the driver name for SQLLite databases.

func (cfg SQLLiteConfig) Driver() string {
	return DriverSQLLite
}

// DSN returns the Data Source Name (DSN) for connecting to the SQLLite database.
// The DSN includes the following options:
//
// * file: the path to the SQLLite database file
// * cache: the cache mode (e.g., shared, private)
// * mode: the access mode (e.g., ro, rw, rwc, memory)
// * _foreign_keys: whether to enable foreign key constraints
// * _busy_timeout: the busy timeout duration in milliseconds
func (cfg SQLLiteConfig) DSN() string {
	return fmt.Sprintf(
		"file:%s?cache=%s&mode=%s&_foreign_keys=%t&_busy_timeout=%d",
		cfg.FilePath, cfg.CacheMode, cfg.Mode, cfg.ForeignKeys, int(cfg.BusyTimeout.Milliseconds()),
	)
}

type SQLLite struct {
	DB *sql.DB
}

// NewSQLLite initializes a new SQLLite connection using the provided config.
func NewSQLLite(cfg SQLLiteConfig) (*SQLLite, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("SQLLite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLLite connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLLite: %w", err)
	}

	return &SQLLite{DB: db}, nil
}

// Close closes the SQLLite database connection.
func (s *SQLLite) Close() {
	if s.DB == nil {
		return
	}
	_ = s.DB.Close()
}
