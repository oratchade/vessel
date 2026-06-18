package v1

import (
	"context"
	"fmt"
	"time"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

const (
	defaultTimeout = 30 * time.Second
)

// waitForResponse waits for a query response from a channel with timeout handling.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline is set, applies defaultTimeout.
//	responseCh: Channel receiving the query response.
//	defaultTimeout: Timeout duration to use if context has no deadline.
//
// Returns:
//
//	*QueryResponse: The response from the channel if successful.
//	error: context.Err() if context canceled/deadline exceeded, or channel error.
//
// Behavior:
//   - If context has a deadline, uses that deadline
//   - If context has no deadline, creates a child context with defaultTimeout
//   - If channel receives response, returns it immediately
//   - If context canceled, returns context.Err() (context.Canceled or context.DeadlineExceeded)
//   - If channel closed without response, returns error
func waitForResponse(ctx context.Context, responseCh <-chan *QueryResponse) (*QueryResponse, error) {
	// Check if context already has a deadline
	_, hasDeadline := ctx.Deadline()

	// If no deadline set, create a new context with default timeout
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	// Wait for response or context cancellation
	select {
	case resp := <-responseCh:
		if resp == nil {
			return nil, fmt.Errorf("query response channel closed unexpectedly")
		}
		return resp, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("context error: %w", ctx.Err())
	}
}

// extractDataFromResponse extracts structured data from a query response.
// Returns the Data field if available, or an error if response had an error.
func extractDataFromResponse(resp *QueryResponse) ([]map[string]any, error) {
	if resp.Error != nil {
		return nil, resp.Error
	}
	if resp.Data == nil {
		return make([]map[string]any, 0), nil
	}
	return resp.Data, nil
}

// extractRawDataFromResponse extracts raw row data from a query response.
// Returns the RawData field if available, or an error if response had an error.
func extractRawDataFromResponse(resp *QueryResponse) (*db.RowsAdapter, error) {
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.RawData, nil
}

// extractExecResultFromResponse extracts execution result from a query response.
// Returns the ExecData field if available, or an error if response had an error.
func extractExecResultFromResponse(resp *QueryResponse) (*db.ExecResult, error) {
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.ExecData, nil
}

func waitForExecResult(ctx context.Context, responseCh <-chan *QueryResponse) (*db.ExecResult, error) {
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractExecResultFromResponse(resp)
}

// Get fetches data from the database synchronously based on the specified table, columns, and conditions.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to query.
//	columns: List of columns to retrieve. Use []string{"*"} for all columns.
//	joins: Optional join conditions.
//	cond: Optional where conditions.
//	opts: Optional query options (pagination, sorting).
//
// Returns:
//
//	[]map[string]any: List of records as maps with column names as keys.
//	error: Query error or context error.
//
// Example:
//
//	rows, err := dbManager.Get(ctx, "users", []string{"id", "name"}, nil, nil, nil)
//	if err != nil {
//	  return err
//	}
//	for _, row := range rows {
//	  fmt.Println(row["name"])
//	}
func (dm *DBManager) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	responseCh, err := dm.GetAsync(ctx, table, columns, joins, cond, opts)
	if err != nil {
		return nil, err
	}
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractDataFromResponse(resp)
}

// GetRaw fetches raw data from the database synchronously.
// Returns a RowsAdapter for streaming access to large result sets.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to query.
//	columns: List of columns to retrieve.
//	joins: Optional join conditions.
//	cond: Optional where conditions.
//	opts: Optional query options.
//
// Returns:
//
//	*db.RowsAdapter: Adapter for scanning rows incrementally.
//	error: Query error or context error.
//
// Example:
//
//	rows, err := dbManager.GetRaw(ctx, "users", []string{"*"}, nil, nil, nil)
//	if err != nil {
//	  return err
//	}
//	for rows.Next() {
//	  // Process row
//	}
func (dm *DBManager) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (*db.RowsAdapter, error) {
	responseCh, err := dm.GetRawAsync(ctx, table, columns, joins, cond, opts)
	if err != nil {
		return nil, err
	}
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractRawDataFromResponse(resp)
}

