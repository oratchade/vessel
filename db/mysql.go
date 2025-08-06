package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the MySQL driver
	_ "github.com/go-sql-driver/mysql"

	"tounilab.com/db-connector/query/builder"
	sqldialect "tounilab.com/db-connector/query/builder/sqlDialect"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
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
	return definition.DriverMySQL
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
	DB           *sql.DB
	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
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

	return &MySQL{
		DB:           db,
		queryBuilder: builder.NewMySQLQueryBuilder(sqldialect.MySQLDialect{}),
	}, nil
}

func (m *MySQL) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []builder.Join,
	conditions condition.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	query, args, err := m.queryBuilder.Select(table, columns, joins, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil && m.logger != nil {
			m.logger.Error("failed to close rows", "error", err)
		}
	}()

	results := make([]map[string]any, 0)
	for rows.Next() {
		rowData := make(map[string]any)
		if err := rows.Scan(rowData); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil && m.logger != nil {
		m.logger.Error("failed to get rows", "error", err)
	}

	return results, nil
}

func (m *MySQL) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []builder.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	cdt := &condition.Expr{}
	cdt.Column("id").Value(id)

	query, args, err := m.queryBuilder.Select(table, []string{"*"}, joins, cdt)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil && m.logger != nil {
			m.logger.Error("failed to close rows", "error", err)
		}
	}()

	results := make([]map[string]any, 0)
	for rows.Next() {
		rowData := make(map[string]any)
		if err := rows.Scan(rowData); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil && m.logger != nil {
		m.logger.Error("failed to get rows", "error", err)
	}

	return results, nil
}

func (m *MySQL) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (sql.Result, error) {
	query, args, err := m.queryBuilder.Insert(table, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	result, err := m.DB.ExecContext(ctx, query, args...)
	return result, fmt.Errorf("failed to execute insert query: %w", err)
}

func (m *MySQL) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions condition.Condition,
	opts *options.QueryOptions,
) (sql.Result, error) {
	query, args, err := m.queryBuilder.Update(table, data, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := m.DB.ExecContext(ctx, query, args...)
	return result, fmt.Errorf("failed to execute update query: %w", err)
}

func (m *MySQL) Delete(
	ctx context.Context,
	table string,
	conditions condition.Condition,
	opts *options.QueryOptions,
) (sql.Result, error) {
	query, args, err := m.queryBuilder.Delete(table, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build delete query: %w", err)
	}

	result, err := m.DB.ExecContext(ctx, query, args...)
	return result, fmt.Errorf("failed to execute delete query: %w", err)
}

// func (m *MySQL) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {}
// func (m *MySQL) QueryRow(ctx context.Context, query string, args ...any) *sql.Row        {}

func (m *MySQL) Exec(
	ctx context.Context,
	query string,
	opts *options.QueryOptions,
	values ...any,
) (sql.Result, error) {
	result, err := m.DB.ExecContext(ctx, query, values...)
	return result, fmt.Errorf("failed to execute query: %w", err)
}

// Close closes the MySQL database connection.
func (m *MySQL) Close() {
	if m.DB == nil {
		return
	}
	_ = m.DB.Close()
}
