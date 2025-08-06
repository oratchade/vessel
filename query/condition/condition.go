package condition

import (
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

type Condition interface {
	ToSQL(dialect SQLDialect, paramBase int) (string, []any, error)
}

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
	//
	// Returns:
	//   string: The SQL fragment representing the supported options for the query.
	SupportedOptions(queryType definition.QueryType, opts *options.QueryOptions) string
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
