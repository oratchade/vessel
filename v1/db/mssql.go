package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the MSSQL driver
	_ "github.com/denisenkom/go-mssqldb"

	"tounilab.com/db-connector/internal/pkg/builder"
	"tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

// MSSQLConfig holds configuration for connecting to a MSSQL database.
//
// Fields include authentication, network and timeout settings plus pool options.
type MSSQLConfig struct {
	User string // Username for authentication
	//nolint:gosec
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
	return definition.DriverMSSQL
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

// MSSQL is a DB implementation for Microsoft SQL Server using database/sql.
type MSSQL struct {
	querier sqlQuerier // Underlying sql.DB connection pool

	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
}

// newMSSQL initializes a new MSSQL connection using the provided config.
func newMSSQL(cfg MSSQLConfig) (*MSSQL, error) {
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

	return &MSSQL{
		querier:      db,
		queryBuilder: builder.NewMSSQLQueryBuilder(sqldialect.MSSQLDialect{}),
	}, nil
}

func mssqlCfgToDB(cfg DBConfig) (*MSSQL, error) {
	switch c := cfg.(type) {
	case MSSQLConfig:
		return newMSSQL(c)
	case *MSSQLConfig:
		return newMSSQL(*c)
	default:
		return nil, fmt.Errorf("unsupported mssql config type: %T", cfg)
	}
}

func (m *MSSQL) Ping(ctx context.Context) error {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return fmt.Errorf("mssql.Ping: underlying db is not *sql.DB")
	}
	err := sqlDB.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("mssql.Ping: failed to ping database: %w", err)
	}
	return nil
}

func (m *MSSQL) Begin(ctx context.Context) (Tx, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("mssql.Begin: underlying db is not *sql.DB")
	}
	t, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mssql.Begin: failed to begin transaction: %w", err)
	}

	return &MSSQL{
		querier: t,

		queryBuilder: m.queryBuilder,
		logger:       m.logger,
	}, nil
}

func (m *MSSQL) Get(
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

func (m *MSSQL) GetRaw(
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

func (m *MSSQL) GetByID(
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

func (m *MSSQL) GetByIDRaw(
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

func (m *MSSQL) Insert(
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

func (m *MSSQL) Update(
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

func (m *MSSQL) Delete(
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

func (m *MSSQL) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	rows, err := m.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mssql.Query: failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			if m.logger != nil {
				m.logger.Error("mssql.Query: failed to close rows", "error", err)
			}
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("mssql.Query: failed to get columns: %w", err)
	}

	return scanRows(rows, cols)
}

func (m *MSSQL) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	rows, err := m.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mssql.QueryRaw: failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			if m.logger != nil {
				m.logger.Error("mssql.QueryRaw: failed to close rows", "error", err)
			}
		}
	}()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("mssql.QueryRaw: failed to create rows adapter: %w", err)
	}

	return ra, nil
}

func (m *MSSQL) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	result, err := m.querier.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("mssql.Exec: failed to execute query: %w", err)
	}
	return fromSQLResult(result), nil
}

func (m *MSSQL) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := m.Begin(ctx)
	if err != nil {
		return fmt.Errorf("mssql.WithTransaction: failed to begin transaction: %w", err)
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(ctx)
			if m.logger != nil {
				m.logger.Error("mssql.WithTransaction: panic in transaction, rolled back", "panic", p, "error", e)
			}
		} else if err != nil {
			e = tx.Rollback(ctx) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf(
					"mssql.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
		} else {
			err = tx.Commit(ctx) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("mssql.WithTransaction: failed to commit transaction: %w", err)
			}
		}
	}()

	err = fn(tx)
	return err
}

// Close closes the MSSQL database connection.
func (m *MSSQL) Close() error {
	if m.querier == nil {
		return nil
	}
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return fmt.Errorf("mssql.Close: underlying db is not *sql.DB")
	}
	err := sqlDB.Close()
	if err != nil {
		return fmt.Errorf("mssql.Close: failed to close database: %w", err)
	}
	return nil
}

func (m *MSSQL) Commit(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("mssql.Commit: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Commit(); err != nil {
		return fmt.Errorf("mssql.Commit: failed to commit transaction: %w", err)
	}
	return nil
}

func (m *MSSQL) Rollback(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("mssql.Rollback: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Rollback(); err != nil {
		return fmt.Errorf("mssql.Rollback: failed to rollback transaction: %w", err)
	}
	return nil
}
