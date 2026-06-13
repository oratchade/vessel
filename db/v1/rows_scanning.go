package v1

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// fieldMapCache stores a global cache of reflect.Type to field map mappings.
// This improves performance by avoiding repeated reflection work for the same struct type.
type fieldMapCache struct {
	mu sync.RWMutex
	m  map[reflect.Type]map[string][]int
}

// globalFieldMapCache is a singleton cache for field maps.
//
//nolint:gochecknoglobals
var globalFieldMapCache = &fieldMapCache{
	m: make(map[reflect.Type]map[string][]int),
}

// get retrieves a cached field map for the given type, or builds and caches it if not found.
func (fmc *fieldMapCache) get(tType reflect.Type) map[string][]int {
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

// buildFieldMap builds a mapping from lower-cased column name to struct field index path.
// Anonymous (embedded) structs are recursed into so their fields appear at the top level,
// mirroring how encoding/json handles embedded structs.
func buildFieldMap(tType reflect.Type) map[string][]int {
	fieldMap := map[string][]int{}
	collectFields(tType, nil, fieldMap)
	return fieldMap
}

// fieldName resolves the column name for f from db/json tags or the field name itself.
// Tag options such as "col,omitempty" are stripped. "-" is returned as-is so the
// caller can decide whether to skip the field.
func fieldName(f reflect.StructField) string {
	name := f.Tag.Get("db")
	if name == "" {
		name = f.Tag.Get("json")
	}
	if name == "" {
		name = f.Name
	}
	if idx := strings.IndexByte(name, ','); idx != -1 {
		name = name[:idx]
	}
	return name
}

// tryEmbedded handles an anonymous (embedded) field during field collection.
// If the field is an embedded struct without an explicit db tag, it recurses into
// the embedded type and returns true (handled). Returns false to signal that the
// field should be treated as a normal named column.
func tryEmbedded(f reflect.StructField, path []int, fieldMap map[string][]int) bool {
	if !f.Anonymous {
		return false
	}
	ft := f.Type
	if ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	if ft.Kind() != reflect.Struct {
		return false
	}
	dbTag := f.Tag.Get("db")
	if dbTag == "-" {
		return true // skip entirely
	}
	if dbTag == "" {
		collectFields(ft, path, fieldMap)
		return true
	}
	// Explicit non-empty db tag on an embedded struct: treat it as a named column.
	return false
}

// collectFields walks tType recursively, accumulating index paths into fieldMap.
// prefix is the index path from the outermost struct to the current level.
func collectFields(tType reflect.Type, prefix []int, fieldMap map[string][]int) {
	for i := 0; i < tType.NumField(); i++ {
		f := tType.Field(i)

		// Build the full index path for this field.
		path := make([]int, len(prefix)+1)
		copy(path, prefix)
		path[len(prefix)] = i

		// Anonymous (embedded) struct without an explicit db tag: recurse into it
		// so its columns appear as top-level keys, same as encoding/json behavior.
		if tryEmbedded(f, path, fieldMap) {
			continue
		}

		name := fieldName(f)
		// db:"-" or json:"-" marks a field as explicitly ignored.
		if name == "-" {
			continue
		}
		fieldMap[strings.ToLower(name)] = path
	}
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

// setSQLNullStringField converts a DB driver value to sql.NullString.
func setSQLNullStringField(cv any) sql.NullString {
	if cv == nil {
		return sql.NullString{}
	}
	switch v := cv.(type) {
	case string:
		return sql.NullString{String: v, Valid: true}
	case []byte:
		return sql.NullString{String: string(v), Valid: true}
	default:
		return sql.NullString{String: fmt.Sprint(v), Valid: true}
	}
}

// cvToInt64 converts an arbitrary driver value to int64.
// Integer types are reflected directly; strings and []byte are parsed;
// other types are formatted with fmt.Sprint then parsed.
func cvToInt64(cv any) (int64, error) {
	switch cv := cv.(type) {
	case int64, int32, int16, int8, int:
		return reflect.ValueOf(cv).Int(), nil
	case float64, float32:
		return int64(reflect.ValueOf(cv).Float()), nil
	case string:
		n, err := strconv.ParseInt(cv, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as int64: %w", cv, err)
		}
		return n, nil
	case []byte:
		s := string(cv)
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as int64: %w", s, err)
		}
		return n, nil
	default:
		s := fmt.Sprint(cv)
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert %T to int64: %w", cv, err)
		}
		return n, nil
	}
}

