package db

import (
	"database/sql"
	"fmt"
	"time"

	// Import the MySQL driver
	_ "github.com/go-sql-driver/mysql"
)

// DBConfig holds configuration for connecting to a MySQL database.
type MysqlConfig struct {
	User            string        // Username for authentication
	Password        string        // Password for authentication
	Host            string        // Hostname or IP address
	Port            uint16        // Port number
	Database        string        // Database name
	Charset         string        // Character set (e.g., utf8mb4)
	ParseTime       bool          // Parse time values to time.Time
	Loc             string        // Time zone location (e.g., Local, UTC)
	Timeout         time.Duration // Connection timeout
	ReadTimeout     time.Duration // Read timeout
	WriteTimeout    time.Duration // Write timeout
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
}

// Driver returns the name of the database driver to use for this configuration.
//
// string: The name of the database driver to use for this configuration.
func (cfg MysqlConfig) Driver() string {
	return DriverMySQL
}

// DSN returns the Data Source Name (DSN) for connecting to the MySQL database.
//
// The DSN includes the following options:
//
// * user: the username for authentication
// * password: the password for authentication
// * host: the hostname or IP address of the MySQL server
// * port: the port number to use for the connection
// * dbname: the database name
// * charset: the character set to use (e.g., utf8mb4)
// * parseTime: whether to parse time values to time.Time
// * loc: the time zone location (e.g., Local, UTC)
// * timeout: the connection timeout
// * readTimeout: the read timeout
// * writeTimeout: the write timeout
func (cfg MysqlConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s&timeout=%s&readTimeout=%s&writeTimeout=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
		cfg.Charset, cfg.ParseTime, cfg.Loc,
		cfg.Timeout.String(), cfg.ReadTimeout.String(), cfg.WriteTimeout.String(),
	)
}

type MySQL struct {
	DB *sql.DB
}

// NewMySQL initializes a new MySQL connection using the provided config.
func NewMySQL(cfg MysqlConfig) (*MySQL, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Optional: Ping to verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	return &MySQL{DB: db}, nil
}

// Close closes the MySQL database connection.
func (m *MySQL) Close() {
	if m.DB == nil {
		return
	}
	_ = m.DB.Close()
}
