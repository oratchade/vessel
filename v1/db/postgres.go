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
	builder "tounilab.com/db-connector/internal/pkg/builder"
	sqldialect "tounilab.com/db-connector/internal/pkg/sqldialect"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
	"tounilab.com/db-connector/v1/db/dberror"
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
	Host string // Database server hostname or IP
	Port uint16 // Database server port
	User string // Username for authentication
	//nolint:gosec
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
	pgPool, ok := pg.querier.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("postgres.Ping: invalid querier type, expected *pgxpool.Pool")
	}

	err := pgPool.Ping(ctx)
	if err != nil {
		return fmt.Errorf("postgres.Ping: failed to ping database: %w", err)
	}
	return nil
}

func (pg *Postgres) Begin(ctx context.Context) (Tx, error) {
	pgPool, ok := pg.querier.(*pgxpool.Pool)
	if !ok {
		return nil, fmt.Errorf("postgres.Begin: invalid querier type, expected *pgxpool.Pool")
	}
	t, err := pgPool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres.Begin: failed to begin transaction: %w", err)
	}

	return &Postgres{
		querier:      t,
		queryBuilder: pg.queryBuilder,
		logger:       pg.logger,
	}, nil
}

func (pg *Postgres) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	query, args, err := pg.queryBuilder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return nil, fmt.Errorf("postgres.Get: failed to build select query: %w", err)
	}

	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Get: failed to execute query (%s): %w", query, err)
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fmt.Sprint(fd.Name)
	}

	return scanRows(rows, cols)
}

func (pg *Postgres) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	query, args, err := pg.queryBuilder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return nil, fmt.Errorf("postgres.Get: failed to build select query: %w", err)
	}

	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Get: failed to execute query (%s): %w", query, err)
	}
	defer rows.Close()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("postgres.Get: failed to create rows adapter: %w", err)
	}

	return ra, nil
}

func (pg *Postgres) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := pg.queryBuilder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetByID: failed to build select query: %w", err)
	}

	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetByID: failed to execute query: %w", err)
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fmt.Sprint(fd.Name)
	}

	return scanRows(rows, cols)
}

func (pg *Postgres) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
) (*RowsAdapter, error) {
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := pg.queryBuilder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetByIDRaw: failed to build select query: %w", err)
	}

	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetByIDRaw: failed to execute query: %w", err)
	}
	defer rows.Close()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetByIDRaw: failed to create rows adapter: %w", err)
	}

	return ra, nil
}

func (pg *Postgres) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	query, args, err := pg.queryBuilder.Insert(table, data)
	if err != nil {
		return nil, fmt.Errorf("postgres.Insert: failed to build insert query: %w", err)
	}

	result, err := pg.querier.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Insert: failed to execute insert query: %w", err)
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	query, args, err := pg.queryBuilder.Update(table, data, conditions)
	if err != nil {
		return nil, fmt.Errorf("postgres.Update: failed to build update query: %w", err)
	}

	result, err := pg.querier.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Update: failed to execute update query: %w", err)
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) Delete(
	ctx context.Context,
	table string,
	conditions cdt.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	query, args, err := pg.queryBuilder.Delete(table, conditions)
	if err != nil {
		return nil, fmt.Errorf("postgres.Delete: failed to build delete query: %w", err)
	}

	result, err := pg.querier.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Delete: failed to execute delete query: %w", err)
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Query: failed to execute query: %w", pg.errorMapper.MapError(err))
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fmt.Sprint(fd.Name)
	}

	return scanRows(rows, cols)
}

func (pg *Postgres) QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error) {
	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres.QueryRaw: failed to execute query: %w", pg.errorMapper.MapError(err))
	}
	defer rows.Close()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("postgres.QueryRaw: failed to create rows adapter: %w", err)
	}

	return ra, nil
}

func (pg *Postgres) Exec(
	ctx context.Context,
	query string,
	values ...any,
) (*ExecResult, error) {
	result, err := pg.querier.Exec(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("postgres.Exec: failed to execute query: %w", pg.errorMapper.MapError(err))
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := pg.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres.WithTransaction: failed to begin transaction: %w", err)
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(ctx)
			if pg.logger != nil {
				pg.logger.Error("postgres.WithTransaction: panic in transaction, rolled back", "panic", p, "error", e)
			}
		} else if err != nil {
			e = tx.Rollback(ctx) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf(
					"postgres.WithTransaction: execution failed with error: %w, transaction rollback: %w",
					err,
					e,
				)
			}
		} else {
			err = tx.Commit(ctx) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("postgres.WithTransaction: failed to commit transaction: %w", err)
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
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		return fmt.Errorf("postgres.Commit: invalid querier type, expected *pgxpool.Pool")
	}
	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres.Commit: failed to commit transaction: %w", err)
	}
	return nil
}

func (pg *Postgres) Rollback(ctx context.Context) error {
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		return fmt.Errorf("postgres.Rollback: invalid querier type, expected *pgxpool.Pool")
	}
	if err := pgxTx.Rollback(ctx); err != nil {
		return fmt.Errorf("postgres.Rollback: failed to rollback transaction: %w", err)
	}
	return nil
}
