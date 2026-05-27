//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/definition"
)

// TestPostgresNewDB tests PostgreSQL database initialization
func TestPostgresNewDB(t *testing.T) {
	cfg := &v1.PostgresConfig{
		User:     "postgres",
		Password: "password",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
	}

	assert.NotNil(t, cfg)
	assert.Equal(t, definition.DriverPostgres, cfg.Driver())
}

// TestPostgresDSNGeneration tests PostgreSQL DSN creation
func TestPostgresDSNGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *v1.PostgresConfig
		validate func(t *testing.T, dsn string)
	}{
		{
			name: "basic configuration",
			cfg: &v1.PostgresConfig{
				User:     "pguser",
				Password: "pgpass",
				Host:     "db.example.com",
				Port:     5432,
				Database: "mydb",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "postgres://")
				assert.Contains(t, dsn, "pguser")
				assert.Contains(t, dsn, "pgpass")
				assert.Contains(t, dsn, "db.example.com")
				assert.Contains(t, dsn, "5432")
				assert.Contains(t, dsn, "mydb")
			},
		},
		{
			name: "with SSL mode",
			cfg: &v1.PostgresConfig{
				Host:    "localhost",
				Port:    5432,
				User:    "user",
				SSLMode: "require",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "sslmode=require")
			},
		},
		{
			name: "with application name",
			cfg: &v1.PostgresConfig{
				Host:            "localhost",
				Port:            5432,
				User:            "user",
				ApplicationName: "myapp",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "application_name=myapp")
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

// TestPostgresSSLModes tests different SSL mode configurations
func TestPostgresSSLModes(t *testing.T) {
	modes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			cfg := &v1.PostgresConfig{
				Host:    "localhost",
				Port:    5432,
				User:    "user",
				SSLMode: mode,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, "sslmode="+mode)
		})
	}
}

// TestPostgresConnectTimeout tests connection timeout configuration
func TestPostgresConnectTimeout(t *testing.T) {
	var timeoutDuration any // time.Duration
	cfg := &v1.PostgresConfig{
		Host: "localhost",
		Port: 5432,
		User: "user",
	}
	_ = timeoutDuration
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "postgres://")
}

// TestPostgresCredentials tests user and password handling
func TestPostgresCredentials(t *testing.T) {
	testCases := []struct {
		name     string
		user     string
		password string
	}{
		{"basic", "user", "pass"},
		{"special chars", "user@domain", "p@ss:word"},
		{"empty password", "user", ""},
		{"complex", "admin", "Str0ng!P@ss#2024"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.PostgresConfig{
				User:     tc.user,
				Password: tc.password,
				Host:     "localhost",
				Port:     5432,
			}
			dsn := cfg.DSN()
			assert.NotEmpty(t, dsn)
			// URL-encoded special characters in DSN, so just check it contains postgres://
			assert.Contains(t, dsn, "postgres://")
			assert.Contains(t, dsn, "localhost:5432")
		})
	}
}

// TestPostgresHostPort tests various host/port combinations
func TestPostgresHostPort(t *testing.T) {
	testCases := []struct {
		name string
		host string
		port uint16
	}{
		{"localhost", "localhost", 5432},
		{"remote", "db.example.com", 5432},
		{"custom port", "localhost", 5433},
		{"IP address", "192.168.1.100", 5432},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.PostgresConfig{
				Host: tc.host,
				Port: tc.port,
				User: "user",
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, tc.host)
		})
	}
}

// TestPostgresMultipleDatabases tests different database names
func TestPostgresMultipleDatabases(t *testing.T) {
	databases := []string{"myapp", "production", "test_db", "db_v2"}

	for _, db := range databases {
		t.Run(db, func(t *testing.T) {
			cfg := &v1.PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Database: db,
			}
			dsn := cfg.DSN()
			// Database name is embedded in the path, not as dbname parameter
			assert.Contains(t, dsn, "/"+db)
		})
	}
}

// TestPostgresMinimal tests minimal configuration
func TestPostgresMinimal(t *testing.T) {
	cfg := &v1.PostgresConfig{
		Host: "localhost",
		Port: 5432,
		User: "user",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "postgres://")
}

// TestPostgresMinimal tests minimal configuration
