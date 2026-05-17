package builder

// PostgresQueryBuilder is an alias of MSSQLQueryBuilder configured for Postgres.
type PostgresQueryBuilder = MSSQLQueryBuilder

// NewPostgresQueryBuilder constructs a new PostgresQueryBuilder with the provided dialect.
func NewPostgresQueryBuilder(dialect optionDialect) *PostgresQueryBuilder {
	return &PostgresQueryBuilder{
		dialect: dialect,
	}
}
