// Package dberror provides sentinel error types and error mapping utilities for database operations.
//
// Sentinel Errors:
// - ErrNotFound: Query returned no rows
// - ErrDuplicateKey: Unique or primary key constraint violated
// - ErrForeignKeyViolation: Foreign key constraint violated
// - ErrConnectionFailed: Database connection failed or unreachable
// - ErrConstraintViolation: General constraint violations
// - ErrSyntaxError: SQL query has invalid syntax
// - ErrQueryTimeout: Query execution exceeded timeout threshold
//
// Error Detection:
// The package provides database-specific mappers (MySQL, PostgreSQL, SQLite, MSSQL)
// that detect and map database errors to sentinel errors. Use GetMapper(dialect)
// to get the appropriate mapper for your database, then call mapper.MapError(err)
// to convert database-specific errors to sentinel errors.
//
// Example:
//
//	if resp.Error == nil {
//	    // Success
//	} else if errors.Is(resp.Error, dberror.ErrDuplicateKey) {
//	    // Handle duplicate key
//	} else if errors.Is(resp.Error, dberror.ErrQueryTimeout) {
//	    // Handle timeout
//	} else if errors.Is(resp.Error, dberror.ErrSyntaxError) {
//	    // Handle syntax error
//	}
package dberror

import (
	"errors"
	"fmt"
	"strings"

	mssql "github.com/denisenkom/go-mssqldb"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	sqlite "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

const (
	// Database prefix identifiers for error messages
	MySQLPrefix    = "[mysql]"
	PostgresPrefix = "[postgres]"
	SQLitePrefix   = "[sqlite]"
	MSSQLPrefix    = "[mssql]"
)

var (
	// ErrNotFound is returned when a query returns no rows.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateKey is returned when a unique constraint or primary key constraint is violated.
	ErrDuplicateKey = errors.New("duplicate key violation")

	// ErrForeignKeyViolation is returned when a foreign key constraint is violated.
	ErrForeignKeyViolation = errors.New("foreign key constraint violation")

	// ErrConnectionFailed is returned when the database connection fails.
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrConstraintViolation is returned for general constraint violations.
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrSyntaxError is returned when a SQL query has invalid syntax.
	// This error indicates the SQL statement is malformed and cannot be executed.
	// It is detected across all database types: MySQL (error 1064), PostgreSQL (SQLSTATE 42601),
	// SQLite, and MSSQL ("syntax error" or "incorrect syntax").
	ErrSyntaxError = errors.New("SQL syntax error")

	// ErrQueryTimeout is returned when query execution exceeds the timeout threshold.
	// This typically indicates the query took too long to complete, often due to
	// heavy queries, slow connections, or insufficient resources.
	// It is detected across all database types via timeout/deadline signals.
	ErrQueryTimeout = errors.New("query timeout")
)

// ErrorMapper is the interface for mapping database-specific errors to sentinel errors.
type ErrorMapper interface {
	MapError(err error) error
}

// wrapError wraps a sentinel error with database prefix and original error.
// This provides clear context about which database reported the error.
func wrapError(prefix string, sentinel, original error) error {
	return fmt.Errorf("%s %w: %w", prefix, sentinel, original)
}

// MySQLErrorMapper maps MySQL error codes to sentinel errors.
type MySQLErrorMapper struct{}

// MapError maps MySQL error codes to sentinel errors with [mysql] prefix.
// Typed detection via *mysql.MySQLError.Number takes precedence over string matching.
func (m MySQLErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return wrapError(MySQLPrefix, ErrDuplicateKey, err)
		case 1452:
			return wrapError(MySQLPrefix, ErrForeignKeyViolation, err)
		case 1064:
			return wrapError(MySQLPrefix, ErrSyntaxError, err)
		}
	}

	return mysqlMapErrorOnString(err)
}

func mysqlMapErrorOnString(err error) error {
	errMsg := err.Error()
	if checkMySQLDuplicateKey(errMsg) {
		return wrapError(MySQLPrefix, ErrDuplicateKey, err)
	}

	if checkMySQLForeignKey(errMsg) {
		return wrapError(MySQLPrefix, ErrForeignKeyViolation, err)
	}

	if checkMySQLConnection(errMsg) {
		return wrapError(MySQLPrefix, ErrConnectionFailed, err)
	}

	if checkMySQLSyntaxError(errMsg) {
		return wrapError(MySQLPrefix, ErrSyntaxError, err)
	}

	if checkMySQLTimeout(errMsg) {
		return wrapError(MySQLPrefix, ErrQueryTimeout, err)
	}

	return err
}

