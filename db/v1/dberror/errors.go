// Package dberror provides sentinel error types and error mapping utilities for database operations.
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

	// ErrGrammarError is returned when there's a SQL syntax error.
	ErrGrammarError = errors.New("SQL syntax error")
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

	// MySQL error code mapping
	// 1062: ER_DUP_ENTRY - Duplicate entry
	if strings.Contains(errMsg, "1062") || strings.Contains(errMsg, "Duplicate entry") {
		return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
	}

	// 1452: ER_NO_REFERENCED_ROW - Foreign key constraint fails
	if strings.Contains(errMsg, "1452") || strings.Contains(errMsg, "foreign key constraint fails") {
		return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
	}

	// Connection errors
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	// Syntax errors
	if strings.Contains(errMsg, "syntax error") || strings.Contains(errMsg, "1064") {
		return fmt.Errorf("%w: %w", ErrGrammarError, err)
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

	// PostgreSQL SQLSTATE codes
	// 23505: unique_violation
	if strings.Contains(errMsg, "23505") || strings.Contains(errMsg, "duplicate key value") {
		return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
	}

	// 23503: foreign_key_violation
	if strings.Contains(errMsg, "23503") || strings.Contains(errMsg, "foreign key constraint") {
		return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
	}

	// Connection errors
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "connect: connection refused") {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}

	// Syntax errors (42601 = syntax_error)
	if strings.Contains(errMsg, "42601") || strings.Contains(errMsg, "syntax error") {
		return fmt.Errorf("%w: %w", ErrGrammarError, err)
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
		return fmt.Errorf("%w: %w", ErrGrammarError, err)
	}

	return err
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
		return fmt.Errorf("%w: %w", ErrGrammarError, err)
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
