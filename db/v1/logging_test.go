//go:build test

package v1_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	db "tounilab.com/vessel/db/v1"
)

// TestClassifyError_ConnectionErrors tests connection error classification.
func TestClassifyError_ConnectionErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  string
		wantLevel db.LogLevel
	}{
		{
			name:      "connection refused",
			err:       fmt.Errorf("connection refused"),
			wantType:  db.ErrorTypeConnection,
			wantLevel: db.LogLevelError,
		},
		{
			name:      "connection reset by peer",
			err:       fmt.Errorf("connection reset by peer"),
			wantType:  db.ErrorTypeConnection,
			wantLevel: db.LogLevelError,
		},
		{
			name:      "broken pipe",
			err:       fmt.Errorf("broken pipe"),
			wantType:  db.ErrorTypeConnection,
			wantLevel: db.LogLevelError,
		},
		{
			name:      "no such host",
			err:       fmt.Errorf("no such host"),
			wantType:  db.ErrorTypeConnection,
			wantLevel: db.LogLevelError,
		},
		{
			name:      "nil error",
			err:       nil,
			wantType:  db.ErrorTypeUnknown,
			wantLevel: db.LogLevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != tt.wantLevel {
				t.Errorf("got level %q, want %q", gotLevel, tt.wantLevel)
			}
		})
	}
}

// TestClassifyError_TimeoutErrors tests timeout error classification.
func TestClassifyError_TimeoutErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "timeout",
			err:      fmt.Errorf("timeout"),
			wantType: db.ErrorTypeTimeout,
		},
		{
			name:     "i/o timeout",
			err:      fmt.Errorf("i/o timeout"),
			wantType: db.ErrorTypeTimeout,
		},
		{
			name:     "deadline exceeded",
			err:      fmt.Errorf("deadline exceeded"),
			wantType: db.ErrorTypeTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelError {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelError)
			}
		})
	}
}

// TestClassifyError_DeadlockErrors tests deadlock error classification.
func TestClassifyError_DeadlockErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "deadlock",
			err:      fmt.Errorf("deadlock"),
			wantType: db.ErrorTypeDeadlock,
		},
		{
			name:     "deadlock detected",
			err:      fmt.Errorf("deadlock detected"),
			wantType: db.ErrorTypeDeadlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelError {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelError)
			}
		})
	}
}

// TestClassifyError_PoolErrors tests connection pool error classification.
func TestClassifyError_PoolErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "connection pool exhausted",
			err:      fmt.Errorf("connection pool exhausted"),
			wantType: db.ErrorTypePoolExhaust,
		},
		{
			name:     "max connections reached",
			err:      fmt.Errorf("max connections reached"),
			wantType: db.ErrorTypePoolExhaust,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelError {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelError)
			}
		})
	}
}

// TestClassifyError_SyntaxErrors tests SQL syntax error classification.
func TestClassifyError_SyntaxErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "syntax error",
			err:      fmt.Errorf("syntax error"),
			wantType: db.ErrorTypeSyntax,
		},
		{
			name:     "parse error",
			err:      fmt.Errorf("parse error"),
			wantType: db.ErrorTypeSyntax,
		},
		{
			name:     "unexpected token",
			err:      fmt.Errorf("unexpected token"),
			wantType: db.ErrorTypeSyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelWarn {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelWarn)
			}
		})
	}
}

// TestClassifyError_ConstraintErrors tests constraint violation classification.
func TestClassifyError_ConstraintErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "unique constraint violation",
			err:      fmt.Errorf("unique constraint failed"),
			wantType: db.ErrorTypeConstraintViolation,
		},
		{
			name:     "check constraint",
			err:      fmt.Errorf("check constraint failed"),
			wantType: db.ErrorTypeConstraintViolation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelWarn {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelWarn)
			}
		})
	}
}

// TestClassifyError_DuplicateKey tests duplicate key error classification.
func TestClassifyError_DuplicateKey(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "duplicate key",
			err:      fmt.Errorf("duplicate key value"),
			wantType: db.ErrorTypeDuplicateKey,
		},
		{
			name:     "duplicate entry",
			err:      fmt.Errorf("duplicate entry for unique index"),
			wantType: db.ErrorTypeDuplicateKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelWarn {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelWarn)
			}
		})
	}
}

// TestClassifyError_ForeignKeyViolation tests foreign key violation classification.
func TestClassifyError_ForeignKeyViolation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "foreign key constraint",
			err:      fmt.Errorf("foreign key constraint failed"),
			wantType: db.ErrorTypeForeignKeyViolation,
		},
		{
			name:     "foreign key",
			err:      fmt.Errorf("foreign key violation"),
			wantType: db.ErrorTypeForeignKeyViolation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelWarn {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelWarn)
			}
		})
	}
}

