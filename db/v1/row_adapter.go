package v1

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RowsAdapter provides a unified wrapper around driver-specific Rows types for a consistent API.
//
// It abstracts over sql.Rows (MySQL, PostgreSQL, SQLite, MSSQL), pgx.Rows (PostgreSQL pgx driver),
// and any custom RowsProvider implementation (including plugin-provided custom rows types),
// enabling the same iteration and scanning code to work with any supported rows source.
//
// Plugin Support:
// Custom database drivers can implement RowsProvider to enable their custom rows types.
// See rows_provider.go and the RowsProvider interface for the required method contract.
//
// Memory Management:
// RowsAdapter holds references to database connections through the underlying RowsProvider.
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
	provider RowsProvider
}

// initRowsAdapterSource initializes the adapter with the provided rows.
// Supports *sql.Rows, pgx.Rows, or any custom RowsProvider implementation.
// This centralizes the type switch logic to avoid duplication in newRowsAdapter and RowsAdapterPool.Acquire.
func (ra *RowsAdapter) initRowsAdapterSource(rows any) error {
	switch r := rows.(type) {
	case *sql.Rows:
		ra.provider = &sqlRowsProvider{rows: r}
	case pgx.Rows:
		ra.provider = &pgxRowsProvider{rows: r}
	case RowsProvider:
		// Plugin or custom implementation
		ra.provider = r
	default:
		return fmt.Errorf("RowsAdapter: unsupported rows type: %T", rows)
	}
	return nil
}

// reset clears the provider for reuse in the pool.
// Centralizes reset logic to ensure consistency between Release and other cleanup paths.
func (ra *RowsAdapter) reset() {
	ra.provider = nil
}

// newRowsAdapter creates a RowsAdapter from either *sql.Rows or pgx.Rows.
func newRowsAdapter(rows any) (*RowsAdapter, error) {
	ra := &RowsAdapter{}
	if err := ra.initRowsAdapterSource(rows); err != nil {
		return nil, err
	}
	return ra, nil
}

// columns returns the column names from the underlying rows.
func (r *RowsAdapter) columns() ([]string, error) {
	if r.provider == nil {
		return nil, fmt.Errorf("RowsAdapter.columns: no provider available")
	}
	return r.provider.columns()
}

// Columns returns the column names from the underlying rows.
// This is the public version of the private columns() method.
func (r *RowsAdapter) Columns() ([]string, error) {
	return r.columns()
}

// next advances to the next row in the result set.
func (r *RowsAdapter) next() bool {
	if r.provider == nil {
		return false
	}
	return r.provider.next()
}

// Next advances to the next row in the result set.
// This is the public version of the private next() method.
func (r *RowsAdapter) Next() bool {
	return r.next()
}

// scan scans values from the current row into the provided destinations.
func (r *RowsAdapter) scan(dest ...any) error {
	if r.provider == nil {
		return fmt.Errorf("RowsAdapter.scan: no provider available")
	}
	return r.provider.scan(dest...)
}

// Scan scans values from the current row into the provided destinations.
// This is the public version of the private scan() method.
func (r *RowsAdapter) Scan(dest ...any) error {
	return r.scan(dest...)
}

// err returns any error that occurred during row iteration.
func (r *RowsAdapter) err() error {
	if r.provider == nil {
		return fmt.Errorf("RowsAdapter.err: no provider available")
	}
	return r.provider.err()
}

// Err returns any error that occurred during row iteration.
// This is the public version of the private err() method.
func (r *RowsAdapter) Err() error {
	return r.err()
}

// close releases the underlying database resources and prevents connection leaks.
// Must be called before the adapter goes out of scope to return connections to the pool.
func (r *RowsAdapter) close() error {
	if r.provider == nil {
		return nil
	}
	return r.provider.close()
}

// Close releases the underlying database resources and prevents connection leaks.
// This is the public version of the private close() method.
// Must be called before the adapter goes out of scope to return connections to the pool.
func (r *RowsAdapter) Close() error {
	return r.close()
}
