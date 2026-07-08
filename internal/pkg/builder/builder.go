package builder

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"tounilab.com/vessel/internal/pkg/helpers"
	"tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/definition"
	"tounilab.com/vessel/pkg/query/options"
)

var sqlFunctionPattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*)\s*\(.*\)$`)

// aliasPattern matches the " AS " separator in projection aliases. Every
// built-in dialect maps the alias operator to "AS", so the pattern is
// compiled once instead of per column per query.
var aliasPattern = regexp.MustCompile(`(?i)\s+AS\s+`)

type optionDialect interface {
	cdt.SQLDialect
	SupportedOptions(definition.QueryType, *options.QueryOptions, int) (string, []any, error)
}

//go:generate mockgen -source=builder.go -destination=builder_mocks.go -package=builder

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

	// Upsert builds a single-row INSERT with conflict handling.
	Upsert(
		table string,
		data map[string]any,
		upsertOpts *options.UpsertOptions,
		opts *options.QueryOptions,
	) (string, []any, error)

	// Upserts builds a bulk INSERT with conflict handling.
	Upserts(
		table string,
		data []map[string]any,
		upsertOpts *options.UpsertOptions,
		opts *options.QueryOptions,
	) (string, []any, error)

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
//
//nolint:cyclop
func selectQ(
	dialect optionDialect,
	table string,
	columns []string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *cdt.Join, paramBase int) (string, []any, error),
) (string, []any, error) {
	cols := make([]string, len(columns))
	for i, col := range columns {
		cols[i] = sanitizeColumn(dialect, col)
	}

	sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + dialect.QuoteIdentifier(table)

	join := make([]string, 0, len(joins))
	var joinValues []any
	nextParam := 1
	for _, j := range joins {
		joinSQL, joinArgs, err := joinFn(table, &j, nextParam)
		if err != nil {
			return "", nil, fmt.Errorf("builder.selectQ: %w", err)
		}
		if joinSQL != "" {
			join = append(join, joinSQL)
		}
		joinValues = append(joinValues, joinArgs...)
		nextParam += len(joinArgs)
	}

	// Build condition fragment (if any)
	var where string
	var values []any
	if cond != nil {
		w, v, err := cond.ToSQL(dialect, nextParam)
		if err != nil {
			return "", nil, fmt.Errorf("builder.selectQ: %w", err)
		}
		where = w
		values = v
	}

	// compute param base for options: placeholders used so far + starting index (1-based)
	nextParam += len(values)

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
	allArgs := make([]any, 0, len(joinValues)+len(values)+len(optArgs))
	allArgs = append(allArgs, joinValues...)
	allArgs = append(allArgs, values...)
	allArgs = append(allArgs, optArgs...)

	return b.String(), allArgs, nil
}

// sanitizeColumn quotes and processes a column identifier, handling aliases and qualified names.
//
//nolint:cyclop
func sanitizeColumn(dialect cdt.SQLDialect, column string) string {
	c, alias := column, ""
	if raw, ok := helpers.IsRawProjection(column); ok {
		c = raw
		if loc := rawAliasIndex(c); loc != nil {
			c, alias = c[:loc[0]], c[loc[1]:]
		}
		if alias != "" {
			alias = " " + dialect.Operator("AS") + " " + dialect.QuoteIdentifier(strings.TrimSpace(alias))
		}
		return strings.TrimSpace(c) + alias
	}
	if column == "*" {
		return "*"
	}

	if loc := aliasPattern.FindStringIndex(column); loc != nil {
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
			part := strings.TrimSpace(p)
			if part == "*" {
				parts = append(parts, "*")
				continue
			}
			parts = append(parts, dialect.QuoteIdentifier(part))
		}

		return fmt.Sprintf("%s%s", strings.Join(parts, "."), alias)
	}
	return fmt.Sprintf("%s%s", dialect.QuoteIdentifier(strings.TrimSpace(c)), alias)
}

func rawAliasIndex(column string) []int {
	return aliasPattern.FindStringIndex(column)
}

// insert builds an INSERT query for the given table and data.
func insert(
	dialect optionDialect,
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

//nolint:cyclop
func upsert(
	dialect optionDialect,
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	if upsertOpts == nil {
		return "", nil, fmt.Errorf("builder.upsert: upsert options are required")
	}
	if upsertOpts.Action == "" {
		return "", nil, fmt.Errorf("builder.upsert: upsert action is required")
	}
	if isMSSQLDialect(dialect) {
		return "", nil, fmt.Errorf(
			"builder.upsert: MSSQL upsert is not supported; use an explicit transaction or raw SQL",
		)
	}

	baseSQL, values, err := insert(dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("builder.upsert: %w", err)
	}
	baseSQL = strings.TrimSuffix(baseSQL, ";")

	switch upsertOpts.Action {
	case options.UpsertDoNothing:
		conflict, err := conflictTarget(dialect, upsertOpts)
		if err != nil {
			return "", nil, err
		}
		if isMySQLDialect(dialect) {
			noOpColumn := dialect.QuoteIdentifier(upsertOpts.ConflictColumns[0])
			return baseSQL + " ON DUPLICATE KEY UPDATE " + noOpColumn + " = " + noOpColumn + ";", values, nil
		}
		return baseSQL + " ON CONFLICT " + conflict + " DO NOTHING;", values, nil
	case options.UpsertDoUpdate:
		fragment, extraValues, err := upsertUpdateFragment(dialect, data, upsertOpts, len(values)+1)
		if err != nil {
			return "", nil, err
		}
		values = append(values, extraValues...)
		if isMySQLDialect(dialect) {
			return baseSQL + " ON DUPLICATE KEY UPDATE " + fragment + ";", values, nil
		}
		conflict, err := conflictTarget(dialect, upsertOpts)
		if err != nil {
			return "", nil, err
		}
		return baseSQL + " ON CONFLICT " + conflict + " DO UPDATE SET " + fragment + ";", values, nil
	default:
		return "", nil, fmt.Errorf("builder.upsert: unsupported upsert action %q", upsertOpts.Action)
	}
}

//nolint:cyclop
func upserts(
	dialect optionDialect,
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	if len(data) == 0 {
		return "", nil, fmt.Errorf("builder.upserts: no data provided for insertion")
	}
	if upsertOpts == nil {
		return "", nil, fmt.Errorf("builder.upserts: upsert options are required")
	}
	if upsertOpts.Action == "" {
		return "", nil, fmt.Errorf("builder.upserts: upsert action is required")
	}
	if isMSSQLDialect(dialect) {
		return "", nil, fmt.Errorf(
			"builder.upserts: MSSQL upsert is not supported; use an explicit transaction or raw SQL",
		)
	}

	baseSQL, values, err := inserts(dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("builder.upserts: %w", err)
	}
	baseSQL = strings.TrimSuffix(baseSQL, ";")

	switch upsertOpts.Action {
	case options.UpsertDoNothing:
		conflict, err := conflictTarget(dialect, upsertOpts)
		if err != nil {
			return "", nil, err
		}
		if isMySQLDialect(dialect) {
			noOpColumn := dialect.QuoteIdentifier(upsertOpts.ConflictColumns[0])
			return baseSQL + " ON DUPLICATE KEY UPDATE " + noOpColumn + " = " + noOpColumn + ";", values, nil
		}
		return baseSQL + " ON CONFLICT " + conflict + " DO NOTHING;", values, nil
	case options.UpsertDoUpdate:
		fragment, extraValues, err := upsertUpdateFragment(dialect, data[0], upsertOpts, len(values)+1)
		if err != nil {
			return "", nil, err
		}
		values = append(values, extraValues...)
		if isMySQLDialect(dialect) {
			return baseSQL + " ON DUPLICATE KEY UPDATE " + fragment + ";", values, nil
		}
		conflict, err := conflictTarget(dialect, upsertOpts)
		if err != nil {
			return "", nil, err
		}
		return baseSQL + " ON CONFLICT " + conflict + " DO UPDATE SET " + fragment + ";", values, nil
	default:
		return "", nil, fmt.Errorf("builder.upserts: unsupported upsert action %q", upsertOpts.Action)
	}
}

func conflictTarget(dialect optionDialect, upsertOpts *options.UpsertOptions) (string, error) {
	if len(upsertOpts.ConflictColumns) == 0 {
		return "", fmt.Errorf("builder.upsert: conflict columns are required")
	}
	quoted := make([]string, 0, len(upsertOpts.ConflictColumns))
	for _, col := range upsertOpts.ConflictColumns {
		if col == "" {
			return "", fmt.Errorf("builder.upsert: conflict column cannot be empty")
		}
		quoted = append(quoted, dialect.QuoteIdentifier(col))
	}
	return "(" + strings.Join(quoted, ", ") + ")", nil
}

//nolint:cyclop
func upsertUpdateFragment(
	dialect optionDialect,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	startIndex int,
) (string, []any, error) {
	columns, err := upsertUpdateColumns(data, upsertOpts)
	if err != nil {
		return "", nil, err
	}
	if len(upsertOpts.UpdateValues) > 0 {
		filtered := columns[:0]
		for _, col := range columns {
			if _, ok := upsertOpts.UpdateValues[col]; ok {
				continue
			}
			filtered = append(filtered, col)
		}
		columns = filtered
	}
	setParts := make([]string, 0, len(columns)+len(upsertOpts.UpdateValues))
	values := make([]any, 0, len(upsertOpts.UpdateValues))
	index := startIndex

	for _, col := range columns {
		quoted := dialect.QuoteIdentifier(col)
		switch {
		case isMySQLDialect(dialect):
			setParts = append(setParts, fmt.Sprintf("%s = VALUES(%s)", quoted, quoted))
		default:
			setParts = append(setParts, fmt.Sprintf("%s = excluded.%s", quoted, quoted))
		}
	}

	if len(upsertOpts.UpdateValues) > 0 {
		valueColumns := make([]string, 0, len(upsertOpts.UpdateValues))
		for col := range upsertOpts.UpdateValues {
			valueColumns = append(valueColumns, col)
		}
		sort.Strings(valueColumns)
		for _, col := range valueColumns {
			setParts = append(setParts, fmt.Sprintf(
				"%s = %s",
				dialect.QuoteIdentifier(col),
				dialect.Placeholder(index),
			))
			values = append(values, upsertOpts.UpdateValues[col])
			index++
		}
	}

	if len(setParts) == 0 {
		return "", nil, fmt.Errorf("builder.upsert: no update columns provided")
	}
	return strings.Join(setParts, ", "), values, nil
}

func upsertUpdateColumns(data map[string]any, upsertOpts *options.UpsertOptions) ([]string, error) {
	columns := append([]string(nil), upsertOpts.UpdateColumns...)
	if len(columns) == 0 {
		conflict := make(map[string]struct{}, len(upsertOpts.ConflictColumns))
		for _, col := range upsertOpts.ConflictColumns {
			conflict[col] = struct{}{}
		}
		for col := range data {
			if _, ok := conflict[col]; ok {
				continue
			}
			columns = append(columns, col)
		}
	}
	sort.Strings(columns)
	for _, col := range columns {
		if col == "" {
			return nil, fmt.Errorf("builder.upsert: update column cannot be empty")
		}
		if _, ok := data[col]; !ok {
			return nil, fmt.Errorf("builder.upsert: update column %q is not present in insert data", col)
		}
	}
	return columns, nil
}

// insert builds an INSERT query for the given table and multiple rows of data.
func inserts(
	dialect optionDialect,
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
//nolint:prealloc,cyclop,gocognit
func update(
	dialect optionDialect,
	table string,
	data map[string]any,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *cdt.Join, paramBase int) (string, []any, error),
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
			joinSQL, joinArgs, err := joinFn(table, &j, index)
			if err != nil {
				return "", nil, fmt.Errorf("builder.update: %w", err)
			}
			if len(joinArgs) > 0 {
				return "", nil, fmt.Errorf("builder.update: joined mutation predicates with values are not supported")
			}
			joinParts = append(joinParts, joinSQL)
		}
	}

	if err := validateMutationOptions(dialect, definition.QueryTypeUpdate, joins, opts); err != nil {
		return "", nil, fmt.Errorf("builder.update: %w", err)
	}

	outputFragment, _, err := dialect.SupportedOptions(definition.QueryTypeUpdate, returningOptions(opts), index)
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
		index += len(condValues)
	}

	tailFragment, tailArgs, err := dialect.SupportedOptions(
		definition.QueryTypeUpdate,
		mutationTailOptions(opts),
		index,
	)
	if err != nil {
		return "", nil, fmt.Errorf("builder.update: %w", err)
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
			predicates, err := joinPredicates(dialect, table, joins)
			if err != nil {
				return "", nil, fmt.Errorf("builder.update: %w", err)
			}
			b.WriteString(strings.Join(predicates, " AND "))
			b.WriteString(" AND ")
		}
		b.WriteString(whereClause)
	} else if len(joins) > 0 && (isPostgresDialect(dialect) || isSQLiteDialect(dialect)) {
		b.WriteString(" WHERE ")
		predicates, err := joinPredicates(dialect, table, joins)
		if err != nil {
			return "", nil, fmt.Errorf("builder.update: %w", err)
		}
		b.WriteString(strings.Join(predicates, " AND "))
	}

	if outputFragment != "" && !isMSSQLDialect(dialect) {
		b.WriteString(" ")
		b.WriteString(outputFragment)
	}
	if tailFragment != "" {
		b.WriteString(" ")
		b.WriteString(tailFragment)
		values = append(values, tailArgs...)
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
	dialect optionDialect,
	table string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
	joinFn func(table string, join *cdt.Join, paramBase int) (string, []any, error),
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
			joinSQL, joinArgs, err := joinFn(table, &j, 1)
			if err != nil {
				return "", nil, fmt.Errorf("builder.delete: %w", err)
			}
			if len(joinArgs) > 0 {
				return "", nil, fmt.Errorf("builder.delete: joined mutation predicates with values are not supported")
			}
			joinParts = append(joinParts, joinSQL)
		}
	}

	if err := validateMutationOptions(dialect, definition.QueryTypeDelete, joins, opts); err != nil {
		return "", nil, fmt.Errorf("builder.delete: %w", err)
	}

	outputFragment, _, err := dialect.SupportedOptions(definition.QueryTypeDelete, returningOptions(opts), 1)
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
	nextParam := 1 + len(values)
	tailFragment, tailArgs, err := dialect.SupportedOptions(
		definition.QueryTypeDelete,
		mutationTailOptions(opts),
		nextParam,
	)
	if err != nil {
		return "", nil, fmt.Errorf("builder.delete: %w", err)
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
			predicates, err := joinPredicates(dialect, table, joins)
			if err != nil {
				return "", nil, fmt.Errorf("builder.delete: %w", err)
			}
			b.WriteString(strings.Join(predicates, " AND "))
			b.WriteString(" AND ")
		}
		b.WriteString(whereClause)
	} else if len(joins) > 0 && isPostgresDialect(dialect) {
		b.WriteString(" WHERE ")
		predicates, err := joinPredicates(dialect, table, joins)
		if err != nil {
			return "", nil, fmt.Errorf("builder.delete: %w", err)
		}
		b.WriteString(strings.Join(predicates, " AND "))
	}

	if outputFragment != "" && !isMSSQLDialect(dialect) {
		b.WriteString(" ")
		b.WriteString(outputFragment)
	}
	if tailFragment != "" {
		b.WriteString(" ")
		b.WriteString(tailFragment)
		values = append(values, tailArgs...)
	}

	b.WriteString(";")

	return b.String(), values, nil
}

func validateMutationOptions(
	dialect cdt.SQLDialect,
	queryType definition.QueryType,
	joins []cdt.Join,
	opts *options.QueryOptions,
) error {
	if opts == nil {
		return nil
	}
	if opts.Offset != nil {
		return fmt.Errorf("%s does not support Offset", queryType)
	}
	if len(opts.OrderBy) == 0 && opts.Limit == nil {
		return nil
	}
	caps := sqldialect.CapabilitiesFor(dialect)
	if !caps.MutationOrderLimit {
		return fmt.Errorf("%s does not support OrderBy or Limit for %s", dialectTypeName(dialect), queryType)
	}
	if len(joins) > 0 {
		return fmt.Errorf("%s does not support OrderBy or Limit with joined %s", dialectTypeName(dialect), queryType)
	}
	return nil
}

func returningOptions(opts *options.QueryOptions) *options.QueryOptions {
	if opts == nil || len(opts.Returning) == 0 {
		return nil
	}
	return &options.QueryOptions{Returning: opts.Returning}
}

func mutationTailOptions(opts *options.QueryOptions) *options.QueryOptions {
	if opts == nil || (len(opts.OrderBy) == 0 && opts.Limit == nil) {
		return nil
	}
	return &options.QueryOptions{
		OrderBy: opts.OrderBy,
		Limit:   opts.Limit,
	}
}

func dialectTypeName(dialect cdt.SQLDialect) string {
	return fmt.Sprintf("%T", dialect)
}

func isMySQLDialect(dialect cdt.SQLDialect) bool {
	switch dialect.(type) {
	case sqldialect.MySQLDialect, *sqldialect.MySQLDialect:
		return true
	default:
		return false
	}
}

func isPostgresDialect(dialect cdt.SQLDialect) bool {
	switch dialect.(type) {
	case sqldialect.PostgresDialect, *sqldialect.PostgresDialect:
		return true
	default:
		return false
	}
}

func isSQLiteDialect(dialect cdt.SQLDialect) bool {
	switch dialect.(type) {
	case sqldialect.SQLiteDialect, *sqldialect.SQLiteDialect:
		return true
	default:
		return false
	}
}

func isMSSQLDialect(dialect cdt.SQLDialect) bool {
	switch dialect.(type) {
	case sqldialect.MSSQLDialect, *sqldialect.MSSQLDialect:
		return true
	default:
		return false
	}
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

func joinPredicates(dialect cdt.SQLDialect, table string, joins []cdt.Join) ([]string, error) {
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
		if join.On != nil {
			onSQL, onArgs, err := join.On.ToSQL(dialect, 1)
			if err != nil {
				return nil, fmt.Errorf("join ON condition: %w", err)
			}
			if len(onArgs) > 0 {
				return nil, fmt.Errorf("join ON condition values are not supported")
			}
			predicates = append(predicates, onSQL)
		}
	}
	if len(predicates) == 0 {
		return nil, fmt.Errorf("joined mutation requires conditions or ON condition")
	}
	return predicates, nil
}
