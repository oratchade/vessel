# FluentDB Examples

Complete working examples demonstrating FluentDB builder API: a fluent and
chainable query builder for Vessel's multi-database abstraction layer.

## Overview

The **FluentDB** API provides an ergonomic, chainable interface for building
and executing SQL queries. It wraps the existing `DBActions` interface while
providing a more intuitive API for common database operations.

## Prerequisites

- Go 1.20+
- PostgreSQL 12+ (or MySQL/SQLite/MSSQL supported by Vessel)
- A running database instance with test tables

### Setup Test Database

Start your database and create test tables. For PostgreSQL:

```bash
psql -U postgres -c "CREATE DATABASE myapp;"

# Create tables
psql -d myapp -U postgres -c "
  CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100) UNIQUE,
    role VARCHAR(50) DEFAULT 'user',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_login TIMESTAMP
  );

  CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE,
    description TEXT
  );

  CREATE TABLE profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    bio TEXT,
    avatar_url VARCHAR(255)
  );

  CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    balance DECIMAL(10, 2)
  );

  CREATE TABLE departments (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100)
  );

  CREATE TABLE teams (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100)
  );

  CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    plan VARCHAR(50)
  );

  CREATE TABLE transaction_logs (
    id SERIAL PRIMARY KEY,
    from_user_id INTEGER,
    to_user_id INTEGER,
    amount DECIMAL(10, 2),
    type VARCHAR(50),
    timestamp TIMESTAMP DEFAULT NOW()
  );

  CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    action VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
  );
"
```

## Examples

### 1. `basic/main.go` - Basic CRUD Operations

**What it demonstrates:**

- SELECT queries with WHERE, ORDER BY, LIMIT
- INSERT single row and bulk operations
- UPDATE with filters
- DELETE with conditions
- COUNT aggregation
- One() for single row retrieval

**Run:**

```bash
go run ./basic/main.go
```

**Key Features:**

```go
// SELECT with fluent chaining
rows, err := fdb.Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    OrderBy("name", "ASC").
    Limit(10).
    Get()

// INSERT with Set/SetMap
result, err := fdb.Insert().Into("users").
    Set("name", "John").
    Set("email", "john@example.com").
    Exec()

// UPDATE with conditions
result, err := fdb.Update("users").
    Set("status", "active").
    Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
    Exec()

// DELETE with filters
result, err := fdb.Delete().From("users").
    Where(cdt.NewExpr().Column("active").Op("=").Value(false)).
    Limit(10).
    Exec()

// COUNT
count, err := fdb.Select("users").Count()

// Get single row
row, err := fdb.Select("users", "id", "name").
    Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
    One()
```

### 2. `advanced/main.go` - Advanced Queries with JOINs

**What it demonstrates:**

- SELECT with INNER JOIN
- SELECT with LEFT JOIN
- UPDATE with JOINs
- DELETE with JOINs
- Pagination with LIMIT/OFFSET
- Multiple JOINs

**Run:**

```bash
go run ./advanced/main.go
```

**Key Features:**

```go
// SELECT with INNER JOIN
rows, err := fdb.Select("users", "users.id", "users.name", "roles.name").
    Join(cdt.Join{
        Type:  "INNER",
        Table: "roles",
        Conditions: []cdt.JoinCdt{{
            Left:  "users.role_id",
            Right: "roles.id",
        }},
    }).
    Get()

// UPDATE with JOIN
result, err := fdb.Update("users").
    Set("is_premium", true).
    Join(cdt.Join{
        Type:  "INNER",
        Table: "roles",
        Conditions: []cdt.JoinCdt{{
            Left:  "users.role_id",
            Right: "roles.id",
        }},
    }).
    Where(cdt.NewExpr().Column("roles.name").Op("=").Value("vip")).
    Exec()

// Pagination
rows, err := fdb.Select("users", "id", "name").
    OrderBy("id", "ASC").
    Limit(10).
    Offset(20).
    Get()
```

### 3. `transactions/main.go` - Transaction Handling

**What it demonstrates:**

- Starting and committing transactions
- Using FluentDB within transactions
- Error handling and rollback
- Conditional operations in transactions
- Multi-step operations with atomic guarantees

**Run:**

```bash
go run ./transactions/main.go
```

**Key Features:**

