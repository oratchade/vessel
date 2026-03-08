// Package db provides database abstraction interfaces and implementations for multiple database engines.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the MSSQL driver
	_ "github.com/denisenkom/go-mssqldb"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"tounilab.com/db-connector/internal/pkg/builder"
	oh "tounilab.com/db-connector/internal/pkg/otel"
	"tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
	"tounilab.com/db-connector/v1/db/dberror"
)

// MSSQLConfig holds configuration for connecting to a MSSQL database.
//
// Fields include authentication, network and timeout settings plus pool options.
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
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
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
		errorMapper:  dberror.GetMapper(definition.DriverMSSQL),
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

// MSSQLCfgToDB is an exported version of mssqlCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the MSSQL driver implementation.
// The config type must be MSSQLConfig or *MSSQLConfig.
func MSSQLCfgToDB(cfg DBConfig) (DB, error) {
	return mssqlCfgToDB(cfg)
}

func (m *MSSQL) PoolStats() (*PoolStatistics, error) {
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("mssql.PoolStats: underlying db is not *sql.DB")
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

func (m *MSSQL) Ping(ctx context.Context) error {
	c, span := oh.UseTracer(ctx, "mssql.Ping",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("ping"),
		))
	defer span.End()
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		err := fmt.Errorf("mssql.Ping: underlying db is not *sql.DB")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	err := sqlDB.PingContext(c)
	if err != nil {
		err := fmt.Errorf("mssql.Ping: failed to ping database: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "ping successful")
	return nil
}

func (m *MSSQL) Begin(ctx context.Context) (Tx, error) {
	c, span := oh.UseTracer(ctx, "mssql.Begin",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("begin"),
		))
	defer span.End()
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		err := fmt.Errorf("mssql.Begin: underlying db is not *sql.DB")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	t, err := sqlDB.BeginTx(c, nil)
	if err != nil {
		err := fmt.Errorf("mssql.Begin: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "transaction begun")
	return &MSSQL{
		querier: t,

		queryBuilder: m.queryBuilder,
		logger:       m.logger,
	}, nil
}

func (m *MSSQL) GetQuery(
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

func (m *MSSQL) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	c, span := oh.UseTracer(ctx, "mssql.Get",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	results, err := get(c, table, columns, joins, conditions, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "get successful")
	return results, nil
}

func (m *MSSQL) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	c, span := oh.UseTracer(ctx, "mssql.GetRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	results, err := getRaw(c, table, columns, joins, conditions, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "getRaw successful")
	return results, nil
}

func (m *MSSQL) GetByIDQuery(
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

func (m *MSSQL) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	c, span := oh.UseTracer(ctx, "mssql.GetByID",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	results, err := getByID(c, table, id, joins, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "getByID successful")
	return results, nil
}

func (m *MSSQL) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	c, span := oh.UseTracer(ctx, "mssql.GetByIDRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	results, err := getByIDRaw(c, table, id, joins, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "getByIDRaw successful")
	return results, nil
}

func (m *MSSQL) InsertQuery(
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

func (m *MSSQL) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	c, span := oh.UseTracer(ctx, "mssql.Insert",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	result, err := insert(c, table, data, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return result, err
	}
	span.SetStatus(codes.Ok, "insert successful")
	return result, nil
}

func (m *MSSQL) InsertsQuery(
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

func (m *MSSQL) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	c, span := oh.UseTracer(ctx, "mssql.Inserts",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	result, err := inserts(c, table, data, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return result, err
	}
	span.SetStatus(codes.Ok, "inserts successful")
	return result, nil
}

func (m *MSSQL) UpdateQuery(
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

func (m *MSSQL) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	c, span := oh.UseTracer(ctx, "mssql.Update",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("update"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	result, err := update(c, table, data, conditions, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return result, err
	}
	span.SetStatus(codes.Ok, "update successful")
	return result, nil
}

func (m *MSSQL) DeleteQuery(
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

func (m *MSSQL) Delete(
	ctx context.Context,
	table string,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	c, span := oh.UseTracer(ctx, "mssql.Delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("delete"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	result, err := delete(c, table, conditions, opts, o)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return result, err
	}
	span.SetStatus(codes.Ok, "delete successful")
	return result, nil
}

//nolint:dupl
func (m *MSSQL) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	c, span := oh.UseTracer(ctx, "mssql.Query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := m.querier.QueryContext(c, query, args...)
	if err != nil {
		err := fmt.Errorf("mssql.Query: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
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
		err := fmt.Errorf("mssql.Query: failed to get columns: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "query executed")
	return results, nil
}

//nolint:dupl
func (m *MSSQL) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	c, span := oh.UseTracer(ctx, "mssql.QueryRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := m.querier.QueryContext(c, query, args...)
	if err != nil {
		err := fmt.Errorf("mssql.QueryRaw: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("mssql.QueryRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "query executed")
	return ra, nil
}

func (m *MSSQL) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	c, span := oh.UseTracer(ctx, "mssql.Exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("exec"),
		))
	defer span.End()
	result, err := m.querier.ExecContext(c, query, values...)
	if err != nil {
		err := fmt.Errorf("mssql.Exec: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "exec completed")
	return fromSQLResult(result), nil
}

func (m *MSSQL) Explain(
	ctx context.Context,
	query string,
	args ...any,
) (*RowsAdapter, error) {
	c, span := oh.UseTracer(ctx, "mssql.Explain",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("explain"),
		))
	defer span.End()
	explainQuery := "SET STATISTICS SHOWPLAN_TEXT ON; " + query + " SET STATISTICS SHOWPLAN_TEXT OFF;"
	rows, err := m.QueryRaw(c, explainQuery, args...)
	if err != nil {
		err := fmt.Errorf("mssql.Explain: failed to execute explain query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "explain executed")
	return rows, nil
}

//nolint:dupl
func (m *MSSQL) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	c, span := oh.UseTracer(ctx, "mssql.WithTransaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("transaction"),
		))
	defer span.End()
	tx, err := m.Begin(c)
	if err != nil {
		err := fmt.Errorf("mssql.WithTransaction: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(c)
			if m.logger != nil {
				m.logger.Error("mssql.WithTransaction: panic in transaction, rolled back", "panic", p, "error", e)
			}
			span.RecordError(fmt.Errorf("panic in transaction: %v", p))
			span.SetStatus(codes.Error, "panic occurred in transaction")
		} else if err != nil {
			e = tx.Rollback(c) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf(
					"mssql.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			err = tx.Commit(c) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("mssql.WithTransaction: failed to commit transaction: %w", err)
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

func (m *MSSQL) Commit(ctx context.Context) error {
	_, span := oh.UseTracer(ctx, "mssql.Commit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("commit"),
		))
	defer span.End()
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		err := fmt.Errorf("mssql.Commit: underlying db is not *sql.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := sqlTX.Commit(); err != nil {
		err := fmt.Errorf("mssql.Commit: failed to commit transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "transaction committed")
	return nil
}

func (m *MSSQL) Rollback(ctx context.Context) error {
	_, span := oh.UseTracer(ctx, "mssql.Rollback",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMicrosoftSQLServer,
			semconv.DBOperationName("rollback"),
		))
	defer span.End()
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		err := fmt.Errorf("mssql.Rollback: underlying db is not *sql.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := sqlTX.Rollback(); err != nil {
		err := fmt.Errorf("mssql.Rollback: failed to rollback transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "transaction rolled back")
	return nil
}
