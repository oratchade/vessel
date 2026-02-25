package builder

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/options"
)

// MySQLQueryBuilder builds SQL queries that are compatible with MySQL dialects.
type MySQLQueryBuilder struct {
	dialect condition.SQLDialect
}

// NewMySQLQueryBuilder constructs a new MySQLQueryBuilder using the provided dialect.
func NewMySQLQueryBuilder(dialect condition.SQLDialect) *MySQLQueryBuilder {
	return &MySQLQueryBuilder{
		dialect: dialect,
	}
}

func (m *MySQLQueryBuilder) Select(
	table string,
	columns []string,
	joins []Join,
	opts *options.QueryOptions,
	cond condition.Condition,
) (string, []any, error) {
	q, v, err := selectQ(m.dialect, table, columns, joins, cond, opts, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("select mssqlSQL Builder: error building select query: %w", err)
	}
	return q, v, nil
}

func (m *MySQLQueryBuilder) Insert(table string, data map[string]any) (string, []any, error) {
	q, v, err := insert(m.dialect, table, data)
	if err != nil {
		return "", nil, fmt.Errorf("insert mysqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

func (m *MySQLQueryBuilder) Update(table string, data map[string]any, cond condition.Condition) (string, []any, error) {
	q, v, err := update(m.dialect, table, data, cond)
	if err != nil {
		return "", nil, fmt.Errorf("update mysqlSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

func (m *MySQLQueryBuilder) Delete(table string, cond condition.Condition) (string, []any, error) {
	q, v, err := delete(m.dialect, table, cond)
	if err != nil {
		return "", nil, fmt.Errorf("delete mysqlSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

func (m *MySQLQueryBuilder) join(table string, join *Join) string {
	switch strings.ToLower(join.Type) {
	case query.Inner, query.Right, query.Left:
		return join.ToSQL(table, m.dialect)
	default:
		return ""
	}
}
