//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/pkg/query/definition"
)

// TestSQLiteNewDB tests SQLite database initialization
func TestSQLiteNewDB(t *testing.T) {
	cfg := &v1.SQLiteConfig{
		FilePath: "/tmp/test.db",
	}

	assert.NotNil(t, cfg)
	assert.Equal(t, definition.DriverSQLite, cfg.Driver())
}

// TestSQLiteDSNGeneration tests SQLite DSN creation
func TestSQLiteDSNGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *v1.SQLiteConfig
		validate func(t *testing.T, dsn string)
	}{
		{
			name: "file-based database",
			cfg: &v1.SQLiteConfig{
				FilePath: "/tmp/myapp.db",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "file:")
				assert.Contains(t, dsn, "/tmp/myapp.db")
			},
		},
		{
			name: "in-memory database",
			cfg: &v1.SQLiteConfig{
				FilePath: ":memory:",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, ":memory:")
			},
		},
		{
			name: "with cache mode",
			cfg: &v1.SQLiteConfig{
				FilePath:  "/tmp/test.db",
				CacheMode: "shared",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "cache=shared")
			},
		},
		{
			name: "with foreign keys enabled",
			cfg: &v1.SQLiteConfig{
				FilePath:    "/tmp/test.db",
				ForeignKeys: true,
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "_pragma=foreign_keys%281%29")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := tc.cfg.DSN()
			assert.NotEmpty(t, dsn)
			tc.validate(t, dsn)
		})
	}
}

// TestSQLiteInMemory tests in-memory database configuration
func TestSQLiteInMemory(t *testing.T) {
	cfg := &v1.SQLiteConfig{
		FilePath:  ":memory:",
		CacheMode: "private",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, ":memory:")
	assert.Contains(t, dsn, "cache=private")
}

// TestSQLiteFilePaths tests various file path configurations
func TestSQLiteFilePaths(t *testing.T) {
	paths := []string{
		"/tmp/test.db",
		"./local.db",
		"../data/app.db",
		"/var/lib/app/data.db",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			cfg := &v1.SQLiteConfig{
				FilePath: path,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, path)
		})
	}
}

// TestSQLiteCacheModes tests different cache modes
func TestSQLiteCacheModes(t *testing.T) {
	modes := []string{"shared", "private", "memory"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			cfg := &v1.SQLiteConfig{
				FilePath:  ":memory:",
				CacheMode: mode,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, "cache="+mode)
		})
	}
}

// TestSQLiteMode tests different file open modes
func TestSQLiteMode(t *testing.T) {
	modes := []string{"ro", "rw", "rwc", "memory"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			cfg := &v1.SQLiteConfig{
				FilePath: ":memory:",
				Mode:     mode,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, "mode="+mode)
		})
	}
}

// TestSQLiteForeignKeys tests foreign key constraint configuration
func TestSQLiteForeignKeys(t *testing.T) {
	testCases := []struct {
		name     string
		enabled  bool
		expected string
	}{
		{"enabled", true, "_pragma=foreign_keys%281%29"},
		{"disabled", false, "_pragma=foreign_keys%281%29"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.SQLiteConfig{
				FilePath:    "/tmp/test.db",
				ForeignKeys: tc.enabled,
			}
			dsn := cfg.DSN()
			if tc.enabled {
				assert.Contains(t, dsn, tc.expected)
			} else {
				assert.NotContains(t, dsn, tc.expected)
			}
		})
	}
}

// TestSQLiteMinimal tests minimal configuration
func TestSQLiteMinimal(t *testing.T) {
	cfg := &v1.SQLiteConfig{
		FilePath: "/tmp/test.db",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "/tmp/test.db")
}

// TestSQLiteJournal tests journal mode configuration
func TestSQLiteJournal(t *testing.T) {
	cfg := &v1.SQLiteConfig{
		FilePath: "/tmp/test.db",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
}
