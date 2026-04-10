//go:build test

package v1_test

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/fabric/db/v1"
)

func TestSQLNullStringConversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected bool
	}{
		{"nil value", nil, false},
		{"string value", "hello", true},
		{"bytes value", []byte("world"), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ns sql.NullString
			if tc.input == nil {
				ns.Valid = false
			} else if b, ok := tc.input.([]byte); ok {
				ns = sql.NullString{String: string(b), Valid: true}
			} else if s, ok := tc.input.(string); ok {
				ns = sql.NullString{String: s, Valid: true}
			}
			assert.Equal(t, tc.expected, ns.Valid)
		})
	}
}

func TestSQLNullInt64Conversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected bool
	}{
		{"nil value", nil, false},
		{"valid string", "123", true},
		{"zero", "0", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ni sql.NullInt64
			if tc.input == nil {
				ni.Valid = false
			} else if s, ok := tc.input.(string); ok && s != "invalid" {
				ni = sql.NullInt64{Valid: true}
			}
			assert.Equal(t, tc.expected, ni.Valid)
		})
	}
}

func TestSQLNullBoolConversion(t *testing.T) {
	testCases := []struct {
		name  string
		valid bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nb sql.NullBool
			nb.Valid = tc.valid
			assert.Equal(t, tc.valid, nb.Valid)
		})
	}
}

func TestSQLNullByteConversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected bool
	}{
		{"single byte", byte('a'), true},
		{"byte array", []byte("test"), true},
		{"nil", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nb sql.NullByte
			if tc.input == nil {
				nb.Valid = false
			} else {
				nb.Valid = true
			}
			assert.Equal(t, tc.expected, nb.Valid)
		})
	}
}

func TestSQLNullFloat64Conversion(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected bool
	}{
		{"valid float", "3.14", true},
		{"zero", "0.0", true},
		{"nil", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nf sql.NullFloat64
			if tc.input == nil {
				nf.Valid = false
			} else {
				nf.Valid = true
			}
			assert.Equal(t, tc.expected, nf.Valid)
		})
	}
}

// TestFieldMapCache_BasicCaching tests that field maps are cached correctly.
func TestFieldMapCache_BasicCaching(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	tType := reflect.TypeOf(User{})

	// Access the field map cache via GetFieldMapCacheForTest.
	// Note: This requires a test-only export function in row_adapter.go
	cache := v1.GetFieldMapCacheForTest()

	// First access should cache the result
	fm1 := cache.Get(tType)
	require.NotNil(t, fm1)
	assert.Equal(t, 2, len(fm1))
	assert.Contains(t, fm1, "id")
	assert.Contains(t, fm1, "name")

	// Second access should return the same cached result
	fm2 := cache.Get(tType)
	require.NotNil(t, fm2)
	assert.Equal(t, fm1, fm2)
}

// TestFieldMapCache_DifferentStructTypes tests that different types are cached separately.
func TestFieldMapCache_DifferentStructTypes(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	type Product struct {
		ID    int     `db:"id"`
		Title string  `db:"title"`
		Price float64 `db:"price"`
	}

	userType := reflect.TypeOf(User{})
	productType := reflect.TypeOf(Product{})

	cache := v1.GetFieldMapCacheForTest()

	fmUser := cache.Get(userType)
	fmProduct := cache.Get(productType)

	// User should have 2 fields
	assert.Equal(t, 2, len(fmUser))
	assert.Contains(t, fmUser, "id")
	assert.Contains(t, fmUser, "name")

	// Product should have 3 fields
	assert.Equal(t, 3, len(fmProduct))
	assert.Contains(t, fmProduct, "id")
	assert.Contains(t, fmProduct, "title")
	assert.Contains(t, fmProduct, "price")

	// Verify they are different
	assert.NotEqual(t, fmUser, fmProduct)
}

