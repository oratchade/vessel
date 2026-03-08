// Package db provides database abstraction interfaces and implementations for multiple database engines.
package db

import (
	"context"
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
	"tounilab.com/db-connector/db/v1/dberror"
	builder "tounilab.com/db-connector/internal/pkg/builder"
	"tounilab.com/db-connector/internal/pkg/otel"
	sqldialect "tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

func fromCommandTag(tag pgconn.CommandTag) *ExecResult {
	return &ExecResult{
		LastInsertID: 0, // use RETURNING if you want this
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
type PostgresConfig struct {
	Host            string        // Database server hostname or IP
	Port            uint16        // Database server port
	User            string        // Username for authentication
	Password        string        // Password for authentication
	Database        string        // Database name
	SSLMode         string        // SSL mode (disable, require, verify-ca, verify-full)
	ConnectTimeout  time.Duration // Connection timeout
	PoolMaxConns    int32         // Maximum number of connections in the pool
	PoolMinConns    int32         // Minimum number of connections in the pool
	PoolMaxConnIdle time.Duration // Maximum idle time for a connection
	PoolMaxConnLife time.Duration // Maximum lifetime of a connection
	ApplicationName string        // Application name for logging/tracking
	SearchPath      string        // PostgreSQL schema search path
	LogLevel        string        // Logging level (debug, info, warn, error)
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
	querier      pgQuerier            // PostgreSQL connection pool
	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
	errorMapper  dberror.ErrorMapper  // Error mapper for standardizing database errors
}

// newPostgres initializes a new Postgres connection pool using the provided config.
func newPostgres(cfg PostgresConfig) (*Postgres, error) {
	dsn := cfg.DSN()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	poolConfig.MaxConns = cfg.PoolMaxConns
	poolConfig.MinConns = cfg.PoolMinConns
	poolConfig.MaxConnIdleTime = cfg.PoolMaxConnIdle
	poolConfig.MaxConnLifetime = cfg.PoolMaxConnLife

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &Postgres{
		querier:      pool,
		queryBuilder: builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{}),
		errorMapper:  dberror.GetMapper(definition.DriverPostgres),
	}, nil
}

func postgresCfgToDB(cfg DBConfig) (*Postgres, error) {
	switch c := cfg.(type) {
	case PostgresConfig:
		return newPostgres(c)
	case *PostgresConfig:
		return newPostgres(*c)
	default:
		return nil, fmt.Errorf("unsupported postgres config type: %T", cfg)
	}
}

// PostgresCfgToDB is an exported version of postgresCfgToDB for use by plugin implementations.
// Plugin authors can use this to reuse the PostgreSQL driver implementation.
// The config type must be PostgresConfig or *PostgresConfig.
func PostgresCfgToDB(cfg DBConfig) (DB, error) {
	return postgresCfgToDB(cfg)
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
		return err
	}
	span.SetStatus(codes.Ok, "ping successful")
	return nil
}

func (pg *Postgres) Begin(ctx context.Context) (Tx, error) {
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
		return nil, err
	}
	t, err := pgPool.Begin(c)
	if err != nil {
		err := fmt.Errorf("postgres.Begin: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "transaction begun")
	return &Postgres{
		querier:      t,
		queryBuilder: pg.queryBuilder,
		logger:       pg.logger,
	}, nil
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
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Get: failed to execute query (%s): %w", query, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fmt.Sprint(fd.Name)
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "get successful")
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
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.GetRaw: failed to execute query (%s): %w", query, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("postgres.GetRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "getRaw successful")
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
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.GetByID: failed to execute query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fmt.Sprint(fd.Name)
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return results, err
	}
	span.SetStatus(codes.Ok, "getByID successful")
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
		return nil, err
	}

	rows, err := pg.querier.Query(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.GetByIDRaw: failed to execute query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("postgres.GetByIDRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "getByIDRaw successful")
	return ra, nil
}

func (pg *Postgres) InsertQuery(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Insert(table, data)
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
	c, span := otel.UseTracer(ctx, "postgres.Insert",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	query, args, err := pg.queryBuilder.Insert(table, data)
	if err != nil {
		err := fmt.Errorf("postgres.Insert: failed to build insert query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Insert: failed to execute insert query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "insert successful")
	return fromCommandTag(result), nil
}

func (pg *Postgres) InsertsQuery(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Inserts(table, data)
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
	c, span := otel.UseTracer(ctx, "postgres.Inserts",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("insert"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	query, args, err := pg.queryBuilder.Inserts(table, data)
	if err != nil {
		err := fmt.Errorf("postgres.Inserts: failed to build insert query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Inserts: failed to execute insert query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "inserts successful")
	return fromCommandTag(result), nil
}

func (pg *Postgres) UpdateQuery(
	table string,
	data map[string]any,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Update(table, data, conditions)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Update: failed to build update query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	c, span := otel.UseTracer(ctx, "postgres.Update",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("update"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	query, args, err := pg.queryBuilder.Update(table, data, conditions)
	if err != nil {
		err := fmt.Errorf("postgres.Update: failed to build update query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Update: failed to execute update query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "update successful")
	return fromCommandTag(result), nil
}

func (pg *Postgres) DeleteQuery(
	table string,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	query, args, err := pg.queryBuilder.Delete(table, conditions)
	if err != nil {
		return "", nil, fmt.Errorf("postgres.Delete: failed to build delete query: %w", err)
	}

	return query, args, nil
}

func (pg *Postgres) Delete(
	ctx context.Context,
	table string,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	c, span := otel.UseTracer(ctx, "postgres.Delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("delete"),
			semconv.DBCollectionName(table),
		))
	defer span.End()
	query, args, err := pg.queryBuilder.Delete(table, conditions)
	if err != nil {
		err := fmt.Errorf("postgres.Delete: failed to build delete query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result, err := pg.querier.Exec(c, query, args...)
	if err != nil {
		err := fmt.Errorf("postgres.Delete: failed to execute delete query: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "delete successful")
	return fromCommandTag(result), nil
}

func (pg *Postgres) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
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
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fmt.Sprint(fd.Name)
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

//nolint:dupl,sqlclosecheck
func (pg *Postgres) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
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
		return nil, err
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		err := fmt.Errorf("postgres.QueryRaw: failed to create rows adapter: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "query executed")
	return ra, nil
}

func (pg *Postgres) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
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
		return nil, err
	}
	span.SetStatus(codes.Ok, "exec completed")
	return fromCommandTag(result), nil
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
		return nil, err
	}
	span.SetStatus(codes.Ok, "explain executed")
	return rows, nil
}

//nolint:dupl
func (pg *Postgres) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	c, span := otel.UseTracer(ctx, "postgres.WithTransaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("transaction"),
		))
	defer span.End()
	tx, err := pg.Begin(c)
	if err != nil {
		err := fmt.Errorf("postgres.WithTransaction: failed to begin transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(c)
			if pg.logger != nil {
				pg.logger.Error("postgres.WithTransaction: panic in transaction, rolled back", "panic", p, "error", e)
			}
			span.RecordError(fmt.Errorf("panic in transaction: %v", p))
			span.SetStatus(codes.Error, "panic occurred in transaction")
		} else if err != nil {
			e = tx.Rollback(c) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf(
					"postgres.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			err = tx.Commit(c) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("postgres.WithTransaction: failed to commit transaction: %w", err)
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

func (pg *Postgres) Commit(ctx context.Context) error {
	_, span := otel.UseTracer(ctx, "postgres.Commit",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("commit"),
		))
	defer span.End()
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		err := fmt.Errorf("postgres.Commit: invalid querier type, expected *pgxpool.Pool")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := pgxTx.Commit(ctx); err != nil {
		err := fmt.Errorf("postgres.Commit: failed to commit transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "transaction committed")
	return nil
}

func (pg *Postgres) Rollback(ctx context.Context) error {
	_, span := otel.UseTracer(ctx, "postgres.Rollback",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("rollback"),
		))
	defer span.End()
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		err := fmt.Errorf("postgres.Rollback: invalid querier type, expected *pgxpool.Pool")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if err := pgxTx.Rollback(ctx); err != nil {
		err := fmt.Errorf("postgres.Rollback: failed to rollback transaction: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "transaction rolled back")
	return nil
}
