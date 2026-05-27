package builder

import (
	"fmt"
	"strings"

	"tounilab.com/vessel/internal/pkg/operator"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

// MySQLQueryBuilder builds SQL queries that are compatible with MySQL dialects.
type MySQLQueryBuilder struct {
	dialect optionDialect
}

// NewMySQLQueryBuilder constructs a new MySQLQueryBuilder using the provided dialect.
func NewMySQLQueryBuilder(dialect optionDialect) *MySQLQueryBuilder {
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
		return "", nil, fmt.Errorf("select mysqlSQL Builder: error building select query: %w", err)
	}
	return q, v, nil
}

// Insert implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Insert(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := insert(m.dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("insert mysqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Inserts implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Inserts(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := inserts(m.dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("inserts mysqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Upsert implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Upsert(
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := upsert(m.dialect, table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("upsert mysqlSQL Builder: error building upsert query: %w", err)
	}
	return q, v, nil
}

// Update implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Update(
	table string,
	data map[string]any,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := update(m.dialect, table, data, joins, cond, opts, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("update mysqlSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

// Delete implements the QueryBuilder interface for MySQL.
func (m *MySQLQueryBuilder) Delete(
	table string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := delete(m.dialect, table, joins, cond, opts, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("delete mysqlSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

// join converts a Join to a SQL JOIN clause.
func (m *MySQLQueryBuilder) join(table string, join *cdt.Join, paramBase int) (string, []any, error) {
	switch strings.ToLower(join.Type) {
	case operator.Inner, operator.Right, operator.Left:
		sql, args, err := join.ToSQLWithArgs(table, m.dialect, paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("mysql join: %w", err)
		}
		return sql, args, nil
	default:
		return "", nil, nil
	}
}
