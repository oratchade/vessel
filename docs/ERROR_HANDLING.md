# Error Handling

This guide covers the current error handling surface in Vessel: sentinel
errors, dialect mapping, wrapping, and production handling patterns.

## Sentinel Errors

Sentinel errors live in `db/v1/dberror`:

```go
import dberror "tounilab.com/vessel/db/v1/dberror"
```

| Error | Meaning |
| --- | --- |
| `ErrNotFound` | A lookup found no record. |
| `ErrDuplicateKey` | A unique or primary-key constraint failed. |
| `ErrForeignKeyViolation` | A foreign-key constraint failed. |
| `ErrConnectionFailed` | The database connection failed. |
| `ErrConstraintViolation` | A general constraint failed. |
| `ErrSyntaxError` | The SQL statement was rejected as invalid. |
| `ErrQueryTimeout` | The query timed out or the database reported lock/busy timeout. |

Use `errors.Is` to check them:

```go
result, err := database.Insert(ctx, "users", map[string]any{
    "email": email,
}, nil)
if err != nil {
    if errors.Is(err, dberror.ErrDuplicateKey) {
        return fmt.Errorf("email already exists: %w", err)
    }
    return fmt.Errorf("create user: %w", err)
}

log.Printf("created rows=%d", result.RowsAffected)
```

## Dialect Mapping

Built-in drivers map common native database errors to the sentinel errors above.
The original driver error remains wrapped in the error chain.

| Dialect | Examples mapped |
| --- | --- |
| MySQL | duplicate key `1062`, foreign key `1452`, syntax `1064` |
| PostgreSQL | unique `23505`, foreign key `23503`, syntax `42601`, connection `08xxx`, cancel `57014` |
| SQLite | unique/primary-key constraint, foreign key constraint, cant-open, busy/locked |
| MSSQL | duplicate key `2601`/`2627`, foreign key `547`, login/connectivity, syntax |

Returned errors are intentionally contextual. A PostgreSQL duplicate might look
like:

```text
postgres.Insert: failed to execute insert query: [postgres] duplicate key violation: ...
```

Do not match exact strings in application code. Use `errors.Is`.

## Query Errors

Read methods include context in their returned errors. Keep application context
at the boundary where the query is meaningful:

```go
rows, err := database.Get(
    ctx,
    "users",
    []string{"id", "email"},
    nil,
    cdt.NewExpr().Column("active").Op("=").Value(true),
    nil,
)
if err != nil {
    return fmt.Errorf("load active users: %w", err)
}
```

For no-row flows, check the behavior of the specific method you use. `ScanOne`
returns an error when the result set is empty or has more than one row; `Get`
returns a slice.

## Transactions

`WithTransaction` rolls back when the callback returns an error or panics, and
commits when it returns nil.

```go
err := database.WithTransaction(ctx, func(tx db.Tx) error {
    orderID := uuid.NewString()

    _, err := tx.Insert(ctx, "orders", map[string]any{
        "id":      orderID,
        "user_id": userID,
        "status":  "pending",
    }, nil)
    if err != nil {
        return fmt.Errorf("insert order: %w", err)
    }

    for _, item := range items {
        _, err := tx.Insert(ctx, "order_items", map[string]any{
            "order_id":   orderID,
            "product_id": item.ProductID,
            "quantity":   item.Quantity,
        }, nil)
        if err != nil {
            return fmt.Errorf("insert order item: %w", err)
        }
    }

    return nil
})
if err != nil {
    return fmt.Errorf("create order: %w", err)
}
```

Vessel's `ExecResult` exposes `RowsAffected`. If application code needs a
portable created identifier, generate it before insert and store it explicitly.
For create-and-fetch, use `InsertAndFetch` or query by the generated key inside
a transaction.

## Context Timeout

Always use request-scoped contexts or explicit deadlines:

```go
ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
defer cancel()

rows, err := database.Query(ctx, "SELECT id FROM events WHERE user_id = ?", userID)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) ||
        errors.Is(err, dberror.ErrQueryTimeout) {
        return fmt.Errorf("events query timed out: %w", err)
    }
    return fmt.Errorf("events query failed: %w", err)
}
```

Context cancellation and database timeout errors are not identical. Check both
when a production path treats timeouts specially.

## Retry Policy

Retry only errors that are safe for the operation. Retrying a plain insert after
an unknown failure can duplicate side effects unless the operation is
idempotent.

Common retry candidates:

- `ErrConnectionFailed`
- `ErrQueryTimeout`
- `context.DeadlineExceeded` when the operation is idempotent

Common non-retry candidates:

- `ErrDuplicateKey`
- `ErrForeignKeyViolation`
- `ErrSyntaxError`
- `ErrConstraintViolation`

```go
func retryable(err error) bool {
    return errors.Is(err, dberror.ErrConnectionFailed) ||
        errors.Is(err, dberror.ErrQueryTimeout) ||
        errors.Is(err, context.DeadlineExceeded)
}
```

## Logging

Log operation context, not credentials or full row data:

```go
if err != nil {
    logger.Error("database operation failed",
        "operation", "insert",
        "table", "users",
        "error", err,
    )
}
```

Vessel's built-in logger path already classifies query failures into structured
fields. See [LOGGING.md](./LOGGING.md).

## Testing

Exercise error paths explicitly:

```go
err := createUser(ctx, database, existingEmail)
if !errors.Is(err, dberror.ErrDuplicateKey) {
    t.Fatalf("expected duplicate key, got %v", err)
}
```

For retry behavior, test the decision function separately from the database.
Integration tests should still cover real duplicate-key and foreign-key
failures for every dialect the application supports.

## Production Practices

- Use `errors.Is`, never string matching.
- Wrap errors once at each application boundary with useful context.
- Use app-generated IDs for portable create flows.
- Keep retries idempotent and bounded.
- Treat unsupported-feature errors as design feedback.
- Pass contexts with deadlines into every call.

## See Also

- [LOGGING.md](./LOGGING.md)
- [CONFIGURATION.md](./CONFIGURATION.md)
- [PORTABILITY_MATRIX.md](./PORTABILITY_MATRIX.md)
