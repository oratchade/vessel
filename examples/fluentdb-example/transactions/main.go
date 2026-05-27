package main

import (
	"context"
	"fmt"
	"log"

	db "tounilab.com/vessel/db/v1"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

// Example: Transaction handling with FluentDB
// This example demonstrates:
// - Starting transactions
// - Using FluentDB within transactions
// - Error handling and rollback
// - Nested transactions
// - Complex multi-operation transactions
func main() {
	// Setup database connection
	dbConn, err := db.NewDB(db.PostgresConfig{
		User:     "postgres",
		Password: "password",
		Host:     "localhost",
		Port:     5432,
		Database: "myapp",
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = dbConn.Close() }()

	ctx := context.Background()

	// Example 1: Simple transaction
	fmt.Println("=== Simple Transaction ===")
	simpleTransaction(dbConn, ctx)

	// Example 2: Transaction with rollback on error
	fmt.Println("\n=== Transaction with Error Handling ===")
	transactionWithError(dbConn, ctx)

	// Example 3: Complex multi-step transaction
	fmt.Println("\n=== Complex Multi-Step Transaction ===")
	complexTransaction(dbConn, ctx)

	// Example 4: Transaction with nested operations
	fmt.Println("\n=== Transaction with Nested Operations ===")
	nestedOperations(dbConn, ctx)
}

func simpleTransaction(conn db.DB, ctx context.Context) {
	fmt.Println("\n1. Simple INSERT within transaction:")

	// Start transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}

	// Use FluentDB with transaction
	fdb := db.NewFluentDB(tx)

	// Insert user
	_, err = fdb.Insert().
		Into("users").
		Set("name", "Transaction User").
		Set("email", "tx@example.com").
		Set("active", true).
		Exec(ctx)
	if err != nil {
		log.Printf("Error inserting user: %v", err)
		if err := tx.Rollback(ctx); err != nil {
			log.Printf("Error rolling back: %v", err)
		}
		return
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing: %v", err)
		return
	}

	fmt.Println("  ✓ Transaction committed successfully")
}

func transactionWithError(conn db.DB, ctx context.Context) {
	fmt.Println("\n1. Transaction with error handling:")

	// Start transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic recovered: %v", r)
			if err := tx.Rollback(ctx); err != nil {
				log.Printf("Error rolling back: %v", err)
			}
		}
	}()

	fdb := db.NewFluentDB(tx)

	// Step 1: Insert user
	fmt.Println("  Step 1: Inserting user...")
	_, err = fdb.Insert().
		Into("users").
		Set("name", "Test User").
		Set("email", "test@example.com").
		Exec(ctx)
	if err != nil {
		log.Printf("  Error at step 1: %v. Rolling back...", err)
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			log.Printf("  Error rolling back: %v", rollbackErr)
		}
		return
	}
	fmt.Println("  ✓ Step 1 complete")

	// Step 2: Insert related data
	fmt.Println("  Step 2: Inserting profile...")
	_, err = fdb.Insert().
		Into("profiles").
		Set("user_id", 1).
		Set("bio", "Test bio").
		Exec(ctx)
	if err != nil {
		log.Printf("  Error at step 2: %v. Rolling back...", err)
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			log.Printf("  Error rolling back: %v", rollbackErr)
		}
		return
	}
	fmt.Println("  ✓ Step 2 complete")

	// All steps successful, commit
	if err := tx.Commit(ctx); err != nil {
		log.Printf("  Error committing: %v", err)
		return
	}
	fmt.Println("  ✓ Transaction committed successfully")
}