// setSQLNullInt64Field converts a DB driver value to sql.NullInt64.
// Returns an error when the value cannot be coerced to int64.
func setSQLNullInt64Field(cv any) (sql.NullInt64, error) {
	if cv == nil {
		return sql.NullInt64{}, nil
	}
	n, err := cvToInt64(cv)
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("setSQLNullInt64Field: %w", err)
	}
	return sql.NullInt64{Int64: n, Valid: true}, nil
}

// cvToFloat64 converts an arbitrary driver value to float64.
// Float/integer types are reflected directly; strings and []byte are parsed;
// other types are formatted with fmt.Sprint then parsed.
func cvToFloat64(cv any) (float64, error) {
	switch cv := cv.(type) {
	case float64, float32:
		return reflect.ValueOf(cv).Float(), nil
	case int64, int32:
		return float64(reflect.ValueOf(cv).Int()), nil
	case string:
		f, err := strconv.ParseFloat(cv, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as float64: %w", cv, err)
		}
		return f, nil
	case []byte:
		s := string(cv)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as float64: %w", s, err)
		}
		return f, nil
	default:
		f, err := strconv.ParseFloat(fmt.Sprint(cv), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert %T to float64: %w", cv, err)
		}
		return f, nil
	}
}

// setSQLNullFloat64Field converts a DB driver value to sql.NullFloat64.
// Returns an error when the value cannot be coerced to float64.
func setSQLNullFloat64Field(cv any) (sql.NullFloat64, error) {
	if cv == nil {
		return sql.NullFloat64{}, nil
	}
	f, err := cvToFloat64(cv)
	if err != nil {
		return sql.NullFloat64{}, fmt.Errorf("setSQLNullFloat64Field: %w", err)
	}
	return sql.NullFloat64{Float64: f, Valid: true}, nil
}

// setSQLNullBoolField sets a sql.NullBool from a DB value.
// Returns an error when the value cannot be coerced to bool.
func setSQLNullBoolField(cv any) (sql.NullBool, error) {
	if cv == nil {
		return sql.NullBool{}, nil
	}
	switch v := cv.(type) {
	case bool:
		return sql.NullBool{Bool: v, Valid: true}, nil
	case int64:
		// Treat non-zero as true, matching SQL convention for boolean-like integer columns.
		return sql.NullBool{Bool: v != 0, Valid: true}, nil
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return sql.NullBool{}, fmt.Errorf("setSQLNullBoolField: cannot parse %q as bool: %w", v, err)
		}
		return sql.NullBool{Bool: b, Valid: true}, nil
	case []byte:
		b, err := strconv.ParseBool(string(v))
		if err != nil {
			return sql.NullBool{}, fmt.Errorf("setSQLNullBoolField: cannot parse %q as bool: %w", string(v), err)
		}
		return sql.NullBool{Bool: b, Valid: true}, nil
	default:
		b, err := strconv.ParseBool(fmt.Sprint(v))
		if err != nil {
			return sql.NullBool{}, fmt.Errorf("setSQLNullBoolField: cannot convert %T to bool: %w", cv, err)
		}
		return sql.NullBool{Bool: b, Valid: true}, nil
	}
}

// setSQLNullByteField sets a sql.NullByte from a DB value.
// Returns an error when the value cannot be coerced to a single byte.
func setSQLNullByteField(cv any) (sql.NullByte, error) {
	if cv == nil {
		return sql.NullByte{}, nil
	}
	switch v := cv.(type) {
	case byte: // same as uint8
		return sql.NullByte{Byte: v, Valid: true}, nil
	case []byte:
		if len(v) == 0 {
			return sql.NullByte{}, fmt.Errorf("setSQLNullByteField: empty []byte for single-byte field")
		}
		return sql.NullByte{Byte: v[0], Valid: true}, nil
	default:
		return sql.NullByte{}, fmt.Errorf("setSQLNullByteField: cannot convert %T to byte", cv)
	}
}

