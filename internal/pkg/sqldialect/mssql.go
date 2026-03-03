// Package sqldialect provides SQL dialect implementations for various database engines.
package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/internal/pkg/operator"
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
		return strings.ToUpper(operator.Output)
	case operator.InsensitiveCaseLike:
		return strings.ToUpper(operator.Like) // MSSQL does not have a case-insensitive LIKE, so we use LIKE
	case operator.Distinct:
		return strings.ToUpper(operator.IsDistinctFrom) // emulate
	case operator.NotDistinct:
		return "" // MSSQL does not support IS NOT DISTINCT FROM
	case operator.Contains, operator.Contained, operator.Overlaps:
		return strings.ToUpper(operator.Like) // MSSQL does not support @> like Postgres, so we use LIKE
	case operator.Regex, operator.NotRegex, operator.InsensitiveCaseRegex, operator.NotInsensitiveCaseRegex:
		return "" // not supported
	case operator.Limit:
		return "FETCH NEXT %s ROWS ONLY"
	case operator.Offset:
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
