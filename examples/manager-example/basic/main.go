package main

import (
	"context"
	"log"
	"os"
	"time"

	v1 "tounilab.com/fabric/manager/v1"
	"tounilab.com/fabric/pkg/query/condition"
)

// Example: Basic DBManager Usage
//
// This example demonstrates:
// - Loading configuration from YAML file
// - Starting manager and worker pools
// - Executing async queries
// - Handling responses and errors
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
	dm, err := v1.NewDBManager(ctx, configPath)
	if err != nil {
		log.Fatalf("Failed to create DBManager: %v", err)
	}

	// Start workers and health checks
	dm.Start(ctx)

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

// insertExample demonstrates inserting a single row
func insertExample(ctx context.Context, dm *v1.DBManager) {
	// Fire async insert query
	respCh := dm.Insert(ctx, "", "users", map[string]interface{}{
		"name":  "Alice",
		"email": "alice@example.com",
	}, nil)

	// Receive response
	resp := <-respCh

	if resp.Error != nil {
		log.Printf("Insert error: %v\n", resp.Error)
		return
	}

	if resp.ExecData != nil {
		//nolint:gosec
		log.Printf("✓ Inserted: ID=%v, Rows=%d\n", resp.ExecData.LastInsertID, resp.ExecData.RowsAffected)
	}
}

// getExample demonstrates fetching multiple rows
func getExample(ctx context.Context, dm *v1.DBManager) {
	// Build condition: WHERE age IS NOT NULL (or just all users)
	var cond condition.Condition

	// Fire async get query
	respCh := dm.Get(ctx, "", "users", []string{"id", "name", "email"}, nil, cond, nil)

	// Receive response
	resp := <-respCh

	if resp.Error != nil {
		log.Printf("Get error: %v\n", resp.Error)
		return
	}

	//nolint:gosec
	log.Printf("✓ Found %d users:\n", len(resp.Data))
	for _, row := range resp.Data {
		//nolint:gosec
		log.Printf("  - ID: %v, Name: %v, Email: %v\n", row["id"], row["name"], row["email"])
	}
}

// updateExample demonstrates updating rows
func updateExample(ctx context.Context, dm *v1.DBManager) {
	// Build condition: WHERE id = 1
	cond := condition.NewExpr().Column("id").Op("=").Value(1)

	// Fire async update query
	respCh := dm.Update(ctx, "", "users", map[string]interface{}{
		"email": "alice.updated@example.com",
	}, cond, nil, nil)

	// Receive response
	resp := <-respCh

	if resp.Error != nil {
		log.Printf("Update error: %v\n", resp.Error)
		return
	}

	if resp.ExecData != nil {
		//nolint:gosec
		log.Printf("✓ Updated %d rows\n", resp.ExecData.RowsAffected)
	}
}

// deleteExample demonstrates deleting rows
func deleteExample(ctx context.Context, dm *v1.DBManager) {
	// Build condition: WHERE name = 'Alice'
	cond := condition.NewExpr().Column("name").Op("=").Value("Alice")

	// Fire async delete query
	respCh := dm.Delete(ctx, "", "users", cond, nil, nil)

	// Receive response
	resp := <-respCh

	if resp.Error != nil {
		log.Printf("Delete error: %v\n", resp.Error)
		return
	}

	if resp.ExecData != nil {
		//nolint:gosec
		log.Printf("✓ Deleted %d rows\n", resp.ExecData.RowsAffected)
	}
}
