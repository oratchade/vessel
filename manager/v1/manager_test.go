//go:build test

package v1_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/vessel/manager/v1"
	"tounilab.com/vessel/pkg/query/options"
)

// TestQueryData tests the QueryData structure for different query types.
func TestQueryData(t *testing.T) {
	data := &v1.QueryData{
		Table:    "users",
		ID:       1,
		Columns:  []string{"id", "name"},
		Data:     map[string]any{"name": "John"},
		BulkData: []map[string]any{{}, {}},
		Params:   []any{1, 2},
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
		Error: nil,
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
	assert.NotEmpty(t, v1.ReqUpsert)
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
		Data: []map[string]any{
			{"id": 1, "name": "Alice", "active": true},
			{"id": 2, "name": "Bob", "active": false},
			{"id": 3, "name": "Charlie", "active": true},
		},
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
			ID: id,
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

	dm.Start()
	dm.Start()

	dm.Stop()
	dm.Stop()
}

func TestDBManagerAsyncBeforeStartReturnsNotStarted(t *testing.T) {
	dm := &v1.DBManager{}

	ch, err := dm.QueryAsync(context.Background(), "SELECT 1")

	assert.Nil(t, ch)
	assert.ErrorIs(t, err, v1.ErrManagerNotStarted)
}

func TestDBManagerAsyncAfterStopReturnsClosed(t *testing.T) {
	dm := &v1.DBManager{}
	dm.Start()
	dm.Stop()

	ch, err := dm.QueryAsync(context.Background(), "SELECT 1")

	assert.Nil(t, ch)
	assert.ErrorIs(t, err, v1.ErrManagerClosed)
}

func TestDBManagerConcurrentStopAndAsyncCalls(t *testing.T) {
	dm := &v1.DBManager{}
	dm.Start()

	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, err := dm.QueryAsync(context.Background(), "SELECT 1")
			assert.Nil(t, ch)
			assert.Error(t, err)
			if err != nil {
				assert.True(t,
					errors.Is(err, v1.ErrManagerClosed) ||
						errors.Is(err, v1.ErrManagerNotStarted) ||
						err.Error() == "no read-only database entries available",
				)
			}
		}()
	}

	dm.Stop()
	wg.Wait()
	dm.Stop()
}

