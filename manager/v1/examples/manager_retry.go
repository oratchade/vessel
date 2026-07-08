package v1

import (
	"context"
	"log"
	"time"

	db "tounilab.com/vessel/db/v1"
	v1 "tounilab.com/vessel/manager/v1"
	"tounilab.com/vessel/pkg/retry"
)

// ExampleQueryWithRetry demonstrates basic query retry with DBManager
func ExampleQueryWithRetry() {
	ctx := context.Background()

	// In real code: dm, err := NewDBManager(ctx, "config.yaml", logger)
	// For this example, assuming dm is initialized
	var dm *v1.DBManager

	// Example 1: Simple query with default retry strategy
	{
		log.Println("Example 1: Basic query with default exponential backoff")

		cfg := v1.DefaultQueryRetryConfig()

		rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context, _ int) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM users WHERE age > ?", 18)
		})
		if err != nil {
			log.Printf("Query failed: %v\n", err)
			return
		}

		log.Printf("Query succeeded: %d rows\n", len(rows))
	}

	// Example 2: Query with custom linear backoff strategy
	{
		log.Println("Example 2: Linear backoff strategy")

		cfg := &v1.QueryWithRetryConfig{
			Strategy: retry.NewLinearBackoff(
				100*time.Millisecond, // initialDelay
				1*time.Second,        // maxDelay
				100*time.Millisecond, // increment
				5,                    // maxAttempts
				0.05,                 // jitterFactor
			),
			MaxEntryAttempts: -1, // Try all available entries
			Logger:           nil,
		}

		rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context, _ int) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM orders WHERE status = ?", "pending")
		})
		if err != nil {
			log.Printf("Query failed: %v\n", err)
			return
		}

		log.Printf("Query succeeded: %d rows\n", len(rows))
	}

	// Example 3: Exec with retry (for INSERT/UPDATE/DELETE)
	{
		log.Println("Example 3: Exec with retry")

		cfg := v1.DefaultQueryRetryConfig()

		result, err := dm.ExecWithRetry(ctx, cfg, func(ctx context.Context) (*db.ExecResult, error) {
			return dm.Exec(ctx, "UPDATE users SET status = ? WHERE id = ?", "active", 123)
		})
		if err != nil {
			log.Printf("Exec failed: %v\n", err)
			return
		}

		log.Printf("Exec succeeded: %v\n", result)
	}

	// Example 4: Query with fine-grained attempt tracking
	{
		log.Println("Example 4: Query with per-attempt logging")

		cfg := v1.DefaultQueryRetryConfig()

		rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context, attempt int) ([]map[string]any, error) {
			log.Printf("  Attempt #%d\n", attempt)
			return dm.Query(ctx, "SELECT * FROM users")
		})
		if err != nil {
			log.Printf("Query failed: %v\n", err)
			return
		}

		log.Printf("Query succeeded: %d rows\n", len(rows))
	}

	// Example 5: Health check with retry
	{
		log.Println("Example 5: Health check with retry")

		cfg := &v1.QueryWithRetryConfig{
			Strategy: retry.NewFixedBackoff(500*time.Millisecond, 3, 0.1),
			Logger:   nil,
		}

		err := dm.HealthCheckWithRetry(ctx, cfg)
		if err != nil {
			log.Printf("Health check failed: %v\n", err)
			return
		}

		log.Println("Health check passed")
	}

	// Example 6: Batch queries with retry
	{
		log.Println("Example 6: Batch queries with shared retry strategy")

		cfg := v1.DefaultQueryRetryConfig()

		jobs := []*v1.BatchQueryJob{
			{
				Name: "fetch_users",
				Query: func(ctx context.Context) ([]map[string]any, error) {
					return dm.Query(ctx, "SELECT * FROM users LIMIT 10")
				},
			},
			{
				Name: "fetch_orders",
				Query: func(ctx context.Context) ([]map[string]any, error) {
					return dm.Query(ctx, "SELECT * FROM orders LIMIT 10")
				},
			},
			{
				Name: "fetch_products",
				Query: func(ctx context.Context) ([]map[string]any, error) {
					return dm.Query(ctx, "SELECT * FROM products LIMIT 10")
				},
			},
		}

		results := dm.BatchQueryWithRetry(ctx, cfg, jobs)

		for _, result := range results {
			if result.Error != nil {
				log.Printf("  %s: FAILED (attempt %d): %v\n", result.Name, result.Attempt, result.Error)
			} else {
				log.Printf("  %s: OK (attempt %d, %d rows)\n", result.Name, result.Attempt, len(result.Data))
			}
		}
	}

	// Example 7: Custom strategy with attempt counter
	{
		log.Println("Example 7: Custom strategy with per-attempt logging")

		cfg := &v1.QueryWithRetryConfig{
			Strategy: retry.NewExponentialBackoff(
				100*time.Millisecond,
				5*time.Second,
				2.0,
				3,
				0.1,
			),
		}

		rows, err := dm.QueryWithRetry(
			ctx, cfg,
			func(ctx context.Context, attempt int) ([]map[string]any, error) {
				log.Printf("  Query attempt #%d\n", attempt)
				return dm.Query(ctx, "SELECT * FROM critical_data")
			})
		if err != nil {
			log.Printf("Custom retry query failed: %v\n", err)
			return
		}

		log.Printf("Custom retry query succeeded: %d rows\n", len(rows))
	}
}

