# Retry Package Documentation

## Overview

The `retry` package provides a flexible retry mechanism for
Go applications with multiple backoff strategies. It
handles context cancellation, error propagation, and timing.

**Key Features:**

- Multiple backoff strategies (exponential, linear, fixed, no-op)
- Jitter support for thundering herd prevention
- Context-aware with deadline support
- Two execution models: simple error handling and generic result returns
- Race-condition safe
- 100% test coverage

## Installation

```go
import "tounilab.com/vessel/pkg/retry"
```

## Core Concepts

### Strategy Interface

All backoff implementations conform to the `Strategy` interface:

```go
type Strategy interface {
    // NextDelay returns the delay before the next retry attempt.
    // attempt is zero-indexed (0 for first attempt, 1 for second, etc.).
    // Returns a negative duration to indicate no more retries should be attempted.
    NextDelay(attempt int) time.Duration
}
```

### Backoff Strategies

#### ExponentialBackoff

Increases delay exponentially with each attempt.

**Configuration:**

- `initialDelay`: starting delay (e.g., 100ms)
- `maxDelay`: cap on maximum delay (e.g., 10s)
- `baseMultiplier`: exponential growth factor (e.g., 2.0)
- `maxAttempts`: maximum retries (-1 for unlimited)
- `jitterFactor`: randomization as fraction of delay (0.0-1.0)

**Sequence:** `initialDelay * (baseMultiplier ^ attempt)`
capped at `maxDelay`

**Use Case:** Network requests, API calls, transient failures

```go
strategy := retry.NewExponentialBackoff(
    100*time.Millisecond,  // initialDelay
    10*time.Second,        // maxDelay
    2.0,                   // baseMultiplier
    10,                    // maxAttempts
    0.1,                   // jitterFactor (±10%)
)
```

Example sequence (no jitter):

- Attempt 0: 100ms
- Attempt 1: 200ms
- Attempt 2: 400ms
- Attempt 3: 800ms
- Attempt 4: 1.6s
- Attempt 5+: capped at 10s

#### LinearBackoff

Increases delay linearly with each attempt.

**Configuration:**

- `initialDelay`: starting delay (e.g., 100ms)
- `increment`: increase per attempt (e.g., 50ms)
- `maxDelay`: cap on maximum delay (e.g., 10s)
- `maxAttempts`: maximum retries (-1 for unlimited)
- `jitterFactor`: randomization as fraction of delay (0.0-1.0)

**Sequence:** `initialDelay + (increment * attempt)`, capped at `maxDelay`

**Use Case:** Queue processing, database operations with backpressure

```go
strategy := retry.NewLinearBackoff(
    100*time.Millisecond,  // initialDelay
    50*time.Millisecond,   // increment
    5*time.Second,         // maxDelay
    -1,                    // unlimited attempts
    0.05,                  // jitterFactor (±5%)
)
```

Example sequence (no jitter):

- Attempt 0: 100ms
- Attempt 1: 150ms
- Attempt 2: 200ms
- Attempt 3: 250ms
- ...capped at 5s

#### FixedBackoff

Constant delay between retries.

**Configuration:**

- `delay`: fixed delay between retries
- `maxAttempts`: maximum retries (-1 for unlimited)
- `jitterFactor`: randomization as fraction of delay (0.0-1.0)

**Use Case:** Simple retry scenarios, tests, predictable workloads

```go
strategy := retry.NewFixedBackoff(
    200*time.Millisecond,  // delay
    5,                     // maxAttempts
    0.1,                   // jitterFactor (±10%)
)
```

#### NoOpBackoff

Never retries; fails immediately.

**Use Case:** Testing, disabling retries, single-attempt scenarios

```go
strategy := retry.NewNoOpBackoff()
```

## Execution Models

### Do() - Simple Error Handling

Executes a function with automatic retries until success or all retries exhausted.

```go
func Do(ctx context.Context, strategy Strategy, fn func() error) error
```

**Example:**

```go
err := retry.Do(
    context.Background(),
    strategy,
    func() error {
        resp, err := http.Get("https://api.example.com/data")
        if err != nil {
            return err
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            return fmt.Errorf("unexpected status: %d", resp.StatusCode)
        }

        // Success
        return nil
    },
)

if err != nil {
    log.Fatal("Failed after all retries:", err)
}
```

### DoWithResult\[T\]() - Generic Result Returns

Executes a function and returns both the result and error.

```go
func DoWithResult[T any](ctx context.Context, strategy
    Strategy, fn func() (T, error)) (T, error)
```

**Example:**

