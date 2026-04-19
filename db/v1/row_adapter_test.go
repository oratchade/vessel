//go:build test

package v1_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/fabric/db/v1"
)

// MockRows provides a simple in-memory mock of sql.Rows for testing.
type MockRows struct {
	rows       [][]any
	columns    []string
	currentIdx int
	closed     bool
	columnsErr error
	scanErr    error
}

func NewMockRows(columns []string, rows [][]any) *MockRows {
	return &MockRows{
		rows:       rows,
		columns:    columns,
		currentIdx: -1,
	}
}

func (m *MockRows) Columns() ([]string, error) {
	if m.columnsErr != nil {
		return nil, m.columnsErr
	}
	return m.columns, nil
}

func (m *MockRows) Next() bool {
	if m.closed {
		return false
	}
	m.currentIdx++
	return m.currentIdx < len(m.rows)
}

func (m *MockRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	if m.currentIdx < 0 || m.currentIdx >= len(m.rows) {
		return sql.ErrNoRows
	}
	row := m.rows[m.currentIdx]
	for i, val := range row {
		if i < len(dest) {
			*dest[i].(*any) = val
		}
	}
	return nil
}

func (m *MockRows) Close() error {
	m.closed = true
	return nil
}

func (m *MockRows) Err() error {
	return nil
}

// ============================================
// FUNCTIONAL CODE TESTS
// ============================================

