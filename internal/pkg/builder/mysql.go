// Package builder provides SQL query building for multiple database engines.
package builder

import (
	"fmt"
	"strings"

	"tounilab.com/fabric/internal/pkg/operator"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

// MySQLQueryBuilder builds SQL queries that are compatible with MySQL dialects.
type MySQLQueryBuilder struct {
	dialect cdt.SQLDialect
}

// NewMySQLQueryBuilder constructs a new MySQLQueryBuilder using the provided dialect.
func NewMySQLQueryBuilder(dialect cdt.SQLDialect) *MySQLQueryBuilder {
	return &MySQLQueryBuilder{
		dialect: dialect,
	}
}

// Select implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Select(
	table string,
	columns []string,
	joins []cdt.Join,
	opts *options.QueryOptions,
	cond cdt.Condition,
) (string, []any, error) {
	q, v, err := selectQ(m.dialect, table, columns, joins, cond, opts, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("select mssqlSQL Builder: error building select query: %w", err)
	}
	return q, v, nil
}

// Insert implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Insert(table string, data map[string]any) (string, []any, error) {
	q, v, err := insert(m.dialect, table, data)
	if err != nil {
		return "", nil, fmt.Errorf("insert mysqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Inserts implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Inserts(table string, data []map[string]any) (string, []any, error) {
	q, v, err := inserts(m.dialect, table, data)
	if err != nil {
		return "", nil, fmt.Errorf("inserts mysqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Update implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Update(
	table string,
	data map[string]any,
	joins []cdt.Join,
	cond cdt.Condition,
) (string, []any, error) {
	q, v, err := update(m.dialect, table, data, joins, cond, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("update mysqlSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

// Delete implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Delete(table string, joins []cdt.Join, cond cdt.Condition) (string, []any, error) {
	q, v, err := delete(m.dialect, table, joins, cond, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("delete mysqlSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

// join converts a Join to a SQL JOIN clause.
func (m *MySQLQueryBuilder) join(table string, join *cdt.Join) string {
	switch strings.ToLower(join.Type) {
	case operator.Inner, operator.Right, operator.Left:
		return join.ToSQL(table, m.dialect)
	default:
		return ""
	}
}
