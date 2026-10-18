# Configuration

This guide covers runtime configuration for vessel: per-dialect connection
configs and connection pool tuning. For test environment credentials
(`DB_MYSQL_USER` etc.), see [ENVIRONMENT_VARIABLES.md](./ENVIRONMENT_VARIABLES.md).

**Target Audience**: Application developers, platform engineers, AI agents.

## Overview

Every database is opened through `db.NewDB(config, logger)`. The `config`
argument is one of the per-dialect config structs:

- `db.MysqlConfig` — MySQL
- `db.PostgresConfig` — PostgreSQL (uses `pgxpool`)
- `db.SQLiteConfig` — SQLite (pure-Go `modernc.org/sqlite`)
- `db.MSSQLConfig` — Microsoft SQL Server

All four config structs implement the `db.DBConfig` interface, which is what
`NewDB` accepts. The same `DB` interface is returned regardless of dialect, so
application code that goes through the `DB` interface stays portable.

`Host` takes a name, an IPv4 address or an IPv6 address. An IPv6 address may be
written bare or in brackets, `fd00::5` or `[fd00::5]`, and is bracketed beside the
port as the drivers need. A zone (`fe80::1%eth0`) reaches PostgreSQL and MySQL; for
SQL Server, `MSSQLConfig.Validate` refuses one, since its driver dials without it. Write
the zone raw, `fe80::1%eth0`: the DSN escapes it, so the URL form `%25eth0` would reach
the driver as the zone `25eth0`. Any other host is passed as written.

## Connection pool concepts

vessel exposes connection pool tuning through the config structs. The exact
field names differ between dialects because MySQL/SQLite/MSSQL use
`database/sql`'s pool while PostgreSQL uses `pgxpool`. The concepts map
1:1:

| Concept                  | MySQL/SQLite/MSSQL field | PostgreSQL field     |
| ------------------------ | ------------------------ | -------------------- |
| Maximum open connections | `MaxOpenConns`           | `PoolMaxConns`       |
| Minimum/idle connections | `MaxIdleConns`           | `PoolMinConns`       |
| Connection lifetime      | `ConnMaxLifetime`        | `PoolMaxConnLife`    |
| Idle connection timeout  | Not currently exposed    | `PoolMaxConnIdle`    |

Connection acquisition blocks under load when the pool is saturated. That is
the natural backpressure signal — callers experience it as latency. Tune the
pool to the workload rather than to the database server's connection limit.

## MySQL

```go
import (
    "time"
    db "tounilab.com/vessel/db/v1"
)

cfg := db.MysqlConfig{
    User:            "app_user",
    Password:        password,
    Host:            "mysql.internal",
    Port:            3306,
    Database:        "app",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 15 * time.Minute,
}

database, err := db.NewDB(cfg, logger)
if err != nil {
    log.Fatal(err)
}
defer database.Close()
```

Notes:

- MySQL has no native upsert in the form vessel exposes through
  `OnConflict/DoUpdate`. vessel renders `INSERT ... ON DUPLICATE KEY UPDATE`
  when those builder methods are used.
- Use `Charset`, `ParseTime`, `Loc`, `Timeout`, `ReadTimeout`, and
  `WriteTimeout` when those DSN options are required.

## PostgreSQL

```go
cfg := db.PostgresConfig{
    User:            "app_user",
    Password:        password,
    Host:            "postgres.internal",
    Port:            5432,
    Database:        "app",
    SSLMode:         "verify-full",
    ApplicationName: "orders-api",
    PoolMaxConns:    25,
    PoolMinConns:    5,
    PoolMaxConnIdle: 15 * time.Minute,
    PoolMaxConnLife: 1 * time.Hour,
}

database, err := db.NewDB(cfg, logger)
```

Notes:

- PostgreSQL is the dialect with the richest feature support in vessel:
  `RETURNING` preview, parameterized `HAVING`, `ON CONFLICT` upsert, joined
  UPDATE/DELETE.
- `SSLMode`, `ApplicationName`, `SearchPath`, and `ConnectTimeout` are included
  in the generated DSN when set.
- Set `PoolMaxConnIdle` and `PoolMaxConnLife` for long-running services so
  stale connections are recycled.

## SQLite

```go
cfg := db.SQLiteConfig{
    FilePath:    "/var/lib/app/app.db",
    ForeignKeys: true,
    BusyTimeout: 5 * time.Second,
    MaxOpenConns: 1,
}

database, err := db.NewDB(cfg, logger)
```

Notes:

- SQLite uses the pure-Go `modernc.org/sqlite` driver. The driver name
  registered with `database/sql` is `sqlite`, not `sqlite3`.
- Use `":memory:"` as `FilePath` for an in-memory database (useful in
  tests).
- `CacheMode`, `Mode`, `ForeignKeys`, and `BusyTimeout` are encoded into the
  SQLite DSN when set.
- SQLite serialises writes through file locking. Concurrent readers are
  fine in WAL mode; concurrent writers will queue. Configure pool size
  accordingly — for a single-process service, `MaxOpenConns: 1` is often
  the right choice when writes dominate.
- Joined `DELETE` is not supported in vessel for SQLite (returns an
  explicit error).

## MSSQL

```go
cfg := db.MSSQLConfig{
    User:              "app_user",
    Password:          password,
    Host:              "mssql.internal",
    Port:              1433,
    Database:          "app",
    Encrypt:           "true",
    TrustServerCert:   false,
    ConnectionTimeout: 10 * time.Second,
    MaxOpenConns:      25,
    MaxIdleConns:      5,
}

database, err := db.NewDB(cfg, logger)
```

