# DBManager Examples

Complete working examples demonstrating DBManager features: multi-database
routing, priority-based selection, async operations, and error handling.

## Prerequisites

- Go 1.20+
- PostgreSQL 12+ running locally
- Three PostgreSQL databases on different ports (for replication simulation)

### Setup PostgreSQL for Examples

```bash
# Create three PostgreSQL instances (or use existing)
# Instance 1: localhost:5432 (primary-db)
# Instance 2: localhost:5433 (replica-1)
# Instance 3: localhost:5434 (replica-2)

# Create test database on all three instances
psql -p 5432 -U postgres -c "CREATE DATABASE myapp;"
psql -p 5433 -U postgres -c "CREATE DATABASE myapp;"
psql -p 5434 -U postgres -c "CREATE DATABASE myapp;"

# Create test table on primary
psql -p 5432 -U postgres -d myapp -c "
  CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100) UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
  );
"

# Replicate to other databases (manual for example)
pg_dump -p 5432 -U postgres -d myapp --schema-only
  | psql -p 5433 -U postgres -d myapp
pg_dump -p 5432 -U postgres -d myapp --schema-only
  | psql -p 5434 -U postgres -d myapp
```

### Alternative: Use Docker

```bash
docker-compose up -d   # See docker-compose.yml in parent directory
```

## Configuration File

**`config.yaml`** - Three-tier database configuration:

```yaml
entries:
  - name: primary-db
    type: read-write
    priority: 100
    # Writes always go here

  - name: replica-1
    type: read-only
    priority: 50
    # Reads load-balanced here

  - name: replica-2
    type: read-only
    priority: 50
    # Reads load-balanced here
```

**Custom config:** Copy `config.yaml` and adjust database host/port/credentials.

## Examples

### 1. `basic.go` - Basic CRUD Operations

**What it does:**

- Initialize DBManager from config file
- Insert a user
- Fetch users
- Update a user
- Delete a user
- Demonstrates async channel-based API

**Run:**

```bash
go run basic.go
# or with custom config:
go run basic.go /path/to/config.yaml
```

**Output:**

```text
=== DBManager Basic Example ===

1. Inserting user...
✓ Inserted: ID=1, Rows=1

2. Fetching users...
✓ Found 1 users:
  - ID: 1, Name: Alice, Email: alice@example.com

3. Updating user...
✓ Updated 1 rows

4. Deleting user...
✓ Deleted 1 rows

=== Example Complete ===
```

### 2. `priority_selection.go` - Priority-Based Routing

**What it does:**

- Demonstrates how queries are routed based on priority
- Shows write queries going to primary (priority:100)
- Shows read queries load-balanced across replicas (priority:50 each)
- Fires multiple concurrent queries asynchronously
- Aggregates results from multiple queries

**Run:**

```bash
go run priority_selection.go
```

**Output:**

```text
=== DBManager Priority Selection Example ===

1. Testing priority-based selection:
   - Writes (Insert) → Always goes to priority:100 (primary-db)
   - Reads (Get) → Load-balanced between priority:50 (replica-1, replica-2)

2. Firing 10 concurrent read queries (async):
   Query 1: OK - found X rows
   Query 2: OK - found X rows
   ...
   Results: 10 successful, 0 errors
   Note: All read queries were routed to replicas with priority:50

3. Mixed write and read operations:
   Sending INSERT to primary-db (priority:100)...
   Sending GET #1 (routed via load-balancer to replica-1 or replica-2)...
   ...
```

**Key concepts:**

- Write operations always route to highest-priority read-write entry
- Read operations route to highest-priority read-only entry (or read-write as fallback)
- Same-priority entries are load-balanced using round-robin
- If primary is unavailable, reads still work on replicas

### 3. `error_handling.go` - Error Handling Patterns

**What it does:**

- Detects specific error types (duplicate key, connection failed, etc.)
- Implements retry logic with exponential backoff
- Handles context timeouts
- Shows how to decide if an error is retryable
- Demonstrates graceful degradation

**Run:**

```bash
go run error_handling.go
```

**Output:**

