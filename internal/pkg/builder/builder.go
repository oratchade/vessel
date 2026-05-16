// Package builder provides SQL query building for multiple database engines.
package builder

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

var sqlFunctionPattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*)\s*\(.*\)$`)

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
	Insert(table string, data map[string]any, opts *options.QueryOptions) (string, []any, error)

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
	Inserts(table string, data []map[string]any, opts *options.QueryOptions) (string, []any, error)

	// Update builds an UPDATE query for the given table, data, joins, and conditions.
	//
	// Parameters:
	//   table: Name of the table to update.
	//   data: Map of column names to new values.
	//   joins: Slice of Join structs describing SQL JOIN clauses (optional, may be nil or empty).
	//   cond: Query conditions to select rows to update.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails.
	//
	// Note: JOIN support varies by database driver. SQLite UPDATE with JOINs is supported,
	// but not all complex join patterns may be supported across all drivers.
	Update(
		table string,
		data map[string]any,
		joins []cdt.Join,
		cond cdt.Condition,
		opts *options.QueryOptions,
	) (string, []any, error)

	// Delete builds a DELETE query for the given table, joins, and conditions.
	//
	// Parameters:
	//   table: Name of the table to delete from.
	//   joins: Slice of Join structs describing SQL JOIN clauses (optional, may be nil or empty).
	//          Note: SQLite does not support DELETE with JOINs and will return an error if joins are provided.
	//   cond: Query conditions to select rows to delete.
	//
	// Returns:
	//   string: The generated SQL query.
	//   []any: Arguments for parameterized query.
	//   error: Error if query building fails, or if DELETE with JOINs is attempted on unsupported databases.
	Delete(table string, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) (string, []any, error)
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

	asPattern := regexp.MustCompile(`(?i)\s+` + regexp.QuoteMeta(dialect.Operator("AS")) + `\s+`)
	if loc := asPattern.FindStringIndex(column); loc != nil {
		c, alias = column[:loc[0]], column[loc[1]:]
	}
	if alias != "" {
		alias = " " + dialect.Operator("AS") + " " + dialect.QuoteIdentifier(strings.TrimSpace(alias))
	}
	c = strings.TrimSpace(c)
	if sqlFunctionPattern.MatchString(c) {
		return fmt.Sprintf("%s%s", c, alias)
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
func insert(
	dialect cdt.SQLDialect,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
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

	outputFragment, _, err := dialect.SupportedOptions(definition.QueryTypeInsert, opts, index)
	if err != nil {
		return "", nil, fmt.Errorf("builder.insert: %w", err)
	}

	outputPrefix, outputSuffix := "", ""
	if outputFragment != "" && isMSSQLDialect(dialect) {
		outputPrefix = " " + outputFragment
	} else if outputFragment != "" {
		outputSuffix = " " + outputFragment
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s)%s VALUES (%s)%s;",
		dialect.QuoteIdentifier(table),
		strings.Join(quotedColumns, ", "),
		outputPrefix,
		strings.Join(placeholders, ", "),
		outputSuffix,
	), values, nil
}

// insert builds an INSERT query for the given table and multiple rows of data.
func inserts(
	dialect cdt.SQLDialect,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
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

	outputFragment, _, err := dialect.SupportedOptions(definition.QueryTypeInsert, opts, index)
	if err != nil {
		return "", nil, fmt.Errorf("builder.inserts: %w", err)
	}

	outputPrefix, outputSuffix := "", ""
	if outputFragment != "" && isMSSQLDialect(dialect) {
		outputPrefix = " " + outputFragment
	} else if outputFragment != "" {
		outputSuffix = " " + outputFragment
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s)%s VALUES %s%s;",
		dialect.QuoteIdentifier(table),
		strings.Join(quotedColumns, ", "),
		outputPrefix,
		strings.Join(rowPlaceholders, ", "),
		outputSuffix,
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

// update builds an UPDATE query for the given table, data, joins, and conditions.
// The joins parameter is optional and may be nil or empty.
//
//nolint:prealloc,cyclop
func update(
	dialect cdt.SQLDialect,
	table string,
	data map[string]any,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *cdt.Join) string,
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

	sql := fmt.Sprintf("UPDATE %s", dialect.QuoteIdentifier(table))

	// Build JOIN clauses if provided
	var joinParts []string
	if len(joins) > 0 {
		for _, j := range joins {
			joinParts = append(joinParts, joinFn(table, &j))
		}
	}

	outputFragment, _, err := dialect.SupportedOptions(definition.QueryTypeUpdate, opts, index)
	if err != nil {
		return "", nil, fmt.Errorf("builder.update: %w", err)
	}

	// Build the WHERE clause and its arguments
	var whereClause string
	if cond != nil {
		where, condValues, err := cond.ToSQL(dialect, index)
		if err != nil {
			return "", nil, fmt.Errorf("builder.update: %w", err)
		}
		whereClause = where
		values = append(values, condValues...)
	}

	// Assemble the final query
	var b strings.Builder
	switch {
	case isMySQLDialect(dialect):
		b.WriteString(sql)
		if len(joinParts) > 0 {
			b.WriteString(" ")
			b.WriteString(strings.Join(joinParts, " "))
		}
		b.WriteString(" SET ")
		b.WriteString(strings.Join(sets, ", "))
	case isMSSQLDialect(dialect):
		b.WriteString(sql)
		b.WriteString(" SET ")
		b.WriteString(strings.Join(sets, ", "))
		if outputFragment != "" {
			b.WriteString(" ")
			b.WriteString(outputFragment)
		}
		if len(joinParts) > 0 {
			b.WriteString(" FROM ")
			b.WriteString(dialect.QuoteIdentifier(table))
			b.WriteString(" ")
			b.WriteString(strings.Join(joinParts, " "))
		}
	default:
		b.WriteString(sql)
		b.WriteString(" SET ")
		b.WriteString(strings.Join(sets, ", "))
		if len(joins) > 0 {
			b.WriteString(" FROM ")
			b.WriteString(strings.Join(joinTableRefs(dialect, joins), ", "))
		}
	}

	if whereClause != "" {
		b.WriteString(" WHERE ")
		if len(joins) > 0 && (isPostgresDialect(dialect) || isSQLiteDialect(dialect)) {
			b.WriteString(strings.Join(joinPredicates(dialect, table, joins), " AND "))
			b.WriteString(" AND ")
		}
		b.WriteString(whereClause)
	} else if len(joins) > 0 && (isPostgresDialect(dialect) || isSQLiteDialect(dialect)) {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(joinPredicates(dialect, table, joins), " AND "))
	}

	if outputFragment != "" && !isMSSQLDialect(dialect) {
		b.WriteString(" ")
		b.WriteString(outputFragment)
	}

	b.WriteString(";")

	return b.String(), values, nil
}

// delete builds a DELETE query for the given table, joins, and conditions.
// The joins parameter is optional and may be nil or empty.
// Note: SQLite does not support DELETE with JOINs and will return an error.
//
//nolint:cyclop,gocognit
func delete(
	dialect cdt.SQLDialect,
	table string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *cdt.Join) string,
) (string, []any, error) {
	// Check if SQLite is trying to use DELETE with JOINs
	if len(joins) > 0 {
		if dialectName := fmt.Sprintf("%T", dialect); strings.Contains(dialectName, "SQLite") {
			return "", nil, fmt.Errorf("DELETE with JOINs is not supported in SQLite")
		}
	}

	// Build JOIN clauses if provided
	var joinParts []string
	if len(joins) > 0 {
		for _, j := range joins {
			joinParts = append(joinParts, joinFn(table, &j))
		}
	}

	outputFragment, _, err := dialect.SupportedOptions(definition.QueryTypeDelete, opts, 1)
	if err != nil {
		return "", nil, fmt.Errorf("builder.delete: %w", err)
	}

	var values []any
	var whereClause string

	// Build the WHERE clause and its arguments
	if cond != nil {
		where, condValues, err := cond.ToSQL(dialect, 1)
		if err != nil {
			return "", nil, fmt.Errorf("builder.delete: %w", err)
		}
		whereClause = where
		values = condValues
	}

	// Assemble the final query
	var b strings.Builder
	switch {
	case isMySQLDialect(dialect):
		if len(joinParts) > 0 {
			b.WriteString("DELETE ")
			b.WriteString(dialect.QuoteIdentifier(table))
			b.WriteString(" FROM ")
			b.WriteString(dialect.QuoteIdentifier(table))
			b.WriteString(" ")
			b.WriteString(strings.Join(joinParts, " "))
		} else {
			b.WriteString("DELETE FROM ")
			b.WriteString(dialect.QuoteIdentifier(table))
		}
	case isMSSQLDialect(dialect):
		if len(joinParts) > 0 {
			b.WriteString("DELETE ")
			b.WriteString(dialect.QuoteIdentifier(table))
			if outputFragment != "" {
				b.WriteString(" ")
				b.WriteString(outputFragment)
			}
			b.WriteString(" FROM ")
			b.WriteString(dialect.QuoteIdentifier(table))
			b.WriteString(" ")
			b.WriteString(strings.Join(joinParts, " "))
		} else {
			b.WriteString("DELETE FROM ")
			b.WriteString(dialect.QuoteIdentifier(table))
			if outputFragment != "" {
				b.WriteString(" ")
				b.WriteString(outputFragment)
			}
		}
	default:
		b.WriteString("DELETE FROM ")
		b.WriteString(dialect.QuoteIdentifier(table))
		if len(joins) > 0 {
			b.WriteString(" USING ")
			b.WriteString(strings.Join(joinTableRefs(dialect, joins), ", "))
		}
	}

	if whereClause != "" {
		b.WriteString(" WHERE ")
		if len(joins) > 0 && isPostgresDialect(dialect) {
			b.WriteString(strings.Join(joinPredicates(dialect, table, joins), " AND "))
			b.WriteString(" AND ")
		}
		b.WriteString(whereClause)
	} else if len(joins) > 0 && isPostgresDialect(dialect) {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(joinPredicates(dialect, table, joins), " AND "))
	}

	if outputFragment != "" && !isMSSQLDialect(dialect) {
		b.WriteString(" ")
		b.WriteString(outputFragment)
	}

	b.WriteString(";")

	return b.String(), values, nil
}

func dialectTypeName(dialect cdt.SQLDialect) string {
	return fmt.Sprintf("%T", dialect)
}

func isMySQLDialect(dialect cdt.SQLDialect) bool {
	return strings.Contains(dialectTypeName(dialect), "MySQL")
}

func isPostgresDialect(dialect cdt.SQLDialect) bool {
	return strings.Contains(dialectTypeName(dialect), "Postgres")
}

func isSQLiteDialect(dialect cdt.SQLDialect) bool {
	return strings.Contains(dialectTypeName(dialect), "SQLite")
}

func isMSSQLDialect(dialect cdt.SQLDialect) bool {
	return strings.Contains(dialectTypeName(dialect), "MSSQL")
}

func joinTableRefs(dialect cdt.SQLDialect, joins []cdt.Join) []string {
	refs := make([]string, 0, len(joins))
	for _, join := range joins {
		ref := dialect.QuoteIdentifier(join.Table)
		if join.Alias != "" {
			ref += " AS " + dialect.QuoteIdentifier(join.Alias)
		}
		refs = append(refs, ref)
	}
	return refs
}

func joinPredicates(dialect cdt.SQLDialect, table string, joins []cdt.Join) []string {
	predicates := make([]string, 0)
	for _, join := range joins {
		rightTable := join.Table
		if join.Alias != "" {
			rightTable = join.Alias
		}
		for _, cdt := range join.Conditions {
			predicates = append(predicates, fmt.Sprintf(
				"%s.%s = %s.%s",
				dialect.QuoteIdentifier(table), dialect.QuoteIdentifier(cdt.Left),
				dialect.QuoteIdentifier(rightTable), dialect.QuoteIdentifier(cdt.Right),
			))
		}
	}
	return predicates
}