// TestClassifyError_IOErrors tests I/O error classification.
func TestClassifyError_IOErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "i/o error",
			err:      fmt.Errorf("i/o error"),
			wantType: db.ErrorTypeIOError,
		},
		{
			name:     "eof",
			err:      fmt.Errorf("eof"),
			wantType: db.ErrorTypeIOError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelError {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelError)
			}
		})
	}
}

// TestClassifyError_PermissionErrors tests permission error classification.
func TestClassifyError_PermissionErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "permission denied",
			err:      fmt.Errorf("permission denied"),
			wantType: db.ErrorTypePermission,
		},
		{
			name:     "access denied",
			err:      fmt.Errorf("access denied"),
			wantType: db.ErrorTypePermission,
		},
		{
			name:     "unauthorized",
			err:      fmt.Errorf("unauthorized"),
			wantType: db.ErrorTypePermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelError {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelError)
			}
		})
	}
}

// TestClassifyError_ContextCanceled tests context canceled error classification.
func TestClassifyError_ContextCanceled(t *testing.T) {
	err := fmt.Errorf("context canceled")
	gotType, gotLevel := db.ClassifyError(err)

	if gotType != db.ErrorTypeContextCanceled {
		t.Errorf("got type %q, want %q", gotType, db.ErrorTypeContextCanceled)
	}
	if gotLevel != db.LogLevelInfo {
		t.Errorf("got level %q, want %q", gotLevel, db.LogLevelInfo)
	}
}

// TestClassifyError_DataTypeErrors tests data type error classification.
func TestClassifyError_DataTypeErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "cannot convert",
			err:      fmt.Errorf("cannot convert to int"),
			wantType: db.ErrorTypeDataTypeError,
		},
		{
			name:     "type mismatch",
			err:      fmt.Errorf("type mismatch"),
			wantType: db.ErrorTypeDataTypeError,
		},
		{
			name:     "data type error",
			err:      fmt.Errorf("data type error"),
			wantType: db.ErrorTypeDataTypeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotLevel := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
			if gotLevel != db.LogLevelWarn {
				t.Errorf("got level %q, want %q", gotLevel, db.LogLevelWarn)
			}
		})
	}
}

