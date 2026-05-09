//go:build test

// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"database/sql"
	"fmt"
	"reflect"

	"tounilab.com/fabric/pkg/query/options"
)

func ExportValidateQueryOptions(opts *options.QueryOptions) error {
	return validateQueryOptions(opts)
}

// ExportFromSQLResult allows tests to call the unexported fromSQLResult function.
func ExportFromSQLResult(res sql.Result) (*ExecResult, error) {
	return fromSQLResult(res)
}

// ExportScanRows allows tests to call the unexported scanRows function.
func ExportScanRows(rows any, cols []string) ([]map[string]any, error) {
	return scanRows(rows, cols)
}

// BuildFieldMapForTest exposes the buildFieldMap function for testing.
func BuildFieldMapForTest(tType reflect.Type) map[string][]int {
	return buildFieldMap(tType)
}

// ============================================
// TEST HELPERS
// ============================================

// NewRowsAdapterForTest creates a RowsAdapter directly from sql.Rows.
// This is for testing purposes only. Production code should use NewRowsAdapterPool().Acquire().
func NewRowsAdapterForTest(rows any) (*RowsAdapter, error) {
	return newRowsAdapter(rows)
}

// TestRowsAdapter wraps a mock rows object for testing purposes.
// It implements the same interface as RowsAdapter but works with test mocks.
type TestRowsAdapter struct {
	rows interface {
		Columns() ([]string, error)
		Next() bool
		Scan(...any) error
		Close() error
		Err() error
	}
}

func (tra *TestRowsAdapter) Columns() ([]string, error) {
	cols, err := tra.rows.Columns()
	if err != nil {
		return cols, fmt.Errorf("TestRowsAdapter.Columns: %w", err)
	}
	return cols, nil
}

func (tra *TestRowsAdapter) Next() bool {
	return tra.rows.Next()
}

func (tra *TestRowsAdapter) Scan(dest ...any) error {
	err := tra.rows.Scan(dest...)
	if err != nil {
		return fmt.Errorf("TestRowsAdapter.Scan: %w", err)
	}
	return nil
}

func (tra *TestRowsAdapter) Close() error {
	err := tra.rows.Close()
	if err != nil {
		return fmt.Errorf("TestRowsAdapter.Close: %w", err)
	}
	return nil
}

func (tra *TestRowsAdapter) Err() error {
	err := tra.rows.Err()
	if err != nil {
		return fmt.Errorf("TestRowsAdapter.Err: %w", err)
	}
	return nil
}

// Private methods to satisfy ScanRowsTo requirements
func (tra *TestRowsAdapter) columns() ([]string, error) {
	cols, err := tra.rows.Columns()
	if err != nil {
		return cols, fmt.Errorf("TestRowsAdapter.columns: %w", err)
	}
	return cols, nil
}

func (tra *TestRowsAdapter) next() bool {
	return tra.rows.Next()
}

func (tra *TestRowsAdapter) scan(dest ...any) error {
	err := tra.rows.Scan(dest...)
	if err != nil {
		return fmt.Errorf("TestRowsAdapter.scan: %w", err)
	}
	return nil
}

func (tra *TestRowsAdapter) close() error {
	err := tra.rows.Close()
	if err != nil {
		return fmt.Errorf("TestRowsAdapter.close: %w", err)
	}
	return nil
}

func (tra *TestRowsAdapter) err() error {
	err := tra.rows.Err()
	if err != nil {
		return fmt.Errorf("TestRowsAdapter.err: %w", err)
	}
	return nil
}

// NewRowsAdapterWithMockRows creates a test adapter for testing with mock Rows implementations.
// This helper is used to create adapters from test mocks that implement the sql.Rows interface.
// For use with ScanRowsTo in tests only - do not use with production code.
func NewRowsAdapterWithMockRows(mockRows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
},
) *TestRowsAdapter {
	return &TestRowsAdapter{rows: mockRows}
}
