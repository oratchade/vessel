package builder

import (
	"tounilab.com/db-connector/query/condition"
)

// PostgresQueryBuilder is an alias of MSSQLQueryBuilder configured for Postgres.
type PostgresQueryBuilder = MSSQLQueryBuilder

// NewPostgresQueryBuilder constructs a new PostgresQueryBuilder with the provided dialect.
func NewPostgresQueryBuilder(dialect condition.SQLDialect) *PostgresQueryBuilder {
	return &PostgresQueryBuilder{
		dialect: dialect,
	}
}
