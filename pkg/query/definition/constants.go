package definition

type QueryType = string

const (
	QueryTypeSelect   QueryType = "SELECT"
	QueryTypeInsert   QueryType = "INSERT"
	QueryTypeUpdate   QueryType = "UPDATE"
	QueryTypeDelete   QueryType = "DELETE"
	QueryTypeUpsert   QueryType = "UPSERT"
	QueryTypeTruncate QueryType = "TRUNCATE"
)

type DBType = string

const (
	// DriverMySQL is the driver name for MySQL databases.
	DriverMySQL DBType = "mysql"
	// DriverPostgres is the driver name for PostgreSQL databases.
	DriverPostgres DBType = "postgres"
	// DriverSQLLite is the driver name for SQLLite databases.
	DriverSQLLite DBType = "sqlite3"
	// DriverMSSQL is the driver name for Microsoft SQL Server databases.
	DriverMSSQL DBType = "sqlserver"
)
