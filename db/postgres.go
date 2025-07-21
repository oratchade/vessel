package db

import (
	"context"
	"fmt"
	"time"

	// Import the PostgreSQL driver
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBConfig holds all configuration options for connecting to a PostgreSQL database.
type PostgresConfig struct {
	Host            string        // Database server hostname or IP
	Port            uint16        // Database server port
	User            string        // Username for authentication
	Password        string        // Password for authentication
	Database        string        // Database name
	SSLMode         string        // SSL mode (disable, require, verify-ca, verify-full)
	ConnectTimeout  time.Duration // Connection timeout
	PoolMaxConns    int32         // Maximum number of connections in the pool
	PoolMinConns    int32         // Minimum number of connections in the pool
	PoolMaxConnIdle time.Duration // Maximum idle time for a connection
	PoolMaxConnLife time.Duration // Maximum lifetime of a connection
	ApplicationName string        // Application name for logging/tracking
	SearchPath      string        // PostgreSQL schema search path
	LogLevel        string        // Logging level (debug, info, warn, error)
}

// Driver returns the name of the database driver to use for this configuration.
//
//	string: The name of the database driver to use for this configuration.
func (cfg PostgresConfig) Driver() string {
	return DriverPostgres
}

// DSN returns the Data Source Name (DSN) for connecting to the PostgreSQL database.
//
// This DSN includes the following options:
//
// * user: the username for authentication
// * password: the password for authentication
// * host: the hostname or IP address of the PostgreSQL server
// * port: the port number to use for the connection
// * dbname: the database name
// * sslmode: the SSL mode to use (disable, require, verify-ca, verify-full)
// * application_name: the application name for logging/tracking
// * search_path: the PostgreSQL schema search path
//
// See the pgx documentation for more information on the available options.
func (cfg PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"user=%s password=%s host=%s port=%d dbname=%s sslmode=%s application_name=%s search_path=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode, cfg.ApplicationName, cfg.SearchPath,
	)
}

type Postgres struct {
	Pool *pgxpool.Pool
}

// NewPostgres initializes a new Postgres connection pool using the provided config.
func NewPostgres(cfg PostgresConfig) (*Postgres, error) {
	dsn := cfg.DSN()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	poolConfig.MaxConns = cfg.PoolMaxConns
	poolConfig.MinConns = cfg.PoolMinConns
	poolConfig.MaxConnIdleTime = cfg.PoolMaxConnIdle
	poolConfig.MaxConnLifetime = cfg.PoolMaxConnLife

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &Postgres{Pool: pool}, nil
}

// Close closes the database connection pool.
func (pg *Postgres) Close() {
	if pg.Pool == nil {
		return
	}
	pg.Pool.Close()
}