// PostgresErrorMapper maps PostgreSQL SQLSTATE codes to sentinel errors.
type PostgresErrorMapper struct{}

// MapError maps PostgreSQL error codes to sentinel errors with [postgres] prefix.
// Typed detection via *pgconn.PgError.Code (SQLSTATE) takes precedence over string matching.
func (m PostgresErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return wrapError(PostgresPrefix, ErrDuplicateKey, err)
		case "23503":
			return wrapError(PostgresPrefix, ErrForeignKeyViolation, err)
		case "42601":
			return wrapError(PostgresPrefix, ErrSyntaxError, err)
		case "08006", "08001", "08004", "08000":
			return wrapError(PostgresPrefix, ErrConnectionFailed, err)
		case "57014":
			return wrapError(PostgresPrefix, ErrQueryTimeout, err)
		}
	}

	return pgMapErrorOnString(err)
}

func pgMapErrorOnString(err error) error {
	errMsg := err.Error()

	if checkPostgresDuplicateKey(errMsg) {
		return wrapError(PostgresPrefix, ErrDuplicateKey, err)
	}

	if checkPostgresForeignKey(errMsg) {
		return wrapError(PostgresPrefix, ErrForeignKeyViolation, err)
	}

	if checkPostgresConnection(errMsg) {
		return wrapError(PostgresPrefix, ErrConnectionFailed, err)
	}

	if checkPostgresSyntaxError(errMsg) {
		return wrapError(PostgresPrefix, ErrSyntaxError, err)
	}

	if checkPostgresTimeout(errMsg) {
		return wrapError(PostgresPrefix, ErrQueryTimeout, err)
	}

	return err
}

// SQLiteErrorMapper maps SQLite error messages to sentinel errors.
type SQLiteErrorMapper struct{}

// MapError maps SQLite error codes to sentinel errors with [sqlite] prefix.
// Typed detection via modernc.org/sqlite.Error takes precedence over string matching.
func (m SQLiteErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqlitelib.SQLITE_CONSTRAINT_UNIQUE,
			sqlitelib.SQLITE_CONSTRAINT_PRIMARYKEY,
			sqlitelib.SQLITE_CONSTRAINT_ROWID:
			return wrapError(SQLitePrefix, ErrDuplicateKey, err)
		case sqlitelib.SQLITE_CONSTRAINT_FOREIGNKEY:
			return wrapError(SQLitePrefix, ErrForeignKeyViolation, err)
		case sqlitelib.SQLITE_CANTOPEN,
			sqlitelib.SQLITE_CANTOPEN_CONVPATH,
			sqlitelib.SQLITE_CANTOPEN_DIRTYWAL,
			sqlitelib.SQLITE_CANTOPEN_FULLPATH,
			sqlitelib.SQLITE_CANTOPEN_ISDIR,
			sqlitelib.SQLITE_CANTOPEN_NOTEMPDIR,
			sqlitelib.SQLITE_CANTOPEN_SYMLINK:
			return wrapError(SQLitePrefix, ErrConnectionFailed, err)
		case sqlitelib.SQLITE_ERROR:
			return wrapError(SQLitePrefix, ErrSyntaxError, err)
		case sqlitelib.SQLITE_BUSY, sqlitelib.SQLITE_LOCKED:
			return wrapError(SQLitePrefix, ErrQueryTimeout, err)
		}
	}

	return sqliteMapErrorOnString(err)
}

func sqliteMapErrorOnString(err error) error {
	errMsg := err.Error()

	// UNIQUE constraint failed
	if strings.Contains(errMsg, "UNIQUE constraint failed") {
		return wrapError(SQLitePrefix, ErrDuplicateKey, err)
	}

	// FOREIGN KEY constraint failed
	if strings.Contains(errMsg, "FOREIGN KEY constraint failed") || strings.Contains(errMsg, "foreign key constraint") {
		return wrapError(SQLitePrefix, ErrForeignKeyViolation, err)
	}

	// Connection errors
	if strings.Contains(errMsg, "unable to open database") || strings.Contains(errMsg, "cannot open") {
		return wrapError(SQLitePrefix, ErrConnectionFailed, err)
	}

	// Syntax errors
	if strings.Contains(errMsg, "syntax error") {
		return wrapError(SQLitePrefix, ErrSyntaxError, err)
	}

	// Query timeout (context-level, not a SQLite driver error)
	if strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "query timeout") {
		return wrapError(SQLitePrefix, ErrQueryTimeout, err)
	}

	return err
}

