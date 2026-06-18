package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	// Import the PostgreSQL driver
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"tounilab.com/vessel/db/v1/dberror"
	builder "tounilab.com/vessel/internal/pkg/builder"
	"tounilab.com/vessel/internal/pkg/otel"
	sqldialect "tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/definition"
	"tounilab.com/vessel/pkg/query/options"
)

func fromCommandTag(tag pgconn.CommandTag) *ExecResult {
	return &ExecResult{
		RowsAffected: tag.RowsAffected(),
	}
}

type pgQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// PostgresConfig holds configuration options for connecting to a PostgreSQL database.
//
// Fields include connection, pooling and application-specific settings.
//
//nolint:revive,tagalign
type PostgresConfig struct {
	Host            string        `json:"host" yaml:"host" toml:"host"`                                                                         // Database server hostname or IP
	Port            uint16        `json:"port" yaml:"port" toml:"port"`                                                                         // Database server port
	User            string        `json:"user" yaml:"user" toml:"user"`                                                                         // Username for authentication
	Password        string        `json:"password" yaml:"password" toml:"password"`                                                             // Password for authentication
	Database        string        `json:"database" yaml:"database" toml:"database"`                                                             // Database name
	SSLMode         string        `json:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty" toml:"ssl_mode,omitempty"`                               // SSL mode (disable, require, verify-ca, verify-full)
	ConnectTimeout  time.Duration `json:"connect_timeout,omitempty" yaml:"connect_timeout,omitempty" toml:"connect_timeout,omitempty"`          // Connection timeout
	PoolMaxConns    int32         `json:"pool_max_conns,omitempty" yaml:"pool_max_conns,omitempty" toml:"pool_max_conns,omitempty"`             // Maximum number of connections in the pool
	PoolMinConns    int32         `json:"pool_min_conns,omitempty" yaml:"pool_min_conns,omitempty" toml:"pool_min_conns,omitempty"`             // Minimum number of connections in the pool
	PoolMaxConnIdle time.Duration `json:"pool_max_conn_idle,omitempty" yaml:"pool_max_conn_idle,omitempty" toml:"pool_max_conn_idle,omitempty"` // Maximum idle time for a connection
	PoolMaxConnLife time.Duration `json:"pool_max_conn_life,omitempty" yaml:"pool_max_conn_life,omitempty" toml:"pool_max_conn_life,omitempty"` // Maximum lifetime of a connection
	ApplicationName string        `json:"application_name,omitempty" yaml:"application_name,omitempty" toml:"application_name,omitempty"`       // Application name for logging/tracking
	SearchPath      string        `json:"search_path,omitempty" yaml:"search_path,omitempty" toml:"search_path,omitempty"`                      // PostgreSQL schema search path
	LogLevel        string        `json:"log_level,omitempty" yaml:"log_level,omitempty" toml:"log_level,omitempty"`                            // Logging level (debug, info, warn, error)
}

// Driver returns the name of the database driver to use for this configuration.
//
//	string: The name of the database driver to use for this configuration.
func (cfg PostgresConfig) Driver() string {
	return definition.DriverPostgres
}

// DSN returns the Data Source Name (DSN) for connecting to the PostgreSQL database.
//
// This DSN includes the following options:
//
// * user: the username for authentication
// * password: the password for authentication
// * host: the hostname or IP address of the PostgreSQL server
// * port: the port number to use for the connection
// * dbname: the database name
// * sslmode: the SSL mode to use (disable, require, verify-ca, verify-full)
// * application_name: the application name for logging/tracking
// * search_path: the PostgreSQL schema search path
//
// See the pgx documentation for more information on the available options.
func (cfg PostgresConfig) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.Database,
	}

	q := url.Values{}
	if cfg.SSLMode != "" {
		q.Set("sslmode", cfg.SSLMode)
	}
	if cfg.ApplicationName != "" {
		q.Set("application_name", cfg.ApplicationName)
	}
	if cfg.SearchPath != "" {
		q.Set("search_path", cfg.SearchPath)
	}
	if cfg.ConnectTimeout > 0 {
		q.Set("connect_timeout", strconv.Itoa(int(cfg.ConnectTimeout.Seconds())))
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// Postgres is a DB implementation for PostgreSQL using pgx.
type Postgres struct {
	querier pgQuerier // PostgreSQL connection pool

	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
	safeLogger   *SafeLogger          // Nil-safe logger wrapper (created once, reused)
}

// newPostgres initializes a new Postgres connection pool using the provided config.
func newPostgres(cfg PostgresConfig, logger Logger) (*Postgres, error) {
	dsn := cfg.DSN()

	var ctx context.Context
	var cancel context.CancelFunc
	if cfg.ConnectTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), cfg.ConnectTimeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.PoolMaxConns > 0 {
		poolConfig.MaxConns = cfg.PoolMaxConns
	}
	if cfg.PoolMinConns > 0 {
		poolConfig.MinConns = cfg.PoolMinConns
	}
	if cfg.PoolMaxConnIdle > 0 {
		poolConfig.MaxConnIdleTime = cfg.PoolMaxConnIdle
	}
	if cfg.PoolMaxConnLife > 0 {
		poolConfig.MaxConnLifetime = cfg.PoolMaxConnLife
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &Postgres{
		querier:      pool,
		queryBuilder: builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverPostgres),
		safeLogger:   NewSafeLogger(logger),
	}, nil
}