```go
type UserData struct {
    ID   int
    Name string
}

user, err := retry.DoWithResult(
    ctx,
    strategy,
    func() (UserData, error) {
        // Fetch user from database
        row := db.QueryRow("SELECT id, name FROM users WHERE id = ?", userID)
        var user UserData
        if err := row.Scan(&user.ID, &user.Name); err != nil {
            return UserData{}, fmt.Errorf("scan failed: %w", err)
        }
        return user, nil
    },
)

if err != nil {
    return nil, fmt.Errorf("failed to fetch user: %w", err)
}

log.Printf("Fetched user: %+v", user)
```

## Context Handling

Both execution functions respect context timeouts and cancellations:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retry.Do(ctx, strategy, func() error {
    // Will fail if total time exceeds 30 seconds
    return makeRequest()
})

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(5 * time.Second)
    cancel() // Stop retrying
}()

err := retry.Do(ctx, strategy, func() error {
    return makeRequest()
})
```

## Jitter

Jitter prevents thundering herd problems in distributed
systems. It adds randomness to delays.

**Without jitter:** All clients retry at exact same time → overload spike

**With jitter:** Retries spread across time window → even load distribution

**Configuration:**

- `jitterFactor`: fraction to randomize (0.0 = no
  jitter, 0.1 = ±10%, 0.5 = ±50%)
- Actual delay = `baseDelay ± (baseDelay * jitterFactor * random)`

**Example:**

```go
// Without jitter: 200ms, 200ms, 200ms...
fixed := retry.NewFixedBackoff(200*time.Millisecond, -1, 0.0)

// With jitter: random between 180ms-220ms
jittered := retry.NewFixedBackoff(200*time.Millisecond, -1, 0.1)
```

## Error Messages

The package provides clear, actionable error messages:

````go
var result string
err := Do(ctx, func() error {
    result = "value"
    return nil
}, strategy)

## Common Patterns

### Environment-Specific Strategies

```go
func getRetryStrategy() retry.Strategy {
    env := os.Getenv("ENVIRONMENT")

    switch env {
    case "production":
        // Aggressive backoff for production
        return retry.NewExponentialBackoff(
            100*time.Millisecond,
            1*time.Minute,
            2.0,
            15,
            0.1,
        )
    case "staging":
        return retry.NewExponentialBackoff(
            50*time.Millisecond,
            30*time.Second,
            2.0,
            10,
            0.05,
        )
    default:
        // Tests: no retries
        return retry.NewNoOpBackoff()
    }
}
````

### Resource-Type Specific Strategies

```go
// For API calls (fast, transient failures likely)
apiStrategy := retry.NewExponentialBackoff(
    10*time.Millisecond, 5*time.Second, 2.0, 8, 0.1,
)

// For database operations (slower, more resilient)
dbStrategy := retry.NewLinearBackoff(
    100*time.Millisecond, 100*time.Millisecond,
    30*time.Second, -1, 0.05,
)

// For long-running operations
longOpStrategy := retry.NewExponentialBackoff(
    1*time.Second, 2*time.Minute, 1.5, 10, 0.2,
)
```

### Wrapped Function Pattern

```go
func callAPIWithRetry(ctx context.Context, url string) ([]byte, error) {
    strategy := retry.NewExponentialBackoff(
        50*time.Millisecond, 10*time.Second, 2.0, -1, 0.1,
    )

    return retry.DoWithResult(ctx, strategy, func() ([]byte, error) {
        return callAPI(url)
    })
}

// Usage
data, err := callAPIWithRetry(ctx, "https://api.example.com/data")
```

## Testing

The package includes comprehensive tests covering:

- All backoff strategies
- Jitter behavior
- Context cancellation and timeouts
- Error handling and exhaustion
- Integration scenarios
- Edge cases

**Run tests:**

```bash
go test -race -cover ./pkg/retry/...
```

**Expected output:** ✓ All tests pass with 100% coverage

## Benchmarks

The package is optimized for performance:

```bash
go test -bench=. ./pkg/retry/...
```

Typical results:

- ExponentialBackoff.NextDelay: ~200ns
- LinearBackoff.NextDelay: ~200ns
- FixedBackoff.NextDelay: ~100ns

## Best Practices

1. **Choose appropriate backoff strategy:**
   - Network calls: Exponential with jitter
   - Database: Linear or exponential
   - Tests: Fixed or NoOp

2. **Always use context:** Enables timeout and cancellation support

3. **Set reasonable timeouts:**

   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ```

4. **Use jitter for distributed systems:** Prevents thundering herd

5. **Monitor retry rates:** Add observability to track when retries occur

6. **Document your strategy:** Help future maintainers understand retry logic

7. **Test failure scenarios:** Verify retry behavior under failure conditions

## Performance Considerations

- **Allocation:** Each NextDelay() call is allocation-free
- **Goroutines:** Minimal overhead; uses standard `time.After()`
- **Memory:** Fixed memory per strategy instance
- **Concurrency:** Safe for concurrent use (strategies are stateless)

## License

Vessel is licensed under the MIT License. See LICENSE.md for details.
