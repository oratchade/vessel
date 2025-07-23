//nolint:dupl
package builder

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query/condition"
)

type MSSQLQueryBuilder struct {
	dialect condition.SQLDialect
}

func NewMSSQLQueryBuilder(dialect condition.SQLDialect) *MSSQLQueryBuilder {
	return &MSSQLQueryBuilder{
		dialect: dialect,
	}
}

func (m *MSSQLQueryBuilder) Select(table string, columns []string, cond condition.Condition) (string, []any, error) {
	for i, col := range columns {
		columns[i] = "[" + col + "]"
	}

	sql := "SELECT " + strings.Join(columns, ", ") + " FROM [" + table + "]"
	if cond == nil {
		return fmt.Sprintf("%s;", sql), nil, nil
	}

	where, values, err := cond.ToSQL(m.dialect, 1)
	if err != nil {
		return "", nil, fmt.Errorf("select mssqlSQL Builder: error converting condition to SQL: %w", err)
	}

	return fmt.Sprintf("%s WHERE %s;", sql, where), values, nil
}

func (m *MSSQLQueryBuilder) Insert(table string, data map[string]any) (string, []any, error) {
	index := 1
	columns, placeholders, values := make([]string, 0), make([]string, 0), make([]any, 0)

	for col, val := range data {
		columns = append(columns, "["+col+"]")
		placeholders = append(placeholders, m.dialect.Placeholder(index))
		values = append(values, val)
		index++
	}

	return fmt.Sprintf(
		"INSERT INTO [%s] (%s) VALUES (%s);",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	), values, nil
}

func (m *MSSQLQueryBuilder) Update(table string, data map[string]any, cond condition.Condition) (string, []any, error) {
	index := 1
	sets, values := make([]string, 0), make([]any, 0)

	for col, val := range data {
		sets = append(sets, fmt.Sprintf("[%s] = %s", col, m.dialect.Placeholder(index)))
		values = append(values, val)
		index++
	}

	sql := fmt.Sprintf("UPDATE [%s] SET %s", table, strings.Join(sets, ", "))
	if cond == nil {
		return fmt.Sprintf("%s;", sql), values, nil
	}

	where, condValues, err := cond.ToSQL(m.dialect, index)
	if err != nil {
		return "", nil, fmt.Errorf("update mssqlSQL Builder: error converting condition to SQL: %w", err)
	}
	values = append(values, condValues...)

	return fmt.Sprintf("%s WHERE %s;", sql, where), values, nil
}

func (m *MSSQLQueryBuilder) Delete(table string, cond condition.Condition) (string, []any, error) {
	if cond == nil {
		return fmt.Sprintf("DELETE FROM [%s];", table), nil, nil
	}

	where, values, err := cond.ToSQL(m.dialect, 1)
	if err != nil {
		return "", nil, fmt.Errorf("delete mssqlSQL Builder: error converting condition to SQL: %w", err)
	}

	return fmt.Sprintf("DELETE FROM [%s] WHERE %s;", table, where), values, nil
}
