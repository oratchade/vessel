// Package db provides database abstraction interfaces and implementations for multiple database engines.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
	"tounilab.com/db-connector/v1/db/plugin"
)

// ExecResult holds metadata returned by mutation statements such as INSERT,
// UPDATE and DELETE (last insert id when available and number of rows affected).
type ExecResult struct {
	LastInsertID int64
	RowsAffected int64
}

// PoolStatistics exposes connection pool metrics for diagnostics and monitoring.
type PoolStatistics struct {
	OpenConnections    int           // Total number of open connections in the pool
	InUse              int           // Number of connections currently in use
	Idle               int           // Number of idle connections available
	MaxOpenConnections int           // Maximum number of open connections allowed
	WaitCount          int64         // Cumulative count of wait operations for connections
	WaitDuration       time.Duration // Total time spent waiting for connections
	MaxIdleClosed      int64         // Cumulative count of connections closed due to SetMaxIdleConns
	MaxIdleTimeClosed  int64         // Cumulative count of connections closed due to SetConnMaxIdleTime
	MaxLifetimeClosed  int64         // Cumulative count of connections closed due to SetConnMaxLifetime
}

func fromSQLResult(res sql.Result) *ExecResult {
	liid, _ := res.LastInsertId() // ignore unsupported err
	ra, _ := res.RowsAffected()
	return &ExecResult{
		LastInsertID: liid,
		RowsAffected: ra,
	}
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=db DBConfig

// DBConfig represents configuration needed to create a DB connection. It
// exposes the driver name and a DSN builder for connecting to the database.
type DBConfig interface {
	// Driver returns the database driver name (e.g., "mysql", "postgres", "SQLLite3").
	Driver() string
	// DSN returns the Data Source Name (DSN) for connecting to the database.
	DSN() string
}

// NewDB returns a DB implementation for the provided DBConfig.
// It first checks the plugin registry for custom drivers, then falls back to
// the built-in driver implementations (MySQL, PostgreSQL, SQLite, MSSQL).
// Custom drivers can be registered via the plugin package.
func NewDB(cfg DBConfig, logger Logger) (DB, error) {
	driverName := cfg.Driver()

	// Check plugin registry first - allows custom drivers to override or extend built-in ones
	if factory, ok := plugin.Get(driverName); ok {
		result, err := factory.Create(context.Background(), cfg)
		if err != nil {
			return nil, fmt.Errorf("plugin driver %q failed: %w", driverName, err)
		}
		// Type assert the result to DB interface
		db, ok := result.(DB)
		if !ok {
			return nil, fmt.Errorf("plugin driver %q returned invalid DB type: %T", driverName, result)
		}
		return db, nil
	}

	// Fall back to built-in drivers
	switch driverName {
	case definition.DriverMySQL:
		return mysqlCfgToDB(cfg)
	case definition.DriverPostgres, definition.DriverPostgresAlias:
		return postgresCfgToDB(cfg)
	case definition.DriverSQLLite, definition.DriverSQLiteAlias:
		return sqliteCfgToDB(cfg)
	case definition.DriverMSSQL, definition.DriverMSSQLAlias:
		return mssqlCfgToDB(cfg)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driverName)
	}
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=db DBActions

// DBActions defines the core, context-aware data access operations that any
// database connection or transaction must provide. It is an implementation-
// agnostic contract used by higher-level code (builders and services) to perform
// SQL operations without depending on driver details.
//
// Expectations:
//   - Context propagation: every method accepts a context.Context for
//     cancellation and deadlines.
//   - Parameter ordering: implementations must preserve placeholder↔arg
//     ordering (convention: condition args first, then option args).
//   - Identifier handling: implementations should use dialect helpers to quote
//     identifiers and avoid injection.
//   - Mutation results: mutation methods return *ExecResult so callers can
//     inspect execution metadata consistently across drivers.
//   - Concurrency: implementations must be safe for concurrent use.
//
// Transactions:
//   - Tx implementations embed DBActions and add Commit/Rollback lifecycle methods.
//   - WithTransaction helpers should Begin, execute the provided function, and
//     Commit on success or Rollback on error/panic.
//
// Testing guidance:
//   - Keep DBActions small and mockable; add integration tests that verify
//     SQL+args ordering and option rendering for each dialect.
type DBActions interface {
	// Get retrieves multiple rows from the specified table, with optional SQL joins and query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the main table to query.
	//   columns: List of columns to select.
	//   joins: Slice of Join structs describing SQL JOIN clauses.
	//   conditions: Query conditions for filtering results.
	//   opts: Optional query parameters (limit, offset, order, etc.).
	//
	// Returns:
	//   []map[string]any: Slice of rows, each as a map of column names to values.
	//   error: Error if the query fails.
	Get(
		ctx context.Context,
		table string,
		columns []string,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) ([]map[string]any, error)

	GetRaw(
		ctx context.Context,
		table string,
		columns []string,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (*RowsAdapter, error)

	// GetByID retrieves a single row by its primary key, with optional SQL joins and query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the main table to query.
	//   id: Value of the primary key.
	//   joins: Slice of Join structs describing SQL JOIN clauses.
	//   opts: Optional query parameters (limit, offset, order, etc.).
	//
	// Returns:
	//   map[string]any: Row as a map of column names to values.
	//   error: Error if the query fails or no row is found.
	GetByID(
		ctx context.Context,
		table string,
		id any,
		joins []cdt.Join,
		opts *options.QueryOptions,
	) ([]map[string]any, error)

	GetByIDRaw(
		ctx context.Context,
		table string,
		id any,
		joins []cdt.Join,
		opts *options.QueryOptions,
	) (*RowsAdapter, error)

	// Insert adds a new row to the specified table, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to insert into.
	//   data: Map of column names to values for the new row.
	//   opts: Optional query parameters (e.g., returning columns).
	//
	// Returns:
	//   ExecResult: Result of the insert operation.
	//   error: Error if the insert fails.
	Insert(ctx context.Context, table string, data map[string]any, opts *options.QueryOptions) (*ExecResult, error)

	// Inserts adds multiple new rows to the specified table, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to insert into.
	//   data: Slice of maps, each representing a row to insert with column names as keys and values as values.
	//   opts: Optional query parameters (e.g., returning columns).
	//
	// Returns:
	//   ExecResult: Result of the insert operation.
	//   error: Error if the insert fails.
	Inserts(ctx context.Context, table string, data []map[string]any, opts *options.QueryOptions) (*ExecResult, error)

	// Update modifies existing rows in the specified table, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to update.
	//   data: Map of column names to new values.
	//   conditions: Query conditions to select rows to update.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   ExecResult: Result of the update operation.
	//   error: Error if the update fails.
	Update(
		ctx context.Context,
		table string,
		data map[string]any,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (*ExecResult, error)

	// Delete removes rows from the specified table, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to delete from.
	//   conditions: Query conditions to select rows to delete.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   ExecResult: Result of the delete operation.
	//   error: Error if the delete fails.
	Delete(ctx context.Context, table string, conditions cdt.Condition, opts *options.QueryOptions) (*ExecResult, error)

	// Query executes a raw SQL query and returns multiple rows, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL query string.
	//   args: Arguments for parameterized query.
	//
	// Returns:
	//   *sql.Rows: Result rows from the query.
	//   error: Error if the query fails.
	Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)

	QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error)

	// Exec executes a raw SQL statement (insert, update, delete, etc.), with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL statement.
	//   args: Arguments for parameterized statement.
	//
	// Returns:
	//   ExecResult: Result of the execution.
	//   error: Error if the execution fails.
	Exec(ctx context.Context, query string, args ...any) (*ExecResult, error)
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=db DB

// DB represents a database connection interface.
// Each method is documented with its purpose, parameters, and expected return values.
// Now supports SQL joins via the joins parameter and accepts options.QueryOptions for extensibility.
type DB interface {
	DBActions

	// Ping checks the database connection.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//
	// Returns:
	//   error: Error if the connection fails.
	Ping(ctx context.Context) error

	// PoolStats returns current connection pool statistics.
	// Useful for monitoring and diagnosing connection pool issues.
	//
	// Returns:
	//   *PoolStatistics: Current pool metrics including open, idle, and in-use connections.
	//   error: Error if statistics cannot be retrieved.
	PoolStats() (*PoolStatistics, error)

	// Begin starts a new transaction and returns a Tx.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//
	// Returns:
	//   Tx: Transaction object to execute queries within the transaction.
	//   error: Error if starting the transaction fails.
	Begin(ctx context.Context) (Tx, error)

	// WithTransaction executes a function within a database transaction.
	//
	// Implementation note:
	//   This helper should call Begin(ctx) to start a Tx, pass the Tx to fn, and commit the transaction
	//   if fn returns nil. If fn returns an error, the transaction should be rolled back.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   fn: Function to execute, receiving a Tx transaction object.
	//
	// Returns:
	//   error: Error if the transaction fails or is rolled back.
	WithTransaction(ctx context.Context, fn func(Tx) error) error

	// Close closes the database connection.
	//
	// Returns:
	//   error: Error if the closure fails.
	Close() error
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=db Tx

// Tx represents a database transaction. All methods accept context so deadlines and cancellations propagate.
type Tx interface {
	DBActions

	// Commit commits the transaction.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//
	// Returns:
	//   error: An error if the commit fails.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//
	// Returns:
	//   error: An error if the rollback fails.
	Rollback(ctx context.Context) error
}

// ScanRowsTo takes a RowsAdapter and scans the rows into a slice of T.
//
// The slice of T is returned, and an error if the scan fails.
//
// ScanRowsTo expects T to be a struct or pointer to a struct.
// The Columns of the RowsAdapter are mapped to fields in T
// by column name, ignoring case. If a column does not map to a field in T,
// the value is skipped.
//
// The order of the columns in the RowsAdapter does not affect the order of
// the fields in T. The order of the fields in T is determined by the order of
// the fields in the reflect.Struct.
//
//nolint:cyclop
func ScanRowsTo[T any](ra *RowsAdapter) ([]T, error) {
	var cols []string
	var err error

	cols, err = ra.columns()
	if err != nil {
		return nil, fmt.Errorf("scanRowsTo: failed to get columns: %w", err)
	}

	var out []T

	defer func() { _ = ra.Close() }()

	// prepare scan destinations
	vals, ptrs := makeScanPtrs(len(cols))

	// reflect type information for T
	tType := reflect.TypeOf((*T)(nil)).Elem()
	isPtr := false
	if tType.Kind() == reflect.Ptr {
		isPtr = true
		tType = tType.Elem()
	}
	if tType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("scanRowsTo: T must be a struct or pointer to struct")
	}

	// build mapping column -> struct field index
	fieldMap := buildFieldMap(tType)

	for ra.next() {
		if err := ra.scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		var itemVal reflect.Value
		var itemPtr reflect.Value
		if isPtr {
			itemPtr = reflect.New(tType)
			itemVal = itemPtr.Elem()
		} else {
			itemVal = reflect.New(tType).Elem()
		}

		for i, col := range cols {
			raw := vals[i]
			if raw == nil {
				continue
			}
			colKey := strings.ToLower(col)
			if fi, ok := fieldMap[colKey]; ok {
				f := itemVal.Field(fi)
				if !f.CanSet() {
					continue
				}
				err := setFieldFromValue(f, raw)
				if err != nil {
					return nil, fmt.Errorf("scanRowsTo: failed to set field %s: %w", col, err)
				}
			}
		}

		if isPtr {
			out = append(out, itemPtr.Interface().(T))
		} else {
			out = append(out, itemVal.Interface().(T))
		}
	}

	if err := ra.err(); err != nil {
		return nil, fmt.Errorf("scanRowsTo: rows iteration failed: %w", err)
	}
	return out, nil
}
