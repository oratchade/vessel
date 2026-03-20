// Package db provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

// contextKeyCorrelationID is the key for storing correlation IDs in context.
const contextKeyCorrelationID contextKey = "correlation-id"

// Error type constants define categories for error classification and logging.
const (
	// WARN level errors - user/application errors
	ErrorTypeSyntax              = "syntax_error"
	ErrorTypeConstraintViolation = "constraint_violation"
	ErrorTypeDuplicateKey        = "duplicate_key"
	ErrorTypeForeignKeyViolation = "foreign_key_violation"
	ErrorTypeValidationError     = "validation_error"
	ErrorTypeDataTypeError       = "data_type_error"

	// ERROR level errors - system errors
	ErrorTypeConnection  = "connection_error"
	ErrorTypeTimeout     = "timeout_error"
	ErrorTypeDeadlock    = "deadlock_error"
	ErrorTypePoolExhaust = "pool_exhausted"
	ErrorTypeIOError     = "io_error"
	ErrorTypePermission  = "permission_denied"
	ErrorTypeNotImpl     = "not_implemented"

	// INFO level errors - expected/non-errors
	ErrorTypeContextCanceled = "context_canceled"

	// Unknown error
	ErrorTypeUnknown = "unknown_error"
)

// LogLevel represents the logging level for an operation.
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// SafeLogger is a nil-safe wrapper around Logger that automatically extracts logging context
// (correlation ID, sanitized table names, formatted durations, etc.) and only logs if the
// underlying logger is not nil. This eliminates the need for repetitive nil-checks throughout
// the codebase. Created once per driver instance and reused for all operations.
type SafeLogger struct {
	logger Logger
}

// NewSafeLogger creates a new SafeLogger that wraps the given logger.
// If logger is nil, logging operations become no-ops (zero overhead).
// SafeLogger should be created once per driver instance and reused.
func NewSafeLogger(logger Logger) *SafeLogger {
	return &SafeLogger{
		logger: logger,
	}
}

// QueryError logs a query error with automatic context extraction from ctx.
// Handles error classification and selects appropriate log level (Error or Warn).
// Called when a database query fails.
func (sl *SafeLogger) QueryError(
	ctx context.Context,
	dbDriver, operation, table string,
	duration time.Duration,
	err error,
) {
	if sl.logger == nil {
		return
	}

	sanitizedTable := SanitizeTableName(table)
	correlationID := ExtractCorrelationID(ctx)
	durationMS := FormatDuration(duration)
	errorType, level := ClassifyError(err)

	fields := []interface{}{
		"db_driver", dbDriver,
		"operation", operation,
		"table", sanitizedTable,
		"duration_ms", durationMS,
		"error_type", errorType,
		"correlation_id", correlationID,
		"error", err,
	}

	if level == LogLevelError {
		sl.logger.Error(fmt.Sprintf("%s.%s: query failed", dbDriver, strings.ToUpper(operation)), fields...)
	} else {
		sl.logger.Warn(fmt.Sprintf("%s.%s: query failed", dbDriver, strings.ToUpper(operation)), fields...)
	}
}

func (sl *SafeLogger) Error(err error) {
	if sl.logger == nil {
		return
	}

	sl.logger.Error("database error", "error", err)
}

func (sl *SafeLogger) Debug(msg ...string) {
	if sl.logger == nil {
		return
	}

	sl.logger.Debug("database debug", "message", strings.Join(msg, " "))
}

// QuerySuccess logs a successful query with automatic context extraction from ctx.
// Detects slow queries and logs them at WARN level; normal queries at DEBUG level.
// Called when a database query completes successfully.
func (sl *SafeLogger) QuerySuccess(
	ctx context.Context,
	dbDriver, operation, table string,
	duration time.Duration,
	rowsReturned int,
) {
	if sl.logger == nil {
		return
	}

	sanitizedTable := SanitizeTableName(table)
	correlationID := ExtractCorrelationID(ctx)
	durationMS := FormatDuration(duration)

	fields := []interface{}{
		"db_driver", dbDriver,
		"operation", operation,
		"table", sanitizedTable,
		"rows_returned", rowsReturned,
		"duration_ms", durationMS,
		"correlation_id", correlationID,
	}

	isSlowQuery := IsSlowQuery(duration, 500)
	if isSlowQuery {
		fields = append(fields, "is_slow", true)
		sl.logger.Warn(
			fmt.Sprintf("%s.%s: slow query detected", dbDriver, strings.ToUpper(operation)),
			fields...,
		)
	} else {
		sl.logger.Debug(
			fmt.Sprintf("%s.%s: query executed successfully", dbDriver, strings.ToUpper(operation)),
			fields...,
		)
	}
}

