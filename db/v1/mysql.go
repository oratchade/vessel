package v1

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	// Import the MySQL driver and provide config builder
	mysql "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"tounilab.com/vessel/db/v1/dberror"
	"tounilab.com/vessel/internal/pkg/builder"
	oh "tounilab.com/vessel/internal/pkg/otel"
	sqldialect "tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/definition"
	"tounilab.com/vessel/pkg/query/options"
)

type sqlQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// MysqlConfig holds configuration for connecting to a MySQL database.
//
// Fields include authentication, network, timeouts and connection pool settings.
//
//nolint:revive,tagalign
type MysqlConfig struct {
	User            string        `json:"user" yaml:"user" toml:"user"`                                                                      // Username for authentication
	Password        string        `json:"password" yaml:"password" toml:"password"`                                                          // Password for authentication
	Host            string        `json:"host" yaml:"host" toml:"host"`                                                                      // Hostname or IP address
	Port            uint16        `json:"port" yaml:"port" toml:"port"`                                                                      // Port number
	Database        string        `json:"database" yaml:"database" toml:"database"`                                                          // Database name
	Charset         string        `json:"charset,omitempty" yaml:"charset,omitempty" toml:"charset,omitempty"`                               // Character set (e.g., utf8mb4)
	ParseTime       bool          `json:"parse_time,omitempty" yaml:"parse_time,omitempty" toml:"parse_time,omitempty"`                      // Parse time values to time.Time
	Loc             string        `json:"loc,omitempty" yaml:"loc,omitempty" toml:"loc,omitempty"`                                           // Time zone location (e.g., Local, UTC)
	Timeout         time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout,omitempty"`                               // Connection timeout
	ReadTimeout     time.Duration `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty" toml:"read_timeout,omitempty"`                // Read timeout
	WriteTimeout    time.Duration `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty" toml:"write_timeout,omitempty"`             // Write timeout
	MaxOpenConns    int           `json:"max_open_conns,omitempty" yaml:"max_open_conns,omitempty" toml:"max_open_conns,omitempty"`          // Maximum number of open connections
	MaxIdleConns    int           `json:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" toml:"max_idle_conns,omitempty"`          // Maximum number of idle connections
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime,omitempty" yaml:"conn_max_lifetime,omitempty" toml:"conn_max_lifetime,omitempty"` // Maximum lifetime of a connection
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
		"parseTime":    strconv.FormatBool(cfg.ParseTime),
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
	safeLogger   *SafeLogger          // Nil-safe logger wrapper (created once, reused)
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
}

// newMySQL initializes a new MySQL connection using the provided config.
func newMySQL(cfg MysqlConfig, logger Logger) (*MySQL, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	return &MySQL{
		querier:      db,
		queryBuilder: builder.NewMySQLQueryBuilder(sqldialect.MySQLDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverMySQL),
		safeLogger:   NewSafeLogger(logger),
	}, nil
}

// mysqlCfgToDB converts a DBConfig to a MySQL instance.
func mysqlCfgToDB(cfg DBConfig, logger Logger) (*MySQL, error) {
	switch c := cfg.(type) {
	case MysqlConfig:
		return newMySQL(c, logger)
	case *MysqlConfig:
		return newMySQL(*c, logger)
	default:
		return nil, fmt.Errorf("unsupported mysql config type: %T", cfg)
	}
}

// MySQLCfgToDB is an exported version of mysqlCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the MySQL driver implementation.
// The config type must be MysqlConfig or *MysqlConfig.
func MySQLCfgToDB(cfg DBConfig, logger Logger) (DB, error) {
	return mysqlCfgToDB(cfg, logger)
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
//
//nolint:dupl
func (m *MySQL) Ping(ctx context.Context) error {
	c, span := oh.UseTracer(ctx, "mysql.Ping",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("ping"),
		))
	defer span.End()
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		err := fmt.Errorf("mysql.Ping: underlying db is not *sql.DB")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	err := sqlDB.PingContext(c)
	if err != nil {
		err = fmt.Errorf("mysql.Ping: failed to ping database: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return err
	}
	span.SetStatus(codes.Ok, "ping successful")
	return nil
}

// Begin implements the DB interface method to start a new transaction.
func (m *MySQL) Begin(ctx context.Context, opts ...TransactionOptions) (Tx, error) {
	txOpts := firstTransactionOptions(opts)
	c, span := oh.UseTracer(ctx, "mysql.Begin",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("begin"),
		))
	defer span.End()
	sqlDB, ok := m.querier.(*sql.DB)
	if !ok {
		err := fmt.Errorf("mysql.Begin: underlying db is not *sql.DB")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mysql", "begin", "", 0, err)
		return nil, err
	}
	t, err := sqlDB.BeginTx(c, &sql.TxOptions{Isolation: txOpts.Isolation, ReadOnly: txOpts.ReadOnly})
	if err != nil {
		err := fmt.Errorf("mysql.Begin: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mysql", "begin", "", 0, err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "transaction begun")
	m.safeLogger.TransactionSuccess(ctx, "mysql", "begin")
	return &MySQL{
		querier: t,

		queryBuilder: m.queryBuilder,
		safeLogger:   m.safeLogger,
		errorMapper:  m.errorMapper,
	}, nil
}

// GetQuery builds the SELECT query without executing it.
func (m *MySQL) GetQuery(
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return getQuery(table, columns, joins, conditions, opts, o)
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
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Get",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	results, err := get(c, table, columns, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "get successful")
	m.safeLogger.QuerySuccess(c, "mysql", "select", table, duration, len(results))
	return results, nil
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
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.GetRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	results, err := getRaw(c, table, columns, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getRaw successful")
	m.safeLogger.QuerySuccess(c, "mysql", "select", table, duration, -1)
	return results, nil
}

// GetByIDQuery builds the SELECT by ID query without executing it.
func (m *MySQL) GetByIDQuery(
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return getByIDQuery(table, id, joins, opts, o)
}

// GetByID implements the DBActions interface method to retrieve a row by primary key.
//
//nolint:dupl
func (m *MySQL) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.GetByID",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	results, err := getByID(c, table, id, joins, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByID successful")
	m.safeLogger.QuerySuccess(c, "mysql", "select", table, duration, len(results))
	return results, nil
}

