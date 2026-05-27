package tests

import (
	"strings"

	"tounilab.com/vessel/pkg/query/options"
)

// Mock SQLDialect for testing
type MockDialect struct{}

func (m MockDialect) Placeholder(_ int) string {
	return "?"
}

func (m MockDialect) Operator(op string) string {
	return strings.ToUpper(op)
}

func (m MockDialect) QuoteIdentifier(id string) string {
	return "`" + id + "`"
}

func (m MockDialect) QuoteString(id string) string {
	return `"` + id + `"`
}

func (m MockDialect) SupportedOptions(queryType string,
	opts *options.QueryOptions,
	paramBase int,
) (string, []any, error) {
	return "", nil, nil
}
