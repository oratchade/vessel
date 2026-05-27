# Plugins

This guide covers Vessel's current plugin surface. Plugins register custom
driver factories so `db.NewDB` can create a `db.DB` implementation for a
non-built-in driver name.

The plugin API is intentionally narrow. It does **not** expose a stable public
dialect API, and it does not ask plugin authors to implement a public low-level
connection provider.

## When to Write a Plugin

Write a plugin when:

- You have a database that can be adapted to Vessel's `db.DB` interface.
- You want to package that adapter separately from the core module.
- You want `db.NewDB(customConfig, logger)` to work through the registry.

Do not write a plugin when:

- You only need MySQL, PostgreSQL, SQLite, or MSSQL. Use the built-ins.
- You only need connection-pool or DSN settings. Put those in the config struct.
- You need a stable custom SQL dialect API. That is not public today.

## Public API Shape

Plugin code imports `tounilab.com/vessel/db/v1/plugin`.

```go
type DriverFactory interface {
    Name() string
    Create(ctx context.Context, cfg any) (any, error)
}
```

`Create` returns `any` for registry flexibility, but `db.NewDB` requires the
value to satisfy `db.DB`. If the returned value does not implement `db.DB`,
`NewDB` returns an error.

Your config type must implement `db.DBConfig`:

```go
type DBConfig interface {
    Driver() string
    DSN() string
}
```

`Driver()` must match the factory `Name()`.

## Registering a Driver

Registration is normally done in `init`:

```go
package cockroachdb

import "tounilab.com/vessel/db/v1/plugin"

func init() {
    plugin.MustRegister(&Factory{})
}
```

Use `plugin.Register` when you want to handle duplicate-name errors explicitly,
for example in tests. Use `plugin.Unregister` or `plugin.Clear` only in tests;
they mutate global registry state.

## Example: Reusing the PostgreSQL Driver

CockroachDB is PostgreSQL wire-compatible for many workloads. A plugin can
translate its config to `db.PostgresConfig` and reuse the built-in PostgreSQL
implementation.

```go
package cockroachdb

import (
    "context"
    "fmt"

    db "tounilab.com/vessel/db/v1"
    "tounilab.com/vessel/db/v1/plugin"
)

type Config struct {
    Host     string
    Port     uint16
    User     string
    Password string
    Database string
    SSLMode  string
}

func (c *Config) Driver() string { return "cockroachdb" }

func (c *Config) DSN() string {
    return fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
    )
}

type Factory struct{}

func (f *Factory) Name() string { return "cockroachdb" }

func (f *Factory) Create(ctx context.Context, cfg any) (any, error) {
    _ = ctx // NewDB currently calls factories with context.Background().

    crdbCfg, ok := cfg.(*Config)
    if !ok {
        return nil, fmt.Errorf("expected *cockroachdb.Config, got %T", cfg)
    }

    pgCfg := &db.PostgresConfig{
        Host:     crdbCfg.Host,
        Port:     crdbCfg.Port,
        User:     crdbCfg.User,
        Password: crdbCfg.Password,
        Database: crdbCfg.Database,
        SSLMode:  crdbCfg.SSLMode,
    }

    return db.PostgresCfgToDB(pgCfg, nil)
}

func init() {
    plugin.MustRegister(&Factory{})
}
```

Consumers import the plugin package for side effects and then call `db.NewDB`:

```go
import (
    db "tounilab.com/vessel/db/v1"
    "example.com/internal/cockroachdb"
)

database, err := db.NewDB(&cockroachdb.Config{
    Host:     "db.internal",
    Port:     26257,
    User:     "app",
    Password: password,
    Database: "app",
    SSLMode:  "verify-full",
}, logger)
```

## Conformance Test

Use `db/v1/plugin/conformance` in the plugin's test suite to catch common
contract mistakes.

```go
func TestFactoryConformance(t *testing.T) {
    err := conformance.CheckFactory(
        context.Background(),
        &cockroachdb.Factory{},
        &cockroachdb.Config{ /* test config */ },
    )
    require.NoError(t, err)
}
```

The check verifies factory/config names, creates a database, confirms the
result implements `db.DB`, and closes it.

## Custom Dialects

The built-in dialect implementations live under `internal/pkg/sqldialect`.
That path is intentionally not importable outside the module. A plugin cannot
currently register a new SQL dialect through a stable public API.

For production use, prefer one of these options:

- Reuse an existing built-in implementation when the target database is
  wire-compatible and SQL-compatible enough for your workload.
- Keep dialect-specific queries at the application layer with `DB.QueryRaw` or
  `DB.Exec`.
- Open a PR if the dialect belongs in Vessel's built-in support matrix.

## Production Practices

- Register plugins during process initialization, before serving traffic.
- Avoid `plugin.Clear` and `plugin.Unregister` outside tests.
- Make factory `Create` validate required config fields and return contextual
  errors.
- Ensure the returned `db.DB` implements `Close`, `Ping`, `PoolStats`, and
  transaction methods correctly.
- Add integration tests against the real database. A plugin that only compiles
  can still generate invalid SQL at runtime.
- Document the exact dialect assumptions if the plugin reuses a built-in driver.

## See Also

- [examples/plugin-example](../examples/plugin-example)
- [PORTABILITY_MATRIX.md](./PORTABILITY_MATRIX.md)
