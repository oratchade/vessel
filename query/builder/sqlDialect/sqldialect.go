package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

func supportedOptions(
	dialect condition.SQLDialect,
	queryType definition.QueryType,
	opts *options.QueryOptions,
	paramBase int,
) (string, []any, error) {
	if opts == nil {
		return "", nil, nil
	}

	var parts []string
	var args []any
	next := paramBase

	switch queryType {
	case definition.QueryTypeSelect:
		if opts.Limit != nil {
			ph := dialect.Placeholder(next)
			parts = append(parts, fmt.Sprintf("%s %s", dialect.Operator(query.Limit), ph))
			args = append(args, *opts.Limit)
			next++
		}
		if opts.Offset != nil {
			ph := dialect.Placeholder(next)
			parts = append(parts, fmt.Sprintf("%s %s", dialect.Operator(query.Offset), ph))
			args = append(args, *opts.Offset)
		}
		if len(opts.OrderBy) > 0 {
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				dialect.Operator(query.OrderBy),
				strings.Join(query.QuoteIdentifierSlice(dialect, opts.OrderBy, ""), ", "),
			))
		}
		if opts.Having != nil {
			parts = append(
				parts,
				fmt.Sprintf(
					"%s %s",
					dialect.Operator(query.Having),
					dialect.QuoteIdentifier(*opts.Having),
				),
			)
		}
		if len(opts.GroupBy) > 0 {
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				dialect.Operator(query.GroupBy),
				strings.Join(query.QuoteIdentifierSlice(dialect, opts.GroupBy, ""), ", "),
			))
		}
	case definition.QueryTypeInsert, definition.QueryTypeUpdate, definition.QueryTypeDelete:
		if len(opts.Returning) > 0 {
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				dialect.Operator(query.Returning),
				strings.Join(query.QuoteIdentifierSlice(dialect, opts.Returning, getPrefix(queryType)), ", "),
			))
		}
	}

	return strings.Join(parts, " "), args, nil
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
