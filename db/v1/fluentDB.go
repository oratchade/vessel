// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"context"
	"fmt"
	"strings"

	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

// Direction constants for ORDER BY clauses.
const (
	// AscDirection specifies ascending sort order.
	AscDirection = "ASC"
	// DescDirection specifies descending sort order.
	DescDirection = "DESC"
)

const ascDirection = AscDirection //nolint:unused // Kept for backward compatibility

// ValidateQueryOptions checks query options for validity and returns an error if invalid.
// Returns nil if all options are valid.
// This is exported for testing validation logic independently.
func ValidateQueryOptions(opts *options.QueryOptions) error {
	if opts == nil {
		return nil
	}

	// Validate Limit is non-negative
	if opts.Limit != nil && *opts.Limit < 0 {
		return fmt.Errorf("Limit: cannot be negative, got %d", *opts.Limit)
	}

	// Validate Offset is non-negative
	if opts.Offset != nil && *opts.Offset < 0 {
		return fmt.Errorf("Offset: cannot be negative, got %d", *opts.Offset)
	}

	// Validate OrderBy directions
	if opts.OrderBy != nil {
		for _, ob := range opts.OrderBy {
			if ob.Direction != AscDirection && ob.Direction != DescDirection {
				return fmt.Errorf(
					"OrderBy: invalid direction %q for column %q, must be ASC or DESC",
					ob.Direction, ob.Column,
				)
			}
		}
	}

	return nil
}

// validateQueryOptions is the internal version of ValidateQueryOptions
func validateQueryOptions(opts *options.QueryOptions) error {
	return ValidateQueryOptions(opts)
}

// FluentDB provides a fluent/builder interface for constructing and executing database queries.
// It acts as an entry point for building SELECT, INSERT, UPDATE, and DELETE operations
// with a chainable, ergonomic API while reusing the existing DBActions interface.
type FluentDB struct {
	db  DBActions
	ctx context.Context
}

// NewFluentDB creates a new FluentDB instance that wraps the provided DBActions.
//
// Parameters:
//
//	db: The underlying database actions interface to execute queries.
//	ctx: Context for cancellation and deadlines, propagated to all queries.
//
// Returns:
//
//	*FluentDB: A new fluent database builder.
//
// Example:
//
//	result, err := NewFluentDB(db, ctx).
//	    Select("users", "id", "name", "email").
//	    Where(cdt.NewExpr().Column("age").Op(">").Value(18)).
//	    Get()
func NewFluentDB(db DBActions, ctx context.Context) *FluentDB {
	return &FluentDB{db: db, ctx: ctx}
}

// Select begins a SELECT query by specifying the table and columns to retrieve.
//
// Parameters:
//
//	table: The name of the table to query.
//	columns: The column names to select (use "*" or omit for all columns).
//
// Returns:
//
//	*SelectBuilder: A builder for chaining additional query parameters.
//
// Example:
//
//	NewFluentDB(db, ctx).
//	    Select("users", "id", "name").
//	    Where(...).
//	    Get()
func (f *FluentDB) Select(table string, columns ...string) *SelectBuilder {
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	return &SelectBuilder{
		db:      f.db,
		ctx:     f.ctx,
		table:   table,
		columns: columns,
	}
}

// Insert begins an INSERT query.
//
// Returns:
//
//	*InsertBuilder: A builder for specifying table and values.
//
// Example:
//
//	NewFluentDB(db, ctx).
//	    Insert().
//	    Into("users").
//	    Values(map[string]any{"name": "John", "age": 30}).
//	    Exec()
func (f *FluentDB) Insert() *InsertBuilder {
	return &InsertBuilder{
		db:  f.db,
		ctx: f.ctx,
	}
}

// Update begins an UPDATE query by specifying the table.
//
// Parameters:
//
//	table: The name of the table to update.
//
// Returns:
//
//	*UpdateBuilder: A builder for specifying values and conditions.
//
// Example:
//
//	NewFluentDB(db, ctx).
//	    Update("users").
//	    Set("age", 31).
//	    Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
//	    Exec()
func (f *FluentDB) Update(table string) *UpdateBuilder {
	return &UpdateBuilder{
		db:    f.db,
		ctx:   f.ctx,
		table: table,
		data:  make(map[string]any),
	}
}

