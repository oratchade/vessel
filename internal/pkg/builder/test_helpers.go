//go:build test

package builder

import (
	"tounilab.com/fabric/pkg/query/condition"
)

// ExportSanitizeColumn is a test-only wrapper to allow external-package tests
// (package builder_test) to exercise the unexported sanitizeColumn function.
// This file is built only during `go test` because of the //go:build test tag.
func ExportSanitizeColumn(d condition.SQLDialect, column string) string {
	return sanitizeColumn(d, column)
}
