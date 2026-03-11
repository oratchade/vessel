// Package builder provides SQL query building for multiple database engines.
package builder

import (
	"fmt"
	"sort"
	"strings"

	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

//go:generate mockgen -source=builder.go -destination=builder_mocks.go -package=builder QueryBuilder

// QueryBuilder defines methods for building SQL queries, including support for SQL joins.
// Each method returns the SQL string, its arguments, and an error if building fails.
type QueryBuilder interface {
	// Select builds a SELECT query for the given table, columns, joins, and conditions.
	//
	// Parameters:
	//   table: Name of the table to query.
	//   columns: List of columns to select.
	//   joins: Slice of Join structs describing SQL JOIN clauses (e.g., INNER, LEFT).
	//   opts: Optional query parameters (limit, offset, order, etc.).
	//   cond: Query conditions for filtering results.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails.
	Select(
		table string,
		columns []string,
		joins []cdt.Join,
		opts *options.QueryOptions,
		cond cdt.Condition,
	) (string, []any, error)

	// Insert builds an INSERT query for the given table and data.
	//
	// Parameters:
	//   table: Name of the table to insert into.
	//   data: Map of column names to values for the new row.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails.
	Insert(table string, data map[string]any) (string, []any, error)

	// Inserts builds an INSERT query for the given table and multiple rows of data.
	//
	// Parameters:
	//   table: Name of the table to insert into.
	//   data: Slice of maps, each representing a row to insert with column names as keys and values as values.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails.
	Inserts(table string, data []map[string]any) (string, []any, error)

	// Update builds an UPDATE query for the given table, data, and conditions.
	//
	// Parameters:
	//   table: Name of the table to update.
	//   data: Map of column names to new values.
	//   cond: Query conditions to select rows to update.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails.
	Update(table string, data map[string]any, cond cdt.Condition) (string, []any, error)

	// Delete builds a DELETE query for the given table and conditions.
	//
	// Parameters:
	//   table: Name of the table to delete from.
	//   cond: Query conditions to select rows to delete.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails.
	Delete(table string, cond cdt.Condition) (string, []any, error)
}

// selectQ builds a SELECT query with support for columns, joins, conditions, and options.
func selectQ(
	dialect cdt.SQLDialect,
	table string,
	columns []string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *cdt.Join) string,
) (string, []any, error) {
	cols := make([]string, len(columns))
	for i, col := range columns {
		cols[i] = sanitizeColumn(dialect, col)
	}

	sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + dialect.QuoteIdentifier(table)

	join := make([]string, 0, len(joins))
	for _, j := range joins {
		join = append(join, joinFn(table, &j))
	}

	// Build condition fragment (if any)
	var where string
	var values []any
	if cond != nil {
		w, v, err := cond.ToSQL(dialect, 1)
		if err != nil {
			return "", nil, fmt.Errorf("builder.selectQ: %w", err)
		}
		where = w
		values = v
	}

	// compute param base for options: placeholders used so far + starting index (1-based)
	nextParam := 1 + len(values)

	// Ask dialect to render supported options with placeholders starting at nextParam
	optFragment, optArgs, err := dialect.SupportedOptions(definition.QueryTypeSelect, opts, nextParam)
	if err != nil {
		return "", nil, fmt.Errorf("builder.selectQ: %w", err)
	}

	// assemble SQL parts
	var b strings.Builder
	b.WriteString(sql)

	if len(join) > 0 {
		b.WriteString(" ")
		b.WriteString(strings.Join(join, " "))
	}

	if where != "" {
		b.WriteString(" WHERE ")
		b.WriteString(where)
	}

	if optFragment != "" {
		b.WriteString(" ")
		b.WriteString(optFragment)
	}

	b.WriteString(";")

	// merge args: condition args first, then options args
	allArgs := make([]any, 0, len(values)+len(optArgs))
	allArgs = append(allArgs, values...)
	allArgs = append(allArgs, optArgs...)

	return b.String(), allArgs, nil
}

// sanitizeColumn quotes and processes a column identifier, handling aliases and qualified names.
func sanitizeColumn(dialect cdt.SQLDialect, column string) string {
	c, alias := column, ""
	if column == "*" {
		return "*"
	}

	if strings.Contains(column, dialect.Operator("AS")) {
		p := strings.SplitN(column, dialect.Operator("AS"), 2)
		c, alias = p[0], p[1]
	}
	if alias != "" {
		alias = " " + dialect.Operator("AS") + " " + dialect.QuoteIdentifier(strings.TrimSpace(alias))
	}
	if strings.Contains(c, ".") {
		parts := []string{}
		columns := strings.Split(c, ".")
		for _, p := range columns {
			parts = append(parts, dialect.QuoteIdentifier(strings.TrimSpace(p)))
		}

		return fmt.Sprintf("%s%s", strings.Join(parts, "."), alias)
	}
	return fmt.Sprintf("%s%s", dialect.QuoteIdentifier(strings.TrimSpace(c)), alias)
}

