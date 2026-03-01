package db

import (
	"context"
	"fmt"

	builder "tounilab.com/db-connector/internal/pkg/builder"
	cdt "tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/options"
)

type dbOpts struct {
	builder builder.QueryBuilder
	querier sqlQuerier
	logger  Logger
}

func get(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) ([]map[string]any, error) {
	query, args, err := dbOpts.builder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil && dbOpts.logger != nil {
			dbOpts.logger.Error("failed to close rows", "error", err)
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		return nil, fmt.Errorf("scan rows: %w", err)
	}

	return results, nil
}

func getRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*RowsAdapter, error) {
	query, args, err := dbOpts.builder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil && dbOpts.logger != nil {
			dbOpts.logger.Error("failed to close rows", "error", err)
		}
	}()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to create RowsAdapter: %w", err)
	}

	return ra, nil
}

func getByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) ([]map[string]any, error) {
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := dbOpts.builder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil && dbOpts.logger != nil {
			dbOpts.logger.Error("failed to close rows", "error", err)
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		return nil, fmt.Errorf("scan rows: %w", err)
	}

	return results, nil
}

func getByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*RowsAdapter, error) {
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := dbOpts.builder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil && dbOpts.logger != nil {
			dbOpts.logger.Error("failed to close rows", "error", err)
		}
	}()

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to create RowsAdapter: %w", err)
	}

	return ra, nil
}

func insert(
	ctx context.Context,
	table string,
	data map[string]any,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := dbOpts.builder.Insert(table, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute insert query: %w", err)
	}
	return fromSQLResult(result), nil
}

func update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions cdt.Condition,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := dbOpts.builder.Update(table, data, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute update query: %w", err)
	}
	return fromSQLResult(result), nil
}

func delete(
	ctx context.Context,
	table string,
	conditions cdt.Condition,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := dbOpts.builder.Delete(table, conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to build delete query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute delete query: %w", err)
	}
	return fromSQLResult(result), nil
}
