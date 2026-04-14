// Package definition contains constants for query types and database driver names.
package definition

type QueryType = string

const (
	// QueryTypeSelect represents a SELECT query.
	QueryTypeSelect QueryType = "SELECT"
	// QueryTypeInsert represents an INSERT query.
	QueryTypeInsert QueryType = "INSERT"
	// QueryTypeUpdate represents an UPDATE query.
	QueryTypeUpdate QueryType = "UPDATE"
	// QueryTypeDelete represents a DELETE query.
	QueryTypeDelete QueryType = "DELETE"
	// QueryTypeUpsert represents an UPSERT query.
	QueryTypeUpsert QueryType = "UPSERT"
	// QueryTypeTruncate represents a TRUNCATE query.
	QueryTypeTruncate QueryType = "TRUNCATE"
)

type DBType = string

const (
	// DriverMySQL is the driver name for MySQL databases.
	DriverMySQL DBType = "mysql"
	// DriverPostgres is the driver name for PostgreSQL databases.
	DriverPostgres DBType = "postgres"
	// DriverPostgresAlias is an alias for PostgreSQL databases.
	DriverPostgresAlias DBType = "postgresql"
	// DriverSQLLite is the driver name for SQLITE databases.
	DriverSQLLite DBType = "sqlite3"
	// DriverSQLiteAlias is an alias for SQLite databases.
	DriverSQLiteAlias DBType = "sqlite"
	// DriverMSSQL is the driver name for Microsoft SQL Server databases.
	DriverMSSQL DBType = "sqlserver"
	// DriverMSSQLAlias is an alias for Microsoft SQL Server databases.
	DriverMSSQLAlias DBType = "mssql"
)
