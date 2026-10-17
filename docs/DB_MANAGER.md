# DB Manager

`manager/v1` is an optional package for services that need to route work across
multiple database connections. Most services should use `db/v1` directly.

Use `DBManager` when you need read/write routing, read replicas, bounded
request queues, background workers, health-aware selection, or opt-in async
insert coalescing.

## Configuration Shape

`NewDBManager(ctx, configPath, logger, envOpts...)` loads YAML, JSON, or TOML.
The config uses snake_case field names.

```yaml
read_queue_size: 1000
write_queue_size: 500
read_workers: 4
write_workers: 2
health_interval: 30s

write_batching_enabled: false
write_batch_max_rows: 100
write_batch_max_delay: 5ms

entries:
  - name: primary
    type: readwrite
    priority: 100
    postgres:
      user: app
      password: ${DB_PASSWORD}
      host: postgres.internal
      port: 5432
      database: app
      ssl_mode: require

  - name: replica
    type: readonly
    priority: 50
    read_workers: 8
    postgres:
      user: app_readonly
      password: ${DB_PASSWORD}
      host: replica.internal
      port: 5432
      database: app
      ssl_mode: require
```

Entry `type` values are `readonly` and `readwrite`. Each entry must configure
exactly one database config: `mysql`, `postgres`, `sqlite`, or `mssql`.

Since v0.2.0, configuration is validated fail-fast at load: every entry must
have a `name`, `type` is required (a missing or misspelled value is an error,
not a silently dropped entry), and entry names must be unique.

Per-entry queue, worker, health, priority, and batch settings override global
settings.

## Environment Expansion

Environment expansion is opt-in. Variables use `${VAR}` or `${VAR:default}`.

```go
import mgr "tounilab.com/vessel/manager/v1"

dm, err := mgr.NewDBManager(ctx, "config.yaml", logger,
    mgr.WithEnvVars(map[string]string{
        "DB_PASSWORD": os.Getenv("DB_PASSWORD"),
    }),
)
```

Available options:

- `WithEnvVars(map[string]string{...})` for explicit values.
- `WithEnvPrefix("DB_", "VESSEL_")` for selected process environment values.
- `WithEnvFile(".env")` for local development files.

For production secrets, prefer explicit values from your secret store via
`WithEnvVars`. Avoid relying on `.env` files in deployed environments.

A substituted value is only ever a value, never syntax, so any password works as it is.
While the file parses, each placeholder stands as a number, which every format accepts
both inside a string and as a bare value; values then go into the parsed data, and the
format's own library escapes them. Defaults are values too.

- **In a string** of any kind (YAML quoted, a block scalar, JSON, every TOML string), a
  value goes in as text, exactly as it is.
- **Unquoted in YAML**, a value is trimmed and typed from what it expands to, as if
  written there: `port: ${DB_PORT:5432}` is a number, `health_interval: ${HEALTH:30s}`
  a duration. A non-empty value that would read as null, such as `null` or `~`, stays a
  string.
- **Outside quotes in JSON and TOML**, a placeholder that is the whole value is typed as
  if written there: a number (in TOML, an integer, with its bases and underscores),
  `true`, `false`, or JSON's `null`; surrounding spaces don't count. Anything else is a string, never
  structure, so a numeric field rejects it. A placeholder glued to other text outside
  quotes (`54${P}`, `${A}${B}`) fails the load: quote it.
- **In a key**, a value is that one key: a dot in it does not reach into another table.
  In JSON and TOML, two keys of one object that name the same field, equal or equal but
  for case, fail the load.
- **Inside an escape or a number**, where the file's syntax would read part of the
  placeholder (`"\u${X}"`, `"\x${X}"`, `${X}.5` in TOML), the load fails naming it: a
  value never builds an escape or a number. Where a backslash is text, as in a TOML
  literal string or YAML outside double quotes, a placeholder after it expands as usual.
- **In YAML**, an anchor can be named by a placeholder if its alias repeats it as
  written (`&${A}` … `*${A}`). A placeholder in a tag fails the load. A placeholder in a
  comment is never expanded.

A value or a default that is not valid UTF-8 fails the load, naming the variable. A
placeholder whose default spans several lines stands as a one-line token while YAML
parses, so a YAML parse or decode error after it reports a line number short by those
lines.

A load or validation error never quotes a substituted value. A decoder error at a YAML
line or a JSON or TOML key holding one names the placeholder instead
(`entries.postgres.port: the value of ${DB_PORT} is not valid here`), since decoders
quote part of what they reject. Any other has each value replaced with `[REDACTED]`, in
every form a message can give it (trimmed, `%q`-escaped, escaped in a connection string,
or decoded by a YAML `!!binary` tag): which field is secret is not known before the
config is decoded.

From setup on it is known: once anything was substituted, a setup error and every line
the manager logs for its lifetime, including what the drivers and entries log through
it, have each password the config holds redacted the same way. The user, host and
database stay readable, so an operator can tell what failed. The trace spans the
drivers record are not redacted.

