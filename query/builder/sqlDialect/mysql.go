package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

// MySQL and SQLite use the same placeholder syntax
// and similar operators, so we can use the same dialect for both.

type MySQLDialect struct{}

func (d MySQLDialect) Placeholder(_ int) string {
	return "?"
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d MySQLDialect) Operator(op string) string {
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
		return ""
	case query.InsensitiveCaseLike:
		return strings.ToUpper(query.Like) // MySQL does not have a case-insensitive LIKE, so we use LIKE
	case query.Distinct:
		return strings.ToUpper(query.IsDistinctFrom) // MySQL does not support IS DISTINCT FROM, but we can emulate it
	case query.NotDistinct:
		return "IS NOT DISTINCT FROM"
	case query.Contains, query.Contained, query.Overlaps:
		return strings.ToUpper(query.Like) // MySQL does not support @> like Postgres, so we use LIKE
	case query.Regex:
		return "REGEXP"
	case query.NotRegex:
		return "NOT REGEXP"
	case query.InsensitiveCaseRegex:
		return "REGEXP" // MySQL does not have a case-insensitive regex operator, so we use REGEXP
	case query.NotInsensitiveCaseRegex:
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

func (d MySQLDialect) SupportedOptions(queryType definition.QueryType, opts *options.QueryOptions) string {
	var o []string

	// Avoid reflection for performance: access fields directly
	if opts == nil {
		return ""
	}

	if queryType == definition.QueryTypeSelect {
		if opts.Limit != nil {
			o = append(o, fmt.Sprintf("%s %d", d.Operator(query.Limit), *opts.Limit))
		}
		if opts.Offset != nil {
			o = append(o, fmt.Sprintf("%s %d", d.Operator(query.Offset), *opts.Offset))
		}
		if len(opts.OrderBy) > 0 {
			o = append(o, fmt.Sprintf(
				"%s %s",
				d.Operator(query.OrderBy),
				strings.Join(query.QuoteIdentifierSlice(d, opts.OrderBy, ""), ", "),
			))
		}
		if opts.Having != nil {
			o = append(o, fmt.Sprintf("%s %s", d.Operator(query.Having), d.QuoteIdentifier(*opts.Having)))
		}
		if len(opts.GroupBy) > 0 {
			o = append(o, fmt.Sprintf(
				"%s %s",
				d.Operator(query.GroupBy),
				strings.Join(query.QuoteIdentifierSlice(d, opts.GroupBy, ""), ", "),
			))
		}
	}

	return strings.Join(o, " ")
}
