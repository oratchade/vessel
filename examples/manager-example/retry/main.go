package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	db "tounilab.com/vessel/db/v1"
	v1 "tounilab.com/vessel/manager/v1"
	"tounilab.com/vessel/pkg/retry"
)

func main() {
	ctx := context.Background()

	// Initialize a DBManager instance
	// In real code: dm, err := v1.NewDBManager(ctx, "config.yaml", logger)
	// For demonstration, we'll show the retry patterns
	var dm *v1.DBManager

	fmt.Println("=== Retry Integration Examples ===")
	fmt.Println()

	exampleBasicRetry(ctx, dm)
	exampleContextTimeout(ctx, dm)
	exampleBackoffStrategies()
	exampleRetryPatterns(ctx, dm)
}

// exampleBasicRetry shows basic query retry patterns
func exampleBasicRetry(ctx context.Context, dm *v1.DBManager) {
	fmt.Println("1. Basic Retry Patterns")
	fmt.Println("───────────────────────")
	fmt.Println()

	// Pattern: Simple query with default exponential backoff
	{
		fmt.Println("  • Query with default exponential backoff (100ms → 5s, 3 attempts)")
		cfg := v1.DefaultQueryRetryConfig()
		_, _ = dm.QueryWithRetry(ctx, cfg, func(ctx context.Context) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM users WHERE age > ?", 18)
		})
		fmt.Println("    Result: Query executed with automatic retry")
		fmt.Println()
	}

	// Pattern: Query with custom linear backoff
	{
		fmt.Println("  • Query with linear backoff (50ms → 2s, 4 attempts)")
		cfg := &v1.QueryWithRetryConfig{
			Strategy:         retry.NewLinearBackoff(50*time.Millisecond, 2*time.Second, 50*time.Millisecond, 4, 0.05),
			MaxEntryAttempts: 3,
			Logger:           nil,
		}
		_, _ = dm.QueryWithRetry(ctx, cfg, func(ctx context.Context) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM products WHERE stock > ?", 0)
		})
		fmt.Println("    Result: Predictable retry intervals")
		fmt.Println()
	}

	// Pattern: Write operation with retry
	{
		fmt.Println("  • Write (INSERT) with retry and guaranteed delivery")
		cfg := &v1.QueryWithRetryConfig{
			Strategy:         retry.NewExponentialBackoff(100*time.Millisecond, 5*time.Second, 2.0, 3, 0.1),
			MaxEntryAttempts: -1,
			Logger:           nil,
		}
		result, _ := dm.ExecWithRetry(ctx, cfg, func(ctx context.Context) (*db.ExecResult, error) {
			return dm.Exec(ctx, "INSERT INTO logs (message, timestamp) VALUES (?, ?)", "system_event", time.Now())
		})
		fmt.Printf("    Result: %v rows affected\n\n", result)
	}

	// Pattern: Health check with retry
	{
		fmt.Println("  • Health check with fixed backoff")
		cfg := &v1.QueryWithRetryConfig{
			Strategy:         retry.NewFixedBackoff(500*time.Millisecond, 3, 0.1),
			MaxEntryAttempts: 2,
			Logger:           nil,
		}
		_ = dm.HealthCheckWithRetry(ctx, cfg)
		fmt.Println("    Result: Health check completed")
		fmt.Println()
	}
}

// exampleContextTimeout shows how context deadlines interact with retry
func exampleContextTimeout(ctx context.Context, dm *v1.DBManager) {
	fmt.Println("2. Context Timeout Handling")
	fmt.Println("──────────────────────────")

	// Pattern: Timeout respects context deadline
	{
		fmt.Println("  • Query with context deadline (2 second timeout)")
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		cfg := v1.DefaultQueryRetryConfig()
		_, err := dm.QueryWithRetry(ctxWithTimeout, cfg, func(ctx context.Context) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM large_table")
		})

		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("    Result: Query exceeded 2s deadline, stopped retrying")
			fmt.Println()
		} else {
			fmt.Println("    Result: Query completed within deadline")
			fmt.Println()
		}
	}
}

