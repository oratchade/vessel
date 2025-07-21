package db

import (
	"database/sql"
	"fmt"
	"time"

	// Import the MSSQL driver
	_ "github.com/denisenkom/go-mssqldb"
)

// DBConfig holds configuration for connecting to a MSSQL database.
type MSSQLConfig struct {
	User              string        // Username for authentication
	Password          string        // Password for authentication
	Host              string        // Hostname or IP address
	Port              uint16        // Port number
	Database          string        // Database name
	Encrypt           string        // Encryption mode (disable, true, false)
	TrustServerCert   bool          // Trust server certificate
	ConnectionTimeout time.Duration // Connection timeout
	ReadTimeout       time.Duration // Read timeout
	WriteTimeout      time.Duration // Write timeout
	MaxOpenConns      int           // Maximum number of open connections
	MaxIdleConns      int           // Maximum number of idle connections
	ConnMaxLifetime   time.Duration // Maximum lifetime of a connection
}

// Driver returns the name of the database driver to use for this configuration.
//
// string: The name of the database driver to use for this configuration.
func (cfg MSSQLConfig) Driver() string {
	return DriverMSSQL
}

// DSN returns the Data Source Name (DSN) for connecting to the MSSQL database.
//
// This DSN includes the following options:
//
// * user: the username for authentication
// * password: the password for authentication
// * host: the hostname or IP address of the MSSQL server
// * port: the port number to use for the connection
// * database: the database name
// * encrypt: the encryption mode (disable, true, false)
// * trustservercertificate: whether to trust the server certificate
// * connection timeout: the maximum time to wait for a connection to be established
// * read timeout: the maximum time to wait for a read operation to complete
// * write timeout: the maximum time to wait for a write operation to complete
func (cfg MSSQLConfig) DSN() string {
	auth := fmt.Sprintf("%s:%s@%s:%d", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	encryption := fmt.Sprintf("encrypt=%s&trustservercertificate=%t", cfg.Encrypt, cfg.TrustServerCert)
	timeout := fmt.Sprintf(
		"connection+timeout=%d&read+timeout=%s&write+timeout=%s",
		int(cfg.ConnectionTimeout.Seconds()), cfg.ReadTimeout, cfg.WriteTimeout,
	)
	return fmt.Sprintf("sqlserver://%s?database=%s&%s&%s", auth, cfg.Database, encryption, timeout)
}

type MSSQL struct {
	DB *sql.DB
}

// NewMSSQL initializes a new MSSQL connection using the provided config.
func NewMSSQL(cfg MSSQLConfig) (*MSSQL, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MSSQL connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MSSQL: %w", err)
	}

	return &MSSQL{DB: db}, nil
}

// Close closes the MSSQL database connection.
func (m *MSSQL) Close() {
	if m.DB == nil {
		return
	}
	_ = m.DB.Close()
}