Notes:

- `Encrypt` is optional, and unset means `"true"`: the session is encrypted, a server
  without encryption is refused, and its certificate is verified unless
  `TrustServerCert` is set, as a self-signed one needs. `"false"` encrypts at most the
  login, and `"disable"` nothing.
- A named instance is written as the host, `Host: "srv\\INSTANCE"`. The driver then
  finds the instance's port through SQL Server Browser (UDP 1434), so `Port` is not sent
  with an instance; to connect on a fixed port, give the host without it. A named
  instance cannot be found at an IPv6 address: the driver asks SQL Server Browser at an
  address it writes without brackets. `MSSQLConfig.Validate` refuses that host, as
  opening a manager and `migrate.New` do; give the instance's port instead.
- vessel does **not** generate `MERGE` for MSSQL upserts. Calls to
  `OnConflict/DoUpdate/Upsert` against MSSQL return an explicit
  unsupported-feature error rather than silently producing risky SQL.
- MSSQL pagination uses `ORDER BY ... OFFSET ... FETCH NEXT`. Provide a stable
  `OrderBy` for production pagination.
- Savepoint release is not supported by SQL Server; vessel's `Tx`
  returns an explicit error if `ReleaseSavepoint` is called on MSSQL.
  `Savepoint` and `RollbackToSavepoint` work via `SAVE TRANSACTION`.

## Pool tuning recommendations

These are starting points, not absolute rules. Profile your workload
before increasing pool sizes — connection-pool saturation is a real
problem at scale, but pool sizes that exceed what the database server
allows or what the workload genuinely needs create their own issues
(idle resource consumption, slower failure modes, harder debugging).

### High-traffic HTTP API

```go
cfg := db.PostgresConfig{
    // ...
    PoolMaxConns:    25,
    PoolMinConns:    5,
    PoolMaxConnIdle: 15 * time.Minute,
    PoolMaxConnLife: 1 * time.Hour,
}
```

Pair with sane HTTP-server timeouts so that a slow query doesn't pin a
connection indefinitely.

### Batch / background worker

```go
cfg := db.MysqlConfig{
    // ...
    MaxOpenConns:    10,             // Long-running queries, fewer connections
    MaxIdleConns:    2,
    ConnMaxLifetime: 1 * time.Hour,  // Less churn for stable workloads
}
```

### Local development / tests

```go
cfg := db.SQLiteConfig{
    FilePath: ":memory:",
}
```

In-memory SQLite is the lightest option for tests and gives test-vs-prod
portability when application code is dialect-portable through vessel's
builders.

## Monitoring pool health

```go
stats, err := database.PoolStats()
if err != nil {
    return err
}

log.Printf("Open: %d, InUse: %d, Idle: %d, Wait count: %d, Wait time: %v",
    stats.OpenConnections, stats.InUse, stats.Idle,
    stats.WaitCount, stats.WaitDuration,
)
```

If `WaitCount` grows steadily, the pool is saturated. Either increase
`MaxOpenConns`/`PoolMaxConns` or reduce time-per-query. Don't increase
the pool blindly without checking what the database server can handle.

## Best practices

### ✅ DO

- Use environment variables or a secrets manager for credentials. Never
  commit credentials to source control.
- Tune pool size to the workload, not to the database server's
  hard limit.
- Set a context timeout on every query so a stuck query cannot pin a
  pool connection forever.
- Monitor `PoolStats()` in production and alert on sustained saturation.
- Use SQLite in-memory for tests when application code routes through
  vessel's portable builders.

### ❌ DON'T

- Don't size the pool to the database server's max connections in a
  single service. Other services and humans need connections too.
- Don't call `database.Close()` from inside a request handler.
  `Close()` is for application shutdown.
- Don't share one `MysqlConfig`/`PostgresConfig` across services with
  different workloads. Different services should have different pool
  sizes.
- Don't ignore `WaitCount` and `WaitDuration` in production. Sustained
  non-zero values are the early warning sign of pool saturation.

## Troubleshooting

### "Too many connections" from the database server

The pool plus other services exceed the server's `max_connections`.
Reduce `MaxOpenConns`/`PoolMaxConns` per service, or increase the
server's limit if the workload genuinely needs it.

### Queries time out under load but the database is healthy

Pool saturation. Check `PoolStats()`. If `WaitCount` is high and
`WaitDuration` is non-trivial, the pool is the bottleneck, not the
database.

### Connections accumulate idle and never close

For PostgreSQL, set `PoolMaxConnIdle`. For MySQL, SQLite, and MSSQL, vessel
currently exposes `MaxIdleConns` and `ConnMaxLifetime`; it does not expose
`database/sql`'s idle-timeout knob on those configs. Keep `MaxIdleConns`
conservative and set `ConnMaxLifetime` so long-lived connections are recycled.

### MSSQL upsert returns an error

vessel does not generate `MERGE` for MSSQL. Use the standard
`INSERT` + conditional `UPDATE` pattern in a transaction, or use a
view-backed alternative. This is intentional — `MERGE` on MSSQL has
edge-case correctness issues that vessel will not paper over.

## See Also

- [ARCHITECTURE.md](./ARCHITECTURE.md) — system design and connection
  pooling strategy at the library level
- [PORTABILITY_MATRIX.md](./PORTABILITY_MATRIX.md) — feature support
  per dialect and unsupported paths
- [ENVIRONMENT_VARIABLES.md](./ENVIRONMENT_VARIABLES.md) — test
  credentials and CI configuration
- [DB_MANAGER.md](./DB_MANAGER.md) — connection routing across multiple
  databases (optional `manager/v1` package)