// insert builds an INSERT query for the given table and data.
func insert(dialect cdt.SQLDialect, table string, data map[string]any) (string, []any, error) {
	if len(data) == 0 {
		return "", nil, fmt.Errorf("builder.insert: no data provided for insertion")
	}

	// Extract and sort column names for deterministic ordering
	columns := make([]string, 0, len(data))
	for col := range data {
		columns = append(columns, col)
	}
	sort.Strings(columns)

	index := 1
	placeholders, values := make([]string, 0), make([]any, 0)

	for _, col := range columns {
		placeholders = append(placeholders, dialect.Placeholder(index))
		values = append(values, data[col])
		index++
	}

	// Quote column names
	var quotedColumns []string
	for _, col := range columns {
		quotedColumns = append(quotedColumns, dialect.QuoteIdentifier(col))
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s);",
		dialect.QuoteIdentifier(table),
		strings.Join(quotedColumns, ", "),
		strings.Join(placeholders, ", "),
	), values, nil
}

// insert builds an INSERT query for the given table and multiple rows of data.
func inserts(dialect cdt.SQLDialect, table string, data []map[string]any) (string, []any, error) {
	if len(data) == 0 {
		return "", nil, fmt.Errorf("builder.inserts: no data provided for insertion")
	}

	// Get column names from the first row and sort them for deterministic ordering
	columns := make([]string, 0)
	for col := range data[0] {
		columns = append(columns, col)
	}
	sort.Strings(columns)

	// Build placeholders for each row
	index := 1
	var rowPlaceholders []string
	var values []any

	for _, row := range data {
		i, rowValues, rowVals := rowValues(dialect, row, columns, index)
		rowPlaceholders = append(rowPlaceholders, fmt.Sprintf("(%s)", strings.Join(rowValues, ", ")))
		values = append(values, rowVals...)
		index = i
	}

	// Quote column names
	var quotedColumns []string
	for _, col := range columns {
		quotedColumns = append(quotedColumns, dialect.QuoteIdentifier(col))
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s;",
		dialect.QuoteIdentifier(table),
		strings.Join(quotedColumns, ", "),
		strings.Join(rowPlaceholders, ", "),
	), values, nil
}

func rowValues(dialect cdt.SQLDialect, row map[string]any, columns []string, index int) (int, []string, []any) {
	var rowValues []string
	var values []any
	for _, col := range columns {
		if val, ok := row[col]; ok {
			rowValues = append(rowValues, dialect.Placeholder(index))
			values = append(values, val)
			index++
		} else {
			rowValues = append(rowValues, "NULL")
		}
	}

	return index, rowValues, values
}

// update builds an UPDATE query for the given table, data, and conditions.
//
//nolint:prealloc
func update(
	dialect cdt.SQLDialect,
	table string,
	data map[string]any,
	cond cdt.Condition,
) (string, []any, error) {
	// Extract and sort column names for deterministic ordering
	columns := make([]string, 0, len(data))
	for col := range data {
		columns = append(columns, col)
	}
	sort.Strings(columns)

	index := 1
	sets, values := make([]string, 0), make([]any, 0)

	for _, col := range columns {
		sets = append(sets, fmt.Sprintf("%s = %s", dialect.QuoteIdentifier(col), dialect.Placeholder(index)))
		values = append(values, data[col])
		index++
	}

	sql := fmt.Sprintf("UPDATE %s SET %s", dialect.QuoteIdentifier(table), strings.Join(sets, ", "))
	if cond == nil {
		return fmt.Sprintf("%s;", sql), values, nil
	}

	where, condValues, err := cond.ToSQL(dialect, index)
	if err != nil {
		return "", nil, fmt.Errorf("builder.update: %w", err)
	}
	values = append(values, condValues...)

	return fmt.Sprintf("%s WHERE %s;", sql, where), values, nil
}

// delete builds a DELETE query for the given table and conditions.
func delete(dialect cdt.SQLDialect, table string, cond cdt.Condition) (string, []any, error) {
	if cond == nil {
		return fmt.Sprintf("DELETE FROM %s;", dialect.QuoteIdentifier(table)), nil, nil
	}

	where, values, err := cond.ToSQL(dialect, 1)
	if err != nil {
		return "", nil, fmt.Errorf("builder.delete: %w", err)
	}

	return fmt.Sprintf("DELETE FROM %s WHERE %s;", dialect.QuoteIdentifier(table), where), values, nil
}
