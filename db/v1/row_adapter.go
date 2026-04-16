// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
//
// The v1 package includes database connection management, query building, row scanning,
// error handling, and logging. It supports multiple database engines (MySQL, PostgreSQL,
// SQLite, MSSQL) with a unified API.
//
// Key types:
//   - DB: Main database connection interface
//   - FluentDB: Builder-style API for constructing queries
//   - RowsAdapter: Unified row scanning interface across database drivers
//
// Note: RowsAdapter should always be properly closed to avoid connection leaks:
//
//	rows, err := db.Query(ctx, ...)
//	if err != nil { ... }
//	defer rows.Close()  // IMPORTANT: Always close rows when done
package v1

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
)

// fieldMapCache stores a global cache of reflect.Type to field map mappings.
// This improves performance by avoiding repeated reflection work for the same struct type.
type fieldMapCache struct {
	mu sync.RWMutex
	m  map[reflect.Type]map[string]int
}

// globalFieldMapCache is a singleton cache for field maps.
//
//nolint:gochecknoglobals
var globalFieldMapCache = &fieldMapCache{
	m: make(map[reflect.Type]map[string]int),
}

// get retrieves a cached field map for the given type, or builds and caches it if not found.
func (fmc *fieldMapCache) get(tType reflect.Type) map[string]int {
	// Fast path: try read lock first
	fmc.mu.RLock()
	if fieldMap, ok := fmc.m[tType]; ok {
		fmc.mu.RUnlock()
		return fieldMap
	}
	fmc.mu.RUnlock()

	// Slow path: build and cache
	fieldMap := buildFieldMap(tType)

	// Write to cache with write lock
	fmc.mu.Lock()
	// Double-check after acquiring write lock
	if existing, ok := fmc.m[tType]; ok {
		fmc.mu.Unlock()
		return existing
	}
	fmc.m[tType] = fieldMap
	fmc.mu.Unlock()

	return fieldMap
}

// RowsAdapter provides a unified wrapper around driver-specific Rows types for a consistent API.
//
// It abstracts over both sql.Rows (MySQL, PostgreSQL, SQLite, MSSQL) and pgx.Rows (PostgreSQL pgx driver),
// enabling the same iteration and scanning code to work with any supported database driver.
//
// Memory Management:
// RowsAdapter holds references to database connections through the underlying Rows.
// To prevent connection leaks, always call Close() before the adapter goes out of scope:
//
//	rows, err := db.GetRaw(ctx, ...)
//	if err != nil { return err }
//
//	adapter, err := db.NewRowsAdapter(rows)
//	if err != nil { return err }
//	defer adapter.Close()  // CRITICAL: Always close
//
//	// Use adapter for iteration and scanning
//	for adapter.Next() {
//		if err := adapter.Scan(...); err != nil { ... }
//	}
type RowsAdapter struct {
	sqlRows *sql.Rows
	pgxRows pgx.Rows
}

// newRowsAdapter creates a RowsAdapter from either *sql.Rows or pgx.Rows.
func newRowsAdapter(rows any) (*RowsAdapter, error) {
	ra := &RowsAdapter{}
	switch r := rows.(type) {
	case *sql.Rows:
		ra.sqlRows = r
	case pgx.Rows:
		ra.pgxRows = r
	default:
		return nil, fmt.Errorf("RowsAdapter: unsupported rows type: %T", rows)
	}
	return ra, nil
}

// columns returns the column names from the underlying rows.
func (r *RowsAdapter) columns() ([]string, error) {
	if r.sqlRows != nil {
		c, err := r.sqlRows.Columns()
		if err != nil {
			return nil, fmt.Errorf("RowsAdapter.columns: %w", err)
		}
		return c, nil
	}
	if r.pgxRows != nil {
		fds := r.pgxRows.FieldDescriptions()
		cols := make([]string, len(fds))
		for i, fd := range fds {
			cols[i] = fd.Name
		}
		return cols, nil
	}
	return nil, fmt.Errorf("RowsAdapter.columns: no rows available")
}

// next advances to the next row in the result set.
func (r *RowsAdapter) next() bool {
	if r.sqlRows != nil {
		return r.sqlRows.Next()
	}
	return r.pgxRows.Next()
}

// scan scans values from the current row into the provided destinations.
func (r *RowsAdapter) scan(dest ...any) error {
	var err error
	if r.sqlRows != nil {
		err = r.sqlRows.Scan(dest...)
		if err != nil {
			return fmt.Errorf("RowsAdapter.scan: %w", err)
		}
		return nil
	}
	err = r.pgxRows.Scan(dest...)
	if err != nil {
		return fmt.Errorf("RowsAdapter.scan: %w", err)
	}
	return nil
}

