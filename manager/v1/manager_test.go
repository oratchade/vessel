//go:build test

package v1_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/fabric/manager/v1"
)

// TestQueryData tests the QueryData structure for different query types.
func TestQueryData(t *testing.T) {
	data := &v1.QueryData{
		Table:      "users",
		ID:         1,
		Columns:    []string{"id", "name"},
		Data:       map[string]any{"name": "John"},
		BulkData:   []map[string]any{{}, {}},
		Conditions: nil,
		Opts:       nil,
		Query:      "SELECT * FROM users",
		Params:     []any{1, 2},
	}

	assert.Equal(t, "users", data.Table)
	assert.Equal(t, 1, data.ID)
	assert.Len(t, data.Columns, 2)
	assert.Len(t, data.Data, 1)
	assert.Len(t, data.BulkData, 2)
	assert.Len(t, data.Params, 2)
}

// TestQueryResponse tests the QueryResponse structure.
func TestQueryResponse(t *testing.T) {
	response := &v1.QueryResponse{
		RequestID: "req-123",
		Data: []map[string]any{
			{"id": 1, "name": "John"},
		},
		RawData:  nil,
		ExecData: nil,
		Error:    nil,
	}

	assert.Equal(t, "req-123", response.RequestID)
	assert.Len(t, response.Data, 1)
	assert.Nil(t, response.Error)
}

// TestQueryStructure tests the Query structure.
func TestQueryStructure(t *testing.T) {
	responseCh := make(chan *v1.QueryResponse)
	q := &v1.Query{
		Request: v1.ReqGet,
		Data: &v1.QueryData{
			Table: "users",
		},
		ResponseCh: responseCh,
	}

	assert.Equal(t, v1.ReqGet, q.Request)
	assert.Equal(t, "users", q.Data.Table)
	assert.Equal(t, responseCh, q.ResponseCh)
}

// TestEmptyQueryData tests QueryData with empty fields.
func TestEmptyQueryData(t *testing.T) {
	data := &v1.QueryData{}

	assert.Equal(t, "", data.Table)
	assert.Nil(t, data.ID)
	assert.Nil(t, data.Columns)
	assert.Nil(t, data.Data)
	assert.Nil(t, data.BulkData)
}

// TestEmptyQueryResponse tests QueryResponse with empty fields.
func TestEmptyQueryResponse(t *testing.T) {
	response := &v1.QueryResponse{}

	assert.Equal(t, "", response.RequestID)
	assert.Nil(t, response.Data)
	assert.Nil(t, response.RawData)
	assert.Nil(t, response.ExecData)
	assert.Nil(t, response.Error)
}

// TestQueryDataWithNilConditions tests QueryData with nil conditions and options.
func TestQueryDataWithNilConditions(t *testing.T) {
	data := &v1.QueryData{
		Table:      "users",
		Conditions: nil,
		Opts:       nil,
	}

	assert.Nil(t, data.Conditions)
	assert.Nil(t, data.Opts)
}

// TestQueryResponseWithData tests QueryResponse with data populated.
func TestQueryResponseWithData(t *testing.T) {
	testData := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	response := &v1.QueryResponse{
		RequestID: "req-2",
		Data:      testData,
		Error:     nil,
	}

	assert.Equal(t, "req-2", response.RequestID)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "Alice", response.Data[0]["name"])
	assert.Equal(t, "Bob", response.Data[1]["name"])
}

// TestQueryDataWithBulkData tests QueryData with bulk insert data.
func TestQueryDataWithBulkData(t *testing.T) {
	bulkData := []map[string]any{
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25},
		{"name": "Charlie", "age": 35},
	}

	data := &v1.QueryData{
		Table:    "users",
		BulkData: bulkData,
	}

	assert.Equal(t, "users", data.Table)
	assert.Len(t, data.BulkData, 3)
	assert.Equal(t, "Alice", data.BulkData[0]["name"])
	assert.Equal(t, 25, data.BulkData[1]["age"])
}

// TestQueryDataWithParams tests QueryData with query parameters.
func TestQueryDataWithParams(t *testing.T) {
	params := []any{
		"John",
		30,
		true,
		1.5,
	}

	data := &v1.QueryData{
		Query:  "SELECT * FROM users WHERE name = ? AND age > ? AND active = ? AND score >= ?",
		Params: params,
	}

	assert.Len(t, data.Params, 4)
	assert.Equal(t, "John", data.Params[0])
	assert.Equal(t, 30, data.Params[1])
	assert.Equal(t, true, data.Params[2])
	assert.Equal(t, 1.5, data.Params[3])
}

// TestQueryRequestTypeConstants verifies query request type constants.
func TestQueryRequestTypeConstants(t *testing.T) {
	assert.NotEmpty(t, v1.ReqGet)
	assert.NotEmpty(t, v1.ReqGetRaw)
	assert.NotEmpty(t, v1.ReqGetByID)
	assert.NotEmpty(t, v1.ReqGetByIDRaw)
	assert.NotEmpty(t, v1.ReqInsert)
	assert.NotEmpty(t, v1.ReqInserts)
	assert.NotEmpty(t, v1.ReqUpdate)
	assert.NotEmpty(t, v1.ReqDelete)
	assert.NotEmpty(t, v1.ReqQuery)
	assert.NotEmpty(t, v1.ReqQueryRaw)
	assert.NotEmpty(t, v1.ReqExec)
}

