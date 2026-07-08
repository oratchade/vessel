//go:build test

package v1_test

import (
	"context"
	"fmt"
	"testing"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/db/v1/dberror"
)

// TestClassifyError_TypedSentinels ensures classification prefers the typed
// dberror sentinels (attached by the drivers' error mappers) over message
// substring matching.
func TestClassifyError_TypedSentinels(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  string
		wantLevel db.LogLevel
	}{
		{
			name:      "duplicate key sentinel",
			err:       fmt.Errorf("insert: %w", dberror.ErrDuplicateKey),
			wantType:  db.ErrorTypeDuplicateKey,
			wantLevel: db.LogLevelWarn,
		},
		{
			name:      "foreign key sentinel",
			err:       fmt.Errorf("insert: %w", dberror.ErrForeignKeyViolation),
			wantType:  db.ErrorTypeForeignKeyViolation,
			wantLevel: db.LogLevelWarn,
		},
		{
			name:      "syntax sentinel without message needle",
			err:       fmt.Errorf("exec: %w", dberror.ErrSyntaxError),
			wantType:  db.ErrorTypeSyntax,
			wantLevel: db.LogLevelWarn,
		},
		{
			name:      "timeout sentinel",
			err:       fmt.Errorf("query: %w", dberror.ErrQueryTimeout),
			wantType:  db.ErrorTypeTimeout,
			wantLevel: db.LogLevelError,
		},
		{
			name:      "connection sentinel",
			err:       fmt.Errorf("ping: %w", dberror.ErrConnectionFailed),
			wantType:  db.ErrorTypeConnection,
			wantLevel: db.LogLevelError,
		},
		{
			name:      "constraint sentinel",
			err:       fmt.Errorf("insert: %w", dberror.ErrConstraintViolation),
			wantType:  db.ErrorTypeConstraintViolation,
			wantLevel: db.LogLevelWarn,
		},
		{
			name:      "context canceled typed",
			err:       fmt.Errorf("query: %w", context.Canceled),
			wantType:  db.ErrorTypeContextCanceled,
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

// TestClassifyError_NarrowNeedles ensures overly generic substrings no longer
// cause misclassification, while real driver messages still classify.
func TestClassifyError_NarrowNeedles(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			// "near" alone must not classify as syntax.
			name:     "near in unrelated message is not syntax",
			err:      fmt.Errorf("reading near the buffer boundary failed"),
			wantType: db.ErrorTypeUnknown,
		},
		{
			// "invalid" alone must not classify as validation.
			name:     "invalid in unrelated message is not validation",
			err:      fmt.Errorf("invalid memory address or nil pointer dereference"),
			wantType: db.ErrorTypeUnknown,
		},
		{
			// MySQL 1064 message does not contain "syntax error" verbatim.
			name:     "mysql sql syntax message classifies as syntax",
			err:      fmt.Errorf("Error 1064: You have an error in your SQL syntax; check the manual"),
			wantType: db.ErrorTypeSyntax,
		},
		{
			name:     "sqlite near message still classifies as syntax",
			err:      fmt.Errorf(`near "FROM": syntax error`),
			wantType: db.ErrorTypeSyntax,
		},
		{
			name:     "validation failed message classifies as validation",
			err:      fmt.Errorf("validation failed for column age"),
			wantType: db.ErrorTypeValidationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, _ := db.ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("got type %q, want %q", gotType, tt.wantType)
			}
		})
	}
}
