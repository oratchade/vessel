// Package db provides a high-performance abstraction layer for Microsoft SQL Server 2016+,
// with support for parameterized queries, connection pooling via go-mssqldb,
// automatic identifier quoting using square brackets ([]), and T-SQL specific
// features like @@IDENTITY, WAITFOR clauses, and table-valued parameters.
package v1

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

	"tounilab.com/fabric/db/v1/dberror"
	"tounilab.com/fabric/internal/pkg/builder"
	oh "tounilab.com/fabric/internal/pkg/otel"
	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

// MSSQLConfig holds configuration for connecting to a MSSQL database.
//
// Fields include authentication, network and timeout settings plus pool options.
//
//nolint:revive,tagalign
type MSSQLConfig struct {
	User              string        `json:"user" yaml:"user" toml:"user"`                                                                         // Username for authentication
	Password          string        `json:"password" yaml:"password" toml:"password"`                                                             // Password for authentication
	Host              string        `json:"host" yaml:"host" toml:"host"`                                                                         // Hostname or IP address
	Port              uint16        `json:"port" yaml:"port" toml:"port"`                                                                         // Port number
	Database          string        `json:"database" yaml:"database" toml:"database"`                                                             // Database name
	Encrypt           string        `json:"encrypt,omitempty" yaml:"encrypt,omitempty" toml:"encrypt,omitempty"`                                  // Encryption mode (disable, true, false)
	TrustServerCert   bool          `json:"trust_server_cert,omitempty" yaml:"trust_server_cert,omitempty" toml:"trust_server_cert,omitempty"`    // Trust server certificate
	ConnectionTimeout time.Duration `json:"connection_timeout,omitempty" yaml:"connection_timeout,omitempty" toml:"connection_timeout,omitempty"` // Connection timeout
	ReadTimeout       time.Duration `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty" toml:"read_timeout,omitempty"`                   // Read timeout
	WriteTimeout      time.Duration `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty" toml:"write_timeout,omitempty"`                // Write timeout
	MaxOpenConns      int           `json:"max_open_conns,omitempty" yaml:"max_open_conns,omitempty" toml:"max_open_conns,omitempty"`             // Maximum number of open connections
	MaxIdleConns      int           `json:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" toml:"max_idle_conns,omitempty"`             // Maximum number of idle connections
	ConnMaxLifetime   time.Duration `json:"conn_max_lifetime,omitempty" yaml:"conn_max_lifetime,omitempty" toml:"conn_max_lifetime,omitempty"`    // Maximum lifetime of a connection
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
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
	safeLogger   *SafeLogger          // SafeLogger for automatic error classification and logging
}

// newMSSQL initializes a new MSSQL connection using the provided config.
func newMSSQL(cfg MSSQLConfig, logger Logger) (*MSSQL, error) {
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
		safeLogger:   NewSafeLogger(logger),
	}, nil
}

func mssqlCfgToDB(cfg DBConfig, logger Logger) (*MSSQL, error) {
	switch c := cfg.(type) {
	case MSSQLConfig:
		return newMSSQL(c, logger)
	case *MSSQLConfig:
		return newMSSQL(*c, logger)
	default:
		return nil, fmt.Errorf("unsupported mssql config type: %T", cfg)
	}
}

// MSSQLCfgToDB is an exported version of mssqlCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the MSSQL driver implementation.
// The config type must be MSSQLConfig or *MSSQLConfig.
func MSSQLCfgToDB(cfg DBConfig, logger Logger) (DB, error) {
	return mssqlCfgToDB(cfg, logger)
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

//nolint:dupl
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
		m.safeLogger.Error(fmt.Errorf("mssql.Ping: failed to ping database: %w", err))
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
		m.safeLogger.QueryError(c, "mssql", "begin", "", 0, err)
		return nil, err
	}
	t, err := sqlDB.BeginTx(c, nil)
	if err != nil {
		err := fmt.Errorf("mssql.Begin: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "begin", "", 0, err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "transaction begun")
	m.safeLogger.TransactionSuccess(c, "mssql", "begin")
	return &MSSQL{
		querier: t,

		queryBuilder: m.queryBuilder,
		safeLogger:   m.safeLogger,
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
	startTime := time.Now()
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
	}
	results, err := get(c, table, columns, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "get successful")
	m.safeLogger.QuerySuccess(c, "mssql", "select", table, duration, len(results))
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
	startTime := time.Now()
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
	}
	results, err := getRaw(c, table, columns, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getRaw successful")
	m.safeLogger.QuerySuccess(c, "mssql", "select", table, duration, -1)
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
	}
	return getByIDQuery(table, id, joins, opts, o)
}

//nolint:dupl
func (m *MSSQL) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	startTime := time.Now()
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
	}
	results, err := getByID(c, table, id, joins, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByID successful")
	m.safeLogger.QuerySuccess(c, "mssql", "select", table, duration, len(results))
	return results, nil
}

