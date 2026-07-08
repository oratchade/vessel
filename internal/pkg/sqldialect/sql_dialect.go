package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/vessel/internal/pkg/helpers"
	"tounilab.com/vessel/internal/pkg/operator"
	"tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/definition"
	"tounilab.com/vessel/pkg/query/options"
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
		if opts.HavingCondition != nil {
			having, havingArgs, err := opts.HavingCondition.ToSQL(dialect, next)
			if err != nil {
				return "", nil, fmt.Errorf("HAVING condition: %w", err)
			}
			if opts.Having != nil {
				parts[len(parts)-1] += " AND " + having
			} else {
				parts = append(parts, fmt.Sprintf("%s %s", dialect.Operator(operator.Having), having))
			}
			args = append(args, havingArgs...)
			next += len(havingArgs)
		}

		if len(opts.OrderBy) > 0 {
			orderByFragments, err := getOrderByFragment(dialect, opts.OrderBy)
			if err != nil {
				return "", nil, err
			}
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
		if queryType == definition.QueryTypeUpdate || queryType == definition.QueryTypeDelete {
			if len(opts.OrderBy) > 0 {
				orderByFragments, err := getOrderByFragment(dialect, opts.OrderBy)
				if err != nil {
					return "", nil, err
				}
				parts = append(parts, fmt.Sprintf(
					"%s %s",
					dialect.Operator(operator.OrderBy),
					strings.Join(orderByFragments, ", "),
				))
			}
			if opts.Limit != nil {
				parts = append(parts, formatOperator(dialect, dialect.Operator(operator.Limit), next))
				args = append(args, *opts.Limit)
			}
		}
	}

	return strings.Join(parts, " "), args, nil
}

func getOrderByFragment(dialect condition.SQLDialect, orderBy []options.OrderBy) ([]string, error) {
	orderByFragments := make([]string, 0, len(orderBy))
	for _, order := range orderBy {
		direction, err := normalizeOrderDirection(order.Direction)
		if err != nil {
			return nil, err
		}
		orderByFragments = append(orderByFragments,
			fmt.Sprintf("%s %s", quoteOptionIdentifier(dialect, order.Column), direction),
		)
	}
	return orderByFragments, nil
}

// normalizeOrderDirection validates an ORDER BY direction and returns its
// canonical uppercase form. An empty direction defaults to ASC. Any value
// other than ASC or DESC is rejected so it cannot be concatenated raw into
// the generated SQL. This holds for every entry point (fluent API, direct DB
// calls, and manager queries), not just callers that pre-validate options.
func normalizeOrderDirection(direction string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "", "ASC":
		return "ASC", nil
	case "DESC":
		return "DESC", nil
	default:
		return "", fmt.Errorf("invalid ORDER BY direction %q, must be ASC or DESC", direction)
	}
}

func quoteOptionIdentifier(dialect condition.SQLDialect, value string) string {
	value = strings.TrimSpace(value)
	if value == "*" || strings.ContainsAny(value, " ()") {
		return value
	}
	return helpers.QuoteIdentifierSlice(dialect, []string{value}, "")[0]
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
