# Schema migrations

`migrate/v1` applies versioned SQL migrations with
[goose](https://github.com/pressly/goose). It runs SQL files you write. It does
not generate schema from Go types, and it does not make SQL portable: each
dialect keeps its own migration files.

## Usage

```go
import (
    "embed"
    "io/fs"

    db "tounilab.com/vessel/db/v1"
    migrate "tounilab.com/vessel/migrate/v1"
)

//go:embed migrations/*.sql
var files embed.FS

func migrateUp(ctx context.Context, cfg db.PostgresConfig) error {
    migrations, err := fs.Sub(files, "migrations")
    if err != nil {
        return err
    }
    m, err := migrate.New(cfg, migrations)
    if err != nil {
        return err
    }
    defer m.Close()

    results, err := m.Up(ctx)
    for _, r := range results {
        log.Printf("applied %s in %s", r.Source, r.Duration)
    }
    return err
}
```

- `New` takes the same `DBConfig` as `db.NewDB`. It opens a connection pool of
  its own from `Driver()` and `DSN()`, which `Close` releases. No `*sql.DB` is
  exposed, and Vessel's own pools are not touched.
- `Up` applies every pending migration in version order. A second `Up` applies
  nothing. On failure it returns the migrations applied before the failure and
  an error naming the failing file; the database stays at the last migration
  that succeeded.
- `Status` lists every migration, applied or pending. `Version` returns the
  highest applied version, 0 when none is.
- `WithVersionTable(name)` changes the table that records applied migrations
  (default `goose_db_version`).

There is no `Down`. Add it when a consumer needs it.

## Migration files

Files are named `<version>_<description>.sql`, for example `00002_add_index.sql`,
and follow goose's SQL format:

```sql
-- +goose Up
CREATE TABLE items (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL);

-- A statement with inner semicolons (a function or trigger body) must be marked,
-- or goose splits it at the first semicolon.
-- +goose StatementBegin
CREATE FUNCTION touch() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
```

Each file runs in a transaction. Put `-- +goose NO TRANSACTION` at the top of a
file whose statements refuse one, such as PostgreSQL's
`CREATE INDEX CONCURRENTLY` or SQLite's `VACUUM`.

goose can also run migrations written in Go. They receive a `*sql.Tx` or
`*sql.DB`, not a Vessel handle, because Vessel's PostgreSQL backend uses pgx
directly and cannot wrap that transaction. `migrate/v1` does not expose them
yet.

## Dialects

| Vessel driver | `database/sql` driver | goose dialect | Concurrent runs |
| --- | --- | --- | --- |
| `postgres`, `postgresql` | `pgx` (pgx/v5/stdlib) | `postgres` | Serialized by an advisory lock, scoped to the database |
| `mysql` | `mysql` | `mysql` | Serialized by a lock table (`goose_lock`) |
| `sqlite` | `sqlite` (modernc) | `sqlite3` | The caller's responsibility |
| `sqlserver`, `mssql` | `sqlserver` | `mssql` | The caller's responsibility |

- **MySQL** needs `ParseTime: true`: goose reads the applied-at timestamps back.
  `New` returns `ErrMySQLParseTime` otherwise.
- **SQLite** migrations run on a single connection, because every connection to
  an in-memory database is a separate, empty database.
- Drivers registered through the plugin package return `ErrUnsupportedDriver`.

## Concurrency

Two processes migrating the same database at once, such as two replicas
starting together, apply each migration exactly once on PostgreSQL and MySQL:
the second waits for the first and then finds nothing pending. On PostgreSQL the
waiting process checks the lock every 5 seconds. Migrating two *different*
databases at once never contends.
