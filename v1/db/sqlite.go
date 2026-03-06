// Package db provides database abstraction interfaces and implementations for multiple database engines.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the SQLLite driver
	_ "github.com/mattn/go-sqlite3"
	builder "tounilab.com/db-connector/internal/pkg/builder"
	"tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
	"tounilab.com/db-connector/v1/db/dberror"
)

// SQLiteConfig holds configuration for connecting to a SQLLite database.
//
// Fields include file path, access mode, and connection pool settings.
type SQLiteConfig struct {
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

func (cfg SQLiteConfig) Driver() string {
	return definition.DriverSQLLite
}

// DSN returns the Data Source Name (DSN) for connecting to the SQLLite database.
// The DSN includes the following options:
//
// * file: the path to the SQLLite database file
// * cache: the cache mode (e.g., shared, private)
// * mode: the access mode (e.g., ro, rw, rwc, memory)
// * _foreign_keys: whether to enable foreign key constraints
// * _busy_timeout: the busy timeout duration in milliseconds
func (cfg SQLiteConfig) DSN() string {
	return fmt.Sprintf(
		"file:%s?cache=%s&mode=%s&_foreign_keys=%t&_busy_timeout=%d",
		cfg.FilePath, cfg.CacheMode, cfg.Mode, cfg.ForeignKeys, int(cfg.BusyTimeout.Milliseconds()),
	)
}

// SQLLite is a DB implementation for SQLite using database/sql.
type SQLLite struct {
	querier sqlQuerier // Underlying sql.DB connection pool

	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
}

// newSQLLite initializes a new SQLLite connection using the provided config.
func newSQLLite(cfg SQLiteConfig) (*SQLLite, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLLite connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLLite: %w", err)
	}

	return &SQLLite{
		querier:      db,
		queryBuilder: builder.NewSQLiteQueryBuilder(sqldialect.MySQLDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverSQLLite),
	}, nil
}

func sqliteCfgToDB(cfg DBConfig) (*SQLLite, error) {
	switch c := cfg.(type) {
	case SQLiteConfig:
		return newSQLLite(c)
	case *SQLiteConfig:
		return newSQLLite(*c)
	default:
		return nil, fmt.Errorf("unsupported sqlite config type: %T", cfg)
	}
}

func (m *SQLLite) PoolStats() (*PoolStatistics, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("sqlite.PoolStats: underlying db is not *sql.DB")
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

func (m *SQLLite) Ping(ctx context.Context) error {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return fmt.Errorf("sqlite.Ping: underlying db is not *sql.DB")
	}
	err := sqlDB.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("sqlite.Ping: failed to ping database: %w", err)
	}
	return nil
}

func (m *SQLLite) Begin(ctx context.Context) (Tx, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("sqlite.Begin: underlying db is not *sql.DB")
	}
	t, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlite.Begin: failed to begin transaction: %w", err)
	}

	return &SQLLite{
		querier: t,

		queryBuilder: m.queryBuilder,
		logger:       m.logger,
	}, nil
}

func (m *SQLLite) Get(
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

func (m *SQLLite) GetRaw(
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

func (m *SQLLite) GetByID(
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

func (m *SQLLite) GetByIDRaw(
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

func (m *SQLLite) Insert(
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

func (m *SQLLite) Inserts(
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

func (m *SQLLite) Update(
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

func (m *SQLLite) Delete(
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

func (m *SQLLite) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	rows, err := m.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite.Query: failed to execute query: %w", m.errorMapper.MapError(err))
	}
	defer func() {
		if err := rows.Close(); err != nil {
			if m.logger != nil {
				m.logger.Error("sqlite.Query: failed to close rows", "error", err)
			}
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("sqlite.Query: failed to get columns: %w", err)
	}

	return scanRows(rows, cols)
}

func (m *SQLLite) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	rows, err := m.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite.QueryRaw: failed to execute query: %w", m.errorMapper.MapError(err))
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("sqlite.QueryRaw: failed to create RowsAdapter: %w", err)
	}
	return ra, nil
}

func (m *SQLLite) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	result, err := m.querier.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("sqlite.Exec: failed to execute query: %w", m.errorMapper.MapError(err))
	}
	return fromSQLResult(result), nil
}

func (m *SQLLite) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := m.Begin(ctx)
	if err != nil {
		return fmt.Errorf("sqlite.WithTransaction: failed to begin transaction: %w", err)
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(ctx)
			if m.logger != nil {
				m.logger.Error("sqlite.WithTransaction: panic in transaction, rolled back", "panic", p, "error", e)
			}
		} else if err != nil {
			e = tx.Rollback(ctx) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf(
					"sqlite.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
		} else {
			err = tx.Commit(ctx) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("sqlite.WithTransaction: failed to commit transaction: %w", err)
			}
		}
	}()

	err = fn(tx)
	return err
}

// Close closes the SQLLite database connection.
func (m *SQLLite) Close() error {
	if m.querier == nil {
		return nil
	}
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return fmt.Errorf("sqlite.Close: underlying db is not *sql.DB")
	}
	err := sqlDB.Close()
	if err != nil {
		return fmt.Errorf("sqlite.Close: failed to close database: %w", err)
	}
	return nil
}

func (m *SQLLite) Commit(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("sqlite.Commit: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Commit(); err != nil {
		return fmt.Errorf("sqlite.Commit: failed to commit transaction: %w", err)
	}
	return nil
}

func (m *SQLLite) Rollback(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("sqlite.Rollback: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Rollback(); err != nil {
		return fmt.Errorf("sqlite.Rollback: failed to rollback transaction: %w", err)
	}
	return nil
}
