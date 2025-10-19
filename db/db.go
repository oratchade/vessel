package db

import (
	"context"
	"database/sql"
	"fmt"

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

// DBActions defines the core, context-aware data access operations that any
// database connection or transaction must provide. It is a stable,
// implementation-agnostic contract used by higher-level code (builders,
// services) to perform SQL work without depending on driver details.
//
// Responsibilities and expectations:
//
//   - Context propagation: every method accepts context.Context so callers can
//     control timeouts and cancellations.
//   - Parameter ordering: implementations must preserve placeholder ↔ arg
//     ordering. Convention: condition args first, then option args.
//   - Identifier handling: callers should provide identifiers; implementations
//     must quote/validate identifiers using the dialect helpers to avoid injection.
//   - Mutation results: mutation methods return *ExecResult (LastInsertID, RowsAffected)
//     so callers can inspect execution metadata consistently across drivers.
//   - Error handling: return wrapped errors (fmt.Errorf("%w")) and provide sentinel
//     errors (e.g. ErrNotFound) when useful for caller-side handling.
//   - Concurrency: implementations must be safe for concurrent use (connection
//     pooling handled internally).
//
// Transactions:
//   - Tx implementations embed DBActions and add Commit/Rollback lifecycle methods.
//   - WithTransaction helpers should Begin, call the provided function, Commit on
//     success and Rollback on error or panic.
//
// Testing:
//   - Keep DBActions small and mockable; add integration tests that verify SQL+args
//     ordering, option rendering per
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
	//   ExecResult: Result of the insert operation.
	//   error: Error if the insert fails.
	Insert(ctx context.Context, table string, data map[string]any, opts *options.QueryOptions) (*ExecResult, error)

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
	//   opts: Optional query parameters (limit, offset, etc.).
	//
	// Returns:
	//   *sql.Rows: Result rows from the query.
	//   error: Error if the query fails.
	// Query(ctx context.Context, query string, opts *options.QueryOptions, args ...any) (*sql.Rows, error)

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
	// QueryRow(ctx context.Context, query string, opts *options.QueryOptions, args ...any) *sql.Row

	// Exec executes a raw SQL statement (insert, update, delete, etc.), with optional query options.
	//
	// Parameters:
	//   ctx: Context for cancellation and deadlines.
	//   query: Raw SQL statement.
	//   args: Arguments for parameterized statement.
	//   opts: Optional query parameters.
	//
	// Returns:
	//   ExecResult: Result of the execution.
	//   error: Error if the execution fails.
	Exec(ctx context.Context, query string, opts *options.QueryOptions, args ...any) (*ExecResult, error)
}

// DB represents a database connection interface.
// Each method is documented with its purpose, parameters, and expected return values.
// Now supports SQL joins via the joins parameter and accepts options.QueryOptions for extensibility.
type DB interface {
	DBActions

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

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(fields ...any) Logger
}

func scanRows(rows *sql.Rows, cols []string) ([]map[string]any, error) {
	results := make([]map[string]any, 0)
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = v
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return results, nil
}