// err returns any error that occurred during row iteration.
func (r *RowsAdapter) err() error {
	if r.sqlRows != nil {
		if sqlErr := r.sqlRows.Err(); sqlErr != nil {
			return fmt.Errorf("RowsAdapter.err: %w", sqlErr)
		}
		return nil
	}
	if pgxErr := r.pgxRows.Err(); pgxErr != nil {
		return fmt.Errorf("RowsAdapter.err: %w", pgxErr)
	}
	return nil
}

// close releases the underlying database resources and prevents connection leaks.
// Must be called before the adapter goes out of scope to return connections to the pool.
func (r *RowsAdapter) close() error {
	if r.sqlRows != nil {
		err := r.sqlRows.Close()
		if err != nil {
			return fmt.Errorf("RowsAdapter.Close: %w", err)
		}
		return nil
	}
	if r.pgxRows != nil {
		r.pgxRows.Close()
	}
	return nil
}

// scanRows scans all rows from the result set into a slice of maps.
func scanRows(rows any, cols []string) ([]map[string]any, error) {
	// use RowsAdapter to unify *sql.Rows and pgx.Rows handling
	ra, err := newRowsAdapter(rows)
	if err != nil {
		return nil, err
	}

	// obtain columns if not provided
	if len(cols) == 0 {
		if cols, err = ra.columns(); err != nil {
			return nil, fmt.Errorf("scanRows: failed to get columns: %w", err)
		}
	}

	results := make([]map[string]any, 0)
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	for ra.next() {
		if err := ra.scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scanRows: failed to scan row: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = v
			}
		}
		results = append(results, row)
	}

	if err := ra.err(); err != nil {
		return nil, fmt.Errorf("scanRows: rows iteration failed: %w", err)
	}

	return results, nil
}

// buildFieldMap builds a mapping from lower-cased column name to struct field index.
func buildFieldMap(tType reflect.Type) map[string]int {
	fieldMap := map[string]int{}
	for i := 0; i < tType.NumField(); i++ {
		f := tType.Field(i)
		name := f.Tag.Get("db")
		if name == "" {
			name = f.Tag.Get("json")
		}
		if name == "" {
			name = f.Name
		}
		fieldMap[strings.ToLower(name)] = i
	}
	return fieldMap
}

// makeScanPtrs prepares slices for scanning rows into generic destinations.
func makeScanPtrs(n int) (vals []any, ptrs []any) {
	vals = make([]any, n)
	ptrs = make([]any, n)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	return vals, ptrs
}

// setSQLNullField attempts to set a sql.Null* field with the given value.
// Supports sql.NullString, sql.NullInt64, sql.NullBool, sql.NullFloat64, sql.NullByte, and sql.NullTime.
// setSQLNullStringField sets a sql.NullString field.
func setSQLNullStringField(cv any) sql.NullString {
	var ns sql.NullString
	if cv == nil {
		return ns
	}

	// convert []byte to string if needed
	if b, ok := cv.([]byte); ok {
		cv = string(b)
	}

	if s, ok := cv.(string); ok {
		ns.String = s
	} else {
		ns.String = fmt.Sprint(cv)
	}
	ns.Valid = true
	return ns
}

// setSQLNullInt64Field sets a sql.NullInt64 field.
func setSQLNullInt64Field(cv any) sql.NullInt64 {
	var ni sql.NullInt64
	if cv == nil {
		return ni
	}

	s := fmt.Sprint(cv)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		ni.Int64 = n
		ni.Valid = true
	}
	return ni
}

// setSQLNullFloat64Field sets a sql.NullFloat64 field.
func setSQLNullFloat64Field(cv any) sql.NullFloat64 {
	var nf sql.NullFloat64
	if cv == nil {
		return nf
	}

	s := fmt.Sprint(cv)
	if f64, err := strconv.ParseFloat(s, 64); err == nil {
		nf.Float64 = f64
		nf.Valid = true
	}
	return nf
}

// setSQLNullBoolField sets a sql.NullBool field.
func setSQLNullBoolField(cv any) sql.NullBool {
	var nb sql.NullBool
	if cv == nil {
		return nb
	}

	s := fmt.Sprint(cv)
	if b, err := strconv.ParseBool(s); err == nil {
		nb.Bool = b
		nb.Valid = true
	}
	return nb
}

// setSQLNullByteField sets a sql.NullByte field.
func setSQLNullByteField(cv any) sql.NullByte {
	var nb sql.NullByte
	if cv == nil {
		return nb
	}

	if b, ok := cv.(byte); ok {
		nb.Byte = b
		nb.Valid = true
	} else if b, ok := cv.([]byte); ok && len(b) > 0 {
		nb.Byte = b[0]
		nb.Valid = true
	}
	return nb
}

