// Package plugin provides registration hooks for custom Vessel database
// drivers.
//
// Custom drivers implement DriverFactory and register themselves with Register
// or MustRegister, usually from an init function in the driver package. NewDB
// checks the plugin registry before falling back to Vessel's built-in MySQL,
// PostgreSQL, SQLite, and Microsoft SQL Server drivers.
//
// Custom drivers that return row results should return values compatible with
// db/v1.RowsProvider, database/sql rows, or pgx rows so Vessel's row adapter can
// scan results consistently.
package plugin
