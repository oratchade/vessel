# Vessel Specification

This is the current implementation specification for Vessel's v0.x public API.
It describes what the library is expected to do today; it is not a product
roadmap.

## Scope

Vessel provides:

- A common `db.DB` interface for MySQL, PostgreSQL, SQLite, and MSSQL.
- Dialect-aware builders for common `SELECT`, `INSERT`, `UPDATE`, `DELETE`, and
  upsert flows.
- Query-preview methods for generated SQL and arguments.
- Typed scanning helpers over `RowsAdapter`.
- Transactions with callback handling and savepoint helpers.
- Structured logging adapters and OpenTelemetry tracing hooks.
- An optional `manager/v1` package for multi-connection services.
- A plugin registry for custom drivers that implement the `db.DB` contract.

Vessel does not provide schema migrations, model relationships, hooks,
compile-time SQL validation, or ORM lifecycle behavior.

## Supported Dialects

| Dialect | Built-in config | Notes |
| --- | --- | --- |
| MySQL | `db.MysqlConfig` | Uses `database/sql`; upsert renders `ON DUPLICATE KEY UPDATE`. |
| PostgreSQL | `db.PostgresConfig` | Uses `pgxpool`; strongest feature support. |
| SQLite | `db.SQLiteConfig` | Uses `modernc.org/sqlite`; single-writer behavior applies. |
| MSSQL | `db.MSSQLConfig` | Uses SQL Server syntax; upsert is unsupported by design. |

Dialect differences must either render intentionally or return explicit errors.
Silent SQL generation for unsupported behavior is not acceptable.

## Connection Creation

```go
database, err := db.NewDB(db.PostgresConfig{
    User:     "app_user",
    Password: password,
    Host:     "postgres.internal",
    Port:     5432,
    Database: "app",
}, logger)
```

`NewDB` accepts any config implementing:

```go
type DBConfig interface {
    Driver() string
    DSN() string
}
```

It checks registered plugin factories first, then falls back to built-in
drivers.

## Core DB Behavior

The public `DB` interface includes:

- Reads: `Get`, `GetRaw`, `GetByID`, `GetByIDRaw`, `Query`, `QueryRaw`.
- Writes: `Insert`, `Inserts`, `Update`, `Delete`, `Exec`.
- Upsert: `Upsert`, `Upserts`, `UpsertQuery`, `UpsertsQuery`.
- Preview/introspection: `GetQuery`, `InsertQuery`, `UpdateQuery`,
  `DeleteQuery`, `Explain`, and related helpers.
- Transactions: `Begin`, `WithTransaction`.
- Operations: `Ping`, `PoolStats`, `Close`.

All execution methods take `context.Context`. Callers are responsible for
timeouts and cancellation policy.

## Fluent Builder

`db.NewFluentDB(database)` wraps a `DB` or `Tx`.

```go
fdb := db.NewFluentDB(database)

rows, err := fdb.Select("users", "id", "email").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    OrderBy("created_at", db.DescDirection).
    Limit(50).
    Get(ctx)
```

Context is passed to terminal methods, not to `NewFluentDB`. Builders mutate
their own state and are not safe to reuse concurrently.

## Parameterization And Raw SQL

Values supplied through condition helpers and mutation `Set`/`Values` paths are
passed as driver arguments. Identifiers are rendered by the dialect where the
builder owns the SQL shape.

Raw SQL APIs are explicit caller-owned boundaries:

- `DB.QueryRaw`
- `DB.Exec`
- `ColumnRaw`
- `ColumnRawAs`
- `HavingRaw`

These APIs must be treated as trusted SQL only. Vessel does not parse or
sanitize raw fragments.

## Returning

`Returning` is supported for query preview where the dialect can render it.
Execution methods return `ExecResult`, not rows, so row-returning mutations are
not part of the portable execution contract. Use dialect-specific raw SQL when a
production workflow requires returned rows from a mutation.

## Typed Scanning

Raw read methods return `*RowsAdapter`. Typed helpers consume and close it:

```go
rows, err := database.GetRaw(ctx, "users", []string{"id", "email"}, nil, cond, nil)
if err != nil {
    return err
}

users, err := db.ScanRowsTo[User](ctx, rows)
```

`ScanAll[T]` and `ScanOne[T]` wrap the same scanner with different cardinality
expectations.

## Transactions

`WithTransaction` is the preferred helper for application code:

```go
err := database.WithTransaction(ctx, func(tx db.Tx) error {
    _, err := tx.Insert(ctx, "orders", order, nil)
    return err
})
```

It commits on nil error and rolls back on error or panic. Manual `Begin`,
`Commit`, and `Rollback` remain available for cases that need explicit control.

## Logging

The logger interface is:

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    With(fields ...any) Logger
}
```

Adapters are provided for slog, logrus, zap, and apex/log. Passing `nil`
disables logging.

## Observability

OpenTelemetry tracing uses the global tracer provider unless
`OTEL_ENABLED=false` is set. If the application does not configure an SDK, spans
are no-op. Vessel does not attach row data or SQL parameters to spans.

## Plugins

Plugins register a `plugin.DriverFactory`:

```go
type DriverFactory interface {
    Name() string
    Create(ctx context.Context, cfg any) (any, error)
}
```

The returned value must satisfy `db.DB`. Conformance helpers in
`db/v1/plugin` verify this contract.

## Production Requirements

- Every request path should pass a bounded context into Vessel calls.
- Credentials must come from environment-specific configuration or secret
  storage, never committed files.
- Pool settings must be tuned per service, not copied blindly.
- Raw SQL must be reviewed and allowlisted.
- Unsupported-feature errors must be handled explicitly.
- Documentation examples should compile against the current public API shape.
