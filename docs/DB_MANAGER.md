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

Health checks call `Ping()` periodically. An entry is marked unhealthy after
five consecutive failures and healthy again after the next successful ping.

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

## Synchronous API

The synchronous methods block until the queued operation returns or the context
is canceled.

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
- `Exec`
- `Ping`, `HealthStatus`

`GetRaw`, `GetByIDRaw`, and `QueryRaw` return `*db.RowsAdapter`. Use
`ScanRowsTo`, `ScanAll`, `ScanOne`, or close the rows manually.

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
- `ExecAsync`

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
worker. `UpsertAsync` and `UpsertsAsync` requests are not coalesced; they flush
any pending insert batch first, then execute directly.

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
