package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/pkg/query"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

// PostgresDialect implements SQL dialect behavior specific to PostgreSQL.
// It provides placeholder syntax, quoting and operator mappings for Postgres.
type PostgresDialect struct{}

func (d PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d PostgresDialect) Operator(op string) string {
	switch strings.ToLower(op) {
	case query.Equal:
		return "="
	case query.NotEqual:
		return "!="
	case query.LowerThan:
		return "<"
	case query.LowerThanOrEqual:
		return "<="
	case query.GreaterThan:
		return ">"
	case query.GreaterThanOrEqual:
		return ">="
	case query.Distinct:
		return strings.ToUpper(query.IsDistinctFrom)
	case query.NotDistinct:
		return "IS NOT DISTINCT FROM"
	case query.Contains:
		return "@>"
	case query.Contained:
		return "<@"
	case query.Overlaps:
		return "&&"
	case query.Regex:
		return "~"
	case query.NotRegex:
		return "!~"
	case query.InsensitiveCaseRegex:
		return "~*"
	case query.NotInsensitiveCaseRegex:
		return "!~*"
	default:
		return strings.ToUpper(op)
	}
}

func (d PostgresDialect) QuoteIdentifier(value string) string {
	return fmt.Sprintf("\"%s\"", value)
}

func (d PostgresDialect) QuoteString(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "''"))
}

func (d PostgresDialect) SupportedOptions(
	queryType definition.QueryType,
	opts *options.QueryOptions,
	paramBase int,
) (string, []any, error) {
	return supportedOptions(d, queryType, opts, paramBase)
}
