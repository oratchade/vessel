//go:build test

// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import "tounilab.com/fabric/pkg/query/options"

func ExportValidateQueryOptions(opts *options.QueryOptions) error {
	return validateQueryOptions(opts)
}
