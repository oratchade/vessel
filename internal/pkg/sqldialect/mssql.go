package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/pkg/query"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

// MSSQLDialect implements SQL dialect behavior specific to Microsoft SQL Server.
// It provides placeholder syntax, quoting and operator mappings for MSSQL.
type MSSQLDialect struct{}

func (d MSSQLDialect) Placeholder(index int) string {
	return fmt.Sprintf("@p%d", index)
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d MSSQLDialect) Operator(op string) string {
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
	case query.Returning:
		return strings.ToUpper(query.Output)
	case query.InsensitiveCaseLike:
		return strings.ToUpper(query.Like) // MSSQL does not have a case-insensitive LIKE, so we use LIKE
	case query.Distinct:
		return strings.ToUpper(query.IsDistinctFrom) // emulate
	case query.NotDistinct, query.Contains, query.Contained, query.Overlaps:
		return strings.ToUpper(query.Like) // MSSQL does not support IS NOT DISTINCT FROM, so we use LIKE
	case query.Regex, query.NotRegex, query.InsensitiveCaseRegex, query.NotInsensitiveCaseRegex:
		return "" // not supported
	case query.Limit:
		return "FETCH NEXT %s ROWS ONLY"
	case query.Offset:
		return "OFFSET %s ROWS"
	default:
		return strings.ToUpper(op)
	}
}

func (d MSSQLDialect) QuoteIdentifier(value string) string {
	return "[" + strings.ReplaceAll(value, "]", "]]") + "]"
}

func (d MSSQLDialect) QuoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (d MSSQLDialect) SupportedOptions(
	queryType definition.QueryType,
	opts *options.QueryOptions,
	paramBase int,
) (string, []any, error) {
	return supportedOptions(d, queryType, opts, paramBase)
}
