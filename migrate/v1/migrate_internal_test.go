//go:build test

package v1

import (
	"errors"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestDialectFor(t *testing.T) {
	tests := []struct {
		driver    string
		sqlDriver string
		dialect   goose.Dialect
	}{
		{"postgres", "pgx", goose.DialectPostgres},
		{"postgresql", "pgx", goose.DialectPostgres},
		{"mysql", "mysql", goose.DialectMySQL},
		{"sqlite", "sqlite", goose.DialectSQLite3},
		{"sqlserver", "sqlserver", goose.DialectMSSQL},
		{"mssql", "sqlserver", goose.DialectMSSQL},
	}
	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			sqlDriver, dialect, err := dialectFor(tt.driver)
			if err != nil {
				t.Fatalf("dialectFor(%q): %v", tt.driver, err)
			}
			if sqlDriver != tt.sqlDriver || dialect != tt.dialect {
				t.Errorf("dialectFor(%q) = %q, %q; want %q, %q", tt.driver, sqlDriver, dialect, tt.sqlDriver, tt.dialect)
			}
		})
	}

	if _, _, err := dialectFor("clickhouse"); !errors.Is(err, ErrUnsupportedDriver) {
		t.Errorf("dialectFor(clickhouse) error = %v, want ErrUnsupportedDriver", err)
	}
}

func TestLockerFor(t *testing.T) {
	for _, dialect := range []goose.Dialect{goose.DialectPostgres, goose.DialectMySQL} {
		opt, err := lockerFor(dialect)
		if err != nil || opt == nil {
			t.Errorf("lockerFor(%s) = %v, %v; want a locker", dialect, opt, err)
		}
	}
	// goose has no locker for these: concurrent runs are the caller's responsibility.
	for _, dialect := range []goose.Dialect{goose.DialectSQLite3, goose.DialectMSSQL} {
		opt, err := lockerFor(dialect)
		if err != nil || opt != nil {
			t.Errorf("lockerFor(%s) = %v, %v; want none", dialect, opt, err)
		}
	}
}
