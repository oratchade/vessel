package builder

import (
	"fmt"

	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

// MSSQLQueryBuilder builds SQL queries compatible with Microsoft SQL Server.
type MSSQLQueryBuilder struct {
	dialect cdt.SQLDialect
}

// NewMSSQLQueryBuilder constructs a new MSSQLQueryBuilder using the provided dialect.
func NewMSSQLQueryBuilder(dialect cdt.SQLDialect) *MSSQLQueryBuilder {
	return &MSSQLQueryBuilder{
		dialect: dialect,
	}
}

// Select implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Select(
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

// Insert implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Insert(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := insert(m.dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("insert mssqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Inserts implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Inserts(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := inserts(m.dialect, table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("inserts mssqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

// Update implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Update(
	table string,
	data map[string]any,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := update(m.dialect, table, data, joins, cond, opts, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("update mssqlSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

// Delete implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Delete(
	table string,
	joins []cdt.Join,
	cond cdt.Condition,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := delete(m.dialect, table, joins, cond, opts, m.join)
	if err != nil {
		return "", nil, fmt.Errorf("delete mssqlSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

// join converts a Join to a SQL JOIN clause.
func (m *MSSQLQueryBuilder) join(table string, join *cdt.Join) string {
	return join.ToSQL(table, m.dialect)
}
