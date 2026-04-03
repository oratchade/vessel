# fabric

A lightweight, multi-database SQL abstraction library for Go with support for MySQL, PostgreSQL, SQLite, and MSSQL.

[![GoDoc](https://godoc.org/tounilab.com/fabric?status.svg)](https://godoc.org/tounilab.com/fabric)
[![Go Report Card](https://goreportcard.com/badge/tounilab.com/fabric)](https://goreportcard.com/report/tounilab.com/fabric)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- 🗄️ **Multi-Database Support** - MySQL, PostgreSQL, SQLite, MSSQL with unified API
- 🔒 **Type-Safe Queries** - Parameterized SQL with automatic escaping
- 🎯 **Query Builder** - Fluent DSL for dynamic SQL construction
- 🔄 **Transaction Support** - ACID compliance with automatic rollback on panic
- 📊 **Connection Pooling** - Per-dialect statistics and configuration
- ✨ **Zero-Copy Row Scanning** - Efficient field mapping to Go types
- 📡 **OpenTelemetry Tracing** - Distributed tracing for all database operations
- 🧪 **Comprehensive Testing** - 829 unit tests with 100% pass rate

## Installation

```bash
go get tounilab.com/fabric
```

Requires Go 1.26.0 or later.

## Status & Releases

**Current Version**: [v1.0.0](RELEASES.md) (Stable ✅)

Fabric v1.0.0 is the first stable release with:

- ✅ Full multi-database support (MySQL, PostgreSQL, SQLite, MSSQL)
- ✅ 829 comprehensive tests (100% pass rate)
- ✅ Retry integration with automatic backoff strategies
- ✅ Production-ready and battle-tested
- ✅ Complete documentation and examples

**See**: [RELEASES.md](RELEASES.md) for release highlights | [CHANGELOG.md](CHANGELOG.md) for detailed changes

## OpenTelemetry Tracing & Observability

All database operations are automatically instrumented with OpenTelemetry for distributed tracing and observability. This includes metrics and spans for all queries, transactions, and row scanning operations.

### Configuration

Tracing is **enabled by default**. To disable tracing, set the `OTEL_ENABLED` environment variable:

```bash
# Disable tracing
export OTEL_ENABLED=false

# Enable tracing (default)
export OTEL_ENABLED=true
```

When disabled, all tracing operations are replaced with no-op implementations, providing zero overhead.

### Captured Operations

Traces include:

- Database operations: `Ping`, `Begin`, `Get`, `GetRaw`, `GetByID`, `Insert`, `Inserts`, `Update`, `Delete`, `Query`, `QueryRaw`, `Exec`, `Explain`
- Transactions: `Commit`, `Rollback`, `WithTransaction`
- Row scanning: `ScanRowsTo[T]` with full error context
- Semantic conventions from OpenTelemetry specification
- Span status and error recording for observability

### Zero Overhead When Disabled

When `OTEL_ENABLED=false`, the library uses OpenTelemetry's no-op tracer provider, resulting in:

- ✅ No performance impact
- ✅ No memory allocations for tracing
- ✅ Complete trace API compatibility
- ✅ Easy enable/disable via environment variable

## Quick Start

### Basic Connection

```go
package main

import (
    "context"
    "log"

    db "tounilab.com/fabric/db/v1"
)

func main() {
    // Open a connection
    database, err := db.NewDB(db.MysqlConfig{
        User:     "user",
        Password: "password",
        Host:     "localhost",
        Port:     3306,
        Database: "mydb",
    }, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()

    // Health check
    if err := database.Ping(context.Background()); err != nil {
        log.Fatal(err)
    }

    log.Println("Connected successfully!")
}
```

### Querying Data

```go
ctx := context.Background()

// Get all users
users, err := database.Get(ctx, "users", []string{"id", "name", "email"}, nil, nil, nil)
if err != nil {
    log.Fatal(err)
}

for _, user := range users {
    log.Printf("User: %v\n", user)
    // Output: User: map[string]any{"id": 1, "name": "Alice", "email": "alice@example.com"}
}
```

### Inserting Data

```go
result, err := database.Insert(ctx, "users", map[string]any{
    "name":  "Bob",
    "email": "bob@example.com",
    "age":   30,
}, nil)
if err != nil {
    log.Fatal(err)
}

log.Printf("Inserted %d rows, ID: %v\n", result.RowsAffected, result.LastInsertID)
```

### Bulk Insert (Multiple Rows)

```go
// Insert multiple rows efficiently in a single query
data := []map[string]any{
    {"name": "Alice", "email": "alice@example.com", "age": 28},
    {"name": "Bob", "email": "bob@example.com", "age": 32},
    {"name": "Charlie", "email": "charlie@example.com", "age": 25},
}

result, err := database.Inserts(ctx, "users", data, nil)
if err != nil {
    log.Fatal(err)
}

log.Printf("Inserted %d rows\n", result.RowsAffected)
// Output: Inserted 3 rows
```

**Benefits of Bulk Insert:**

- Single database round-trip (3 rows in 1 query vs 3 separate queries)
- Automatic parameterization (no SQL injection risk)
- Works identically across MySQL, PostgreSQL, SQLite, and MSSQL

### Updating Data

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

updates := map[string]any{
    "email": "newemail@example.com",
    "age":   31,
}

result, err := database.Update(ctx, "users", updates,
    cdt.NewExpr().Column("id").Op("=").Value(1), nil)
if err != nil {
    log.Fatal(err)
}

log.Printf("Updated %d rows\n", result.RowsAffected)
```

### Deleting Data

```go
result, err := database.Delete(ctx, "users",
    cdt.NewExpr().Column("age").Op(">").Value(50), nil)
if err != nil {
    log.Fatal(err)
}

log.Printf("Deleted %d rows\n", result.RowsAffected)
```

### Using Transactions

```go
err := database.WithTransaction(ctx, func(tx db.Tx) error {
    // Insert user
    _, err := tx.Insert(ctx, "users", map[string]any{
        "name": "Charlie",
    }, nil)
    if err != nil {
        return err
    }

    // Insert profile
    _, err = tx.Insert(ctx, "profiles", map[string]any{
        "user_id": 3,
        "bio":     "Developer",
    }, nil)
    if err != nil {
        return err
    }

    // Both inserts commit on successful return
    return nil
})

if err != nil {
    log.Fatal(err)
}
```

### Raw SQL Queries

```go
// Execute custom SQL
results, err := database.Query(ctx,
    "SELECT id, name FROM users WHERE age > ?", 25)
if err != nil {
    log.Fatal(err)
}

for _, row := range results {
    log.Printf("ID: %v, Name: %v\n", row["id"], row["name"])
}
```

## Setting Up Test Environment

Fabric uses environment variables for database test credentials, making it easy to configure testing for any database.

### Quick Setup

```bash
# Copy example environment file
cp .env.example .env

# Optionally customize for your local setup
# See docs/ENVIRONMENT_VARIABLES.md for complete configuration guide

# Run unit tests
make test
```

**For local development with defaults**, just run tests directly—environment variables are optional with sensible fallbacks.

### Test Configuration

Test credentials are managed via environment variables:

- **MySQL**: `DB_MYSQL_USER`, `DB_MYSQL_PASSWORD`, `DB_MYSQL_HOST`, etc.
- **PostgreSQL**: `DB_POSTGRES_USER`, `DB_POSTGRES_PASSWORD`, etc.
- **SQLite**: Direct file path (no server needed)
- **MSSQL**: `DB_MSSQL_USER`, `DB_MSSQL_PASSWORD`, etc.

For detailed configuration options, defaults, and CI/CD setup, see [docs/ENVIRONMENT_VARIABLES.md](./docs/ENVIRONMENT_VARIABLES.md).

### Running Tests

```bash
# Run all unit tests (no Docker needed)
make test

# Run SQLite integration tests (fast, no Docker)
make integration-test-sqlite

# Run all database integration tests (requires Docker)
docker-compose -f docker-compose.test.yml up -d
make integration-test-all
docker-compose -f docker-compose.test.yml down

# View coverage report
make coverage
make cover-html  # Opens HTML coverage report in browser
```

**All 694 unit tests passing** ✅ with comprehensive coverage across MySQL, PostgreSQL, SQLite, and MSSQL.

See [CODE_REVIEW.md](./docs/CODE_REVIEW.md) for code quality standards and testing requirements.

### Type-Safe Row Scanning with ScanRowsTo

For advanced use cases, use `ScanRowsTo` to efficiently scan rows into strongly-typed structs:

```go
import db "tounilab.com/fabric/db/v1"

// Define your struct matching SELECT columns
type User struct {
    ID    int
    Name  string
    Email string
    Age   int
}

// Execute raw SQL and get unscanned rows
rowsAdapter, err := database.GetRaw(ctx, "users", []string{"*"}, nil, nil, nil)
if err != nil {
    log.Fatal(err)
}
defer rowsAdapter.Close()

// Scan rows into typed structs
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
if err != nil {
    log.Fatal(err)
}

// Use typed results
for _, user := range users {
    log.Printf("User: %s <%s> (Age: %d)\n", user.Name, user.Email, user.Age)
}
```

⚠️ **Important: RowsAdapter Lifecycle Management**

The methods `GetRaw()`, `GetByIDRaw()`, and `QueryRaw()` return a `RowsAdapter` which you must explicitly close. This design allows you the flexibility to:

- Use `db.ScanRowsTo[T](ctx, ...)` for type-safe scanning into structs
- Use third-party row scanning libraries of your choice
- Implement custom row processing logic

**You are responsible for closing the RowsAdapter to prevent resource leaks.** Always use `defer` to ensure proper cleanup:

```go
rowsAdapter, err := database.GetRaw(ctx, "users", []string{"*"}, nil, nil, nil)
if err != nil {
    log.Fatal(err)
}
defer rowsAdapter.Close()  // ⚠️ REQUIRED: Always close the adapter

// Now you can scan or process the rows
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
// ... your code
```

Failure to close the adapter may result in:

- Database connection pool exhaustion
- Memory leaks (unclosed database cursors)
- Degraded application performance

**Benefits:**

- ✅ **Type safety** - Compile-time column mapping verification
- ✅ **Zero-copy** - Efficient field scanning without intermediate allocations
- ✅ **Null handling** - Automatic SQL.Null\* type conversion
- ✅ **Custom queries** - Full SQL flexibility with typed results
- ✅ **Flexibility** - Works with any row scanning approach you prefer

### Query Introspection and Performance Analysis

Generate SQL queries without executing them to inspect, log, or validate before execution. This is especially useful for debugging and performance analysis:

```go
// Introspect a GET query
query, args, err := database.GetQuery(
    "users",
    []string{"id", "name", "email"},
    nil,
    cdt.NewExpr().Column("age").Op(">").Value(25),
    nil,
)
if err != nil {
    log.Fatal(err)
}

log.Printf("Generated SQL: %s\n", query)
log.Printf("Parameters: %v\n", args)
// Output:
// Generated SQL: SELECT id, name, email FROM users WHERE age > ?
// Parameters: [25]

// Execute the EXPLAIN query to analyze performance
explainRows, err := database.Explain(context.Background(), query, args...)
if err != nil {
    log.Fatal(err)
}
defer explainRows.Close()

// Read and display execution plan
for explainRows.Next() {
    var line string
    if err := explainRows.Scan(&line); err != nil {
        log.Fatal(err)
    }
    log.Println(line)
}
```

**Available Query Introspection Methods:**

- `GetQuery()` - Preview SELECT queries
- `GetByIDQuery()` - Preview SELECT queries by primary key
- `InsertQuery()` - Preview INSERT queries
- `InsertsQuery()` - Preview bulk INSERT queries
- `UpdateQuery()` - Preview UPDATE queries
- `DeleteQuery()` - Preview DELETE queries
- `Explain()` - Execute EXPLAIN to analyze query performance

**Benefits:**

- ✅ **SQL Injection Prevention** - When combined with xxxQuery methods, ensures safe, parameterized SQL
- ✅ **Query Debugging** - Verify the actual SQL before execution
- ✅ **Performance Analysis** - Run EXPLAIN to understand query execution plans
- ✅ **Query Logging** - Log all generated SQL for audit trails
- ✅ **Batch Operations** - Build and verify multiple queries before execution

### FluentDB - Fluent Query Builder API

For a more ergonomic, chainable interface, use **FluentDB** - a fluent/builder-style API that wraps DBActions with a readable, SQL-like syntax:

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

fdb := db.NewFluentDB(database, ctx)

// SELECT with chaining
users, err := fdb.Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    OrderBy("name", "ASC").
    Limit(10).
    Get()

// INSERT
result, err := fdb.Insert().
    Into("users").
    Set("name", "Alice").
    Set("email", "alice@example.com").
    Exec()

// UPDATE with conditions
result, err := fdb.Update("users").
    Set("status", "active").
    Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
    Exec()

// DELETE
result, err := fdb.Delete().
    From("users").
    Where(cdt.NewExpr().Column("active").Op("=").Value(false)).
    Limit(10).
    Exec()

// COUNT
count, err := fdb.Select("users").Count()

// Single row retrieval
user, err := fdb.Select("users", "id", "name").
    Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
    One()
```

**FluentDB Features:**

- ✅ **Chainable API** - Methods return builders for fluent method chaining
- ✅ **Readable** - Code reads naturally from left-to-right like SQL
- ✅ **Type-Safe** - Compiler catches method order errors
- ✅ **100% Code Reuse** - Delegates to existing DBActions (no duplication)
- ✅ **JOINs** - INNER, LEFT, RIGHT joins with multiple conditions
- ✅ **Transactions** - Works seamlessly with transactions via `WithTx()`
- ✅ **Pagination** - Built-in LIMIT and OFFSET
- ✅ **Sorting** - Chainable ORDER BY with multiple columns

**Examples:**

```go
// SELECT with INNER JOIN
fdb.Select("users", "users.id", "users.name", "roles.name").
    Join(cdt.Join{
        Type:  "INNER",
        Table: "roles",
        Conditions: []cdt.JoinCdt{{
            Left:  "users.role_id",
            Right: "roles.id",
        }},
    }).
    Get()

// Bulk INSERT
users := []map[string]any{
    {"name": "Alice", "email": "alice@example.com"},
    {"name": "Bob", "email": "bob@example.com"},
}
fdb.Insert().Into("users").ValuesBulk(users).Exec()

// Complex UPDATE
fdb.Update("users").
    Set("status", "inactive").
    Where(cdt.NewExpr().Column("last_login").Op("<").Value("2023-01-01")).
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    Limit(1000).
    Exec()

// Pagination
fdb.Select("users", "id", "name").
    OrderBy("created_at", "DESC").
    Limit(20).
    Offset((page-1)*20).
    Get()
```

See [FluentDB Examples](./examples/fluentdb-example/README.md) for comprehensive tutorials on basic, advanced, and transaction-based usage.

## Database Support

| Feature               | MySQL | PostgreSQL | SQLite | MSSQL |
| --------------------- | ----- | ---------- | ------ | ----- |
| Basic CRUD            | ✅    | ✅         | ✅     | ✅    |
| Bulk Insert (Inserts) | ✅    | ✅         | ✅     | ✅    |
| Transactions          | ✅    | ✅         | ✅     | ✅    |
| Parameterized Queries | ✅    | ✅         | ✅     | ✅    |
| Query Introspection   | ✅    | ✅         | ✅     | ✅    |
| EXPLAIN Analysis      | ✅    | ✅         | ✅     | ✅    |
| Connection Pool Stats | ✅    | ✅         | ✅     | ✅    |
| Error Mapping         | ✅    | ✅         | ✅     | ✅    |

All operators are documented in the [Architecture Guide](./docs/ARCHITECTURE.md).

## Configuration

### MySQL

```go
database, err := db.NewDB(db.MysqlConfig{
    User:            "user",
    Password:        "password",
    Host:            "host",
    Port:            3306,
    Database:        "database",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
}, nil)
```

### PostgreSQL

```go
database, err := db.NewDB(db.PostgresConfig{
    User:         "user",
    Password:     "password",
    Host:         "host",
    Port:         5432,
    Database:     "database",
    PoolMaxConns: 25,
    PoolMinConns: 5,
}, nil)
```

### SQLite

```go
database, err := db.NewDB(db.SQLiteConfig{
    FilePath: "/path/to/database.db",
}, nil)
```

### MSSQL

```go
database, err := db.NewDB(db.MSSQLConfig{
    User:         "sa",
    Password:     "...",
    Host:         "localhost",
    Port:         1433,
    Database:     "mydb",
    MaxOpenConns: 25,
    MaxIdleConns: 5,
}, nil)
```

## Logger Adapters

Fabric supports structured logging with multiple popular Go logging libraries through its logger adapter system. You can use your preferred logging library without modifying Fabric's code.

### Using slog (Standard Library - Recommended)

The `slog` adapter works with Go's standard library structured logger (Go 1.21+):

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "os"

    db "tounilab.com/fabric/db/v1"
)

func main() {
    // Create an slog logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Create adapter
    adapter := db.NewSlogAdapter(logger)

    // Use with database
    database, err := db.NewDB(db.PostgresConfig{
        User:     "user",
        Password: "password",
        Host:     "localhost",
        Port:     5432,
        Database: "mydb",
    }, adapter)
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()
}
```

### Using logrus

The `logrus` adapter works with the popular logrus logging library:

```go
package main

import (
    "github.com/sirupsen/logrus"

    db "tounilab.com/fabric/db/v1"
)

func main() {
    // Create a logrus logger
    logrusLogger := logrus.New()
    logrusLogger.SetFormatter(&logrus.JSONFormatter{})

    // Create adapter
    adapter := db.NewLogrusAdapter(logrusLogger)

    // Use with database
    database, err := db.NewDB(db.MysqlConfig{
        User:     "user",
        Password: "password",
        Host:     "localhost",
        Port:     3306,
        Database: "mydb",
    }, adapter)
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()
}
```

### Using Zap

The `zap` adapter works with Uber's high-performance zap logging library:

```go
package main

import (
    "go.uber.org/zap"

    db "tounilab.com/fabric/db/v1"
)

func main() {
    // Create a zap logger
    zapLogger, _ := zap.NewProduction()
    defer zapLogger.Sync()

    // Create adapter
    adapter := db.NewZapAdapter(zapLogger)

    // Use with database
    database, err := db.NewDB(db.PostgresConfig{
        User:     "user",
        Password: "password",
        Host:     "localhost",
        Port:     5432,
        Database: "mydb",
    }, adapter)
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()
}
```

### Using Apex Log

The `apex/log` adapter works with the Apex log library:

```go
package main

import (
    "log"
    "os"

    apexlog "github.com/apex/log"
    "github.com/apex/log/handlers/json"

    db "tounilab.com/fabric/db/v1"
)

func main() {
    // Create an apex logger
    apexLogger := &apexlog.Logger{
        Handler: json.New(os.Stdout),
        Level:   apexlog.InfoLevel,
    }

    // Create adapter
    adapter := db.NewApexAdapter(apexLogger)

    // Use with database
    database, err := db.NewDB(db.SQLiteConfig{
        FilePath: "/path/to/database.db",
    }, adapter)
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()
}
```

### Passing nil for No Logging

If you don't need logging, you can pass `nil` as the logger:

```go
database, err := db.NewDB(db.MysqlConfig{
    User:     "user",
    Password: "password",
    Host:     "localhost",
    Port:     3306,
    Database: "mydb",
}, nil)
```

## Error Handling

The library provides structured error handling with database-dialect-specific error mapping. See [ERROR_HANDLING.md](./docs/ERROR_HANDLING.md) for comprehensive guidance on error handling patterns.

```go
import "tounilab.com/fabric/db/v1/dberror"

result, err := database.Insert(ctx, "users", data, nil)
if err != nil {
    if errors.Is(err, dberror.ErrDuplicateKey) {
        log.Println("Email already exists")
    } else if errors.Is(err, dberror.ErrConnectionFailed) {
        log.Println("Database connection lost")
    } else {
        log.Fatal(err)
    }
}
```

## Monitoring

### Connection Pool Statistics

```go
stats, err := database.PoolStats()
if err != nil {
    log.Fatal(err)
}

log.Printf("Open Connections: %d\n", stats.OpenConnections)
log.Printf("In Use: %d\n", stats.InUse)
log.Printf("Idle: %d\n", stats.Idle)
log.Printf("Wait Count: %d\n", stats.WaitCount)
log.Printf("Wait Duration: %v\n", stats.WaitDuration)
```

## Multi-Database Management

The `DBManager` provides seamless access to multiple database connections with priority-based routing, automatic load-balancing, and async queries. Perfect for scenarios with primary/replica setups, multi-region deployments, or application-level sharding.

**Key Features:**

- 🎯 **Priority-Based Selection** - Route queries to preferred databases
- ⚖️ **Load Balancing** - Distribute queries among same-priority databases
- 🔧 **Worker Pools** - Configurable read/write workers per database
- 📬 **Async Queries** - Channel-based responses for non-blocking operations
- 🛡️ **Backpressure Handling** - Bounded queues prevent resource exhaustion

For complete guide, configuration examples, and use cases, see [docs/DBManager.md](./docs/DBManager.md). Working examples available in [examples/manager-example/](./examples/manager-example/).

## Plugin System

The fabric supports a registry-based plugin system that allows you to register custom database drivers without modifying the core library.

### Creating a Custom Driver

Implement the `DriverFactory` interface and register it in an `init()` function:

```go
package mydb

import (
    "context"
    "fmt"
    db "tounilab.com/fabric/db/v1"
    "tounilab.com/fabric/db/v1/plugin"
)

// Config implements db.DBConfig for your custom database
type Config struct {
    Host     string
    Port     int
    User     string
    Password string
    Database string
}

func (c *Config) Driver() string { return "mydb" }
func (c *Config) DSN() string    { return fmt.Sprintf("...") }

// Factory implements plugin.DriverFactory
type Factory struct{}

func (f *Factory) Name() string {
    return "mydb"
}

func (f *Factory) Create(ctx context.Context, cfg any) (any, error) {
    mydbCfg, ok := cfg.(*Config)
    if !ok {
        return nil, fmt.Errorf("expected *Config, got %T", cfg)
    }
    // Create and return your DB implementation
    return NewMyDB(mydbCfg)
}

// init() auto-registers the driver when the package is imported
func init() {
    plugin.MustRegister(&Factory{})
}
```

### Using a Custom Driver

Simply import the plugin package (with blank import `_`) and use it:

```go
import (
    "tounilab.com/fabric/db/v1"
    _ "mydb"  // Auto-registers via init()
)

func main() {
    cfg := &mydb.Config{
        Host:     "localhost",
        Port:     5432,
        User:     "user",
        Password: "password",
        Database: "mydb",
    }

    database, err := db.NewDB(cfg, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer database.Close()

    // Use database normally
}
```

### Reusing Built-in Drivers

Plugin authors can reuse built-in driver implementations:

```go
// If your database is compatible with PostgreSQL wire protocol
func (f *Factory) Create(ctx context.Context, cfg any) (any, error) {
    customCfg, _ := cfg.(*Config)

    // Convert to PostgreSQL config and reuse the implementation
    pgCfg := &db.PostgresConfig{
        Host:     customCfg.Host,
        Port:     customCfg.Port,
        User:     customCfg.User,
        Password: customCfg.Password,
        Database: customCfg.Database,
    }

    return db.PostgresCfgToDB(pgCfg)
}
```

Exported driver functions available for reuse:

- `MySQLCfgToDB(cfg DBConfig) (DB, error)`
- `PostgresCfgToDB(cfg DBConfig) (DB, error)`
- `SQLiteCfgToDB(cfg DBConfig) (DB, error)`
- `MSSQLCfgToDB(cfg DBConfig) (DB, error)`

### Plugin Registry API

The `plugin` package provides these functions:

```go
package plugin

// Register a driver (prevents duplicate registrations)
func Register(factory DriverFactory) error

// Register a driver, panic on error (use in init())
func MustRegister(factory DriverFactory)

// Look up a registered driver by name
func Get(driverName string) (DriverFactory, bool)

// List all registered driver names
func List() []string

// Remove a driver (testing)
func Unregister(driverName string) error

// Clear all drivers (testing)
func Clear()
```

## Examples

See the [examples](./examples) directory for complete working examples:

- [`explain-example/`](./examples/explain-example/) - Query introspection with EXPLAIN analysis
- [`manager-example/`](./examples/manager-example/) - Multi-database management:
  - `basic/` - Basic DBManager usage patterns
  - `error-handling/` - Comprehensive error handling
  - `priority-selection/` - Priority-based database selection and routing
- [`plugin-example/`](./examples/plugin-example/) - Custom database driver plugin system with CockroachDB example
- [`tester-example/`](./examples/tester-example/) - Tester utility and test helpers

## Type Support

Supported scalar types for row scanning:

- **Basic Types**: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`, `[]byte`
- **SQL Null Types**: `sql.NullString`, `sql.NullInt32`, `sql.NullInt64`, `sql.NullFloat64`, `sql.NullBool`, `sql.NullTime`

For detailed NULL handling patterns, see [ERROR_HANDLING.md](./docs/ERROR_HANDLING.md).

## Contributing

Contributions are welcome! Please read our [CONTRIBUTING.md](./CONTRIBUTING.md) guidelines before submitting PRs.

## Performance

The library is designed for production use with focus on:

- **Zero-copy scanning** - Efficient field mapping without intermediate allocations
- **Connection pooling** - Per-dialect optimizations (pgxpool for PostgreSQL)
- **Prepared statements** - Parameterized queries throughout
- **Memory efficiency** - Reusable row adapters and minimal allocations

## License

MIT License - see [LICENSE.md](./LICENSE.md)

## Documentation

**Release & Changelog**:

- 🎉 **[RELEASES.md](./RELEASES.md)** - Quick release overview and version history
- 📝 **[CHANGELOG.md](./CHANGELOG.md)** - Detailed changelog (Keep a Changelog format)

**Guides & References**:

- 📐 **[ARCHITECTURE.md](./docs/ARCHITECTURE.md)** - Complete system design, layers, components, and extension points
- 📋 **[CODE_REVIEW.md](./docs/CODE_REVIEW.md)** - Code quality standards and testing requirements
- ⚠️ **[ERROR_HANDLING.md](./docs/ERROR_HANDLING.md)** - Error handling patterns and NULL type mapping
- 🔧 **[DBMANAGER.md](./docs/DBMANAGER.md)** - Multi-database management and load balancing
- 📦 **[ENVIRONMENT_VARIABLES.md](./docs/ENVIRONMENT_VARIABLES.md)** - Configuration and environment setup
- 📚 **[LINTING.md](./docs/LINTING.md)** - Code style and linting standards

## Support

- 🐛 [Issue Tracker](https://github.com/oratchade/fabric/issues)
- 💬 [Discussions](https://github.com/oratchade/fabric/discussions)

## Changelog

See [RELEASES.md](./RELEASES.md) for version history and release notes.

For detailed changes between versions, see [CHANGELOG.md](./CHANGELOG.md) (Keep a Changelog format).

---

**Last Updated:** March 2026  
**Status:** ✅ Production Ready
