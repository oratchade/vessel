package builder

import (
	"fmt"

	"tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/options"
)

// MSSQLQueryBuilder builds SQL queries compatible with Microsoft SQL Server.
type MSSQLQueryBuilder struct {
	dialect condition.SQLDialect
}

// NewMSSQLQueryBuilder constructs a new MSSQLQueryBuilder using the provided dialect.
func NewMSSQLQueryBuilder(dialect condition.SQLDialect) *MSSQLQueryBuilder {
	return &MSSQLQueryBuilder{
		dialect: dialect,
	}
}

func (m *MSSQLQueryBuilder) Select(
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

func (m *MSSQLQueryBuilder) Insert(table string, data map[string]any) (string, []any, error) {
	q, v, err := insert(m.dialect, table, data)
	if err != nil {
		return "", nil, fmt.Errorf("insert mssqlSQL Builder: error building insert query: %w", err)
	}
	return q, v, nil
}

func (m *MSSQLQueryBuilder) Update(table string, data map[string]any, cond condition.Condition) (string, []any, error) {
	q, v, err := update(m.dialect, table, data, cond)
	if err != nil {
		return "", nil, fmt.Errorf("update mssqlSQL Builder: error building update query: %w", err)
	}
	return q, v, nil
}

func (m *MSSQLQueryBuilder) Delete(table string, cond condition.Condition) (string, []any, error) {
	q, v, err := delete(m.dialect, table, cond)
	if err != nil {
		return "", nil, fmt.Errorf("delete mssqlSQL Builder: error building delete query: %w", err)
	}
	return q, v, nil
}

func (m *MSSQLQueryBuilder) join(table string, join *Join) string {
	return join.ToSQL(table, m.dialect)
}