// parseRFC3339 tries RFC3339Nano then RFC3339 layouts.
// Returns the parsed time and true on success, or zero value and false on failure.
func parseRFC3339(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// setSQLNullTimeField converts a DB driver value to sql.NullTime.
// Returns an error when the value cannot be coerced to time.Time.
func setSQLNullTimeField(cv any) (sql.NullTime, error) {
	if cv == nil {
		return sql.NullTime{}, nil
	}
	switch v := cv.(type) {
	case time.Time:
		// Most common case: driver delivers time.Time directly (pgx, lib/pq).
		return sql.NullTime{Time: v, Valid: true}, nil
	case sql.NullTime:
		return v, nil
	case string:
		// RFC3339Nano first to preserve sub-second precision; fall back to RFC3339.
		if t, ok := parseRFC3339(v); ok {
			return sql.NullTime{Time: t, Valid: true}, nil
		}
		return sql.NullTime{}, fmt.Errorf("setSQLNullTimeField: cannot parse %q as time.Time (expected RFC3339)", v)
	case []byte:
		if t, ok := parseRFC3339(string(v)); ok {
			return sql.NullTime{Time: t, Valid: true}, nil
		}
		return sql.NullTime{}, fmt.Errorf(
			"setSQLNullTimeField: cannot parse %q as time.Time (expected RFC3339)", string(v))
	default:
		return sql.NullTime{}, fmt.Errorf("setSQLNullTimeField: cannot convert %T to time.Time", cv)
	}
}

// setSQLNullBasic handles sql.NullString, sql.NullInt64, and sql.NullFloat64.
// Returns (true, err) if the type was handled, or (false, nil) to fall through.
func setSQLNullBasic(f reflect.Value, cv any) (bool, error) {
	switch f.Type() {
	case reflect.TypeOf(sql.NullString{}):
		f.Set(reflect.ValueOf(setSQLNullStringField(cv)))
	case reflect.TypeOf(sql.NullInt64{}):
		v, err := setSQLNullInt64Field(cv)
		if err != nil {
			return true, err
		}
		f.Set(reflect.ValueOf(v))
	case reflect.TypeOf(sql.NullFloat64{}):
		v, err := setSQLNullFloat64Field(cv)
		if err != nil {
			return true, err
		}
		f.Set(reflect.ValueOf(v))
	default:
		return false, nil
	}
	return true, nil
}

// setSQLNullField attempts to set a sql.Null* field with the given value.
// Supports sql.NullString, sql.NullInt64, sql.NullBool, sql.NullFloat64, sql.NullByte, and sql.NullTime.
func setSQLNullField(f reflect.Value, cv any) error {
	if handled, err := setSQLNullBasic(f, cv); handled {
		return err
	}
	switch f.Type() {
	case reflect.TypeOf(sql.NullBool{}):
		v, err := setSQLNullBoolField(cv)
		if err != nil {
			return err
		}
		f.Set(reflect.ValueOf(v))
	case reflect.TypeOf(sql.NullByte{}):
		v, err := setSQLNullByteField(cv)
		if err != nil {
			return err
		}
		f.Set(reflect.ValueOf(v))
	case reflect.TypeOf(sql.NullTime{}):
		v, err := setSQLNullTimeField(cv)
		if err != nil {
			return err
		}
		f.Set(reflect.ValueOf(v))
	default:
		return fmt.Errorf("setSQLNullField: unsupported sql.Null* type: %v", f.Type())
	}
	return nil
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

// toScannerBytes normalizes a DB column value before dispatching to sql.Scanner.
//
// The database/sql spec says Scan's src argument will be one of:
//
//	int64, float64, bool, []byte, string, time.Time, or nil.
//
// Real drivers deviate from this in two ways that we need to absorb here:
//   - pgx/v5 text protocol  → delivers string for JSON/JSONB columns;
//     we normalize to []byte so Scanner implementations follow the []byte convention.
//   - pgx/v5 JSON codec     → delivers pre-decoded map[string]interface{} or
//     []interface{}; we re-marshal to canonical JSON []byte.
//
// All other standard database/sql types (int64, float64, bool, time.Time, []byte)
// are passed through unchanged so Scanner implementations that follow the spec
// receive the expected native type rather than a JSON-marshaled []byte.
func toScannerBytes(cv any) (any, error) {
	switch v := cv.(type) {
	case []byte:
		// Standard: pass through as-is.
		return v, nil
	case string:
		// pgx/v5 text protocol delivers string for JSON columns; normalize to []byte
		// so JSON Scanner implementations receive the standard []byte convention.
		return []byte(v), nil
	case int64, int32, int16, int8, int,
		uint64, uint32, uint16, uint8, uint,
		float64, float32,
		bool,
		time.Time:
		// Standard database/sql scalar types — pass through unchanged.
		return cv, nil
	default:
		// Non-standard types produced by pgx/v5's JSON codec (map[string]interface{},
		// []interface{}, etc.). Re-marshal to canonical JSON []byte so Scanner
		// implementations receive the standard []byte JSON convention.
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("toScannerBytes: failed to marshal %T to JSON: %w", cv, err)
		}
		return b, nil
	}
}

// setNullable handles sql.Null* fields for any cv value (including nil).
// Returns (true, err) if handled, (false, nil) for non-nullable field types.
func setNullable(f reflect.Value, cv any) (bool, error) {
	if !isSQLNullType(f.Type()) {
		return false, nil
	}
	return true, setSQLNullField(f, cv)
}

// tryScanner attempts to call sql.Scanner on the field if it implements the interface.
// Returns (true, err) if handled, (false, nil) otherwise.
func tryScanner(f reflect.Value, cv any) (bool, error) {
	if !f.CanAddr() {
		return false, nil
	}
	scanner, ok := f.Addr().Interface().(sql.Scanner)
	if !ok {
		return false, nil
	}
	scanVal, err := toScannerBytes(cv)
	if err != nil {
		return true, fmt.Errorf("setFieldFromValue: normalizing value for sql.Scanner: %w", err)
	}
	if err := scanner.Scan(scanVal); err != nil {
		return true, fmt.Errorf("setFieldFromValue: sql.Scanner.Scan: %w", err)
	}
	return true, nil
}

// setStringField sets a reflect.String field from cv.
// pgx v5 delivers UUID columns (OID 2950) as [16]byte in the binary protocol;
// only reformat to UUID string when the destination is a string field.
func setStringField(f reflect.Value, cv any, s string) error {
	if arr, ok := cv.([16]byte); ok {
		f.SetString(fmt.Sprintf("%x-%x-%x-%x-%x", arr[0:4], arr[4:6], arr[6:8], arr[8:10], arr[10:16]))
		return nil
	}
	f.SetString(s)
	return nil
}

func setIntField(f reflect.Value, s string) error {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("setFieldFromValue: cannot parse %q as int for field %s: %w", s, f.Type(), err)
	}
	f.SetInt(n)
	return nil
}

