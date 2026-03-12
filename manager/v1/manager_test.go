//go:build test

package v1_test

import (
	"testing"

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
