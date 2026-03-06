// Package db provides database abstraction interfaces and implementations for multiple database engines.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the MySQL driver and provide config builder
	mysql "github.com/go-sql-driver/mysql"

	"tounilab.com/db-connector/internal/pkg/builder"
	sqldialect "tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
	"tounilab.com/db-connector/v1/db/dberror"
)

type sqlQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// MysqlConfig holds configuration for connecting to a MySQL database.
//
// Fields include authentication, network, timeouts and connection pool settings.
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
	cfgMap := map[string]string{
		"charset":      cfg.Charset,
		"parseTime":    fmt.Sprintf("%t", cfg.ParseTime),
		"loc":          cfg.Loc,
		"timeout":      cfg.Timeout.String(),
		"readTimeout":  cfg.ReadTimeout.String(),
		"writeTimeout": cfg.WriteTimeout.String(),
	}

	c := mysql.Config{
		User:   cfg.User,
		Passwd: cfg.Password,
		Net:    "tcp",
		Addr:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		DBName: cfg.Database,
		Params: cfgMap,
	}

	return c.FormatDSN()
}

// MySQL is a DB implementation for MySQL using database/sql.
type MySQL struct {
	querier sqlQuerier // Underlying sql.DB connection pool

	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
}

// newMySQL initializes a new MySQL connection using the provided config.
func newMySQL(cfg MysqlConfig) (*MySQL, error) {
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
		querier:      db,
		queryBuilder: builder.NewMySQLQueryBuilder(sqldialect.MySQLDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverMySQL),
	}, nil
}

// mysqlCfgToDB converts a DBConfig to a MySQL instance.
func mysqlCfgToDB(cfg DBConfig) (*MySQL, error) {
	switch c := cfg.(type) {
	case MysqlConfig:
		return newMySQL(c)
	case *MysqlConfig:
		return newMySQL(*c)
	default:
		return nil, fmt.Errorf("unsupported mysql config type: %T", cfg)
	}
}

// MySQLCfgToDB is an exported version of mysqlCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the MySQL driver implementation.
// The config type must be MysqlConfig or *MysqlConfig.
func MySQLCfgToDB(cfg DBConfig) (DB, error) {
	return mysqlCfgToDB(cfg)
}

// PoolStats implements the DB interface method for retrieving connection pool statistics.
func (m *MySQL) PoolStats() (*PoolStatistics, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("mysql.PoolStats: underlying db is not *sql.DB")
	}

	stats := sqlDB.Stats()
	return &PoolStatistics{
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		MaxOpenConnections: stats.MaxOpenConnections,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}, nil
}

// Ping implements the DB interface method to verify the database connection.
func (m *MySQL) Ping(ctx context.Context) error {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return fmt.Errorf("mysql.Ping: underlying db is not *sql.DB")
	}
	err := sqlDB.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("mysql.Ping: failed to ping database: %w", err)
	}
	return nil
}

// Begin implements the DB interface method to start a new transaction.
func (m *MySQL) Begin(ctx context.Context) (Tx, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("mysql.Begin: underlying db is not *sql.DB")
	}
	t, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql.Begin: failed to begin transaction: %w", err)
	}

	return &MySQL{
		querier: t,

		queryBuilder: m.queryBuilder,
		logger:       m.logger,
	}, nil
}

// Get implements the DBActions interface method to retrieve multiple rows as maps.
func (m *MySQL) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return get(ctx, table, columns, joins, conditions, opts, o)
}

// GetRaw implements the DBActions interface method to retrieve multiple rows as a RowsAdapter.
func (m *MySQL) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return getRaw(ctx, table, columns, joins, conditions, opts, o)
}

// GetByID implements the DBActions interface method to retrieve a row by primary key.
func (m *MySQL) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return getByID(ctx, table, id, joins, opts, o)
}

// GetByIDRaw implements the DBActions interface method to retrieve a row by primary key as a RowsAdapter.
func (m *MySQL) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return getByIDRaw(ctx, table, id, joins, opts, o)
}

// Insert implements the DBActions interface method to insert a new row.
func (m *MySQL) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return insert(ctx, table, data, opts, o)
}

// Inserts implements the DBActions interface method to insert multiple new rows.
func (m *MySQL) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return inserts(ctx, table, data, opts, o)
}

// Update implements the DBActions interface method to update existing rows.
func (m *MySQL) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return update(ctx, table, data, conditions, opts, o)
}

// Delete implements the DBActions interface method to delete rows.
func (m *MySQL) Delete(
	ctx context.Context,
	table string,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return delete(ctx, table, conditions, opts, o)
}

// Query implements the DBActions interface method to execute a raw query and return results as maps.
func (m *MySQL) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	rows, err := m.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql.Query: failed to execute query: %w", m.errorMapper.MapError(err))
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("mysql.Query: failed to get columns: %w", err)
	}

	return scanRows(rows, cols)
}

// QueryRaw implements the DBActions interface method to execute a raw query and return a RowsAdapter.
func (m *MySQL) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	rows, err := m.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql.QueryRaw: failed to execute query: %w", m.errorMapper.MapError(err))
	}
	defer func() {
		if err := rows.Close(); err != nil {
			if m.logger != nil {
				m.logger.Error("mysql.QueryRaw: failed to close rows", "error", err)
			}
		}
	}()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("mysql.QueryRaw: failed to create RowsAdapter: %w", err)
	}
	return ra, nil
}

// Exec implements the DBActions interface method to execute a raw statement (insert, update, delete, etc.).
func (m *MySQL) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	result, err := m.querier.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("mysql.Exec: failed to execute query: %w", m.errorMapper.MapError(err))
	}
	return fromSQLResult(result), nil
}

// WithTransaction implements the DB interface method to execute a function within a database transaction.
func (m *MySQL) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := m.Begin(ctx)
	if err != nil {
		return fmt.Errorf("mysql.WithTransaction: failed to begin transaction: %w", err)
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(ctx)
			if m.logger != nil {
				m.logger.Error("mysql.WithTransaction: panic in transaction, rolled back", "panic", p, "error", e)
			}
		} else if err != nil {
			e = tx.Rollback(ctx) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf(
					"mysql.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
		} else {
			err = tx.Commit(ctx) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("mysql.WithTransaction: failed to commit transaction: %w", err)
			}
		}
	}()

	err = fn(tx)
	return err
}

// Close closes the MySQL database connection.
// Close implements the DB interface method to close the database connection.
func (m *MySQL) Close() error {
	if m.querier == nil {
		return nil
	}
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return fmt.Errorf("mysql.Close: underlying db is not *sql.DB")
	}
	err := sqlDB.Close()
	if err != nil {
		return fmt.Errorf("mysql.Close: failed to close database: %w", err)
	}
	return nil
}

// Commit implements the Tx interface method to commit the transaction.
func (m *MySQL) Commit(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("mysql.Commit: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Commit(); err != nil {
		return fmt.Errorf("mysql.Commit: failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback implements the Tx interface method to rollback the transaction.
func (m *MySQL) Rollback(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("mysql.Rollback: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Rollback(); err != nil {
		return fmt.Errorf("mysql.Rollback: failed to rollback transaction: %w", err)
	}
	return nil
}