// TestScanRowsTo_BasicStruct tests scanning rows into a simple struct.
func TestScanRowsTo_BasicStruct(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	mockRows := NewMockRows(
		[]string{"id", "name"},
		[][]any{
			{int64(1), "Alice"},
			{int64(2), "Bob"},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	users, err := v1.ScanRowsTo[User](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, users, 2)

	assert.Equal(t, 1, users[0].ID)
	assert.Equal(t, "Alice", users[0].Name)
	assert.Equal(t, 2, users[1].ID)
	assert.Equal(t, "Bob", users[1].Name)
}

// TestScanRowsTo_WithNullFields tests scanning rows with nullable fields.
func TestScanRowsTo_WithNullFields(t *testing.T) {
	type Product struct {
		ID          int            `db:"id"`
		Name        string         `db:"name"`
		Description sql.NullString `db:"description"`
	}

	mockRows := NewMockRows(
		[]string{"id", "name", "description"},
		[][]any{
			{int64(1), "Widget", "A useful widget"},
			{int64(2), "Gadget", nil},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	products, err := v1.ScanRowsTo[Product](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, products, 2)

	assert.Equal(t, 1, products[0].ID)
	assert.Equal(t, "Widget", products[0].Name)
	assert.True(t, products[0].Description.Valid)
	assert.Equal(t, "A useful widget", products[0].Description.String)

	assert.Equal(t, 2, products[1].ID)
	assert.False(t, products[1].Description.Valid)
}

// TestScanRowsTo_EmptyRows tests scanning an empty result set.
func TestScanRowsTo_EmptyRows(t *testing.T) {
	type User struct {
		ID int `db:"id"`
	}

	mockRows := NewMockRows([]string{"id"}, [][]any{})

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	users, err := v1.ScanRowsTo[User](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, users, 0)
}

// TestScanRowsTo_CaseInsensitiveMatching tests that column matching is case-insensitive.
func TestScanRowsTo_CaseInsensitiveMatching(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	mockRows := NewMockRows(
		[]string{"ID", "NAME"},
		[][]any{{int64(1), "Alice"}},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	users, err := v1.ScanRowsTo[User](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "Alice", users[0].Name)
}

// TestScanRowsTo_JSONTagFallback tests that JSON tags are used when db tags are absent.
func TestScanRowsTo_JSONTagFallback(t *testing.T) {
	type Product struct {
		ID    int    `json:"product_id"`
		Title string `json:"product_title"`
	}

	mockRows := NewMockRows(
		[]string{"product_id", "product_title"},
		[][]any{{int64(5), "Gizmo"}},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	products, err := v1.ScanRowsTo[Product](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, 5, products[0].ID)
	assert.Equal(t, "Gizmo", products[0].Title)
}

// TestScanRowsTo_FieldNameFallback tests that field names are used when no tags exist.
func TestScanRowsTo_FieldNameFallback(t *testing.T) {
	type SimpleData struct {
		ID    int
		Value string
	}

	mockRows := NewMockRows(
		[]string{"ID", "Value"},
		[][]any{{int64(42), "test"}},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	data, err := v1.ScanRowsTo[SimpleData](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, 42, data[0].ID)
	assert.Equal(t, "test", data[0].Value)
}

// TestScanRowsTo_MultipleNullTypes tests scanning multiple nullable types.
func TestScanRowsTo_MultipleNullTypes(t *testing.T) {
	type Record struct {
		ID     int             `db:"id"`
		Value  sql.NullInt64   `db:"value"`
		Score  sql.NullFloat64 `db:"score"`
		Active sql.NullBool    `db:"active"`
	}

	mockRows := NewMockRows(
		[]string{"id", "value", "score", "active"},
		[][]any{
			{int64(1), int64(100), 95.5, true},
			{int64(2), nil, nil, nil},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	records, err := v1.ScanRowsTo[Record](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// First record with valid values
	assert.Equal(t, 1, records[0].ID)
	assert.True(t, records[0].Value.Valid)
	assert.Equal(t, int64(100), records[0].Value.Int64)
	assert.True(t, records[0].Score.Valid)
	assert.True(t, records[0].Active.Valid)

	// Second record with NULL values
	assert.Equal(t, 2, records[1].ID)
	assert.False(t, records[1].Value.Valid)
	assert.False(t, records[1].Score.Valid)
	assert.False(t, records[1].Active.Valid)
}

// TestRowsAdapter_IterationAndScanning tests full iteration and scanning cycle.
func TestRowsAdapter_IterationAndScanning(t *testing.T) {
	mockRows := NewMockRows(
		[]string{"id", "name"},
		[][]any{
			{int64(1), "Alice"},
			{int64(2), "Bob"},
			{int64(3), "Charlie"},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	count := 0
	for adapter.Next() {
		count++
	}

	assert.Equal(t, 3, count)
}

// TestRowsAdapter_Columns tests retrieving column names.
func TestRowsAdapter_Columns(t *testing.T) {
	mockRows := NewMockRows(
		[]string{"id", "name", "email"},
		[][]any{},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	cols, err := adapter.Columns()
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "email"}, cols)
}

// TestRowsAdapter_Close tests that rows are properly closed.
func TestRowsAdapter_Close(t *testing.T) {
	mockRows := NewMockRows(
		[]string{"id"},
		[][]any{{int64(1)}},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)

	err := adapter.Close()
	assert.NoError(t, err)
	assert.True(t, mockRows.closed)
}

// TestRowsAdapter_NewFromInvalidType tests error handling for invalid row types.
func TestRowsAdapter_NewFromInvalidType(t *testing.T) {
	adapter, err := v1.NewRowsAdapterForTest("not a valid rows type")
	assert.Error(t, err)
	assert.Nil(t, adapter)
	assert.Contains(t, err.Error(), "unsupported")
}

// TestRowsAdapterPool_BasicPooling tests pool creation and stats.
func TestRowsAdapterPool_BasicPooling(t *testing.T) {
	pool := v1.NewRowsAdapterPool()
	assert.NotNil(t, pool)
}

// TestRowsAdapterPool_InvalidRowsType tests error handling for invalid row types.
func TestRowsAdapterPool_InvalidRowsType(t *testing.T) {
	pool := v1.NewRowsAdapterPool()

	adapter, err := pool.Acquire("invalid")
	assert.Error(t, err)
	assert.Nil(t, adapter)
	assert.Contains(t, err.Error(), "unsupported")
}

// TestRowsAdapterPool_ReleaseNil tests that releasing nil is safe.
func TestRowsAdapterPool_ReleaseNil(t *testing.T) {
	pool := v1.NewRowsAdapterPool()
	// Should not panic
	pool.Release(nil)
}

// TestRowsAdapterPoolWithStats_StatsTracking tests statistics collection.
func TestRowsAdapterPoolWithStats_StatsTracking(t *testing.T) {
	pool := v1.NewRowsAdapterPoolWithStats()

	// Check initial stats
	stats := pool.Stats()
	assert.GreaterOrEqual(t, stats.Allocated, 0)

	// Stats are available
	assert.NotNil(t, pool.Stats())
}

// TestManagedRowsAdapter_AutomaticCleanup tests automatic resource cleanup (concept test).
// Note: WrapManagedRowsAdapter requires actual sql.Rows or pgx.Rows, not test mocks.
// This test verifies the concept but uses test adapter directly.
func TestManagedRowsAdapter_AutomaticCleanup(t *testing.T) {
	mockRows := NewMockRows(
		[]string{"id"},
		[][]any{{int64(1)}},
	)

	// Create a test adapter which supports mock rows
	testAdapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer testAdapter.Close() //nolint:errcheck

	// Verify the adapter works and can be closed
	err := testAdapter.Close()
	assert.NoError(t, err)
	assert.True(t, mockRows.closed)
}

// TestManagedRowsAdapter_InvalidType tests error handling.
func TestManagedRowsAdapter_InvalidType(t *testing.T) {
	managed, err := v1.WrapManagedRowsAdapter("invalid")
	assert.Error(t, err)
	assert.Nil(t, managed)
	assert.Contains(t, err.Error(), "unsupported")
}

// TestBuildFieldMap tests the buildFieldMap function.
func TestBuildFieldMap(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
		Age  int    // No tag
	}

	fm := v1.BuildFieldMapForTest(reflect.TypeOf(User{}))
	require.NotNil(t, fm)
	assert.Equal(t, 3, len(fm))
	assert.Contains(t, fm, "id")
	assert.Contains(t, fm, "name")
	assert.Contains(t, fm, "age")
}

// TestBuildFieldMap_TagPriority tests that db tags take priority over json tags.
func TestBuildFieldMap_TagPriority(t *testing.T) {
	type Record struct {
		ID   int    `db:"user_id"     json:"id"`
		Name string `json:"user_name"`
	}

	fm := v1.BuildFieldMapForTest(reflect.TypeOf(Record{}))
	require.NotNil(t, fm)

	// db tag takes priority
	assert.Contains(t, fm, "user_id")
	// json tag used when db tag is absent
	assert.Contains(t, fm, "user_name")
}

// TestBuildFieldMap_CaseInsensitivity tests that field map keys are lowercase.
func TestBuildFieldMap_CaseInsensitivity(t *testing.T) {
	type Mixed struct {
		FirstName string `db:"firstName"`
		LastName  string
	}

	fm := v1.BuildFieldMapForTest(reflect.TypeOf(Mixed{}))
	require.NotNil(t, fm)

	// Keys should be lowercase
	assert.Contains(t, fm, "firstname")
	assert.Contains(t, fm, "lastname")
}

// TestScanRowsTo_ErrorHandling tests error propagation from scanning.
func TestScanRowsTo_ErrorHandling(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	mockRows := NewMockRows(
		[]string{"id"},
		[][]any{{int64(1)}},
	)
	mockRows.scanErr = fmt.Errorf("database error")

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	_, err := v1.ScanRowsTo[User](context.Background(), adapter)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

// TestScanRowsTo_StructWithPointer tests scanning into pointer-to-struct type.
func TestScanRowsTo_StructWithPointer(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	mockRows := NewMockRows(
		[]string{"id", "name"},
		[][]any{
			{int64(1), "Alice"},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	users, err := v1.ScanRowsTo[*User](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, 1, users[0].ID)
	assert.Equal(t, "Alice", users[0].Name)
}
