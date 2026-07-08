//go:build test

package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

// customMySQLDialect is a third-party dialect whose type name contains
// "MySQL". Detection must key on type identity, not the printed name, so
// none of the built-in predicates may claim it.
type customMySQLDialect struct {
	sqldialect.PostgresDialect
}

func TestDialectDetectionByTypeIdentity(t *testing.T) {
	tests := []struct {
		name     string
		dialect  cdt.SQLDialect
		mysql    bool
		postgres bool
		sqlite   bool
		mssql    bool
	}{
		{name: "mysql value", dialect: sqldialect.MySQLDialect{}, mysql: true},
		{name: "mysql pointer", dialect: &sqldialect.MySQLDialect{}, mysql: true},
		{name: "postgres value", dialect: sqldialect.PostgresDialect{}, postgres: true},
		{name: "postgres pointer", dialect: &sqldialect.PostgresDialect{}, postgres: true},
		{name: "sqlite value", dialect: sqldialect.SQLiteDialect{}, sqlite: true},
		{name: "sqlite pointer", dialect: &sqldialect.SQLiteDialect{}, sqlite: true},
		{name: "mssql value", dialect: sqldialect.MSSQLDialect{}, mssql: true},
		{name: "mssql pointer", dialect: &sqldialect.MSSQLDialect{}, mssql: true},
		{name: "lookalike type name is not detected", dialect: customMySQLDialect{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.mysql, isMySQLDialect(tt.dialect), "isMySQLDialect")
			assert.Equal(t, tt.postgres, isPostgresDialect(tt.dialect), "isPostgresDialect")
			assert.Equal(t, tt.sqlite, isSQLiteDialect(tt.dialect), "isSQLiteDialect")
			assert.Equal(t, tt.mssql, isMSSQLDialect(tt.dialect), "isMSSQLDialect")
		})
	}
}