```text
=== DBManager Error Handling Example ===

1. Detecting specific error types:
✓ Insert succeeded

2. Implementing retry logic:
Attempt 1: ✓ Success on attempt 1
  Found X users

3. Handling context timeouts:
Sending query with 1-second timeout...
✓ Query succeeded: X rows

=== Example Complete ===
```

**Error types handled:**

- `ErrDuplicateKey` - Non-retryable
- `ErrConnectionFailed` - Retryable (use exponential backoff)
- `ErrQueryTimeout` - Retryable (with caution)
- `ErrSyntaxError` - Non-retryable
- `ErrPermissionDenied` - Non-retryable

### 4. `retry/` - Retry Integration Examples

**What it does:**

- Demonstrates automatic backoff strategies (Exponential, Linear, Fixed)
- Shows read operations with entry fallback across replicas
- Shows write operations with guaranteed delivery
- Demonstrates health checks with retry
- Shows parallel batch operations with coordinated retry
- Explains when to use each backoff strategy

**Run:**

```bash
cd retry/
go run main.go
```

**Example output:**

```text
=== Retry Integration Examples ===

1. Basic Retry Patterns
───────────────────────

  • Query with default exponential backoff (100ms → 5s, 3 attempts)
    Result: Query executed with automatic retry

  • Query with linear backoff (50ms → 2s, 4 attempts)
    Result: Predictable retry intervals

  • Write (INSERT) with retry and guaranteed delivery
    Result: 1 rows affected

  • Health check with fixed backoff
    Result: Health check completed

...
```

**Use cases demonstrated:**

- **API Rate Limiting** - ExponentialBackoff for transient failures
- **Database Connection** - LinearBackoff for predictable delays
- **Health Checks** - FixedBackoff for known recovery time
- **Critical Operations** - Exponential with conservative limits
- **Thundering Herd** - Fixed with high jitter

See [retry/README.md](./retry/README.md) for detailed backoff strategy explanations.

## Running All Examples

```bash
# Run all examples in sequence
for example in basic error_handling priority_selection; do
  echo "=== Running $example.go ==="
  go run $example.go
  echo ""
done

# Run retry examples separately
echo "=== Running retry examples ==="
cd retry && go run main.go
```

## Common Issues

### Examples hang or timeout

**Cause:** PostgreSQL instances not running or config points to wrong host/port

**Solution:**

```bash
# Verify PostgreSQL is running
psql -U postgres -l

# Update config.yaml with correct host/port
vi config.yaml
```

### "Connection refused" error

**Cause:** PostgreSQL not listening on specified port

**Solution:**

```bash
# Check PostgreSQL port
psql --host localhost --port 5432 -U postgres -c "SELECT 1"

# Update config.yaml ports if different
```

### "Database does not exist" error

**Cause:** Database "myapp" not created

**Solution:**

```bash
psql -U postgres -c "CREATE DATABASE myapp;"
```

### "Table does not exist" error

**Cause:** Table "users" not created

**Solution:**

```bash
psql -d myapp -U postgres -c "
  CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100) UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
  );
"
```

## Advanced Topics

### Testing with primary down

To test failover behavior:

```bash
# Stop primary database
pg_ctl stop -D /path/to/primary

# Run examples - reads should still work on replicas
go run basic.go

# Restart primary
pg_ctl start -D /path/to/primary
```

### Monitoring query routing

Add logging to track which database each query hits:

```go
respCh := dm.Get(ctx, "", "users", []string{"id"}, nil, nil, nil)
resp := <-respCh
log.Printf("Query RequestID=%s, Rows=%d", resp.RequestID, len(resp.Data))
```

### Load testing

Use `priority_selection.go` as a base:

```go
// Fire 1000 concurrent queries
for i := 0; i < 1000; i++ {
    go func() {
        respCh := dm.Get(ctx, "", "users", ...)
        resp := <-respCh
        // Process response
    }()
}
```

## See Also

- [DBManager Documentation](../../docs/DBManager.md) - Complete guide
- [vessel README](../../README.md) - Library overview
- [examples/](../) - Other vessel examples
