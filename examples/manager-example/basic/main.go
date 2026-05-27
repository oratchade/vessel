package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	dbv1 "tounilab.com/vessel/db/v1"
	v1 "tounilab.com/vessel/manager/v1"
	"tounilab.com/vessel/pkg/query/condition"
)

// Example: Basic DBManager Usage
//
// This example demonstrates:
// - Loading configuration from YAML file
// - Starting manager and worker pools
// - Executing synchronous queries using the primary API (Get, Insert, Update, Delete)
// - Handling errors with direct error return values
// - Graceful shutdown
//
// Prerequisites:
// - PostgreSQL running on localhost:5432, 5433, 5434
// - Database "myapp" created
// - Table "users" with columns: id, name, email, created_at
//
// To run:
//   go run basic.go config.yaml

func main() {
	ctx := context.Background()

	// Get config file path from command line or use default
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Initialize DBManager from configuration file
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	adapter := dbv1.NewSlogAdapter(logger)
	dm, err := v1.NewDBManager(ctx, configPath, adapter)
	if err != nil {
		log.Fatalf("Failed to create DBManager: %v", err)
	}

	// Start workers and health checks
	dm.Start()

	// Clean shutdown on exit
	defer dm.Stop()

	// Give workers time to initialize
	time.Sleep(100 * time.Millisecond)

	log.Println("=== DBManager Basic Example ===")

	// Example 1: Insert user
	log.Println("1. Inserting user...")
	insertExample(ctx, dm)

	// Example 2: Get users
	log.Println("\n2. Fetching users...")
	getExample(ctx, dm)

	// Example 3: Update user
	log.Println("\n3. Updating user...")
	updateExample(ctx, dm)

	// Example 4: Delete user
	log.Println("\n4. Deleting user...")
	deleteExample(ctx, dm)

	log.Println("\n=== Example Complete ===")
}

// insertExample demonstrates inserting a single row synchronously
func insertExample(ctx context.Context, dm *v1.DBManager) {
	// Execute synchronous insert query
	result, err := dm.Insert(ctx, "users", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
	}, nil)
	// Check error first
	if err != nil {
		log.Printf("Insert error: %v\n", err)
		return
	}

	// Use result directly
	//nolint:gosec
	log.Printf("✓ Rows=%d\n", result.RowsAffected)
}

// getExample demonstrates fetching multiple rows synchronously
func getExample(ctx context.Context, dm *v1.DBManager) {
	// Build condition: WHERE age IS NOT NULL (or just all users)
	var cond condition.Condition

	// Execute synchronous get query
	data, err := dm.Get(ctx, "users", []string{"id", "name", "email"}, nil, cond, nil)
	// Check error first
	if err != nil {
		log.Printf("Get error: %v\n", err)
		return
	}

	// Use data directly
	//nolint:gosec
	log.Printf("✓ Found %d users:\n", len(data))
	for _, row := range data {
		//nolint:gosec
		log.Printf("  - ID: %v, Name: %v, Email: %v\n", row["id"], row["name"], row["email"])
	}
}

// updateExample demonstrates updating rows synchronously
func updateExample(ctx context.Context, dm *v1.DBManager) {
	// Build condition: WHERE id = 1
	cond := condition.NewExpr().Column("id").Op("=").Value(1)

	// Execute synchronous update query
	result, err := dm.Update(ctx, "users", map[string]any{
		"email": "alice.updated@example.com",
	}, nil, cond, nil)
	// Check error first
	if err != nil {
		log.Printf("Update error: %v\n", err)
		return
	}

	// Use result directly
	//nolint:gosec
	log.Printf("✓ Updated %d rows\n", result.RowsAffected)
}

// deleteExample demonstrates deleting rows synchronously
func deleteExample(ctx context.Context, dm *v1.DBManager) {
	// Build condition: WHERE name = 'Alice'
	cond := condition.NewExpr().Column("name").Op("=").Value("Alice")

	// Execute synchronous delete query
	result, err := dm.Delete(ctx, "users", nil, cond, nil)
	// Check error first
	if err != nil {
		log.Printf("Delete error: %v\n", err)
		return
	}

	// Use result directly
	//nolint:gosec
	log.Printf("✓ Deleted %d rows\n", result.RowsAffected)
}