func setUintField(f reflect.Value, s string) error {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("setFieldFromValue: cannot parse %q as uint for field %s: %w", s, f.Type(), err)
	}
	f.SetUint(n)
	return nil
}

func setFloatField(f reflect.Value, s string) error {
	f64, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("setFieldFromValue: cannot parse %q as float for field %s: %w", s, f.Type(), err)
	}
	f.SetFloat(f64)
	return nil
}

func setBoolField(f reflect.Value, s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("setFieldFromValue: cannot parse %q as bool for field %s: %w", s, f.Type(), err)
	}
	f.SetBool(b)
	return nil
}

func setPtrField(f reflect.Value, cv any) error {
	elem := reflect.New(f.Type().Elem())
	if err := setFieldFromValue(elem.Elem(), cv); err != nil {
		return err
	}
	f.Set(elem)
	return nil
}

// setSliceField populates a slice field from cv.
//
// Postgres array columns (uuid[], text[], int[], …) are delivered by pgx/v5 as a Go
// []interface{} whose elements carry the driver's native per-element type — UUIDs, for
// instance, arrive as [16]byte. The JSON fallback cannot map those element types onto the
// destination's element type (it marshals a [16]byte to a numeric array, then fails to
// unmarshal that into a string), so we convert element-by-element through setFieldFromValue.
// This reuses every scalar/pointer/Scanner rule, including the [16]byte→UUID-string reformat
// in setStringField.
//
// Two cases deliberately bypass element-wise conversion and fall back to JSON:
//   - byte slices ([]byte), which round-trip as base64/raw bytes rather than element arrays;
//   - a cv that is not itself a slice/array (e.g. a JSON-array string from a jsonb column),
//     which the JSON decoder already handles correctly.
func setSliceField(f reflect.Value, cv any) error {
	if f.Type().Elem().Kind() == reflect.Uint8 {
		return setFieldByJSON(f, cv)
	}
	rv := reflect.ValueOf(cv)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return setFieldByJSON(f, cv)
	}

	out := reflect.MakeSlice(f.Type(), rv.Len(), rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		// Unwrap []interface{} elements to their concrete dynamic value so the recursive
		// call sees the driver's element type (e.g. [16]byte) rather than interface{}.
		if elem.Kind() == reflect.Interface {
			elem = elem.Elem()
		}
		var ev any
		if elem.IsValid() {
			ev = elem.Interface()
		}
		if err := setFieldFromValue(out.Index(i), ev); err != nil {
			return fmt.Errorf("setFieldFromValue: slice element %d for field of type %s: %w", i, f.Type(), err)
		}
	}
	f.Set(out)
	return nil
}

