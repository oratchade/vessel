// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"tounilab.com/fabric/db/v1/plugin"
	"tounilab.com/fabric/internal/pkg/otel"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

// ExecResult holds metadata returned by mutation statements such as INSERT,
// UPDATE and DELETE (last insert id when available and number of rows affected).
type ExecResult struct {
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

func fromSQLResult(res sql.Result) (*ExecResult, error) {
	ra, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("fromSQLResult: failed to get rows affected: %w", err)
	}
	return &ExecResult{RowsAffected: ra}, nil
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 DBConfig

// DBConfig represents configuration needed to create a DB connection. It
// exposes the driver name and a DSN builder for connecting to the database.
type DBConfig interface {
	// Driver returns the database driver name (e.g., "mysql", "postgres", "sqlite").
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
			return nil, fmt.Errorf("NewDB: plugin driver %q returned invalid type: %T", driverName, result)
		}
		return db, nil
	}

	// Fall back to built-in drivers
	switch driverName {
	case definition.DriverMySQL:
		return mysqlCfgToDB(cfg, logger)
	case definition.DriverPostgres, definition.DriverPostgresAlias:
		return postgresCfgToDB(cfg, logger)
	case definition.DriverSQLite:
		return sqliteCfgToDB(cfg, logger)
	case definition.DriverMSSQL, definition.DriverMSSQLAlias:
		return mssqlCfgToDB(cfg, logger)
	default:
		return nil, fmt.Errorf("NewDB: unsupported driver: %s", driverName)
	}
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 reader

// reader provides context-aware read operations for querying data without modifying it.
// Methods accept context.Context for cancellation and deadlines.
type reader interface {
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

	// QueryRaw executes a raw SQL query and returns a RowsAdapter for streaming access to result rows,
	// without materializing all rows into memory. This is useful for large result sets.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL query string.
	//   args: Arguments for parameterized query.
	//
	// Returns:
	//   *RowsAdapter: Raw row adapter for streaming result access.
	//   error: Error if the query fails.
	QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error)
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 writer