// GetByID fetches a single record from the database synchronously by ID.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to query.
//	id: The ID value to search for.
//	joins: Optional join conditions.
//	opts: Optional query options.
//
// Returns:
//
//	[]map[string]any: List containing single record if found, empty slice if not found.
//	error: Query error or context error.
//
// Example:
//
//	rows, err := dbManager.GetByID(ctx, "users", 123, nil, nil)
//	if err != nil {
//	  return err
//	}
//	if len(rows) == 0 {
//	  return errors.New("user not found")
//	}
//	user := rows[0]
func (dm *DBManager) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []condition.Join,
	opts *options.QueryOptions,
) ([]map[string]any, error) {
	responseCh, err := dm.GetByIDAsync(ctx, table, id, joins, opts)
	if err != nil {
		return nil, err
	}
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractDataFromResponse(resp)
}

// GetByIDRaw fetches a single record from the database synchronously by ID, returning raw rows.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to query.
//	id: The ID value to search for.
//	joins: Optional join conditions.
//	opts: Optional query options.
//
// Returns:
//
//	*db.RowsAdapter: Adapter for scanning the row.
//	error: Query error or context error.
//
// Example:
//
//	rows, err := dbManager.GetByIDRaw(ctx, "users", 123, nil, nil)
//	if err != nil {
//	  return err
//	}
//	// Use rows adapter to scan
func (dm *DBManager) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []condition.Join,
	opts *options.QueryOptions,
) (*db.RowsAdapter, error) {
	responseCh, err := dm.GetByIDRawAsync(ctx, table, id, joins, opts)
	if err != nil {
		return nil, err
	}
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractRawDataFromResponse(resp)
}

// Query executes a raw SQL query synchronously and returns structured data.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	query: Raw SQL query string with optional placeholders.
//	args: Query arguments matching placeholders in query.
//
// Returns:
//
//	[]map[string]any: List of records as maps with column names as keys.
//	error: Query error or context error.
//
// Example:
//
//	rows, err := dbManager.Query(ctx, "SELECT * FROM users WHERE age > ?", 18)
//	if err != nil {
//	  return err
//	}
func (dm *DBManager) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	responseCh, err := dm.QueryAsync(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractDataFromResponse(resp)
}

