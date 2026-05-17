// Package condition provides composable SQL condition expressions and SQL dialect abstractions
// for building parameterized queries.
package condition

//go:generate mockgen -source=condition.go -destination=condition_mocks.go -package=condition Condition

// Condition represents a composable SQL condition expression that can be
// converted to a parameterized SQL fragment for a specific dialect.
type Condition interface {
	ToSQL(dialect SQLDialect, paramBase int) (string, []any, error)
}

//go:generate mockgen -source=condition.go -destination=condition_mocks.go -package=condition SQLDialect

// SQLDialect defines the dialect-specific behaviors required by the query
// builder: placeholder formatting, operator mapping and quoting rules.
type SQLDialect interface {
	Placeholder(index int) string
	Operator(op string) string
	QuoteIdentifier(value string) string
	QuoteString(value string) string
}

/*
	conditions := And{
	    Expr{"age", ">", 30},
	    Or{
	        Expr{"status", "=", "active"},
	        Expr{"status", "=", "pending"},
	    },
	    Not{
	        Cond: Expr{"deleted_at", strings.ToUpper(query.IsNotNull), nil},
	    },
		In{
			column:   "id",
			operator: strings.ToUpper(query.In),
			values:   []any{1, 2, 3},
		},
		Between{
			column:    "created_at",
			operator:  strings.ToUpper(query.Between),
			from:      "2023-01-01",
			to:        "2023-12-31",
		},
	}
*/
