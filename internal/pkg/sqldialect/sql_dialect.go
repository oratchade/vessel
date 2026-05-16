// Package sqldialect provides SQL dialect implementations for various database engines.
package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/fabric/internal/pkg/helpers"
	"tounilab.com/fabric/internal/pkg/operator"
	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

// supportedOptions builds SQL fragments for query options like ORDER BY, LIMIT, OFFSET, and RETURNING
// according to the specified query type and dialect-specific implementations.
//
//nolint:cyclop,gocognit
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
		// Build fragments in SQL-correct order: GROUP BY -> HAVING -> ORDER BY
		if len(opts.GroupBy) > 0 {
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				dialect.Operator(operator.GroupBy),
				strings.Join(helpers.QuoteIdentifierSlice(dialect, opts.GroupBy, ""), ", "),
			))
		}

		if opts.Having != nil {
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				dialect.Operator(operator.Having),
				*opts.Having,
			))
		}

		if len(opts.OrderBy) > 0 {
			orderByFragments := getOrderByFragment(dialect, opts.OrderBy)
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				dialect.Operator(operator.OrderBy),
				strings.Join(orderByFragments, ", "),
			))
		}

		// LIMIT/OFFSET (tail) should appear after ORDER BY/HAVING/GROUP BY.
		// MSSQL requires OFFSET before FETCH (limit), so handle dialect-specific ordering.
		switch dialect.(type) {
		case MSSQLDialect:
			// MSSQL requires ORDER BY when using OFFSET/FETCH pagination.
			if (opts.Offset != nil || opts.Limit != nil) && len(opts.OrderBy) == 0 {
				return "", nil, fmt.Errorf("MSSQL pagination requires ORDER BY clause")
			}
			if opts.Offset != nil {
				parts = append(parts, formatOperator(dialect, dialect.Operator(operator.Offset), next))
				args = append(args, *opts.Offset)
				next++
			} else if opts.Limit != nil {
				parts = append(parts, "OFFSET 0 ROWS")
			}
			if opts.Limit != nil {
				parts = append(parts, formatOperator(dialect, dialect.Operator(operator.Limit), next))
				args = append(args, *opts.Limit)
			}
		default:
			if opts.Limit != nil {
				parts = append(parts, formatOperator(dialect, dialect.Operator(operator.Limit), next))
				args = append(args, *opts.Limit)
				next++
			}
			if opts.Offset != nil {
				parts = append(parts, formatOperator(dialect, dialect.Operator(operator.Offset), next))
				args = append(args, *opts.Offset)
			}
		}
	case definition.QueryTypeInsert, definition.QueryTypeUpdate, definition.QueryTypeDelete:
		if len(opts.Returning) > 0 {
			op := dialect.Operator(operator.Returning)
			if op == "" {
				return "", nil, nil
			}
			parts = append(parts, fmt.Sprintf(
				"%s %s",
				op,
				strings.Join(
					helpers.QuoteIdentifierSlice(dialect, opts.Returning, getPrefix(dialect, queryType)),
					", ",
				),
			))
		}
	}

	return strings.Join(parts, " "), args, nil
}

func getOrderByFragment(dialect condition.SQLDialect, orderBy []options.OrderBy) []string {
	orderByFragments := make([]string, 0, len(orderBy))
	for _, order := range orderBy {
		direction := order.Direction
		if direction == "" {
			direction = "ASC"
		}
		orderByFragments = append(orderByFragments,
			fmt.Sprintf("%s %s", dialect.QuoteIdentifier(order.Column), direction),
		)
	}
	return orderByFragments
}

// getPrefix returns the prefix to use for column names in RETURNING/OUTPUT clauses
// based on the query type (e.g., "inserted." for INSERT, "deleted." for DELETE).
func getPrefix(dialect condition.SQLDialect, qt definition.QueryType) string {
	if _, ok := dialect.(MSSQLDialect); !ok {
		return ""
	}
	switch qt {
	case definition.QueryTypeInsert, definition.QueryTypeUpdate:
		return "inserted."
	case definition.QueryTypeDelete:
		return "deleted."
	default:
		return ""
	}
}

// formatOperator formats an operator string with a placeholder, treating operators
// containing '%' as format strings (e.g., "OFFSET %s ROWS" for MSSQL).
func formatOperator(
	dialect condition.SQLDialect,
	op string,
	paramBase int,
) string {
	ph := dialect.Placeholder(paramBase)
	if strings.Contains(op, "%") {
		// treat operator as format string (e.g. "OFFSET %s ROWS")
		return fmt.Sprintf(op, ph)
	}

	return fmt.Sprintf("%s %s", op, ph)
}
