# DBManager - Multi-Database Management

The `DBManager` is an advanced component of db-connector that manages multiple database connections simultaneously with intelligent routing, automatic failover, and async operations.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
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

| Concept         | Description                                                               |
| --------------- | ------------------------------------------------------------------------- |
| **Entry**       | A single database connection configuration (name, type, priority, config) |
| **Type**        | Either `read-only` (for read queries) or `read-write` (for read+write)    |
| **Priority**    | Integer (0-infinite) controlling selection order; higher = preferred      |
| **Worker Pool** | Goroutines handling database queries for an entry (separate read/write)   |
| **Queue**       | Bounded channel holding pending queries (backpressure mechanism)          |
| **Async API**   | Channel-based query responses; non-blocking, fire-and-forget pattern      |

## Architecture

### Three-Tier Configuration Resolution

DBManager uses hierarchical configuration with automatic fallback:

```
Query Parameters
    ↓
Entry-Level Overrides (e.g., entry.readQueueSize)
    ↓
Global Defaults (e.g., readQueueSize)
    ↓
Built-in Constants (DefaultReadQueueSize = 1000)
```

### Selection Strategy

```
┌─────────────────────────────────────┐
│  Incoming Query (Get, Insert, etc)  │
└──────────────┬──────────────────────┘
               ↓
          ┌─────────────────────────────────────┐
          │ readOnlyEntry() / readWriteEntry()  │
          │ (Priority-based selection)          │
          └──────────┬────────────────────────┘
                     ↓
            Find maximum priority
                     ↓
         Collect all entries with max priority
                     ↓
      Single entry:          Multiple entries:
      Return directly    →   Round-robin distribute
                             (atomic counter)
                     ↓
          ┌──────────────────────┐
          │ Select DBEntry       │
          └──────────┬─────────────┘
                     ↓
          ┌──────────────────────┐
          │ Route to queue       │
          │ (read or write)      │
          └──────────┬─────────────┘
                     ↓
          ┌──────────────────────┐
          │ Worker processes     │
          │ executes query       │
          └──────────┬─────────────┘
                     ↓
          ┌──────────────────────┐
          │ Send response via    │
          │ channel             │
          └──────────────────────┘
```

### Worker Pool Pattern

Each database entry maintains separate worker pools:

```
Database Entry
├── Read-Only Workers (e.g., 4 goroutines)
│   └── Read Queue (bounded, e.g., 1000 items)
│       └── Processes: Get, GetByID, Query, Exec (read-only)
└── Read-Write Workers (e.g., 4 goroutines)
    └── Write Queue (bounded, e.g., 1000 items)
        └── Processes: Insert, Update, Delete, Exec (write)
```

**Benefits:**

- ✅ Non-blocking: Workers process queries concurrently
- ✅ Backpressure: Bounded queues prevent memory exhaustion
- ✅ Isolation: Read/write operations don't compete for resources
- ✅ Scalability: Configurable worker counts per database

## Configuration

### File Formats

DBManager supports YAML, JSON, and TOML configuration files.

### Schema

```yaml
# Global defaults applied to all entries
writeQueueSize: 1000 # Default write queue size
readQueueSize: 1000 # Default read queue size
writeWorkers: 4 # Default write workers per entry
readWorkers: 4 # Default read workers per entry
healthInterval: 30s # Health check interval

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

| Type         | Purpose                     | Supported Operations                                                       |
| ------------ | --------------------------- | -------------------------------------------------------------------------- |
| `read-only`  | Read-only replica databases | Get, GetByID, Query, GetRaw, GetByIDRaw, QueryRaw, Exec (read-only)        |
| `read-write` | Primary database            | All operations: Get, GetByID, Insert, Inserts, Update, Delete, Query, Exec |

### Priority Rules

- **Higher number = higher priority** (100 > 50 > 10)
- **Default = 0** (if not specified)
- **Multiple same priority** = load-balanced with round-robin
- **Automatic failover** = queries fall back to next priority tier if preferred tier unavailable

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

Result: Write queries go to primary-writer; if unavailable, alternate between replica-1/replica-2; if those unavailable, use analytics.

## Priority-Based Selection

### How Selection Works

**Example 1: Single Priority**

```yaml
entries:
  - name: primary
    priority: 100
  - name: secondary
    priority: 50
```

→ All queries always go to `primary` until it fails

**Example 2: Load Balancing**

```yaml
entries:
  - name: replica-us-east
    priority: 50
  - name: replica-us-west
    priority: 50
