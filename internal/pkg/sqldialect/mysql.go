// Package sqldialect provides SQL dialect implementations for various database engines.
package sqldialect

import (
	"strings"

	"tounilab.com/db-connector/internal/pkg/operator"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

// MySQLDialect implements SQL dialect behavior used by MySQL and SQLite.
// It provides placeholder syntax, quoting and operator mappings for those engines.
type MySQLDialect struct{}

func (d MySQLDialect) Placeholder(_ int) string {
	return "?"
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d MySQLDialect) Operator(op string) string {
	switch strings.ToLower(op) {
	case operator.Equal:
		return "="
	case operator.NotEqual:
		return "!="
	case operator.LowerThan:
		return "<"
	case operator.LowerThanOrEqual:
		return "<="
	case operator.GreaterThan:
		return ">"
	case operator.GreaterThanOrEqual:
		return ">="
	case operator.Returning:
		return ""
	case operator.InsensitiveCaseLike:
		return strings.ToUpper(operator.Like) // MySQL does not have a case-insensitive LIKE, so we use LIKE
	case operator.Distinct:
		// MySQL does not support IS DISTINCT FROM, but we can emulate it
		return strings.ToUpper(operator.IsDistinctFrom)
	case operator.NotDistinct:
		return "" // MySQL does not support IS NOT DISTINCT FROM (no NULL-safe not-equal)
	case operator.Contains, operator.Contained, operator.Overlaps:
		return strings.ToUpper(operator.Like) // MySQL does not support @> like Postgres, so we use LIKE
	case operator.Regex:
		return "REGEXP"
	case operator.NotRegex:
		return "NOT REGEXP"
	case operator.InsensitiveCaseRegex:
		return "REGEXP" // MySQL does not have a case-insensitive regex operator, so we use REGEXP
	case operator.NotInsensitiveCaseRegex:
		return "NOT REGEXP"
	default:
		return strings.ToUpper(op)
	}
}

func (d MySQLDialect) QuoteIdentifier(value string) string {
	return "`" + value + "`"
}

func (d MySQLDialect) QuoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (d MySQLDialect) SupportedOptions(
	queryType definition.QueryType,
	opts *options.QueryOptions,
	paramBase int,
) (string, []any, error) {
	return supportedOptions(d, queryType, opts, paramBase)
}
