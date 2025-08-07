package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

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
		return "FETCH NEXT %d ROWS ONLY"
	case query.Offset:
		return "OFFSET %d ROWS"
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

func (d MSSQLDialect) SupportedOptions(queryType definition.QueryType, opts *options.QueryOptions) string {
	var o []string

	// Avoid reflection for performance: access fields directly
	if opts == nil {
		return ""
	}

	switch queryType {
	case definition.QueryTypeSelect:
		if opts.Limit != nil {
			o = append(o, fmt.Sprintf(d.Operator(query.Limit), *opts.Limit))
		}
		if opts.Offset != nil {
			o = append(o, fmt.Sprintf(d.Operator(query.Offset), *opts.Offset))
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
	case definition.QueryTypeInsert, definition.QueryTypeUpdate, definition.QueryTypeDelete:
		if len(opts.Returning) > 0 {
			o = append(o, fmt.Sprintf(
				"%s %s",
				d.Operator(query.Returning),
				strings.Join(query.QuoteIdentifierSlice(d, opts.Returning, getPrefix(queryType)), ", "),
			))
		}
	}

	return strings.Join(o, " ")
}

func getPrefix(qt definition.QueryType) string {
	switch qt {
	case definition.QueryTypeInsert, definition.QueryTypeUpdate:
		return "inserted."
	case definition.QueryTypeDelete:
		return "deleted."
	default:
		return ""
	}
}
