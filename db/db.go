package db

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgconn"
	builder "tounilab.com/db-connector/query/builder"
	cdt "tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/options"
)

type ExecResult struct {
	LastInsertID int64
	RowsAffected int64
}

func fromSQLResult(res sql.Result) *ExecResult {
	liid, _ := res.LastInsertId() // ignore unsupported err
	ra, _ := res.RowsAffected()
	return &ExecResult{
		LastInsertID: liid,
		RowsAffected: ra,
	}
}

func fromCommandTag(tag pgconn.CommandTag) *ExecResult {
	return &ExecResult{
		LastInsertID: 0, // use RETURNING if you want this
		RowsAffected: tag.RowsAffected(),
	}
}

type DBConfig interface {
	// Driver returns the database driver name (e.g., "mysql", "postgres", "SQLLite3").
	Driver() string
	// DSN returns the Data Source Name (DSN) for connecting to the database.
	DSN() string
}

// DB represents a database connection interface.
// Each method is documented with its purpose, parameters, and expected return values.
// Now supports SQL joins via the joins parameter and accepts options.QueryOptions for extensibility.
type DB interface {
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
		joins []builder.Join,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) ([]map[string]any, error)

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
		joins []builder.Join,
		opts *options.QueryOptions,
	) ([]map[string]any, error)

	// Insert adds a new row to the specified table, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to insert into.
	//   data: Map of column names to values for the new row.
	//   opts: Optional query parameters (e.g., returning columns).
	//
	// Returns:
	//   sql.Result: Result of the insert operation.
	//   error: Error if the insert fails.
	Insert(ctx context.Context, table string, data map[string]any, opts *options.QueryOptions) (ExecResult, error)

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
	//   sql.Result: Result of the update operation.
	//   error: Error if the update fails.
	Update(
		ctx context.Context,
		table string,
		data map[string]any,
		conditions cdt.Condition,
		opts *options.QueryOptions,
	) (sql.Result, error)

	// Delete removes rows from the specified table, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   table: Name of the table to delete from.
	//   conditions: Query conditions to select rows to delete.
	//   opts: Optional query parameters (limit, order, etc.).
	//
	// Returns:
	//   sql.Result: Result of the delete operation.
	//   error: Error if the delete fails.
	Delete(ctx context.Context, table string, conditions cdt.Condition, opts *options.QueryOptions) (ExecResult, error)

	// Query executes a raw SQL query and returns multiple rows, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL query string.
	//   args: Arguments for parameterized query.
	//   opts: Optional query parameters (limit, offset, etc.).
	//
	// Returns:
	//   *sql.Rows: Result rows from the query.
	//   error: Error if the query fails.
	Query(ctx context.Context, query string, opts *options.QueryOptions, args ...any) (*sql.Rows, error)

	// QueryRow executes a raw SQL query and returns a single row, with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL query string.
	//   args: Arguments for parameterized query.
	//   opts: Optional query parameters.
	//
	// Returns:
	//   *sql.Row: Single result row from the query.
	QueryRow(ctx context.Context, query string, opts *options.QueryOptions, args ...any) *sql.Row

	// Exec executes a raw SQL statement (insert, update, delete, etc.), with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL statement.
	//   args: Arguments for parameterized statement.
	//   opts: Optional query parameters.
	//
	// Returns:
	//   sql.Result: Result of the execution.
	//   error: Error if the execution fails.
	Exec(ctx context.Context, query string, opts *options.QueryOptions, args ...any) (ExecResult, error)

	// WithTransaction executes a function within a database transaction.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   fn: Function to execute, receiving a Tx transaction object.
	//
	// Returns:
	//   error: Error if the transaction fails or is rolled back.
	WithTransaction(ctx context.Context, fn func(Tx) error) error

	// Ping checks the database connection.
	//
	// Returns:
	//   error: Error if the connection fails.
	Ping() error

	// Close closes the database connection.
	//
	// Returns:
	//   error: Error if the closure fails.
	Close() error
}

// Tx represents a database transaction
type Tx interface {
	// Exec executes a SQL query with the given arguments.
	//
	// Args:
	//   query: The SQL query to execute.
	//   args: The arguments to the query.
	//
	// Returns:
	//   sql.Result: The result of the execution.
	//   error: An error if the execution fails.
	Exec(query string, args ...any) (sql.Result, error)

	// Query executes a SQL query with the given arguments and returns the result rows.
	//
	// Args:
	//   query: The SQL query to execute.
	//   args: The arguments to the query.
	//
	// Returns:
	//   *sql.Rows: The result rows.
	//   error: An error if the execution fails.
	Query(query string, args ...any) (*sql.Rows, error)

	// Rollback rolls back the current transaction.
	//
	// Returns:
	//   error: An error if the rollback fails.
	Rollback() error
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(fields ...any) Logger
}
