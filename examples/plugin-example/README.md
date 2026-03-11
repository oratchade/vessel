# Plugin System Example

This directory demonstrates how to implement custom database drivers using the **db-connector plugin system**.

## Overview

The db-connector plugin system allows you to register custom database drivers without modifying the core library. This example shows:

1. **CockroachDB Plugin** - A complete driver implementation wrapping PostgreSQL (since CockroachDB is wire-compatible)
2. **Main Example** - How to use custom plugins in your application

## Plugin Architecture

### Core Concepts

**DriverFactory Interface** (defined in `db/v1/plugin/registry.go`):

```go
type DriverFactory interface {
    Name() string
    Create(ctx context.Context, cfg interface{}) (interface{}, error)
}
```

### Plugin Registration Pattern

Each plugin must:

1. **Implement `DriverFactory` interface**

   ```go
   type Factory struct{}

   func (f *Factory) Name() string {
       return "cockroachdb"
   }

   func (f *Factory) Create(ctx context.Context, cfg interface{}) (interface{}, error) {
       // Convert config and create driver
       return db.PostgresCfgToDB(pgConfig)
   }
   ```

2. **Auto-register via `init()`**

   ```go
   func init() {
       plugin.MustRegister(&Factory{})
   }
   ```

3. **Import as blank import in main**
   ```go
   import _ "your-module/examples/plugin-example/cockroachdb"
   ```

## Directory Structure

```
examples/plugin-example/
├── cockroachdb/
│   └── driver.go          # CockroachDB plugin implementation
├── main.go                # Usage example
└── README.md              # This file
```

## CockroachDB Plugin Details

### Config Struct

```go
type Config struct {
    Host     string
    Port     int
    User     string
    Password string
    Database string
    SSLMode  string
}

func (c *Config) Driver() string { return "cockroachdb" }
func (c *Config) DSN() string    { /* PostgreSQL-compatible DSN */ }
```

**Key Feature:** Implements `db.DBConfig` interface, allowing it to work with `NewDB()`.

### Factory Implementation

The factory's `Create()` method:

1. Receives the config as `interface{}`
2. Type asserts to `*Config`
3. Converts CockroachDB config to PostgreSQL config
4. Calls `db.PostgresCfgToDB()` to reuse the built-in PostgreSQL driver

```go
func (f *Factory) Create(ctx context.Context, cfg interface{}) (interface{}, error) {
    crdbCfg := cfg.(*Config)
    pgCfg := &db.PostgresConfig{
        Host:     crdbCfg.Host,
        Port:     crdbCfg.Port,
        User:     crdbCfg.User,
        Password: crdbCfg.Password,
        Database: crdbCfg.Database,
        SSLMode:  crdbCfg.SSLMode,
    }
    return db.PostgresCfgToDB(pgCfg)
}
```

**Why This Works:** CockroachDB uses the same wire protocol as PostgreSQL, so we can wrap the PostgreSQL driver implementation.

## Usage Example

### 1. Create a Config Instance

```go
cfg := &cockroachdb.Config{
    Host:     "localhost",
    Port:     26257,
    User:     "root",
    Password: "password",
    Database: "testdb",
    SSLMode:  "require",
}
```

### 2. Open a Connection

```go
database, err := db.NewDB(cfg, nil)
if err != nil {
    log.Fatal(err)
}
defer database.Close()
```

**How It Works:**

1. `NewDB()` checks the plugin registry for driver "cockroachdb"
2. Finds the CockroachDB factory
3. Calls `factory.Create(ctx, cfg)`
4. Returns the PostgreSQL-based driver instance

### 3. Use the Database

All standard db-connector operations work:

```go
// Insert
result, err := database.Insert(ctx, "users", map[string]any{
    "name":  "Alice",
    "email": "alice@example.com",
}, nil)

// Query
users, err := database.Get(ctx, "users", columns, nil, nil, nil)

// Update
_, err := database.Update(ctx, "users", map[string]any{
    "name": "Bob",
}, Where("id", "=", 1), nil)

// Delete
_, err := database.Delete(ctx, "users", Where("id", "=", 1), nil)

// Check pool stats
stats, err := database.PoolStats()
```

## Creating Your Own Plugin

### Template Structure

```go
package myplugin

import (
    "context"
    "fmt"
    "tounilab.com/fabric/db/v1"
    "tounilab.com/fabric/db/v1/plugin"
)

// Config implements db.DBConfig
type Config struct {
    // Your database-specific fields
}

func (c *Config) Driver() string {
    return "myplugin"
}

func (c *Config) DSN() string {
    // Return your database connection string
    return fmt.Sprintf("...")
}

// Factory implements plugin.DriverFactory
type Factory struct{}

func (f *Factory) Name() string {
    return "myplugin"
}

func (f *Factory) Create(ctx context.Context, cfg interface{}) (interface{}, error) {
    myCfg, ok := cfg.(*Config)
    if !ok {
        return nil, fmt.Errorf("expected *Config, got %T", cfg)
    }

    // Create and return your driver
    // Option 1: Wrap an existing driver
    // return db.PostgresCfgToDB(pgConfig)

    // Option 2: Implement your own
    // driver := &MyDriver{...}
    // return driver, nil
}

// Auto-register in init()
func init() {
    plugin.MustRegister(&Factory{})
}
```

