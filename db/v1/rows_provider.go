package v1

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RowsProvider is the contract that any rows implementation must satisfy.
// It enables support for multiple database drivers (sql.Rows, pgx.Rows) and
// allows plugins to provide custom row implementations without modifying RowsAdapter.
//
// Plugins implementing custom rows should implement all five methods with their own logic.
// The package provides built-in adapters for sql.Rows and pgx.Rows.
//
// Example for a plugin with custom rows:
//
//	type CustomRows struct { ... }
//	func (c *CustomRows) columns() ([]string, error) { ... }
//	func (c *CustomRows) next() bool { ... }
//	func (c *CustomRows) scan(dest ...any) error { ... }
//	func (c *CustomRows) close() error { ... }
//	func (c *CustomRows) err() error { ... }
type RowsProvider interface {
	columns() ([]string, error)
	next() bool
	scan(dest ...any) error
	close() error
	err() error
}

// sqlRowsProvider wraps *sql.Rows to implement RowsProvider.
type sqlRowsProvider struct {
	rows *sql.Rows
}

func (s *sqlRowsProvider) columns() ([]string, error) {
	c, err := s.rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("sqlRowsProvider.columns: %w", err)
	}
	return c, nil
}

func (s *sqlRowsProvider) next() bool {
	return s.rows.Next()
}

func (s *sqlRowsProvider) scan(dest ...any) error {
	if err := s.rows.Scan(dest...); err != nil {
		return fmt.Errorf("sqlRowsProvider.scan: %w", err)
	}
	return nil
}

func (s *sqlRowsProvider) close() error {
	if err := s.rows.Close(); err != nil {
		return fmt.Errorf("sqlRowsProvider.close: %w", err)
	}
	return nil
}

func (s *sqlRowsProvider) err() error {
	if err := s.rows.Err(); err != nil {
		return fmt.Errorf("sqlRowsProvider.err: %w", err)
	}
	return nil
}

// pgxRowsProvider wraps pgx.Rows to implement RowsProvider.
type pgxRowsProvider struct {
	rows pgx.Rows
}

func (p *pgxRowsProvider) columns() ([]string, error) {
	fds := p.rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}
	return cols, nil
}

func (p *pgxRowsProvider) next() bool {
	return p.rows.Next()
}

func (p *pgxRowsProvider) scan(dest ...any) error {
	if err := p.rows.Scan(dest...); err != nil {
		return fmt.Errorf("pgxRowsProvider.scan: %w", err)
	}
	return nil
}

func (p *pgxRowsProvider) close() error {
	p.rows.Close()
	return nil
}

func (p *pgxRowsProvider) err() error {
	if err := p.rows.Err(); err != nil {
		return fmt.Errorf("pgxRowsProvider.err: %w", err)
	}
	return nil
}
