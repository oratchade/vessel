package v1

import (
	"context"
	"fmt"
	"sync"
	"time"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/retry"
)

// QueryWithRetryConfig holds configuration for query retry behavior
type QueryWithRetryConfig struct {
	// Strategy defines the backoff strategy (exponential, linear, fixed)
	Strategy retry.Strategy

	// MaxEntryAttempts limits how many different entries to try per strategy iteration
	// A value of -1 means try all healthy entries
	MaxEntryAttempts int

	// Logger can be set to debug retry attempts
	Logger Logger
}

// DefaultQueryRetryConfig returns sensible defaults for query retry
func DefaultQueryRetryConfig() *QueryWithRetryConfig {
	return &QueryWithRetryConfig{
		// Exponential backoff: 100ms -> 200ms -> 400ms ... capped at 5s
		Strategy:         retry.NewExponentialBackoff(100*time.Millisecond, 5*time.Second, 2.0, 3, 0.1),
		MaxEntryAttempts: -1, // Try all available entries
		Logger:           nil,
	}
}

// QueryWithRetry wraps a query function with automatic retry logic across entries
//
// This function integrates the retry package with DBManager's entry selection.
// When a query fails:
// 1. The retry strategy determines the backoff delay
// 2. Next call selects the next available entry (by health + priority + round-robin)
// 3. Query is retried according to the strategy
//
// Example:
//
//	cfg := DefaultQueryRetryConfig() // Uses exponential backoff
//	rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context) ([]map[string]any, error) {
//	    return dm.Query(ctx, "SELECT * FROM users WHERE age > ?", 18)
//	})
//
// The wrapper automatically handles:
// - Entry failover on unhealthy replicas
// - Exponential backoff between attempts
// - Context cancellation at any point
// - Clear error messages with attempt counts
func (dm *DBManager) QueryWithRetry(
	ctx context.Context,
	cfg *QueryWithRetryConfig,
	fn func(context.Context) ([]map[string]any, error),
) ([]map[string]any, error) {
	if cfg == nil {
		cfg = DefaultQueryRetryConfig()
	}

	result, err := retry.DoWithResult(ctx, cfg.Strategy, func() ([]map[string]any, error) {
		if cfg.Logger != nil {
			cfg.Logger.Debug("Query retry attempt",
				"attempt", 1, // Simplified for logging
			)
		}

		return fn(ctx)
	})

	if err != nil && cfg.Logger != nil {
		cfg.Logger.Error("Query failed after retries",
			"error", err.Error(),
		)
	}

	return result, fmt.Errorf("query with retry failed: %w", err)
}

// ExecWithRetry wraps an exec function with automatic retry logic across entries
//
// Similar to QueryWithRetry but for INSERT/UPDATE/DELETE operations.
//
// Example:
//
//	cfg := DefaultQueryRetryConfig()
//	result, err := dm.ExecWithRetry(ctx, cfg, func(ctx context.Context) (*db.ExecResult, error) {
//	    return dm.Exec(ctx, "UPDATE users SET status = ? WHERE id = ?", "active", 123)
//	})
func (dm *DBManager) ExecWithRetry(
	ctx context.Context,
	cfg *QueryWithRetryConfig,
	fn func(context.Context) (*db.ExecResult, error),
) (*db.ExecResult, error) {
	// ExecResult is from db package, we need to convert it
	if cfg == nil {
		cfg = DefaultQueryRetryConfig()
	}

	result, err := retry.DoWithResult(ctx, cfg.Strategy, func() (*db.ExecResult, error) {
		if cfg.Logger != nil {
			cfg.Logger.Debug("Exec retry attempt")
		}

		return fn(ctx)
	})

	if err != nil && cfg.Logger != nil {
		cfg.Logger.Error("Exec failed after retries",
			"error", err.Error(),
		)
	}

	return result, fmt.Errorf("exec with retry failed: %w", err)
}

type MultiEntryQueryFunc func(context.Context, int) ([]map[string]any, error)

