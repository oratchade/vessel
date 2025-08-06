package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	cdt "tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/options"
)

func retrieveSelectOpts(dialect cdt.SQLDialect, opts *options.QueryOptions) []string {
	var o []string

	if opts == nil {
		return o
	}

	if opts.Limit != nil {
		o = append(o, fmt.Sprintf("%s %d", dialect.Operator(query.Limit), *opts.Limit))
	}
	if opts.Offset != nil {
		o = append(o, fmt.Sprintf("%s %d", dialect.Operator(query.Offset), *opts.Offset))
	}
	if len(opts.OrderBy) > 0 {
		o = append(o, fmt.Sprintf(
			"%s %s",
			dialect.Operator(query.OrderBy),
			strings.Join(query.QuoteIdentifierSlice(dialect, opts.OrderBy, ""), ", "),
		))
	}
	if opts.Having != nil {
		o = append(o, fmt.Sprintf("%s %s", dialect.Operator(query.Having), dialect.QuoteIdentifier(*opts.Having)))
	}
	if len(opts.GroupBy) > 0 {
		o = append(o, fmt.Sprintf(
			"%s %s",
			dialect.Operator(query.GroupBy),
			strings.Join(query.QuoteIdentifierSlice(dialect, opts.GroupBy, ""), ", "),
		))
	}

	return o
}
