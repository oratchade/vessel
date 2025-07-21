package db

import (
	"database/sql"

	cdt "tounilab.com/db-connector/query/condition"
)

const (
	// DriverMySQL is the driver name for MySQL databases.
	DriverMySQL = "mysql"
	// DriverPostgres is the driver name for PostgreSQL databases.
	DriverPostgres = "postgres"
	// DriverSQLLite is the driver name for SQLLite databases.
	DriverSQLLite = "SQLLite3"
	// DriverMSSQL is the driver name for Microsoft SQL Server databases.
	DriverMSSQL = "sqlserver"
)

type DBConfig interface {
	// GetDriver returns the database driver name (e.g., "mysql", "postgres", "SQLLite3").
	Driver() string
	// GetDSN returns the Data Source Name (DSN) for connecting to the database.
	DSN() string
}

// DB represents a database connection interface.
type DB interface {
	// Get retrieves data from a specified table, columns, and conditions.
	//
	// Args:
	//	table (string): The name of the table to retrieve data from.
	//	columns ([]string): The list of columns to retrieve.
	//	conditions (cdt.Condition): The conditions to filter the data.
	//
	// Returns:
	//	([]map[string]any, error): A list of maps containing the retrieved data and an error if any.
	Get(table string, columns []string, conditions cdt.Condition) ([]map[string]any, error)

	// GetByID retrieves a single record from a table by its ID.
	//
	// Args:
	//	table (string): The name of the table to retrieve data from.
	//	id (any): The ID of the record to retrieve.
	//
	// Returns:
	//	(map[string]any, error): A map containing the retrieved data and an error if any.
	GetByID(table string, id any) (map[string]any, error)

	// Insert inserts new data into a table.
	//
	// Args:
	//	table (string): The name of the table to insert data into.
	//	data (map[string]any): The data to insert.
	//
	// Returns:
	//	(sql.Result, error): The result of the insertion and an error if any.
	Insert(table string, data map[string]any) (sql.Result, error)

	// Update updates existing data in a table based on specified conditions.
	//
	// Args:
	//	table (string): The name of the table to update data in.
	//	data (map[string]any): The data to update.
	//	conditions (cdt.Condition): The conditions to filter the data.
	//
	// Returns:
	//	(sql.Result, error): The result of the update and an error if any.
	Update(table string, data map[string]any, conditions cdt.Condition) (sql.Result, error)

	// Delete deletes data from a table based on specified conditions.
	//
	// Args:
	//	table (string): The name of the table to delete data from.
	//	conditions (cdt.Condition): The conditions to filter the data.
	//
	// Returns:
	//	(sql.Result, error): The result of the deletion and an error if any.
	Delete(table string, conditions cdt.Condition) (sql.Result, error)

	// Query executes a raw SQL query.
	//
	// Args:
	//	query (string): The SQL query to execute.
	//	args (...any): The arguments to pass to the query.
	//
	// Returns:
	//	(*sql.Rows, error): The result set of the query and an error if any.
	Query(query string, args ...any) (*sql.Rows, error)

	// QueryRow executes a raw SQL query that returns a single row.
	//
	// Args:
	//	query (string): The SQL query to execute.
	//	args (...any): The arguments to pass to the query.
	//
	// Returns:
	//	*sql.Row: The result row of the query.
	QueryRow(query string, args ...any) *sql.Row

	// Exec executes a raw SQL query.
	//
	// Args:
	//	query (string): The SQL query to execute.
	//	args (...any): The arguments to pass to the query.
	//
	// Returns:
	//	(sql.Result, error): The result of the execution and an error if any.
	Exec(query string, args ...any) (sql.Result, error)

	// WithTransaction executes a function within a database transaction.
	//
	// Args:
	//	fn (func(Tx) error): The function to execute within the transaction.
	//
	// Returns:
	//	error: An error if the transaction fails.
	WithTransaction(fn func(Tx) error) error

	// Ping checks the database connection.
	//
	// Returns:
	//	error: An error if the connection fails.
	Ping() error

	// Close closes the database connection.
	//
	// Returns:
	//	error: An error if the closure fails.
	Close() error
}

// Tx represents a database transaction
type Tx interface {
	// Exec executes a SQL query with the given arguments.
	//
	// Args:
	//	query: The SQL query to execute.
	//	args: The arguments to the query.
	//
	// Returns:
	//	sql.Result: The result of the execution.
	//	error: An error if the execution fails.
	Exec(query string, args ...any) (sql.Result, error)

	// Query executes a SQL query with the given arguments and returns the result rows.
	//
	// Args:
	//
	//	query: The SQL query to execute.
	//	args: The arguments to the query.
	//
	// Returns:
	//
	//	*sql.Rows: The result rows.
	//	error: An error if the execution fails.
	Query(query string, args ...any) (*sql.Rows, error)

	// Rollback rolls back the current transaction.
	//
	// Returns:
	//
	//	error: An error if the rollback fails.
	Rollback() error
}