func postgresCfgToDB(cfg DBConfig, logger Logger) (*Postgres, error) {
	switch c := cfg.(type) {
	case PostgresConfig:
		return newPostgres(c, logger)
	case *PostgresConfig:
		return newPostgres(*c, logger)
	default:
		return nil, fmt.Errorf("unsupported postgres config type: %T", cfg)
	}
}

// PostgresCfgToDB is an exported version of postgresCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the PostgreSQL driver implementation.
// The config type must be PostgresConfig or *PostgresConfig.
func PostgresCfgToDB(cfg DBConfig, logger Logger) (DB, error) {
	return postgresCfgToDB(cfg, logger)
}

func (pg *Postgres) PoolStats() (*PoolStatistics, error) {
	pool, ok := pg.querier.(*pgxpool.Pool)
	if !ok {
		return nil, fmt.Errorf("postgres.PoolStats: underlying connection is not *pgxpool.Pool")
	}

	pgStats := pool.Stat()
	return &PoolStatistics{
		OpenConnections:    int(pgStats.TotalConns()),
		InUse:              int(pgStats.TotalConns() - pgStats.IdleConns()),
		Idle:               int(pgStats.IdleConns()),
		MaxOpenConnections: int(pgStats.MaxConns()),
		WaitCount:          pgStats.AcquireCount(),
		WaitDuration:       pgStats.AcquireDuration(),
	}, nil
}

//nolint:dupl
func (pg *Postgres) Ping(ctx context.Context) error {
	c, span := otel.UseTracer(ctx, "postgres.Ping",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("ping"),
		))
	defer span.End()
	pgPool, ok := pg.querier.(*pgxpool.Pool)
	if !ok {
		err := fmt.Errorf("postgres.Ping: invalid querier type, expected *pgxpool.Pool")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err := pgPool.Ping(c)
	if err != nil {
		err := fmt.Errorf("postgres.Ping: failed to ping database: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(err)
		return err
	}
	span.SetStatus(codes.Ok, "ping successful")
	return nil
}

func (pg *Postgres) Begin(ctx context.Context, opts ...TransactionOptions) (Tx, error) {
	txOpts := firstTransactionOptions(opts)
	c, span := otel.UseTracer(ctx, "postgres.Begin",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("begin"),
		))
	defer span.End()
	pgPool, ok := pg.querier.(*pgxpool.Pool)
	if !ok {
		err := fmt.Errorf("postgres.Begin: invalid querier type, expected *pgxpool.Pool")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "begin", "", 0, err)
		return nil, err
	}
	t, err := pgPool.BeginTx(c, postgresTxOptions(txOpts))
	if err != nil {
		err := fmt.Errorf("postgres.Begin: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "begin", "", 0, err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "transaction begun")
	pg.safeLogger.TransactionSuccess(c, "postgres", "begin")
	return &Postgres{
		querier:      t,
		queryBuilder: pg.queryBuilder,
		safeLogger:   pg.safeLogger,
		errorMapper:  pg.errorMapper,
	}, nil
}

func postgresTxOptions(opts TransactionOptions) pgx.TxOptions {
	return pgx.TxOptions{
		IsoLevel:   postgresIsolation(opts.Isolation),
		AccessMode: postgresAccessMode(opts.ReadOnly),
	}
}

func postgresIsolation(level sql.IsolationLevel) pgx.TxIsoLevel {
	switch level {
	case sql.LevelReadUncommitted, sql.LevelReadCommitted:
		return pgx.ReadCommitted
	case sql.LevelRepeatableRead:
		return pgx.RepeatableRead
	case sql.LevelSerializable:
		return pgx.Serializable
	default:
		return ""
	}
}

func postgresAccessMode(readOnly bool) pgx.TxAccessMode {
	if readOnly {
		return pgx.ReadOnly
	}
	return ""
}

func (pg *Postgres) GetQuery(
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Get: failed to build select query: %w", err)
	}
	return query, args, nil
}

