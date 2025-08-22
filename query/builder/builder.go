package builder

import (
	"fmt"
	"strings"

	cdt "tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
)

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
		joins []Join,
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

func selectQ(
	dialect cdt.SQLDialect,
	table string,
	columns []string,
	joins []Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *Join) string,
) (string, []any, error) {
	cols := make([]string, len(columns))
	for i, col := range columns {
		cols[i] = dialect.QuoteIdentifier(col)
	}

	sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + dialect.QuoteIdentifier(table)
	if cond == nil {
		return fmt.Sprintf("%s;", sql), nil, nil
	}

	where, values, err := cond.ToSQL(dialect, 1)
	if err != nil {
		return "", nil, fmt.Errorf("builder.selectQ: %w", err)
	}

	join := make([]string, 0, len(joins))
	for _, j := range joins {
		join = append(join, joinFn(table, &j))
	}

	opt := dialect.SupportedOptions(definition.QueryTypeSelect, opts)

	return fmt.Sprintf("%s %s WHERE %s %s;", sql, strings.Join(join, " "), where, opt), values, nil
}

func insert(dialect cdt.SQLDialect, table string, data map[string]any) (string, []any, error) {
	if len(data) == 0 {
		return "", nil, fmt.Errorf("builder.insert: no data provided for insertion")
	}

	index := 1
	columns, placeholders, values := make([]string, 0), make([]string, 0), make([]any, 0)

	for col, val := range data {
		columns = append(columns, dialect.QuoteIdentifier(col))
		placeholders = append(placeholders, dialect.Placeholder(index))
		values = append(values, val)
		index++
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s);",
		dialect.QuoteIdentifier(table),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	), values, nil
}

func update(
	dialect cdt.SQLDialect,
	table string,
	data map[string]any,
	cond cdt.Condition,
) (string, []any, error) {
	index := 1
	sets, values := make([]string, 0), make([]any, 0)

	for col, val := range data {
		sets = append(sets, fmt.Sprintf("%s = %s", dialect.QuoteIdentifier(col), dialect.Placeholder(index)))
		values = append(values, val)
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