// Delete begins a DELETE query.
//
// Returns:
//
//	*DeleteBuilder: A builder for specifying the table and conditions.
//
// Example:
//
//	NewFluentDB(db, ctx).
//	    Delete().
//	    From("users").
//	    Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
//	    Exec()
func (f *FluentDB) Delete() *DeleteBuilder {
	return &DeleteBuilder{
		db:  f.db,
		ctx: f.ctx,
	}
}

// SelectBuilder is a fluent builder for SELECT queries.
// It allows chainable method calls to construct complex queries with
// joins, conditions, ordering, and pagination.
type SelectBuilder struct {
	db         DBActions
	ctx        context.Context
	table      string
	columns    []string
	joins      []cdt.Join
	conditions cdt.Condition
	opts       *options.QueryOptions
}

// WithTx specifies a transaction to execute this query within.
// If a transaction is provided, the query will use the transaction's DBActions
// instead of the original database connection.
//
// Parameters:
//
//	tx: A Tx interface (typically from DB.Begin()).
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
func (s *SelectBuilder) WithTx(tx Tx) *SelectBuilder {
	if tx != nil {
		s.db = tx
	}
	return s
}

// Where adds a WHERE condition to the SELECT query.
// Conditions are combined with any existing conditions using AND logic.
//
// Parameters:
//
//	cond: A Condition object (e.g., from cdt.NewExpr(), cdt.NewAnd(), etc.).
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
func (s *SelectBuilder) Where(cond cdt.Condition) *SelectBuilder {
	if cond == nil {
		return s
	}
	if s.conditions == nil {
		s.conditions = cond
	} else {
		s.conditions = cdt.NewAnd().Conditions(s.conditions, cond)
	}
	return s
}

// Join adds a single JOIN clause to the SELECT query.
//
// Parameters:
//
//	join: A Join struct describing the join operation.
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
func (s *SelectBuilder) Join(join cdt.Join) *SelectBuilder {
	if s.joins == nil {
		s.joins = make([]cdt.Join, 0)
	}
	s.joins = append(s.joins, join)
	return s
}

// Joins adds multiple JOIN clauses to the SELECT query.
//
// Parameters:
//
//	joins: A slice of Join structs.
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
func (s *SelectBuilder) Joins(joins []cdt.Join) *SelectBuilder {
	if len(joins) == 0 {
		return s
	}
	if s.joins == nil {
		s.joins = make([]cdt.Join, 0, len(joins))
	}
	s.joins = append(s.joins, joins...)
	return s
}

// OrderBy adds an ORDER BY clause to the SELECT query.
// Can be called multiple times to order by multiple columns.
//
// Parameters:
//
//	column: The column name to order by.
//	direction: The sort direction ("ASC" or "DESC", case-insensitive).
//	           Defaults to "ASC" if empty.
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
//
// Note: Direction validation is deferred to execution time (Get, GetRaw, etc.).
func (s *SelectBuilder) OrderBy(column, direction string) *SelectBuilder {
	if s.opts == nil {
		s.opts = &options.QueryOptions{}
	}
	if s.opts.OrderBy == nil {
		s.opts.OrderBy = make([]options.OrderBy, 0)
	}
	dir := direction
	if dir == "" {
		dir = AscDirection
	} else {
		dir = strings.ToUpper(dir)
	}
	s.opts.OrderBy = append(s.opts.OrderBy, options.OrderBy{
		Column:    column,
		Direction: dir,
	})
	return s
}

// Limit sets the maximum number of rows to return.
//
// Parameters:
//
//	limit: Maximum number of rows (can be any value; validation occurs at execution time).
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
//
// Note: Limit validation is deferred to execution time (Get, GetRaw, etc.).
func (s *SelectBuilder) Limit(limit int) *SelectBuilder {
	if s.opts == nil {
		s.opts = &options.QueryOptions{}
	}
	s.opts.Limit = &limit
	return s
}

// Offset sets the number of rows to skip before returning results.
//
// Parameters:
//
//	offset: The number of rows to skip (can be any value; validation occurs at execution time).
//
// Returns:
//
//	*SelectBuilder: The builder for method chaining.
//
// Note: Offset validation is deferred to execution time (Get, GetRaw, etc.).
func (s *SelectBuilder) Offset(offset int) *SelectBuilder {
	if s.opts == nil {
		s.opts = &options.QueryOptions{}
	}
	s.opts.Offset = &offset
	return s
}

