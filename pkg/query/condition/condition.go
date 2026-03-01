package condition

import (
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

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

	// SupportedOptions generates the SQL fragment for supported options in a query,
	// based on the query type (e.g., "SELECT", "INSERT") and the provided QueryOptions.
	//
	// Parameters:
	//   queryType: The type of SQL query ("SELECT", "INSERT", etc.).
	//   opts: The QueryOptions struct containing optional parameters.
	//   paramBase: The base index to use for placeholders (1-based).
	//
	// Returns:
	//   string: The SQL fragment representing the supported options for the query.
	//   []any:  Arguments corresponding to placeholders in the fragment.
	//   error:  Error if option building/validation fails.
	SupportedOptions(queryType definition.QueryType, opts *options.QueryOptions, paramBase int) (string, []any, error)
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
