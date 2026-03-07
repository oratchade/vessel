// Package db provides database abstraction interfaces and implementations for multiple database engines.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the SQLITE driver
	_ "github.com/mattn/go-sqlite3"
	builder "tounilab.com/db-connector/internal/pkg/builder"
	"tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
	"tounilab.com/db-connector/v1/db/dberror"
)

// SQLiteConfig holds configuration for connecting to a SQLITE database.
//
// Fields include file path, access mode, and connection pool settings.
type SQLiteConfig struct {
	FilePath        string        // Path to the SQLITE database file
	CacheMode       string        // Cache mode (shared, private)
	Mode            string        // Access mode (ro, rw, rwc, memory)
	ForeignKeys     bool          // Enable foreign key constraints
	BusyTimeout     time.Duration // Busy timeout duration
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
}

// Driver returns the driver name for SQLITE databases.

func (cfg SQLiteConfig) Driver() string {
	return definition.DriverSQLLite
}

// DSN returns the Data Source Name (DSN) for connecting to the SQLITE database.
// The DSN includes the following options:
//
// * file: the path to the SQLITE database file
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

// SQLITE is a DB implementation for SQLite using database/sql.
type SQLITE struct {
	querier sqlQuerier // Underlying sql.DB connection pool

	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
}

// newSQLLite initializes a new SQLITE connection using the provided config.
func newSQLLite(cfg SQLiteConfig) (*SQLITE, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLITE connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLITE: %w", err)
	}

	return &SQLITE{
		querier:      db,
		queryBuilder: builder.NewSQLiteQueryBuilder(sqldialect.MySQLDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverSQLLite),
	}, nil
}

func sqliteCfgToDB(cfg DBConfig) (*SQLITE, error) {
	switch c := cfg.(type) {
	case SQLiteConfig:
		return newSQLLite(c)
	case *SQLiteConfig:
		return newSQLLite(*c)
	default:
		return nil, fmt.Errorf("unsupported sqlite config type: %T", cfg)
	}
}

// SQLiteCfgToDB is an exported version of sqliteCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the SQLite driver implementation.
// The config type must be SQLiteConfig or *SQLiteConfig.
func SQLiteCfgToDB(cfg DBConfig) (DB, error) {
	return sqliteCfgToDB(cfg)
}

func (m *SQLITE) PoolStats() (*PoolStatistics, error) {
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

func (m *SQLITE) Ping(ctx context.Context) error {
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

func (m *SQLITE) Begin(ctx context.Context) (Tx, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("sqlite.Begin: underlying db is not *sql.DB")
	}
	t, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlite.Begin: failed to begin transaction: %w", err)
	}

	return &SQLITE{
		querier: t,

		queryBuilder: m.queryBuilder,
		logger:       m.logger,
	}, nil
}

// GetQuery builds the SELECT query without executing it.
func (m *SQLITE) GetQuery(
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return getQuery(table, columns, joins, conditions, opts, o)
}

func (m *SQLITE) Get(
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

func (m *SQLITE) GetRaw(
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

// GetByIDQuery builds the SELECT by ID query without executing it.
func (m *SQLITE) GetByIDQuery(
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return getByIDQuery(table, id, joins, opts, o)
}

func (m *SQLITE) GetByID(
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

func (m *SQLITE) GetByIDRaw(
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

// InsertQuery builds the INSERT query without executing it.
func (m *SQLITE) InsertQuery(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return insertQuery(table, data, opts, o)
}

func (m *SQLITE) Insert(
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

// InsertsQuery builds the INSERT query for multiple rows without executing it.
func (m *SQLITE) InsertsQuery(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return insertsQuery(table, data, opts, o)
}

func (m *SQLITE) Inserts(
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

// UpdateQuery builds the UPDATE query without executing it.
func (m *SQLITE) UpdateQuery(
	table string,
	data map[string]any,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return updateQuery(table, data, conditions, opts, o)
}

func (m *SQLITE) Update(
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

// DeleteQuery builds the DELETE query without executing it.
func (m *SQLITE) DeleteQuery(
	table string,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return deleteQuery(table, conditions, opts, o)
}

func (m *SQLITE) Delete(
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

func (m *SQLITE) Query(
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

func (m *SQLITE) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
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

func (m *SQLITE) Exec(
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

func (m *SQLITE) Explain(
	ctx context.Context,
	query string,
	args ...any,
) (*RowsAdapter, error) {
	explainQuery := "EXPLAIN QUERY PLAN " + query
	rows, err := m.QueryRaw(ctx, explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite.Explain: failed to execute explain query: %w", err)
	}
	return rows, nil
}

func (m *SQLITE) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
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

// Close closes the SQLITE database connection.
func (m *SQLITE) Close() error {
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

func (m *SQLITE) Commit(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("sqlite.Commit: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Commit(); err != nil {
		return fmt.Errorf("sqlite.Commit: failed to commit transaction: %w", err)
	}
	return nil
}

func (m *SQLITE) Rollback(_ context.Context) error {
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		return fmt.Errorf("sqlite.Rollback: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Rollback(); err != nil {
		return fmt.Errorf("sqlite.Rollback: failed to rollback transaction: %w", err)
	}
	return nil
}
