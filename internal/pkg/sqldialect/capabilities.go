package sqldialect

import (
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
// Dialects that do not implement CapabilityProvider report no capabilities.
func CapabilitiesFor(dialect condition.SQLDialect) Capabilities {
	if provider, ok := dialect.(CapabilityProvider); ok {
		return provider.Capabilities()
	}
	return Capabilities{}
}