func (pg *Postgres) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Get",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	query, args, err := pg.queryBuilder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		err := fmt.Errorf("postgres.Get: failed to build select query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, 0, err)
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Get: failed to execute query (%s): %w", query, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, 0, err)
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}

	results, err := scanRows(rows, cols)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "get successful")
	pg.safeLogger.QuerySuccess(c, "postgres", "select", table, duration, len(results))
	return results, nil
}

//nolint:sqlclosecheck
func (pg *Postgres) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.GetRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	query, args, err := pg.queryBuilder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		err := fmt.Errorf("postgres.GetRaw: failed to build select query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, 0, err)
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.GetRaw: failed to execute query (%s): %w", query, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	duration := time.Since(startTime)
	if err != nil {
		err := fmt.Errorf("postgres.GetRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "getRaw successful")
	pg.safeLogger.QuerySuccess(c, "postgres", "select", table, duration, -1)
	return ra, nil
}

func (pg *Postgres) GetByIDQuery(
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (string, []any, error) {
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := pg.queryBuilder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.GetByID: failed to build select query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.GetByID",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := pg.queryBuilder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		err := fmt.Errorf("postgres.GetByID: failed to build select query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, 0, err)
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.GetByID: failed to execute query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}

	results, err := scanRows(rows, cols)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return results, err
	}

	span.SetStatus(codes.Ok, "getByID successful")
	pg.safeLogger.QuerySuccess(c, "postgres", "select", table, duration, len(results))
	return results, nil
}

