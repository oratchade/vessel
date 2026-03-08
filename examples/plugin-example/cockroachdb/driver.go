package cockroachdb

import (
	"context"
	"fmt"

	db "tounilab.com/db-connector/db/v1"
	"tounilab.com/db-connector/db/v1/plugin"
)

// Config represents CockroachDB connection configuration.
// It implements db.DBConfig interface.
type Config struct {
	Host     string
	Port     uint16
	User     string
	Password string
	Database string
	SSLMode  string
}

// Driver returns the driver name.
func (c *Config) Driver() string {
	return "cockroachdb"
}

// DSN returns the connection string in PostgreSQL format.
// CockroachDB is wire-compatible with PostgreSQL.
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// Factory implements plugin.DriverFactory interface.
type Factory struct{}

// Name returns the driver name.
func (f *Factory) Name() string {
	return "cockroachdb"
}

// Create creates a new CockroachDB driver instance.
// It converts the CockroachDB config to PostgreSQL config and reuses the built-in PostgreSQL driver.
func (f *Factory) Create(ctx context.Context, cfg interface{}) (interface{}, error) {
	crdbCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("expected *cockroachdb.Config, got %T", cfg)
	}

	// Validate config
	if crdbCfg.Host == "" || crdbCfg.User == "" || crdbCfg.Database == "" {
		return nil, fmt.Errorf("CockroachDB config missing required fields: host, user, or database")
	}

	// Convert CockroachDB config to PostgreSQL config
	// CockroachDB uses PostgreSQL wire protocol, so we can reuse the PostgreSQL driver
	pgCfg := &db.PostgresConfig{
		Host:     crdbCfg.Host,
		Port:     crdbCfg.Port,
		User:     crdbCfg.User,
		Password: crdbCfg.Password,
		Database: crdbCfg.Database,
		SSLMode:  crdbCfg.SSLMode,
	}

	// Reuse the PostgreSQL driver
	//nolint:contextcheck
	db, err := db.PostgresCfgToDB(pgCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL driver for CockroachDB: %w", err)
	}
	return db, nil
}

//nolint:gochecknoinits
func init() {
	plugin.MustRegister(&Factory{})
}