func complexTransaction(conn db.DB, ctx context.Context) {
	fmt.Println("\n1. Complex multi-step transaction (Transfer operation):")
	fmt.Println("  Scenario: Transfer credits from user A to user B")

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}

	fdb := db.NewFluentDB(tx)

	// Step 1: Deduct credits from sender
	fmt.Println("  Step 1: Deducting 100 credits from sender...")
	_, err = fdb.Update("accounts").
		Set("balance", `balance - 100`).
		Where(cdt.NewExpr().Column("user_id").Op("=").Value(int64(1))).
		Exec(ctx)
	if err != nil {
		log.Printf("  Error at step 1: %v. Rolling back...", err)
		_ = tx.Rollback(ctx)
		return
	}

	// Step 2: Add credits to receiver
	fmt.Println("  Step 2: Adding 100 credits to receiver...")
	_, err = fdb.Update("accounts").
		Set("balance", `balance + 100`).
		Where(cdt.NewExpr().Column("user_id").Op("=").Value(int64(2))).
		Exec(ctx)
	if err != nil {
		log.Printf("  Error at step 2: %v. Rolling back...", err)
		_ = tx.Rollback(ctx)
		return
	}

	// Step 3: Log transaction
	fmt.Println("  Step 3: Logging transaction...")
	_, err = fdb.Insert().
		Into("transaction_logs").
		Set("from_user_id", int64(1)).
		Set("to_user_id", int64(2)).
		Set("amount", 100).
		Set("type", "transfer").
		Set("timestamp", "NOW()").
		Exec(ctx)
	if err != nil {
		log.Printf("  Error at step 3: %v. Rolling back...", err)
		_ = tx.Rollback(ctx)
		return
	}

	// Commit all changes atomically
	if err := tx.Commit(ctx); err != nil {
		log.Printf("  Error committing: %v", err)
		return
	}

	fmt.Println("  ✓ Transfer completed and committed")
}

func nestedOperations(conn db.DB, ctx context.Context) {
	fmt.Println("\n1. Transaction with conditional logic:")

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}

	fdb := db.NewFluentDB(tx)

	// Step 1: Check if user exists
	fmt.Println("  Step 1: Checking if user exists...")
	rows, err := fdb.Select("users", "id", "email").
		Where(cdt.NewExpr().Column("email").Op("=").Value("batch@example.com")).
		Get(ctx)
	if err != nil {
		log.Printf("  Error checking user: %v", err)
		_ = tx.Rollback(ctx)
		return
	}

	if len(rows) == 0 {
		fmt.Println("  Step 2a: User not found, creating new user...")
		_, err = fdb.Insert().
			Into("users").
			Set("email", "batch@example.com").
			Set("name", "Batch User").
			Set("active", true).
			Exec(ctx)
		if err != nil {
			log.Printf("  Error creating user: %v", err)
			_ = tx.Rollback(ctx)
			return
		}

		// Get newly created user ID
		newRows, err := fdb.Select("users", "id").
			Where(cdt.NewExpr().Column("email").Op("=").Value("batch@example.com")).
			One(ctx)
		if err != nil {
			log.Printf("  Error fetching new user: %v", err)
			_ = tx.Rollback(ctx)
			return
		}

		fmt.Printf("  Created new user with ID: %v\n", newRows["id"])
	} else {
		fmt.Printf("  Step 2b: User found with ID: %v\n", rows[0]["id"])
		userID := rows[0]["id"]

		// Update existing user
		fmt.Println("  Updating user last_login...")
		_, err = fdb.Update("users").
			Set("last_login", "NOW()").
			Where(cdt.NewExpr().Column("id").Op("=").Value(userID)).
			Exec(ctx)
		if err != nil {
			log.Printf("  Error updating user: %v", err)
			_ = tx.Rollback(ctx)
			return
		}
	}

	// Step 3: Delete old records
	fmt.Println("  Step 3: Cleaning up old records...")
	_, err = fdb.Delete().
		From("audit_logs").
		Where(cdt.NewExpr().Column("created_at").Op("<").Value("2023-01-01")).
		Limit(10000).
		Exec(ctx)
	if err != nil {
		log.Printf("  Error deleting old records: %v", err)
		_ = tx.Rollback(ctx)
		return
	}

	// Commit all changes
	if err := tx.Commit(ctx); err != nil {
		log.Printf("  Error committing: %v", err)
		return
	}

	fmt.Println("  ✓ All operations completed and committed")
}
