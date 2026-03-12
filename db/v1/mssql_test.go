//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/pkg/query/definition"
)

// TestMSSQLNewDB tests MSSQL database initialization
func TestMSSQLNewDB(t *testing.T) {
	cfg := &v1.MSSQLConfig{
		User:     "sa",
		Password: "password",
		Host:     "localhost",
		Port:     1433,
		Database: "testdb",
	}

	assert.NotNil(t, cfg)
	assert.Equal(t, definition.DriverMSSQL, cfg.Driver())
}

// TestMSSQLDSNGeneration tests MSSQL DSN creation
func TestMSSQLDSNGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *v1.MSSQLConfig
		validate func(t *testing.T, dsn string)
	}{
		{
			name: "basic configuration",
			cfg: &v1.MSSQLConfig{
				User:     "sa",
				Password: "MyPassword123",
				Host:     "sqlserver.example.com",
				Port:     1433,
				Database: "TestDB",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "sqlserver://")
				assert.Contains(t, dsn, "sa:MyPassword123")
				assert.Contains(t, dsn, "sqlserver.example.com:1433")
				assert.Contains(t, dsn, "database=TestDB")
			},
		},
		{
			name: "with encryption enabled",
			cfg: &v1.MSSQLConfig{
				Host:    "localhost",
				Port:    1433,
				User:    "sa",
				Encrypt: "true",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "encrypt=true")
			},
		},
		{
			name: "without certificate verification",
			cfg: &v1.MSSQLConfig{
				Host:            "localhost",
				Port:            1433,
				User:            "sa",
				Encrypt:         "true",
				TrustServerCert: true,
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "trustservercertificate=true")
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

// TestMSSQLEncryption tests encryption configuration options
func TestMSSQLEncryption(t *testing.T) {
	encryptionModes := []string{"true", "false", "disable", "encrypt"}

	for _, mode := range encryptionModes {
		t.Run(mode, func(t *testing.T) {
			cfg := &v1.MSSQLConfig{
				Host:    "localhost",
				Port:    1433,
				User:    "sa",
				Encrypt: mode,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, "encrypt="+mode)
		})
	}
}

// TestMSSQLTrustCert tests certificate trust configuration
func TestMSSQLTrustCert(t *testing.T) {
	testCases := []struct {
		name     string
		trust    bool
		expected string
	}{
		{"trust enabled", true, "true"},
		{"trust disabled", false, "false"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.MSSQLConfig{
				Host:            "localhost",
				Port:            1433,
				User:            "sa",
				TrustServerCert: tc.trust,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, "trustservercertificate="+tc.expected)
		})
	}
}

// TestMSSQLCredentials tests user and password handling
func TestMSSQLCredentials(t *testing.T) {
	testCases := []struct {
		name     string
		user     string
		password string
	}{
		{"basic", "sa", "password"},
		{"domain user", "DOMAIN\\user", "MyPass123!"},
		{"special chars", "user", "P@ss:word#2024"},
		{"complex", "admin@server", "Str0ng!P@ss#123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.MSSQLConfig{
				Host:     "localhost",
				Port:     1433,
				User:     tc.user,
				Password: tc.password,
			}
			dsn := cfg.DSN()
			assert.NotEmpty(t, dsn)
		})
	}
}

// TestMSSQLHostPort tests various host/port combinations
func TestMSSQLHostPort(t *testing.T) {
	testCases := []struct {
		name string
		host string
		port uint16
	}{
		{"localhost", "localhost", 1433},
		{"remote", "sqlserver.example.com", 1433},
		{"custom port", "localhost", 1434},
		{"IP address", "192.168.1.100", 1433},
		{"named instance", "localhost\\SQLEXPRESS", 1433},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.MSSQLConfig{
				Host: tc.host,
				Port: tc.port,
				User: "sa",
			}
			dsn := cfg.DSN()
			assert.NotEmpty(t, dsn)
		})
	}
}

// TestMSSQLDatabases tests different database names
func TestMSSQLDatabases(t *testing.T) {
	databases := []string{"TestDB", "production", "dev_app", "MyDatabase123"}

	for _, db := range databases {
		t.Run(db, func(t *testing.T) {
			cfg := &v1.MSSQLConfig{
				Host:     "localhost",
				Port:     1433,
				User:     "sa",
				Database: db,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, "database="+db)
		})
	}
}

// TestMSSQLMinimal tests minimal configuration
func TestMSSQLMinimal(t *testing.T) {
	cfg := &v1.MSSQLConfig{
		Host: "localhost",
		Port: 1433,
		User: "sa",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "sqlserver://")
}
