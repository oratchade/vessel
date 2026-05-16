// Package db provides a lightweight abstraction layer for SQLite 3.x,
// optimized for embedded and serverless use cases. Supports in-memory databases,
// file-backed persistence, automatic identifier quoting with no special characters,
// and seamless foreign key enforcement. Ideal for local development and edge deployment.
package v1

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the SQLITE driver
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"tounilab.com/fabric/db/v1/dberror"
	builder "tounilab.com/fabric/internal/pkg/builder"
	oh "tounilab.com/fabric/internal/pkg/otel"
	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

// SQLiteConfig holds configuration for connecting to a SQLITE database.
//
// Fields include file path, access mode, and connection pool settings.
//
//nolint:revive,tagalign
type SQLiteConfig struct {
	FilePath        string        `json:"file_path" yaml:"file_path" toml:"file_path"`                                                       // Path to the SQLITE database file
	CacheMode       string        `json:"cache_mode,omitempty" yaml:"cache_mode,omitempty" toml:"cache_mode,omitempty"`                      // Cache mode (shared, private)
	Mode            string        `json:"mode,omitempty" yaml:"mode,omitempty" toml:"mode,omitempty"`                                        // Access mode (ro, rw, rwc, memory)
	ForeignKeys     bool          `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty" toml:"foreign_keys,omitempty"`                // Enable foreign key constraints
	BusyTimeout     time.Duration `json:"busy_timeout,omitempty" yaml:"busy_timeout,omitempty" toml:"busy_timeout,omitempty"`                // Busy timeout duration
	MaxOpenConns    int           `json:"max_open_conns,omitempty" yaml:"max_open_conns,omitempty" toml:"max_open_conns,omitempty"`          // Maximum number of open connections
	MaxIdleConns    int           `json:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" toml:"max_idle_conns,omitempty"`          // Maximum number of idle connections
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime,omitempty" yaml:"conn_max_lifetime,omitempty" toml:"conn_max_lifetime,omitempty"` // Maximum lifetime of a connection
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
	safeLogger   *SafeLogger          // Nil-safe logger wrapper (created once, reused)
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
}

// newSQLLite initializes a new SQLITE connection using the provided config.
func newSQLLite(cfg SQLiteConfig, logger Logger) (*SQLITE, error) {
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
		queryBuilder: builder.NewSQLiteQueryBuilder(sqldialect.SQLiteDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverSQLLite),
		safeLogger:   NewSafeLogger(logger),
	}, nil
}

func sqliteCfgToDB(cfg DBConfig, logger Logger) (*SQLITE, error) {
	switch c := cfg.(type) {
	case SQLiteConfig:
		return newSQLLite(c, logger)
	case *SQLiteConfig:
		return newSQLLite(*c, logger)
	default:
		return nil, fmt.Errorf("unsupported sqlite config type: %T", cfg)
	}
}

// SQLiteCfgToDB is an exported version of sqliteCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the SQLite driver implementation.
// The config type must be SQLiteConfig or *SQLiteConfig.
func SQLiteCfgToDB(cfg DBConfig, logger Logger) (DB, error) {
	return sqliteCfgToDB(cfg, logger)
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

//nolint:dupl
func (m *SQLITE) Ping(ctx context.Context) error {
	c, span := oh.UseTracer(ctx, "sqlite.Ping",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("ping"),
		))
	defer span.End()
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		err := fmt.Errorf("sqlite.Ping: underlying db is not *sql.DB")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	err := sqlDB.PingContext(c)
	if err != nil {
		err = fmt.Errorf("sqlite.Ping: failed to ping database: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return err
	}
	span.SetStatus(codes.Ok, "ping successful")
	return nil
}

// Begin implements the DB interface method to start a new transaction.
func (m *SQLITE) Begin(ctx context.Context) (Tx, error) {
	c, span := oh.UseTracer(ctx, "sqlite.Begin",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("begin"),
		))
	defer span.End()
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		err := fmt.Errorf("sqlite.Begin: underlying db is not *sql.DB")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "begin", "", 0, err)
		return nil, err
	}
	t, err := sqlDB.BeginTx(c, nil)
	if err != nil {
		err := fmt.Errorf("sqlite.Begin: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "sqlite", "begin", "", 0, err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "transaction begun")
	m.safeLogger.TransactionSuccess(c, "sqlite", "begin")
	return &SQLITE{
		querier: t,

		queryBuilder: m.queryBuilder,
		safeLogger:   m.safeLogger,
		errorMapper:  m.errorMapper,
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
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Get",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	results, err := get(c, table, columns, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "get successful")
	m.safeLogger.QuerySuccess(c, "sqlite", "select", table, duration, len(results))
	return results, nil
}

func (m *SQLITE) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.GetRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	results, err := getRaw(c, table, columns, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getRaw successful")
	m.safeLogger.QuerySuccess(c, "sqlite", "select", table, duration, -1)
	return results, nil
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
	}
	return getByIDQuery(table, id, joins, opts, o)
}

//nolint:dupl
func (m *SQLITE) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.GetByID",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	results, err := getByID(c, table, id, joins, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByID successful")
	m.safeLogger.QuerySuccess(c, "sqlite", "select", table, duration, len(results))
	return results, nil
}

func (m *SQLITE) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.GetByIDRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	results, err := getByIDRaw(c, table, id, joins, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByIDRaw successful")
	m.safeLogger.QuerySuccess(c, "sqlite", "select", table, duration, -1)
	return results, nil
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
	}
	return insertQuery(table, data, opts, o)
}