// TransactionSuccess logs a successful transaction operation (begin, commit, rollback)
// with automatic context extraction from ctx. Called when transactions complete without error.
func (sl *SafeLogger) TransactionSuccess(ctx context.Context, dbDriver, operation string) {
	if sl.logger == nil {
		return
	}

	correlationID := ExtractCorrelationID(ctx)
	fields := []interface{}{
		"db_driver", dbDriver,
		"operation", operation,
		"correlation_id", correlationID,
	}

	sl.logger.Debug(fmt.Sprintf("%s.%s: transaction started", dbDriver, strings.ToUpper(operation)), fields...)
}

// ClassifyError analyzes an error and returns its classification type and log level.
// This helps determine how to log different types of database errors appropriately.
//
//nolint:cyclop
func ClassifyError(err error) (errorType string, level LogLevel) {
	if err == nil {
		return ErrorTypeUnknown, LogLevelInfo
	}

	errMsg := err.Error()
	errMsg = strings.ToLower(errMsg)

	// Check for foreign key violations (WARN level) - must be before generic constraint check
	if strings.Contains(errMsg, "foreign key") {
		return ErrorTypeForeignKeyViolation, LogLevelWarn
	}

	// Check for duplicate key (WARN level) - must be before generic constraint check
	if strings.Contains(errMsg, "duplicate") {
		return ErrorTypeDuplicateKey, LogLevelWarn
	}

	// Check for context errors first (should be INFO level)
	if strings.Contains(errMsg, "context canceled") {
		return ErrorTypeContextCanceled, LogLevelInfo
	}

	if errType, errLevel := getConnectionErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getTimeoutErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getPoolErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getDeadlockErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getIOErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getPermissionErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getSyntaxErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getConstraintErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getDataTypeErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	if errType, errLevel := getValidationErrors(errMsg); errType != "" {
		return errType, errLevel
	}

	// Default to unknown error (ERROR level for safety)
	return ErrorTypeUnknown, LogLevelError
}

func getConstraintErrors(errMsg string) (errorType string, level LogLevel) {
	if strings.Contains(errMsg, "unique constraint") ||
		strings.Contains(errMsg, "constraint failed") ||
		strings.Contains(errMsg, "check constraint") {
		return ErrorTypeConstraintViolation, LogLevelWarn
	}

	return "", LogLevelInfo
}

func getDataTypeErrors(errMsg string) (errorType string, level LogLevel) {
	if strings.Contains(errMsg, "cannot convert") ||
		strings.Contains(errMsg, "type mismatch") ||
		strings.Contains(errMsg, "data type") ||
		strings.Contains(errMsg, "invalid value") {
		return ErrorTypeDataTypeError, LogLevelWarn
	}

	return "", LogLevelInfo
}

func getValidationErrors(errMsg string) (errorType string, level LogLevel) {
	if strings.Contains(errMsg, "validation") ||
		strings.Contains(errMsg, "invalid") {
		return ErrorTypeValidationError, LogLevelWarn
	}

	return "", LogLevelInfo
}

func getConnectionErrors(errMsg string) (errorType string, level LogLevel) {
	// Check for connection errors (ERROR level)
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "nodename nor servname provided") {
		return ErrorTypeConnection, LogLevelError
	}

	return "", LogLevelInfo
}

func getTimeoutErrors(errMsg string) (errorType string, level LogLevel) {
	// Check for timeout errors (ERROR level)
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "deadline exceeded") {
		return ErrorTypeTimeout, LogLevelError
	}

	return "", LogLevelInfo
}

func getPoolErrors(errMsg string) (errorType string, level LogLevel) {
	// Check for pool exhaustion (ERROR level)
	if strings.Contains(errMsg, "connection pool") ||
		strings.Contains(errMsg, "pool exhausted") ||
		strings.Contains(errMsg, "max connections") {
		return ErrorTypePoolExhaust, LogLevelError
	}

	return "", LogLevelInfo
}

