//go:build test

package v1_test

import (
	"testing"

	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/definition"
)

// TestMysqlConfigDriver tests the Driver method for MySQL config
func TestMysqlConfigDriver(t *testing.T) {
	cfg := v1.MysqlConfig{}
	assert.Equal(t, definition.DriverMySQL, cfg.Driver())
}

// TestMysqlConfigDSNWithDefaults tests MySQL DSN generation with default values
func TestMysqlConfigDSNWithDefaults(t *testing.T) {
	cfg := v1.MysqlConfig{
		User:     "testuser",
		Password: "testpass",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Charset:  "utf8mb4",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "testuser")
	assert.Contains(t, dsn, "testpass")
	assert.Contains(t, dsn, "localhost")
	assert.Contains(t, dsn, "3306")
	assert.Contains(t, dsn, "testdb")
	assert.Contains(t, dsn, "utf8mb4")
}

// TestMysqlConfigDSNMinimal tests MySQL DSN generation with minimal config
func TestMysqlConfigDSNMinimal(t *testing.T) {
	cfg := v1.MysqlConfig{
		Host: "db.example.com",
		Port: 3306,
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "db.example.com:3306")
}

// TestPostgresConfigDriver tests the Driver method for PostgreSQL config
func TestPostgresConfigDriver(t *testing.T) {
	cfg := v1.PostgresConfig{}
	assert.Equal(t, definition.DriverPostgres, cfg.Driver())
}

// TestPostgresConfigDSN tests PostgreSQL DSN generation
func TestPostgresConfigDSN(t *testing.T) {
	cfg := v1.PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "pguser",
		Password: "pgpass",
		Database: "testdb",
		SSLMode:  "require",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "postgres://")
	assert.Contains(t, dsn, "pguser")
	assert.Contains(t, dsn, "pgpass")
	assert.Contains(t, dsn, "localhost:5432")
	assert.Contains(t, dsn, "testdb")
	assert.Contains(t, dsn, "sslmode=require")
}

// TestPostgresConfigDSNMinimal tests PostgreSQL DSN generation with minimal config
func TestPostgresConfigDSNMinimal(t *testing.T) {
	cfg := v1.PostgresConfig{
		Host: "localhost",
		Port: 5432,
		User: "user",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "postgres://")
}

// TestSQLiteConfigDriver tests the Driver method for SQLite config
func TestSQLiteConfigDriver(t *testing.T) {
	cfg := v1.SQLiteConfig{}
	assert.Equal(t, definition.DriverSQLite, cfg.Driver())
}

// TestSQLiteConfigDSN tests SQLite DSN generation
func TestSQLiteConfigDSN(t *testing.T) {
	cfg := v1.SQLiteConfig{
		FilePath:    "/tmp/test.db",
		CacheMode:   "private",
		Mode:        "rwc",
		ForeignKeys: true,
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "file:/tmp/test.db")
	assert.Contains(t, dsn, "cache=private")
	assert.Contains(t, dsn, "mode=rwc")
	assert.Contains(t, dsn, "_pragma=foreign_keys%281%29")
}

// TestSQLiteConfigDSNMemory tests SQLite DSN generation for in-memory database
func TestSQLiteConfigDSNMemory(t *testing.T) {
	cfg := v1.SQLiteConfig{
		FilePath: ":memory:",
		Mode:     "memory",
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "file::memory:")
	assert.Contains(t, dsn, "mode=memory")
}

// TestMSSQLConfigDriver tests the Driver method for MSSQL config
func TestMSSQLConfigDriver(t *testing.T) {
	cfg := v1.MSSQLConfig{}
	assert.Equal(t, definition.DriverMSSQL, cfg.Driver())
}

// TestMSSQLConfigDSN tests MSSQL DSN generation
func TestMSSQLConfigDSN(t *testing.T) {
	cfg := v1.MSSQLConfig{
		User:            "mssqluser",
		Password:        "mssqlpass",
		Host:            "sqlserver.example.com",
		Port:            1433,
		Database:        "testdb",
		Encrypt:         "true",
		TrustServerCert: false,
	}
	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "sqlserver://")
	assert.Contains(t, dsn, "mssqluser:mssqlpass")
	assert.Contains(t, dsn, "sqlserver.example.com:1433")
	assert.Contains(t, dsn, "database=testdb")
	assert.Contains(t, dsn, "encrypt=true")
	assert.Contains(t, dsn, "trustservercertificate=false")
}

// TestMSSQLConfigDSNKeepsSyntaxCharacters: the driver reads back the user, password and
// database as given, whatever URL syntax they hold. A password pasted into the URL
// unescaped reached the server altered (%41 as A), or failed to parse with an error
// quoting part of it.
func TestMSSQLConfigDSNKeepsSyntaxCharacters(t *testing.T) {
	cfg := v1.MSSQLConfig{
		User:     "dom@in\\user",
		Password: "p%ZX%41@:/?#&=+ w",
		Host:     "localhost",
		Port:     1433,
		Database: "a&b=c d",
		Encrypt:  "true",
	}
	parsed, _, err := msdsn.Parse(cfg.DSN())
	require.NoError(t, err)
	assert.Equal(t, cfg.User, parsed.User)
	assert.Equal(t, cfg.Password, parsed.Password)
	assert.Equal(t, cfg.Database, parsed.Database)
	assert.Equal(t, "localhost", parsed.Host)
	assert.Equal(t, uint64(1433), parsed.Port)
}

// TestMSSQLConfigDSNWithoutEncrypt: encrypt is optional; left unset, the connection
// string leaves it out for the driver to apply its default, rather than writing an empty
// value the driver rejects (#88).
func TestMSSQLConfigDSNWithoutEncrypt(t *testing.T) {
	cfg := v1.MSSQLConfig{User: "app", Password: "pw", Host: "localhost", Port: 1433, Database: "app"}
	_, _, err := msdsn.Parse(cfg.DSN())
	require.NoError(t, err)
	assert.NotContains(t, cfg.DSN(), "encrypt=")
}

// TestMSSQLConfigDSNNamedInstance: a host written server\instance, as SQL Server names a
// named instance, reaches the driver as that server and instance (#89).
func TestMSSQLConfigDSNNamedInstance(t *testing.T) {
	cfg := v1.MSSQLConfig{User: "app", Password: "p@ss/w", Host: `srv\INSTANCE`, Port: 1433, Database: "app"}
	parsed, _, err := msdsn.Parse(cfg.DSN())
	require.NoError(t, err)
	assert.Equal(t, "srv", parsed.Host)
	assert.Equal(t, "INSTANCE", parsed.Instance)
	assert.Equal(t, cfg.Password, parsed.Password)
	assert.Equal(t, cfg.Database, parsed.Database)
}

// TestDatabaseConfigs tests all database config types
func TestDatabaseConfigs(t *testing.T) {
	testCases := []struct {
		name string
		cfg  v1.DBConfig
		want definition.DBType
	}{
		{
			name: "MySQL config",
			cfg:  v1.MysqlConfig{},
			want: definition.DriverMySQL,
		},
		{
			name: "Postgres config",
			cfg:  v1.PostgresConfig{},
			want: definition.DriverPostgres,
		},
		{
			name: "SQLite config",
			cfg:  v1.SQLiteConfig{},
			want: definition.DriverSQLite,
		},
		{
			name: "MSSQL config",
			cfg:  v1.MSSQLConfig{},
			want: definition.DriverMSSQL,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			driver := tc.cfg.Driver()
			require.Equal(t, tc.want, driver)
		})
	}
}
