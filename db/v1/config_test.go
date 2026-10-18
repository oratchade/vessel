//go:build test

package v1_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
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

// TestMSSQLConfigDSNWithoutEncrypt: encrypt is optional, and left unset it means
// encryption required: the driver's own default encrypts at most the login, and none of
// it when the server, or anyone between, answers that it has no encryption (#88).
func TestMSSQLConfigDSNWithoutEncrypt(t *testing.T) {
	cfg := v1.MSSQLConfig{User: "app", Password: "pw", Host: "localhost", Port: 1433, Database: "app"}
	parsed, _, err := msdsn.Parse(cfg.DSN())
	require.NoError(t, err)
	assert.Equal(t, msdsn.Encryption(msdsn.EncryptionRequired), parsed.Encryption)
	require.NotNil(t, parsed.TLSConfig)
	assert.False(t, parsed.TLSConfig.InsecureSkipVerify)
}

// TestMSSQLConfigDSNNamedInstance: a host written server\instance, as SQL Server names a
// named instance, reaches the driver as that server and instance (#89).
func TestMSSQLConfigDSNNamedInstance(t *testing.T) {
	cfg := v1.MSSQLConfig{User: "app", Password: "p@ss/w", Host: `srv\INSTANCE`, Port: 1433, Database: "app", Encrypt: "true"}
	parsed, _, err := msdsn.Parse(cfg.DSN())
	require.NoError(t, err)
	assert.Equal(t, "srv", parsed.Host)
	assert.Equal(t, "INSTANCE", parsed.Instance)
	// The driver finds the instance's port through SQL Server Browser, and calls a port
	// given with an instance a misconfiguration: none is sent.
	sent, err := url.Parse(cfg.DSN())
	require.NoError(t, err)
	assert.Empty(t, sent.Port())
	assert.Equal(t, cfg.Password, parsed.Password)
	assert.Equal(t, cfg.Database, parsed.Database)
}

// hostSpellings are the ways a host is written, each with the host the driver must read
// back: an IPv6 address bare or bracketed alike, as an address beside a port is written.
var hostSpellings = []struct{ written, host, mysqlAddr string }{
	{"db.internal", "db.internal", "db.internal:3307"},
	{"10.0.0.5", "10.0.0.5", "10.0.0.5:3307"},
	{"::1", "::1", "[::1]:3307"},
	{"[::1]", "::1", "[::1]:3307"},
	{"fd00::5", "fd00::5", "[fd00::5]:3307"},
	{"[fd00::5]", "fd00::5", "[fd00::5]:3307"},
	{"fe80::1%eth0", "fe80::1%eth0", "[fe80::1%eth0]:3307"},
	{"[fe80::1%eth0]", "fe80::1%eth0", "[fe80::1%eth0]:3307"},
}

// TestDSNsReachEveryKindOfHost: each driver reads the host and the port apart, an IPv6
// host included. Joined with a bare colon, fd00::5 and its port read as one invalid
// address; joined bracketed, [fd00::5] became [[fd00::5]].
func TestDSNsReachEveryKindOfHost(t *testing.T) {
	for _, s := range hostSpellings {
		t.Run(s.written, func(t *testing.T) {
			pg, err := pgx.ParseConfig(v1.PostgresConfig{Host: s.written, Port: 6432, User: "u", Password: "p", Database: "db"}.DSN())
			if assert.NoError(t, err, "postgres") {
				assert.Equal(t, s.host, pg.Host, "postgres host")
				assert.Equal(t, uint16(6432), pg.Port, "postgres port")
			}

			my, err := mysql.ParseDSN(v1.MysqlConfig{Host: s.written, Port: 3307, User: "u", Password: "p", Database: "db"}.DSN())
			if assert.NoError(t, err, "mysql") {
				assert.Equal(t, s.mysqlAddr, my.Addr, "mysql address")
			}

			if strings.Contains(s.host, "%") {
				return // refused for SQL Server by Validate: its driver dials without the zone
			}
			ms, _, err := msdsn.Parse(v1.MSSQLConfig{Host: s.written, Port: 1434, User: "u", Password: "p", Database: "db", Encrypt: "true"}.DSN())
			if assert.NoError(t, err, "mssql") {
				assert.Equal(t, s.host, ms.Host, "mssql host")
				assert.Equal(t, uint64(1434), ms.Port, "mssql port")
			}
		})
	}
}

// TestDSNsJoinAnyOtherHostAsWritten: only an IPv6 address is bracketed. Any other host
// keeps the address it always had, so whatever a driver made of it before it makes of
// it still: stripping brackets from [] turned the PostgreSQL parse error into a
// connection to PGHOST or localhost.
func TestDSNsJoinAnyOtherHostAsWritten(t *testing.T) {
	for _, host := range []string{"[]", "[db.internal]", "[fd00::5", "[[fd00::5]]", "db.internal:5433"} {
		t.Run(host, func(t *testing.T) {
			pg := v1.PostgresConfig{Host: host, Port: 6432, User: "u", Password: "p", Database: "db"}.DSN()
			assert.Contains(t, pg, "@"+host+":6432/", "postgres address")
			my := v1.MysqlConfig{Host: host, Port: 3307, User: "u", Password: "p", Database: "db"}.DSN()
			assert.Contains(t, my, "@tcp("+host+":3307)/", "mysql address")
			// SQL Server joined any host with net.JoinHostPort, bracketing one with a colon,
			// so a malformed [::1 became [[::1]:1434 and connected; it is passed as written.
			ms := v1.MSSQLConfig{Host: host, Port: 1434, User: "u", Password: "p", Database: "db", Encrypt: "true"}.DSN()
			assert.Contains(t, ms, "@"+host+":1434?", "mssql address")
		})
	}
	_, err := pgx.ParseConfig(v1.PostgresConfig{Host: "[]", Port: 6432, User: "u", Password: "p", Database: "db"}.DSN())
	assert.Error(t, err, "an empty host in brackets is refused, not taken for the default")
}