### Plugin Patterns

#### Pattern 1: Wrapping Existing Drivers (Recommended for Compatible Databases)

```go
// Use PostgreSQL driver for wire-compatible databases (CockroachDB, Postgres-compatible services)
func (f *Factory) Create(ctx context.Context, cfg interface{}) (interface{}, error) {
    myCfg := cfg.(*Config)
    pgCfg := convertToPostgresConfig(myCfg)
    return db.PostgresCfgToDB(pgCfg)
}
```

**Available Wrappers:**

- `db.MySQLCfgToDB(cfg *db.MySQLConfig)`
- `db.PostgresCfgToDB(cfg *db.PostgresConfig)`
- `db.SQLiteCfgToDB(cfg *db.SQLiteConfig)`
- `db.MSSQLCfgToDB(cfg *db.MSSQLConfig)`

#### Pattern 2: Implementing a New Driver

Implement the `db.DB` interface:

```go
type MyDriver struct {
    // connection pool, logger, etc.
}

func (d *MyDriver) Ping(ctx context.Context) error { ... }
func (d *MyDriver) Get(ctx context.Context, table string, ...) ([]map[string]any, error) { ... }
func (d *MyDriver) Insert(ctx context.Context, table string, ...) (*db.Result, error) { ... }
func (d *MyDriver) Update(ctx context.Context, table string, ...) (*db.Result, error) { ... }
func (d *MyDriver) Delete(ctx context.Context, table string, ...) (*db.Result, error) { ... }
func (d *MyDriver) Close() error { ... }
func (d *MyDriver) PoolStats() (*db.PoolStats, error) { ... }
```

#### Pattern 3: Middleware/Wrapper Driver

Decorate an existing driver with additional functionality:

```go
type LoggingDriver struct {
    inner db.DB  // Wrapped driver
}

func (d *LoggingDriver) Get(ctx context.Context, table string, ...) ([]map[string]any, error) {
    log.Printf("Getting from %s", table)
    result, err := d.inner.Get(ctx, table, ...)
    log.Printf("Got %d rows", len(result))
    return result, err
}

// Delegate other methods to inner driver
func (d *LoggingDriver) Insert(ctx context.Context, ...) (*db.Result, error) {
    return d.inner.Insert(ctx, ...)
}
```

## Registry API Reference

The `plugin` package provides these functions:

### Register(factory DriverFactory) error

Register a driver factory (prevents duplicates).

```go
err := plugin.Register(&MyFactory{})
if err != nil {
    // Already registered
}
```

### MustRegister(factory DriverFactory)

Register a driver factory (panics on error). Use in `init()`.

```go
func init() {
    plugin.MustRegister(&MyFactory{})
}
```

### Get(driverName string) (DriverFactory, bool)

Look up a driver factory by name.

```go
factory, exists := plugin.Get("cockroachdb")
if !exists {
    return fmt.Errorf("driver not found")
}
```

### List() []string

List all registered driver names.

```go
names := plugin.List()
// ["cockroachdb", "mysql", "postgres", ...]
```

### Unregister(driverName string) error

Remove a driver registration (mainly for testing).

```go
err := plugin.Unregister("cockroachdb")
```

### Clear()

Remove all driver registrations (mainly for testing).

```go
plugin.Clear()
```

## Running the Example

### Prerequisites

- Go 1.21+
- CockroachDB running on `localhost:26257` (or update config in main.go)
- Go module set up for your workspace

### Steps

1. **Setup module path**

   Update the import paths in `main.go` to match your module:

   ```go
   import _ "your-actual-module/examples/plugin-example/cockroachdb"
   ```

2. **Run the example**

   ```bash
   cd examples/plugin-example
   go run main.go
   ```

3. **Expected output**

   ```
   Registered drivers: [cockroachdb]
   ✅ Connected to CockroachDB
   Inserted 1 rows, ID: 1
   Found 1 users:
     map[email:alice@example.com id:1 name:Alice]
   Pool stats: Open=1, InUse=0, Idle=1
   ✅ Plugin example complete
   ```

## Thread Safety

The plugin registry is thread-safe using `sync.RWMutex`:

- Multiple goroutines can safely call `Get()` and `List()` concurrently
- Registration should happen during `init()` (single-threaded)
- Safe to call throughout application lifetime

## Best Practices

1. **Module Organization**: Place plugins in separate packages
2. **Error Handling**: Return clear error messages from `Create()`
3. **Type Assertions**: Always check type assertions have `.(*Type)` with OK check
4. **Config Validation**: Validate config in `Create()` method
5. **Auto-Registration**: Use `init()` with `MustRegister()` for auto-loading
6. **Documentation**: Document your driver's config requirements and capabilities
7. **Testing**: Test your driver independently from the main application

## See Also

- [Plugin System Documentation](../../README.md#plugin-system) in main README
- [Plugin Registry](../../db/v1/plugin/registry.go) implementation
- [db.NewDB()](../../db/v1/db.go) integration point