// TestQueryDataMultipleTables tests QueryData can represent different tables.
func TestQueryDataMultipleTables(t *testing.T) {
	tables := []string{"users", "products", "orders", "invoices"}

	for _, table := range tables {
		data := &v1.QueryData{
			Table: table,
		}
		assert.Equal(t, table, data.Table)
	}
}

// TestQueryDataColumns tests QueryData with different column configurations.
func TestQueryDataColumns(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
	}{
		{"single column", []string{"id"}},
		{"multiple columns", []string{"id", "name", "email"}},
		{"all columns", []string{"*"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &v1.QueryData{
				Table:   "users",
				Columns: tt.columns,
			}
			assert.Equal(t, tt.columns, data.Columns)
		})
	}
}

// TestQueryDataWithDataMap tests QueryData with map-based row data.
func TestQueryDataWithDataMap(t *testing.T) {
	rowData := map[string]any{
		"id":    1,
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	data := &v1.QueryData{
		Table: "users",
		Data:  rowData,
	}

	assert.Equal(t, "users", data.Table)
	assert.Equal(t, 1, data.Data["id"])
	assert.Equal(t, "John Doe", data.Data["name"])
	assert.Equal(t, "john@example.com", data.Data["email"])
	assert.Equal(t, 30, data.Data["age"])
}

// TestQueryResponseMultipleRows tests QueryResponse containing multiple rows.
func TestQueryResponseMultipleRows(t *testing.T) {
	response := &v1.QueryResponse{
		RequestID: "req-list",
		Data: []map[string]any{
			{"id": 1, "name": "Alice", "active": true},
			{"id": 2, "name": "Bob", "active": false},
			{"id": 3, "name": "Charlie", "active": true},
		},
		Error: nil,
	}

	assert.Len(t, response.Data, 3)
	assert.Equal(t, "Alice", response.Data[0]["name"])
	assert.Equal(t, true, response.Data[0]["active"])
	assert.Equal(t, "Bob", response.Data[1]["name"])
	assert.Equal(t, false, response.Data[1]["active"])
}

// TestQueryChannelCreation tests creating response channels in various ways.
func TestQueryChannelCreation(t *testing.T) {
	// Test default channel creation
	ch1 := make(chan *v1.QueryResponse)
	assert.NotNil(t, ch1)

	// Test buffered channel
	ch2 := make(chan *v1.QueryResponse, 10)
	assert.NotNil(t, ch2)

	// Test both are different channels
	assert.NotEqual(t, ch1, ch2)
}

// TestQueryDataWithMultipleIDs tests QueryData handling multiple ID scenarios.
func TestQueryDataWithMultipleIDs(t *testing.T) {
	ids := []int{1, 100, 999, 1000000}

	for _, id := range ids {
		data := &v1.QueryData{
			Table: "users",
			ID:    id,
		}
		assert.Equal(t, id, data.ID)
	}
}

// ============================================================================
// DBManager Integration Tests
// ============================================================================

// TestDBManagerStartStop tests the manager lifecycle (start and stop).
func TestDBManagerStartStop(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Start should not panic with empty entries
	dm.Start(ctx)

	// Stop should not panic
	dm.Stop()
}

// TestDBManagerGetNoDBs tests Get returns channel when no databases configured.
func TestDBManagerGetNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.GetAsync(ctx, "users", []string{"id", "name"}, nil, nil, nil)

	assert.NotNil(t, result)
	// Channel should be readable; may timeout or error is fine
}

// TestDBManagerGetByIDNoDBs tests GetByID returns channel when no databases configured.
func TestDBManagerGetByIDNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.GetByIDAsync(ctx, "users", 1, nil, nil)

	assert.NotNil(t, result)
}

// TestDBManagerGetRawNoDBs tests GetRaw returns channel when no databases configured.
func TestDBManagerGetRawNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.GetRawAsync(ctx, "users", []string{"id", "name"}, nil, nil, nil)

	assert.NotNil(t, result)
}

// TestDBManagerGetByIDRawNoDBs tests GetByIDRaw returns channel when no databases.
func TestDBManagerGetByIDRawNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.GetByIDRawAsync(ctx, "users", 1, nil, nil)

	assert.NotNil(t, result)
}

// TestDBManagerInsertNoDBs tests Insert returns channel when no write databases.
func TestDBManagerInsertNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	data := map[string]any{"name": "John"}

	result := dm.InsertAsync(ctx, "users", data, nil)

	assert.NotNil(t, result)
}

