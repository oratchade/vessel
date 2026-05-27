package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/vessel/pkg/query/condition"
)

// Capabilities describes syntax features the SQL builder can safely render for a dialect.
type Capabilities struct {
	SelectPagination       bool
	MutationReturning      bool
	MutationOutput         bool
	MutationOrderLimit     bool
	Upsert                 bool
	JoinedUpdate           bool
	JoinedDelete           bool
	JoinedDeleteWithUsing  bool
	MutationOrderLimitName string
}

// CapabilityProvider is implemented by dialects that can report their SQL-generation capabilities directly.
type CapabilityProvider interface {
	Capabilities() Capabilities
}

// CapabilitiesFor returns the SQL-generation capabilities for a dialect.
func CapabilitiesFor(dialect condition.SQLDialect) Capabilities {
	if provider, ok := dialect.(CapabilityProvider); ok {
		return provider.Capabilities()
	}
	switch {
	case isMySQL(dialect):
		return Capabilities{
			SelectPagination:       true,
			MutationOrderLimit:     true,
			Upsert:                 true,
			JoinedUpdate:           true,
			JoinedDelete:           true,
			MutationOrderLimitName: "MySQL",
		}
	case isPostgres(dialect):
		return Capabilities{
			SelectPagination:      true,
			MutationReturning:     true,
			Upsert:                true,
			JoinedUpdate:          true,
			JoinedDelete:          true,
			JoinedDeleteWithUsing: true,
		}
	case isSQLite(dialect):
		return Capabilities{
			SelectPagination:  true,
			JoinedUpdate:      true,
			MutationReturning: false,
			Upsert:            true,
		}
	case isMSSQL(dialect):
		return Capabilities{
			SelectPagination:  true,
			MutationOutput:    true,
			MutationReturning: true,
			JoinedUpdate:      true,
			JoinedDelete:      true,
		}
	default:
		return Capabilities{}
	}
}

func isMySQL(dialect condition.SQLDialect) bool {
	return strings.Contains(typeName(dialect), "MySQL")
}

func isPostgres(dialect condition.SQLDialect) bool {
	return strings.Contains(typeName(dialect), "Postgres")
}

func isSQLite(dialect condition.SQLDialect) bool {
	return strings.Contains(typeName(dialect), "SQLite")
}

func isMSSQL(dialect condition.SQLDialect) bool {
	return strings.Contains(typeName(dialect), "MSSQL")
}

func typeName(dialect condition.SQLDialect) string {
	return fmt.Sprintf("%T", dialect)
}
