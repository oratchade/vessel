package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Import the SQLLite driver
	_ "github.com/mattn/go-sqlite3"
	builder "tounilab.com/db-connector/query/builder"
	"tounilab.com/db-connector/query/builder/sqldialect"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

// SQLLiteConfig holds configuration for connecting to a SQLLite database.
//
// Fields include file path, access mode, and connection pool settings.
type SQLLiteConfig struct {
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

func (cfg SQLLiteConfig) Driver() string {
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
func (cfg SQLLiteConfig) DSN() string {
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
}

// newSQLLite initializes a new SQLLite connection using the provided config.
func newSQLLite(cfg SQLLiteConfig) (*SQLLite, error) {
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
	}, nil
}

func sqliteCfgToDB(cfg DBConfig) (*SQLLite, error) {
	switch c := cfg.(type) {
	case SQLLiteConfig:
		return newSQLLite(c)
	case *SQLLiteConfig:
		return newSQLLite(*c)
	default:
		return nil, fmt.Errorf("unsupported sqlite config type: %T", cfg)
	}
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
	joins []builder.Join,
	conditions condition.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return get(ctx, table, columns, joins, conditions, opts, o)
}

func (m *SQLLite) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []builder.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return getByID(ctx, table, id, joins, opts, o)
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

func (m *SQLLite) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions condition.Condition,
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
	conditions condition.Condition,
	opts *options.QueryOptions,
) (*ExecResult, error) {
	o := dbOpts{
		builder: m.queryBuilder,
		querier: m.querier,
		logger:  m.logger,
	}
	return delete(ctx, table, conditions, opts, o)
}

// func (m *SQLLite) Query(
// 	ctx context.Context,
// 	query string,
// 	opts *options.QueryOptions,
// 	args ...any,
// ) (*sql.Rows, error) {
// 	return m.querier.QueryContext(ctx, query, args...)
// }

// func (m *SQLLite) QueryRow(
// 	ctx context.Context,
// 	query string,
// 	opts *options.QueryOptions,
// 	args ...any,
// ) *sql.Row {
// 	return m.querier.QueryRowContext(ctx, query, args...)
// }

func (m *SQLLite) Exec(
	ctx context.Context,
	query string,
	opts *options.QueryOptions,
	values ...any,
) (*ExecResult, error) {
	result, err := m.querier.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return fromSQLResult(result), nil
}

func (m *SQLLite) WithTransaction(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := m.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		var e error
		if p := recover(); p != nil {
			e = tx.Rollback(ctx)
			if m.logger != nil {
				m.logger.Error("panic in transaction, rolled back", "panic", p, "error", e)
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
		return fmt.Errorf("sqlite.Commit: underlying db is not *sql.Tx")
	}
	if err := sqlTX.Rollback(); err != nil {
		return fmt.Errorf("sqlite.Rollback: failed to rollback transaction: %w", err)
	}
	return nil
}
