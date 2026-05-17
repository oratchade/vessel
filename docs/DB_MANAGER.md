# DBManager - Multi-Database Management

The `DBManager` is an advanced component of fabric that manages multiple
database connections simultaneously with intelligent routing, automatic
failover, and async operations.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Health Monitoring](#health-monitoring)
- [Configuration](#configuration)
- [Priority-Based Selection](#priority-based-selection)
- [Usage](#usage)
- [API Reference](#api-reference)
- [Configuration Examples](#configuration-examples)
- [Best Practices](#best-practices)

## Overview

### When to Use DBManager

Use DBManager when you need:

- **Multi-Database Support** - Query multiple databases (e.g., primary + replicas)
- **Read/Write Separation** - Direct reads to replicas, writes to primary
- **Geographic Distribution** - Route queries to nearest database
- **Load Balancing** - Distribute queries across multiple databases
- **Application-Level Sharding** - Manage multiple sharded databases
- **High Availability** - Automatic failover to secondary databases

### Key Concepts

- **Entry**: Single database connection configuration (name, type,
  priority, config)
- **Type**: Either `read-only` (read queries) or `read-write`
  (read+write)
- **Priority**: Integer (0-infinite) controlling selection order; higher
  = preferred
- **Health**: Periodic `Ping()` checks; marks database unhealthy after 5
  consecutive failures
- **Worker Pool**: Goroutines handling database queries per entry
  (separate read/write)
- **Queue**: Bounded channel holding pending queries (backpressure)
- **Async API**: Channel-based query responses; non-blocking, fire-and-
  forget pattern

## Architecture

### Three-Tier Configuration Resolution

DBManager uses hierarchical configuration with automatic fallback:

```text
Query Parameters
    ↓
Entry-Level Overrides (e.g., entry.readQueueSize)
    ↓
Global Defaults (e.g., readQueueSize)
    ↓
Built-in Constants (DefaultReadQueueSize = 1000)
```

### Selection Strategy

**DBManager uses a health-first selection strategy with intelligent fallback:**

```text
        ┌─────────────────────────────────────┐
        │  Incoming Query (Get, Insert, etc)  │
        └──────────────┬──────────────────────┘
                       ↓
        ┌─────────────────────────────────────┐
        │ readOnlyEntry() / readWriteEntry()  │
        │ (Health-first routing)              │
        └──────────┬──────────────────────────┘
                   ↓
        ┌───────────────────────────────┐
        │ Try HEALTHY entries with      │
        │ highest priority              │
        └───────────┬───────────────────┘
                    ↓
        Found healthy entry? YES → Select using round-robin
                    ↓
            NO (all unhealthy)
                    ↓
        ┌───────────────────────────────┐
        │ Fallback to ALL entries       │
        │ by highest priority           │
        │ (graceful degradation)        │
        └───────────┬───────────────────┘
                    ↓
      Single entry:          Multiple entries:
      Return directly    →   Round-robin distribute
                             (atomic counter)
                    ↓
        ┌──────────────────────┐
        │ Select DBEntry       │
        └──────────┬───────────┘
                   ↓
        ┌──────────────────────┐
        │ Route to queue       │
        │ (read or write)      │
        └──────────┬───────────┘
                   ↓
        ┌──────────────────────┐
        │ Worker processes     │
        │ executes query       │
        └──────────┬───────────┘
                   ↓
        ┌──────────────────────┐
        │ Send response via    │
        │ channel              │
        └──────────────────────┘
```

**Key Benefits:**

- ✅ Automatically avoids failed databases
- ✅ Reduces latency by routing to responsive connections
- ✅ Gracefully degrades when unhealthy databases exist
- ✅ Transparent to callers (no API changes)

### Worker Pool Pattern

Each database entry maintains separate worker pools:

```text
Database Entry
├── Read-Only Workers (e.g., 4 goroutines)
│   └── Read Queue (bounded, e.g., 1000 items)
│       └── Processes: Get, GetByID, Query, Exec (read-only)
└── Read-Write Workers (e.g., 4 goroutines)
    └── Write Queue (bounded, e.g., 1000 items)
        └── Processes: Insert, Inserts, Update, Delete, Exec (write)
```

**Benefits:**

- ✅ Non-blocking: Workers process queries concurrently
- ✅ Backpressure: Bounded queues prevent memory exhaustion
- ✅ Isolation: Read/write operations don't compete for resources
- ✅ Scalability: Configurable worker counts per database
- ✅ Optional batching: Compatible `InsertAsync` requests can flush as bulk writes

### Insert Coalescing

Write batching is opt-in. When enabled, each write worker can collect compatible
`InsertAsync` requests and flush them as one `Inserts` call. Explicit
`InsertsAsync` calls already contain caller-owned bulk data and execute
directly.

Requests are compatible when they target the same worker, table, query options,
and column set. A pending insert batch flushes when it reaches
`writeBatchMaxRows`, waits `writeBatchMaxDelay`, sees an incompatible write, or
the manager stops.

Batch errors are returned to every original caller. When database rows affected
matches the batch size, each caller receives `RowsAffected: 1`; otherwise each
caller receives the aggregate rows affected value because Fabric cannot infer
per-row effects from a database-level bulk result.

## Health Monitoring

### How Health Checking Works

Each database entry runs a periodic health check goroutine that:

1. **Checks connectivity** - Executes `Ping()` at configured interval
   (default: 30s)
2. **Tracks failures** - Increments failure counter on errors
3. **Marks unhealthy** - Database marked unhealthy after 5 consecutive
   failures
4. **Recovery** - Resets to healthy immediately on next successful `Ping()`

**Health Status:** Stored with atomic operations (thread-safe,
lock-free)

### Health Check Example

Given this configuration:

```yaml
healthInterval: 30s # Check every 30 seconds

entries:
  - name: primary
    priority: 100
  - name: replica
    priority: 50
```

**Scenario:** Primary database becomes unavailable at time T

- **T + 0s:** First failure detected → failureCount = 1
- **T + 30s:** Second failure → failureCount = 2
- **T + 60s:** Third failure → failureCount = 3
- **T + 90s:** Fourth failure → failureCount = 4
- **T + 120s:** Fifth failure → failureCount = 5 → **Mark UNHEALTHY**

**Routing Before T+120s:** Queries use primary database (highest priority)

**Routing After T+120s:** Queries automatically route to replica
(healthy, next priority)

**Recovery:** When primary recovers and `Ping()` succeeds → immediately
returns to HEALTHY

### Configuration

Health check interval is configurable globally and per-entry:

```yaml
# Global default (all entries)
healthInterval: 30s

entries:
  - name: primary
    healthInterval: 10s # More frequent checks for critical database

  - name: replica
    healthInterval: 60s # Less frequent for non-critical
```

### Health Status Visibility

Access current health status programmatically:

```go
entry := dm.readOnlyEntries["primary"]
if entry.Health() {
    log.Println("Primary database is healthy")
} else {
    log.Println("Primary database is unhealthy")
}
```

## Configuration Basics

### File Formats

DBManager supports YAML, JSON, and TOML configuration files.

### Variables Expansion

Configuration files support optional environment variable expansion using
the `${VAR}` and `${VAR:default}` syntax. Variable expansion is **opt-in
and disabled by default** for security.

#### Enabling Variables Expansion

Pass `EnvOption`s to `NewDBManager()`:

```go
dm, err := v1.NewDBManager(ctx, "config.yaml", logger,
    v1.WithEnvVars(map[string]string{
        "DB_HOST":     "localhost",
        "DB_PASSWORD": os.Getenv("SECURE_PASSWORD"),
    }),
)
```

#### Available EnvOptions

**WithEnvVars** - Explicit map (highest priority):

```go
v1.WithEnvVars(map[string]string{
    "DB_HOST":     "prod-db.example.com",
    "DB_PASSWORD": "secret123",
})
```

**WithEnvPrefix** - Filter process environment by prefix:

```go
v1.WithEnvPrefix("DB_", "FABRIC_")
// Expands ${DB_HOST}, ${DB_PORT}, ${FABRIC_NAME}, etc.
// Other vars like ${SECRET_KEY} are protected (not expanded)
```

**WithEnvFile** - Load from .env file:

```go
v1.WithEnvFile(".env")
// Reads key=value pairs, supports comments and quoted values
```

**Combine Options** - Multiple options work together:

```go
dm, err := v1.NewDBManager(ctx, "config.yaml", logger,
    v1.WithEnvVars(map[string]string{"DB_HOST": "localhost"}),
    v1.WithEnvFile(".env"),
    v1.WithEnvPrefix("DB_", "FABRIC_"),
)
// Priority: explicit vars > file vars > prefix env
```

#### Variable Syntax

**Simple variable:**

```yaml
host: ${DB_HOST}
```

**With default value:**

```yaml
port: ${DB_PORT:5432}
```

**Empty default:**

```yaml
password: ${DB_PASSWORD:}
```

**Bare $VAR is NOT expanded:**

```yaml
# This is left as-is
example: $DB_HOST
```

#### Variable Resolution Behavior

| Syntax           | Variable Found    | Variable Missing                     |
| ---------------- | ----------------- | ------------------------------------ |
| `${VAR}`         | Replaced w/ value | Left as literal `${VAR}` (fail-loud) |
| `${VAR:default}` | Replaced w/ value | Replaced with `default`              |
| `${VAR:}`        | Replaced w/ value | Replaced with empty string           |

#### .env File Format

```env
# Comments start with #
DB_HOST=localhost
DB_PORT=5432

# Quoted values are unquoted automatically
DB_PASSWORD="my-secret-password"
DB_USER='readonly-user'
```

#### Configuration Example with Variables

```yaml
# config.yaml
writeQueueSize: ${WRITE_QUEUE:1000}
readQueueSize: ${READ_QUEUE:2000}
healthInterval: ${HEALTH_CHECK:30s}

entries:
  - name: primary
    type: read-write
    priority: 100
    database: postgres
    config:
      host: ${DB_PRIMARY_HOST}
      port: ${DB_PRIMARY_PORT:5432}
      user: ${DB_USER}
      password: ${DB_PASSWORD}
      database: myapp
      sslmode: require

  - name: replica
    type: read-only
    priority: 50
    database: postgres
    config:
      host: ${DB_REPLICA_HOST}
      port: ${DB_REPLICA_PORT:5432}
      user: ${DB_USER}
      password: ${DB_PASSWORD:} # Empty password fallback
      database: myapp
      sslmode: require
```

Usage:

```go
dm, err := v1.NewDBManager(ctx, "config.yaml", logger,
    v1.WithEnvVars(map[string]string{
        "DB_PRIMARY_HOST": "primary.example.com",
        "DB_REPLICA_HOST": "replica.example.com",
        "DB_USER":         "app",
        "DB_PASSWORD":     os.Getenv("VAULT_DB_PASSWORD"),
    }),
)
```

#### Security Best Practices

✅ **DO:**

- Use `WithEnvVars()` to pass sensitive values
- Use `WithEnvPrefix()` to restrict which process env vars are accessible
- Load secrets from vaults via `os.Getenv()` and pass to `WithEnvVars()`
- Document which variables are expected in config files

❌ **DON'T:**

- Enable `WithEnvPrefix()` without prefixes (exposes all env vars)
- Leave defaults empty for sensitive values like passwords
- Hardcode secrets in config files
- Enable variables expansion if not needed (keep default disabled)
- Use `WithEnvFile()` for production secrets (use `WithEnvVars()` instead)

#### Missing Variables

- **With no option configured:** Variables are left unexpanded (default, safe)
- **With option configured, var not found, no default:**
  Variable is left as literal `${VAR}` (fail-loud, helps debug)
- **File missing:** `WithEnvFile()` silently ignores missing files,
  defaults still work

#### .env File Example

```bash
# .env (committed to VCS, safe to share)
DB_PRIMARY_HOST=primary.example.com
DB_REPLICA_HOST=replica.example.com
DB_USER=app

# Production secrets (NOT committed, loaded from secure storage)
# DB_PASSWORD=actual-secret-from-vault
```

Load only safe vars from file:

```go
dm, err := v1.NewDBManager(ctx, "config.yaml", logger,
    v1.WithEnvFile(".env"),
    v1.WithEnvVars(map[string]string{
        "DB_PASSWORD": os.Getenv("VAULT_SECRET"),
    }),
)
```

### Schema

```yaml
# Global defaults applied to all entries
writeQueueSize: 1000 # Default write queue size
readQueueSize: 1000 # Default read queue size
writeWorkers: 4 # Default write workers per entry
readWorkers: 4 # Default read workers per entry
healthInterval: 30s # Health check interval
writeBatchingEnabled: false # Opt-in automatic InsertAsync batching
writeBatchMaxRows: 100 # Flush once this many compatible inserts accumulate
writeBatchMaxDelay: 5ms # Flush after the first insert waits this long

# Database entries
entries:
  - name: primary-db # Unique identifier
    type: read-write # Type: read-only or read-write
    priority: 100 # Selection priority (higher preferred)
    database: postgres # Driver type: postgres, mysql, sqlite, mssql

    # Per-entry overrides (optional)
    writeQueueSize: 500 # Override global default
    readQueueSize: 2000 # Override global default
    writeWorkers: 2 # Override global default
    readWorkers: 8 # Override global default
    healthInterval: 60s # Override global default
    writeBatchingEnabled: true # Override global default
    writeBatchMaxRows: 250 # Override global default
    writeBatchMaxDelay: 10ms # Override global default

    # Database-specific config
    config:
      user: postgres
      password: secure_password
      host: primary.example.com
      port: 5432
      database: myapp
      sslmode: require
```

### Entry Type

**read-only**: Read-only replica databases

- Supported operations: Get, GetByID, Query, GetRaw, GetByIDRaw, QueryRaw,
  Exec (read-only)

**read-write**: Primary database

- Supported operations: Get, GetByID, Insert, Inserts, Update, Delete,
  Query, Exec

### Priority Rules

- **Higher number = higher priority** (100 > 50 > 10)
- **Default = 0** (if not specified)
- **Multiple same priority** = load-balanced with round-robin
- **Automatic failover** = queries fall back to next priority tier if
  preferred tier unavailable

Example:

```yaml
entries:
  - name: primary-writer
    priority: 100 # Always preferred for writes

  - name: replica-1
    priority: 50 # Secondary; used if primary unavailable

  - name: replica-2
    priority: 50 # Load-balanced with replica-1

  - name: analytics
    priority: 10 # Last resort
```

Result: Write queries go to primary-writer; if unavailable, alternate
between replica-1/replica-2; if those unavailable, use analytics.

## Priority-Based Selection

### How Selection Works

DBManager selects entries using a **health-aware priority system**:

1. **Health-First:** Filters for healthy entries only (those with
   successful recent `Ping()`)
2. **Priority-Based:** Among healthy entries, selects the group with
   highest priority
3. **Load-Balanced:** Within same priority, distributes using round-robin
   with atomic counter
4. **Graceful Fallback:** If no healthy entries exist, uses all entries
   ranked by priority

**Selection Hierarchy:**

```text
HEALTHY + HIGHEST PRIORITY + ROUND-ROBIN
    ↓ (if no healthy with highest priority)
HEALTHY + NEXT PRIORITY + ROUND-ROBIN
    ↓ (if no healthy entries at all)
UNHEALTHY + HIGHEST PRIORITY + ROUND-ROBIN
    ↓ (last resort)
nil (no entries available)
```

### Example 1: Single Priority (Health-Aware Failover)

```yaml
entries:
  - name: primary
    priority: 100
  - name: secondary
    priority: 50
```

### Example: Single Priority (Health-Aware Failover)

```yaml
entries:
  - name: primary
    priority: 100
  - name: secondary
    priority: 50
```

**Routing Behavior:**

- **While primary is healthy:** All queries → primary (highest
  priority)
- **When primary becomes unhealthy:** Automatically route → secondary
  (highest healthy priority)
- **When primary recovers:** Automatically resume → primary (highest
  priority, now healthy)

**No code changes required** - health-aware routing is automatic and transparent.

### Example 2: Load Balancing (Health-Aware)

```yaml
entries:
  - name: replica-us-east
    priority: 50
  - name: replica-us-west
    priority: 50
```

**Routing Behavior:**

- **Both healthy:** Load-balanced round-robin between both replicas
- **One unhealthy:** All queries → remaining healthy replica
- **Both unhealthy:** Graceful fallback - queries continue using both
  (with potential errors logged)

**Benefits:** Automatic handling of replica failures without
reconfiguration.

### Example 3: Tiered Failover (Health-Aware)

```yaml
entries:
  - name: primary-eu-central
    priority: 100
  - name: replica-eu-west
    priority: 50
  - name: replica-us-east
    priority: 10
```

**Routing Behavior:**

- **Primary healthy:** Primary receives all queries
- **Primary unhealthy:** EU replica (priority 50) receives all queries
- **Primary + EU unhealthy:** US replica (priority 10) receives all
  queries
- **All unhealthy:** Fallback to highest priority (primary) - graceful
  degradation

**Advantages:** Automatic cascading failover based on health status,
with smart fallback to ensure service availability.

### Thread-Safe Round-Robin

Within a priority tier, queries are distributed using atomic counters
(no locking):

```go
// Two replicas with same priority
idx := atomicCounter.Next() % 2
return replicas[idx]
```

**Advantages:**

- ✅ **Lock-free** - No mutexes, minimal contention
- ✅ **High performance** - Scales to 1000s QPS
- ✅ **Predictable overhead** - Atomic operation only
- ✅ **Deterministic** - Consistent distribution

**Note:** Order within priority tier is non-deterministic (Go map iteration),
but each query gets dispatched fairly.

## Usage

### Initialization

```go
package main

import (
    "context"
    "log"
    "tounilab.com/fabric/manager/v1"
)

func main() {
    ctx := context.Background()

    // Load configuration from YAML file
    dm, err := v1.NewDBManager(ctx, "config.yaml", nil)
    if err != nil {
        log.Fatal(err)
    }

    // Start workers and health checks
    dm.Start()

    // Always cleanup
    defer dm.Stop()

    // Now ready to query
    exampleUsage(ctx, dm)
}

func exampleUsage(ctx context.Context, dm *v1.DBManager) {
    // Fire async query - returns immediately
    respCh, err := dm.GetAsync(ctx, "users", []string{"id", "name"}, nil, nil, nil)
    if err != nil {
        log.Printf("Queue error: %v\n", err)
        return
    }

    // Handle response asynchronously
    go func() {
        for resp := range respCh {
            if resp.Error != nil {
                log.Printf("Query error: %v\n", resp.Error)
                return
            }
            for _, row := range resp.Data {
                log.Printf("User: %v\n", row)
            }
        }
    }()
}
```

### Async Pattern

All DBManager queries return a channel immediately (non-blocking):

```go
// Method signature
func (dm *DBManager) Get(
    ctx context.Context,
    dbName string,                           // Unused: auto-routed by priority
    table string,
    columns []string,
    joins []condition.Join,
    cond condition.Condition,
    opts *options.QueryOptions,
) chan<- *QueryResponse

// Usage
respCh := dm.Get(ctx, "", "users", []string{"id", "name"}, nil, nil, nil)

// Receive response (blocking)
resp := <-respCh

// Or handle asynchronously
go func() {
    resp := <-respCh
    if resp.Error != nil {
        handleError(resp.Error)
    } else {
        processData(resp.Data)
    }
}()
```

### Supported Operations

```go
// Read operations (routed to read-only entries first, then read-write)
respCh := dm.Get(ctx, "", "users", cols, joins, cond, opts)
respCh := dm.GetByID(ctx, "", "users", id, cols, opts)
respCh := dm.GetRaw(ctx, "", "users", cols, joins, cond, opts)
respCh := dm.GetByIDRaw(ctx, "", "users", id, cols, opts)
respCh := dm.Query(ctx, "", sql, params...)
respCh := dm.QueryRaw(ctx, "", sql, params...)

// Write operations (routed to read-write entries only)
respCh := dm.Insert(ctx, "", "users", data, opts)
respCh := dm.Inserts(ctx, "", "users", bulkData, opts)
respCh := dm.Update(ctx, "", "users", updates, cond, opts)
respCh := dm.Delete(ctx, "", "users", cond, opts)
respCh := dm.Exec(ctx, "", sql, params...)
```

### Error Handling

```go
respCh := dm.Insert(ctx, "", "users", data, nil)

resp := <-respCh
if resp.Error != nil {
    // Handle specific errors
    if errors.Is(resp.Error, dberror.ErrDuplicateKey) {
        log.Println("User email already exists")
    } else if errors.Is(resp.Error, dberror.ErrConnectionFailed) {
        log.Println("Database connection lost")
    } else {
        log.Printf("Unexpected error: %v\n", resp.Error)
    }
    return
}

// Process successful results
for _, row := range resp.Data {
    log.Printf("Inserted row: %v\n", row)
}
```

## API Reference

### DBManager Methods

#### Get

```go
func (dm *DBManager) Get(
    ctx context.Context,
    dbName string,
    table string,
    columns []string,
    joins []condition.Join,
    cond condition.Condition,
    opts *options.QueryOptions,
) chan<- *QueryResponse
```

Fetch multiple rows with optional conditions and joins.
Automatically selects read-only entry with highest priority.

#### GetByID

```go
func (dm *DBManager) GetByID(
    ctx context.Context,
    dbName string,
    table string,
    id any,
    columns []string,
    opts *options.QueryOptions,
) chan<- *QueryResponse
```

Fetch single row by primary key.

#### Insert

```go
func (dm *DBManager) Insert(
    ctx context.Context,
    dbName string,
    table string,
    data map[string]any,
    opts *options.QueryOptions,
) chan<- *QueryResponse
```

Insert single row. Routes to read-write entry with highest priority.

#### Inserts

```go
func (dm *DBManager) Inserts(
    ctx context.Context,
    dbName string,
    table string,
    data []map[string]any,
    opts *options.QueryOptions,
) chan<- *QueryResponse
```

Bulk insert multiple rows in single query.

#### Update

```go
func (dm *DBManager) Update(
    ctx context.Context,
    dbName string,
    table string,
    updates map[string]any,
    cond condition.Condition,
    opts *options.QueryOptions,
) chan<- *QueryResponse
```

Update rows matching condition.

#### Delete

```go
func (dm *DBManager) Delete(
    ctx context.Context,
    dbName string,
    table string,
    cond condition.Condition,
    opts *options.QueryOptions,
) chan<- *QueryResponse
```

Delete rows matching condition.

#### Query

```go
func (dm *DBManager) Query(
    ctx context.Context,
    dbName string,
    query string,
    params ...any,
) chan<- *QueryResponse
```

Execute custom SQL with parameterized arguments.

#### Exec

```go
func (dm *DBManager) Exec(
    ctx context.Context,
    dbName string,
    query string,
    params ...any,
) chan<- *QueryResponse
```

Execute DDL/DML without returning rows.

### QueryResponse

```go
type QueryResponse struct {
    RequestID string            // Unique request identifier
    Data      []map[string]any  // Rows (only for SELECT queries)
    RawData   *db.RowsAdapter   // For GetRaw/QueryRaw - use ScanRowsTo[T], pool, or managed wrapper
    ExecData  *db.ExecResult    // For INSERT/UPDATE/DELETE
    Error     error             // Any error during execution
}
```

**Note on RawData usage:** When GetRaw/QueryRaw is called, use one of these patterns:

1. **ScanRowsTo[T]** (recommended) - Automatic cleanup: `users, _ := db.ScanRowsTo[User](ctx, resp.RawData)`
2. **RowsAdapterPool** (high-throughput) - Explicit pooling for tight loops
3. **ManagedRowsAdapter** (explicit) - Managed cleanup with finalizer fallback

See [Resource Pooling Guide](./RESOURCE_POOLING.md) for comprehensive examples.

### Lifecycle Methods

```go
// Create manager from config file
dm, err := v1.NewDBManager(ctx, configPath, logger)

// Start workers and health checks
dm.Start()

// Stop workers and close connections
dm.Stop()
```

`Start` and `Stop` are idempotent. Async calls before `Start` return
`ErrManagerNotStarted`; calls after shutdown begins return `ErrManagerClosed`.
`Stop` cancels manager-owned worker contexts, waits for workers and health
checks to exit, then closes database connections.

## Configuration Examples

### Example 1: Primary + Read Replicas

```yaml
# Global defaults
readQueueSize: 2000
writeQueueSize: 500
readWorkers: 8
writeWorkers: 2

entries:
  - name: primary
    type: read-write
    priority: 100
    database: postgres
    config:
      host: primary.db.example.com
      user: app
      password: ...
      database: myapp

  - name: replica-1
    type: read-only
    priority: 50
    database: postgres
    config:
      host: replica1.db.example.com
      user: app
      password: ...
      database: myapp

  - name: replica-2
    type: read-only
    priority: 50
    database: postgres
    config:
      host: replica2.db.example.com
      user: app
      password: ...
      database: myapp
```

**Routing:** Writes → primary. Reads → replicas (load-balanced).

### Example 2: Geographic Distribution

```yaml
readQueueSize: 1000
readWorkers: 4

entries:
  - name: database-eu-central
    type: read-only
    priority: 100
    database: postgres
    config:
      host: db-eu-central.example.com
      user: app
      password: ...
      database: myapp

  - name: database-us-east
    type: read-only
    priority: 50
    database: postgres
    config:
      host: db-us-east.example.com
      user: app
      password: ...
      database: myapp

  - name: database-ap-southeast
    type: read-only
    priority: 10
    database: postgres
    config:
      host: db-ap-southeast.example.com
      user: app
      password: ...
      database: myapp
```

**Routing:** Prefer EU (lowest latency) → US → Asia.

### Example 3: Multi-Database Setup

```yaml
entries:
  - name: main-db
    type: read-write
    priority: 100
    database: postgres
    config:
      host: main.example.com
      user: app
      password: ...
      database: myapp

  - name: analytics-db
    type: read-only
    priority: 50
    database: postgres
    config:
      host: analytics.example.com
      user: app_readonly
      password: ...
      database: analytics

  - name: cache-db
    type: read-only
    priority: 50
    database: sqlite
    config:
      filepath: /tmp/cache.db
```

**Routing:** Application queries → main-db. Analytics → analytics-db or cache-db.

## Best Practices

### 1. Configuration Management

✅ **DO:**

- Use environment-specific config files (config-dev.yaml, config-prod.yaml)
- Load config from environment variable: `configPath := os.Getenv("DB_CONFIG")`
- Validate config file path exists before calling `NewDBManager()`

❌ **DON'T:**

- Hardcode database credentials in code
- Mix environment configs in single file
- Load config from untrusted sources

### 2. Priority Design

✅ **DO:**

- Use hierarchical priority structures (100, 50, 10) for clarity
- Document your priority strategy in code comments
- Consider geographic proximity or response latency when setting priorities
- Use health-first selection to avoid unreliable databases automatically

❌ **DON'T:**

- Use the same priority for primary and replicas (defeats failover)
- Change priorities without understanding the impact
- Use negative priorities
- Assume unhealthy databases won't receive queries
  (they will if all healthy are down)

### 3. Health Monitoring

✅ **DO:**

- Set appropriate health check intervals (30-60s for production, 5-10s for development)
- Monitor health status via logs or metrics
- Alert when databases remain unhealthy for extended periods
- Configure different intervals for critical vs non-critical databases

❌ **DON'T:**

- Set health check interval too low (excessive Ping() overhead)
- Ignore unhealthy database alerts
- Assume health checks have zero performance impact (they don't)
- Disable health checks in production (they catch failures quickly)

### Example: Health-Aware Configuration

```yaml
entries:
  - name: primary
    priority: 100
    healthInterval: 10s # Check critical database frequently

  - name: replica
    priority: 50
    healthInterval: 30s # Check replica less frequently
```

### 4. Resource Allocation

✅ **DO:**

- Start with default workers/queues; adjust based on load testing
- Monitor queue depth and adjust queue size if needed
- Set appropriate health check intervals (30-60s typical)

❌ **DON'T:**

- Create too many workers (CPU overhead, thread thrashing)
- Set queue sizes too large (memory exhaustion risk)
- Forget to call `Stop()` on shutdown

### 4. Error Handling

✅ **DO:**

- Always check `resp.Error` before processing `resp.Data`
- Distinguish between permanent errors (logs) and transient errors (retry/failover)
- Set timeouts on channel reads to avoid indefinite hangs
- Monitor and alert on repeated errors from specific databases

❌ **DON'T:**

- Ignore errors silently
- Block forever on channel reads
- Assume queries will always succeed
- Treat all errors identically (some indicate health issues)

### 5. Async Operations

✅ **DO:**

- Use goroutines for I/O-bound processing
- Implement backoff for retries on transient failures
- Use `context.WithTimeout()` for operations with deadlines
- Configure health check intervals based on acceptable failover time

❌ **DON'T:**

- Block on every response channel (defeats async benefit)
- Fire unlimited concurrent queries (will hit backpressure)
- Ignore context cancellation
- Rely on health checks alone for error handling (use proper error checks too)

### 6. Testing

✅ **DO:**

- Test with multiple database entries at different priorities
- Test priority selection with entry failures
- Test queue backpressure scenarios
- **Test health-aware routing** - verify healthy entries are preferred
- **Test graceful degradation** - verify fallback to unhealthy when all are down
- Test automatic recovery when unhealthy entries return to health

❌ **DON'T:**

- Test only happy path
- Assume all entries stay up forever
- Skip load testing before production
- Forget to test failover scenarios
- Test only the first entry in configuration (round-robin matters)

## Troubleshooting

### Queries Stuck/Slow

**Symptom:** Channel receive hangs or is very slow.

**Causes:**

1. Queue full (backpressure activated) → increase `writeQueueSize`/`readQueueSize`
2. Workers overloaded → increase `writeWorkers`/`readWorkers`
3. Database connection slow → increase `MaxOpenConns` in database
   config
4. All databases unhealthy → queries fall back to
   unhealthy entries (slower, more errors)

**Solution:**

```yaml
writeQueueSize: 5000 # Increase queue
readQueueSize: 5000
writeWorkers: 8 # Add workers
readWorkers: 16
healthInterval: 10s # Detect failures faster
```

### Health Check Not Detecting Failures

**Symptom:** Database is down but queries still route to it; takes too long to failover.

**Causes:**

1. `healthInterval` is too long (default 30s, takes up to 150s to mark unhealthy)
2. Database is intermittently failing (needs 5 consecutive failures)
3. Health check network connectivity different from query path

**Solution:**

```yaml
# For critical databases, use faster health checks
entries:
  - name: primary
    priority: 100
    healthInterval: 5s # Fail over within 25 seconds (5 failures × 5s)

  - name: replica
    priority: 50
    healthInterval: 30s # Non-critical, standard interval
```

**Example Health Check Timeline:**

With `healthInterval: 5s`:

- **T + 0s:** Network partition, first `Ping()` fails
- **T + 5s:** Second failure (failureCount = 2)
- **T + 10s:** Third failure (failureCount = 3)
- **T + 15s:** Fourth failure (failureCount = 4)
- **T + 20s:** Fifth failure (failureCount = 5) → **UNHEALTHY**
- **Queries now route to next priority entry** (happened within 25 seconds)

### Queries Route to Unhealthy Database

**Symptom:** Queries fail even though another database is available and healthy.

**Causes:**

1. The unhealthy database has higher priority - attempting graceful degradation
2. All databases are unhealthy (no healthy entries available)
3. Health check shows database as healthy but it's actually slow/failing

**Solution:**

```go
// Check health status
if !entry.Health() {
    log.Printf("Entry %s is unhealthy\n", entry.Name())
}

// Verify routing logic
readOnlyEntry := dm.readOnlyEntry()
log.Printf("Selected entry: %s (healthy: %v)\n", readOnlyEntry.Name(), readOnlyEntry.Health())
```

**If all databases are unhealthy:**

- This is graceful fallback behavior (prevents complete service loss)
- All queries route to highest priority entry
- Investigate why all databases are failing:
  - Network partition?
  - All databases down?
  - Changed credentials?
  - Resource exhaustion?

### Queries Always Route to Same Database

**Symptom:** Only one database receives queries despite multiple configured entries.

**Causes:**

1. Multiple entries have different priorities (correct - highest priority used)
2. Lower-priority entries are all unhealthy (correct - healthy entries
   preferred)
3. All same-priority entries healthy (round-robin should work,
   but may appear like one)

**Solution:**

- Verify all entries are configured with appropriate priorities
- Check health status with `entry.Health()`:

  ```go
  for name, entry := range dm.readOnlyEntries {
      log.Printf("%s: healthy=%v, priority=%d\n", name, entry.Health(), entry.Priority())
  }
  ```

- Confirm round-robin working by monitoring distribution over time
- Add metrics to track queries per entry

### All Databases Are Unhealthy

**Symptom:** Repeated error messages; all database connections failing.

**Causes:**

1. Real infrastructure failure - all databases actually down
2. Network partition - queries can't reach any database
3. Authentication failure - wrong credentials in all entries
4. Firewall/security group blocking all databases

**Troubleshooting Steps:**

1. **Check health status:**

   ```go
   for name, entry := range dm.readOnlyEntries {
       if !entry.Health() {
           log.Printf("%s is unhealthy\n", name)
       }
   }
   ```

2. **Manual connectivity test:**

   ```bash
   psql -h primary.example.com -U user -d database
   psql -h replica.example.com -U user -d database
   ```

3. **Check logs for specific error:**
   - Connection refused → database not running
   - Authentication failed → wrong credentials
   - Network unreachable → firewall/routing issue
   - No such host → DNS issue

4. **Verify health check behavior:**
   - Look for health check error logs
   - Confirm `healthInterval` is set reasonably
   - Manual Ping() test from application host

### Panic: "all databases down" or nil entry returned

**Symptom:** Panic or nil pointer error when trying to use returned entry.

**Causes:**

1. No entries configured (empty configuration)
2. All entries failed to initialize
3. Configuration file not found or invalid

**Solution:**

```go
dm, err := v1.NewDBManager(ctx, "config.yaml", logger)
if err != nil {
    log.Fatalf("Failed to create DBManager: %v", err)
}

dm.Start()

// Verify at least one entry is available
if _, err := dm.Ping(ctx); err != nil {
    log.Fatalf("No healthy entries available: %v", err)
}
```

- Verify config file syntax: `cat config.yaml | yaml lint`
- Check database connectivity before using DBManager
- Review logs for initialization errors

## See Also

- [fabric README](../README.md)
- [ERROR_HANDLING.md](./ERROR_HANDLING.md)
- [examples/manager-example/](../examples/manager-example/)