```

→ Queries are round-robin distributed between both replicas

**Example 3: Tiered Failover**

```yaml
entries:
  - name: primary-eu-central
    priority: 100
  - name: replica-eu-west
    priority: 50
  - name: replica-us-east
    priority: 10
```

→ Primary used first → EU replica if primary fails → US replica as last resort

### Thread-Safe Round-Robin

Within a priority tier, queries are distributed using atomic counters (no locking):

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

**Note:** Order within priority tier is non-deterministic (Go map iteration), but each query gets dispatched fairly.

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
    dm, err := v1.NewDBManager("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Start workers and health checks
    if err := dm.Start(ctx); err != nil {
        log.Fatal(err)
    }

    // Always cleanup
    defer dm.Stop(ctx)

    // Now ready to query
    exampleUsage(ctx, dm)
}

func exampleUsage(ctx context.Context, dm *v1.DBManager) {
    // Fire async query - returns immediately
    respCh := dm.Get(ctx, "", "users", []string{"id", "name"}, nil, nil, nil)

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

Fetch multiple rows with optional conditions and joins. Automatically selects read-only entry with highest priority.

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
    RawData   *db.RowsAdapter   // For GetRaw/QueryRaw (must close)
    ExecData  *db.ExecResult    // For INSERT/UPDATE/DELETE
    Error     error             // Any error during execution
}
```

### Lifecycle Methods

```go
// Create manager from config file
dm, err := v1.NewDBManager(configPath)

// Start workers and health checks
err := dm.Start(ctx)

// Stop workers and close connections
err := dm.Stop(ctx)
```

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

- Use 0-100+ scale (e.g., 100, 50, 10) for clarity
- Document your priority strategy in code comments
- Use consistent priority values across environments

❌ **DON'T:**

- Use the same priority for primary and replicas
- Change priorities without understanding the impact
- Use negative priorities

### 3. Resource Allocation

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
- Distinguish between permanent errors (logs) and transient errors (retry)
- Set timeouts on channel reads to avoid indefinite hangs

❌ **DON'T:**

- Ignore errors silently
- Block forever on channel reads
- Assume queries will always succeed

### 5. Async Operations

✅ **DO:**

- Use goroutines for I/O-bound processing
- Implement backoff for retries
- Use `context.WithTimeout()` for operations

❌ **DON'T:**

- Block on every response channel (defeats async benefit)
- Fire unlimited concurrent queries (will hit backpressure)
- Ignore context cancellation

### 6. Testing

✅ **DO:**

- Test with multiple database entries
- Test priority selection with entry failures
- Test queue backpressure scenarios

❌ **DON'T:**

- Test only happy path
- Assume all entries stay up forever
- Skip load testing before production

## Troubleshooting

### Queries Stuck/Slow

**Symptom:** Channel receive hangs or is very slow.

**Causes:**

1. Queue full (backpressure activated) → increase `writeQueueSize`/`readQueueSize`
2. Workers overloaded → increase `writeWorkers`/`readWorkers`
3. Database connection slow → increase `MaxOpenConns` in database config

**Solution:**

```yaml
writeQueueSize: 5000 # Increase queue
readQueueSize: 5000
writeWorkers: 8 # Add workers
readWorkers: 16
```

### High Memory Usage

**Symptom:** Memory grows continuously.

**Causes:**

1. Queue sizes too large
2. Unclosed `RowsAdapter` from `GetRaw()`/`QueryRaw()`
3. Memory leak in worker goroutines

**Solution:**

- Reduce queue sizes
- Always `defer rowsAdapter.Close()`
- Check logs for goroutine leaks

### Queries Always Route to Same Database

**Symptom:** Only one database receives queries.

**Causes:**

1. Multiple entries have different priorities (correct behavior)
2. High-priority entry is unavailable but health check hasn't detected it yet

**Solution:**

- Verify priority configuration
- Check health check interval is reasonable
- Look at logs to see which entry is selected

### Panic: "all databases down"

**Symptom:** Panic when no entries are available.

**Causes:**

1. Misconfigured entries
2. All databases are unreachable
3. Configuration file not found

**Solution:**

- Verify config file syntax: `go run examples/manager-example/*.go`
- Check database connectivity manually
- Review logs for specific errors

## See Also

- [db-connector README](../README.md)
- [ERROR_HANDLING.md](./ERROR_HANDLING.md)
- [examples/manager-example/](../examples/manager-example/)
