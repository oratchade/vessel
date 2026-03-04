# db-connector

A lightweight, multi-database SQL abstraction library for Go with support for MySQL, PostgreSQL, SQLite, and MSSQL.

[![GoDoc](https://godoc.org/github.com/oratchade/db-connector?status.svg)](https://godoc.org/github.com/oratchade/db-connector)
[![Go Report Card](https://goreportcard.com/badge/github.com/oratchade/db-connector)](https://goreportcard.com/report/github.com/oratchade/db-connector)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- 🗄️ **Multi-Database Support** - MySQL, PostgreSQL, SQLite, MSSQL with unified API
- 🔒 **Type-Safe Queries** - Parameterized SQL with automatic escaping
- 🎯 **Query Builder** - Fluent DSL for dynamic SQL construction
- 🔄 **Transaction Support** - ACID compliance with automatic rollback on panic
- 📊 **Connection Pooling** - Per-dialect statistics and configuration
- ✨ **Zero-Copy Row Scanning** - Efficient field mapping to Go types
- 🧪 **Comprehensive Testing** - 97+ test cases with 100% pass rate

## Installation

```bash
go get tounilab.com/db-connector
```

Requires Go 1.26.0 or later.

## Quick Start

### Basic Connection

```go
package main

import (
    "context"
    "log"

    "tounilab.com/db-connector/v1/db"
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

### Updating Data

```go
import (
    cdt "tounilab.com/db-connector/pkg/query/condition"
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

### Type-Safe Row Scanning with ScanRowsTo

For advanced use cases, use `ScanRowsTo` to efficiently scan rows into strongly-typed structs:

```go
import "tounilab.com/db-connector/v1/db"

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
users, err := db.ScanRowsTo[User](rowsAdapter)
if err != nil {
    log.Fatal(err)
}

// Use typed results
for _, user := range users {
    log.Printf("User: %s <%s> (Age: %d)\n", user.Name, user.Email, user.Age)
}
```

**Benefits:**

- ✅ **Type safety** - Compile-time column mapping verification
- ✅ **Zero-copy** - Efficient field scanning without intermediate allocations
- ✅ **Null handling** - Automatic SQL.Null\* type conversion
- ✅ **Custom queries** - Full SQL flexibility with typed results

## Database Support

| Feature               | MySQL | PostgreSQL | SQLite | MSSQL |
| --------------------- | ----- | ---------- | ------ | ----- |
| Basic CRUD            | ✅    | ✅         | ✅     | ✅    |
| Transactions          | ✅    | ✅         | ✅     | ✅    |
| Parameterized Queries | ✅    | ✅         | ✅     | ✅    |
| Connection Pool Stats | ✅    | ✅         | ✅     | ✅    |
| Error Mapping         | ✅    | ✅         | ✅     | ✅    |

See [OPERATORS_COMPATIBILITY.md](./OPERATORS_COMPATIBILITY.md) for detailed operator support by dialect.

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
database, err := db.NewDB(db.SQLLiteConfig{
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

## Error Handling

The library provides structured error handling with database-dialect-specific error mapping. See [ERROR_HANDLING.md](./ERROR_HANDLING.md) for comprehensive guidance on error handling patterns.

```go
import "tounilab.com/db-connector/v1/db/dberror"

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

## Examples

See the [examples](./examples) directory for complete working examples:

- `basic_crud.go` - Insert, Get, Update, Delete operations
- `query_builder.go` - Dynamic query construction with the builder DSL
- `transactions.go` - Multi-step transactions with automatic rollback
- `error_handling.go` - Comprehensive error handling patterns
- `pool_stats.go` - Connection pool monitoring
- `raw_sql.go` - Custom SQL execution

## Type Support

Supported scalar types for row scanning:

- **Basic Types**: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`, `[]byte`
- **SQL Null Types**: `sql.NullString`, `sql.NullInt32`, `sql.NullInt64`, `sql.NullFloat64`, `sql.NullBool`, `sql.NullTime`

See [SQL_NULL_TYPES.md](./docs/SQL_NULL_TYPES.md) for detailed NULL handling.

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

## Support

- 📖 [Full API Documentation](./docs)
- 🐛 [Issue Tracker](https://github.com/oratchade/db-connector/issues)
- 💬 [Discussions](https://github.com/oratchade/db-connector/discussions)

## Changelog

See [RELEASES.md](./RELEASES.md) for version history and release notes.

For detailed changes between versions, see [CHANGELOG.md](./CHANGELOG.md) (Keep a Changelog format). For migration guides, see [MIGRATION.md](./MIGRATION.md).

---

**Last Updated:** March 2026  
**Status:** ✅ Production Ready
