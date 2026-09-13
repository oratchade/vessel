package builder

import (
	"fmt"
	"strings"

	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/definition"
	"tounilab.com/vessel/pkg/query/options"
)

// MSSQLQueryBuilder builds SQL queries compatible with Microsoft SQL Server.
type MSSQLQueryBuilder struct {
	dialect optionDialect
}

// NewMSSQLQueryBuilder constructs a new MSSQLQueryBuilder using the provided dialect.
func NewMSSQLQueryBuilder(dialect optionDialect) *MSSQLQueryBuilder {
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

// Upsert implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Upsert(
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := upsert(m.dialect, table, data, upsertOpts, withoutReturning(opts))
	if err != nil {
		return "", nil, fmt.Errorf("upsert mssqlSQL Builder: error building upsert query: %w", err)
	}
	q, err = appendInsertReturning(m.dialect, q, opts)
	if err != nil {
		return "", nil, fmt.Errorf("upsert mssqlSQL Builder: error building returning clause: %w", err)
	}
	return q, v, nil
}

// Upserts implements the QueryBuilder interface for MSSQL.
func (m *MSSQLQueryBuilder) Upserts(
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (string, []any, error) {
	q, v, err := upserts(m.dialect, table, data, upsertOpts, withoutReturning(opts))
	if err != nil {
		return "", nil, fmt.Errorf("upserts mssqlSQL Builder: error building upsert query: %w", err)
	}
	q, err = appendInsertReturning(m.dialect, q, opts)
	if err != nil {
		return "", nil, fmt.Errorf("upserts mssqlSQL Builder: error building returning clause: %w", err)
	}
	return q, v, nil
}

// withoutReturning drops Returning so it can be rendered after the conflict
// clause; the INSERT renderer would emit the invalid RETURNING ... ON CONFLICT.
func withoutReturning(opts *options.QueryOptions) *options.QueryOptions {
	if opts == nil || len(opts.Returning) == 0 {
		return opts
	}
	stripped := *opts
	stripped.Returning = nil
	return &stripped
}

func appendInsertReturning(dialect optionDialect, query string, opts *options.QueryOptions) (string, error) {
	fragment, _, err := dialect.SupportedOptions(definition.QueryTypeInsert, returningOptions(opts), 0)
	if err != nil {
		return "", fmt.Errorf("render RETURNING: %w", err)
	}
	if fragment == "" {
		return query, nil
	}
	return strings.TrimSuffix(query, ";") + " " + fragment + ";", nil
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
func (m *MSSQLQueryBuilder) join(table string, join *cdt.Join, paramBase int) (string, []any, error) {
	sql, args, err := join.ToSQLWithArgs(table, m.dialect, paramBase)
	if err != nil {
		return "", nil, fmt.Errorf("mssql join: %w", err)
	}
	return sql, args, nil
}