//nolint:sqlclosecheck
func (pg *Postgres) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.GetByIDRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("select"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := pg.queryBuilder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		err := fmt.Errorf("postgres.GetByIDRaw: failed to build select query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "select", table, 0, err)
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.GetByIDRaw: failed to execute query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("postgres.GetByIDRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "select", table, duration, err)
		return nil, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "getByIDRaw successful")
	pg.safeLogger.QuerySuccess(c, "postgres", "select", table, duration, -1)
	return ra, nil
}

func (pg *Postgres) InsertQuery(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Insert(table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Insert: failed to build insert query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Insert",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	if err := rejectExecutingReturning("Insert", opts); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "insert", table, 0, err)
		return nil, err
	}
	query, args, err := pg.queryBuilder.Insert(table, data, opts)
	if err != nil {
		err := fmt.Errorf("postgres.Insert: failed to build insert query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "insert", table, 0, err)
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Insert: failed to execute insert query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "insert", table, duration, err)
		return nil, err
	}

	duration := time.Since(startTime)
	execResult := fromCommandTag(result)

	span.SetStatus(codes.Ok, "insert successful")
	rowsReturned := 0
	if execResult != nil {
		rowsReturned = int(execResult.RowsAffected)
	}
	pg.safeLogger.QuerySuccess(c, "postgres", "insert", table, duration, rowsReturned)
	return execResult, nil
}

func (pg *Postgres) InsertsQuery(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Inserts(table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Inserts: failed to build insert query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Inserts",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	if err := rejectExecutingReturning("Inserts", opts); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "insert", table, 0, err)
		return nil, err
	}
	query, args, err := pg.queryBuilder.Inserts(table, data, opts)
	if err != nil {
		err := fmt.Errorf("postgres.Inserts: failed to build insert query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "insert", table, 0, err)
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	duration := time.Since(startTime)

	if err != nil {
		err := fmt.Errorf("postgres.Inserts: failed to execute insert query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "insert", table, duration, err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "inserts successful")
	execResult := fromCommandTag(result)
	rowsReturned := 0
	if execResult != nil {
		rowsReturned = int(execResult.RowsAffected)
	}
	pg.safeLogger.QuerySuccess(c, "postgres", "insert", table, duration, rowsReturned)
	return execResult, nil
}

// UpsertQuery builds the UPSERT query without executing it.
func (pg *Postgres) UpsertQuery(
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Upsert(table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Upsert: failed to build upsert query: %w", err)
	}
	return query, args, nil
}

// UpsertsQuery builds the bulk UPSERT query without executing it.
func (pg *Postgres) UpsertsQuery(
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Upserts(table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Upserts: failed to build upserts query: %w", err)
	}
	return query, args, nil
}

// Upsert inserts one row or updates an existing row when a uniqueness conflict occurs.
func (pg *Postgres) Upsert(
	ctx context.Context,
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	return pg.executeUpsert(ctx, "Upsert", "upsert", table, opts, func() (string, []any, error) {
		return pg.queryBuilder.Upsert(table, data, upsertOpts, opts)
	})
}

// Upserts inserts multiple rows or updates existing rows when uniqueness conflicts occur.
func (pg *Postgres) Upserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	return pg.executeUpsert(ctx, "Upserts", "upserts", table, opts, func() (string, []any, error) {
		return pg.queryBuilder.Upserts(table, data, upsertOpts, opts)
	})
}

func (pg *Postgres) executeUpsert(
	ctx context.Context,
	operation string,
	successStatus string,
	table string,
	opts *options.QueryOptions,
	buildQuery func() (string, []any, error),
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("upsert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	if err := rejectExecutingReturning(operation, opts); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "upsert", table, 0, err)
		return nil, err
	}
	query, args, err := buildQuery()
	if err != nil {
		err := fmt.Errorf("postgres.%s: failed to build upsert query: %w", operation, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "upsert", table, 0, err)
		return nil, err
	}
	result, err := pg.querier.Exec(c, query, args...)
	duration := time.Since(startTime)
	if err != nil {
		err := fmt.Errorf("postgres.%s: failed to execute upsert query: %w", operation, pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "upsert", table, duration, err)
		return nil, err
	}
	span.SetStatus(codes.Ok, successStatus+" successful")
	execResult := fromCommandTag(result)
	rowsReturned := 0
	if execResult != nil {
		rowsReturned = int(execResult.RowsAffected)
	}
	pg.safeLogger.QuerySuccess(c, "postgres", "upsert", table, duration, rowsReturned)
	return execResult, nil
}

func (pg *Postgres) UpdateQuery(
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Update(table, data, joins, conditions, opts)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Update: failed to build update query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Update",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("update"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	if err := rejectExecutingReturning("Update", opts); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "update", table, 0, err)
		return nil, err
	}
	query, args, err := pg.queryBuilder.Update(table, data, joins, conditions, opts)
	if err != nil {
		err := fmt.Errorf("postgres.Update: failed to build update query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "update", table, 0, err)
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Update: failed to execute update query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "update", table, duration, err)
		return nil, err
	}
	duration := time.Since(startTime)
	execResult := fromCommandTag(result)

	span.SetStatus(codes.Ok, "update successful")
	rowsReturned := 0
	if execResult != nil {
		rowsReturned = int(execResult.RowsAffected)
	}
	pg.safeLogger.QuerySuccess(c, "postgres", "update", table, duration, rowsReturned)
	return execResult, nil
}

func (pg *Postgres) DeleteQuery(
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Delete(table, joins, conditions, opts)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Delete: failed to build delete query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) Delete(
	ctx context.Context,
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("delete"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	if err := rejectExecutingReturning("Delete", opts); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "delete", table, 0, err)
		return nil, err
	}
	query, args, err := pg.queryBuilder.Delete(table, joins, conditions, opts)
	if err != nil {
		err := fmt.Errorf("postgres.Delete: failed to build delete query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(c, "postgres", "delete", table, 0, err)
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Delete: failed to execute delete query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		duration := time.Since(startTime)
		pg.safeLogger.QueryError(c, "postgres", "delete", table, duration, err)
		return nil, err
	}
	duration := time.Since(startTime)
	execResult := fromCommandTag(result)

	span.SetStatus(codes.Ok, "delete successful")
	rowsReturned := 0
	if execResult != nil {
		rowsReturned = int(execResult.RowsAffected)
	}
	pg.safeLogger.QuerySuccess(c, "postgres", "delete", table, duration, rowsReturned)
	return execResult, nil
}

func (pg *Postgres) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Query",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Query: failed to execute query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(err)
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(fmt.Errorf("postgres.Query: failed to scan rows: %w", err))
		return results, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	pg.safeLogger.QuerySuccess(c, "postgres", "query", "", duration, len(results))
	return results, nil
}

//nolint:dupl,sqlclosecheck
func (pg *Postgres) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.QueryRaw",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("query"),
		))
	defer span.End()
	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.QueryRaw: failed to execute query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(err)
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("postgres.QueryRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(err)
		return nil, err
	}

	duration := time.Since(startTime)
	span.SetStatus(codes.Ok, "query executed")
	pg.safeLogger.QuerySuccess(c, "postgres", "query", "", duration, -1)
	return ra, nil
}

