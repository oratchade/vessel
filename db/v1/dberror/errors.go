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

// MySQLErrorMapper maps MySQL error codes to sentinel errors.
type MySQLErrorMapper struct{}

// MapError maps MySQL error codes to sentinel errors.
func (m MySQLErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	if checkMySQLDuplicateKey(errMsg) {
		return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
	}

	if checkMySQLForeignKey(errMsg) {
		return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
	}

	if checkMySQLConnection(errMsg) {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	if checkMySQLSyntaxError(errMsg) {
		return fmt.Errorf("%w: %w", ErrSyntaxError, err)
	}

	if checkMySQLTimeout(errMsg) {
		return fmt.Errorf("%w: %w", ErrQueryTimeout, err)
	}

	return err
}

// PostgresErrorMapper maps PostgreSQL SQLSTATE codes to sentinel errors.
type PostgresErrorMapper struct{}

// MapError maps PostgreSQL error codes to sentinel errors.
func (m PostgresErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	if checkPostgresDuplicateKey(errMsg) {
		return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
	}

	if checkPostgresForeignKey(errMsg) {
		return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
	}

	if checkPostgresConnection(errMsg) {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	if checkPostgresSyntaxError(errMsg) {
		return fmt.Errorf("%w: %w", ErrSyntaxError, err)
	}

	if checkPostgresTimeout(errMsg) {
		return fmt.Errorf("%w: %w", ErrQueryTimeout, err)
	}

	return err
}

// SQLiteErrorMapper maps SQLite error messages to sentinel errors.
type SQLiteErrorMapper struct{}

// MapError maps SQLite error messages to sentinel errors.
func (m SQLiteErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// SQLite uses string error messages
	// UNIQUE constraint failed
	if strings.Contains(errMsg, "UNIQUE constraint failed") {
		return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
	}

	// FOREIGN KEY constraint failed
	if strings.Contains(errMsg, "FOREIGN KEY constraint failed") || strings.Contains(errMsg, "foreign key constraint") {
		return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
	}

	// Connection errors
	if strings.Contains(errMsg, "unable to open database") || strings.Contains(errMsg, "cannot open") {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	// Syntax errors
	if strings.Contains(errMsg, "syntax error") {
		return fmt.Errorf("%w: %w", ErrSyntaxError, err)
	}

	// Query timeout
	if strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "query timeout") {
		return fmt.Errorf("%w: %w", ErrQueryTimeout, err)
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

// MapError maps MSSQL error codes to sentinel errors.
func (m MSSQLErrorMapper) MapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	if checkMSSQLDuplicateKey(errMsg) {
		return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
	}

	if checkMSSQLForeignKey(errMsg) {
		return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
	}

	if checkMSSQLConnection(errMsg) {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	if checkMSSQLSyntaxError(errMsg) {
		return fmt.Errorf("%w: %w", ErrSyntaxError, err)
	}

	if checkMSSQLTimeout(errMsg) {
		return fmt.Errorf("%w: %w", ErrQueryTimeout, err)
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
// Supports both primary names and aliases:
// - postgres/postgresql, sqlite3/sqlite, sqlserver/mssql.
func GetMapper(dialect string) ErrorMapper {
	switch strings.ToLower(dialect) {
	case "postgres", "postgresql":
		return PostgresErrorMapper{}
	case "sqlite3", "sqlite":
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
