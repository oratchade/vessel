# Error Handling Guide

This guide explains how to properly handle errors in vessel applications.

## Overview

vessel provides structured error handling with:

1. **Sentinel Errors** - Specific error types for common database errors
2. **Error Mapping** - Dialect-specific error translation
   (MySQL, PostgreSQL, SQLite, MSSQL)
3. **Error Wrapping** - Full error context with stack traces
   via `fmt.Errorf`
4. **Context Propagation** - Proper timeout and cancellation handling

## Sentinel Errors

The library defines sentinel errors in `db/v1/dberror` for common database conditions:

```go
import "tounilab.com/vessel/db/v1/dberror"
```

### Common Sentinel Errors

| Error                     | Cause                     | Handling             |
| ------------------------- | ------------------------- | -------------------- |
| `ErrDuplicateKey`         | Unique constraint         | Retry with different |
| `ErrForeignKeyConstraint` | Foreign key violation     | Ensure ref exists    |
| `ErrConnectionFailed`     | Cannot connect to DB      | Retry with backoff   |
| `ErrNoRows`               | Query returned no results | Check conditions     |
| `ErrNotSupported`         | Not supported by dialect  | Use fallback         |

### Checking Sentinel Errors

```go
import (
    "errors"
    "log"
    dberror "tounilab.com/vessel/db/v1/dberror"
)

result, err := database.Insert(ctx, "users", map[string]any{
    "email": "duplicate@example.com",
})

if err != nil {
    // Check for specific error type
    if errors.Is(err, dberror.ErrDuplicateKey) {
        log.Println("This email is already registered")
        // Handle duplicate key: maybe update instead?
    } else if errors.Is(err, dberror.ErrConnectionFailed) {
        log.Println("Database is unavailable, retrying...")
        // Implement retry logic
    } else {
        log.Fatal("Unexpected error:", err)
    }
}
```

## Dialect-Specific Error Mapping

Each database dialect maps its native errors to sentinel errors:

### MySQL Error Examples

```go
// Duplicate key (MySQL error 1062)
// Maps to: dberror.ErrDuplicateKey

// Foreign key constraint (MySQL error 1452)
// Maps to: dberror.ErrForeignKeyConstraint

// Connection lost (MySQL error 2006)
// Maps to: dberror.ErrConnectionFailed
```

### PostgreSQL Error Examples

```go
// Unique violation (SQLSTATE 23505)
// Maps to: dberror.ErrDuplicateKey

// Foreign key violation (SQLSTATE 23503)
// Maps to: dberror.ErrForeignKeyConstraint

// Connection refused (SQLSTATE 08001)
// Maps to: dberror.ErrConnectionFailed
```

### SQLite Error Examples

```go
// UNIQUE constraint failed
// Maps to: dberror.ErrDuplicateKey

// FOREIGN KEY constraint failed
// Maps to: dberror.ErrForeignKeyConstraint

// Unable to open database file
// Maps to: dberror.ErrConnectionFailed
```

### MSSQL Error Examples

```go
// Violation of PRIMARY KEY/UNIQUE constraint (Error 2627)
// Maps to: dberror.ErrDuplicateKey

// The statement conflicted with a FOREIGN KEY constraint (Error 547)
// Maps to: dberror.ErrForeignKeyConstraint

// Login failed for user (Error 18456)
// Maps to: dberror.ErrConnectionFailed
```

## Error Wrapping and Context

All errors from vessel follow the standardized wrapping pattern:
`function: operation: %w`

This ensures consistent error messages across the library with full error context:

```go
import "errors"

result, err := database.Get(ctx, "users", cols, "", conds, nil)
if err != nil {
    // Error follows pattern: function: operation: %w
    // Example: "Get: scan rows: driver: bad connection"
    // Unwrap to see the original error
    cause := errors.Unwrap(err)
    // cause = "driver: bad connection"
}
```

### Error Message Format

All errors use the format: **`function: operation: %w`**

- **function**: The exported method that failed (Get, Insert, Update, Delete, etc.)
- **operation**: The specific step that failed
  (parse SQL, scan rows, bind params, etc.)
- **%w**: The underlying error (always wrapped with `fmt.Errorf()` for proper chain)

**Examples**:

- `Get: scan rows: sql: Rows.Scan called after Close`
- `Insert: execute: driver: duplicate key value violates unique constraint`
- `Update: bind params: invalid value type int64`
- `NewDB: parse config: invalid host address`

## Common Error Patterns

### Pattern 1: Insert with Duplicate Key Handling

```go
result, err := database.Insert(ctx, "users", map[string]any{
    "id":    uuid.New().String(),
    "email": email,
    "name":  name,
}, nil)

if err != nil {
    if errors.Is(err, dberror.ErrDuplicateKey) {
        // User already exists, return meaningful error to client
        // Error message format:
        // "Insert: execute: duplicate key violates unique constraint"
        return fmt.Errorf("email already registered: %w", err)
    }
    // Error message format: "Insert: bind params: invalid value type"
    return fmt.Errorf("failed to create user: %w", err)
}

log.Printf("Created user with ID: %v\n", result.LastInsertID)
```

### Pattern 2: Query with Retry Logic

```go
const maxRetries = 3
const retryDelay = 100 * time.Millisecond

var rows []map[string]any
var err error

for attempt := 0; attempt < maxRetries; attempt++ {
    rows, err = database.Get(ctx, "users", cols, "", conds, nil)
    if err == nil {
        break
    }

    if errors.Is(err, dberror.ErrConnectionFailed) {
        if attempt < maxRetries-1 {
            time.Sleep(retryDelay * time.Duration(1+attempt))
            continue
        }
    }

    return fmt.Errorf("query failed after %d attempts: %w", maxRetries, err)
}

return rows, nil
```

