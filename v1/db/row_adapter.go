package db

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RowsAdapter provides a small wrapper around driver-specific Rows types so
// callers can use a unified API (Next, Scan, Err, Columns).
type rowsAdapter struct {
	sqlRows *sql.Rows
	pgxRows pgx.Rows
}

func newRowsAdapter(rows any) (*rowsAdapter, error) {
	ra := &rowsAdapter{}
	switch r := rows.(type) {
	case *sql.Rows:
		ra.sqlRows = r
	case pgx.Rows:
		ra.pgxRows = r
	default:
		return nil, fmt.Errorf("rowsAdapter: unsupported rows type: %T", rows)
	}
	return ra, nil
}

func (r *rowsAdapter) columns() ([]string, error) {
	if r.sqlRows != nil {
		c, err := r.sqlRows.Columns()
		if err != nil {
			return nil, fmt.Errorf("rowsAdapter.columns: %w", err)
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
	return nil, fmt.Errorf("rowsAdapter.columns: no rows available")
}

func (r *rowsAdapter) next() bool {
	if r.sqlRows != nil {
		return r.sqlRows.Next()
	}
	return r.pgxRows.Next()
}

func (r *rowsAdapter) scan(dest ...any) error {
	var err error
	if r.sqlRows != nil {
		err = r.sqlRows.Scan(dest...)
		if err != nil {
			return fmt.Errorf("rowsAdapter.scan: %w", err)
		}
		return nil
	}
	err = r.pgxRows.Scan(dest...)
	if err != nil {
		return fmt.Errorf("rowsAdapter.scan: %w", err)
	}
	return nil
}

func (r *rowsAdapter) err() error {
	if r.sqlRows != nil {
		return fmt.Errorf("rowsAdapter.err: %w", r.sqlRows.Err())
	}
	return fmt.Errorf("rowsAdapter.err: %w", r.pgxRows.Err())
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

// // buildFieldMap builds a mapping from lower-cased column name to struct field index.
// func buildFieldMap(tType reflect.Type) map[string]int {
// 	fieldMap := map[string]int{}
// 	for i := 0; i < tType.NumField(); i++ {
// 		f := tType.Field(i)
// 		name := f.Tag.Get("db")
// 		if name == "" {
// 			name = f.Tag.Get("json")
// 		}
// 		if name == "" {
// 			name = f.Name
// 		}
// 		fieldMap[strings.ToLower(name)] = i
// 	}
// 	return fieldMap
// }

// // makeScanPtrs prepares slices for scanning rows into generic destinations.
// func makeScanPtrs(n int) (vals []any, ptrs []any) {
// 	vals = make([]any, n)
// 	ptrs = make([]any, n)
// 	for i := range vals {
// 		ptrs[i] = &vals[i]
// 	}
// 	return vals, ptrs
// }

// // setFieldFromValue attempts to set reflect.Value `f` from the generic value `cv`.
// func setFieldFromValue(f reflect.Value, cv any) {
// 	if cv == nil {
// 		return
// 	}

// 	// convert []byte to string for common DB drivers
// 	if b, ok := cv.([]byte); ok {
// 		cv = string(b)
// 	}

// 	rv := reflect.ValueOf(cv)
// 	if rv.IsValid() {
// 		if rv.Type().AssignableTo(f.Type()) {
// 			f.Set(rv)
// 			return
// 		}
// 		if rv.Type().ConvertibleTo(f.Type()) {
// 			f.Set(rv.Convert(f.Type()))
// 			return
// 		}
// 	}

// 	// fallback: try parsing from string representation
// 	s := fmt.Sprint(cv)
// 	switch f.Kind() {
// 	case reflect.String:
// 		f.SetString(s)
// 	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
// 		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
// 			f.SetInt(n)
// 		}
// 	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
// 		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
// 			f.SetUint(n)
// 		}
// 	case reflect.Float32, reflect.Float64:
// 		if f64, err := strconv.ParseFloat(s, 64); err == nil {
// 			f.SetFloat(f64)
// 		}
// 	case reflect.Bool:
// 		if b, err := strconv.ParseBool(s); err == nil {
// 			f.SetBool(b)
// 		}
// 	default:
// 		// try JSON unmarshal for complex types
// 		_ = json.Unmarshal([]byte(s), f.Addr().Interface())
// 	}
// }

// func scanRowsTo[T any](rows any, cols []string) ([]T, error) {
// 	// use RowsAdapter to unify *sql.Rows and pgx.Rows handling
// 	ra, err := newRowsAdapter(rows)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// obtain columns if not provided
// 	if len(cols) == 0 {
// 		if cols, err = ra.columns(); err != nil {
// 			return nil, fmt.Errorf("scanRowsTo: failed to get columns: %w", err)
// 		}
// 	}

// 	var out []T

// 	// prepare scan destinations
// 	vals, ptrs := makeScanPtrs(len(cols))

// 	// reflect type information for T
// 	tType := reflect.TypeOf((*T)(nil)).Elem()
// 	isPtr := false
// 	if tType.Kind() == reflect.Ptr {
// 		isPtr = true
// 		tType = tType.Elem()
// 	}
// 	if tType.Kind() != reflect.Struct {
// 		return nil, fmt.Errorf("scanRowsTo: T must be a struct or pointer to struct")
// 	}

// 	// build mapping column -> struct field index
// 	fieldMap := buildFieldMap(tType)

// 	for ra.next() {
// 		if err := ra.scan(ptrs...); err != nil {
// 			return nil, fmt.Errorf("scan: %w", err)
// 		}

// 		var itemVal reflect.Value
// 		var itemPtr reflect.Value
// 		if isPtr {
// 			itemPtr = reflect.New(tType)
// 			itemVal = itemPtr.Elem()
// 		} else {
// 			itemVal = reflect.New(tType).Elem()
// 		}

// 		for i, col := range cols {
// 			raw := vals[i]
// 			if raw == nil {
// 				continue
// 			}
// 			colKey := strings.ToLower(col)
// 			if fi, ok := fieldMap[colKey]; ok {
// 				f := itemVal.Field(fi)
// 				if !f.CanSet() {
// 					continue
// 				}
// 				setFieldFromValue(f, raw)
// 			}
// 		}

// 		if isPtr {
// 			out = append(out, itemPtr.Interface().(T))
// 		} else {
// 			out = append(out, itemVal.Interface().(T))
// 		}
// 	}

// 	if err := ra.err(); err != nil {
// 		return nil, fmt.Errorf("scanRowsTo: rows iteration failed: %w", err)
// 	}
// 	return out, nil
// }