// exampleBackoffStrategies shows different backoff strategies for various scenarios
func exampleBackoffStrategies() {
	fmt.Println("3. Backoff Strategy Recommendations")
	fmt.Println("──────────────────────────────────")

	scenarios := []struct {
		name     string
		scenario string
		strategy string
	}{
		{
			name:     "API Rate Limiting",
			scenario: "External API with rate limits or load balancing",
			strategy: "ExponentialBackoff(100ms, 30s, 2.0, -1, 0.1)",
		},
		{
			name:     "Database Connection",
			scenario: "Database temporarily unavailable or replica failover",
			strategy: "LinearBackoff(50ms, 5s, 50ms, 5, 0.05)",
		},
		{
			name:     "Health Checks",
			scenario: "Periodic monitoring with known recovery time",
			strategy: "FixedBackoff(500ms, 3, 0.1)",
		},
		{
			name:     "Critical Writes",
			scenario: "Important inserts/updates needing guaranteed success",
			strategy: "ExponentialBackoff(200ms, 10s, 2.0, 3, 0.05)",
		},
		{
			name:     "Thundering Herd",
			scenario: "Many clients retrying simultaneously",
			strategy: "FixedBackoff(1s, 5, 0.3) with high jitter",
		},
	}

	for i, s := range scenarios {
		fmt.Printf("  %d. %s\n", i+1, s.name)
		fmt.Printf("     Scenario: %s\n", s.scenario)
		fmt.Printf("     Strategy: %s\n\n", s.strategy)
	}
}

// exampleRetryPatterns shows practical retry patterns for common use cases
func exampleRetryPatterns(ctx context.Context, dm *v1.DBManager) {
	fmt.Println("4. Practical Retry Patterns")
	fmt.Println("──────────────────────────")

	// Pattern 1: Read with fallback
	{
		fmt.Println("  Pattern 1: Read with Entry Fallback")
		fmt.Println("  ─────────────────────────────────")
		fmt.Println("  Use case: Try multiple database replicas")
		cfg := &v1.QueryWithRetryConfig{
			Strategy: retry.NewLinearBackoff(
				100*time.Millisecond, 2*time.Second, 100*time.Millisecond, 3, 0.05,
			),
			MaxEntryAttempts: 2,
			Logger:           nil,
		}
		rows, _ := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM users")
		})
		fmt.Printf("  Result: Retrieved %d rows with automatic fallback\n\n", len(rows))
	}

	// Pattern 2: Write with guaranteed delivery
	{
		fmt.Println("  Pattern 2: Write with Guaranteed Delivery")
		fmt.Println("  ──────────────────────────────────────")
		fmt.Println("  Use case: Critical audit logs or financial transactions")
		cfg := &v1.QueryWithRetryConfig{
			Strategy:         retry.NewExponentialBackoff(200*time.Millisecond, 10*time.Second, 2.0, 5, 0.05),
			MaxEntryAttempts: -1,
			Logger:           nil,
		}
		result, _ := dm.ExecWithRetry(ctx, cfg, func(ctx context.Context) (*db.ExecResult, error) {
			return dm.Exec(ctx, "INSERT INTO audit_log (action, timestamp) VALUES (?, ?)", "user_login", time.Now())
		})
		fmt.Printf("  Result: Write succeeded with %d attempt(s)\n\n", result.RowsAffected)
	}

	// Pattern 3: Parallel batch with coordinated retry
	{
		fmt.Println("  Pattern 3: Parallel Batch with Coordinated Retry")
		fmt.Println("  ───────────────────────────────────────────────")
		fmt.Println("  Use case: Load multiple related tables simultaneously")
		cfg := v1.DefaultQueryRetryConfig()

		jobs := []*v1.BatchQueryJob{
			{Name: "users", Query: func(ctx context.Context) ([]map[string]any, error) {
				return dm.Query(ctx, "SELECT * FROM users")
			}},
			{Name: "products", Query: func(ctx context.Context) ([]map[string]any, error) {
				return dm.Query(ctx, "SELECT * FROM products")
			}},
			{Name: "orders", Query: func(ctx context.Context) ([]map[string]any, error) {
				return dm.Query(ctx, "SELECT * FROM orders")
			}},
		}

		results := dm.BatchQueryWithRetry(ctx, cfg, jobs)
		for _, r := range results {
			if r.Error == nil {
				fmt.Printf("  • %s: %d rows (attempt %d)\n", r.Name, len(r.Data), r.Attempt)
			} else {
				fmt.Printf("  • %s: failed - %v\n", r.Name, r.Error)
			}
		}
		fmt.Println()
	}
}