// GetByIDRaw implements the DBActions interface method to retrieve a row by primary key as a RowsAdapter.
func (m *MySQL) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.GetByIDRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	results, err := getByIDRaw(c, table, id, joins, opts, o)
	duration := time.Since(startTime)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByIDRaw successful")
	m.safeLogger.QuerySuccess(c, "mysql", "select", table, duration, -1)
	return results, nil
}

// InsertQuery builds the INSERT query without executing it.
func (m *MySQL) InsertQuery(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return insertQuery(table, data, opts, o)
}

// Insert implements the DBActions interface method to insert a new row.
//
//nolint:dupl
func (m *MySQL) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Insert",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	result, err := insert(c, table, data, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "insert", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "insert successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "insert", table, duration, rowsReturned)
	return result, nil
}

// InsertsQuery builds the INSERT query for multiple rows without executing it.
func (m *MySQL) InsertsQuery(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return insertsQuery(table, data, opts, o)
}

// Inserts implements the DBActions interface method to insert multiple new rows.
//
//nolint:dupl
func (m *MySQL) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Inserts",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	result, err := inserts(c, table, data, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "insert", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "inserts successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "insert", table, duration, rowsReturned)
	return result, nil
}

// UpsertQuery builds the UPSERT query without executing it.
func (m *MySQL) UpsertQuery(
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return upsertQuery(table, data, upsertOpts, opts, o)
}

// UpsertsQuery builds the bulk UPSERT query without executing it.
func (m *MySQL) UpsertsQuery(
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return upsertsQuery(table, data, upsertOpts, opts, o)
}

// Upsert inserts one row or updates an existing row when a uniqueness conflict occurs.
//
//nolint:dupl
func (m *MySQL) Upsert(
	ctx context.Context,
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Upsert",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("upsert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	result, err := upsert(c, table, data, upsertOpts, opts, o)
	duration := time.Since(startTime)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "upsert", table, duration, err)
		return result, err
	}
	span.SetStatus(codes.Ok, "upsert successful")
	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "upsert", table, duration, rowsReturned)
	return result, nil
}

// Upserts inserts multiple rows or updates existing rows when uniqueness conflicts occur.
//
//nolint:dupl
func (m *MySQL) Upserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Upserts",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("upsert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	result, err := upserts(c, table, data, upsertOpts, opts, o)
	duration := time.Since(startTime)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "upsert", table, duration, err)
		return result, err
	}
	span.SetStatus(codes.Ok, "upserts successful")
	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "upsert", table, duration, rowsReturned)
	return result, nil
}

// UpdateQuery builds the UPDATE query without executing it.
func (m *MySQL) UpdateQuery(
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return updateQuery(table, data, joins, conditions, opts, o)
}

// Update implements the DBActions interface method to update existing rows.
func (m *MySQL) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Update",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("update"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	result, err := update(c, table, data, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "update", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "update successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "update", table, duration, rowsReturned)
	return result, nil
}

// DeleteQuery builds the DELETE query without executing it.
func (m *MySQL) DeleteQuery(
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	return deleteQuery(table, joins, conditions, opts, o)
}