// Get executes the SELECT query and returns all matching rows as a slice of maps.
//
// Returns:
//
//	[]map[string]any: A slice of rows, each as a map of column names to values.
//	error: An error if the query fails or the table/columns are invalid.
//
// Example:
//
//	rows, err := NewFluentDB(db, ctx).
//	    Select("users", "id", "name").
//	    Where(cdt.NewExpr().Column("age").Op(">").Value(18)).
//	    Get()
func (s *SelectBuilder) Get() ([]map[string]any, error) {
	if s.table == "" {
		return nil, fmt.Errorf("selectBuilder: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return nil, fmt.Errorf("selectBuilder: invalid query options: %w", err)
	}
	rows, err := s.db.Get(s.ctx, s.table, s.columns, s.joins, s.conditions, s.opts)
	if err != nil {
		return nil, fmt.Errorf("selectBuilder: failed to get rows: %w", err)
	}
	return rows, nil
}

// GetRaw executes the SELECT query and returns a RowsAdapter for streaming access.
// This is useful for large result sets that should not be materialized into memory all at once.
//
// Returns:
//
//	*RowsAdapter: An adapter for iterating over result rows.
//	error: An error if the query fails or the table/columns are invalid.
func (s *SelectBuilder) GetRaw() (*RowsAdapter, error) {
	if s.table == "" {
		return nil, fmt.Errorf("selectBuilder: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return nil, fmt.Errorf("selectBuilder: invalid query options: %w", err)
	}
	rows, err := s.db.GetRaw(s.ctx, s.table, s.columns, s.joins, s.conditions, s.opts)
	if err != nil {
		return nil, fmt.Errorf("selectBuilder: failed to get raw rows: %w", err)
	}
	return rows, nil
}

// One executes the SELECT query with Limit(1) and returns the first matching row.
// Returns an error if no rows are found.
//
// Returns:
//
//	map[string]any: The first matching row as a map of column names to values.
//	error: An error if the query fails, table/columns are invalid, or no rows are found.
//
// Example:
//
//	user, err := NewFluentDB(db, ctx).
//	    Select("users", "id", "name", "email").
//	    Where(cdt.NewExpr().Column("id").Op("=").Value(42)).
//	    One()
func (s *SelectBuilder) One() (map[string]any, error) {
	if s.table == "" {
		return nil, fmt.Errorf("selectBuilder: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return nil, fmt.Errorf("selectBuilder: invalid query options: %w", err)
	}

	// Clone options and set limit to 1
	opts := s.opts
	if opts == nil {
		limit := 1
		opts = &options.QueryOptions{Limit: &limit}
	} else {
		// Make a copy to avoid modifying the original
		optsCopy := *opts
		limit := 1
		optsCopy.Limit = &limit
		opts = &optsCopy
	}

	rows, err := s.db.Get(s.ctx, s.table, s.columns, s.joins, s.conditions, opts)
	if err != nil {
		return nil, fmt.Errorf("selectBuilder: failed to get row: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("selectBuilder: no rows found")
	}
	return rows[0], nil
}

// Count executes a COUNT(*) query on the specified table with the same
// WHERE conditions and JOINs as the current builder.
//
// Returns:
//
//	int64: The number of matching rows.
//	error: An error if the query fails.
//
// Example:
//
//	count, err := NewFluentDB(db, ctx).
//	    Select("users").
//	    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
//	    Count()
func (s *SelectBuilder) Count() (int64, error) {
	if s.table == "" {
		return 0, fmt.Errorf("selectBuilder: table not specified")
	}

	rows, err := s.db.Get(s.ctx, s.table, []string{"COUNT(*) as count"}, s.joins, s.conditions, nil)
	if err != nil {
		return 0, fmt.Errorf("selectBuilder: failed to count rows: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	countVal, exists := rows[0]["count"]
	if !exists {
		return 0, fmt.Errorf("selectBuilder: count query did not return a count value")
	}

	// Handle different database driver return types for COUNT (int64, int, etc.)
	switch v := countVal.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("selectBuilder: unexpected count type: %T", countVal)
	}
}

// InsertBuilder is a fluent builder for INSERT queries.
// It allows specification of the table and values to insert, either as
// single or bulk operations.
type InsertBuilder struct {
	db    DBActions
	ctx   context.Context
	table string
	data  map[string]any
	bulk  []map[string]any
	opts  *options.QueryOptions
}

// WithTx specifies a transaction to execute this query within.
// If a transaction is provided, the query will use the transaction's DBActions
// instead of the original database connection.
//
// Parameters:
//
//	tx: A Tx interface (typically from DB.Begin()).
//
// Returns:
//
//	*InsertBuilder: The builder for method chaining.
func (i *InsertBuilder) WithTx(tx Tx) *InsertBuilder {
	if tx != nil {
		i.db = tx
	}
	return i
}

// Into specifies the table to insert into.
//
// Parameters:
//
//	table: The name of the table.
//
// Returns:
//
//	*InsertBuilder: The builder for method chaining.
func (i *InsertBuilder) Into(table string) *InsertBuilder {
	i.table = table
	return i
}

// Values sets the values for a single INSERT operation.
// Subsequent calls replace previous values.
//
// Parameters:
//
//	data: A map of column names to values.
//
// Returns:
//
//	*InsertBuilder: The builder for method chaining.
func (i *InsertBuilder) Values(data map[string]any) *InsertBuilder {
	i.data = data
	i.bulk = nil // Clear bulk data when setting single values
	return i
}

// ValuesBulk sets multiple rows for a bulk INSERT operation.
// Subsequent calls replace previous bulk data.
//
// Parameters:
//
//	data: A slice of maps, each representing a row to insert.
//
// Returns:
//
//	*InsertBuilder: The builder for method chaining.
func (i *InsertBuilder) ValuesBulk(data []map[string]any) *InsertBuilder {
	i.bulk = data
	i.data = nil // Clear single data when setting bulk values
	return i
}

// Set sets an individual column value for the INSERT operation.
// Multiple calls accumulate values.
//
// Parameters:
//
//	column: The column name.
//	value: The value to insert.
//
// Returns:
//
//	*InsertBuilder: The builder for method chaining.
func (i *InsertBuilder) Set(column string, value any) *InsertBuilder {
	if i.data == nil {
		i.data = make(map[string]any)
	}
	i.data[column] = value
	return i
}

// SetMap sets multiple column values for the INSERT operation.
// Values from this call are merged with any existing values.
//
// Parameters:
//
//	data: A map of column names to values.
//
// Returns:
//
//	*InsertBuilder: The builder for method chaining.
func (i *InsertBuilder) SetMap(data map[string]any) *InsertBuilder {
	if i.data == nil {
		i.data = make(map[string]any)
	}
	for k, v := range data {
		i.data[k] = v
	}
	return i
}

// Exec executes the INSERT query and returns the result.
// Uses bulk insert if ValuesBulk was called, otherwise uses single insert.
//
// Returns:
//
//	*ExecResult: The result of the insert operation (LastInsertID, RowsAffected).
//	error: An error if the insert fails or required parameters are missing.
//
// Example:
//
//	result, err := NewFluentDB(db, ctx).
//	    Insert().
//	    Into("users").
//	    Set("name", "John").
//	    Set("email", "john@example.com").
//	    Exec()
func (i *InsertBuilder) Exec() (*ExecResult, error) {
	if i.table == "" {
		return nil, fmt.Errorf("insertBuilder: table not specified")
	}

	// Use bulk insert if data was provided via ValuesBulk
	if len(i.bulk) > 0 {
		result, err := i.db.Inserts(i.ctx, i.table, i.bulk, i.opts)
		if err != nil {
			return nil, fmt.Errorf("insertBuilder: failed to insert bulk data: %w", err)
		}
		return result, nil
	}

	// Use single insert
	if len(i.data) == 0 {
		return nil, fmt.Errorf("insertBuilder: no data provided")
	}
	result, err := i.db.Insert(i.ctx, i.table, i.data, i.opts)
	if err != nil {
		return nil, fmt.Errorf("insertBuilder: failed to insert data: %w", err)
	}
	return result, nil
}

// UpdateBuilder is a fluent builder for UPDATE queries.
// It allows specification of the table, values to update, conditions to filter rows,
// and optional joins for complex updates.
type UpdateBuilder struct {
	db         DBActions
	ctx        context.Context
	table      string
	data       map[string]any
	joins      []cdt.Join
	conditions cdt.Condition
	opts       *options.QueryOptions
}

// WithTx specifies a transaction to execute this query within.
// If a transaction is provided, the query will use the transaction's DBActions
// instead of the original database connection.
//
// Parameters:
//
//	tx: A Tx interface (typically from DB.Begin()).
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
func (u *UpdateBuilder) WithTx(tx Tx) *UpdateBuilder {
	if tx != nil {
		u.db = tx
	}
	return u
}

// Set sets an individual column value for the UPDATE operation.
// Multiple calls accumulate values.
//
// Parameters:
//
//	column: The column name.
//	value: The new value.
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
func (u *UpdateBuilder) Set(column string, value any) *UpdateBuilder {
	if u.data == nil {
		u.data = make(map[string]any)
	}
	u.data[column] = value
	return u
}

// SetMap sets multiple column values for the UPDATE operation.
// Values from this call are merged with any existing values.
//
// Parameters:
//
//	data: A map of column names to new values.
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
func (u *UpdateBuilder) SetMap(data map[string]any) *UpdateBuilder {
	if u.data == nil {
		u.data = make(map[string]any)
	}
	for k, v := range data {
		u.data[k] = v
	}
	return u
}

// Where adds a WHERE condition to the UPDATE query.
// Conditions are combined with any existing conditions using AND logic.
//
// Parameters:
//
//	cond: A Condition object.
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
func (u *UpdateBuilder) Where(cond cdt.Condition) *UpdateBuilder {
	if cond == nil {
		return u
	}
	if u.conditions == nil {
		u.conditions = cond
	} else {
		u.conditions = cdt.NewAnd().Conditions(u.conditions, cond)
	}
	return u
}

// Join adds a single JOIN clause to the UPDATE query.
// Note: Not all databases support UPDATE with JOINs (SQLite does not).
//
// Parameters:
//
//	join: A Join struct describing the join operation.
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
func (u *UpdateBuilder) Join(join cdt.Join) *UpdateBuilder {
	if u.joins == nil {
		u.joins = make([]cdt.Join, 0)
	}
	u.joins = append(u.joins, join)
	return u
}

// Joins adds multiple JOIN clauses to the UPDATE query.
//
// Parameters:
//
//	joins: A slice of Join structs.
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
func (u *UpdateBuilder) Joins(joins []cdt.Join) *UpdateBuilder {
	if len(joins) == 0 {
		return u
	}
	if u.joins == nil {
		u.joins = make([]cdt.Join, 0, len(joins))
	}
	u.joins = append(u.joins, joins...)
	return u
}

// OrderBy adds an ORDER BY clause to the UPDATE query.
// Can be called multiple times to order by multiple columns.
//
// Parameters:
//
//	column: The column name to order by.
//	direction: The sort direction ("ASC" or "DESC"). Defaults to "ASC" if empty.
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
//
// Note: Direction validation is deferred to execution time (Exec, UpdateAll).
func (u *UpdateBuilder) OrderBy(column, direction string) *UpdateBuilder {
	if u.opts == nil {
		u.opts = &options.QueryOptions{}
	}
	if u.opts.OrderBy == nil {
		u.opts.OrderBy = make([]options.OrderBy, 0)
	}
	dir := direction
	if dir == "" {
		dir = AscDirection
	} else {
		dir = strings.ToUpper(dir)
	}
	u.opts.OrderBy = append(u.opts.OrderBy, options.OrderBy{
		Column:    column,
		Direction: dir,
	})
	return u
}

// Limit sets the maximum number of rows to update.
//
// Parameters:
//
//	limit: The maximum number of rows to update (can be any value; validation occurs at execution time).
//
// Returns:
//
//	*UpdateBuilder: The builder for method chaining.
//
// Note: Limit validation is deferred to execution time (Exec, UpdateAll).
func (u *UpdateBuilder) Limit(limit int) *UpdateBuilder {
	if u.opts == nil {
		u.opts = &options.QueryOptions{}
	}
	u.opts.Limit = &limit
	return u
}

// Exec executes the UPDATE query and returns the result.
//
// Returns:
//
//	*ExecResult: The result of the update operation (RowsAffected).
//	error: An error if the update fails or required parameters are missing.
//
// Example:
//
//	result, err := NewFluentDB(db, ctx).
//	    Update("users").
//	    Set("age", 31).
//	    Set("updated_at", time.Now()).
//	    Where(cdt.NewExpr().Column("id").Op("=").Value(42)).
//	    Exec()
func (u *UpdateBuilder) Exec() (*ExecResult, error) {
	if u.table == "" {
		return nil, fmt.Errorf("updateBuilder: table not specified")
	}
	if len(u.data) == 0 {
		return nil, fmt.Errorf("updateBuilder: no data to update")
	}
	if u.conditions == nil {
		return nil, fmt.Errorf(
			"updateBuilder: WHERE condition required (use Where method or call UpdateAll for unfiltered update)")
	}
	if err := validateQueryOptions(u.opts); err != nil {
		return nil, fmt.Errorf("updateBuilder: invalid query options: %w", err)
	}
	result, err := u.db.Update(u.ctx, u.table, u.data, u.joins, u.conditions, u.opts)
	if err != nil {
		return nil, fmt.Errorf("updateBuilder: failed to update rows: %w", err)
	}
	return result, nil
}

// UpdateAll executes an UPDATE query without a WHERE condition.
// ⚠️ WARNING: This updates ALL rows in the table. Use with extreme caution.
// Requires explicit method call to prevent accidental data loss.
//
// Returns:
//
//	*ExecResult: The result of the update operation (RowsAffected).
//	error: An error if the update fails.
//
// Example:
//
//	// Update all users' status field
//	result, err := NewFluentDB(db, ctx).
//	    Update("users").
//	    Set("status", "inactive").
//	    UpdateAll()  // No WHERE clause
func (u *UpdateBuilder) UpdateAll() (*ExecResult, error) {
	if u.table == "" {
		return nil, fmt.Errorf("updateBuilder: table not specified")
	}
	if err := validateQueryOptions(u.opts); err != nil {
		return nil, fmt.Errorf("updateBuilder: invalid query options: %w", err)
	}
	if len(u.data) == 0 {
		return nil, fmt.Errorf("updateBuilder: no data to update")
	}
	result, err := u.db.Update(u.ctx, u.table, u.data, u.joins, u.conditions, u.opts)
	if err != nil {
		return nil, fmt.Errorf("updateBuilder: failed to update rows: %w", err)
	}
	return result, nil
}

// DeleteBuilder is a fluent builder for DELETE queries.
// It allows specification of the table, conditions to filter rows,
// and optional joins for complex deletes.
type DeleteBuilder struct {
	db         DBActions
	ctx        context.Context
	table      string
	joins      []cdt.Join
	conditions cdt.Condition
	opts       *options.QueryOptions
}

// WithTx specifies a transaction to execute this query within.
// If a transaction is provided, the query will use the transaction's DBActions
// instead of the original database connection.
//
// Parameters:
//
//	tx: A Tx interface (typically from DB.Begin()).
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
func (d *DeleteBuilder) WithTx(tx Tx) *DeleteBuilder {
	if tx != nil {
		d.db = tx
	}
	return d
}

// From specifies the table to delete from.
//
// Parameters:
//
//	table: The name of the table.
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
func (d *DeleteBuilder) From(table string) *DeleteBuilder {
	d.table = table
	return d
}

// Where adds a WHERE condition to the DELETE query.
// Conditions are combined with any existing conditions using AND logic.
//
// Parameters:
//
//	cond: A Condition object.
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
func (d *DeleteBuilder) Where(cond cdt.Condition) *DeleteBuilder {
	if cond == nil {
		return d
	}
	if d.conditions == nil {
		d.conditions = cond
	} else {
		d.conditions = cdt.NewAnd().Conditions(d.conditions, cond)
	}
	return d
}

// Join adds a single JOIN clause to the DELETE query.
// Note: SQLite does not support DELETE with JOINs.
//
// Parameters:
//
//	join: A Join struct describing the join operation.
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
func (d *DeleteBuilder) Join(join cdt.Join) *DeleteBuilder {
	if d.joins == nil {
		d.joins = make([]cdt.Join, 0)
	}
	d.joins = append(d.joins, join)
	return d
}

// Joins adds multiple JOIN clauses to the DELETE query.
//
// Parameters:
//
//	joins: A slice of Join structs.
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
func (d *DeleteBuilder) Joins(joins []cdt.Join) *DeleteBuilder {
	if len(joins) == 0 {
		return d
	}
	if d.joins == nil {
		d.joins = make([]cdt.Join, 0, len(joins))
	}
	d.joins = append(d.joins, joins...)
	return d
}

// OrderBy adds an ORDER BY clause to the DELETE query.
// Can be called multiple times to order by multiple columns.
//
// Parameters:
//
//	column: The column name to order by.
//	direction: The sort direction ("ASC" or "DESC"). Defaults to "ASC" if empty.
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
//
// Note: Direction validation is deferred to execution time (Exec, DeleteAll).
func (d *DeleteBuilder) OrderBy(column, direction string) *DeleteBuilder {
	if d.opts == nil {
		d.opts = &options.QueryOptions{}
	}
	if d.opts.OrderBy == nil {
		d.opts.OrderBy = make([]options.OrderBy, 0)
	}
	dir := direction
	if dir == "" {
		dir = AscDirection
	} else {
		dir = strings.ToUpper(dir)
	}
	d.opts.OrderBy = append(d.opts.OrderBy, options.OrderBy{
		Column:    column,
		Direction: dir,
	})
	return d
}

// Limit sets the maximum number of rows to delete.
//
// Parameters:
//
//	limit: The maximum number of rows to delete (can be any value; validation occurs at execution time).
//
// Returns:
//
//	*DeleteBuilder: The builder for method chaining.
//
// Note: Limit validation is deferred to execution time (Exec, DeleteAll).
func (d *DeleteBuilder) Limit(limit int) *DeleteBuilder {
	if d.opts == nil {
		d.opts = &options.QueryOptions{}
	}
	d.opts.Limit = &limit
	return d
}

// Exec executes the DELETE query and returns the result.
//
// Returns:
//
//	*ExecResult: The result of the delete operation (RowsAffected).
//	error: An error if the delete fails or required parameters are missing.
//
// Example:
//
//	result, err := NewFluentDB(db, ctx).
//	    Delete().
//	    From("users").
//	    Where(cdt.NewExpr().Column("id").Op("=").Value(42)).
//	    Exec()
func (d *DeleteBuilder) Exec() (*ExecResult, error) {
	if d.table == "" {
		return nil, fmt.Errorf("deleteBuilder: table not specified")
	}
	if d.conditions == nil {
		return nil, fmt.Errorf(
			"deleteBuilder: WHERE condition required (use Where method or call DeleteAll for unfiltered delete)")
	}
	if err := validateQueryOptions(d.opts); err != nil {
		return nil, fmt.Errorf("deleteBuilder: invalid query options: %w", err)
	}
	result, err := d.db.Delete(d.ctx, d.table, d.joins, d.conditions, d.opts)
	if err != nil {
		return nil, fmt.Errorf("deleteBuilder: failed to delete rows: %w", err)
	}
	return result, nil
}

// DeleteAll executes a DELETE query without a WHERE condition.
// ⚠️ WARNING: This deletes ALL rows in the table. Use with extreme caution.
// Requires explicit method call to prevent accidental data loss.
//
// Returns:
//
//	*ExecResult: The result of the delete operation (RowsAffected).
//	error: An error if the delete fails.
//
// Example:
//
//	// Delete all inactive users
//	result, err := NewFluentDB(db, ctx).
//	    Delete().
//	    From("users").
//	    Where(cdt.NewExpr().Column("status").Op("=").Value("inactive")).
//	    Exec()
//	// For truly unfiltered delete:
//	result, err := NewFluentDB(db, ctx).
//	    Delete().
//	    From("users").
//	    DeleteAll()  // No WHERE clause
func (d *DeleteBuilder) DeleteAll() (*ExecResult, error) {
	if d.table == "" {
		return nil, fmt.Errorf("deleteBuilder: table not specified")
	}
	if err := validateQueryOptions(d.opts); err != nil {
		return nil, fmt.Errorf("deleteBuilder: invalid query options: %w", err)
	}
	result, err := d.db.Delete(d.ctx, d.table, d.joins, d.conditions, d.opts)
	if err != nil {
		return nil, fmt.Errorf("deleteBuilder: failed to delete rows: %w", err)
	}
	return result, nil
}
