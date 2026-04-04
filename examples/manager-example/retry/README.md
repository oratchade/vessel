# Retry Integration Examples

Comprehensive examples demonstrating the retry integration system in
fabric's DBManager.

## Overview

The retry system provides automatic backoff strategies for
transient failures in database operations, with support for:

- **Multiple backoff strategies**: Exponential, Linear, Fixed
- **Configurable retry attempts**: Per-operation or all entries
- **Context-aware execution**: Respects deadlines
- **Batch operations**: Parallel queries with coordinated retry
- **Health checks**: Periodic connectivity verification

## Running the Examples

```bash
go run main.go
```

## Backoff Strategies

### 1. Exponential Backoff

Best for: External APIs, initial fast retries that slow down

- Pattern: 100ms → 200ms → 400ms → ... (caps at max)
- Use when: Rate limiting, transient API/network failures

### 2. Linear Backoff

Best for: Predictable delays, steady retry intervals

- Pattern: 50ms → 100ms → 150ms → ... (constant increment)
- Use when: Database replica failover, scheduled maintenance

### 3. Fixed Backoff

Best for: Unknown recovery time, constant polling

- Pattern: 500ms, 500ms, 500ms (equal intervals)
- Use when: Connection pool exhaustion, health checks

## Configuration

All retry operations use `QueryWithRetryConfig`:

```go
cfg := &v1.QueryWithRetryConfig{
    Strategy:         retry.NewExponentialBackoff(
        100*time.Millisecond, 5*time.Second, 2.0, 3, 0.1),
    MaxEntryAttempts: 3,  // Try 3 different entries (-1 = all)
    Logger:           logger,
}
```

## Use Cases

### Read Operations

- Query with automatic fallback across replicas
- `MaxEntryAttempts: 2` tries up to 2 different database entries

### Write Operations

- Insert/Update with guaranteed delivery
- `MaxEntryAttempts: -1` tries all entries until success

### Health Checks

- Periodic connectivity verification
- Uses fixed backoff for predictable intervals

### Batch Operations

- Parallel queries with coordinated retry
- All jobs use the same retry configuration

## Examples

Run the main program to see:

1. **Basic Retry Patterns** - Query, write, health check examples
2. **Context Timeout Handling** - How deadlines interact with retry
3. **Backoff Strategy Recommendations** - When to use each strategy
4. **Practical Patterns** - Real-world use cases (read fallback,
   write guarantee, batch operations)

## Integration

For integration with your application:

```go
dm, err := v1.NewDBManager(ctx, "config.yaml", logger)
if err != nil {
    return err
}

// Basic query with default retry
rows, err := dm.QueryWithRetry(ctx, v1.DefaultQueryRetryConfig(),
    func(ctx context.Context) ([]map[string]any, error) {
    return dm.Query(ctx, "SELECT * FROM users")
})

// Custom retry configuration
cfg := &v1.QueryWithRetryConfig{
    Strategy: retry.NewExponentialBackoff(
        100*time.Millisecond, 10*time.Second, 2.0, 5, 0.1),
    MaxEntryAttempts: -1,  // Guaranteed delivery
    Logger:           logger,
}
result, err := dm.ExecWithRetry(ctx, cfg,
    func(ctx context.Context) (*db.ExecResult, error) {
    return dm.Exec(ctx, "INSERT INTO user_log (action) VALUES (?)", "login")
})
```

## Related Documentation

- [manager/v1 package](../../) - Main DBManager implementation
- [retry package](../../../pkg/retry) - Backoff strategy implementations
- [example_manager_retry.go](../../example_manager_retry.go) - Examples
