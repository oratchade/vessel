// Package options defines query option structures for controlling query behavior.
package options

// OrderBy specifies a column and its sort direction for result ordering.
// Column is the column name, and Direction is either "ASC" or "DESC".
// If Direction is empty, it defaults to "ASC".
type OrderBy struct {
	// Column specifies the column name to order by.
	Column string

	// Direction specifies the sort direction: "ASC" or "DESC".
	// Defaults to "ASC" if empty.
	Direction string
}

// QueryOptions holds optional parameters for SQL queries.
// Each field is documented with its intended usage and applicable query types.
type QueryOptions struct {
	// Limit specifies the maximum number of rows to return.
	// Applies to: SELECT, UPDATE, DELETE.
	Limit *int

	// Offset specifies the number of rows to skip before starting to return rows.
	// Applies to: SELECT.
	Offset *int

	// OrderBy specifies the columns and directions to order the results by.
	// Applies to: SELECT, UPDATE, DELETE.
	OrderBy []OrderBy

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