### Pattern 3: Transaction with Automatic Rollback

```go
err := database.WithTransaction(ctx, func(tx db.Tx) error {
    // Insert order
    orderResult, err := tx.Insert(ctx, "orders", map[string]any{
        "user_id":    userID,
        "total":      100.00,
        "status":     "pending",
    })
    if err != nil {
        // Automatically rolled back
        return fmt.Errorf("failed to create order: %w", err)
    }

    orderID := orderResult.LastInsertID

    // Insert line items
    for _, item := range items {
        _, err := tx.Insert(ctx, "order_items", map[string]any{
            "order_id":   orderID,
            "product_id": item.ProductID,
            "quantity":   item.Quantity,
        })
        if err != nil {
            // Entire transaction rolls back
            return fmt.Errorf("failed to add item: %w", err)
        }
    }

    // All inserts commit together
    return nil
})

if err != nil {
    log.Printf("Transaction failed: %v\n", err)
}
```

### Pattern 4: Foreign Key Constraint Handling

```go
result, err := database.Insert(ctx, "comments", map[string]any{
    "post_id": postID,
    "author":  author,
    "text":    text,
})

if err != nil {
    if errors.Is(err, dberror.ErrForeignKeyConstraint) {
        return fmt.Errorf("post not found: %w", err)
    }
    return fmt.Errorf("failed to create comment: %w", err)
}

log.Printf("Created comment with ID: %v\n", result.LastInsertID)
```

### Pattern 5: Context Timeout Handling

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

rows, err := database.Get(ctx, "events", cols, "", conds, nil)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return fmt.Errorf("query timeout: took longer than 5 seconds")
    }
    if errors.Is(err, context.Canceled) {
        return fmt.Errorf("query was cancelled by caller")
    }
    return fmt.Errorf("query failed: %w", err)
}
```

## Error Logging Best Practices

### Good: Include Context

```go
if err != nil {
    log.Printf("failed to insert user email=%s: %v\n", email, err)
}
```

### Better: Use Structured Logging

```go
type Event struct {
    Timestamp time.Time
    Level     string
    Message   string
    Error     string
    Context   map[string]any
}

if err != nil {
    log.Printf(
        "event=insert status=failed table=users email=%s error=%v\n",
        email, err)
}
```

### Best: Use Logging Middleware

```go
func logQuery(operation, table string, err error) {
    if err != nil {
        slog.Error("database operation failed",
            slog.String("operation", operation),
            slog.String("table", table),
            slog.String("error", err.Error()),
        )
    }
}

// Usage
_, err := database.Insert(ctx, "users", data)
logQuery("insert", "users", err)
```

## Testing Error Handling

### Mock Error Scenarios

```go
import dberror "tounilab.com/vessel/db/v1/dberror"

// In tests, you can inject specific errors via mocks
mockDB := &MockDB{
    InsertFn: func(ctx context.Context, ...) (*db.ExecResult, error) {
        return nil, dberror.ErrDuplicateKey
    },
}

// Test duplicate key handling
err := createUser(mockDB, "email@example.com")
if !errors.Is(err, dberror.ErrDuplicateKey) {
    t.Errorf("expected duplicate key error, got %v", err)
}
```

### Test Timeout Handling

```go
func TestQueryTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()

    // This should timeout
    _, err := database.Get(ctx, "slow_table", cols, "", nil, nil)
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("expected timeout error, got %v", err)
    }
}
```

## Error Recovery Strategies

### Strategy 1: Exponential Backoff

```go
func execWithBackoff(fn func() error, maxRetries int) error {
    backoff := 100 * time.Millisecond

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        if !isRetryable(err) {
            return err
        }

        if attempt < maxRetries-1 {
            time.Sleep(backoff)
            backoff *= 2
            if backoff > 10*time.Second {
                backoff = 10 * time.Second
            }
        }
    }

    return fmt.Errorf("operation failed after %d retries", maxRetries)
}

func isRetryable(err error) bool {
    return errors.Is(err, dberror.ErrConnectionFailed) ||
           errors.Is(err, context.DeadlineExceeded)
}
```

### Strategy 2: Circuit Breaker

```go
type CircuitBreaker struct {
    failures    int
    lastFailure time.Time
    state       string // "closed", "open", "half-open"
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == "open" {
        if time.Since(cb.lastFailure) > 30*time.Second {
            cb.state = "half-open"
        } else {
            return fmt.Errorf("circuit breaker is open")
        }
    }

    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures > 5 {
            cb.state = "open"
        }
    } else {
        cb.failures = 0
        cb.state = "closed"
    }

    return err
}
```

## Migration Guide: Error Handling Updates

When upgrading vessel, check the changelog for error handling changes:

1. **New Sentinel Errors** - May add new sentinel error types
2. **Error Message Changes** - Error messages may change between versions
3. **Error Wrapping** - Wrapping strategy may be refined

Always test error handling paths when upgrading versions.

## Summary

Key takeaways for error handling:

- ✅ Always check for `nil` errors before using results
- ✅ Use `errors.Is()` to check sentinel errors
- ✅ Wrap errors with context using `fmt.Errorf("%w", err)`
- ✅ Handle retryable errors (connection, timeout) differently
- ✅ Test error paths in unit and integration tests
- ✅ Log errors with sufficient context for debugging
- ✅ Never ignore errors; always handle or return them

See [CODE_REVIEW.md](./CODE_REVIEW.md) for overall code quality assessment.

---

**Last Updated:** March 2026
