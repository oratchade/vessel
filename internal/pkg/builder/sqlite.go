package builder

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/internal/pkg/operator"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/options"
)

// SQLiteQueryBuilder builds SQL queries compatible with SQLite dialects.
type SQLiteQueryBuilder struct {
	dialect cdt.SQLDialect
}

// NewSQLiteQueryBuilder constructs a new SQLiteQueryBuilder using the provided dialect.
func NewSQLiteQueryBuilder(dialect cdt.SQLDialect) *SQLiteQueryBuilder {
	return &SQLiteQueryBuilder{
		dialect: dialect,
	}
}

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

func (s *SQLiteQueryBuilder) Insert(table string, data map[string]any) (string, []any, error) {
	q, v, err := insert(s.dialect, table, data)
	if err != nil {
		return "", nil, fmt.Errorf("insert sqliteSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

func (s *SQLiteQueryBuilder) Update(
	table string,
	data map[string]any,
	cond cdt.Condition,
) (string, []any, error) {
	q, v, err := update(s.dialect, table, data, cond)
	if err != nil {
		return "", nil, fmt.Errorf("update sqliteSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

func (s *SQLiteQueryBuilder) Delete(table string, cond cdt.Condition) (string, []any, error) {
	q, v, err := delete(s.dialect, table, cond)
	if err != nil {
		return "", nil, fmt.Errorf("delete sqliteSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

func (m *SQLiteQueryBuilder) join(table string, join *cdt.Join) string {
	switch strings.ToLower(join.Type) {
	case operator.Inner, operator.Left:
		return join.ToSQL(table, m.dialect)
	case operator.Right:
		j := &cdt.Join{
			Type:       operator.Left,
			Table:      table,
			Conditions: join.Conditions.Reverse(),
			Alias:      "",
		}
		return j.ToSQL(table, m.dialect)
	default:
		return ""
	}
}