// Delete implements the DBActions interface method to delete rows.
//
//nolint:dupl
func (m *MySQL) Delete(
	ctx context.Context,
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("delete"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	o := dbOpts{
		builder:     m.queryBuilder,
		querier:     m.querier,
		errorMapper: m.errorMapper,
	}
	result, err := delete(c, table, joins, conditions, opts, o)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(c, "mysql", "delete", table, duration, err)
		return result, err
	}

	span.SetStatus(codes.Ok, "delete successful")

	rowsReturned := 0
	if result != nil {
		rowsReturned = int(result.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "delete", table, duration, rowsReturned)
	return result, nil
}

// Query implements the DBActions interface method to execute a raw query and return results as maps.
//
//nolint:dupl
func (m *MySQL) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := m.querier.QueryContext(c, query, args...)
	if err != nil {
		err := fmt.Errorf("mysql.Query: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			m.safeLogger.Error(fmt.Errorf("mysql.Query: failed to close rows: %w", err))
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		err := fmt.Errorf("mysql.Query: failed to get columns: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		m.safeLogger.Error(fmt.Errorf("mysql.Query: failed to scan rows: %w", err))
		return results, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	m.safeLogger.QuerySuccess(c, "mysql", "query", "", duration, len(results))
	return results, nil
}

// QueryRaw implements the DBActions interface method to execute a raw query and return a RowsAdapter.
//
//nolint:dupl
func (m *MySQL) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.QueryRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := m.querier.QueryContext(c, query, args...)
	if err != nil {
		err := fmt.Errorf("mysql.QueryRaw: failed to execute query: %w", m.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("mysql.QueryRaw: failed to create RowsAdapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	m.safeLogger.QuerySuccess(c, "mysql", "query", "", duration, -1)
	return ra, nil
}

// Exec implements the DBActions interface method to execute a raw statement (insert, update, delete, etc.).
func (m *MySQL) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := oh.UseTracer(ctx, "mysql.Exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("exec"),
		))
	defer span.End()
	result, err := m.querier.ExecContext(c, query, values...)
	if err != nil {
		err := fmt.Errorf("mysql.Exec: failed to execute query: %w", m.errorMapper.MapError(err))
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
		return nil, fmt.Errorf("mysql.Exec: %w", err)
	}
	duration := time.Since(startTime)

	span.SetStatus(codes.Ok, "exec completed")
	rowsAffected := int(0)
	if execResult != nil {
		rowsAffected = int(execResult.RowsAffected)
	}
	m.safeLogger.QuerySuccess(c, "mysql", "exec", "", duration, rowsAffected)
	return execResult, nil
}

func (m *MySQL) Explain(
	ctx context.Context,
	query string,
	args ...any,
) (*RowsAdapter, error) {
	c, span := oh.UseTracer(ctx, "mysql.Explain",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("explain"),
		))
	defer span.End()
	explainQuery := "EXPLAIN " + query
	rows, err := m.QueryRaw(c, explainQuery, args...)
	if err != nil {
		err := fmt.Errorf("mysql.Explain: failed to execute explain query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "explain executed")
	m.safeLogger.Debug("mysql.Explain: explain query executed successfully")
	return rows, nil
}

// WithTransaction implements the DB interface method to execute a function within a database transaction.
//
//nolint:dupl
func (m *MySQL) WithTransaction(ctx context.Context, fn func(tx Tx) error, opts ...TransactionOptions) error {
	c, span := oh.UseTracer(ctx, "mysql.WithTransaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("transaction"),
		))
	defer span.End()
	tx, err := m.Begin(c, opts...)
	if err != nil {
		err := fmt.Errorf("mysql.WithTransaction: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = runTransaction(c, "mysql.WithTransaction", tx, fn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.Error(err)
		return err
	}
	span.SetStatus(codes.Ok, "transaction committed")
	return nil
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
//
//nolint:dupl
func (m *MySQL) Commit(ctx context.Context) error {
	startTime := time.Now()
	_, span := oh.UseTracer(ctx, "mysql.Commit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("commit"),
		))
	defer span.End()
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		err := fmt.Errorf("mysql.Commit: underlying db is not *sql.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mysql", "commit", "", 0, err)
		return err
	}
	if err := sqlTX.Commit(); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("mysql.Commit: failed to commit transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mysql", "commit", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction committed")
	m.safeLogger.TransactionSuccess(ctx, "mysql", "commit")
	return nil
}

// Rollback implements the Tx interface method to rollback the transaction.
//
//nolint:dupl
func (m *MySQL) Rollback(ctx context.Context) error {
	startTime := time.Now()
	_, span := oh.UseTracer(ctx, "mysql.Rollback",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameMySQL,
			semconv.DBOperationName("rollback"),
		))
	defer span.End()
	sqlTX, ok := m.querier.(*sql.Tx)
	if !ok {
		err := fmt.Errorf("mysql.Rollback: underlying db is not *sql.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mysql", "rollback", "", 0, err)
		return err
	}
	if err := sqlTX.Rollback(); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("mysql.Rollback: failed to rollback transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		m.safeLogger.QueryError(ctx, "mysql", "rollback", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction rolled back")
	m.safeLogger.TransactionSuccess(ctx, "mysql", "rollback")
	return nil
}

// Savepoint creates a transaction savepoint.
func (m *MySQL) Savepoint(ctx context.Context, name string) error {
	return savepoint(ctx, m, name, false)
}

// RollbackToSavepoint rolls the transaction back to a savepoint.
func (m *MySQL) RollbackToSavepoint(ctx context.Context, name string) error {
	return rollbackToSavepoint(ctx, m, name, false)
}

// ReleaseSavepoint releases a transaction savepoint.
func (m *MySQL) ReleaseSavepoint(ctx context.Context, name string) error {
	return releaseSavepoint(ctx, m, name, false)
}
