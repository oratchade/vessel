// Package main demonstrates the use of the Explain() method to analyze query execution plans
// across multiple database backends (PostgreSQL, MySQL, SQLite, MSSQL).
package main

import (
	"log"
)

func main() {
	// This example demonstrates query introspection and performance analysis using the Explain() method.
	// The process involves three steps:
	// 1. Generate a query using xxxQuery methods (safe, parameterized SQL)
	// 2. Execute Explain() to analyze the execution plan (non-executing)
	// 3. Read and display the plan results

	// Example database connection would be initialized like:
	// database, err := db.NewDB(db.PostgresConfig{...}, nil)

	log.Println("=== Query Introspection & Performance Analysis Example ===")
	log.Println("This example shows how to use the Explain() method to preview execution plans.")
	log.Println("Note: Explain() does NOT execute the query or retrieve data.")

	// Example 1: Simple SELECT query with WHERE condition
	exampleSimpleSelect()

	// Example 2: Complex query with multiple conditions
	exampleComplexSelect()

	// Example 3: Bulk INSERT analysis
	exampleBulkInsert()

	// Example 4: UPDATE query analysis
	exampleUpdate()
}

// exampleSimpleSelect demonstrates analyzing a simple SELECT query
func exampleSimpleSelect() {
	log.Println("--- Example 1: Simple SELECT Query ---")

	log.Println("Code:")
	log.Print(`
  // Step 1: Generate query using GetQuery()
  query, args, err := database.GetQuery(
      "users",
      []string{"id", "name", "email"},
      nil,  // no joins
      condition.NewExpr().Column("age").Op(">").Value(25),
      nil,  // no options
  )
  if err != nil {
      log.Fatal(err)
  }

  // Output from GetQuery:
  // query = "SELECT id, name, email FROM users WHERE age > ?"
  // args = [25]

  // Step 2: Use Explain() to analyze the execution plan
  ctx := context.Background()
  plan, err := database.Explain(ctx, query, args...)
  if err != nil {
      log.Fatal(err)
  }
  defer plan.Close()

  // Step 3: Read and display the execution plan
  for plan.Next() {
      var line string
      if err := plan.Scan(&line); err != nil {
          log.Fatal(err)
      }
      log.Println(line)
  }
`)

	log.Println("Expected Output (PostgreSQL):")
	log.Print(`
  Seq Scan on users  (cost=0.00..35.50 rows=333 width=32)
    Filter: (age > 25)
  Planning Time: 0.234 ms
  Execution Time: 0.451 ms
`)

	log.Println("Expected Output (MySQL):")
	log.Print(`
  id  select_type  table  partitions  type   possible_keys  key   key_len  ref   rows  filtered  Extra
  1   SIMPLE       users  NULL        ALL    idx_age        NULL  NULL     NULL  1000  50.00     Using where
`)
}

// exampleComplexSelect demonstrates analyzing a complex SELECT with multiple conditions
func exampleComplexSelect() {
	log.Println("--- Example 2: Complex SELECT with Multiple Conditions ---")

	log.Println("Code:")
	log.Print(`
  // Build complex WHERE clause: (age > 25 AND city = 'NYC') OR (salary >= 50000)
  cond := condition.NewOr().Conditions(
      condition.NewAnd().Conditions(
          condition.NewExpr().Column("age").Op(">").Value(25),
          condition.NewExpr().Column("city").Op("=").Value("NYC"),
      ),
      condition.NewExpr().Column("salary").Op(">=").Value(50000),
  )

  // Generate and analyze the query
  query, args, err := database.GetQuery(
      "users",
      []string{"id", "name", "age", "city", "salary"},
      nil,  // no joins
      cond,
      nil,  // no options
  )
  if err != nil {
      log.Fatal(err)
  }

  // query = "SELECT id, name, age, city, salary FROM users WHERE (age > ? AND city = ?) OR (salary >= ?)"
  // args = [25, "NYC", 50000]

  // Analyze with Explain
  ctx := context.Background()
  plan, err := database.Explain(ctx, query, args...)
  if err != nil {
      log.Fatal(err)
  }
  defer plan.Close()

  for plan.Next() {
      var line string
      plan.Scan(&line)
      log.Println(line)
  }
`)

	log.Println("\nBenefits:")
	log.Println("✓ Preview SQL before execution")
	log.Println("✓ Verify parameter binding is correct")
	log.Println("✓ Analyze execution plan without retrieving data")
	log.Println("✓ Identify missing indexes")
	log.Println("✓ Optimize queries before production deployment")
}

// exampleBulkInsert demonstrates analyzing a bulk INSERT query
func exampleBulkInsert() {
	log.Println("--- Example 3: Bulk INSERT Analysis ---")

	log.Println("Code:")
	log.Print(`
  // Prepare bulk data
  data := []map[string]any{
      {"id": 1, "name": "Alice", "email": "alice@example.com"},
      {"id": 2, "name": "Bob", "email": "bob@example.com"},
      {"id": 3, "name": "Charlie", "email": "charlie@example.com"},
  }

  // Generate the bulk INSERT query
  query, args, err := database.InsertsQuery("users", data, nil)
  if err != nil {
      log.Fatal(err)
  }

  // query = "INSERT INTO users (id, name, email) VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?);"
  // args = [1, "Alice", "alice@example.com", 2, "Bob", "bob@example.com", 3, "Charlie", "charlie@example.com"]

  // Preview the execution plan
  ctx := context.Background()
  plan, err := database.Explain(ctx, query, args...)
  if err != nil {
      log.Fatal(err)
  }
  defer plan.Close()

  for plan.Next() {
      var line string
      plan.Scan(&line)
      log.Println(line)
  }
`)

	log.Println("\nBenefits:")
	log.Println("✓ Validate bulk INSERT query generation")
	log.Println("✓ Check how database handles multi-row inserts")
	log.Println("✓ Identify potential constraint violations before execution")
	log.Println("✓ Safe parameterization prevents SQL injection")
}

// exampleUpdate demonstrates analyzing an UPDATE query
func exampleUpdate() {
	log.Println("--- Example 4: UPDATE Query Analysis ---")

	log.Println("Code:")
	log.Print(`
  // Build UPDATE query to increase salary for experienced staff
  cond := condition.NewExpr().Column("years_experience").Op(">=").Value(5)

  query, args, err := database.UpdateQuery(
      "employees",
      map[string]any{"salary": 75000, "level": "senior"},
      cond,
      nil,  // no options
  )
  if err != nil {
      log.Fatal(err)
  }

  // query = "UPDATE employees SET salary = ?, level = ? WHERE years_experience >= ?"
  // args = [75000, "senior", 5]

  // Preview the execution plan
  ctx := context.Background()
  plan, err := database.Explain(ctx, query, args...)
  if err != nil {
      log.Fatal(err)
  }
  defer plan.Close()

  for plan.Next() {
      var line string
      plan.Scan(&line)
      log.Println(line)
  }

  // Once confident, execute the actual UPDATE:
  // result, err := database.Update(ctx, "employees", map[string]any{"salary": 75000, "level": "senior"}, cond, nil)
`)

	log.Println("\nBenefits:")
	log.Println("✓ Verify UPDATE query correctness before execution")
	log.Println("✓ Check if WHERE condition uses indexes")
	log.Println("✓ Estimate number of rows affected")
	log.Println("✓ Identify potential performance issues")
}
