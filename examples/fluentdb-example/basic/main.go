package main

import (
	"context"
	"fmt"
	"log"

	db "tounilab.com/vessel/db/v1"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

// Example: Basic CRUD operations using FluentDB builder API
// This example demonstrates:
// - SELECT queries with WHERE, ORDER BY, LIMIT
// - INSERT single and bulk operations
// - UPDATE with conditions
// - DELETE with filters
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

	// Example 1: SELECT - Retrieve users
	fmt.Println("=== SELECT Examples ===")
	selectExample(dbConn, ctx)

	// Example 2: INSERT - Add users
	fmt.Println("\n=== INSERT Examples ===")
	insertExample(dbConn, ctx)

	// Example 3: UPDATE - Modify users
	fmt.Println("\n=== UPDATE Examples ===")
	updateExample(dbConn, ctx)

	// Example 4: DELETE - Remove users
	fmt.Println("\n=== DELETE Examples ===")
	deleteExample(dbConn, ctx)
}

func selectExample(conn db.DB, ctx context.Context) {
	fdb := db.NewFluentDB(conn)

	// Basic SELECT: Get all users
	fmt.Println("\n1. Get all users:")
	rows, err := fdb.Select("users", "id", "name", "email", "created_at").Get(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Found %d users\n", len(rows))
	for _, row := range rows {
		fmt.Printf("  ID: %v, Name: %s, Email: %s\n", row["id"], row["name"], row["email"])
	}

	// SELECT with WHERE: Get specific user
	fmt.Println("\n2. Get user with ID = 1:")
	fdb = db.NewFluentDB(conn)
	row, err := fdb.Select("users", "id", "name", "email").
		Where(cdt.NewExpr().Column("id").Op("=").Value(int64(1))).
		One(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if row != nil {
		fmt.Printf("User: %s (%s)\n", row["name"], row["email"])
	}

	// SELECT with ORDER BY and LIMIT: Get first 10 users sorted by name
	fmt.Println("\n3. Get first 10 users sorted by name:")
	fdb = db.NewFluentDB(conn)
	rows, err = fdb.Select("users", "id", "name").
		OrderBy("name", "ASC").
		Limit(10).
		Get(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Found %d users\n", len(rows))

	// SELECT with multiple WHERE conditions
	fmt.Println("\n4. Get active users with specific role:")
	fdb = db.NewFluentDB(conn)
	rows, err = fdb.Select("users", "id", "name", "role").
		Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
		Where(cdt.NewExpr().Column("role").Op("=").Value("admin")).
		OrderBy("name", "ASC").
		Get(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Found %d admin users\n", len(rows))

	// COUNT: Get total number of users
	fmt.Println("\n5. Count total users:")
	fdb = db.NewFluentDB(conn)
	count, err := fdb.Select("users").Count(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Total users: %d\n", count)
}

func insertExample(conn db.DB, ctx context.Context) {
	// INSERT single row
	fmt.Println("\n1. Insert single user:")
	fdb := db.NewFluentDB(conn)
	result, err := fdb.Insert().
		Into("users").
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Set("role", "user").
		Set("active", true).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Insert result: %v\n", result)

	// INSERT with map
	fmt.Println("\n2. Insert user using SetMap:")
	fdb = db.NewFluentDB(conn)
	userData := map[string]any{
		"name":   "Jane Smith",
		"email":  "jane@example.com",
		"role":   "admin",
		"active": true,
	}
	result, err = fdb.Insert().
		Into("users").
		SetMap(userData).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Insert result: %v\n", result)

	// BULK INSERT: Add multiple users at once
	fmt.Println("\n3. Bulk insert multiple users:")
	fdb = db.NewFluentDB(conn)
	users := []map[string]any{
		{"name": "Alice", "email": "alice@example.com", "role": "user", "active": true},
		{"name": "Bob", "email": "bob@example.com", "role": "user", "active": true},
		{"name": "Charlie", "email": "charlie@example.com", "role": "moderator", "active": false},
	}
	result, err = fdb.Insert().
		Into("users").
		ValuesBulk(users).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Bulk insert result: %v\n", result)
}

func updateExample(conn db.DB, ctx context.Context) {
	// UPDATE single field
	fmt.Println("\n1. Update user name:")
	fdb := db.NewFluentDB(conn)
	result, err := fdb.Update("users").
		Set("name", "Jonathan Doe").
		Where(cdt.NewExpr().Column("id").Op("=").Value(int64(1))).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Update result: %v\n", result)

	// UPDATE multiple fields with SetMap
	fmt.Println("\n2. Update multiple fields using SetMap:")
	fdb = db.NewFluentDB(conn)
	updates := map[string]any{
		"name":   "Jane Smith Updated",
		"role":   "super_admin",
		"active": false,
	}
	result, err = fdb.Update("users").
		SetMap(updates).
		Where(cdt.NewExpr().Column("email").Op("=").Value("jane@example.com")).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Update result: %v\n", result)

	// UPDATE with multiple WHERE conditions
	fmt.Println("\n3. Activate all inactive users created before specific date:")
	fdb = db.NewFluentDB(conn)
	result, err = fdb.Update("users").
		Set("active", true).
		Where(cdt.NewExpr().Column("active").Op("=").Value(false)).
		Where(cdt.NewExpr().Column("created_at").Op("<").Value("2024-01-01")).
		Limit(100).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Update result: %v\n", result)
}

func deleteExample(conn db.DB, ctx context.Context) {
	// DELETE single row
	fmt.Println("\n1. Delete user by ID:")
	fdb := db.NewFluentDB(conn)
	result, err := fdb.Delete().
		From("users").
		Where(cdt.NewExpr().Column("id").Op("=").Value(int64(999))).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Delete result: %v\n", result)

	// DELETE with multiple conditions
	fmt.Println("\n2. Delete inactive users not created recently:")
	fdb = db.NewFluentDB(conn)
	result, err = fdb.Delete().
		From("users").
		Where(cdt.NewExpr().Column("active").Op("=").Value(false)).
		Where(cdt.NewExpr().Column("created_at").Op("<").Value("2023-01-01")).
		Limit(50).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Delete result: %v\n", result)

	// DELETE with ORDER BY and LIMIT: Delete oldest inactive users
	fmt.Println("\n3. Delete 10 oldest inactive users:")
	fdb = db.NewFluentDB(conn)
	result, err = fdb.Delete().
		From("users").
		Where(cdt.NewExpr().Column("active").Op("=").Value(false)).
		OrderBy("created_at", "ASC").
		Limit(10).
		Exec(ctx)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Delete result: %v\n", result)
}
