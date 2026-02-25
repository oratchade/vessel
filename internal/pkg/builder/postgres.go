package builder

import (
	cdt "tounilab.com/db-connector/pkg/query/condition"
)

// PostgresQueryBuilder is an alias of MSSQLQueryBuilder configured for Postgres.
type PostgresQueryBuilder = MSSQLQueryBuilder

// NewPostgresQueryBuilder constructs a new PostgresQueryBuilder with the provided dialect.
func NewPostgresQueryBuilder(dialect cdt.SQLDialect) *PostgresQueryBuilder {
	return &PostgresQueryBuilder{
		dialect: dialect,
	}
}
