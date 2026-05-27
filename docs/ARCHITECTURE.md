# Vessel Architecture

Vessel is a personal SQL toolkit for Go services that need a small, explicit
data layer across MySQL, PostgreSQL, SQLite, and MSSQL. It sits between raw
`database/sql` and an ORM: query builders, typed scanning, transactions,
logging, tracing hooks, and optional multi-connection management.

This document describes the current production shape of the project. It avoids
claims about scale or guarantees that are not enforced by code.

## Package Map

```text
db/v1/                  Public DB, Tx, config, logger, fluent builder, rows
db/v1/plugin/           Driver factory registry and conformance helpers
internal/pkg/builder/   SQL builders used by the public operations
internal/pkg/sqldialect/Dialect rendering rules
internal/pkg/otel/      OpenTelemetry tracer wrapper
pkg/query/condition/    Condition DSL
pkg/query/options/      QueryOptions and UpsertOptions
manager/v1/             Optional multi-connection manager
tests/                  Integration tests
```

Only `db/v1`, `db/v1/plugin`, `pkg/query/*`, and `manager/v1` are public API.
Anything under `internal/` can change without a compatibility promise.

## Public API

`db.NewDB(config, logger)` creates a `db.DB`. The config must implement:

```go
type DBConfig interface {
    Driver() string
    DSN() string
}
```

Built-in configs are `MysqlConfig`, `PostgresConfig`, `SQLiteConfig`, and
`MSSQLConfig`. The returned `DB` combines read, write, upsert, introspection,
transaction, health-check, pool-stat, and close operations.

`db.NewFluentDB(database)` creates the fluent builder wrapper. Context is not
stored on the wrapper; it is passed to terminal methods such as `Get(ctx)`,
`Exec(ctx)`, `Upsert(ctx)`, and `Count(ctx)`.

Builders mutate their own state and are not meant to be shared across
goroutines. Create a new builder per query.

## Query Flow

1. Application code builds a query through `DB` methods or `FluentDB`.
2. Conditions and options are converted into internal builder input.
3. The dialect renders identifiers, placeholders, joins, pagination, upsert,
   and unsupported-feature checks.
4. Values are passed as driver parameters.
5. Results are returned as `[]map[string]any`, `*RowsAdapter`, or `ExecResult`.

Raw SQL remains caller-owned. `QueryRaw`, `Exec`, `ColumnRaw`, `ColumnRawAs`,
and `HavingRaw` do not parse or sanitize SQL fragments; use them only with
trusted or allowlisted SQL.

## Dialects

The built-in dialects cover MySQL, PostgreSQL, SQLite, and MSSQL. They differ
where the databases differ:

- PostgreSQL and SQLite use `ON CONFLICT` for upsert.
- MySQL uses `ON DUPLICATE KEY UPDATE`.
- MSSQL upsert returns an explicit unsupported-feature error.
- SQLite joined `DELETE` returns an explicit unsupported-feature error.
- MSSQL pagination uses `OFFSET/FETCH` and requires a stable `ORDER BY`.

`Returning` support is currently query-preview only. Mutation execution methods
return `ExecResult`, not returned rows, and reject/ignore returning paths as
documented in the portability matrix.

## Rows And Scanning

`Get` and `Query` materialize rows as `[]map[string]any`.

`GetRaw`, `GetByIDRaw`, `QueryRaw`, and `Explain` return `*RowsAdapter` for
streaming access. Use one of these lifecycle patterns:

- `ScanRowsTo[T]`, `ScanAll[T]`, or `ScanOne[T]` for typed scans with automatic
  close.
- `defer rows.Close()` for manual iteration.
- `RowsAdapterPool` only where allocation pressure has been measured.

## Transactions

`Begin(ctx, opts...)` returns a `Tx`. `WithTransaction(ctx, fn, opts...)`
commits when `fn` returns nil and rolls back when `fn` returns an error or
panics. A `Tx` supports the same read/write/query methods plus savepoints.

Savepoint behavior is dialect-specific. SQL Server supports savepoint and
rollback-to-savepoint via `SAVE TRANSACTION`; release is unsupported.

## Logging

The public logger interface is intentionally small:

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    With(fields ...any) Logger
}
```

Adapters exist for slog, logrus, zap, and apex/log. Passing `nil` disables
logging. Vessel logs structured metadata such as driver, operation, table,
duration, row count, error type, and correlation id; it does not log row data or
connection passwords.

## Observability

Database operations use `internal/pkg/otel.UseTracer`. By default the wrapper
uses the process-global OpenTelemetry tracer provider. If no SDK is configured,
OpenTelemetry is no-op. Set `OTEL_ENABLED=false` to force Vessel to use a no-op
tracer even when the process has a global provider.

Span names are method-oriented, for example `postgres.Get`,
`postgres.QueryRaw`, `postgres.WithTransaction`, and `db.ScanRowsTo`.

## Plugins

Plugins register a `plugin.DriverFactory` by driver name. The factory receives a
`db.DBConfig` and must return a value satisfying `db.DB`.

The plugin registry is for custom connection implementations. It is not a
stable public hook for replacing internal SQL dialect rendering. If a custom
database needs different SQL syntax, keep those queries behind `QueryRaw`/`Exec`
or implement the full `db.DB` contract in the plugin.

## Manager

`manager/v1` is optional. Use it when a service needs multiple database entries,
read/write routing, health-aware selection, bounded queues, or async insert
coalescing. Small services that use one database should use `db.DB` directly.

## Production Guidelines

- Pass request-scoped contexts with deadlines into every DB call.
- Keep raw SQL fragments reviewed and allowlisted.
- Treat unsupported-feature errors as design feedback, not something to ignore.
- Monitor `PoolStats()` and tune per service workload.
- Keep config and credentials outside source control.
- Run `GOWORK=off go test -tags=test ./...` for Vessel-only checks when the
  repository is opened inside a larger workspace.
