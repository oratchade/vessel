package builder

import (
	"tounilab.com/db-connector/query/condition"
)

type PostgresQueryBuilder = MSSQLQueryBuilder

func NewPostgresQueryBuilder(dialect condition.SQLDialect) *PostgresQueryBuilder {
	return &PostgresQueryBuilder{
		dialect: dialect,
	}
}
