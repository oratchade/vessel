package builder

import (
	"fmt"
	"strings"

	"tounilab.com/vessel/internal/pkg/operator"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

// SQLiteQueryBuilder builds SQL queries compatible with SQLite dialects.
type SQLiteQueryBuilder struct {
	dialect optionDialect
}

// NewSQLiteQueryBuilder constructs a new SQLiteQueryBuilder using the provided dialect.
func NewSQLiteQueryBuilder(dialect optionDialect) *SQLiteQueryBuilder {
	return &SQLiteQueryBuilder{
		dialect: dialect,
	}
}

// Select implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Select(
	table string,
	columns []string,
	joins []cdt.Join,
	opts *options.QueryOptions,
	cond cdt.Condition,
) (string, []any, error) {
	q, v, err := selectQ(s.dialect, table, columns, joins, cond, opts, s.join)
	if err != nil {
		return "", nil, fmt.Errorf("select sqliteSQL Builder: error building select query: %w", err)
	}
	return q, v, nil
}

// Insert implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Insert(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := insert(s.dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("insert sqliteSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Inserts implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Inserts(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := inserts(s.dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("inserts sqliteSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Upsert implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Upsert(
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := upsert(s.dialect, table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("upsert sqliteSQL Builder: error building upsert query: %w", err)
	}
	return q, v, nil
}

// Upserts implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Upserts(
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := upserts(s.dialect, table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("upserts sqliteSQL Builder: error building upsert query: %w", err)
	}
	return q, v, nil
}

// Update implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Update(
	table string,
	data map[string]any,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := update(s.dialect, table, data, joins, cond, opts, s.join)
	if err != nil {
		return "", nil, fmt.Errorf("update sqliteSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

// Delete implements the QueryBuilder interface for SQLite.
func (s *SQLiteQueryBuilder) Delete(
	table string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := delete(s.dialect, table, joins, cond, opts, s.join)
	if err != nil {
		return "", nil, fmt.Errorf("delete sqliteSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

// join converts a Join to a SQL JOIN clause, handling SQLite-specific RIGHT JOIN conversion.
func (m *SQLiteQueryBuilder) join(table string, join *cdt.Join, paramBase int) (string, []any, error) {
	switch strings.ToLower(join.Type) {
	case operator.Inner, operator.Left:
		sql, args, err := join.ToSQLWithArgs(table, m.dialect, paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("sqlite join: %w", err)
		}
		return sql, args, nil
	case operator.Right:
		j := &cdt.Join{
			Type:       operator.Left,
			Table:      table,
			Conditions: join.Conditions.Reverse(),
			Alias:      "",
		}
		sql, args, err := j.ToSQLWithArgs(table, m.dialect, paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("sqlite right join conversion: %w", err)
		}
		return sql, args, nil
	default:
		return "", nil, nil
	}
}