A redacted error is a new error rather than a wrapper, since a driver's error can keep
the connection string. It unwraps only to the causes that hold no value, so
`errors.Is(err, syscall.ECONNREFUSED)` and `errors.Is(err, context.DeadlineExceeded)`
still work; `errors.As` reaches no driver error.

## Lifecycle

```go
import mgr "tounilab.com/vessel/manager/v1"

dm, err := mgr.NewDBManager(ctx, "config.yaml", logger)
if err != nil {
    return err
}

dm.Start()
defer dm.Stop()
```

`Start` and `Stop` are idempotent. Calls before `Start` return
`ErrManagerNotStarted`; calls after shutdown begins return `ErrManagerClosed`.

## Routing

Reads use `readonly` entries first. If no read-only entry exists, reads can use
`readwrite` entries. Writes require `readwrite` entries.

Selection is health-aware:

1. Prefer healthy entries.
2. Pick the highest priority group.
3. Round-robin within the same priority.
4. If every entry in a category is unhealthy, fall back to all entries by
   priority so the caller receives the real database error.

Health checks call `Ping()` at each entry's health interval. An entry is marked
unhealthy after five consecutive failures and healthy again after the next
successful ping. Each ping is bounded by the interval: a ping slower than that
counts as a failure, even when the driver ignores cancellation, and no new ping
starts while one is still outstanding. A database that stalls without closing
its connections therefore turns unhealthy too.

Use exported health methods:

```go
status := dm.HealthStatus()
log.Printf("readonly healthy=%d/%d readwrite healthy=%d/%d",
    status.ReadOnlyHealthy,
    status.ReadOnlyTotal,
    status.ReadWriteHealthy,
    status.ReadWriteTotal,
)

if _, err := dm.Ping(ctx); err != nil {
    return fmt.Errorf("manager health check: %w", err)
}
```

## Transactions

`WithTransaction(ctx, entryName, fn, opts...)` runs `fn` inside a database
transaction on the named `readwrite` entry:

```go
import (
    "database/sql"

    db "tounilab.com/vessel/db/v1"
)

err := dm.WithTransaction(ctx, "primary", func(tx db.Tx) error {
    // Lock the owning row before reading the set it guards.
    if _, err := tx.Query(ctx, "SELECT id FROM routes WHERE id = $1 FOR UPDATE", routeID); err != nil {
        return err
    }
    if _, err := tx.Exec(ctx, "DELETE FROM route_stops WHERE route_id = $1", routeID); err != nil {
        return err
    }
    _, err := db.NewFluentDB(tx).Insert().Into("route_stops").ValuesBulk(stops).Exec(ctx)
    return err
}, db.TransactionOptions{Isolation: sql.LevelReadCommitted})
```

- The transaction commits when `fn` returns nil and rolls back when `fn`
  returns an error or panics. A panic is returned as an error.
- Every statement issued through `tx`, including a plain
  `SELECT ... FOR UPDATE`, runs on the transaction's connection. Read/write
  routing does not apply inside the callback, so a lock is never taken on a
  replica. Do not call `dm.Get`, `dm.Insert`, and similar methods inside `fn`:
  they are routed and queued as independent statements outside the
  transaction.
- Transactions bypass the worker queues and hold a pool connection for the
  whole callback. Keep them short.
- Before touching the database, `WithTransaction` returns
  `ErrManagerNotStarted` or `ErrManagerClosed`, `ErrEntryNotFound` for an
  unknown name, `ErrEntryReadOnly` for a `readonly` entry (a transaction on a
  replica protects nothing), and `ErrEntryUnhealthy` for an entry failing
  health checks.
- `WithTransaction` never retries. To retry, for example on a serialization
  failure, wrap the whole `WithTransaction` call in your retry loop or a
  `QueryWithRetry` callback. Never retry individual statements inside `fn`.
- `Stop` closes connections without waiting for in-flight transactions. Finish
  them before shutdown.

## Synchronous API

The synchronous methods block until the queued operation returns or the context
is canceled. The context governs the query itself, not only the wait: a query
runs in a context derived from the caller's, so cancelling it, or reaching its
deadline, cancels the statement in the database. A caller with no deadline gets
the 30-second default, which then bounds the query too. A query whose caller
gave up while it waited in the queue is not run. To keep work running after its
caller's context ends, detach it with `context.WithoutCancel`. Batched inserts
are the exception: one batch carries several callers' rows, so it runs with the
worker's context and no single caller cancels it.

```go
rows, err := dm.Get(
    ctx,
    "users",
    []string{"id", "email"},
    nil,
    cdt.NewExpr().Column("active").Op("=").Value(true),
    nil,
)
```

```go
result, err := dm.Insert(ctx, "users", map[string]any{
    "id":    userID,
    "email": email,
}, nil)
if err != nil {
    return err
}
log.Printf("rows affected=%d", result.RowsAffected)
```

```go
result, err := dm.Upsert(
    ctx,
    "users",
    map[string]any{
        "email": email,
        "name":  name,
    },
    &options.UpsertOptions{
        ConflictColumns: []string{"email"},
        Action:          options.UpsertDoUpdate,
        UpdateColumns:   []string{"name"},
    },
    nil,
)
if err != nil {
    return err
}
log.Printf("rows affected=%d", result.RowsAffected)
```