// TestDBManagerGetNoDBs tests Get returns error when no databases configured.
func TestDBManagerGetNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.GetAsync(ctx, "users", []string{"id", "name"}, nil, nil, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerGetByIDNoDBs tests GetByID returns error when no databases configured.
func TestDBManagerGetByIDNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.GetByIDAsync(ctx, "users", 1, nil, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerGetRawNoDBs tests GetRaw returns error when no databases configured.
func TestDBManagerGetRawNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.GetRawAsync(ctx, "users", []string{"id", "name"}, nil, nil, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerGetByIDRawNoDBs tests GetByIDRaw returns error when no databases.
func TestDBManagerGetByIDRawNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.GetByIDRawAsync(ctx, "users", 1, nil, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerInsertNoDBs tests Insert returns error when no write databases.
func TestDBManagerInsertNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	data := map[string]any{"name": "John"}

	ch, err := dm.InsertAsync(ctx, "users", data, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerInsertsNoDBs tests Inserts returns error when no write databases.
func TestDBManagerInsertsNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	bulkData := []map[string]any{
		{"name": "Alice"},
		{"name": "Bob"},
	}

	ch, err := dm.InsertsAsync(ctx, "users", bulkData, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerUpsertNoDBs tests Upsert returns error when no write databases.
func TestDBManagerUpsertNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	data := map[string]any{"email": "john@example.com", "name": "John"}
	upsertOpts := &options.UpsertOptions{
		ConflictColumns: []string{"email"},
		Action:          options.UpsertDoUpdate,
		UpdateColumns:   []string{"name"},
	}

	ch, err := dm.UpsertAsync(ctx, "users", data, upsertOpts, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerUpdateNoDBs tests Update returns error when no write databases.
func TestDBManagerUpdateNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()
	data := map[string]any{"name": "Jane"}

	ch, err := dm.UpdateAsync(ctx, "users", data, nil, nil, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerDeleteNoDBs tests Delete returns error when no write databases.
func TestDBManagerDeleteNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.DeleteAsync(ctx, "users", nil, nil, nil)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerQueryNoDBs tests Query returns error when no databases.
func TestDBManagerQueryNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.QueryAsync(ctx, "SELECT * FROM users")

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerQueryRawNoDBs tests QueryRaw returns error when no databases.
func TestDBManagerQueryRawNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.QueryRawAsync(ctx, "SELECT * FROM users")

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerExecNoDBs tests Exec returns error when no write databases.
func TestDBManagerExecNoDBs(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	ch, err := dm.ExecAsync(ctx, "INSERT INTO users VALUES (?, ?)", "John", 30)

	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerContextCancellation tests that canceled context is handled gracefully.
func TestDBManagerContextCancellation(t *testing.T) {
	dm := &v1.DBManager{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should handle canceled context without panicking
	ch, err := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	// With no DBs configured, should immediately return error
	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerContextTimeout tests that timed out context is handled gracefully.
func TestDBManagerContextTimeout(t *testing.T) {
	dm := &v1.DBManager{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	ch, err := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	// With no DBs configured, should immediately return error
	assert.Nil(t, ch)
	assert.Error(t, err)
}

// TestDBManagerMultipleOperations tests that manager can handle multiple sequential operations.
func TestDBManagerMultipleOperations(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Execute multiple operations in sequence
	// With no DBs configured, all should return errors
	ch1, err1 := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	ch2, err2 := dm.InsertAsync(ctx, "users", map[string]any{}, nil)
	ch3, err3 := dm.UpsertAsync(ctx, "users", map[string]any{}, nil, nil)
	ch4, err4 := dm.UpdateAsync(ctx, "users", map[string]any{}, nil, nil, nil)
	ch5, err5 := dm.DeleteAsync(ctx, "users", nil, nil, nil)
	ch6, err6 := dm.QueryAsync(ctx, "SELECT * FROM users")

	assert.Nil(t, ch1)
	assert.Nil(t, ch2)
	assert.Nil(t, ch3)
	assert.Nil(t, ch4)
	assert.Nil(t, ch5)
	assert.Nil(t, ch6)

	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Error(t, err3)
	assert.Error(t, err4)
	assert.Error(t, err5)
	assert.Error(t, err6)
}

// TestDBManagerReadOnlyVsReadWrite tests that read operations and write operations return proper errors when no DBs exist.
func TestDBManagerReadOnlyVsReadWrite(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Read operations - all should return error with no DBs
	ch1, err1 := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	assert.Nil(t, ch1)
	assert.Error(t, err1)

	ch2, err2 := dm.GetRawAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	assert.Nil(t, ch2)
	assert.Error(t, err2)

	ch3, err3 := dm.GetByIDAsync(ctx, "users", 1, nil, nil)
	assert.Nil(t, ch3)
	assert.Error(t, err3)

	ch4, err4 := dm.GetByIDRawAsync(ctx, "users", 1, nil, nil)
	assert.Nil(t, ch4)
	assert.Error(t, err4)

	ch5, err5 := dm.QueryAsync(ctx, "SELECT * FROM users")
	assert.Nil(t, ch5)
	assert.Error(t, err5)

	ch6, err6 := dm.QueryRawAsync(ctx, "SELECT * FROM users")
	assert.Nil(t, ch6)
	assert.Error(t, err6)

	// Write operations - all should return error with no DBs
	ch7, err7 := dm.InsertAsync(ctx, "users", map[string]any{}, nil)
	assert.Nil(t, ch7)
	assert.Error(t, err7)

	ch8, err8 := dm.InsertsAsync(ctx, "users", []map[string]any{}, nil)
	assert.Nil(t, ch8)
	assert.Error(t, err8)

	ch9, err9 := dm.UpsertAsync(ctx, "users", map[string]any{}, nil, nil)
	assert.Nil(t, ch9)
	assert.Error(t, err9)

	ch10, err10 := dm.UpdateAsync(ctx, "users", map[string]any{}, nil, nil, nil)
	assert.Nil(t, ch10)
	assert.Error(t, err10)

	ch11, err11 := dm.DeleteAsync(ctx, "users", nil, nil, nil)
	assert.Nil(t, ch11)
	assert.Error(t, err11)

	ch12, err12 := dm.ExecAsync(ctx, "INSERT INTO users VALUES (?, ?)", "test")
	assert.Nil(t, ch12)
	assert.Error(t, err12)
}

// TestDBManagerQueryParameters tests that query parameters are properly passed.
func TestDBManagerQueryParameters(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// Test Query with multiple parameters - should return error with no DBs
	ch, err := dm.QueryAsync(ctx, "SELECT * FROM users WHERE id = ? AND name = ?", 1, "John")
	assert.Nil(t, ch)
	assert.Error(t, err)

	// Test Exec with multiple parameters - should return error with no DBs
	execCh, execErr := dm.ExecAsync(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", 1, "John", 30)
	assert.Nil(t, execCh)
	assert.Error(t, execErr)
}

// TestDBManagerChannelNonBlocking tests that operations return errors immediately when no DBs exist.
func TestDBManagerChannelNonBlocking(t *testing.T) {
	dm := &v1.DBManager{}
	ctx := context.Background()

	// These should all return immediately with errors when no DBs are configured
	ch1, err1 := dm.GetAsync(ctx, "users", []string{"id"}, nil, nil, nil)
	ch2, err2 := dm.InsertAsync(ctx, "users", map[string]any{}, nil)
	ch3, err3 := dm.QueryAsync(ctx, "SELECT * FROM users")

	// All channels should be nil when no DBs exist
	assert.Nil(t, ch1)
	assert.Nil(t, ch2)
	assert.Nil(t, ch3)

	// All should return errors
	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Error(t, err3)
}
