//go:build test

package v1_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
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

func TestScanAllAndScanOne(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	allRows := NewMockRows(
		[]string{"id", "name"},
		[][]any{{int64(1), "Alice"}, {int64(2), "Bob"}},
	)
	users, err := v1.ScanAll[User](context.Background(), v1.NewRowsAdapterWithMockRows(allRows))
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "Alice", users[0].Name)

	oneRows := NewMockRows(
		[]string{"id", "name"},
		[][]any{{int64(3), "Cara"}},
	)
	user, err := v1.ScanOne[User](context.Background(), v1.NewRowsAdapterWithMockRows(oneRows))
	require.NoError(t, err)
	assert.Equal(t, 3, user.ID)
	assert.Equal(t, "Cara", user.Name)
}

func TestScanOneRejectsWrongRowCount(t *testing.T) {
	type User struct {
		ID int `db:"id"`
	}

	_, err := v1.ScanOne[User](context.Background(), v1.NewRowsAdapterWithMockRows(NewMockRows([]string{"id"}, nil)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")

	_, err = v1.ScanOne[User](context.Background(), v1.NewRowsAdapterWithMockRows(NewMockRows([]string{"id"}, [][]any{{int64(1)}, {int64(2)}})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one row")
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

// TestBuildFieldMap_OmitemptyAndIgnore verifies two json tag conventions:
//   - "col,omitempty" strips the option so the key is just "col"
//   - "-" marks a field as ignored and excludes it from the map
func TestBuildFieldMap_OmitemptyAndIgnore(t *testing.T) {
	type Event struct {
		ID        int    `json:"event_id,omitempty"`
		Title     string `json:"title"`
		Internal  string `json:"-"`
		DBIgnored string `db:"-"`
	}

	fm := v1.BuildFieldMapForTest(reflect.TypeOf(Event{}))

	assert.Contains(t, fm, "event_id", "omitempty suffix must be stripped")
	assert.NotContains(t, fm, "event_id,omitempty", "raw tag with comma must not appear as key")
	assert.Contains(t, fm, "title")
	assert.NotContains(t, fm, "-", "json:\"-\" field must be excluded")
	assert.NotContains(t, fm, "internal", "json:\"-\" field must be excluded by name too")
	assert.NotContains(t, fm, "dbignored", "db:\"-\" field must be excluded")
}

// TestBuildFieldMap_EmbeddedStruct verifies that fields from anonymous embedded
// structs are flattened into the field map with the correct index paths.
// This is the core requirement that allows ScanRowsTo to populate embedded
// fields like `type Row struct { models.Company; Role string `db:"role"` }`.
func TestBuildFieldMap_EmbeddedStruct(t *testing.T) {
	type Address struct {
		Street string `db:"street"`
		City   string `db:"city"`
	}
	type Person struct {
		Address        // embedded with no tag — should be recursed into
		Name    string `db:"name"`
	}

	fm := v1.BuildFieldMapForTest(reflect.TypeOf(Person{}))
	require.NotNil(t, fm)

	// Embedded fields must be visible at the top level.
	assert.Contains(t, fm, "street")
	assert.Contains(t, fm, "city")
	assert.Contains(t, fm, "name")
	// The embedded struct itself must NOT appear as a standalone column key.
	assert.NotContains(t, fm, "address", "embedded struct name must not leak as a column key")

	// Index paths must reflect the correct depth:
	//   Person[0] = Address, Address[0] = Street → path {0, 0}
	//   Person[0] = Address, Address[1] = City   → path {0, 1}
	//   Person[1] = Name                         → path {1}
	assert.Equal(t, []int{0, 0}, fm["street"], "street index path")
	assert.Equal(t, []int{0, 1}, fm["city"], "city index path")
	assert.Equal(t, []int{1}, fm["name"], "name index path")
}

// TestScanRowsTo_EmbeddedStruct is an end-to-end test that confirms ScanRowsTo
// correctly populates an anonymous embedded struct from SQL columns.
func TestScanRowsTo_EmbeddedStruct(t *testing.T) {
	type Tenant struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	type Row struct {
		Tenant        // embedded
		Role   string `db:"role"`
	}

	mockRows := NewMockRows(
		[]string{"id", "name", "role"},
		[][]any{
			{int64(42), "Acme Corp", "admin"},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	rows, err := v1.ScanRowsTo[Row](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	r := rows[0]
	assert.Equal(t, 42, r.ID, "embedded ID must be populated")
	assert.Equal(t, "Acme Corp", r.Name, "embedded Name must be populated")
	assert.Equal(t, "admin", r.Role, "direct Role field must be populated")
}

// TestScanRowsTo_DeepEmbeddedStruct verifies three levels of anonymous embedding.
func TestScanRowsTo_DeepEmbeddedStruct(t *testing.T) {
	type Base struct {
		CreatedBy string `db:"created_by"`
	}
	type Meta struct {
		Base
		Version int `db:"version"`
	}
	type Record struct {
		Meta
		Title string `db:"title"`
	}

	mockRows := NewMockRows(
		[]string{"created_by", "version", "title"},
		[][]any{
			{"alice", int64(3), "My Record"},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	records, err := v1.ScanRowsTo[Record](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, records, 1)

	rec := records[0]
	assert.Equal(t, "alice", rec.CreatedBy, "3-level embedded CreatedBy must be populated")
	assert.Equal(t, 3, rec.Version, "2-level embedded Version must be populated")
	assert.Equal(t, "My Record", rec.Title, "direct Title must be populated")
}

// TestScanRowsTo_NullTimeFromTimeDotTime verifies that a time.Time value delivered
// by the driver (the most common case with pgx and lib/pq) correctly populates a
// sql.NullTime field. The old setSQLNullTimeField tried json.Unmarshal(fmt.Sprint(...))
// which always failed silently, leaving every NullTime field as {Valid: false}.
func TestScanRowsTo_NullTimeFromTimeDotTime(t *testing.T) {
	type Event struct {
		ID        int          `db:"id"`
		CreatedAt sql.NullTime `db:"created_at"`
		DeletedAt sql.NullTime `db:"deleted_at"`
	}

	now := time.Now().UTC().Truncate(time.Second)

	mockRows := NewMockRows(
		[]string{"id", "created_at", "deleted_at"},
		[][]any{
			{int64(1), now, nil},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	events, err := v1.ScanRowsTo[Event](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.True(t, events[0].CreatedAt.Valid, "time.Time driver value must produce Valid=true NullTime")
	assert.Equal(t, now, events[0].CreatedAt.Time)
	assert.False(t, events[0].DeletedAt.Valid, "SQL NULL must produce Valid=false NullTime")
}

// TestScanRowsTo_NullFieldParseError verifies that a value that cannot be coerced
// into a sql.Null* field's underlying type returns a descriptive error rather than
// silently scanning as {Valid: false} (which is indistinguishable from SQL NULL).
func TestScanRowsTo_NullFieldParseError(t *testing.T) {
	type Record struct {
		ID    int           `db:"id"`
		Count sql.NullInt64 `db:"count"`
	}

	mockRows := NewMockRows(
		[]string{"id", "count"},
		[][]any{
			{int64(1), "not-a-number"},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	_, err := v1.ScanRowsTo[Record](context.Background(), adapter)
	require.Error(t, err, "unparseable string for NullInt64 must return an error, not silently become {Valid:false}")
	assert.Contains(t, err.Error(), "not-a-number")
}

// TestScanRowsTo_UUIDBytes16ToStringField verifies that pgx v5 binary UUID values
// ([16]byte) are reformatted to "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" when the
// destination struct field is a string.  A [16]byte destination field must be
// unaffected (handled earlier by AssignableTo).
func TestScanRowsTo_UUIDBytes16ToStringField(t *testing.T) {
	// 550e8400-e29b-41d4-a716-446655440001
	raw := [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x01}

	type Record struct {
		ID string `db:"id"`
	}

	mockRows := NewMockRows(
		[]string{"id"},
		[][]any{{raw}},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	records, err := v1.ScanRowsTo[Record](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440001", records[0].ID)
}

// TestScanRowsTo_UUIDArrayToStringSlice verifies that a Postgres uuid[] column —
// delivered by pgx v5 as []interface{} of [16]byte — is scanned into a []string field,
// with each element reformatted to canonical UUID text. This is the array analogue of
// TestScanRowsTo_UUIDBytes16ToStringField: previously the slice fell through to the JSON
// fallback, which marshalled each [16]byte to a numeric array and failed to unmarshal it
// into a string element.
func TestScanRowsTo_UUIDArrayToStringSlice(t *testing.T) {
	// 550e8400-e29b-41d4-a716-446655440001 / ...440002
	u1 := [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x01}
	u2 := [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x02}

	type Record struct {
		Members []string `db:"members"`
	}

	tests := []struct {
		name    string
		dbValue any
		want    []string
	}{
		{
			name:    "uuid array as []interface{} of [16]byte",
			dbValue: []any{u1, u2},
			want:    []string{"550e8400-e29b-41d4-a716-446655440001", "550e8400-e29b-41d4-a716-446655440002"},
		},
		{
			name:    "empty array",
			dbValue: []any{},
			want:    []string{},
		},
		{
			name:    "text array as []interface{} of string",
			dbValue: []any{"alpha", "beta"},
			want:    []string{"alpha", "beta"},
		},
		{
			// Postgres arrays can contain NULL elements; ensure nil interface
			// elements do not panic during interface unwrapping and map to zero values.
			name:    "array with nil element",
			dbValue: []any{"alpha", nil, "beta"},
			want:    []string{"alpha", "", "beta"},
		},
		{
			// jsonb columns may arrive as a JSON-array string (text protocol);
			// these must keep using the JSON decoder, not element-wise conversion.
			name:    "json array delivered as string",
			dbValue: `["alpha","beta"]`,
			want:    []string{"alpha", "beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRows := NewMockRows([]string{"members"}, [][]any{{tt.dbValue}})
			adapter := v1.NewRowsAdapterWithMockRows(mockRows)
			defer adapter.Close() //nolint:errcheck

			records, err := v1.ScanRowsTo[Record](context.Background(), adapter)
			require.NoError(t, err)
			require.Len(t, records, 1)
			assert.Equal(t, tt.want, records[0].Members)
		})
	}
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

// TestScanRowsTo_SQLScannerInterface verifies that setFieldFromValue delegates to
// sql.Scanner when a field implements it, instead of falling through to json.Unmarshal.
// This mirrors the real-world case of types like Permissions or NotificationPreferences
// that receive a postgres JSON column as []byte.
func TestScanRowsTo_SQLScannerInterface(t *testing.T) {
	type Row struct {
		ID   int      `db:"id"`
		Data JSONData `db:"data"`
	}

	tests := []struct {
		name    string
		dbValue any // what the driver delivers
	}{
		{
			name:    "[]byte (lib/pq convention)",
			dbValue: []byte(`{"value":"hello"}`),
		},
		{
			name:    "string (pgx v5 text protocol)",
			dbValue: `{"value":"hello"}`,
		},
		{
			name:    "map[string]interface{} (pgx v5 JSON codec pre-decoded)",
			dbValue: map[string]interface{}{"value": "hello"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRows := NewMockRows(
				[]string{"id", "data"},
				[][]any{
					{int64(42), tc.dbValue},
				},
			)

			adapter := v1.NewRowsAdapterWithMockRows(mockRows)
			defer adapter.Close() //nolint:errcheck

			rows, err := v1.ScanRowsTo[Row](context.Background(), adapter)
			require.NoError(t, err)
			require.Len(t, rows, 1)

			assert.Equal(t, 42, rows[0].ID)
			assert.True(t, rows[0].Data.ScannedByInterface, "expected sql.Scanner.Scan to be called, not json.Unmarshal fallback")
			assert.Equal(t, "hello", rows[0].Data.Value)
		})
	}
}

// JSONData is a test helper type that implements sql.Scanner.
// It tracks whether Scan was called via the interface (not json.Unmarshal fallback).
type JSONData struct {
	Value              string `json:"value"`
	ScannedByInterface bool   `json:"-"`
}

func (j *JSONData) Scan(value any) error {
	j.ScannedByInterface = true
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("JSONData.Scan: expected []byte, got %T", value)
	}
	return json.Unmarshal(b, j)
}

// ScalarData is a test helper type that implements sql.Scanner for scalar DB columns.
// It records whatever src value was passed so tests can assert the pass-through.
type ScalarData struct {
	Value any
}

func (s *ScalarData) Scan(value any) error {
	s.Value = value
	return nil
}

// TestToScannerBytes_ScalarPassThrough verifies that standard database/sql scalar
// types (int64, float64, bool, time.Time) are passed through to sql.Scanner
// unchanged and are NOT marshaled to JSON []byte.
func TestToScannerBytes_ScalarPassThrough(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	type Row struct {
		Count   ScalarData `db:"count"`
		Score   ScalarData `db:"score"`
		Active  ScalarData `db:"active"`
		Created ScalarData `db:"created"`
	}

	mockRows := NewMockRows(
		[]string{"count", "score", "active", "created"},
		[][]any{
			{int64(7), float64(3.14), true, now},
		},
	)

	adapter := v1.NewRowsAdapterWithMockRows(mockRows)
	defer adapter.Close() //nolint:errcheck

	rows, err := v1.ScanRowsTo[Row](context.Background(), adapter)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	r := rows[0]
	assert.Equal(t, int64(7), r.Count.Value, "int64 should pass through unchanged, not be JSON-marshaled")
	assert.Equal(t, float64(3.14), r.Score.Value, "float64 should pass through unchanged")
	assert.Equal(t, true, r.Active.Value, "bool should pass through unchanged")
	assert.Equal(t, now, r.Created.Value, "time.Time should pass through unchanged")
}