Supported synchronous methods:

- `Get`, `GetRaw`, `GetByID`, `GetByIDRaw`
- `Query`, `QueryRaw`
- `Insert`, `Inserts`, `Upsert`, `Upserts`, `Update`, `Delete`
- `Exec`, `ExecReturning`
- `Ping`, `HealthStatus`

`GetRaw`, `GetByIDRaw`, `QueryRaw`, and `ExecReturning` return
`*db.RowsAdapter`. Use `ScanRowsTo`, `ScanAll`, `ScanOne`, or close the rows
manually.

These rows keep their query's context alive until they are closed, because the
driver reads them under it: read and close them while that context lasts.
`RowsAdapter.OnClose(f)` registers a function to run once they are closed; every
function registered runs, so it never replaces the manager's own cleanup.

If the caller's context is cancelled, or the default timeout expires, before the
rows arrive, these methods cancel the query, return the context error, and close
the rows if they still arrive, so their connection goes back to the pool. Their async
variants (`GetRawAsync`, `GetByIDRawAsync`, `QueryRawAsync`,
`ExecReturningAsync`) hand the response to the caller, who must close `RawData`.

`ExecReturning` runs an `INSERT`, `UPDATE`, or `DELETE` that returns rows
(PostgreSQL `RETURNING`, MSSQL `OUTPUT`) on a read-write entry, never on a
read-only replica. Pass SQL from a FluentDB mutation builder's
`Returning(...).Query()`, or trusted raw SQL. MySQL and SQLite entries return an
unsupported error without executing the statement.

```go
rows, err := dm.ExecReturning(ctx, query, args...)
if err != nil {
    return err
}
routes, err := db.ScanAll[Route](ctx, rows)
```

## Async API

Async methods return a response channel and an immediate enqueue error:

```go
respCh, err := dm.GetAsync(ctx, "users", []string{"id", "email"}, nil, nil, nil)
if err != nil {
    return err
}

select {
case resp := <-respCh:
    if resp.Error != nil {
        return resp.Error
    }
    handleRows(resp.Data)
case <-ctx.Done():
    return ctx.Err()
}
```

Supported async methods mirror the synchronous methods with an `Async` suffix:

- `GetAsync`, `GetRawAsync`, `GetByIDAsync`, `GetByIDRawAsync`
- `QueryAsync`, `QueryRawAsync`
- `InsertAsync`, `InsertsAsync`, `UpsertAsync`, `UpsertsAsync`, `UpdateAsync`,
  `DeleteAsync`
- `ExecAsync`, `ExecReturningAsync`

`PingAsync` is named for compatibility but checks the selected connection
immediately and returns `error`, not a response channel.

The response type is:

```go
type QueryResponse struct {
    RequestID string
    Data      []map[string]any
    RawData   *db.RowsAdapter
    ExecData  *db.ExecResult
    Error     error
}
```

Always check `resp.Error` before reading `Data`, `RawData`, or `ExecData`.

## Insert Coalescing

Automatic insert batching is disabled by default. When enabled, compatible
`InsertAsync` requests may be flushed as one `Inserts` call by the same write
worker. `UpsertAsync`, `UpsertsAsync`, and `ExecReturningAsync` requests are not
coalesced; they flush any pending insert batch first, then execute directly.

Requests are compatible when they target the same worker, table, query options,
and column set. A batch flushes when it reaches `write_batch_max_rows`, waits
`write_batch_max_delay`, sees an incompatible write, or the manager stops.

Use this only after measuring a write-heavy workload. It changes execution
timing and can make per-row error attribution less precise.

## Production Practices

- Use bounded contexts for every call.
- Size queues and workers from load tests, not defaults copied across services.
- Monitor queue saturation and database `PoolStats`.
- Keep read and write priorities simple, for example `100`, `50`, `10`.
- Treat fallback to unhealthy entries as degraded mode; alert on it.
- Call `Stop` during shutdown so workers exit and database connections close.
- Prefer synchronous methods unless your service genuinely benefits from async
  queueing.

## Troubleshooting

### Queries Fail Before Enqueue

Check whether `Start` has been called, the manager is shutting down, or the
context is already canceled.

### Queries Wait Too Long

The queue may be saturated, workers may be overloaded, or the database pool may
be saturated. Increase queues/workers only after checking database capacity and
query latency.

### Reads Hit The Writer

Reads use `readwrite` entries when no `readonly` entry exists or when all
read-only entries are unavailable. Check `HealthStatus` and entry types.

### All Entries Are Unhealthy

`Ping(ctx)` returns an error when an entire configured category is unhealthy.
Check credentials, network access, DNS, and the underlying database logs.

## See Also

- [CONFIGURATION.md](./CONFIGURATION.md)
- [ERROR_HANDLING.md](./ERROR_HANDLING.md)
- [RESOURCE_POOLING.md](./RESOURCE_POOLING.md)