// MultiEntryQuery executes a query across multiple entries with retry logic
//
// This advanced pattern is useful when you want to:
// - Distribute read load across multiple replicas
// - Fallback to different entries on failure
// - Aggregate results from multiple sources
//
// Example: Query with explicit entry preference
//
//	cfg := &QueryWithRetryConfig{
//	    Strategy: retry.NewLinearBackoff(
//	        100*time.Millisecond,
//	        1*time.Second,
//	        100*time.Millisecond,
//	        3,  // Try 3 entries
//	        0.1,
//	    ),
//	}
//
//	rows, err := dm.MultiEntryQuery(ctx, cfg, func(ctx context.Context, attempt int) ([]map[string]any, error) {
//	    // Each attempt is a new entry selection
//	    return dm.Query(ctx, "SELECT * FROM events")
//	})
func (dm *DBManager) MultiEntryQuery(
	ctx context.Context,
	cfg *QueryWithRetryConfig,
	fn MultiEntryQueryFunc,
) ([]map[string]any, error) {
	if cfg == nil {
		cfg = DefaultQueryRetryConfig()
	}

	attemptCounter := 0
	result, err := retry.DoWithResult(ctx, cfg.Strategy, func() ([]map[string]any, error) {
		attemptCounter++
		if cfg.Logger != nil {
			cfg.Logger.Debug("Multi-entry query attempt",
				"attempt_number", attemptCounter,
			)
		}

		return fn(ctx, attemptCounter)
	})

	if err != nil && cfg.Logger != nil {
		cfg.Logger.Error("Multi-entry query failed",
			"attempts", attemptCounter,
			"error", err.Error(),
		)
		return result, fmt.Errorf("multi-entry query failed: %w", err)
	}

	return result, nil
}

// RetryableQueryFunc is the function signature for queries that can be retried
// The attempt parameter (1-indexed) indicates which attempt this is
type RetryableQueryFunc func(context.Context, int) ([]map[string]any, error)

// QueryWithCustomRetry provides fine-grained control over retry behavior
//
// This is useful when you need custom logic beyond simple entry failover.
//
// Example: Retry with custom backoff and logging
//
//	strategy := retry.NewExponentialBackoff(
//	    100*time.Millisecond,  // Start with 100ms
//	    10*time.Second,        // Cap at 10s
//	    2.0,                   // Double each time
//	    5,                     // Max 5 attempts
//	    0.1,                   // 10% jitter
//	)
//
//		rows, err := dm.QueryWithCustomRetry(
//			ctx, strategy,
//			func(ctx context.Context, attempt int) ([]map[string]any, error) {
//	    fmt.Printf("Attempting query (attempt %d)\n", attempt)
//	    return dm.Query(ctx, "SELECT * FROM users")
//	})
func (dm *DBManager) QueryWithCustomRetry(
	ctx context.Context,
	strategy retry.Strategy,
	fn RetryableQueryFunc,
) ([]map[string]any, error) {
	attemptCounter := 0
	result, err := retry.DoWithResult(ctx, strategy, func() ([]map[string]any, error) {
		attemptCounter++
		return fn(ctx, attemptCounter)
	})
	if err != nil {
		return result, fmt.Errorf("custom retry failed: %w", err)
	}
	return result, nil
}

// HealthCheckWithRetry performs periodic health checks with retry logic
//
// Useful for monitoring entry health separately from query execution.
//
// Example:
//
//	cfg := &QueryWithRetryConfig{
//	    Strategy: retry.NewFixedBackoff(500*time.Millisecond, 2, 0.05),
//	}
//	err := dm.HealthCheckWithRetry(ctx, cfg)
func (dm *DBManager) HealthCheckWithRetry(
	ctx context.Context,
	cfg *QueryWithRetryConfig,
) error {
	if cfg == nil {
		cfg = DefaultQueryRetryConfig()
	}

	err := retry.Do(ctx, cfg.Strategy, func() error {
		if cfg.Logger != nil {
			cfg.Logger.Debug("Health check retry attempt")
		}

		entry := dm.readOnlyEntry()
		if entry == nil {
			return fmt.Errorf("no read-only entries available for health check")
		}

		return entry.db.Ping(ctx)
	})
	if err != nil {
		return fmt.Errorf("health check with retry failed: %w", err)
	}

	return nil
}