func (m *MSSQL) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	startTime := time.Now()
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
	}
	results, err := getByIDRaw(c, table, id, joins, opts, o)
	duration := time.Since(startTime)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByIDRaw successful")
	m.safeLogger.QuerySuccess(c, "mssql", "select", table, duration, -1)
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
	}
	return insertQuery(table, data, opts, o)
}

//nolint:dupl
func (m *MSSQL) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
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
	}
	result, err := insert(c, table, data, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "msql", "insert", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "insert successful")

	rowsAffected := int(0)
	if result != nil {
		rowsAffected = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mssql", "insert", table, duration, rowsAffected)
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
	}
	return insertsQuery(table, data, opts, o)
}

//nolint:dupl
func (m *MSSQL) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
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
	}
	result, err := inserts(c, table, data, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "insert", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "inserts successful")

	rowsAffected := int(0)
	if result != nil {
		rowsAffected = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mssql", "insert", table, duration, rowsAffected)
	return result, nil
}

func (m *MSSQL) UpdateQuery(
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

func (m *MSSQL) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
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
	}
	result, err := update(c, table, data, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "update", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "update successful")

	rowsAffected := int(0)
	if result != nil {
		rowsAffected = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mssql", "update", table, duration, rowsAffected)
	return result, nil
}

func (m *MSSQL) DeleteQuery(
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

func (m *MSSQL) Delete(
	ctx context.Context,
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
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
	}
	result, err := delete(c, table, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mssql", "delete", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "delete successful")

	rowsAffected := int(0)
	if result != nil {
		rowsAffected = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mssql", "delete", table, duration, rowsAffected)
	return result, nil
}

//nolint:dupl
func (m *MSSQL) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	startTime := time.Now()
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
		m.safeLogger.Error(fmt.Errorf("mssql.Query: failed to execute query: %w", err))
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			m.safeLogger.Error(fmt.Errorf("mssql.Query: failed to close rows: %w", err))
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		err := fmt.Errorf("mssql.Query: failed to get columns: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(fmt.Errorf("mssql.Query: failed to get columns: %w", err))
		return nil, err
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		m.safeLogger.Error(fmt.Errorf("mssql.Query: failed to scan rows: %w", err))
		return results, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	m.safeLogger.QuerySuccess(c, "mssql", "query", "", duration, len(results))
	return results, nil
}

//nolint:dupl
func (m *MSSQL) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	startTime := time.Now()
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
		m.safeLogger.Error(fmt.Errorf("mssql.QueryRaw: failed to execute query: %w", err))
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("mssql.QueryRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(fmt.Errorf("mssql.QueryRaw: failed to create rows adapter: %w", err))
		return nil, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	m.safeLogger.QuerySuccess(c, "mssql", "query", "", duration, -1)
	return ra, nil
}

func (m *MSSQL) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	startTime := time.Now()
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
		m.safeLogger.Error(fmt.Errorf("mssql.Exec: failed to execute query: %w", err))
		return nil, err
	}

	execResult := fromSQLResult(result)
	duration := time.Since(startTime)

	span.SetStatus(codes.Ok, "exec completed")
	rowsAffected := int(0)
	if execResult != nil {
		rowsAffected = int(execResult.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mssql", "exec", "", duration, rowsAffected)
	return execResult, nil
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
		m.safeLogger.Error(fmt.Errorf("mssql.Explain: failed to execute explain query: %w", err))
		return nil, err
	}

	span.SetStatus(codes.Ok, "explain executed")
	m.safeLogger.Debug("mssql.Explain: explain query executed successfully")
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
			span.RecordError(fmt.Errorf("panic in transaction: %v", p))
			span.SetStatus(codes.Error, "panic occurred in transaction")
			//nolint:errorlint
			m.safeLogger.Error(fmt.Errorf("mssql.WithTransaction: panic occurred: %v, rollback error: %v", p, e))
		} else if err != nil {
			e = tx.Rollback(c)
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
			err = tx.Commit(c)
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

//nolint:dupl
func (m *MSSQL) Commit(ctx context.Context) error {
	startTime := time.Now()
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
		m.safeLogger.QueryError(ctx, "mssql", "commit", "", 0, err)
		return err
	}
	if err := sqlTX.Commit(); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("mssql.Commit: failed to commit transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mssql", "commit", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction committed")
	m.safeLogger.TransactionSuccess(ctx, "mssql", "commit")
	return nil
}

//nolint:dupl
func (m *MSSQL) Rollback(ctx context.Context) error {
	startTime := time.Now()
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
		m.safeLogger.QueryError(ctx, "mssql", "rollback", "", 0, err)
		return err
	}
	if err := sqlTX.Rollback(); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("mssql.Rollback: failed to rollback transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mssql", "rollback", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction rolled back")
	m.safeLogger.TransactionSuccess(ctx, "mssql", "rollback")
	return nil
}
