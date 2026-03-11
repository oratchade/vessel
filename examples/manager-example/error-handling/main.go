package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"tounilab.com/db-connector/db/v1/dberror"
	v1 "tounilab.com/db-connector/manager/v1"
)

// Example: Error Handling with DBManager
//
// This example demonstrates:
// - Checking for errors in query responses
// - Identifying specific error types (duplicate key, connection failed, etc.)
// - Implementing retry logic
// - Handling backpressure when queues are full
// - Graceful degradation
//
// To run:
//   go run error_handling.go config.yaml

func main() {
	ctx := context.Background()

	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	dm, err := v1.NewDBManager(ctx, configPath)
	if err != nil {
		log.Fatalf("Failed to create DBManager: %v", err)
	}

	// Start workers and health checks
	dm.Start(ctx)

	defer dm.Stop()

	time.Sleep(100 * time.Millisecond)

	log.Println("=== DBManager Error Handling Example ===")

	// Example 1: Detect specific error types
	log.Println("1. Detecting specific error types:")
	detectErrorTypesExample(ctx, dm)

	// Example 2: Implement retry logic
	log.Println("2. Implementing retry logic:")
	retryExample(ctx, dm)

	// Example 3: Handle timeouts
	log.Println("\n3. Handling context timeouts:")
	timeoutExample(ctx, dm)

	log.Println("\n=== Example Complete ===")
}

// detectErrorTypesExample shows how to identify specific database errors
func detectErrorTypesExample(ctx context.Context, dm *v1.DBManager) {
	// Try to insert duplicate key (will fail if email already exists)
	respCh := dm.Insert(ctx, "", "users", map[string]interface{}{
		"name":  "Test User",
		"email": "duplicate@example.com",
	}, nil)

	resp := <-respCh

	// Always check resp.Error first
	if resp.Error == nil {
		log.Println("✓ Insert succeeded")
		return
	}

	// Check for specific error types
	switch {
	case errors.Is(resp.Error, dberror.ErrDuplicateKey):
		// DUPLICATE KEY error - email already exists
		log.Println("⚠ Duplicate key error: Email already exists")
		log.Println("  Action: Use different email or update existing record")

	case errors.Is(resp.Error, dberror.ErrConnectionFailed):
		// CONNECTION FAILED - database is unreachable
		log.Println("⚠ Connection error: Database unreachable")
		log.Println("  Action: Check database connectivity, retry with backoff")

	case errors.Is(resp.Error, dberror.ErrQueryTimeout):
		// TIMEOUT - query took too long
		log.Println("⚠ Timeout error: Query took too long")
		log.Println("  Action: Optimize query or increase timeout")

	case errors.Is(resp.Error, dberror.ErrSyntaxError):
		// SYNTAX ERROR - malformed query
		log.Println("⚠ Syntax error: Invalid SQL")
		log.Println("  Action: Review SQL syntax")

	default:
		// Other errors
		log.Printf("✗ Unknown error: %v (type: %T)\n", resp.Error, resp.Error)
	}
}

// retryExample demonstrates retry logic with exponential backoff
func retryExample(ctx context.Context, dm *v1.DBManager) {
	maxRetries := 3
	backoff := 100 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Attempt %d: ", attempt)

		respCh := dm.Get(ctx, "", "users", []string{"id", "name"}, nil, nil, nil)

		// Timeout protection
		select {
		case resp := <-respCh:
			if resp.Error == nil {
				log.Printf("✓ Success on attempt %d\n", attempt)
				if len(resp.Data) > 0 {
					log.Printf("  Found %d users\n", len(resp.Data))
				}
				return
			}

			// Decide if we should retry
			log.Printf("Error: %v\n", resp.Error)

			if attempt < maxRetries {
				// Check if this is a retryable error
				if isRetryable(resp.Error) {
					log.Printf("  Retrying after %v...\n", backoff)
					time.Sleep(backoff)
					backoff *= 2 // Exponential backoff
					continue
				} else {
					log.Println("  Non-retryable error, giving up")
					return
				}
			}

		case <-time.After(5 * time.Second):
			// Response channel didn't send in time
			log.Println("Timeout waiting for response")
			return
		}
	}

	log.Printf("Failed after %d attempts\n", maxRetries)
}

// timeoutExample demonstrates handling context timeouts
func timeoutExample(ctx context.Context, dm *v1.DBManager) {
	// Create context with 1 second timeout
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	log.Println("Sending query with 1-second timeout...")
	respCh := dm.Get(ctx, "", "users", []string{"id"}, nil, nil, nil)

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			log.Printf("✗ Query error: %v\n", resp.Error)
		} else {
			log.Printf("✓ Query succeeded: %d rows\n", len(resp.Data))
		}

	case <-ctx.Done():
		// Context canceled (timeout)
		log.Println("⏱ Query timeout: Context deadline exceeded")
		log.Println("  Query took longer than 1 second")
	}
}

// isRetryable determines if an error should trigger a retry
func isRetryable(err error) bool {
	// These errors might be transient and worth retrying
	if errors.Is(err, dberror.ErrConnectionFailed) {
		// Connection errors can be transient
		return true
	}

	if errors.Is(err, dberror.ErrQueryTimeout) {
		// Timeouts might be transient load issues
		return true
	}

	// These errors are usually permanent and shouldn't be retried
	if errors.Is(err, dberror.ErrDuplicateKey) {
		return false // Will fail again
	}

	if errors.Is(err, dberror.ErrSyntaxError) {
		return false // Query is invalid
	}

	// For unknown errors, default to true (better to retry and fail)
	return true
}

/*
Common error handling patterns:

1. DUPLICATE KEY:
   - Usually means data already exists
   - Action: Check if update needed, or use INSERT OR IGNORE
   - Retryable: No

2. CONNECTION FAILED:
   - Database temporarily unreachable
   - Action: Implement exponential backoff retry
   - Retryable: Yes

3. QUERY TIMEOUT:
   - Query took too long
   - Action: Increase timeout, optimize query, or retry
   - Retryable: Yes (but with care)

4. SYNTAX ERROR:
   - Malformed SQL
   - Action: Review and fix query
   - Retryable: No

5. PERMISSION DENIED:
   - User lacks permissions
   - Action: Check credentials, adjust permissions
   - Retryable: No

6. CONSTRAINT VIOLATION:
   - Foreign key or check constraint failed
   - Action: Validate data before insert/update
   - Retryable: No (but might be retryable if due to async operations)

Best Practices:
✓ Always check resp.Error before accessing resp.Data
✓ Use errors.Is() to check specific error types
✓ Implement exponential backoff for transient errors
✓ Use context.WithTimeout() to prevent indefinite hangs
✓ Log errors with context (which database, which query, etc.)
✓ Monitor error rates to detect systemic issues
*/