// ExampleContextTimeout demonstrates context cancellation with retry
func ExampleContextTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var dm *v1.DBManager

	cfg := v1.DefaultQueryRetryConfig()

	rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context, _ int) ([]map[string]any, error) {
		// Simulated slow query
		select {
		case <-time.After(3 * time.Second):
			return []map[string]any{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err != nil {
		log.Printf("Query failed due to context deadline: %v\n", err)
		return
	}

	log.Printf("Query succeeded: %d rows\n", len(rows))
}

// ExampleRecommendedStrategies shows recommended backoff strategies for different scenarios
func ExampleRecommendedStrategies() {
	log.Println("=== Recommended Retry Strategies ===")

	// 1. API calls with rate limiting
	{
		log.Println("1. API Rate Limit (Exponential Backoff)")
		log.Println("   Strategy: ExponentialBackoff(100ms, 30s, 2.0, -1, 0.1)")
		log.Println("   Use when: Hitting rate limits, transient API failures")
		log.Println("   Pattern: 100ms → 200ms → 400ms → 800ms ... capped at 30s")
		_ = retry.NewExponentialBackoff(100*time.Millisecond, 30*time.Second, 2.0, -1, 0.1)
	}

	// 2. Database connection failures
	{
		log.Println("2. Database Connection (Linear Backoff)")
		log.Println("   Strategy: LinearBackoff(50ms, 5s, 50ms, 5, 0.05)")
		log.Println("   Use when: Database temporarily unavailable, replica failover")
		log.Println("   Pattern: 50ms → 100ms → 150ms → 200ms → 250ms")
		_ = retry.NewLinearBackoff(50*time.Millisecond, 5*time.Second, 50*time.Millisecond, 5, 0.05)
	}

	// 3. Health checks
	{
		log.Println("3. Health Checks (Fixed Backoff)")
		log.Println("   Strategy: FixedBackoff(500ms, 3, 0.1)")
		log.Println("   Use when: Periodic monitoring, known recovery time")
		log.Println("   Pattern: 500ms, 500ms, 500ms (with ±50ms jitter)")
		_ = retry.NewFixedBackoff(500*time.Millisecond, 3, 0.1)
	}

	// 4. Critical operations
	{
		log.Println("4. Critical Operations (Conservative)")
		log.Println("   Strategy: ExponentialBackoff(200ms, 10s, 2.0, 3, 0.05)")
		log.Println("   Use when: Important writes, need guaranteed success")
		log.Println("   Pattern: 200ms → 400ms → 800ms (3 attempts max)")
		_ = retry.NewExponentialBackoff(200*time.Millisecond, 10*time.Second, 2.0, 3, 0.05)
	}

	// 5. Thundering herd prevention
	{
		log.Println("5. Thundering Herd Prevention")
		log.Println("   Strategy: FixedBackoff(1s, 5, 0.3)")
		log.Println("   Use when: Many clients retrying, need jitter")
		log.Println("   Pattern: 1s ±300ms (5 attempts)")
		_ = retry.NewFixedBackoff(1*time.Second, 5, 0.3)
	}
}

// ExampleQueryWithLogging shows how to use a logger with retry
func ExampleQueryWithLogging(logger v1.Logger) {
	ctx := context.Background()

	var dm *v1.DBManager

	cfg := &v1.QueryWithRetryConfig{
		Strategy:         retry.NewExponentialBackoff(100*time.Millisecond, 5*time.Second, 2.0, 5, 0.1),
		MaxEntryAttempts: -1,
		Logger:           logger,
	}

	rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context, _ int) ([]map[string]any, error) {
		return dm.Query(ctx, "SELECT * FROM users")
	})
	if err != nil {
		log.Printf("Query failed: %v\n", err)
		return
	}

	log.Printf("Query succeeded: %d rows\n", len(rows))
}

