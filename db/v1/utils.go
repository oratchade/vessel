package v1

import (
	"context"
	"fmt"

	"tounilab.com/vessel/db/v1/dberror"
	builder "tounilab.com/vessel/internal/pkg/builder"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

// dbOpts holds common database operation dependencies used by helper functions.
type dbOpts struct {
	builder     builder.QueryBuilder
	querier     sqlQuerier
	errorMapper dberror.ErrorMapper
}

func (d dbOpts) mapError(err error) error {
	if err == nil || d.errorMapper == nil {
		return err
	}
	return fmt.Errorf("mapped database error: %w", d.errorMapper.MapError(err))
}

func rejectExecutingReturning(operation string, opts *options.QueryOptions) error {
	if opts == nil || len(opts.Returning) == 0 {
		return nil
	}
	return fmt.Errorf(
		"%s: RETURNING/OUTPUT execution is not supported; use %sQuery to preview the SQL",
		operation,
		operation,
	)
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
		return nil, fmt.Errorf("get: build query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get: execute query: %w", dbOpts.mapError(err))
	}
	defer func() {
		_ = rows.Close()
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get: get columns: %w", err)
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		return nil, fmt.Errorf("get: scan rows: %w", err)
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
		return nil, fmt.Errorf("getRaw: build query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getRaw: execute query: %w", dbOpts.mapError(err))
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("getRaw: create RowsAdapter: %w", err)
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
		return "", nil, fmt.Errorf("getQuery: build query: %w", err)
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
		return nil, fmt.Errorf("getByID: build query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getByID: execute query: %w", dbOpts.mapError(err))
	}
	defer func() {
		_ = rows.Close()
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getByID: get columns: %w", err)
	}

	results, err := scanRows(rows, cols)
	if err != nil {
		return nil, fmt.Errorf("getByID: scan rows: %w", err)
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
		return nil, fmt.Errorf("getByIDRaw: build query: %w", err)
	}

	rows, err := dbOpts.querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getByIDRaw: execute query: %w", dbOpts.mapError(err))
	}

	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, fmt.Errorf("getByIDRaw: create RowsAdapter: %w", err)
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
		return "", nil, fmt.Errorf("getByIDQuery: build query: %w", err)
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
	if err := rejectExecutingReturning("Insert", opts); err != nil {
		return nil, err
	}
	query, args, err := insertQuery(table, data, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("insert: build query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("insert: execute query: %w", dbOpts.mapError(err))
	}

	execResult, err := fromSQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	return execResult, nil
}

func insertQuery(
	table string,
	data map[string]any,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Insert(table, data, opts)
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
	if err := rejectExecutingReturning("Inserts", opts); err != nil {
		return nil, err
	}
	query, args, err := insertsQuery(table, data, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("inserts: build query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("inserts: execute query: %w", dbOpts.mapError(err))
	}

	execResult, err := fromSQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("inserts: %w", err)
	}

	return execResult, nil
}

func insertsQuery(
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Inserts(table, data, opts)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build insert query: %w", err)
	}

	return query, args, nil
}

func upsert(
	ctx context.Context,
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	if err := rejectExecutingReturning("Upsert", opts); err != nil {
		return nil, err
	}
	query, args, err := upsertQuery(table, data, upsertOpts, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("upsert: build query: %w", err)
	}
	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("upsert: execute query: %w", dbOpts.mapError(err))
	}
	execResult, err := fromSQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("upsert: %w", err)
	}
	return execResult, nil
}

func upserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (*ExecResult, error) {
	if err := rejectExecutingReturning("Upserts", opts); err != nil {
		return nil, err
	}
	query, args, err := upsertsQuery(table, data, upsertOpts, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("upserts: build query: %w", err)
	}
	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("upserts: execute query: %w", dbOpts.mapError(err))
	}
	execResult, err := fromSQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("upserts: %w", err)
	}
	return execResult, nil
}

func upsertQuery(
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Upsert(table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build upsert query: %w", err)
	}
	return query, args, nil
}

func upsertsQuery(
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Upserts(table, data, upsertOpts, opts)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build upserts query: %w", err)
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
	if err := rejectExecutingReturning("Update", opts); err != nil {
		return nil, err
	}
	query, args, err := updateQuery(table, data, joins, conditions, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("update: build query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update: execute query: %w", dbOpts.mapError(err))
	}

	execResult, err := fromSQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}

	return execResult, nil
}

func updateQuery(
	table string,
	data map[string]any,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Update(table, data, joins, conditions, opts)
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
	if err := rejectExecutingReturning("Delete", opts); err != nil {
		return nil, err
	}
	query, args, err := deleteQuery(table, joins, conditions, opts, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("delete: build query: %w", err)
	}

	result, err := dbOpts.querier.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("delete: execute query: %w", dbOpts.mapError(err))
	}

	execResult, err := fromSQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}

	return execResult, nil
}

func deleteQuery(
	table string,
	joins []cdt.Join,
	conditions cdt.Condition,
	opts *options.QueryOptions,
	dbOpts dbOpts,
) (string, []any, error) {
	query, args, err := dbOpts.builder.Delete(table, joins, conditions, opts)
	if err != nil {
		return "", nil, fmt.Errorf("deleteQuery: build query: %w", err)
	}

	return query, args, nil
}
