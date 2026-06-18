# vessel

A SQL toolkit for Go services that work with multiple databases.

I built vessel from experience supporting Go services that need the same
data-access shape across different SQL backends. I use it in my own Go
projects today as a small toolkit for query building, typed scanning,
transactions, and database portability.

It is not the right tool for every project. The "Who this is for" and
"Who this isn't for" sections below are honest about that.

**Status:** v0.x — single maintainer, used in the author's projects.
Multi-database test coverage with integration tests across MySQL,
PostgreSQL, SQLite, and MSSQL. Useful for production-style service code, but
not broadly battle hardened. Expect a slow response on issues and PRs.

## What vessel is

A SQL-first data layer that sits between raw `database/sql` and a full ORM.
It provides:

- **Portable query builders** that render correctly across MySQL,
  PostgreSQL, SQLite, and MSSQL — write the query once, run it on any
  supported dialect
- **Typed row scanning** without code generation (`ScanRowsTo[T]`,
  `ScanAll[T]`, `ScanOne[T]`)
- **Explicit unsupported-feature errors** instead of silent SQL generation
  when a dialect cannot do something (MSSQL upsert is the canonical example)
- **Transactions** with options, savepoints, and rollback on callback error
  or panic
- **OpenTelemetry tracing** for database operations when the application
  configures an OpenTelemetry SDK, with `OTEL_ENABLED=false` as an explicit
  kill switch
- **A plugin system** for adding custom database drivers without modifying
  vessel
- **An optional manager package** (`manager/v1`) for services that need
  connection routing, backpressure, and async write coalescing

The query builder and typed scanning live in `db/v1`. The manager is opt-in
through a separate package, so services that don't need it don't pay for it.

## What vessel deliberately doesn't do

Vessel is intentionally smaller than an ORM and intentionally larger than
`database/sql`. It does not provide:

- **No model lifecycle, associations, hooks, or migrations.** Use GORM, Bun,
  or ent if you need those.
- **No compile-time SQL validation.** Use sqlc if compile-time query safety
  matters more than runtime composition.
- **No magic.** Raw SQL stays raw SQL through `QueryRaw` and the trusted-raw
  projection helpers. Vessel will not parse, rewrite, or hide your SQL.
- **No PostgreSQL-specific features in the builder** (`ANY`, array
  operators, `DISTINCT ON`, recursive CTEs, vendor-specific functions).
  These remain raw SQL or view-backed.
- **No MSSQL upsert generation.** Vessel returns an explicit error rather
  than generating `MERGE`.

If you need any of the above, vessel is the wrong tool.

## Who this is for

- Go services that work across multiple SQL databases — for example, SQLite
  in tests and PostgreSQL in production
- Teams that want portability without committing to an ORM
- Personal or team-owned services that need one small SQL toolkit across
  different environments
- Plugin authors who need to add support for a custom or proprietary
  database without forking the library
- Projects that want OpenTelemetry tracing on database operations without
  wiring it manually

## Who this isn't for

- Services targeting one database where dialect-specific features matter —
  use that database's native library (`pgx` for PostgreSQL is the clearest
  example)
- Teams that want compile-time SQL safety — use `sqlc`
- Teams that want a full ORM with relationships, migrations, and hooks —
  use GORM, Bun, or ent
- Teams that write almost entirely raw SQL and want only a thin scanning
  wrapper — use `sqlx`
- Projects that need a broadly proven database layer today — vessel is v0.x and
  single-maintainer

## Quick start

```bash
go get tounilab.com/vessel
```

Requires Go 1.26 or later.

### Connect

```go
import db "tounilab.com/vessel/db/v1"

database, err := db.NewDB(db.PostgresConfig{
    User:     "user",
    Password: "password",
    Host:     "localhost",
    Port:     5432,
    Database: "mydb",
}, nil)
if err != nil {
    log.Fatal(err)
}
defer database.Close()
```

### Query with the builder and typed scanning

```go
import (
    db  "tounilab.com/vessel/db/v1"
    cdt "tounilab.com/vessel/pkg/query/condition"
)

type User struct {
    ID    int
    Name  string
    Email string
}

rows, err := database.GetRaw(ctx, "users",
    []string{"id", "name", "email"},
    nil,
    cdt.NewExpr().Column("active").Op("=").Value(true),
    nil)
if err != nil {
    return err
}

users, err := db.ScanRowsTo[User](ctx, rows)
// rows are closed automatically by ScanRowsTo
```

### Transactions

```go
err := database.WithTransaction(ctx, func(tx db.Tx) error {
    _, err := tx.Insert(ctx, "users", map[string]any{
        "name":  "Alice",
        "email": "alice@example.com",
    }, nil)
    return err
})
```

### Portable upsert

```go
fdb := db.NewFluentDB(database)

_, err := fdb.Insert().
    Into("users").
    Set("id", userID).
    Set("email", email).
    OnConflict("id").
    DoUpdate("email").
    Upsert(ctx)
// Renders ON CONFLICT for PostgreSQL/SQLite, ON DUPLICATE KEY UPDATE for
// MySQL, returns an explicit unsupported error on MSSQL.
```

Use `ValuesBulk(...).Upserts(ctx)` for multi-row upsert.

See [examples/](./examples) for more, including transactions, the manager
package, and plugin authoring.

## Dialect support summary

| Capability                            | MySQL | PostgreSQL | SQLite | MSSQL          |
|---------------------------------------|-------|------------|--------|----------------|
| CRUD, builders, typed scanning        | yes   | yes        | yes    | yes            |
| Transactions, options                 | yes   | yes        | yes    | yes            |
| Savepoints                            | yes   | yes        | yes    | partial        |
| Portable upsert                       | yes   | yes        | yes    | explicit error |
| EXPLAIN and query preview             | yes   | yes        | yes    | yes            |
| Connection pool statistics            | yes   | yes        | yes    | yes            |

Full feature-by-dialect breakdown, including unsupported paths and raw-SQL
boundaries, in [docs/PORTABILITY_MATRIX.md](./docs/PORTABILITY_MATRIX.md).

## Documentation

Detailed reference material lives in `/docs`:

- [Architecture](./docs/ARCHITECTURE.md) — system design, layers, extension
  points
- [Portability matrix](./docs/PORTABILITY_MATRIX.md) — dialect feature
  support and unsupported paths
- [Error handling](./docs/ERROR_HANDLING.md) — error types, NULL handling,
  dialect-specific errors
- [DB Manager](./docs/DB_MANAGER.md) — the optional package for connection
  routing, backpressure, and async write coalescing
- [FluentDB](./docs/FLUENTDB.md) — current fluent builder API
- [Configuration](./docs/CONFIGURATION.md) — dialect configs and pool tuning
- [Logging](./docs/LOGGING.md) and [observability](./docs/OBSERVABILITY.md) —
  production logging and tracing
- [Plugin system](./docs/PLUGINS.md) — adding custom database
  drivers
- [Environment variables](./docs/ENVIRONMENT_VARIABLES.md) — configuration
  for tests and runtime

## Contributing

Vessel is single-maintainer and v0.x. Issues and PRs are welcome; expect
slow response. See [CONTRIBUTING.md](./CONTRIBUTING.md) for the
contribution flow.

## License

MIT — see [LICENSE.md](./LICENSE.md).