//nolint:dupl
func (m *SQLITE) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Insert",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	result, err := insert(c, table, data, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "insert", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "insert successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "sqlite", "insert", table, duration, rowsReturned)
	return result, nil
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
	}
	return insertsQuery(table, data, opts, o)
}

//nolint:dupl
func (m *SQLITE) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Inserts",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	result, err := inserts(c, table, data, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "insert", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "inserts successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "sqlite", "insert", table, duration, rowsReturned)
	return result, nil
}

// UpdateQuery builds the UPDATE query without executing it.
func (m *SQLITE) UpdateQuery(
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	return updateQuery(table, data, joins, conditions, opts, o)
}

func (m *SQLITE) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Update",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("update"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	result, err := update(c, table, data, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "update", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "update successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "sqlite", "update", table, duration, rowsReturned)
	return result, nil
}

// DeleteQuery builds the DELETE query without executing it.
func (m *SQLITE) DeleteQuery(
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	return deleteQuery(table, joins, conditions, opts, o)
}

func (m *SQLITE) Delete(
	ctx context.Context,
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("delete"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
	}
	result, err := delete(c, table, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "sqlite", "delete", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "delete successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "sqlite", "delete", table, duration, rowsReturned)
	return result, nil
}

//nolint:dupl
func (m *SQLITE) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := m.querier.QueryContext(c, query, args...)
	if err != nil {
		err := fmt.Errorf("sqlite.Query: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			m.safeLogger.Error(fmt.Errorf("sqlite.Query: failed to close rows: %w", err))
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		err := fmt.Errorf("sqlite.Query: failed to get columns: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		m.safeLogger.Error(fmt.Errorf("sqlite.Query: failed to scan rows: %w", err))
		return results, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	m.safeLogger.QuerySuccess(c, "sqlite", "query", "", duration, len(results))
	return results, nil
}

//nolint:dupl
func (m *SQLITE) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.QueryRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := m.querier.QueryContext(c, query, args...)
	if err != nil {
		err := fmt.Errorf("sqlite.QueryRaw: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("sqlite.QueryRaw: failed to create RowsAdapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	m.safeLogger.QuerySuccess(c, "sqlite", "query", "", duration, -1)
	return ra, nil
}

func (m *SQLITE) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "sqlite.Exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("exec"),
		))
	defer span.End()
	result, err := m.querier.ExecContext(c, query, values...)
	if err != nil {
		err := fmt.Errorf("sqlite.Exec: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	execResult, err := fromSQLResult(result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, fmt.Errorf("sqlite.Exec: %w", err)
	}
	duration := time.Since(startTime)

	span.SetStatus(codes.Ok, "exec completed")
	rowsAffected := int(0)
	if execResult != nil {
		rowsAffected = int(execResult.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "sqlite", "exec", "", duration, rowsAffected)
	return execResult, nil
}

func (m *SQLITE) Explain(
	ctx context.Context,
	query string,
	args ...any,
) (*RowsAdapter, error) {
	c, span := oh.UseTracer(ctx, "sqlite.Explain",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("explain"),
		))
	defer span.End()
	explainQuery := "EXPLAIN QUERY PLAN " + query
	rows, err := m.QueryRaw(c, explainQuery, args...)
	if err != nil {
		err := fmt.Errorf("sqlite.Explain: failed to execute explain query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "explain executed")
	m.safeLogger.Debug("sqlite.Explain: explain query executed successfully")
	return rows, nil
}

//nolint:dupl
func (m *SQLITE) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	c, span := oh.UseTracer(ctx, "sqlite.WithTransaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("transaction"),
		))
	defer span.End()
	tx, err := m.Begin(c)
	if err != nil {
		err := fmt.Errorf("sqlite.WithTransaction: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(c)
			span.RecordError(fmt.Errorf("panic in transaction: %v", p))
			span.SetStatus(codes.Error, "panic occurred in transaction")
			//nolint:errorlint
			m.safeLogger.Error(fmt.Errorf("sqlite.WithTransaction: panic occurred: %v, rollback error: %v", p, e))
		} else if err != nil {
			e = tx.Rollback(c)
			if e != nil {
				err = fmt.Errorf(
					"sqlite.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			err = tx.Commit(c)
			if err != nil {
				err = fmt.Errorf("sqlite.WithTransaction: failed to commit transaction: %w", err)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "transaction committed")
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

//nolint:dupl
func (m *SQLITE) Commit(ctx context.Context) error {
	startTime := time.Now()
	_, span := oh.UseTracer(ctx, "sqlite.Commit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("commit"),
		))
	defer span.End()
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		err := fmt.Errorf("sqlite.Commit: underlying db is not *sql.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "sqlite", "commit", "", 0, err)
		return err
	}
	if err := sqlTX.Commit(); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("sqlite.Commit: failed to commit transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "sqlite", "commit", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction committed")
	m.safeLogger.TransactionSuccess(ctx, "sqlite", "commit")
	return nil
}

//nolint:dupl
func (m *SQLITE) Rollback(ctx context.Context) error {
	startTime := time.Now()
	_, span := oh.UseTracer(ctx, "sqlite.Rollback",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("rollback"),
		))
	defer span.End()
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		err := fmt.Errorf("sqlite.Rollback: underlying db is not *sql.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "sqlite", "rollback", "", 0, err)
		return err
	}
	if err := sqlTX.Rollback(); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("sqlite.Rollback: failed to rollback transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "sqlite", "rollback", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction rolled back")
	m.safeLogger.TransactionSuccess(ctx, "sqlite", "rollback")
	return nil
}