// TestMaskQueryParameters tests parameter masking for security.
func TestMaskQueryParameters(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		paramCount int
		want       string
	}{
		{
			name:       "no parameters",
			sql:        "SELECT * FROM users",
			paramCount: 0,
			want:       "SELECT * FROM users",
		},
		{
			name:       "single parameter",
			sql:        "SELECT * FROM users WHERE id = ?",
			paramCount: 1,
			want:       "SELECT * FROM users WHERE id = ? [MASKED: 1 param]",
		},
		{
			name:       "multiple parameters",
			sql:        "SELECT * FROM users WHERE id = ? AND email = ?",
			paramCount: 2,
			want:       "SELECT * FROM users WHERE id = ? AND email = ? [MASKED: 2 params]",
		},
		{
			name:       "many parameters",
			sql:        "INSERT INTO users (name, email, password, age) VALUES (?, ?, ?, ?)",
			paramCount: 4,
			want:       "INSERT INTO users (name, email, password, age) VALUES (?, ?, ?, ?) [MASKED: 4 params]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.MaskQueryParameters(tt.sql, tt.paramCount)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTruncateQueryForLogging tests query truncation for logging.
func TestTruncateQueryForLogging(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		maxLength int
		want      string
	}{
		{
			name:      "short query not truncated",
			sql:       "SELECT * FROM users",
			maxLength: 100,
			want:      "SELECT * FROM users",
		},
		{
			name:      "long query truncated",
			sql:       "SELECT * FROM very_long_table_name WHERE id = ? AND name = ? AND email = ?",
			maxLength: 30,
			want:      "SELECT * FROM very_long_table_...",
		},
		{
			name:      "exactly at limit",
			sql:       "SELECT * FROM",
			maxLength: 13,
			want:      "SELECT * FROM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.TruncateQueryForLogging(tt.sql, tt.maxLength)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExtractCorrelationID tests correlation ID extraction from context.
func TestExtractCorrelationID(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantID  string
		isEmpty bool
	}{
		{
			name:    "no correlation id",
			ctx:     context.Background(),
			wantID:  "",
			isEmpty: true,
		},
		{
			name: "correlation-id string key (not matched by contextKey type)",
			//nolint:staticcheck
			ctx:     context.WithValue(context.Background(), "correlation-id", "test-123"),
			wantID:  "",
			isEmpty: true,
		},
		{
			name: "x-correlation-id key",
			//nolint:staticcheck
			ctx:     context.WithValue(context.Background(), "x-correlation-id", "test-456"),
			wantID:  "test-456",
			isEmpty: false,
		},
		{
			name: "request-id key",
			//nolint:staticcheck
			ctx:     context.WithValue(context.Background(), "request-id", "test-789"),
			wantID:  "test-789",
			isEmpty: false,
		},
		{
			name: "empty correlation id ignored",
			//nolint:staticcheck
			ctx:     context.WithValue(context.Background(), "correlation-id", ""),
			wantID:  "",
			isEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.ExtractCorrelationID(tt.ctx)
			if got != tt.wantID {
				t.Errorf("got %q, want %q", got, tt.wantID)
			}
		})
	}
}

// TestGenerateTransactionID tests transaction ID generation.
func TestGenerateTransactionID(t *testing.T) {
	id1 := db.GenerateTransactionID()
	id2 := db.GenerateTransactionID()

	if id1 == "" {
		t.Error("got empty transaction ID")
	}
	if id2 == "" {
		t.Error("got empty transaction ID")
	}
	if id1 == id2 {
		t.Error("transaction IDs should be unique")
	}
	if !strings.HasPrefix(id1, "tx_") {
		t.Errorf("transaction ID should start with tx_, got %q", id1)
	}
}

// TestSanitizeTableName tests table name sanitization.
func TestSanitizeTableName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		isInvalid bool
	}{
		{
			name:      "valid simple table",
			input:     "users",
			want:      "users",
			isInvalid: false,
		},
		{
			name:      "valid with underscore",
			input:     "user_profiles",
			want:      "user_profiles",
			isInvalid: false,
		},
		{
			name:      "valid with dot",
			input:     "schema.users",
			want:      "schema.users",
			isInvalid: false,
		},
		{
			name:      "valid with numbers",
			input:     "table_123",
			want:      "table_123",
			isInvalid: false,
		},
		{
			name:      "invalid with semicolon",
			input:     "users;DROP TABLE",
			want:      "[INVALID_TABLE_NAME]",
			isInvalid: true,
		},
		{
			name:      "invalid with space",
			input:     "users DROP",
			want:      "[INVALID_TABLE_NAME]",
			isInvalid: true,
		},
		{
			name:      "invalid with special chars",
			input:     "users`DROP",
			want:      "[INVALID_TABLE_NAME]",
			isInvalid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.SanitizeTableName(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatDuration tests duration formatting for logging.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "milliseconds",
			duration: 1500 * time.Microsecond,
			want:     "1.5ms",
		},
		{
			name:     "one millisecond",
			duration: 1000 * time.Microsecond,
			want:     "1.0ms",
		},
		{
			name:     "sub millisecond",
			duration: 500 * time.Microsecond,
			want:     "0.50ms",
		},
		{
			name:     "multiple milliseconds",
			duration: 250 * time.Millisecond,
			want:     "250ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestIsSlowQuery tests slow query detection.
func TestIsSlowQuery(t *testing.T) {
	tests := []struct {
		name        string
		duration    time.Duration
		thresholdMS int
		wantSlow    bool
	}{
		{
			name:        "below default threshold",
			duration:    100 * time.Millisecond,
			thresholdMS: 0, // Use default 500ms
			wantSlow:    false,
		},
		{
			name:        "above default threshold",
			duration:    600 * time.Millisecond,
			thresholdMS: 0, // Use default 500ms
			wantSlow:    true,
		},
		{
			name:        "at custom threshold",
			duration:    100 * time.Millisecond,
			thresholdMS: 100,
			wantSlow:    true,
		},
		{
			name:        "below custom threshold",
			duration:    50 * time.Millisecond,
			thresholdMS: 100,
			wantSlow:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.IsSlowQuery(tt.duration, tt.thresholdMS)
			if got != tt.wantSlow {
				t.Errorf("got %v, want %v", got, tt.wantSlow)
			}
		})
	}
}

// TestContextWithCorrelationID tests adding correlation ID to context.
func TestContextWithCorrelationID(t *testing.T) {
	correlationID := "test-correlation-123"
	ctx := db.ContextWithCorrelationID(context.Background(), correlationID)

	got := db.ExtractCorrelationID(ctx)
	if got != correlationID {
		t.Errorf("got %q, want %q", got, correlationID)
	}
}