// writer provides context-aware write operations for modifying data.
// Methods accept context.Context for cancellation and deadlines.
type writer interface {
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

	// Update modifies existing rows in the specified table, with optional SQL joins and query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to update.
	//   data: Map of column names to new values.
	//   joins: Slice of Join structs describing SQL JOIN clauses (optional, may be nil or empty).
	//   conditions: Query conditions to select rows to update.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   ExecResult: Result of the update operation.
	//   error: Error if the update fails or unsupported operation is attempted
	//          (e.g., UPDATE with JOINs on certain databases).
	Update(
		ctx context.Context,
		table string,
		data map[string]any,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (*ExecResult, error)

	// Delete removes rows from the specified table, with optional SQL joins and query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to delete from.
	//   joins: Slice of Join structs describing SQL JOIN clauses (optional, may be nil or empty).
	//          Note: SQLite does not support DELETE with JOINs and will return an error if joins are provided.
	//   conditions: Query conditions to select rows to delete.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   ExecResult: Result of the delete operation.
	//   error: Error if the delete fails, or if DELETE with JOINs is attempted on
	//          unsupported databases like SQLite.
	Delete(
		ctx context.Context,
		table string,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (*ExecResult, error)

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

type upserter interface {
	// Upsert inserts one row or applies conflict behavior when a uniqueness conflict occurs.
	Upsert(
		ctx context.Context,
		table string,
		data map[string]any,
		upsertOpts *options.UpsertOptions,
		opts *options.QueryOptions,
	) (*ExecResult, error)

	// UpsertQuery builds an UPSERT statement without executing it.
	UpsertQuery(
		table string,
		data map[string]any,
		upsertOpts *options.UpsertOptions,
		opts *options.QueryOptions,
	) (string, []any, error)
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 introspector

// introspector provides access to SQL queries without executing them.
// Useful for query introspection, logging, validation, and preview purposes.
type introspector interface {
	// GetQuery constructs and returns the SQL SELECT query and arguments that would be executed by Get,
	// without actually executing the query. This allows callers to inspect or log the query before execution.
	//
	// Parameters:
	//   table: Name of the main table to query.
	//   columns: List of columns to select.
	//   joins: Slice of Join structs describing SQL JOIN clauses.
	//   conditions: Query conditions for filtering results.
	//   opts: Optional query parameters (limit, offset, order, etc.).
	//
	// Returns:
	//   string: The SQL query string.
	//   []any: The query arguments/parameters.
	//   error: Error if the query cannot be built.
	GetQuery(
		table string,
		columns []string,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (string, []any, error)

	// GetByIDQuery constructs and returns the SQL SELECT query and arguments that would be executed by GetByID,
	// without actually executing the query. This allows callers to inspect or log the query before execution.
	//
	// Parameters:
	//   table: Name of the main table to query.
	//   id: Value of the primary key.
	//   joins: Slice of Join structs describing SQL JOIN clauses.
	//   opts: Optional query parameters (limit, offset, order, etc.).
	//
	// Returns:
	//   string: The SQL query string.
	//   []any: The query arguments/parameters.
	//   error: Error if the query cannot be built.
	GetByIDQuery(
		table string,
		id any,
		joins []cdt.Join,
		opts *options.QueryOptions,
	) (string, []any, error)

	// InsertQuery constructs and returns the SQL INSERT query and arguments that would be executed by Insert,
	// without actually executing the query. This allows callers to inspect or log the query before execution.
	//
	// Parameters:
	//   table: Name of the table to insert into.
	//   data: Map of column names to values for the new row.
	//   opts: Optional query parameters (e.g., returning columns).
	//
	// Returns:
	//   string: The SQL query string.
	//   []any: The query arguments/parameters.
	//   error: Error if the query cannot be built.
	InsertQuery(
		table string,
		data map[string]any,
		opts *options.QueryOptions,
	) (string, []any, error)

	// InsertsQuery constructs and returns the SQL INSERT query and arguments that would be executed by Inserts,
	// without actually executing the query. This allows callers to inspect or log the query before execution.
	//
	// Parameters:
	//   table: Name of the table to insert into.
	//   data: Slice of maps, each representing a row to insert with column names as keys and values as values.
	//   opts: Optional query parameters (e.g., returning columns).
	//
	// Returns:
	//   string: The SQL query string.
	//   []any: The query arguments/parameters.
	//   error: Error if the query cannot be built.
	InsertsQuery(
		table string,
		data []map[string]any,
		opts *options.QueryOptions,
	) (string, []any, error)

	// UpdateQuery constructs and returns the SQL UPDATE query and arguments that would be executed by Update,
	// without actually executing the query. This allows callers to inspect or log the query before execution.
	//
	// Parameters:
	//   table: Name of the table to update.
	//   data: Map of column names to new values.
	//   joins: Slice of Join structs describing SQL JOIN clauses (optional, may be nil or empty).
	//   conditions: Query conditions to select rows to update.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   string: The SQL query string.
	//   []any: The query arguments/parameters.
	//   error: Error if the query cannot be built.
	UpdateQuery(
		table string,
		data map[string]any,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (string, []any, error)

	// DeleteQuery constructs and returns the SQL DELETE query and arguments that would be executed by Delete,
	// without actually executing the query. This allows callers to inspect or log the query before execution.
	//
	// Parameters:
	//   table: Name of the table to delete from.
	//   joins: Slice of Join structs describing SQL JOIN clauses (optional, may be nil or empty).
	//          Note: SQLite does not support DELETE with JOINs and will return an error if joins are provided.
	//   conditions: Query conditions to select rows to delete.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   string: The SQL query string.
	//   []any: The query arguments/parameters.
	//   error: Error if the query cannot be built, or if an unsupported operation is attempted.
	DeleteQuery(
		table string,
		joins []cdt.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (string, []any, error)

	// Explain executes an EXPLAIN query on the provided SQL query and returns execution plan details.
	// This is useful for analyzing query performance and understanding how the database executes the query.
	// The query parameter should be generated via one of the xxxQuery methods to ensure proper SQL construction.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: SQL query string (commonly generated via GetQuery, InsertQuery, etc.).
	//   args: Arguments for parameterized query (from the xxxQuery method).
	//
	// Returns:
	//   *RowsAdapter: Raw adapter containing execution plan rows.
	//   error: Error if the explain query fails.
	Explain(
		ctx context.Context,
		query string,
		args ...any,
	) (*RowsAdapter, error)
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 transactional

// transactional provides transaction management operations.
type transactional interface {
	// Begin starts a new transaction and returns a Tx.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//
	// Returns:
	//   Tx: Transaction object to execute queries within the transaction.
	//   error: Error if starting the transaction fails.
	Begin(ctx context.Context, opts ...TransactionOptions) (Tx, error)

	// WithTransaction executes a function within a database transaction.
	//
	// Implementation note:
	//   This helper should call Begin(ctx) to start a Tx, pass the Tx to fn, and commit the transaction
	//   if fn returns nil. If fn returns an error, the transaction should be rolled back. If fn panics,
	//   the transaction should be rolled back and the panic should be returned as an error.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   fn: Function to execute, receiving a Tx transaction object.
	//
	// Returns:
	//   error: Error if the transaction fails or is rolled back.
	WithTransaction(ctx context.Context, fn func(Tx) error, opts ...TransactionOptions) error
}

type savepointer interface {
	Savepoint(ctx context.Context, name string) error
	RollbackToSavepoint(ctx context.Context, name string) error
	ReleaseSavepoint(ctx context.Context, name string) error
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 healthCheck

// healthCheck provides connection health diagnostics and monitoring.
type healthCheck interface {
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
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 closer

// closer provides resource cleanup.
type closer interface {
	// Close closes the database connection.
	//
	// Returns:
	//   error: Error if the closure fails.
	Close() error
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 DB

// DB represents a complete database connection interface combining read, write,
// introspection, transaction management, health checks, and resource cleanup.
type DB interface {
	reader
	writer
	upserter
	introspector
	transactional
	healthCheck
	closer
}

//go:generate mockgen -source=db.go -destination=db_mocks.go -package=v1 Tx

// Tx represents a database transaction supporting all query operations plus commit/rollback.
type Tx interface {
	reader
	writer
	upserter
	introspector
	savepointer

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

// ScanRowsTo is a generic, type-safe function that scans rows into a slice of struct T.
//
// Type Safety:
// ScanRowsTo uses reflection to map database columns to struct fields by name (case-insensitive).
// This provides compile-time type checking while avoiding manual Scan() calls for each field.
//
// Struct Requirements:
// T must be a struct or pointer-to-struct. Exported fields are mapped to columns;
// unexported fields are ignored. If a column has no matching struct field, it is skipped.
// If a struct field has no matching column, it retains its zero value.
//
// Column Mapping:
// Mapping is case-insensitive and by name only (no struct tags required, though they may be supported).
// The order of columns in RowsAdapter does not affect the order in T; struct field order is preserved.
//
// Example:
//
//	type User struct {
//		ID   int    `db:"id"`
//		Name string `db:"name"`
//	}
//
//	userList, err := ScanRowsTo[User](ctx, rows)
//	if err != nil { return err }
//	// userList is now a fully typed, safe slice of Users
//
// Error Handling:
// Returns an error if column retrieval, reflection, or scanning fails.
// Users should explicitly check errors to avoid partial scans.
//
//nolint:cyclop
func ScanRowsTo[T any](ctx context.Context, ra RowsProvider) ([]T, error) {
	_, span := otel.UseTracer(ctx, "db.ScanRowsTo",
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	var cols []string
	var err error

	cols, err = ra.columns()
	if err != nil {
		err := fmt.Errorf("scanRowsTo: failed to get columns: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	var out []T

	defer func() { _ = ra.close() }()

	// prepare scan destinations
	vals, ptrs := makeScanPtrs(len(cols))

	// reflect type information for T
	tType := reflect.TypeOf((*T)(nil)).Elem()
	isPtr := false
	if tType.Kind() == reflect.Pointer {
		isPtr = true
		tType = tType.Elem()
	}
	if tType.Kind() != reflect.Struct {
		err := fmt.Errorf("scanRowsTo: T must be a struct or pointer to struct")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	fieldMap := globalFieldMapCache.get(tType)

	// pre-compute lowercase column names to avoid repeated string operations
	lowerCols := make([]string, len(cols))
	for i, col := range cols {
		lowerCols[i] = strings.ToLower(col)
	}

	for ra.next() {
		if err := ra.scan(ptrs...); err != nil {
			err := fmt.Errorf("scanRowsTo: scan failed: %w", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
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
			colKey := lowerCols[i]
			if path, ok := fieldMap[colKey]; ok {
				f := itemVal.FieldByIndex(path)
				if !f.CanSet() {
					continue
				}
				err := setFieldFromValue(f, raw)
				if err != nil {
					err := fmt.Errorf("scanRowsTo: failed to set field %s: %w", col, err)
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return nil, err
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
		err := fmt.Errorf("scanRowsTo: rows iteration failed: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "scanRowsTo completed successfully")
	return out, nil
}

// ScanAll scans every row from a RowsProvider into a typed slice.
//
// It is the preferred public helper for typed reads. It uses db/json tags and
// field-name fallback through the same mapper as ScanRowsTo.
func ScanAll[T any](ctx context.Context, rows RowsProvider) ([]T, error) {
	result, err := ScanRowsTo[T](ctx, rows)
	if err != nil {
		return nil, fmt.Errorf("ScanAll: %w", err)
	}
	return result, nil
}

// ScanOne scans exactly one row from a RowsProvider.
//
// It returns an error when the result set is empty or contains more than one row.
func ScanOne[T any](ctx context.Context, rows RowsProvider) (T, error) {
	var zero T
	result, err := ScanRowsTo[T](ctx, rows)
	if err != nil {
		return zero, fmt.Errorf("ScanOne: %w", err)
	}
	if len(result) == 0 {
		return zero, fmt.Errorf("ScanOne: no rows")
	}
	if len(result) > 1 {
		return zero, fmt.Errorf("ScanOne: expected one row, got %d", len(result))
	}
	return result[0], nil
}