func getDeadlockErrors(errMsg string) (errorType string, level LogLevel) {
	// Check for deadlock errors (ERROR level)
	if strings.Contains(errMsg, "deadlock") ||
		strings.Contains(errMsg, "deadlock detected") {
		return ErrorTypeDeadlock, LogLevelError
	}

	return "", LogLevelInfo
}

func getIOErrors(errMsg string) (errorType string, level LogLevel) {
	if strings.Contains(errMsg, "i/o error") ||
		strings.Contains(errMsg, "eof") ||
		strings.Contains(errMsg, "io eof") {
		return ErrorTypeIOError, LogLevelError
	}

	return "", LogLevelInfo
}

func getPermissionErrors(errMsg string) (errorType string, level LogLevel) {
	if strings.Contains(errMsg, "permission denied") ||
		strings.Contains(errMsg, "access denied") ||
		strings.Contains(errMsg, "unauthorized") {
		return ErrorTypePermission, LogLevelError
	}

	return "", LogLevelInfo
}

func getSyntaxErrors(errMsg string) (errorType string, level LogLevel) {
	if strings.Contains(errMsg, "syntax error") ||
		strings.Contains(errMsg, "parse error") ||
		strings.Contains(errMsg, "unexpected token") ||
		strings.Contains(errMsg, "near") {
		return ErrorTypeSyntax, LogLevelWarn
	}

	return "", LogLevelInfo
}

// MaskQueryParameters replaces query parameter values with a MASKED indicator
// to prevent logging sensitive data. Returns the query with parameter count.
// Example: "SELECT * FROM users WHERE id = ? AND email = ?" becomes
// "SELECT * FROM users WHERE id = ? AND email = ? [MASKED: 2 params]"
func MaskQueryParameters(sql string, paramCount int) string {
	if paramCount == 0 {
		return sql
	}

	if paramCount == 1 {
		return fmt.Sprintf("%s [MASKED: 1 param]", sql)
	}

	return fmt.Sprintf("%s [MASKED: %d params]", sql, paramCount)
}

// TruncateQueryForLogging truncates long SQL queries for readable logging.
// Limits to maxLength characters, adding "..." if truncated.
func TruncateQueryForLogging(sql string, maxLength int) string {
	if len(sql) <= maxLength {
		return sql
	}
	return sql[:maxLength] + "..."
}

// ExtractCorrelationID extracts correlation ID from context if available.
// Looks for standard context keys used in distributed tracing.
func ExtractCorrelationID(ctx context.Context) string {
	// Try standard context keys
	if correlationID, ok := ctx.Value(contextKeyCorrelationID).(string); ok && correlationID != "" {
		return correlationID
	}

	if correlationID, ok := ctx.Value("x-correlation-id").(string); ok && correlationID != "" {
		return correlationID
	}

	if correlationID, ok := ctx.Value("request-id").(string); ok && correlationID != "" {
		return correlationID
	}

	// Try OpenTelemetry trace ID if available
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}

	return ""
}

// GenerateTransactionID generates a unique transaction ID for tracking.
// This is used to correlate all operations within a transaction in logs.
func GenerateTransactionID() string {
	return fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// SanitizeTableName ensures table name is safe for logging.
// Removes any suspicious patterns that might indicate injection attempts.
func SanitizeTableName(tableName string) string {
	// Basic validation - should only contain alphanumeric, underscore, and dot
	re := regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)
	if re.MatchString(tableName) {
		return tableName
	}
	return "[INVALID_TABLE_NAME]"
}

// FormatDuration formats a duration in milliseconds for logging.
func FormatDuration(d time.Duration) string {
	ms := float64(d.Microseconds()) / 1000.0
	if ms < 1 {
		return fmt.Sprintf("%.2fms", ms)
	}
	// For values >= 1ms, format with appropriate precision
	if ms < 10 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return fmt.Sprintf("%.0fms", ms)
}

// IsSlowQuery determines if a query duration should be logged as a slow query.
// Default threshold is 500ms (configurable via parameter).
func IsSlowQuery(duration time.Duration, thresholdMS int) bool {
	if thresholdMS <= 0 {
		thresholdMS = 500 // default threshold
	}
	return duration >= time.Duration(thresholdMS)*time.Millisecond
}

// ContextWithCorrelationID adds a correlation ID to a context for distributed tracing.
// This allows all logs from a request to be correlated together.
func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, contextKeyCorrelationID, correlationID)
}
