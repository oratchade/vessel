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
			parts = append(
				parts,
				expandLimitOffset(
					dialect,
					dialect.Operator(query.Limit),
					next,
				),
			)
			args = append(args, *opts.Limit)
			next++
		}
		if opts.Offset != nil {
			parts = append(
				parts,
				expandLimitOffset(
					dialect,
					dialect.Operator(query.Offset),
					next,
				),
			)
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

func expandLimitOffset(
	dialect condition.SQLDialect,
	op string,
	paramBase int,
) string {
	part := ""
	ph := dialect.Placeholder(paramBase)
	if strings.Contains(op, "%") {
		part = strings.ReplaceAll(op, "%%d", ph)
	} else {
		part = fmt.Sprintf("%s %s", op, ph)
	}
	return part
}
