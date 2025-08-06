package options

// QueryOptions holds optional parameters for SQL queries.
// Each field is documented with its intended usage and applicable query types.
type QueryOptions struct {
	// Limit specifies the maximum number of rows to return.
	// Applies to: SELECT, UPDATE, DELETE.
	Limit *int

	// Offset specifies the number of rows to skip before starting to return rows.
	// Applies to: SELECT.
	Offset *int

	// OrderBy specifies the columns to order the results by.
	// Applies to: SELECT, UPDATE, DELETE.
	OrderBy []string

	// Having specifies a HAVING clause for grouped queries.
	// Applies to: SELECT (with GROUP BY).
	Having *string

	// GroupBy specifies the columns to group the results by.
	// Applies to: SELECT.
	GroupBy []string

	// Returning specifies columns to return after INSERT, UPDATE, or DELETE.
	// Applies to: INSERT, UPDATE, DELETE (supported by some databases, e.g., PostgreSQL).
	Returning []string
}
