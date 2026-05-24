// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"context"
	"fmt"
	"strings"

	"tounilab.com/fabric/internal/pkg/helpers"
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

// validateQueryOptions checks query options for validity and returns an error if invalid.
// Returns nil if all options are valid.
// This is exported for testing validation logic independently.
func validateQueryOptions(opts *options.QueryOptions) error {
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

func ensureQueryOptions(opts **options.QueryOptions) {
	if *opts == nil {
		*opts = &options.QueryOptions{}
	}
}

// dbActions defines the interface required by FluentDB to execute all types of database operations.
// It combines read, write, and query building capabilities.
type dbActions interface {
	reader
	writer
	upserter
	introspector
}

// FluentDB provides a fluent/builder interface for constructing and executing database queries.
// It acts as an entry point for building SELECT, INSERT, UPDATE, and DELETE operations
// with a chainable, ergonomic API while reusing the database operation interfaces.
type FluentDB struct {
	db dbActions
}

// NewFluentDB creates a new FluentDB instance that wraps the provided DB interface.
//
// Parameters:
//
//	db: The underlying database interface to execute queries.
//
// Returns:
//
//	*FluentDB: A new fluent database builder.
//
// Example:
//
//	result, err := NewFluentDB(db).
//	    Select("users", "id", "name", "email").
//	    Where(cdt.NewExpr().Column("age").Op(">").Value(18)).
//	    Get()
func NewFluentDB(db interface {
	reader
	writer
	upserter
	introspector
},
) *FluentDB {
	return &FluentDB{db: db.(dbActions)}
}

// Select begins a SELECT query by specifying the table and columns.
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
//	NewFluentDB(db).
//	    Select("users", "id", "name").
//	    Where(...).
//	    Get()
func (f *FluentDB) Select(table string, columns ...string) *SelectBuilder {
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	return &SelectBuilder{
		db:      f.db,
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
		db: f.db,
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
		db: f.db,
	}
}

// SelectBuilder is a fluent builder for SELECT queries.
// It allows chainable method calls to construct complex queries with
// joins, conditions, ordering, and pagination.
type SelectBuilder struct {
	db         dbActions
	table      string
	columns    []string
	joins      []cdt.Join
	conditions cdt.Condition
	opts       *options.QueryOptions
}

// WithTx specifies a transaction to execute this query within.
// If a transaction is provided, the query will use the transaction's database operations
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

// JoinOn adds a JOIN with a condition-based ON clause.
//
// The ON condition is rendered by Fabric. Conditions with values are rejected
// by the current join builder; use this for column predicates and unary checks
// such as condition.IsNull("tu.deleted_at").
func (s *SelectBuilder) JoinOn(joinType, table, alias string, on cdt.Condition) *SelectBuilder {
	return s.Join(cdt.Join{Type: joinType, Table: table, Alias: alias, On: on})
}

// LeftJoinOn adds a LEFT JOIN with a condition-based ON clause.
func (s *SelectBuilder) LeftJoinOn(table, alias string, on cdt.Condition) *SelectBuilder {
	return s.JoinOn("LEFT", table, alias, on)
}

// InnerJoinOn adds an INNER JOIN with a condition-based ON clause.
func (s *SelectBuilder) InnerJoinOn(table, alias string, on cdt.Condition) *SelectBuilder {
	return s.JoinOn("INNER", table, alias, on)
}

// Column appends a safely quoted projection column.
func (s *SelectBuilder) Column(column string) *SelectBuilder {
	if column == "" {
		return s
	}
	s.appendColumn(column)
	return s
}

// ColumnAs appends a safely quoted projection column with an alias.
func (s *SelectBuilder) ColumnAs(column, alias string) *SelectBuilder {
	if column == "" {
		return s
	}
	if alias == "" {
		return s.Column(column)
	}
	s.appendColumn(column + " AS " + alias)
	return s
}

// ColumnRaw appends a trusted raw projection fragment.
//
// The SQL fragment is caller-owned and is not quoted or parameterized. Only pass
// trusted, allowlisted SQL syntax here; values should still be supplied through
// parameterized conditions where possible.
func (s *SelectBuilder) ColumnRaw(sql string) *SelectBuilder {
	if sql == "" {
		return s
	}
	s.appendColumn(helpers.RawProjection(sql))
	return s
}

// ColumnRawAs appends a trusted raw projection fragment with a safely quoted alias.
func (s *SelectBuilder) ColumnRawAs(sql, alias string) *SelectBuilder {
	if sql == "" {
		return s
	}
	if alias == "" {
		return s.ColumnRaw(sql)
	}
	s.appendColumn(helpers.RawProjection(sql + " AS " + alias))
	return s
}

func (s *SelectBuilder) appendColumn(column string) {
	if len(s.columns) == 1 && s.columns[0] == "*" {
		s.columns = []string{column}
		return
	}
	s.columns = append(s.columns, column)
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
	ensureQueryOptions(&s.opts)
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

// OrderByAsc adds an ascending ORDER BY clause to the SELECT query.
func (s *SelectBuilder) OrderByAsc(column string) *SelectBuilder {
	return s.OrderBy(column, AscDirection)
}

// OrderByDesc adds a descending ORDER BY clause to the SELECT query.
func (s *SelectBuilder) OrderByDesc(column string) *SelectBuilder {
	return s.OrderBy(column, DescDirection)
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
	ensureQueryOptions(&s.opts)
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
	ensureQueryOptions(&s.opts)
	s.opts.Offset = &offset
	return s
}

// GroupBy adds one or more GROUP BY columns to the SELECT query.
func (s *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	if len(columns) == 0 {
		return s
	}
	ensureQueryOptions(&s.opts)
	s.opts.GroupBy = append(s.opts.GroupBy, columns...)
	return s
}

// HavingRaw sets a raw HAVING clause for the SELECT query.
//
// The SQL fragment is caller-owned and is not quoted or parameterized. Only pass
// trusted, allowlisted SQL syntax here; values should still be supplied through
// parameterized conditions where possible.
func (s *SelectBuilder) HavingRaw(sql string) *SelectBuilder {
	ensureQueryOptions(&s.opts)
	s.opts.Having = &sql
	return s
}

// Having adds a parameterized HAVING condition for the SELECT query.
func (s *SelectBuilder) Having(cond cdt.Condition) *SelectBuilder {
	if cond == nil {
		return s
	}
	ensureQueryOptions(&s.opts)
	if s.opts.HavingCondition == nil {
		s.opts.HavingCondition = cond
	} else {
		s.opts.HavingCondition = cdt.NewAnd().Conditions(s.opts.HavingCondition, cond)
	}
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
func (s *SelectBuilder) Get(ctx context.Context) ([]map[string]any, error) {
	if s.table == "" {
		return nil, fmt.Errorf("SelectBuilder.Get: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return nil, fmt.Errorf("SelectBuilder.Get: invalid query options: %w", err)
	}
	rows, err := s.db.Get(ctx, s.table, s.columns, s.joins, s.conditions, s.opts)
	if err != nil {
		return nil, fmt.Errorf("SelectBuilder.Get: failed to get rows: %w", err)
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
func (s *SelectBuilder) GetRaw(ctx context.Context) (*RowsAdapter, error) {
	if s.table == "" {
		return nil, fmt.Errorf("SelectBuilder.GetRaw: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return nil, fmt.Errorf("SelectBuilder.GetRaw: invalid query options: %w", err)
	}
	rows, err := s.db.GetRaw(ctx, s.table, s.columns, s.joins, s.conditions, s.opts)
	if err != nil {
		return nil, fmt.Errorf("SelectBuilder.GetRaw: failed to get raw rows: %w", err)
	}
	return rows, nil
}

// SelectQuery returns the generated SELECT SQL and arguments without executing it.
func (s *SelectBuilder) SelectQuery() (string, []any, error) {
	if s.table == "" {
		return "", nil, fmt.Errorf("SelectBuilder.SelectQuery: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return "", nil, fmt.Errorf("SelectBuilder.SelectQuery: invalid query options: %w", err)
	}
	query, args, err := s.db.GetQuery(s.table, s.columns, s.joins, s.conditions, s.opts)
	if err != nil {
		return "", nil, fmt.Errorf("SelectBuilder.SelectQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

// Query returns the generated SELECT SQL and arguments without executing it.
func (s *SelectBuilder) Query() (string, []any, error) {
	return s.SelectQuery()
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
//	    One(ctx)
func (s *SelectBuilder) One(ctx context.Context) (map[string]any, error) {
	if s.table == "" {
		return nil, fmt.Errorf("SelectBuilder.One: table not specified")
	}
	if err := validateQueryOptions(s.opts); err != nil {
		return nil, fmt.Errorf("SelectBuilder.One: invalid query options: %w", err)
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

	rows, err := s.db.Get(ctx, s.table, s.columns, s.joins, s.conditions, opts)
	if err != nil {
		return nil, fmt.Errorf("SelectBuilder.One: failed to get row: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("SelectBuilder.One: no rows found")
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
//	    Count(ctx)
func (s *SelectBuilder) Count(ctx context.Context) (int64, error) {
	if s.table == "" {
		return 0, fmt.Errorf("SelectBuilder.Count: table not specified")
	}

	rows, err := s.db.Get(
		ctx, s.table, []string{helpers.RawProjection("COUNT(*) AS count")}, s.joins, s.conditions, nil,
	)
	if err != nil {
		return 0, fmt.Errorf("SelectBuilder.Count: failed to count rows: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	countVal, exists := rows[0]["count"]
	if !exists {
		return 0, fmt.Errorf("SelectBuilder.Count: count query did not return a count value")
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
		return 0, fmt.Errorf("SelectBuilder.Count: unexpected count type: %T", countVal)
	}
}

// CountRaw executes a COUNT(*) query and returns a RowsAdapter.
func (s *SelectBuilder) CountRaw(ctx context.Context, alias ...string) (*RowsAdapter, error) {
	if s.table == "" {
		return nil, fmt.Errorf("SelectBuilder.CountRaw: table not specified")
	}
	rows, err := s.db.GetRaw(ctx, s.table, []string{countProjection(alias...)}, s.joins, s.conditions, nil)
	if err != nil {
		return nil, fmt.Errorf("SelectBuilder.CountRaw: failed to count rows: %w", err)
	}
	return rows, nil
}

// CountQuery returns the generated COUNT(*) SQL and arguments without executing it.
func (s *SelectBuilder) CountQuery(alias ...string) (string, []any, error) {
	if s.table == "" {
		return "", nil, fmt.Errorf("SelectBuilder.CountQuery: table not specified")
	}
	query, args, err := s.db.GetQuery(s.table, []string{countProjection(alias...)}, s.joins, s.conditions, nil)
	if err != nil {
		return "", nil, fmt.Errorf("SelectBuilder.CountQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

func countProjection(alias ...string) string {
	name := "cnt"
	if len(alias) > 0 && strings.TrimSpace(alias[0]) != "" {
		name = strings.TrimSpace(alias[0])
	}
	return helpers.RawProjection("COUNT(*) AS " + name)
}

// InsertBuilder is a fluent builder for INSERT queries.
// It allows specification of the table and values to insert, either as
// single or bulk operations.
type InsertBuilder struct {
	db         dbActions
	table      string
	data       map[string]any
	bulk       []map[string]any
	opts       *options.QueryOptions
	upsertOpts *options.UpsertOptions
}

// WithTx specifies a transaction to execute this query within.
// If a transaction is provided, the query will use the transaction's database operations
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

// Returning requests mutation RETURNING/OUTPUT columns in query preview.
//
// Returning is supported for query preview on PostgreSQL and MSSQL. Mutation
// execution methods reject Returning because they return ExecResult, not rows.
// MySQL and SQLite ignore Returning in generated preview SQL.
func (i *InsertBuilder) Returning(columns ...string) *InsertBuilder {
	if len(columns) == 0 {
		return i
	}
	ensureQueryOptions(&i.opts)
	i.opts.Returning = append(i.opts.Returning, columns...)
	return i
}

// OnConflict configures the uniqueness target for a portable upsert.
//
// PostgreSQL and SQLite render ON CONFLICT with these columns. MySQL uses the
// table's duplicate-key constraints but still accepts the columns so Fabric can
// choose default update columns and no-op update behavior.
func (i *InsertBuilder) OnConflict(columns ...string) *InsertBuilder {
	if i.upsertOpts == nil {
		i.upsertOpts = &options.UpsertOptions{}
	}
	i.upsertOpts.ConflictColumns = append([]string(nil), columns...)
	return i
}

// DoNothing makes the upsert skip conflicting rows.
func (i *InsertBuilder) DoNothing() *InsertBuilder {
	if i.upsertOpts == nil {
		i.upsertOpts = &options.UpsertOptions{}
	}
	i.upsertOpts.Action = options.UpsertDoNothing
	return i
}

// DoUpdate makes the upsert copy inserted column values into the conflicting row.
// If columns are omitted, all inserted columns except conflict columns are updated.
func (i *InsertBuilder) DoUpdate(columns ...string) *InsertBuilder {
	if i.upsertOpts == nil {
		i.upsertOpts = &options.UpsertOptions{}
	}
	i.upsertOpts.Action = options.UpsertDoUpdate
	i.upsertOpts.UpdateColumns = append([]string(nil), columns...)
	return i
}

// DoUpdateSet makes the upsert set explicit values on conflict.
func (i *InsertBuilder) DoUpdateSet(data map[string]any) *InsertBuilder {
	if i.upsertOpts == nil {
		i.upsertOpts = &options.UpsertOptions{}
	}
	i.upsertOpts.Action = options.UpsertDoUpdate
	i.upsertOpts.UpdateValues = data
	return i
}

// InsertQuery returns the generated single-row INSERT SQL and arguments without executing it.
func (i *InsertBuilder) InsertQuery() (string, []any, error) {
	if i.table == "" {
		return "", nil, fmt.Errorf("InsertBuilder.InsertQuery: table not specified")
	}
	if len(i.data) == 0 {
		return "", nil, fmt.Errorf("InsertBuilder.InsertQuery: no data provided")
	}
	if err := validateQueryOptions(i.opts); err != nil {
		return "", nil, fmt.Errorf("InsertBuilder.InsertQuery: invalid query options: %w", err)
	}
	query, args, err := i.db.InsertQuery(i.table, i.data, i.opts)
	if err != nil {
		return "", nil, fmt.Errorf("InsertBuilder.InsertQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

// InsertsQuery returns the generated bulk INSERT SQL and arguments without executing it.
func (i *InsertBuilder) InsertsQuery() (string, []any, error) {
	if i.table == "" {
		return "", nil, fmt.Errorf("InsertBuilder.InsertsQuery: table not specified")
	}
	if len(i.bulk) == 0 {
		return "", nil, fmt.Errorf("InsertBuilder.InsertsQuery: no bulk data provided")
	}
	if err := validateQueryOptions(i.opts); err != nil {
		return "", nil, fmt.Errorf("InsertBuilder.InsertsQuery: invalid query options: %w", err)
	}
	query, args, err := i.db.InsertsQuery(i.table, i.bulk, i.opts)
	if err != nil {
		return "", nil, fmt.Errorf("InsertBuilder.InsertsQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

// UpsertQuery returns the generated UPSERT SQL and arguments without executing it.
func (i *InsertBuilder) UpsertQuery() (string, []any, error) {
	if i.table == "" {
		return "", nil, fmt.Errorf("InsertBuilder.UpsertQuery: table not specified")
	}
	if len(i.bulk) > 0 {
		return "", nil, fmt.Errorf("InsertBuilder.UpsertQuery: bulk upserts are not supported")
	}
	if len(i.data) == 0 {
		return "", nil, fmt.Errorf("InsertBuilder.UpsertQuery: no data provided")
	}
	if i.upsertOpts == nil {
		return "", nil, fmt.Errorf("InsertBuilder.UpsertQuery: upsert options not specified")
	}
	if err := validateQueryOptions(i.opts); err != nil {
		return "", nil, fmt.Errorf("InsertBuilder.UpsertQuery: invalid query options: %w", err)
	}
	query, args, err := i.db.UpsertQuery(i.table, i.data, i.upsertOpts, i.opts)
	if err != nil {
		return "", nil, fmt.Errorf("InsertBuilder.UpsertQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

// Query returns the generated INSERT SQL and arguments without executing it.
// It uses bulk insert SQL when ValuesBulk was called, otherwise single insert SQL.
func (i *InsertBuilder) Query() (string, []any, error) {
	if i.upsertOpts != nil {
		return i.UpsertQuery()
	}
	if len(i.bulk) > 0 {
		return i.InsertsQuery()
	}
	return i.InsertQuery()
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
//	    Exec(ctx)
func (i *InsertBuilder) Exec(ctx context.Context) (*ExecResult, error) {
	if i.table == "" {
		return nil, fmt.Errorf("InsertBuilder.Exec: table not specified")
	}

	if i.upsertOpts != nil {
		result, err := i.Upsert(ctx)
		if err != nil {
			return nil, fmt.Errorf("InsertBuilder.Exec: failed to upsert data: %w", err)
		}
		return result, nil
	}

	// Use bulk insert if data was provided via ValuesBulk
	if len(i.bulk) > 0 {
		result, err := i.db.Inserts(ctx, i.table, i.bulk, i.opts)
		if err != nil {
			return nil, fmt.Errorf("InsertBuilder.Exec: failed to insert bulk data: %w", err)
		}
		return result, nil
	}

	// Use single insert
	if len(i.data) == 0 {
		return nil, fmt.Errorf("InsertBuilder.Exec: no data provided")
	}
	result, err := i.db.Insert(ctx, i.table, i.data, i.opts)
	if err != nil {
		return nil, fmt.Errorf("InsertBuilder.Exec: failed to insert data: %w", err)
	}
	return result, nil
}

// Upsert executes the configured single-row upsert.
func (i *InsertBuilder) Upsert(ctx context.Context) (*ExecResult, error) {
	if i.table == "" {
		return nil, fmt.Errorf("InsertBuilder.Upsert: table not specified")
	}
	if len(i.bulk) > 0 {
		return nil, fmt.Errorf("InsertBuilder.Upsert: bulk upserts are not supported")
	}
	if len(i.data) == 0 {
		return nil, fmt.Errorf("InsertBuilder.Upsert: no data provided")
	}
	if i.upsertOpts == nil {
		return nil, fmt.Errorf("InsertBuilder.Upsert: upsert options not specified")
	}
	result, err := i.db.Upsert(ctx, i.table, i.data, i.upsertOpts, i.opts)
	if err != nil {
		return nil, fmt.Errorf("InsertBuilder.Upsert: failed to execute upsert: %w", err)
	}
	return result, nil
}

// InsertAndFetch inserts one row and fetches it by an application-provided key.
//
// The insert and follow-up select are separate statements unless this builder is
// bound to a transaction with WithTx. For atomic create-and-fetch workflows,
// call InsertAndFetch inside WithTransaction and pass that transaction via WithTx.
func (i *InsertBuilder) InsertAndFetch(
	ctx context.Context,
	keyColumn string,
	columns ...string,
) (map[string]any, error) {
	keyValue, err := i.validateInsertAndFetch(keyColumn)
	if err != nil {
		return nil, err
	}
	if _, err := i.Exec(ctx); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	rows, err := i.db.Get(ctx, i.table, columns, nil, cdt.NewExpr().Column(keyColumn).Op("=").Value(keyValue), nil)
	if err != nil {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: failed to fetch inserted row: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: inserted row was not found")
	}
	return rows[0], nil
}

func (i *InsertBuilder) validateInsertAndFetch(keyColumn string) (any, error) {
	if i.table == "" {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: table not specified")
	}
	if keyColumn == "" {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: key column not specified")
	}
	if len(i.bulk) > 0 {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: bulk inserts are not supported")
	}
	if len(i.data) == 0 {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: no data provided")
	}
	if i.upsertOpts != nil {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: upsert is not supported")
	}
	if i.opts != nil && len(i.opts.Returning) > 0 {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: Returning is not supported; fetch by key instead")
	}
	keyValue, ok := i.data[keyColumn]
	if !ok {
		return nil, fmt.Errorf("InsertBuilder.InsertAndFetch: key column %q missing from insert data", keyColumn)
	}
	return keyValue, nil
}

// UpdateBuilder is a fluent builder for UPDATE queries.
// It allows specification of the table, values to update, conditions to filter rows,
// and optional joins for complex updates.
type UpdateBuilder struct {
	db         dbActions
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
	ensureQueryOptions(&u.opts)
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

// OrderByAsc adds an ascending ORDER BY clause to the UPDATE query.
func (u *UpdateBuilder) OrderByAsc(column string) *UpdateBuilder {
	return u.OrderBy(column, AscDirection)
}

// OrderByDesc adds a descending ORDER BY clause to the UPDATE query.
func (u *UpdateBuilder) OrderByDesc(column string) *UpdateBuilder {
	return u.OrderBy(column, DescDirection)
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
	ensureQueryOptions(&u.opts)
	u.opts.Limit = &limit
	return u
}

// Returning requests mutation RETURNING/OUTPUT columns in query preview.
//
// Returning is supported for query preview on PostgreSQL and MSSQL. Mutation
// execution methods reject Returning because they return ExecResult, not rows.
// MySQL and SQLite ignore Returning in generated preview SQL.
func (u *UpdateBuilder) Returning(columns ...string) *UpdateBuilder {
	if len(columns) == 0 {
		return u
	}
	ensureQueryOptions(&u.opts)
	u.opts.Returning = append(u.opts.Returning, columns...)
	return u
}

// UpdateQuery returns the generated UPDATE SQL and arguments without executing it.
func (u *UpdateBuilder) UpdateQuery() (string, []any, error) {
	if u.table == "" {
		return "", nil, fmt.Errorf("UpdateBuilder.UpdateQuery: table not specified")
	}
	if len(u.data) == 0 {
		return "", nil, fmt.Errorf("UpdateBuilder.UpdateQuery: no data to update")
	}
	if err := validateQueryOptions(u.opts); err != nil {
		return "", nil, fmt.Errorf("UpdateBuilder.UpdateQuery: invalid query options: %w", err)
	}
	query, args, err := u.db.UpdateQuery(u.table, u.data, u.joins, u.conditions, u.opts)
	if err != nil {
		return "", nil, fmt.Errorf("UpdateBuilder.UpdateQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

// Query returns the generated UPDATE SQL and arguments without executing it.
func (u *UpdateBuilder) Query() (string, []any, error) {
	return u.UpdateQuery()
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
//	    Exec(ctx)
func (u *UpdateBuilder) Exec(ctx context.Context) (*ExecResult, error) {
	if u.table == "" {
		return nil, fmt.Errorf("UpdateBuilder.Exec: table not specified")
	}
	if len(u.data) == 0 {
		return nil, fmt.Errorf("UpdateBuilder.Exec: no data to update")
	}
	if u.conditions == nil {
		return nil, fmt.Errorf(
			"UpdateBuilder.Exec: WHERE condition required (use Where method or call UpdateAll for unfiltered update)")
	}
	if err := validateQueryOptions(u.opts); err != nil {
		return nil, fmt.Errorf("UpdateBuilder.Exec: invalid query options: %w", err)
	}
	result, err := u.db.Update(ctx, u.table, u.data, u.joins, u.conditions, u.opts)
	if err != nil {
		return nil, fmt.Errorf("UpdateBuilder.Exec: failed to update rows: %w", err)
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
//	    UpdateAll(ctx)  // No WHERE clause
func (u *UpdateBuilder) UpdateAll(ctx context.Context) (*ExecResult, error) {
	if u.table == "" {
		return nil, fmt.Errorf("UpdateBuilder.UpdateAll: table not specified")
	}
	if err := validateQueryOptions(u.opts); err != nil {
		return nil, fmt.Errorf("UpdateBuilder.UpdateAll: invalid query options: %w", err)
	}
	if len(u.data) == 0 {
		return nil, fmt.Errorf("UpdateBuilder.UpdateAll: no data to update")
	}
	result, err := u.db.Update(ctx, u.table, u.data, u.joins, u.conditions, u.opts)
	if err != nil {
		return nil, fmt.Errorf("UpdateBuilder.UpdateAll: failed to update rows: %w", err)
	}
	return result, nil
}

// DeleteBuilder is a fluent builder for DELETE queries.
// It allows specification of the table, conditions to filter rows,
// and optional joins for complex deletes.
type DeleteBuilder struct {
	db         dbActions
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
	ensureQueryOptions(&d.opts)
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

// OrderByAsc adds an ascending ORDER BY clause to the DELETE query.
func (d *DeleteBuilder) OrderByAsc(column string) *DeleteBuilder {
	return d.OrderBy(column, AscDirection)
}

// OrderByDesc adds a descending ORDER BY clause to the DELETE query.
func (d *DeleteBuilder) OrderByDesc(column string) *DeleteBuilder {
	return d.OrderBy(column, DescDirection)
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
	ensureQueryOptions(&d.opts)
	d.opts.Limit = &limit
	return d
}

// Returning requests mutation RETURNING/OUTPUT columns in query preview.
//
// Returning is supported for query preview on PostgreSQL and MSSQL. Mutation
// execution methods reject Returning because they return ExecResult, not rows.
// MySQL and SQLite ignore Returning in generated preview SQL.
func (d *DeleteBuilder) Returning(columns ...string) *DeleteBuilder {
	if len(columns) == 0 {
		return d
	}
	ensureQueryOptions(&d.opts)
	d.opts.Returning = append(d.opts.Returning, columns...)
	return d
}

// DeleteQuery returns the generated DELETE SQL and arguments without executing it.
func (d *DeleteBuilder) DeleteQuery() (string, []any, error) {
	if d.table == "" {
		return "", nil, fmt.Errorf("DeleteBuilder.DeleteQuery: table not specified")
	}
	if err := validateQueryOptions(d.opts); err != nil {
		return "", nil, fmt.Errorf("DeleteBuilder.DeleteQuery: invalid query options: %w", err)
	}
	query, args, err := d.db.DeleteQuery(d.table, d.joins, d.conditions, d.opts)
	if err != nil {
		return "", nil, fmt.Errorf("DeleteBuilder.DeleteQuery: failed to build query: %w", err)
	}
	return query, args, nil
}

// Query returns the generated DELETE SQL and arguments without executing it.
func (d *DeleteBuilder) Query() (string, []any, error) {
	return d.DeleteQuery()
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
//	    Exec(ctx)
func (d *DeleteBuilder) Exec(ctx context.Context) (*ExecResult, error) {
	if d.table == "" {
		return nil, fmt.Errorf("DeleteBuilder.Exec: table not specified")
	}
	if d.conditions == nil {
		return nil, fmt.Errorf(
			"DeleteBuilder.Exec: WHERE condition required (use Where method or call DeleteAll for unfiltered delete)")
	}
	if err := validateQueryOptions(d.opts); err != nil {
		return nil, fmt.Errorf("DeleteBuilder.Exec: invalid query options: %w", err)
	}
	result, err := d.db.Delete(ctx, d.table, d.joins, d.conditions, d.opts)
	if err != nil {
		return nil, fmt.Errorf("DeleteBuilder.Exec: failed to delete rows: %w", err)
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
//	    Exec(ctx)
//	// For truly unfiltered delete:
//	result, err := NewFluentDB(db, ctx).
//	    Delete().
//	    From("users").
//	    DeleteAll(ctx)  // No WHERE clause
func (d *DeleteBuilder) DeleteAll(ctx context.Context) (*ExecResult, error) {
	if d.table == "" {
		return nil, fmt.Errorf("DeleteBuilder.DeleteAll: table not specified")
	}
	if err := validateQueryOptions(d.opts); err != nil {
		return nil, fmt.Errorf("DeleteBuilder.DeleteAll: invalid query options: %w", err)
	}
	result, err := d.db.Delete(ctx, d.table, d.joins, d.conditions, d.opts)
	if err != nil {
		return nil, fmt.Errorf("DeleteBuilder.DeleteAll: failed to delete rows: %w", err)
	}
	return result, nil
}