// setSQLNullTimeField sets a sql.NullTime field.
func setSQLNullTimeField(cv any) sql.NullTime {
	var nt sql.NullTime
	if cv == nil {
		return nt
	}

	// Try to unmarshal as time.Time
	if t, ok := cv.(sql.NullTime); ok {
		return t
	}

	if err := json.Unmarshal([]byte(fmt.Sprint(cv)), &nt.Time); err == nil {
		nt.Valid = true
	}
	return nt
}

// setSQLNullField attempts to set a sql.Null* field with the given value.
// Supports sql.NullString, sql.NullInt64, sql.NullBool, sql.NullFloat64, sql.NullByte, and sql.NullTime.
func setSQLNullField(f reflect.Value, cv any) error {
	switch f.Type() {
	case reflect.TypeOf(sql.NullString{}):
		f.Set(reflect.ValueOf(setSQLNullStringField(cv)))
		return nil
	case reflect.TypeOf(sql.NullInt64{}):
		f.Set(reflect.ValueOf(setSQLNullInt64Field(cv)))
		return nil
	case reflect.TypeOf(sql.NullFloat64{}):
		f.Set(reflect.ValueOf(setSQLNullFloat64Field(cv)))
		return nil
	case reflect.TypeOf(sql.NullBool{}):
		f.Set(reflect.ValueOf(setSQLNullBoolField(cv)))
		return nil
	case reflect.TypeOf(sql.NullByte{}):
		f.Set(reflect.ValueOf(setSQLNullByteField(cv)))
		return nil
	case reflect.TypeOf(sql.NullTime{}):
		f.Set(reflect.ValueOf(setSQLNullTimeField(cv)))
		return nil
	}

	return fmt.Errorf("setSQLNullField: unsupported sql.Null* type: %v", f.Type())
}

// isSQLNullType checks if the given type is a sql.Null* type.
func isSQLNullType(t reflect.Type) bool {
	switch t {
	case reflect.TypeOf(sql.NullString{}),
		reflect.TypeOf(sql.NullInt64{}),
		reflect.TypeOf(sql.NullFloat64{}),
		reflect.TypeOf(sql.NullBool{}),
		reflect.TypeOf(sql.NullByte{}),
		reflect.TypeOf(sql.NullTime{}):
		return true
	}
	return false
}

// setFieldFromValue attempts to set reflect.Value `f` from the generic value `cv`.
//
//nolint:cyclop
func setFieldFromValue(f reflect.Value, cv any) error {
	if cv == nil {
		// Handle nil for sql.Null* types - set with Valid=false
		if isSQLNullType(f.Type()) {
			return setSQLNullField(f, nil)
		}
		return nil
	}

	// Handle sql.Null* types first
	if isSQLNullType(f.Type()) {
		return setSQLNullField(f, cv)
	}

	// convert []byte to string for common DB drivers
	if b, ok := cv.([]byte); ok {
		cv = string(b)
	}

	rv := reflect.ValueOf(cv)
	if rv.IsValid() {
		if rv.Type().AssignableTo(f.Type()) {
			f.Set(rv)
			return nil
		}
		if rv.Type().ConvertibleTo(f.Type()) {
			f.Set(rv.Convert(f.Type()))
			return nil
		}
	}

	// fallback: try parsing from string representation
	s := fmt.Sprint(cv)
	switch f.Kind() {
	case reflect.String:
		f.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			f.SetInt(n)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			f.SetUint(n)
		}
	case reflect.Float32, reflect.Float64:
		if f64, err := strconv.ParseFloat(s, 64); err == nil {
			f.SetFloat(f64)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(s); err == nil {
			f.SetBool(b)
		}
	default:
		// try JSON unmarshal for complex types
		err := json.Unmarshal([]byte(s), f.Addr().Interface())
		if err != nil {
			return fmt.Errorf("setFieldFromValue: failed to unmarshal JSON for field of type %s: %w", f.Type(), err)
		}
	}

	return nil
}

// GetFieldMapCacheForTest returns the global field map cache for testing purposes.
// This function is only available during tests.
func GetFieldMapCacheForTest() *fieldMapCacheTest {
	return &fieldMapCacheTest{cache: globalFieldMapCache}
}

// fieldMapCacheTest is a test wrapper around fieldMapCache that exposes the Get method.
type fieldMapCacheTest struct {
	cache *fieldMapCache
}

// Get retrieves a field map from the cache, building and caching it if necessary.
func (fct *fieldMapCacheTest) Get(tType reflect.Type) map[string]int {
	return fct.cache.get(tType)
}

// BuildFieldMapForTest exposes the buildFieldMap function for testing.
func BuildFieldMapForTest(tType reflect.Type) map[string]int {
	return buildFieldMap(tType)
}
