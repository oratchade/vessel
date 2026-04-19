//go:build test

// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"database/sql"

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
func ExportScanRows(rows interface{}, cols []string) ([]map[string]any, error) {
	return scanRows(rows, cols)
}
