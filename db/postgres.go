package db

import (
	"context"
	"fmt"
	"time"

	// Import the PostgreSQL driver
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	builder "tounilab.com/db-connector/query/builder"
	sqldialect "tounilab.com/db-connector/query/builder/sqldialect"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

type pgQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// DBConfig holds all configuration options for connecting to a PostgreSQL database.
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
	return fmt.Sprintf(
		"user=%s password=%s host=%s port=%d dbname=%s sslmode=%s application_name=%s search_path=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode, cfg.ApplicationName, cfg.SearchPath,
	)
}

type Postgres struct {
	querier      pgQuerier            // PostgreSQL connection pool
	queryBuilder builder.QueryBuilder // Query builder for constructing SQL queries
	logger       Logger               // Logger for logging database operations
}

// NewPostgres initializes a new Postgres connection pool using the provided config.
func NewPostgres(cfg PostgresConfig) (*Postgres, error) {
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
	}, nil
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
	joins []builder.Join,
	conditions condition.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	query, args, err := pg.queryBuilder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values() // pgx.Rows.Values() returns []any
		if err != nil {
			return nil, fmt.Errorf("failed to read row values: %w", err)
		}
		fds := rows.FieldDescriptions()
		rowData := make(map[string]any, len(fds))
		for i, fd := range fds {
			rowData[fd.Name] = values[i]
		}
		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil && pg.logger != nil {
		pg.logger.Error("failed to get rows", "error", err)
	}

	return results, nil
}

func (pg *Postgres) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []builder.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	cdt := &condition.Expr{}
	cdt.Column("id").Value(id)

	query, args, err := pg.queryBuilder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := pg.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values() // pgx.Rows.Values() returns []any
		if err != nil {
			return nil, fmt.Errorf("failed to read row values: %w", err)
		}
		fds := rows.FieldDescriptions()
		rowData := make(map[string]any, len(fds))
		for i, fd := range fds {
			rowData[fd.Name] = values[i]
		}
		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil && pg.logger != nil {
		pg.logger.Error("failed to get rows", "error", err)
	}

	return results, nil
}

func (pg *Postgres) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	query, args, err := pg.queryBuilder.Insert(table, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	result, err := pg.querier.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute insert query: %w", err)
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions condition.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	query, args, err := pg.queryBuilder.Update(table, data, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := pg.querier.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute update query: %w", err)
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) Delete(
	ctx context.Context,
	table string,
	conditions condition.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	query, args, err := pg.queryBuilder.Delete(table, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build delete query: %w", err)
	}

	result, err := pg.querier.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute delete query: %w", err)
	}
	return fromCommandTag(result), nil
}

// func (pg *Postgres) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {}
// func (pg *Postgres) QueryRow(ctx context.Context, query string, args ...any) *sql.Row        {}

func (pg *Postgres) Exec(
	ctx context.Context,
	query string,
	opts *options.QueryOptions,
	values ...any,
) (*ExecResult, error) {
	result, err := pg.querier.Exec(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return fromCommandTag(result), nil
}

func (pg *Postgres) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := pg.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(ctx)
			if pg.logger != nil {
				pg.logger.Error("panic in transaction, rolled back", "panic", p, "error", e)
			}
		} else if err != nil {
			e = tx.Rollback(ctx) // err is non-nil; don't change it
			if e != nil {
				err = fmt.Errorf("execution failed with error: %w, transaction rollback: %w", err, e)
			}
		} else {
			err = tx.Commit(ctx) // err is nil; if Commit returns error update err
			if err != nil {
				err = fmt.Errorf("failed to commit transaction: %w", err)
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
		return fmt.Errorf("postgres.Close: invalid querier type, expected *pgxpool.Pool")
	}
	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (pg *Postgres) Rollback(ctx context.Context) error {
	pgxTx, ok := pg.querier.(pgx.Tx)
	if !ok {
		return fmt.Errorf("postgres.Close: invalid querier type, expected *pgxpool.Pool")
	}
	if err := pgxTx.Rollback(ctx); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}
