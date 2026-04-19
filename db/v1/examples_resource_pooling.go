package v1

import (
	"context"
	"log"
	"sync"
)

// ExampleRowsAdapterPoolBasicUsage demonstrates basic pooling with Acquire/Release.
func ExampleRowsAdapterPoolBasicUsage() {
	// Create a pool (typically once at application startup)
	pool := NewRowsAdapterPool()

	// In a typical query handler:
	// rows, err := db.QueryRaw(ctx, query, args...)
	// adapter, err := pool.Acquire(rows)
	// defer pool.Release(adapter)

	// Use adapter for iteration...
	_ = pool
}

// ExampleRowsAdapterPoolWithStats demonstrates pool statistics monitoring.
func ExampleRowsAdapterPoolWithStats() {
	pool := NewRowsAdapterPoolWithStats()

	// Simulate some acquire/release cycles
	adapter := &RowsAdapter{}
	pool.Release(adapter)

	// Check statistics
	stats := pool.Stats()
	log.Printf("Pool Stats: Allocated=%d, Available=%d\n", stats.Allocated, stats.Available)
}

// ExampleRowsAdapterPoolHighThroughput shows how to use pool in a tight loop.
func ExampleRowsAdapterPoolHighThroughput() {
	// This example shows the pattern for high-throughput scenarios
	pool := NewRowsAdapterPool()

	// Simulate processing many queries
	for i := 0; i < 100; i++ {
		// In real code:
		// rows, err := db.QueryRaw(ctx, "SELECT * FROM table WHERE id = ?", i)
		// adapter, err := pool.Acquire(rows)
		// defer func() {
		//     _ = rows.Close()      // IMPORTANT: Close the underlying rows
		//     pool.Release(adapter) // Then release to pool
		// }()

		_ = pool
	}

	// The pool has reused the same adapter instances throughout!
}

// ExampleManagedRowsAdapterAutomaticCleanup shows automatic resource cleanup.
func ExampleManagedRowsAdapterAutomaticCleanup() {
	// In a function that receives rows from somewhere:
	// managed, err := WrapManagedRowsAdapter(rows)
	// if err != nil {
	//     log.Fatal(err)
	// }
	// defer managed.Close()  // Guaranteed cleanup

	// Get the underlying adapter
	// adapter := managed.Adapter()
	// if adapter != nil {
	//     // Use adapter for iteration
	// }

	// Check if closed
	// if managed.IsClosed() {
	//     log.Println("Adapter already closed")
	// }

	managed, err := WrapManagedRowsAdapter(nil)
	if err != nil {
		_ = err // Expected error in this example
	}
	_ = managed
}

// ExampleScanRowsToRecommended shows the recommended approach.
func ExampleScanRowsToRecommended() {
	// This is the recommended pattern for most users
	type User struct {
		ID    int    `db:"id"`
		Name  string `db:"name"`
		Email string `db:"email"`
	}

	// In real code:
	// rows, err := db.GetRaw(ctx, "users", []string{"id", "name", "email"}, nil, nil, nil)
	// if err != nil {
	//     log.Fatal(err)
	// }
	//
	// users, err := ScanRowsTo[User](context.Background(), rows)
	// if err != nil {
	//     log.Fatal(err)
	// }
	//
	// // rows are automatically closed, users are typed and ready to use
	// for _, user := range users {
	//     log.Printf("User: %s <%s>\n", user.Name, user.Email)
	// }

	_ = context.Background()
	var user User
	_ = user
}

// ExamplePoolConcurrency shows thread-safe pool usage from multiple goroutines.
func ExamplePoolConcurrency() {
	pool := NewRowsAdapterPool()
	var wg sync.WaitGroup

	// Multiple goroutines using the same pool safely
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// In real code:
			// rows, _ := db.QueryRaw(ctx, "SELECT * FROM data WHERE id = ?", id)
			// adapter, _ := pool.Acquire(rows)
			// defer func() {
			//     _ = rows.Close()
			//     pool.Release(adapter)
			// }()

			// Use the pool in real code
			_ = pool
			_ = id
		}(i)
	}

	wg.Wait()
	log.Println("All goroutines completed safely")
}

// ExampleResourceManagementMigrationPath shows the progression of patterns.
func ExampleResourceManagementMigrationPath() {
	log.Println("Pattern 1: Manual (Simplest, works for most)")
	log.Println("  rows, _ := db.GetRaw(ctx, table, cols, joins, cond, opts)")
	log.Println("  defer rows.Close()")

	log.Println("\nPattern 2: ScanRowsTo[T] (Recommended, type-safe)")
	log.Println("  rows, _ := db.GetRaw(ctx, table, cols, joins, cond, opts)")
	log.Println("  users, _ := ScanRowsTo[User](ctx, rows)")

	log.Println("\nPattern 3: Pool (High-throughput loops)")
	log.Println("  pool := NewRowsAdapterPool()")
	log.Println("  adapter, _ := pool.Acquire(rows)")
	log.Println("  defer pool.Release(adapter)")

	log.Println("\nPattern 4: ManagedRowsAdapter (Explicit wrappers)")
	log.Println("  managed, _ := WrapManagedRowsAdapter(rows)")
	log.Println("  defer managed.Close()")
}