// QueryRaw executes a raw SQL query synchronously and returns raw rows.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	query: Raw SQL query string with optional placeholders.
//	args: Query arguments matching placeholders in query.
//
// Returns:
//
//	*db.RowsAdapter: Adapter for scanning rows incrementally.
//	error: Query error or context error.
//
// Example:
//
//	rows, err := dbManager.QueryRaw(ctx, "SELECT * FROM users WHERE age > ?", 18)
//	if err != nil {
//	  return err
//	}
//	for rows.Next() {
//	  // Process row
//	}
func (dm *DBManager) QueryRaw(
	ctx context.Context,
	query string,
	args ...any,
) (*db.RowsAdapter, error) {
	responseCh, err := dm.QueryRawAsync(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	resp, err := waitForResponse(ctx, responseCh)
	if err != nil {
		return nil, err
	}
	return extractRawDataFromResponse(resp)
}

// Insert adds a single new record to the database synchronously.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to insert into.
//	data: Map of column names to values. Key should match column names.
//	opts: Optional query options.
//
// Returns:
//
//	*db.ExecResult: Execution result containing LastInsertID and RowsAffected.
//	error: Query error or context error.
//
// Example:
//
//	result, err := dbManager.Insert(ctx, "users", map[string]any{
//	  "name": "John",
//	  "age": 30,
//	}, nil)
//	if err != nil {
//	  return err
//	}
//	fmt.Println("Inserted ID:", result.LastInsertID)
func (dm *DBManager) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (*db.ExecResult, error) {
	responseCh, err := dm.InsertAsync(ctx, table, data, opts)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// Inserts adds multiple new records to the database synchronously in bulk.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to insert into.
//	data: Slice of maps, each representing a record with column names as keys.
//	opts: Optional query options.
//
// Returns:
//
//	*db.ExecResult: Execution result containing RowsAffected.
//	error: Query error or context error.
//
// Example:
//
//	result, err := dbManager.Inserts(ctx, "users", []map[string]any{
//	  {"name": "John", "age": 30},
//	  {"name": "Jane", "age": 28},
//	}, nil)
//	if err != nil {
//	  return err
//	}
//	fmt.Println("Rows affected:", result.RowsAffected)
func (dm *DBManager) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (*db.ExecResult, error) {
	responseCh, err := dm.InsertsAsync(ctx, table, data, opts)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// Upsert inserts a record or applies conflict behavior synchronously.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to insert into.
//	data: Map of column names to values. Key should match column names.
//	upsertOpts: Conflict behavior such as conflict columns and update action.
//	opts: Optional query options.
//
// Returns:
//
//	*db.ExecResult: Execution result containing RowsAffected.
//	error: Query error or context error.
func (dm *DBManager) Upsert(
	ctx context.Context,
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (*db.ExecResult, error) {
	responseCh, err := dm.UpsertAsync(ctx, table, data, upsertOpts, opts)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// Upserts inserts multiple records or applies conflict behavior synchronously.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to insert into.
//	data: Slice of maps, each representing a record with column names as keys.
//	upsertOpts: Conflict behavior such as conflict columns and update action.
//	opts: Optional query options.
//
// Returns:
//
//	*db.ExecResult: Execution result containing RowsAffected.
//	error: Query error or context error.
func (dm *DBManager) Upserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (*db.ExecResult, error) {
	responseCh, err := dm.UpsertsAsync(ctx, table, data, upsertOpts, opts)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// Update updates one or more records in the database synchronously.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to update.
//	data: Map of column names to new values.
//	joins: Optional join conditions.
//	cond: Condition specifying which records to update.
//	opts: Optional query options.
//
// Returns:
//
//	*db.ExecResult: Execution result containing RowsAffected.
//	error: Query error or context error.
//
// Example:
//
//	cond := condition.Equal("id", 123)
//	result, err := dbManager.Update(ctx, "users", map[string]any{
//	  "name": "Johnny",
//	  "age": 31,
//	}, nil, cond, nil)
//	if err != nil {
//	  return err
//	}
//	fmt.Println("Rows affected:", result.RowsAffected)
func (dm *DBManager) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (*db.ExecResult, error) {
	responseCh, err := dm.UpdateAsync(ctx, table, data, joins, cond, opts)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// Delete deletes one or more records from the database synchronously.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	table: Name of the table to delete from.
//	joins: Optional join conditions.
//	cond: Condition specifying which records to delete.
//	opts: Optional query options.
//
// Returns:
//
//	*db.ExecResult: Execution result containing RowsAffected.
//	error: Query error or context error.
//
// Example:
//
//	cond := condition.Equal("id", 123)
//	result, err := dbManager.Delete(ctx, "users", nil, cond, nil)
//	if err != nil {
//	  return err
//	}
//	fmt.Println("Rows deleted:", result.RowsAffected)
func (dm *DBManager) Delete(
	ctx context.Context,
	table string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (*db.ExecResult, error) {
	responseCh, err := dm.DeleteAsync(ctx, table, joins, cond, opts)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// Exec executes a raw SQL statement synchronously without returning result rows.
// Useful for DML operations like INSERT, UPDATE, DELETE, or DDL operations.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//	query: Raw SQL query string with optional placeholders.
//	args: Query arguments matching placeholders in query.
//
// Returns:
//
//	*db.ExecResult: Execution result containing RowsAffected.
//	error: Query error or context error.
//
// Example:
//
//	result, err := dbManager.Exec(ctx, "UPDATE users SET status = ? WHERE id = ?", "active", 123)
//	if err != nil {
//	  return err
//	}
//	fmt.Println("Rows affected:", result.RowsAffected)
func (dm *DBManager) Exec(
	ctx context.Context,
	query string,
	args ...any,
) (*db.ExecResult, error) {
	responseCh, err := dm.ExecAsync(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return waitForExecResult(ctx, responseCh)
}

// PingAsync checks the database connection asynchronously and returns a channel.
//
// Parameters:
//
//	ctx: Context for cancellation and deadlines. If no deadline, 30s default is applied.
//
// Returns:
//
//	error: Error if the connection fails.
//
// Example:
//
//	err := dbManager.Ping(ctx)
//	if err != nil {
//	  return err
//	}
func (dm *DBManager) PingAsync(ctx context.Context) error {
	entry := dm.readOnlyEntry()
	if entry == nil {
		return fmt.Errorf("no read-only database entries configured")
	}
	if err := entry.db.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}
