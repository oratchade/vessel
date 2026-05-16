// Package sqldialect provides SQL dialect implementations for various database engines.
package sqldialect

// SQLiteDialect implements SQLite SQL dialect behavior.
// SQLite uses question-mark placeholders and backtick-compatible identifier quoting,
// so it reuses the MySQL implementation while keeping a distinct type for
// dialect-specific builder decisions.
type SQLiteDialect struct {
	MySQLDialect
}