// checkMySQLDuplicateKey checks if error is a duplicate key violation.
func checkMySQLDuplicateKey(errMsg string) bool {
	return strings.Contains(errMsg, "1062") || strings.Contains(errMsg, "Duplicate entry")
}

// checkMySQLForeignKey checks if error is a foreign key constraint violation.
func checkMySQLForeignKey(errMsg string) bool {
	return strings.Contains(errMsg, "1452") || strings.Contains(errMsg, "foreign key constraint fails")
}

// checkMySQLConnection checks if error is a connection error.
func checkMySQLConnection(errMsg string) bool {
	return strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host")
}

// checkMySQLSyntaxError checks if error is a syntax error.
func checkMySQLSyntaxError(errMsg string) bool {
	return strings.Contains(errMsg, "syntax error") || strings.Contains(errMsg, "1064")
}

// checkMySQLTimeout checks if error is a query timeout.
func checkMySQLTimeout(errMsg string) bool {
	return strings.Contains(errMsg, "deadline exceeded") ||
		strings.Contains(errMsg, "query timeout") ||
		strings.Contains(errMsg, "max statement time")
}

// checkPostgresDuplicateKey checks if error is a duplicate key violation.
func checkPostgresDuplicateKey(errMsg string) bool {
	return strings.Contains(errMsg, "23505") || strings.Contains(errMsg, "duplicate key value")
}

// checkPostgresForeignKey checks if error is a foreign key constraint violation.
func checkPostgresForeignKey(errMsg string) bool {
	return strings.Contains(errMsg, "23503") || strings.Contains(errMsg, "foreign key constraint")
}

// checkPostgresConnection checks if error is a connection error.
func checkPostgresConnection(errMsg string) bool {
	return strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "connect: connection refused")
}

// checkPostgresSyntaxError checks if error is a syntax error.
func checkPostgresSyntaxError(errMsg string) bool {
	return strings.Contains(errMsg, "42601") || strings.Contains(errMsg, "syntax error")
}

// checkPostgresTimeout checks if error is a query timeout.
func checkPostgresTimeout(errMsg string) bool {
	return strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "query timeout")
}

// MSSQLErrorMapper maps SQL Server error codes to sentinel errors.
type MSSQLErrorMapper struct{}

// MapError maps MSSQL error codes to sentinel errors with [mssql] prefix.
// Typed detection via mssql.Error.Number takes precedence over string matching.
func (m MSSQLErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	var mssqlErr mssql.Error
	if errors.As(err, &mssqlErr) {
		switch mssqlErr.Number {
		case 2627, 2601:
			return wrapError(MSSQLPrefix, ErrDuplicateKey, err)
		case 547:
			return wrapError(MSSQLPrefix, ErrForeignKeyViolation, err)
		case 102, 156:
			return wrapError(MSSQLPrefix, ErrSyntaxError, err)
		case -2:
			return wrapError(MSSQLPrefix, ErrQueryTimeout, err)
		}
	}

	return mssqlMapErrorOnString(err)
}

func mssqlMapErrorOnString(err error) error {
	errMsg := err.Error()

	if checkMSSQLDuplicateKey(errMsg) {
		return wrapError(MSSQLPrefix, ErrDuplicateKey, err)
	}

	if checkMSSQLForeignKey(errMsg) {
		return wrapError(MSSQLPrefix, ErrForeignKeyViolation, err)
	}

	if checkMSSQLConnection(errMsg) {
		return wrapError(MSSQLPrefix, ErrConnectionFailed, err)
	}

	if checkMSSQLSyntaxError(errMsg) {
		return wrapError(MSSQLPrefix, ErrSyntaxError, err)
	}

	if checkMSSQLTimeout(errMsg) {
		return wrapError(MSSQLPrefix, ErrQueryTimeout, err)
	}

	return err
}

// checkMSSQLDuplicateKey checks if error is a duplicate key violation.
func checkMSSQLDuplicateKey(errMsg string) bool {
	return strings.Contains(errMsg, "2601") || strings.Contains(errMsg, "2627") ||
		strings.Contains(errMsg, "duplicate key") || strings.Contains(errMsg, "PRIMARY KEY constraint")
}

// checkMSSQLForeignKey checks if error is a foreign key constraint violation.
func checkMSSQLForeignKey(errMsg string) bool {
	return strings.Contains(errMsg, "547") || strings.Contains(errMsg, "FOREIGN KEY constraint")
}

// checkMSSQLConnection checks if error is a connection error.
func checkMSSQLConnection(errMsg string) bool {
	return strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "cannot open database")
}

