package builder

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/condition"
)

type SQLiteQueryBuilder struct {
	dialect condition.SQLDialect
}

func NewSQLiteQueryBuilder(dialect condition.SQLDialect) *SQLiteQueryBuilder {
	return &SQLiteQueryBuilder{
		dialect: dialect,
	}
}

func (s *SQLiteQueryBuilder) Select(
	table string,
	columns []string,
	joins []Join,
	cond condition.Condition,
) (string, []any, error) {
	q, v, err := selectQ(s.dialect, table, columns, joins, cond, s.join)
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
	cond condition.Condition,
) (string, []any, error) {
	q, v, err := update(s.dialect, table, data, cond)
	if err != nil {
		return "", nil, fmt.Errorf("update sqliteSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

func (s *SQLiteQueryBuilder) Delete(table string, cond condition.Condition) (string, []any, error) {
	q, v, err := delete(s.dialect, table, cond)
	if err != nil {
		return "", nil, fmt.Errorf("delete sqliteSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

func (m *SQLiteQueryBuilder) join(table string, join *Join) string {
	switch strings.ToLower(join.Type) {
	case query.Inner, query.Left:
		return join.ToSQL(table, m.dialect)
	case query.Right:
		j := &Join{
			Type:  query.Left,
			Table: table,
			Left:  join.Right,
			Right: join.Left,
			Alias: "",
		}
		return j.ToSQL(table, m.dialect)
	default:
		return ""
	}
}