func (pg *Postgres) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	startTime := time.Now()
	c, span := otel.UseTracer(ctx, "postgres.Exec",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("exec"),
		))
	defer span.End()
	result, err := pg.querier.Exec(c, query, values...)
	if err != nil {
		err := fmt.Errorf("postgres.Exec: failed to execute query: %w", pg.errorMapper.MapError(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(err)
		return nil, err
	}

	execResult := fromCommandTag(result)
	duration := time.Since(startTime)

	span.SetStatus(codes.Ok, "exec completed")
	rowsAffected := int(0)
	if execResult != nil {
		rowsAffected = int(execResult.RowsAffected)
	}
	pg.safeLogger.QuerySuccess(c, "postgres", "exec", "", duration, rowsAffected)
	return execResult, nil
}

func (pg *Postgres) Explain(
	ctx context.Context,
	query string,
	args ...any,
) (*RowsAdapter, error) {
	c, span := otel.UseTracer(ctx, "postgres.Explain",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("explain"),
		))
	defer span.End()
	explainQuery := "EXPLAIN " + query
	rows, err := pg.QueryRaw(c, explainQuery, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Explain: failed to execute explain query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		pg.safeLogger.Error(err)
		return nil, err
	}

	span.SetStatus(codes.Ok, "explain executed")
	pg.safeLogger.Debug("postgres.Explain: explain query executed successfully")
	return rows, nil
}

//nolint:dupl
func (pg *Postgres) WithTransaction(ctx context.Context, fn func(tx Tx) error, opts ...TransactionOptions) error {
	c, span := otel.UseTracer(ctx, "postgres.WithTransaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("transaction"),
		))
	defer span.End()
	tx, err := pg.Begin(c, opts...)
	if err != nil {
		err := fmt.Errorf("postgres.WithTransaction: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = runTransaction(c, "postgres.WithTransaction", tx, fn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.Error(err)
		return err
	}
	span.SetStatus(codes.Ok, "transaction committed")
	return nil
}

// Close closes the database connection pool.
func (pg *Postgres) Close() error {
	if pg.querier == nil {
		return nil
	}
	pgPool, ok := pg.querier.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("postgres.Close: invalid querier type, expected *pgxpool.Pool")
	}
	pgPool.Close()
	return nil
}

//nolint:dupl
func (pg *Postgres) Commit(ctx context.Context) error {
	startTime := time.Now()
	_, span := otel.UseTracer(ctx, "postgres.Commit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("commit"),
		))
	defer span.End()
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		err := fmt.Errorf("postgres.Commit: invalid querier type, expected pgx.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(ctx, "postgres", "commit", "", 0, err)
		return err
	}
	if err := pgxTx.Commit(ctx); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("postgres.Commit: failed to commit transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(ctx, "postgres", "commit", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction committed")
	pg.safeLogger.TransactionSuccess(ctx, "postgres", "commit")
	return nil
}

//nolint:dupl
func (pg *Postgres) Rollback(ctx context.Context) error {
	startTime := time.Now()
	_, span := otel.UseTracer(ctx, "postgres.Rollback",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("rollback"),
		))
	defer span.End()
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		err := fmt.Errorf("postgres.Rollback: invalid querier type, expected pgx.Tx")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		pg.safeLogger.QueryError(ctx, "postgres", "rollback", "", 0, err)
		return err
	}
	if err := pgxTx.Rollback(ctx); err != nil {
		duration := time.Since(startTime)
		err := fmt.Errorf("postgres.Rollback: failed to rollback transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		pg.safeLogger.QueryError(ctx, "postgres", "rollback", "", duration, err)
		return err
	}

	span.SetStatus(codes.Ok, "transaction rolled back")
	pg.safeLogger.TransactionSuccess(ctx, "postgres", "rollback")
	return nil
}

// Savepoint creates a transaction savepoint.
func (pg *Postgres) Savepoint(ctx context.Context, name string) error {
	return savepoint(ctx, pg, name, false)
}

// RollbackToSavepoint rolls the transaction back to a savepoint.
func (pg *Postgres) RollbackToSavepoint(ctx context.Context, name string) error {
	return rollbackToSavepoint(ctx, pg, name, false)
}

// ReleaseSavepoint releases a transaction savepoint.
func (pg *Postgres) ReleaseSavepoint(ctx context.Context, name string) error {
	return releaseSavepoint(ctx, pg, name, false)
}
