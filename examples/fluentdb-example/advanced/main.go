package main

import (
	"context"
	"fmt"
	"log"

	db "tounilab.com/fabric/db/v1"
	cdt "tounilab.com/fabric/pkg/query/condition"
)

// Example: Advanced FluentDB queries with JOINs and complex conditions
// This example demonstrates:
// - SELECT with JOINs
// - UPDATE with JOINs
// - DELETE with JOINs
// - Complex WHERE conditions
// - Pagination with OFFSET
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

	// Example 1: SELECT with INNER JOIN
	fmt.Println("=== SELECT with JOINs ===")
	selectWithJoinExample(dbConn, ctx)

	// Example 2: UPDATE with JOIN
	fmt.Println("\n=== UPDATE with JOIN ===")
	updateWithJoinExample(dbConn, ctx)

	// Example 3: DELETE with JOIN
	fmt.Println("\n=== DELETE with JOIN ===")
	deleteWithJoinExample(dbConn, ctx)

	// Example 4: Complex pagination
	fmt.Println("\n=== Pagination Example ===")
	paginationExample(dbConn, ctx)
}

func selectWithJoinExample(conn db.DB, ctx context.Context) {
	// SELECT with INNER JOIN: Get users and their roles
	fmt.Println("\n1. Get users with their role details (INNER JOIN):")
	fdb := db.NewFluentDB(conn, ctx)
	rows, err := fdb.Select("users", "users.id", "users.name", "users.email", "roles.name as role_name").
		Join(cdt.Join{
			Type:  "INNER",
			Table: "roles",
			Alias: "roles",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.role_id",
				Right: "roles.id",
			}},
		}).
		Where(cdt.NewExpr().Column("users.active").Op("=").Value(true)).
		OrderBy("users.name", "ASC").
		Get()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Found %d active users with roles\n", len(rows))
	for _, row := range rows {
		fmt.Printf("  %s (%s) - Role: %v\n", row["name"], row["email"], row["role_name"])
	}

	// SELECT with LEFT JOIN: Get users and their profiles (if any)
	fmt.Println("\n2. Get users with optional profile information (LEFT JOIN):")
	fdb = db.NewFluentDB(conn, ctx)
	rows, err = fdb.Select("users", "users.id", "users.name", "profiles.bio", "profiles.avatar_url").
		Join(cdt.Join{
			Type:  "LEFT",
			Table: "profiles",
			Alias: "profiles",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.id",
				Right: "profiles.user_id",
			}},
		}).
		OrderBy("users.name", "ASC").
		Limit(20).
		Get()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Found %d users with profile information\n", len(rows))
}

func updateWithJoinExample(conn db.DB, ctx context.Context) {
	// UPDATE with JOIN: Update users based on role information
	fmt.Println("\n1. Update users to premium status if they have 'vip' role:")
	fdb := db.NewFluentDB(conn, ctx)
	result, err := fdb.Update("users").
		Set("is_premium", true).
		Set("updated_at", "NOW()").
		Join(cdt.Join{
			Type:  "INNER",
			Table: "roles",
			Alias: "roles",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.role_id",
				Right: "roles.id",
			}},
		}).
		Where(cdt.NewExpr().Column("roles.name").Op("=").Value("vip")).
		Exec()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Update result: %v\n", result)

	// UPDATE with multiple JOINs
	fmt.Println("\n2. Update subscription status for users in specific departments:")
	fdb = db.NewFluentDB(conn, ctx)
	result, err = fdb.Update("users").
		Set("subscription_active", true).
		Join(cdt.Join{
			Type:  "INNER",
			Table: "departments",
			Alias: "depts",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.department_id",
				Right: "depts.id",
			}},
		}).
		Join(cdt.Join{
			Type:  "INNER",
			Table: "subscriptions",
			Alias: "subs",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.id",
				Right: "subs.user_id",
			}},
		}).
		Where(cdt.NewExpr().Column("depts.name").Op("=").Value("engineering")).
		Where(cdt.NewExpr().Column("subs.plan").Op("=").Value("professional")).
		Exec()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Update result: %v\n", result)
}

func deleteWithJoinExample(conn db.DB, ctx context.Context) {
	// DELETE with JOIN: Delete users from deleted accounts
	fmt.Println("\n1. Delete users associated with deleted accounts:")
	fdb := db.NewFluentDB(conn, ctx)
	result, err := fdb.Delete().
		From("users").
		Join(cdt.Join{
			Type:  "INNER",
			Table: "accounts",
			Alias: "accounts",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.account_id",
				Right: "accounts.id",
			}},
		}).
		Where(cdt.NewExpr().Column("accounts.deleted_at").Op("IS NOT").Value(nil)).
		Limit(1000).
		Exec()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Delete result: %v\n", result)

	// DELETE with ordered results: Delete oldest inactive users from specific teams
	fmt.Println("\n2. Delete 50 oldest inactive users from tech team:")
	fdb = db.NewFluentDB(conn, ctx)
	result, err = fdb.Delete().
		From("users").
		Join(cdt.Join{
			Type:  "INNER",
			Table: "teams",
			Alias: "teams",
			Conditions: []cdt.JoinCdt{{
				Left:  "users.team_id",
				Right: "teams.id",
			}},
		}).
		Where(cdt.NewExpr().Column("teams.name").Op("=").Value("technology")).
		Where(cdt.NewExpr().Column("users.last_login").Op("<").Value("2023-01-01")).
		OrderBy("users.last_login", "ASC").
		Limit(50).
		Exec()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Delete result: %v\n", result)
}

func paginationExample(conn db.DB, ctx context.Context) {
	pageSize := 10
	fmt.Printf("\n1. Paginate through users (page size: %d):\n", pageSize)

	for page := 1; page <= 3; page++ {
		offset := (page - 1) * pageSize
		fdb := db.NewFluentDB(conn, ctx)

		rows, err := fdb.Select("users", "id", "name", "email").
			OrderBy("id", "ASC").
			Limit(pageSize).
			Offset(offset).
			Get()
		if err != nil {
			log.Printf("Error: %v", err)
			return
		}

		fmt.Printf("\nPage %d (offset=%d):\n", page, offset)
		if len(rows) == 0 {
			fmt.Println("  No more records")
			break
		}

		for _, row := range rows {
			fmt.Printf("  ID: %v, Name: %s\n", row["id"], row["name"])
		}
	}

	// Pagination with filtering
	fmt.Println("\n2. Paginate active users:")
	fdb := db.NewFluentDB(conn, ctx)
	rows, err := fdb.Select("users", "id", "name", "email").
		Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
		OrderBy("created_at", "DESC").
		Limit(pageSize).
		Offset(0).
		Get()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("First page of active users: %d records\n", len(rows))
}