```go
// Simple transaction
tx, err := conn.Begin(ctx)
if err != nil {
    log.Fatal(err)
}

fdb := db.NewFluentDB(tx, ctx)

// Use FluentDB normally, all operations are in transaction
result, err := fdb.Insert().Into("users").
    Set("name", "John").
    Exec()

if err != nil {
    tx.Rollback(ctx)
    log.Fatal(err)
}

tx.Commit(ctx)

// With error handling
tx, err := conn.Begin(ctx)
_, err = fdb.Insert().Into("users").Set("name", "John").Exec()
if err != nil {
    tx.Rollback(ctx)
    return
}

_, err = fdb.Insert().Into("profiles").Set("user_id", 1).Exec()
if err != nil {
    tx.Rollback(ctx)
    return
}

tx.Commit(ctx)
```

## API Comparison

### FluentDB vs Raw DBActions

**Raw DBActions:**

```go
rows, err := db.Get(ctx, "", "users", []string{"id", "name"}, nil,
    cdt.NewExpr().Column("active").Op("=").Value(true),
    &options.QueryOptions{Limit: 10})
```

**FluentDB:**

```go
rows, err := fdb.Select("users", "id", "name").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    Limit(10).
    Get()
```

## Builder Pattern Benefits

1. **Chainable API** - Methods return builders for fluent chaining
2. **Readable** - Code reads left-to-right like SQL
3. **Type-Safe** - Compiler catches order errors
4. **Reusable** - All builders delegate to existing DBActions
5. **Ergonomic** - Common patterns are simple and obvious

## Advanced Topics

### Error Wrapping

All errors from database operations are wrapped with context:

```go
rows, err := fdb.Select("users", "id").Get()
// Error contains: "selectBuilder: failed to get rows: <original error>"
```

### Context Propagation

Contexts are propagated through the entire builder chain:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

fdb := db.NewFluentDB(dbConn, ctx)
// All queries use the same context with 5-second timeout
rows, err := fdb.Select("users", "id").Get()
```

### Transaction Switching

Switch from normal operations to transaction context:

```go
// Normal operations
fdb := db.NewFluentDB(dbConn, ctx)
rows, err := fdb.Select("users", "id").Get()

// Start transaction
tx, _ := dbConn.Begin(ctx)
txFdb := db.NewFluentDB(tx, ctx)

// Use transaction
_, _ = txFdb.Insert().Into("users").Set("name", "John").Exec()

// Or switch existing builder
txFdb := fdb.WithTx(tx)
rows, _ := txFdb.Select("users", "id").Get()
```

## Common Patterns

### Bulk Operations

```go
// Bulk INSERT
users := []map[string]any{
    {"name": "Alice", "email": "alice@example.com"},
    {"name": "Bob", "email": "bob@example.com"},
}

result, err := fdb.Insert().
    Into("users").
    ValuesBulk(users).
    Exec()
```

### Conditional Updates

```go
// Update multiple rows based on conditions
result, err := fdb.Update("users").
    Set("status", "inactive").
    Where(cdt.NewExpr().Column("last_login").Op("<").Value("2023-01-01")).
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    Limit(1000).
    Exec()
```

### Pagination

```go
pageSize := 20
page := 1
offset := (page - 1) * pageSize

rows, err := fdb.Select("users", "id", "name").
    OrderBy("created_at", "DESC").
    Limit(pageSize).
    Offset(offset).
    Get()
```

### Cleanup Operations

```go
// Delete old records
result, err := fdb.Delete().
    From("audit_logs").
    Where(cdt.NewExpr().Column("created_at").Op("<").Value("2023-01-01")).
    OrderBy("created_at", "ASC").
    Limit(10000).
    Exec()
```

## Troubleshooting

### "table not specified" error

Make sure you specify the table in Select:

```go
// ✗ Wrong
fdb.Select("id", "name")

// ✓ Correct
fdb.Select("users", "id", "name")
```

### Context deadline exceeded

Ensure your database operations complete within the context timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

### Transaction commit errors

Always check error returns from Commit and Rollback:

```go
err := tx.Commit(ctx)
if err != nil {
    // Handle commit error
    log.Printf("Commit failed: %v", err)
}
```

## Performance Tips

1. **Use LIMIT for large tables** - Prevent loading too much data
2. **Add indexes** - For frequently queried columns
3. **Batch operations** - Use bulk INSERT/UPDATE when possible
4. **Use JOINs wisely** - Reduce round trips with joins
5. **Profile queries** - Use database monitoring tools

## See Also

- [FluentDB Implementation](../../../db/v1/fluentDB.go) - Source code
- [Condition Package](../../../pkg/query/condition) - Building conditions
- [vessel README](../../README.md) - Library overview
- [DBManager Examples](../manager-example) - Multi-database routing
