package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// RowsAdapter provides a small wrapper around driver-specific Rows types so
// callers can use a unified API (Next, Scan, Err, Columns).
type RowsAdapter struct {
	sqlRows *sql.Rows
	pgxRows pgx.Rows
}

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
			cols[i] = fmt.Sprint(fd.Name)
		}
		return cols, nil
	}
	return nil, fmt.Errorf("RowsAdapter.columns: no rows available")
}

func (r *RowsAdapter) next() bool {
	if r.sqlRows != nil {
		return r.sqlRows.Next()
	}
	return r.pgxRows.Next()
}

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

func (r *RowsAdapter) err() error {
	if r.sqlRows != nil {
		return fmt.Errorf("RowsAdapter.err: %w", r.sqlRows.Err())
	}
	return fmt.Errorf("RowsAdapter.err: %w", r.pgxRows.Err())
}

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

// setFieldFromValue attempts to set reflect.Value `f` from the generic value `cv`.
//
//nolint:cyclop
func setFieldFromValue(f reflect.Value, cv any) error {
	if cv == nil {
		return nil
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
