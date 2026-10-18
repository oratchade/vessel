//go:build test

package v1_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
)

// hostSpellings are the ways a host is written, each with the host the driver must read
// back: an IPv6 address bare or bracketed alike, as an address beside a port is written.
var hostSpellings = []struct{ written, host string }{
	{"db.internal", "db.internal"},
	{"10.0.0.5", "10.0.0.5"},
	{"::1", "::1"},
	{"[::1]", "::1"},
	{"fd00::5", "fd00::5"},
	{"[fd00::5]", "fd00::5"},
	{"fe80::1%eth0", "fe80::1%eth0"},
	{"[fe80::1%eth0]", "fe80::1%eth0"},
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
				assert.Equal(t, net.JoinHostPort(s.host, strconv.Itoa(3307)), my.Addr, "mysql address")
			}

			ms, _, err := msdsn.Parse(v1.MSSQLConfig{Host: s.written, Port: 1434, User: "u", Password: "p", Database: "db", Encrypt: "true"}.DSN())
			require.NoError(t, err, "mssql")
			assert.Equal(t, s.host, ms.Host, "mssql host")
			assert.Equal(t, uint64(1434), ms.Port, "mssql port")
		})
	}
}

// TestDSNsJoinAnyOtherHostAsWritten: only an IPv6 address is bracketed. Any other host
// keeps the address it always had, so whatever the driver made of it before it makes of
// it still; stripping brackets from [] turned a parse error into a connection to PGHOST
// or localhost.
func TestDSNsJoinAnyOtherHostAsWritten(t *testing.T) {
	for _, host := range []string{"[]", "[db.internal]", "[fd00::5", "[[fd00::5]]", "db.internal:5433"} {
		t.Run(host, func(t *testing.T) {
			pg := v1.PostgresConfig{Host: host, Port: 6432, User: "u", Password: "p", Database: "db"}.DSN()
			assert.Contains(t, pg, "@"+host+":6432/", "postgres address")
			my := v1.MysqlConfig{Host: host, Port: 3307, User: "u", Password: "p", Database: "db"}.DSN()
			assert.Contains(t, my, "@tcp("+host+":3307)/", "mysql address")
		})
	}
	_, err := pgx.ParseConfig(v1.PostgresConfig{Host: "[]", Port: 6432, User: "u", Password: "p", Database: "db"}.DSN())
	assert.Error(t, err, "an empty host in brackets is refused, not taken for the default")
}