// checkMSSQLSyntaxError checks if error is a syntax error.
func checkMSSQLSyntaxError(errMsg string) bool {
	lowerMsg := strings.ToLower(errMsg)
	return strings.Contains(lowerMsg, "syntax error") || strings.Contains(lowerMsg, "incorrect syntax")
}

// checkMSSQLTimeout checks if error is a query timeout.
func checkMSSQLTimeout(errMsg string) bool {
	lowerMsg := strings.ToLower(errMsg)
	return strings.Contains(lowerMsg, "timeout") || strings.Contains(lowerMsg, "deadline exceeded") ||
		strings.Contains(lowerMsg, "query timeout") || strings.Contains(errMsg, "-2")
}

// GetMapper returns the appropriate error mapper for the given dialect name.
// The dialect should be one of the driver constants from pkg/query/definition.
// Supports primary names and documented aliases:
// - postgres/postgresql, sqlite, sqlserver/mssql.
func GetMapper(dialect string) ErrorMapper {
	switch strings.ToLower(dialect) {
	case "postgres", "postgresql":
		return PostgresErrorMapper{}
	case "sqlite":
		return SQLiteErrorMapper{}
	case "sqlserver", "mssql":
		return MSSQLErrorMapper{}
	case "mysql":
		return MySQLErrorMapper{}
	default:
		return MySQLErrorMapper{}
	}
}

/*
ERROR MAPPING REFERENCE

This package maps database-specific errors to sentinel error types across all supported
database backends. Below is the error mapping strategy for each backend:

SYNTAX ERROR (ErrSyntaxError)
- MySQL: Error code 1064 or message contains "syntax error"
- PostgreSQL: SQLSTATE 42601 or message contains "syntax error"
- SQLite: Message contains "syntax error"
- MSSQL: Message contains "syntax error" or "incorrect syntax"
Usage:
  if errors.Is(err, ErrSyntaxError) {
      // SQL statement is malformed - check query syntax
  }

QUERY TIMEOUT (ErrQueryTimeout)
- MySQL: Message contains "deadline exceeded", "query timeout", or "max statement time"
- PostgreSQL: Message contains "deadline exceeded" or "query timeout"
- SQLite: Message contains "deadline exceeded" or "query timeout"
- MSSQL: Message contains "timeout", "deadline exceeded", "query timeout", or error code -2
Usage:
  if errors.Is(err, ErrQueryTimeout) {
      // Query took too long - increase timeout or optimize query
  }

DUPLICATE KEY (ErrDuplicateKey)
- MySQL: Error code 1062 or message contains "Duplicate entry"
- PostgreSQL: SQLSTATE 23505 or message contains "duplicate key value"
- SQLite: Message contains "UNIQUE constraint failed"
- MSSQL: Error code 2601/2627 or contains "duplicate key" or "PRIMARY KEY constraint"
Usage:
  if errors.Is(err, ErrDuplicateKey) {
      // Data already exists - use INSERT OR IGNORE, UPDATE, or different data
  }

FOREIGN KEY VIOLATION (ErrForeignKeyViolation)
- MySQL: Error code 1452 or message contains "foreign key constraint fails"
- PostgreSQL: SQLSTATE 23503 or message contains "foreign key constraint"
- SQLite: Message contains "FOREIGN KEY constraint failed" or "foreign key constraint"
- MSSQL: Error code 547 or contains "FOREIGN KEY constraint"
Usage:
  if errors.Is(err, ErrForeignKeyViolation) {
      // Related record doesn't exist - ensure foreign key data exists first
  }

CONNECTION FAILED (ErrConnectionFailed)
- MySQL: Message contains "connection refused" or "no such host"
- PostgreSQL: Message contains "connection refused" or "connect: connection refused"
- SQLite: Message contains "unable to open database" or "cannot open"
- MSSQL: Message contains "connection refused" or "cannot open database"
Usage:
  if errors.Is(err, ErrConnectionFailed) {
      // Database is unreachable - check connectivity, retry with backoff
  }

NOT FOUND (ErrNotFound)
- All databases: Returned when query returns zero rows
Usage:
  if errors.Is(err, ErrNotFound) {
      // Query matched no records - verify search criteria
  }

BEST PRACTICES:
1. Always check resp.Error before accessing resp.Data
2. Use errors.Is(err, ErrType) for error type checking
3. Implement exponential backoff retry for transient errors (connection, timeout)
4. Log errors with context (database, query, parameters)
5. Monitor error rates to detect systemic issues
6. Non-retryable errors (duplicate key, syntax, foreign key) should be handled differently
*/