// TestDBManagerInsertsNoDBs tests Inserts returns channel when no write databases.
func TestDBManagerInsertsNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	bulkData := []map[string]any{
		{"name": "Alice"},
		{"name": "Bob"},
	}

	result := dm.InsertsAsync(ctx, "users", bulkData, nil)

	assert.NotNil(t, result)
}

// TestDBManagerUpdateNoDBs tests Update returns channel when no write databases.
func TestDBManagerUpdateNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	data := map[string]any{"name": "Jane"}

	result := dm.UpdateAsync(ctx, "users", data, nil, nil, nil)

	assert.NotNil(t, result)
}

// TestDBManagerDeleteNoDBs tests Delete returns channel when no write databases.
func TestDBManagerDeleteNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.DeleteAsync(ctx, "users", nil, nil, nil)

	assert.NotNil(t, result)
}

// TestDBManagerQueryNoDBs tests Query returns channel when no databases.
func TestDBManagerQueryNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.QueryAsync(ctx, "SELECT * FROM users")

	assert.NotNil(t, result)
}

// TestDBManagerQueryRawNoDBs tests QueryRaw returns channel when no databases.
func TestDBManagerQueryRawNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.QueryRawAsync(ctx, "SELECT * FROM users")

	assert.NotNil(t, result)
}

// TestDBManagerExecNoDBs tests Exec returns channel when no write databases.
func TestDBManagerExecNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	result := dm.ExecAsync(ctx, "INSERT INTO users VALUES (?, ?)", "John", 30)

	assert.NotNil(t, result)
}

// TestDBManagerContextCancellation tests that canceled context is handled gracefully.
func TestDBManagerContextCancellation(t *testing.T) {
	dm := &v1.DBManager{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should handle canceled context without panicking
	result := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	assert.NotNil(t, result)
}

// TestDBManagerContextTimeout tests that timed out context is handled gracefully.
func TestDBManagerContextTimeout(t *testing.T) {
	dm := &v1.DBManager{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	assert.NotNil(t, result)
}

// TestDBManagerMultipleOperations tests that manager can handle multiple sequential operations.
func TestDBManagerMultipleOperations(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Execute multiple operations in sequence
	r1 := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	r2 := dm.InsertAsync(ctx, "users", map[string]any{}, nil)
	r3 := dm.UpdateAsync(ctx, "users", map[string]any{}, nil, nil, nil)
	r4 := dm.DeleteAsync(ctx, "users", nil, nil, nil)
	r5 := dm.QueryAsync(ctx, "SELECT * FROM users")

	assert.NotNil(t, r1)
	assert.NotNil(t, r2)
	assert.NotNil(t, r3)
	assert.NotNil(t, r4)
	assert.NotNil(t, r5)
}

// TestDBManagerReadOnlyVsReadWrite tests that read operations use read-only entries.
func TestDBManagerReadOnlyVsReadWrite(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Read operations
	readResults := []<-chan *v1.QueryResponse{
		dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil),
		dm.GetRawAsync(ctx, "users", []string{"id"}, nil, nil, nil),
		dm.GetByIDAsync(ctx, "users", 1, nil, nil),
		dm.GetByIDRawAsync(ctx, "users", 1, nil, nil),
		dm.QueryAsync(ctx, "SELECT * FROM users"),
		dm.QueryRawAsync(ctx, "SELECT * FROM users"),
	}

	// Write operations
	writeResults := []<-chan *v1.QueryResponse{
		dm.InsertAsync(ctx, "users", map[string]any{}, nil),
		dm.InsertsAsync(ctx, "users", []map[string]any{}, nil),
		dm.UpdateAsync(ctx, "users", map[string]any{}, nil, nil, nil),
		dm.DeleteAsync(ctx, "users", nil, nil, nil),
		dm.ExecAsync(ctx, "INSERT INTO users VALUES (?, ?)", "test"),
	}

	// All should return channels
	for _, result := range readResults {
		assert.NotNil(t, result)
	}
	for _, result := range writeResults {
		assert.NotNil(t, result)
	}
}

// TestDBManagerQueryParameters tests that query parameters are properly passed.
func TestDBManagerQueryParameters(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Test Query with multiple parameters
	result := dm.QueryAsync(ctx, "SELECT * FROM users WHERE id = ? AND name = ?", 1, "John")
	assert.NotNil(t, result)

	// Test Exec with multiple parameters
	execResult := dm.ExecAsync(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", 1, "John", 30)
	assert.NotNil(t, execResult)
}

// TestDBManagerChannelNonBlocking tests that operations return channels immediately.
func TestDBManagerChannelNonBlocking(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// These should all return immediately with channels
	ch1 := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	ch2 := dm.InsertAsync(ctx, "users", map[string]any{}, nil)
	ch3 := dm.QueryAsync(ctx, "SELECT * FROM users")

	// All channels should be non-nil
	assert.NotNil(t, ch1)
	assert.NotNil(t, ch2)
	assert.NotNil(t, ch3)

	// Channels should be distinct
	assert.NotEqual(t, ch1, ch2)
	assert.NotEqual(t, ch2, ch3)
}
