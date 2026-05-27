//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/definition"
)

// TestMysqlNewDB tests MySQL database initialization
func TestMysqlNewDB(t *testing.T) {
	cfg := &v1.MysqlConfig{
		User:     "user",
		Password: "password",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
	}

	assert.NotNil(t, cfg)
	assert.Equal(t, definition.DriverMySQL, cfg.Driver())
}

// TestMysqlDSNGeneration tests MySQL DSN creation
func TestMysqlDSNGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *v1.MysqlConfig
		validate func(t *testing.T, dsn string)
	}{
		{
			name: "basic configuration",
			cfg: &v1.MysqlConfig{
				User:     "testuser",
				Password: "testpass",
				Host:     "db.example.com",
				Port:     3306,
				Database: "mydb",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "testuser")
				assert.Contains(t, dsn, "testpass")
				assert.Contains(t, dsn, "db.example.com")
				assert.Contains(t, dsn, "3306")
				assert.Contains(t, dsn, "mydb")
			},
		},
		{
			name: "with charset",
			cfg: &v1.MysqlConfig{
				User:     "user",
				Password: "pass",
				Host:     "localhost",
				Port:     3306,
				Database: "db",
				Charset:  "utf8mb4",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "utf8mb4")
			},
		},
		{
			name: "with timezone",
			cfg: &v1.MysqlConfig{
				User:     "user",
				Password: "pass",
				Host:     "localhost",
				Port:     3306,
				Database: "db",
				Loc:      "UTC",
			},
			validate: func(t *testing.T, dsn string) {
				assert.Contains(t, dsn, "UTC")
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

// TestMysqlHostPort tests host and port combination
func TestMysqlHostPort(t *testing.T) {
	testCases := []struct {
		name string
		host string
		port uint16
	}{
		{"localhost", "localhost", 3306},
		{"remote host", "db.example.com", 3306},
		{"custom port", "localhost", 3307},
		{"IP address", "192.168.1.100", 3306},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.MysqlConfig{
				Host: tc.host,
				Port: tc.port,
			}
			dsn := cfg.DSN()
			assert.Contains(t, dsn, tc.host)
			assert.NotEmpty(t, dsn)
		})
	}
}

// TestMysqlCredentials tests user and password handling
func TestMysqlCredentials(t *testing.T) {
	testCases := []struct {
		name     string
		user     string
		password string
	}{
		{"basic", "user", "pass"},
		{"with special chars", "user@host", "p@ss:word"},
		{"empty password", "user", ""},
		{"complex password", "admin", "P@ssw0rd!#$%"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &v1.MysqlConfig{
				Host:     "localhost",
				Port:     3306,
				Password: tc.password,
				User:     tc.user,
			}
			dsn := cfg.DSN()
			assert.NotEmpty(t, dsn)
			assert.Contains(t, dsn, tc.user)
		})
	}
}

// TestMysqlParsetime tests parseTime option
func TestMysqlParsetime(t *testing.T) {
	cfg := &v1.MysqlConfig{
		Host:      "localhost",
		Port:      3306,
		ParseTime: true,
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "parseTime=true")
}

// TestMysqlMultipleDatabases tests different database names
func TestMysqlMultipleDatabases(t *testing.T) {
	databases := []string{"myapp", "production", "test_db", "db_123"}
	for _, db := range databases {
		t.Run(db, func(t *testing.T) {
			cfg := &v1.MysqlConfig{
				Host:     "localhost",
				Port:     3306,
				Database: db,
			}
			dsn := cfg.DSN()
			assert.NotEmpty(t, dsn)
			assert.Contains(t, dsn, db)
		})
	}
}

// TestMysqlMinimal tests minimal configuration
func TestMysqlMinimal(t *testing.T) {
	cfg := &v1.MysqlConfig{
		Host: "localhost",
		Port: 3306,
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "localhost:3306")
}

// TestMysqlDialIndex tests multiple instance configuration
func TestMysqlDialIndex(t *testing.T) {
	cfgs := []*v1.MysqlConfig{
		{Host: "host1", Port: 3306},
		{Host: "host2", Port: 3306},
		{Host: "host3", Port: 3307},
	}

	for i, cfg := range cfgs {
		t.Run(string(rune(i)), func(t *testing.T) {
			dsn := cfg.DSN()
			assert.NotEmpty(t, dsn)
			assert.Contains(t, dsn, cfg.Host)
		})
	}
}