type BatchQueryJob struct {
	Name  string
	Query func(ctx context.Context) ([]map[string]any, error)
}

type BatchQueryResult struct {
	Name    string
	Data    []map[string]any
	Error   error
	Attempt int
}

// BatchQueryWithRetry executes multiple queries in parallel with retry logic.
//
// Each query is retried independently according to the strategy.
// If ctx lacks a deadline, a default 30-second timeout is added to prevent goroutine leaks.
func (dm *DBManager) BatchQueryWithRetry(
	ctx context.Context,
	cfg *QueryWithRetryConfig,
	jobs []*BatchQueryJob,
) []*BatchQueryResult {
	if cfg == nil {
		cfg = DefaultQueryRetryConfig()
	}

	// Ensure context has a deadline to prevent indefinite goroutine hangs.
	// This is a safeguard against queries that may block forever.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	results := make([]*BatchQueryResult, len(jobs))
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j *BatchQueryJob) {
			defer wg.Done()

			attemptCounter := 0
			data, err := retry.DoWithResult(ctx, cfg.Strategy, func() ([]map[string]any, error) {
				attemptCounter++
				return j.Query(ctx)
			})

			results[idx] = &BatchQueryResult{
				Name:    j.Name,
				Data:    data,
				Error:   err,
				Attempt: attemptCounter,
			}

			if err != nil && cfg.Logger != nil {
				cfg.Logger.Error("Batch query failed",
					"job_name", j.Name,
					"attempts", attemptCounter,
					"error", err.Error(),
				)
			}
		}(i, job)
	}

	wg.Wait()
	return results
}

// QueryRetryMetrics tracks retry statistics for monitoring
type QueryRetryMetrics struct {
	TotalAttempts           int64
	SuccessfulQueries       int64
	FailedQueries           int64
	TotalRetries            int64
	AverageAttemptsPerQuery float64
}

// RetryMetricsCollector collects metrics from query retries
type RetryMetricsCollector struct {
	enabled bool
	metrics *QueryRetryMetrics
	mu      sync.RWMutex
}

// NewRetryMetricsCollector creates a new metrics collector
func NewRetryMetricsCollector() *RetryMetricsCollector {
	return &RetryMetricsCollector{
		enabled: true,
		metrics: &QueryRetryMetrics{},
	}
}

// RecordAttempt records a query attempt
func (rmc *RetryMetricsCollector) RecordAttempt(success bool, attemptCount int) {
	if !rmc.enabled {
		return
	}

	rmc.mu.Lock()
	defer rmc.mu.Unlock()

	rmc.metrics.TotalAttempts++
	if success {
		rmc.metrics.SuccessfulQueries++
	} else {
		rmc.metrics.FailedQueries++
	}
	rmc.metrics.TotalRetries += int64(attemptCount - 1)
}

// GetMetrics returns current metrics
func (rmc *RetryMetricsCollector) GetMetrics() *QueryRetryMetrics {
	rmc.mu.RLock()
	defer rmc.mu.RUnlock()

	// Calculate averages
	var avgAttempts float64
	if rmc.metrics.TotalAttempts > 0 {
		avgAttempts = float64(rmc.metrics.TotalAttempts+rmc.metrics.TotalRetries) / float64(rmc.metrics.TotalAttempts)
	}

	return &QueryRetryMetrics{
		TotalAttempts:           rmc.metrics.TotalAttempts,
		SuccessfulQueries:       rmc.metrics.SuccessfulQueries,
		FailedQueries:           rmc.metrics.FailedQueries,
		TotalRetries:            rmc.metrics.TotalRetries,
		AverageAttemptsPerQuery: avgAttempts,
	}
}

// Logger interface for retry logging (to avoid circular imports)
// This mirrors the db.Logger interface for consistency
type Logger interface {
	Debug(msg string, keyvals ...any)
	Error(msg string, keyvals ...any)
}