// ExampleRetryPatterns shows various practical retry patterns
func ExampleRetryPatterns() {
	ctx := context.Background()

	var dm *v1.DBManager

	log.Println("=== Practical Retry Patterns ===")

	// Pattern 1: Read with fallback
	{
		log.Println("Pattern 1: Read with entry fallback")
		cfg := &v1.QueryWithRetryConfig{
			Strategy: retry.NewLinearBackoff(
				100*time.Millisecond, 2*time.Second, 100*time.Millisecond, 3, 0.05,
			),
			MaxEntryAttempts: 2,
			Logger:           nil,
		}

		rows, err := dm.QueryWithRetry(ctx, cfg, func(ctx context.Context, _ int) ([]map[string]any, error) {
			return dm.Query(ctx, "SELECT * FROM users")
		})
		if err != nil {
			log.Printf("Read failed: %v\n", err)
			return
		}

		log.Printf("Read succeeded with fallback: %d rows\n", len(rows))
	}

	// Pattern 2: Write with guaranteed delivery
	{
		log.Println("Pattern 2: Write with guaranteed delivery")
		cfg := &v1.QueryWithRetryConfig{
			Strategy:         retry.NewExponentialBackoff(200*time.Millisecond, 10*time.Second, 2.0, 5, 0.05),
			MaxEntryAttempts: -1,
			Logger:           nil,
		}

		result, err := dm.ExecWithRetry(ctx, cfg, func(ctx context.Context) (*db.ExecResult, error) {
			return dm.Exec(ctx, "INSERT INTO audit_log (action, timestamp) VALUES (?, ?)", "user_created", time.Now())
		})
		if err != nil {
			log.Printf("Write failed: %v\n", err)
			return
		}

		log.Printf("Write succeeded with retry: %v\n", result)
	}

	// Pattern 3: Parallel batch with coordinated retry
	{
		log.Println("Pattern 3: Parallel batch with coordinated retry")
		cfg := v1.DefaultQueryRetryConfig()

		jobs := []*v1.BatchQueryJob{
			{Name: "table_1", Query: func(ctx context.Context) ([]map[string]any, error) {
				return dm.Query(ctx, "SELECT * FROM table_1")
			}},
			{Name: "table_2", Query: func(ctx context.Context) ([]map[string]any, error) {
				return dm.Query(ctx, "SELECT * FROM table_2")
			}},
			{Name: "table_3", Query: func(ctx context.Context) ([]map[string]any, error) {
				return dm.Query(ctx, "SELECT * FROM table_3")
			}},
		}

		results := dm.BatchQueryWithRetry(ctx, cfg, jobs)
		for _, r := range results {
			if r.Error == nil {
				log.Printf("%s: %d rows after %d attempt(s)\n", r.Name, len(r.Data), r.Attempt)
			} else {
				log.Printf("%s: error after %d attempt(s): %v\n", r.Name, r.Attempt, r.Error)
			}
		}
	}
}
