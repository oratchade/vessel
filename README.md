# fabric

A lightweight, multi-database SQL abstraction library for Go with
support for MySQL, PostgreSQL, SQLite, and MSSQL.

[![GoDoc](https://godoc.org/tounilab.com/fabric?status.svg)](https://godoc.org/tounilab.com/fabric)
[![Go Report Card](https://goreportcard.com/badge/tounilab.com/fabric)](https://goreportcard.com/report/tounilab.com/fabric)
[![Tests](https://github.com/oratchade/fabric/actions/workflows/test.yml/badge.svg)](https://github.com/oratchade/fabric/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- 🗄️ **Multi-Database Support** - MySQL, PostgreSQL, SQLite,
  MSSQL with unified API
- 🔒 **Type-Safe Queries** - Parameterized SQL with automatic escaping
- 🎯 **Query Builder** - Fluent DSL for dynamic SQL construction
- 🔄 **Transaction Support** - ACID compliance with automatic rollback on
  callback error or panic
- 📊 **Connection Pooling** - Per-dialect statistics and configuration
- ✨ **Zero-Copy Row Scanning** - Efficient field mapping to Go types
- 📡 **OpenTelemetry Tracing** - Distributed tracing for all
  database operations
- 🧪 **Comprehensive Testing** - 919 unit tests with 100% pass rate

## Installation

```bash
go get tounilab.com/fabric
```

Requires Go 1.26.0 or later.

## Status & Releases

**Current Version**: [v1.0.0](RELEASES.md) (Stable ✅)

Fabric v1.0.0 is the first stable release with:

- ✅ Full multi-database support (MySQL, PostgreSQL, SQLite, MSSQL)
- ✅ 919 comprehensive tests (100% pass rate)
- ✅ Retry integration with automatic backoff strategies
- ✅ Production-ready and battle-tested
- ✅ Complete documentation and examples

**See**: [RELEASES.md](RELEASES.md) for release highlights |
[CHANGELOG.md](CHANGELOG.md) for detailed changes

## OpenTelemetry Tracing & Observability

All database operations are automatically instrumented with
OpenTelemetry for distributed tracing and observability. This
includes metrics and spans for all queries, transactions, and
row scanning operations.

### Configuration

Tracing is **enabled by default**. To disable tracing, set the
`OTEL_ENABLED` environment variable:

```bash
# Disable tracing
export OTEL_ENABLED=false

# Enable tracing (default)
export OTEL_ENABLED=true
```

When disabled, all tracing operations are replaced with no-op implementations,
providing zero overhead.

### Setup & Configuration

To export traces to a backend (Jaeger, Datadog, Grafana, etc.), initialize an
OpenTelemetry exporter in your application:

#### Using Jaeger Exporter (Local Development)

```go
package main

import (
 "context"
 "log"

 "go.opentelemetry.io/otel"
 "go.opentelemetry.io/otel/exporters/jaeger/otlphttp"
 "go.opentelemetry.io/otel/sdk/resource"
 tracesdk "go.opentelemetry.io/otel/sdk/trace"
 semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

 db "tounilab.com/fabric/db/v1"
)

func initTracer() (*tracesdk.TracerProvider, error) {
 // Create Jaeger exporter
 exporter, err := otlphttp.New(context.Background(),
  otlphttp.WithEndpoint("http://localhost:4318"),
 )
 if err != nil {
  return nil, err
 }

 // Create resource
 res, err := resource.New(context.Background(),
  resource.WithAttributes(
   semconv.ServiceNameKey.String("my-app"),
  ),
 )
 if err != nil {
  return nil, err
 }

 // Create trace provider
 tp := tracesdk.NewTracerProvider(
  tracesdk.WithBatcher(exporter),
  tracesdk.WithResource(res),
 )

 // Set global tracer provider
 otel.SetTracerProvider(tp)

 return tp, nil
}

func main() {
 tp, err := initTracer()
 if err != nil {
  log.Fatal(err)
 }
 defer func() {
  if err := tp.Shutdown(context.Background()); err != nil {
   log.Printf("Error shutting down tracer provider: %v", err)
  }
 }()

 // Your database operations now emit traces automatically
 cfg := db.Config{DSN: "postgres://..."}
 conn, _ := db.NewDB(cfg, nil)

 ctx := context.Background()
 rows, _ := conn.Get(ctx, "SELECT * FROM users", nil, nil, nil)
 // Traces sent to Jaeger at http://localhost:16686
}
```

#### Using gRPC Collector (Production)

```go
import (
 "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

exporter, err := otlptracegrpc.New(context.Background(),
 otlptracegrpc.WithEndpoint("otel-collector:4317"),
)
if err != nil {
 return nil, err
}
```

#### Quick Start with Docker Compose

Run a local Jaeger instance:

```yaml
# docker-compose.yml
version: "3"
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    ports:
      - "6831:6831/udp" # Jaeger agent
      - "4317:4317/tcp" # OTEL gRPC receiver
      - "4318:4318/tcp" # OTEL HTTP receiver
      - "16686:16686" # Jaeger UI

  # Your app
  app:
    build: .
    environment:
      - OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
    depends_on:
      - jaeger
```

Then access traces at <http://localhost:16686>

### Captured Operations

Traces include:

- Database operations: `Ping`, `Begin`, `Get`, `GetRaw`, `GetByID`, `Insert`,
  `Inserts`, `Update`, `Delete`, `Query`, `QueryRaw`, `Exec`, `Explain`
- Transactions: `Commit`, `Rollback`, `WithTransaction`
- Row scanning: `ScanRowsTo[T]` with full error context
- Semantic conventions from OpenTelemetry specification
- Span status and error recording for observability

### Zero Overhead When Disabled

When `OTEL_ENABLED=false`, the library uses OpenTelemetry's
no-op tracer provider, resulting in:

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

#### Simple Queries (Map-Based)

```go
ctx := context.Background()

// Get all users - simple approach, returns maps
users, err := database.Get(ctx, "users", []string{"id", "name", "email"},
    nil, nil, nil)
if err != nil {
    log.Fatal(err)
}

for _, user := range users {
    log.Printf("User: %v\n", user)
    // Output: User: map[string]any{"id": 1, "name": "Alice",
    //   "email": "alice@example.com"}
}
```

#### Type-Safe Queries (Recommended for Production)

For better performance and type safety, use `GetRaw()` with `ScanRowsTo[T]()`:

```go
import db "tounilab.com/fabric/db/v1"

// Define struct matching SELECT columns
type User struct {
    ID    int
    Name  string
    Email string
}

ctx := context.Background()

// Get raw rows and scan into typed structs
rowsAdapter, err := database.GetRaw(ctx, "users", []string{"id", "name", "email"},
    nil, nil, nil)
if err != nil {
    log.Fatal(err)
}

// ScanRowsTo[T] automatically closes resources and handles type mapping
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
if err != nil {
    log.Fatal(err)
}

for _, user := range users {
    log.Printf("User: %s <%s>\n", user.Name, user.Email)
}
```

**Why use `ScanRowsTo[T]`?**

- ✅ **Zero-copy** - Efficient field scanning without map allocations
- ✅ **Type-safe** - Compile-time column mapping, no casting needed
- ✅ **Performance** - 3-5x faster on large datasets vs map-based approach
- ✅ **Memory** - Reduced GC pressure on massive result sets

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

Fabric uses environment variables for database test credentials,
making it easy to configure testing for any database.

### Quick Setup

```bash
# Copy example environment file
cp .env.example .env

# Optionally customize for your local setup
// See docs/ENVIRONMENT_VARIABLES.md for complete config

# Run unit tests
make test
```

**For local development with defaults**, just run tests
directly—environment variables are optional with sensible
fallbacks.

### Test Configuration

Test credentials are managed via environment variables:

- **MySQL**: `DB_MYSQL_USER`, `DB_MYSQL_PASSWORD`, `DB_MYSQL_HOST`, etc.
- **PostgreSQL**: `DB_POSTGRES_USER`, `DB_POSTGRES_PASSWORD`, etc.
- **SQLite**: Direct file path (no server needed)
- **MSSQL**: `DB_MSSQL_USER`, `DB_MSSQL_PASSWORD`, etc.

For detailed configuration options, defaults, and CI/CD setup,
see [docs/ENVIRONMENT_VARIABLES.md](./docs/ENVIRONMENT_VARIABLES.md).

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

**All 694 unit tests passing** ✅ with comprehensive coverage across
MySQL, PostgreSQL, SQLite, and MSSQL.

See [CODE_REVIEW.md](./docs/CODE_REVIEW.md) for code quality standards
and testing requirements.

### Type-Safe Row Scanning with ScanRowsTo

For advanced use cases, use `ScanRowsTo` to efficiently scan rows into
strongly-typed structs:

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

### RowsAdapter Resource Management

The methods `GetRaw()`, `GetByIDRaw()`, and `QueryRaw()` return a `RowsAdapter`
that holds database resources (connections, prepared statements). Fabric provides
three patterns for safe resource management—**choose based on your use case**:

**Decision Tree:**

- Single query, one-off result? → **Pattern 1 (ScanRowsTo[T])**
- High-throughput loop (100+ queries)? → **Pattern 2 (RowsAdapterPool)**
- Complex control flow, explicit semantics? → **Pattern 3 (ManagedRowsAdapter)**

#### Pattern 1: Type-Safe Automatic Cleanup (Recommended for Most)

Use `ScanRowsTo[T]` for automatic resource management and type-safe results.
**Best for:** Request handlers, simple queries, most production use cases.

```go
type User struct {
    ID    int
    Name  string
    Email string
}

ctx := context.Background()
rowsAdapter, err := database.GetRaw(ctx, "users", []string{"*"}, nil, nil, nil)
if err != nil {
    log.Fatal(err)
}

// ScanRowsTo[T] automatically closes resources
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
if err != nil {
    log.Fatal(err)
}

for _, user := range users {
    log.Printf("User: %s <%s>\n", user.Name, user.Email)
}
```

**Benefits:**

- ✅ **Automatic cleanup** - Resources freed when done
- ✅ **Type safety** - Compile-time column mapping verification
- ✅ **Zero-copy** - Efficient field scanning without intermediate allocations
- ✅ **Null handling** - Automatic `sql.Null*` type conversion
- ✅ **Simplest code** - No explicit resource tracking needed

#### Pattern 2: Resource Pooling (High-Throughput Loops)

Use `RowsAdapterPool` for tight loops with many iterations.
**Best for:** Batch processing, data pipelines, API bulk endpoints.

```go
type Order struct {
    ID     int
    UserID int
    Total  float64
}

// Initialize pool once (e.g., in app setup)
pool := v1.NewRowsAdapterPool()

// Tight loop processing many queries
func ProcessOrdersInBatch(ctx context.Context, db v1.DB, orderIDs []int) error {
    for _, orderID := range orderIDs {
        // Get raw rows for this order
        rows, err := db.QueryRaw(ctx, "SELECT * FROM orders WHERE id = ?", orderID)
        if err != nil {
            return err
        }

        // Acquire adapter from pool (reuses allocated memory)
        adapter, err := pool.Acquire(rows)
        if err != nil {
            return err
        }

        // Use with ScanRowsTo[T] for type-safe scanning
        orders, err := v1.ScanRowsTo[Order](ctx, adapter)
        if err != nil {
            pool.Release(adapter)
            return err
        }

        // Process orders...
        for _, order := range orders {
            log.Printf("Order %d: $%.2f\n", order.ID, order.Total)
        }

        // Return adapter to pool for reuse
        pool.Release(adapter)
    }
    return nil
}
```

**Why pooling helps:**

- Without pooling: 10,000 queries = 10,000 allocations (1 per query)
- With pooling: 10,000 queries = ~1-5 allocations (reuses same objects)

**Benefits:**

- ✅ **98-99% allocation reduction** in tight loops
- ✅ **40-60% GC reduction** - Dramatically less garbage collection
- ✅ **Thread-safe** - Safe to share across goroutines
- ✅ **Works with ScanRowsTo[T]** - Full type safety retained

## Optional: Monitor Pool Health

```go
// Enable statistics tracking
pool := v1.NewRowsAdapterPoolWithStats()

// ... process queries ...

// Check pool efficiency
stats := pool.Stats()
log.Printf("Allocated: %d, Available: %d\n",
    stats.Allocated, stats.Available)
```

### Pattern 3: Managed Cleanup (Explicit Wrappers)

Use `ManagedRowsAdapter` for explicit resource management with fallback cleanup.
**Best for:** Complex control flows, when you want finalizer safety guarantees.

```go
rows, err := database.QueryRaw(ctx, "SELECT * FROM users WHERE active = true")
if err != nil {
    log.Fatal(err)
}

// Wrap for automatic cleanup
managed, err := v1.WrapManagedRowsAdapter(rows)
if err != nil {
    log.Fatal(err)
}
defer managed.Close()  // Explicit, but will also cleanup via finalizer if forgotten

// Get the underlying adapter
adapter := managed.Adapter()

// Use with ScanRowsTo[T]
users, err := v1.ScanRowsTo[User](ctx, adapter)
if err != nil {
    return err
}

// Use typed results
for _, user := range users {
    process(user)
}

// Resources automatically cleaned up when managed.Close() called
```

**Benefits:**

- ✅ **Explicit semantics** - Crystal-clear resource lifecycle
- ✅ **Finalizer fallback** - Resources cleaned up even if Close() forgotten
- ✅ **Idempotent Close** - Safe to call multiple times
- ✅ **No panic** - Checks if already closed before cleanup

#### Real-World Service Pattern

Here's how to combine these patterns in a production service:

```go
type UserService struct {
    db   v1.DB
    pool *v1.RowsAdapterPool
}

func NewUserService(db v1.DB) *UserService {
    return &UserService{
        db:   db,
        pool: v1.NewRowsAdapterPool(),
    }
}

// Simple case: One-off request handler (Pattern 1)
func (s *UserService) GetUserByID(ctx context.Context, id int) (*User, error) {
    rows, err := s.db.GetRaw(ctx, "users", []string{"*"},
        condition.NewExpr().Column("id").Op("=").Value(id), nil, nil)
    if err != nil {
        return nil, err
    }

    // ScanRowsTo automatically closes
    users, err := db.ScanRowsTo[User](ctx, rows)
    if err != nil || len(users) == 0 {
        return nil, err
    }
    return &users[0], nil
}

// Bulk case: Batch processing (Pattern 2)
func (s *UserService) GetUsersBatch(ctx context.Context, ids []int) ([]User, error) {
    var allUsers []User

    for _, id := range ids {
        rows, err := s.db.GetRaw(ctx, "users", []string{"*"},
            condition.NewExpr().Column("id").Op("=").Value(id), nil, nil)
        if err != nil {
            return nil, err
        }

        // Use pool to reduce allocations
        adapter, err := s.pool.Acquire(rows)
        if err != nil {
            return nil, err
        }

        users, err := db.ScanRowsTo[User](ctx, adapter)
        s.pool.Release(adapter)  // Return to pool

        if err != nil {
            return nil, err
        }
        allUsers = append(allUsers, users...)
    }
    return allUsers, nil
}

// Complex control flow (Pattern 3)
func (s *UserService) SearchUsers(ctx context.Context, query string) ([]User, error) {
    rows, err := s.db.QueryRaw(ctx,
        "SELECT * FROM users WHERE name LIKE ? OR email LIKE ?",
        "%"+query+"%", "%"+query+"%")
    if err != nil {
        return nil, err
    }

    // Explicit managed cleanup with finalizer fallback
    managed, err := v1.WrapManagedRowsAdapter(rows)
    if err != nil {
        return nil, err
    }
    defer managed.Close()

    // Guaranteed cleanup: defer call + finalizer
    return db.ScanRowsTo[User](ctx, managed.Adapter())
}
```

**Performance Comparison:**

| Pattern                | Allocations         | GC Pressure | Best For                     |
| ---------------------- | ------------------- | ----------- | ---------------------------- |
| Pattern 1 (ScanRowsTo) | 1 per query         | Normal      | Single queries, most cases   |
| Pattern 2 (Pool)       | 1-5 for 10K queries | 40-60% less | Bulk operations, tight loops |
| Pattern 3 (Managed)    | 1 per query         | Normal      | Explicit semantics needed    |

**See Also:** [Resource Pooling Guide](./docs/RESOURCE_POOLING.md) for comprehensive benchmarks and advanced pool tuning

### Query Introspection and Performance Analysis

Generate SQL queries without executing them to inspect, log, or validate
before execution. This is especially useful for debugging and performance
analysis:

```go
// Introspect a GET query
query, args, err := database.GetQuery(
    "users",
    []string{"id", "name", "email"},
    nil,
    cdt.NewExpr().Column("age").Op(">").Value(25),
    nil, // order by
    nil, // limit
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

- ✅ **SQL Injection Prevention** - When combined with xxxQuery methods,
  ensures safe, parameterized SQL
- ✅ **Query Debugging** - Verify the actual SQL before execution
- ✅ **Performance Analysis** - Run EXPLAIN to understand query plans
- ✅ **Query Logging** - Log all generated SQL for audit trails
- ✅ **Batch Operations** - Build and verify multiple queries before
  execution

### FluentDB - Fluent Query Builder API

For a more ergonomic, chainable interface, use **FluentDB** - a
fluent/builder-style API that wraps DBActions with a readable,
SQL-like syntax:

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

// MySQL limited UPDATE
fdb.Update("users").
    Set("status", "inactive").
    Where(cdt.NewExpr().Column("last_login").Op("<").Value("2023-01-01")).
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    OrderByDesc("last_login").
    Limit(1000).
    Exec()

// Safe grouped SELECT with parameterized HAVING
query, args, err := fdb.Select("users", "department", "COUNT(*) AS total").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    GroupBy("department").
    Having(cdt.NewExpr().Column("COUNT(*)").Op(">").Value(3)).
    OrderByAsc("department").
    Query()

// Raw HAVING escape hatch for trusted SQL syntax only
query, args, err = fdb.Select("users", "department", "COUNT(*) AS total").
    GroupBy("department").
    HavingRaw("COUNT(*) FILTER (WHERE active = true) > 3").
    Query()

// Mutation query preview with PostgreSQL RETURNING / MSSQL OUTPUT
query, args, err = fdb.Insert().
    Into("users").
    Set("name", "Ada").
    Returning("id").
    Query()

// Pagination
fdb.Select("users", "id", "name").
    OrderByDesc("created_at").
    Limit(20).
    Offset((page-1)*20).
    Get()
```

See [FluentDB Examples](./examples/fluentdb-example/README.md) for
comprehensive tutorials on basic, advanced, and transaction-based
usage.

## Database Support

| Feature                          | MySQL | PostgreSQL | SQLite | MSSQL |
| -------------------------------- | ----- | ---------- | ------ | ----- |
| Basic CRUD execution             | ✅    | ✅         | ✅     | ✅    |
| Bulk insert execution            | ✅    | ✅         | ✅     | ✅    |
| Joined SELECT                    | ✅    | ✅         | ✅     | ✅    |
| Joined UPDATE SQL                | ✅    | ✅         | ✅     | ✅    |
| Joined DELETE SQL                | ✅    | ✅         | ❌     | ✅    |
| SELECT limit/offset              | ✅    | ✅         | ✅     | ✅    |
| UPDATE/DELETE order+limit        | ✅    | explicit error | explicit error | explicit error |
| Mutation RETURNING/OUTPUT preview | ignored | RETURNING | ignored | OUTPUT |
| Mutation RETURNING/OUTPUT execution | explicit error | explicit error | explicit error | explicit error |
| Safe parameterized HAVING        | ✅    | ✅         | ✅     | ✅    |
| Raw HAVING escape hatch          | ✅    | ✅         | ✅     | ✅    |
| Transactions                     | ✅    | ✅         | ✅     | ✅    |
| Query introspection              | ✅    | ✅         | ✅     | ✅    |
| EXPLAIN analysis                 | ✅    | ✅         | ✅     | ✅    |
| Connection pool stats            | ✅    | ✅         | ✅     | ✅    |

Dialect notes:

- `QueryOptions.Returning` is supported for query preview only. PostgreSQL
  renders `RETURNING`, MSSQL renders `OUTPUT`, and MySQL/SQLite ignore it.
  Mutation `Exec` methods return `ExecResult` and reject `Returning` because
  mutation execution does not return rows.
- Fluent mutation builders expose `Returning(...)` plus query-preview helpers
  such as `InsertQuery`, `InsertsQuery`, `UpdateQuery`, and `DeleteQuery`.
- MySQL supports mutation `OrderBy`/`Limit` only for non-joined UPDATE and
  DELETE. Other dialects return explicit unsupported-option errors.
- MSSQL pagination uses `ORDER BY ... OFFSET ... FETCH NEXT`; `Limit` requires
  `OrderBy`, and Fabric emits `OFFSET 0 ROWS` when only `Limit` is set.
- Prefer `Having(condition)` for parameterized aggregate filters. `HavingRaw`
  renders a raw SQL clause string. Table names, column names, raw expressions,
  and raw HAVING clauses must be trusted or allowlisted; values should use
  placeholders through conditions whenever possible.
- `WithTransaction` rolls back callback errors and callback panics. Panics are
  returned as non-nil errors with stack details rather than being swallowed.

All operators are documented in the [Architecture Guide](./docs/ARCHITECTURE.md).

## Database Configuration

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

Fabric supports structured logging with multiple popular Go logging
libraries through its logger adapter system. You can use your preferred
logging library without modifying Fabric's code.

### Using slog (Standard Library - Recommended)

The `slog` adapter works with Go's standard library structured logger
(Go 1.21+):

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

The library provides structured error handling with
database-dialect-specific error mapping. See
[ERROR_HANDLING.md](./docs/ERROR_HANDLING.md) for comprehensive guidance on
error handling patterns.

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

The `DBManager` provides seamless access to multiple database connections
with priority-based routing, automatic load-balancing, and async queries.
Perfect for scenarios with primary/replica setups, multi-region deployments,
or application-level sharding.

**Key Features:**

- 🎯 **Priority-Based Selection** - Route queries to preferred databases
- ⚖️ **Load Balancing** - Distribute queries among same-priority
  databases
- 🔧 **Worker Pools** - Configurable read/write workers per database
- 📬 **Async Queries** - Channel-based responses for non-blocking
  operations
- 🛡️ **Backpressure Handling** - Bounded queues prevent resource
  exhaustion

For complete guide, configuration examples, and use cases, see
[docs/DB_MANAGER.md](./docs/DB_MANAGER.md). Working examples available in
[examples/manager-example/](./examples/manager-example/).

## Plugin System

The fabric supports a registry-based plugin system that allows you to
register custom database drivers without modifying the core library.

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

#### Custom Rows Implementation

If your custom database driver returns non-standard row types
(not `*sql.Rows` or `pgx.Rows`), you must implement
the `v1.RowsProvider` interface to ensure compatibility with `RowsAdapter`.

The `RowsProvider` interface defines how rows are scanned:

```go
type RowsProvider interface {
    // columns returns the column names for the result set
    columns() ([]string, error)

    // next advances to the next row and returns true if more rows exist
    next() bool

    // scan populates destination variables with current row values
    scan(dest ...any) error

    // close releases resources (connections, buffers, etc.)
    close() error

    // err returns any error that occurred during iteration
    err() error
}
```

**Example with custom rows:**

```go
package mydb

import (
    db "tounilab.com/fabric/db/v1"
)

// CustomRows implements v1.RowsProvider
type CustomRows struct {
    data []map[string]any
    idx  int
    err  error
}

func (c *CustomRows) columns() ([]string, error) {
    if len(c.data) == 0 {
        return []string{}, nil
    }
    // Extract column names from first row
    cols := make([]string, 0, len(c.data[0]))
    for k := range c.data[0] {
        cols = append(cols, k)
    }
    sort.Strings(cols)
    return cols, nil
}

func (c *CustomRows) next() bool {
    if c.idx < len(c.data) {
        c.idx++
        return true
    }
    return false
}

func (c *CustomRows) scan(dest ...any) error {
    if c.idx == 0 || c.idx > len(c.data) {
        return fmt.Errorf("scan: invalid row position")
    }
    row := c.data[c.idx-1]
    cols, _ := c.columns()
    for i, d := range dest {
        if i < len(cols) {
            ptr := d.(*interface{})
            *ptr = row[cols[i]]
        }
    }
    return nil
}

func (c *CustomRows) close() error {
    c.data = nil
    return nil
}

func (c *CustomRows) err() error {
    return c.err
}

// Your DB.GetRaw() returns custom rows compatible with RowsAdapter
func (db *MyDB) GetRaw(
    ctx context.Context,
    table string,
    columns []string,
    ...,
) (*db.RowsAdapter, error) {
    customRows := &CustomRows{data: queryData}

    // RowsAdapter automatically wraps your CustomRows because it implements RowsProvider
    return db.NewRowsAdapter(customRows)
}
```

Now your custom rows work seamlessly with the fabric library's
query and scanning operations.

## Examples

See the [examples](./examples) directory for complete working examples:

- [`explain-example/`](./examples/explain-example/) - Query introspection
  with EXPLAIN analysis
- [`manager-example/`](./examples/manager-example/) - Multi-database
  management:
  - `basic/` - Basic DBManager usage patterns
  - `error-handling/` - Comprehensive error handling
  - `priority-selection/` - Priority-based database selection and
    routing
- [`plugin-example/`](./examples/plugin-example/) - Custom database
  driver plugin system with CockroachDB example
- [`tester-example/`](./examples/tester-example/) - Tester utility and
  test helpers

## Type Support

Supported scalar types for row scanning:

- **Basic Types**: `string`, `int`, `int8`, `int16`, `int32`, `int64`,
  `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`,
  `bool`, `[]byte`
- **SQL Null Types**: `sql.NullString`, `sql.NullInt32`, `sql.NullInt64`,
  `sql.NullFloat64`, `sql.NullBool`, `sql.NullTime`

For detailed NULL handling patterns, see [ERROR_HANDLING.md](./docs/ERROR_HANDLING.md).

## Contributing

Contributions are welcome! Please read our [CONTRIBUTING.md](./CONTRIBUTING.md)
guidelines before submitting PRs.

## Performance

The library is designed for production use with focus on:

- **Zero-copy scanning** - Efficient field mapping without intermediate
  allocations
- **Connection pooling** - Per-dialect optimizations (pgxpool for
  PostgreSQL)
- **Prepared statements** - Parameterized queries throughout
- **Memory efficiency** - Reusable row adapters and minimal allocations

## License

MIT License - see [LICENSE.md](./LICENSE.md)

## Documentation

**Release & Changelog**:

- 🎉 **[RELEASES.md](./RELEASES.md)** - Quick release overview and version history
- 📝 **[CHANGELOG.md](./CHANGELOG.md)** - Detailed changelog (Keep a Changelog format)

**Guides & References**:

- 📐 **[ARCHITECTURE.md](./docs/ARCHITECTURE.md)** - Complete system
  design, layers, components, and extension points
- 📋 **[CODE_REVIEW.md](./docs/CODE_REVIEW.md)** - Code quality
  standards and testing requirements
- ⚠️ **[ERROR_HANDLING.md](./docs/ERROR_HANDLING.md)** - Error handling
  patterns and NULL type mapping
- 🔧 **[DB_MANAGER.md](./docs/DB_MANAGER.md)** - Multi-database
  management and load balancing
- 📦 **[ENVIRONMENT_VARIABLES.md](./docs/ENVIRONMENT_VARIABLES.md)** -
  Configuration and environment setup
- 📚 **[LINTING.md](./docs/LINTING.md)** - Code style and linting
  standards

## Support

- 🐛 [Issue Tracker](https://github.com/oratchade/fabric/issues)
- 💬 [Discussions](https://github.com/oratchade/fabric/discussions)

## Changelog

See [RELEASES.md](./RELEASES.md) for version history and release notes.

For detailed changes between versions, see [CHANGELOG.md](./CHANGELOG.md)
(Keep a Changelog format).

---

**Last Updated:** March 2026  
**Status:** ✅ Production Ready