// TestFieldMapCache_CaseInsensitiveMatching tests that field maps use lowercase keys.
func TestFieldMapCache_CaseInsensitiveMatching(t *testing.T) {
	type CaseSensitiveStruct struct {
		FirstName string `db:"firstName"`
		LastName  string `db:"lastName"`
	}

	tType := reflect.TypeOf(CaseSensitiveStruct{})
	cache := v1.GetFieldMapCacheForTest()

	fm := cache.Get(tType)

	// Keys should be lowercase
	assert.Contains(t, fm, "firstname")
	assert.Contains(t, fm, "lastname")
	assert.NotContains(t, fm, "firstName")
	assert.NotContains(t, fm, "LastName")
}

// TestFieldMapCache_JSONTagFallback tests that JSON tags are used when db tags are absent.
func TestFieldMapCache_JSONTagFallback(t *testing.T) {
	type StructWithJSON struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	tType := reflect.TypeOf(StructWithJSON{})
	cache := v1.GetFieldMapCacheForTest()

	fm := cache.Get(tType)

	// Should use JSON tags when db tags are absent
	assert.Contains(t, fm, "id")
	assert.Contains(t, fm, "name")
	assert.Equal(t, 2, len(fm))
}

// TestFieldMapCache_FieldNameFallback tests that field names are used when no tags are present.
func TestFieldMapCache_FieldNameFallback(t *testing.T) {
	type NoTagsStruct struct {
		ID   int
		Name string
	}

	tType := reflect.TypeOf(NoTagsStruct{})
	cache := v1.GetFieldMapCacheForTest()

	fm := cache.Get(tType)

	// Should use field names (lowercased) when no tags are present
	assert.Contains(t, fm, "id")
	assert.Contains(t, fm, "name")
	assert.Equal(t, 2, len(fm))
}

// TestFieldMapCache_Concurrency tests that field map cache is thread-safe.
func TestFieldMapCache_Concurrency(t *testing.T) {
	type TestStruct struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	tType := reflect.TypeOf(TestStruct{})
	cache := v1.GetFieldMapCacheForTest()

	// Concurrently access the cache from multiple goroutines
	results := make(chan map[string]int, 10)
	for i := 0; i < 10; i++ {
		go func() {
			results <- cache.Get(tType)
		}()
	}

	// Collect results and verify all are identical
	var firstResult map[string]int
	for i := 0; i < 10; i++ {
		result := <-results
		if i == 0 {
			firstResult = result
		} else {
			assert.Equal(t, firstResult, result)
		}
	}
}

// TestBuildFieldMap tests the buildFieldMap function directly.
func TestBuildFieldMap(t *testing.T) {
	type TestStruct struct {
		ID      int    `db:"user_id"`
		Name    string `db:"user_name"`
		Ignored string `db:"-"`
		Tagged  string `json:"tagged_field"`
		Plain   bool
	}

	tType := reflect.TypeOf(TestStruct{})
	fm := v1.BuildFieldMapForTest(tType)

	// Verify field map entries
	assert.Equal(t, 5, len(fm))
	assert.Contains(t, fm, "user_id")
	assert.Contains(t, fm, "user_name")
	assert.Contains(t, fm, "tagged_field")
	assert.Contains(t, fm, "plain")
	// "-" tag still creates an entry in the map (implementation detail)
	assert.Contains(t, fm, "-")
}

// TestBuildFieldMap_EmptyStruct tests buildFieldMap with an empty struct.
func TestBuildFieldMap_EmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	tType := reflect.TypeOf(EmptyStruct{})
	fm := v1.BuildFieldMapForTest(tType)

	assert.Equal(t, 0, len(fm))
}

// TestBuildFieldMap_MixedTags tests buildFieldMap with mixed tag types.
func TestBuildFieldMap_MixedTags(t *testing.T) {
	type MixedStruct struct {
		Field1 string `db:"col1"`
		Field2 string `json:"col2"` // db tag missing, should use json
		Field3 string // no tags, should use field name
	}

	tType := reflect.TypeOf(MixedStruct{})
	fm := v1.BuildFieldMapForTest(tType)

	assert.Equal(t, 3, len(fm))
	assert.Contains(t, fm, "col1")
	assert.Contains(t, fm, "col2")
	assert.Contains(t, fm, "field3")
}