// TestMSSQLConfigValidate: a named instance at an IPv6 address cannot be reached, since
// the driver asks SQL Server Browser at host:1434 joined without brackets, then looks
// the host up with them kept; Validate refuses it rather than leave the driver to fail
// on an error that names neither cause.
func TestMSSQLConfigValidate(t *testing.T) {
	for _, host := range []string{`fd00::5\INST`, `[fd00::5]\INST`} {
		err := v1.MSSQLConfig{Host: host}.Validate()
		if assert.Error(t, err, host) {
			assert.Contains(t, err.Error(), "named instance", host)
		}
	}
	// The driver dials a zoned address with its zone dropped, so a link-local one,
	// which needs it, cannot be reached either.
	for _, host := range []string{"fe80::1%eth0", "[fe80::1%eth0]"} {
		err := v1.MSSQLConfig{Host: host}.Validate()
		if assert.Error(t, err, host) {
			assert.Contains(t, err.Error(), "zone", host)
		}
	}
	for _, host := range []string{"fd00::5", "[fd00::5]", "db.internal", `srv\INST`, `10.0.0.5\INST`, ""} {
		assert.NoError(t, v1.MSSQLConfig{Host: host}.Validate(), host)
	}
}

// TestMSSQLConfigValidateRefusesWhatTheDriverWouldMisread: a host the driver would not
// read back as written is refused, naming the host and never the DSN. Handed on, it
// connected elsewhere (on Go 1.27, 0:0:0:0:0:0:0 reached [::1]:1433), or failed with
// the driver's parse error, which quotes the whole DSN, password included.
func TestMSSQLConfigValidateRefusesWhatTheDriverWouldMisread(t *testing.T) {
	for _, host := range []string{"0:0:0:0:0:0:0", "::1%", "[::1", "srv host", "[db.internal]", "db.internal:5433"} {
		t.Run(host, func(t *testing.T) {
			err := v1.MSSQLConfig{Host: host, Port: 1434, User: "u", Password: "pw-secret", Encrypt: "true"}.Validate()
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), host)
				assert.NotContains(t, err.Error(), "pw-secret")
			}
		})
	}
}

// TestMSSQLConfigValidateAcceptsWhatTheDriverReadsBack: every host Validate lets through
// reaches the driver's own parser as the host, port and instance configured.
func TestMSSQLConfigValidateAcceptsWhatTheDriverReadsBack(t *testing.T) {
	tests := []struct {
		written, host, instance string
		port                    uint64
	}{
		{"localhost", "localhost", "", 1434},
		{"db.internal", "db.internal", "", 1434},
		{"10.0.0.5", "10.0.0.5", "", 1434},
		{"::1", "::1", "", 1434},
		{"[fd00::5]", "fd00::5", "", 1434},
		{`srv\INST`, "srv", "INST", 0},
		{`10.0.0.5\INST`, "10.0.0.5", "INST", 0},
		// A zone the dial can do without: the loopback and a global address route
		// without one, so dropping it loses nothing. These connected before.
		{"::1%lo0", "::1%lo0", "", 1434},
		{"fd00::5%eth0", "fd00::5%eth0", "", 1434},
	}
	for _, tt := range tests {
		t.Run(tt.written, func(t *testing.T) {
			cfg := v1.MSSQLConfig{Host: tt.written, Port: 1434, User: "u", Password: "p", Database: "db", Encrypt: "true"}
			require.NoError(t, cfg.Validate())
			parsed, _, err := msdsn.Parse(cfg.DSN())
			require.NoError(t, err)
			assert.Equal(t, tt.host, parsed.Host)
			assert.Equal(t, tt.instance, parsed.Instance)
			assert.Equal(t, tt.port, parsed.Port)
		})
	}
}

// TestNewMSSQLKeepsThePasswordOutOfAHostError: a malformed host stops the manager with
// Validate's error, before the driver can quote the DSN.
func TestNewMSSQLKeepsThePasswordOutOfAHostError(t *testing.T) {
	_, err := v1.MSSQLCfgToDB(v1.MSSQLConfig{
		Host: "[::1", Port: 1434, User: "u", Password: "pw-secret", Database: "db", Encrypt: "true", ConnectionTimeout: time.Second,
	}, nil)
	if assert.Error(t, err) {
		assert.NotContains(t, err.Error(), "pw-secret")
	}
}

// TestNewMSSQLValidatesTheHost: opening a manager runs Validate before the driver. The
// timeout bounds the dial a regression would make.
func TestNewMSSQLValidatesTheHost(t *testing.T) {
	_, err := v1.MSSQLCfgToDB(v1.MSSQLConfig{
		Host: `fd00::5\INST`, User: "u", Password: "p", Database: "db", Encrypt: "true", ConnectionTimeout: time.Second,
	}, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "named instance")
	}
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