// setFieldByJSON JSON-unmarshals cv into f.
// When cv is not a string (e.g. map[string]interface{} from pgx/v5's JSON codec),
// it is marshaled to canonical JSON first; fmt.Sprint would produce invalid JSON.
func setFieldByJSON(f reflect.Value, cv any) error {
	if !f.CanAddr() {
		return fmt.Errorf("setFieldFromValue: cannot unmarshal into non-addressable field of type %s", f.Type())
	}
	var jsonBytes []byte
	if str, ok := cv.(string); ok {
		jsonBytes = []byte(str)
	} else {
		b, err := json.Marshal(cv)
		if err != nil {
			return fmt.Errorf("setFieldFromValue: cannot marshal %T to JSON for field type %s: %w", cv, f.Type(), err)
		}
		jsonBytes = b
	}
	if err := json.Unmarshal(jsonBytes, f.Addr().Interface()); err != nil {
		return fmt.Errorf("setFieldFromValue: failed to unmarshal JSON for field of type %s: %w", f.Type(), err)
	}
	return nil
}

// setFieldByKind dispatches cv to the appropriate setter based on f's kind.
// cv must not be nil and must not be []byte (both are handled upstream).
func setFieldByKind(f reflect.Value, cv any) error {
	s := fmt.Sprint(cv)
	switch f.Kind() {
	case reflect.String:
		return setStringField(f, cv, s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setIntField(f, s)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUintField(f, s)
	case reflect.Float32, reflect.Float64:
		return setFloatField(f, s)
	case reflect.Bool:
		return setBoolField(f, s)
	case reflect.Pointer:
		return setPtrField(f, cv)
	case reflect.Slice:
		return setSliceField(f, cv)
	default:
		return setFieldByJSON(f, cv)
	}
}

// setFieldFromValue attempts to set reflect.Value `f` from the generic value `cv`.
func setFieldFromValue(f reflect.Value, cv any) error {
	if handled, err := setNullable(f, cv); handled {
		return err
	}
	if cv == nil {
		return nil
	}
	if handled, err := tryScanner(f, cv); handled {
		return err
	}

	// Normalize []byte → string for the reflection and string-parse paths below.
	// (tryScanner has already handled the sql.Scanner path above.)
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

	// Last-resort coercion by field kind.
	return setFieldByKind(f, cv)
}
