package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/vessel/internal/pkg/operator"
	"tounilab.com/vessel/pkg/query/definition"
	"tounilab.com/vessel/pkg/query/options"
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
	case operator.As:
		return "AS"
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
	case operator.Distinct:
		return strings.ToUpper(operator.IsDistinctFrom)
	case operator.NotDistinct:
		return "IS NOT DISTINCT FROM"
	case operator.Contains:
		return "@>"
	case operator.Contained:
		return "<@"
	case operator.Overlaps:
		return "&&"
	case operator.Regex:
		return "~"
	case operator.NotRegex:
		return "!~"
	case operator.InsensitiveCaseRegex:
		return "~*"
	case operator.NotInsensitiveCaseRegex:
		return "!~*"
	default:
		return strings.ToUpper(op)
	}
}

func (d PostgresDialect) QuoteIdentifier(value string) string {
	return fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\"\""))
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

func (d PostgresDialect) Capabilities() Capabilities {
	return Capabilities{
		SelectPagination:      true,
		MutationReturning:     true,
		Upsert:                true,
		JoinedUpdate:          true,
		JoinedDelete:          true,
		JoinedDeleteWithUsing: true,
	}
}
