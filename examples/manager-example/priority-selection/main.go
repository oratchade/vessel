package main

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	v1 "tounilab.com/db-connector/manager/v1"
)

// Example: Priority-Based Selection and Async Processing
//
// This example demonstrates:
// - How priority-based selection routes queries
// - Async fire-and-forget pattern
// - Handling multiple concurrent queries
// - Response aggregation
//
// Query routing with example config:
// - Priority 100 (primary): Gets all writes
// - Priority 50 (replica-1, replica-2): Load-balanced for reads
// - If primary down: Reads still work on replicas
//
// To run:
//   go run priority_selection.go config.yaml

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

	defer func() {
		dm.Stop()
	}()

	time.Sleep(100 * time.Millisecond)

	log.Println("=== DBManager Priority Selection Example ===")

	// Example 1: Demonstrate priority-based routing
	log.Println("1. Testing priority-based selection:")
	log.Println("   - Writes (Insert) → Always goes to priority:100 (primary-db)")
	log.Println("   - Reads (Get) → Load-balanced between priority:50 (replica-1, replica-2)")
	log.Println()

	// Example 2: Fire multiple concurrent queries
	log.Println("2. Firing 10 concurrent read queries (async):")
	concurrentReadExample(ctx, dm)

	// Example 3: Mixed write and read operations
	log.Println("\n3. Mixed write and read operations:")
	mixedOperationsExample(ctx, dm)

	log.Println("\n=== Example Complete ===")
}

// concurrentReadExample fires multiple read queries concurrently
// and aggregates results asynchronously
func concurrentReadExample(ctx context.Context, dm *v1.DBManager) {
	var wg sync.WaitGroup
	successCount := atomic.Int32{}
	errorCount := atomic.Int32{}

	// Fire 10 queries without waiting for responses
	responseChs := make([]<-chan *v1.QueryResponse, 10)
	for i := 0; i < 10; i++ {
		respCh := dm.Get(ctx, "", "users", []string{"id", "name"}, nil, nil, nil)
		responseChs[i] = respCh

		wg.Add(1)
		// Handle response asynchronously
		go func(idx int, ch <-chan *v1.QueryResponse) {
			defer wg.Done()

			resp := <-ch
			if resp.Error != nil {
				log.Printf("   Query %d: ERROR - %v\n", idx+1, resp.Error)
				errorCount.Add(1)
			} else {
				//nolint:gosec
				log.Printf("   Query %d: OK - found %d rows\n", idx+1, len(resp.Data))
				successCount.Add(1)
			}
		}(i, respCh)
	}

	// Wait for all queries to complete
	wg.Wait()

	log.Printf("   Results: %d successful, %d errors\n", successCount.Load(), errorCount.Load())
	log.Println("   Note: All read queries were routed to replicas with priority:50")
}

// mixedOperationsExample shows writes going to primary, reads balancing across replicas
func mixedOperationsExample(ctx context.Context, dm *v1.DBManager) {
	var wg sync.WaitGroup

	// Fire write query (goes to primary)
	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Println("   Sending INSERT to primary-db (priority:100)...")
		respCh := dm.Insert(ctx, "", "users", map[string]interface{}{
			"name":  "Bob",
			"email": "bob@example.com",
		}, nil)

		resp := <-respCh
		if resp.Error != nil {
			log.Printf("   INSERT error: %v\n", resp.Error)
		} else {
			//nolint:gosec
			log.Printf("   INSERT OK: ID=%v\n", resp.ExecData.LastInsertID)
		}
	}()

	// Fire multiple read queries (distributed across replicas)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()

			time.Sleep(time.Duration(num*50) * time.Millisecond)
			log.Printf("   Sending GET #%d (routed via load-balancer to replica-1 or replica-2)...\n", num+1)
			respCh := dm.Get(ctx, "", "users", []string{"id", "name"}, nil, nil, nil)

			resp := <-respCh
			if resp.Error != nil {
				log.Printf("   GET #%d error: %v\n", num+1, resp.Error)
			} else {
				//nolint:gosec
				log.Printf("   GET #%d OK: found %d rows\n", num+1, len(resp.Data))
			}
		}(i)
	}

	wg.Wait()
}

// Note: To truly observe priority-based routing in action, you could:
// 1. Stop the primary database and see reads continue on replicas
// 2. Monitor query logs to see which database each query hits
// 3. Add instrumentation to track which entry each query selected
//
// Example extended usage tracking which entry handled each query:
/*
func trackQueryRouting(ctx context.Context, dm *manager.DBManager) {
	for i := 0; i < 5; i++ {
		respCh := dm.Get(ctx, "", "users", []string{"id"}, nil, nil, nil)

		resp := <-respCh
		if resp.Error != nil {
			log.Printf("Query %d: ERROR - %v\n", i+1, resp.Error)
			continue
		}

		// In a real implementation, you could add RequestID tracking
		// to log which database entry handled each query
		log.Printf("Query %d: SUCCESS - RequestID=%s\n", i+1, resp.RequestID)
	}
}
*/
