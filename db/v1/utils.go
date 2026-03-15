// Package db provides database abstraction and query execution utilities.
package v1

import (
	"context"
	"fmt"

	builder "tounilab.com/fabric/internal/pkg/builder"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

// dbOpts holds common database operation dependencies used by helper functions.
type dbOpts struct {
	builder builder.QueryBuilder
	querier sqlQuerier
	logger  Logger
}

// get executes a SELECT query and returns results as a slice of maps.
func get(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) ([]map[string]any, error) {
	query, args, err := getQuery(table, columns, joins, conditions, opts, dbOpts)
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

// getRaw executes a SELECT query and returns a RowsAdapter for raw row processing.
func getRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*RowsAdapter, error) {
	query, args, err := getQuery(table, columns, joins, conditions, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to create RowsAdapter: %w", err)
	}

	return ra, nil
}

// getQuery executes a SELECT query and returns the query string and arguments.
func getQuery(
	table string,
	columns []string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Select(table, columns, joins, opts, conditions)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build select query: %w", err)
	}

	return query, args, nil
}

// getByID executes a SELECT query filtered by ID and returns results as a slice of maps.
func getByID(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) ([]map[string]any, error) {
	query, args, err := getByIDQuery(table, id, joins, opts, dbOpts)
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

// getByIDRaw executes a SELECT query filtered by ID and returns a RowsAdapter for raw row processing.
func getByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*RowsAdapter, error) {
	query, args, err := getByIDQuery(table, id, joins, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build select query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to create RowsAdapter: %w", err)
	}

	return ra, nil
}

func getByIDQuery(
	table string,
	id any,
	joins []cdt.Join,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	cdt := &cdt.Expr{}
	cdt.Column("id").Op("=").Value(id)

	query, args, err := dbOpts.builder.Select(table, []string{"*"}, joins, opts, cdt)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build select query: %w", err)
	}

	return query, args, nil
}

// insert executes an INSERT query and returns the result with last insert ID and rows affected.
func insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := insertQuery(table, data, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute insert query: %w", err)
	}
	return fromSQLResult(result), nil
}

func insertQuery(
	table string,
	data map[string]any,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Insert(table, data)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	return query, args, nil
}

// inserts executes an INSERT query for multiple rows and returns the result with last insert ID and rows affected.
func inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := insertsQuery(table, data, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute insert query: %w", err)
	}
	return fromSQLResult(result), nil
}

func insertsQuery(
	table string,
	data []map[string]any,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Inserts(table, data)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	return query, args, nil
}

// update executes an UPDATE query and returns the result with rows affected.
func update(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := updateQuery(table, data, joins, conditions, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build update query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute update query: %w", err)
	}
	return fromSQLResult(result), nil
}

func updateQuery(
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Update(table, data, joins, conditions)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build update query: %w", err)
	}

	return query, args, nil
}

// delete executes a DELETE query and returns the result with rows affected.
func delete(
	ctx context.Context,
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	query, args, err := deleteQuery(table, joins, conditions, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build delete query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute delete query: %w", err)
	}
	return fromSQLResult(result), nil
}

func deleteQuery(
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	_ *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Delete(table, joins, conditions)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build delete query: %w", err)
	}

	return query, args, nil
}
