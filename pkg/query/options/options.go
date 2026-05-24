// Package options defines query option structures for controlling query behavior.
package options

import "tounilab.com/fabric/pkg/query/condition"

// UpsertAction describes what an upsert should do when a uniqueness conflict occurs.
type UpsertAction string

const (
	// UpsertDoNothing skips conflicting rows.
	UpsertDoNothing UpsertAction = "DO NOTHING"
	// UpsertDoUpdate updates existing rows on conflict.
	UpsertDoUpdate UpsertAction = "DO UPDATE"
)

// UpsertOptions describes conflict behavior for an INSERT ... ON CONFLICT style statement.
type UpsertOptions struct {
	// ConflictColumns identifies the uniqueness target for PostgreSQL and SQLite.
	// MySQL ignores this field and uses the table's duplicate-key constraints.
	ConflictColumns []string

	// Action controls whether conflicts are ignored or updated.
	Action UpsertAction

	// UpdateColumns lists inserted columns to copy into the existing row for DoUpdate.
	// If empty and Action is UpsertDoUpdate, all insert columns except conflict columns are updated.
	UpdateColumns []string

	// UpdateValues contains explicit values to set on conflict.
	// These values are parameterized. If provided, they are appended after insert values.
	UpdateValues map[string]any
}

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
//
// Mutual Exclusivity & Interactions:
//   - GroupBy and Having should be used together: Having without GroupBy is
//     semantically invalid. (Some databases may tolerate this; others will error.
//     Always pair them.)
//   - Limit and Offset can be used independently, but Offset without Limit may
//     behave unexpectedly on some engines (e.g., MySQL requires Limit to honor
//     Offset).
//   - OrderBy is optional and can be combined with any other clause.
//   - Returning is database-specific and is used for query preview only:
//     PostgreSQL renders RETURNING, MSSQL renders OUTPUT, and SQLite/MySQL
//     ignore it. Mutation execution methods return ExecResult and reject
//     Returning to avoid silently dropping returned rows.
type QueryOptions struct {
	// Limit specifies the maximum number of rows to return.
	// Applies to: SELECT, UPDATE, DELETE.
	// Must be non-negative (enforced at builder level).
	Limit *int

	// Offset specifies the number of rows to skip before starting to return rows.
	// Applies to: SELECT.
	// Must be non-negative (enforced at builder level).
	// Note: Most engines require Limit when using Offset; refer to your database documentation.
	Offset *int

	// OrderBy specifies the columns and directions to order the results by.
	// Applies to: SELECT, UPDATE, DELETE.
	// Direction must be "ASC" or "DESC" (enforced at builder level, case-insensitive).
	OrderBy []OrderBy

	// Having specifies a HAVING clause for grouped queries.
	// Applies to: SELECT (with GROUP BY).
	// IMPORTANT: Should only be used in conjunction with GroupBy; using Having alone is invalid.
	Having *string

	// HavingCondition specifies a parameterized HAVING condition for grouped queries.
	// Applies to: SELECT (with GROUP BY). Prefer this for dynamic values.
	HavingCondition condition.Condition

	// GroupBy specifies the columns to group the results by.
	// Applies to: SELECT.
	GroupBy []string

	// Returning specifies columns to return after INSERT, UPDATE, or DELETE in query preview.
	// Supported in generated SQL by: PostgreSQL (RETURNING), MSSQL (OUTPUT).
	// Ignored in generated SQL by: SQLite, MySQL.
	// Mutation Exec methods reject Returning because they return ExecResult, not rows.
	Returning []string
}
