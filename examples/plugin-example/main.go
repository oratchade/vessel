package main

import (
	"context"
	"log"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/db/v1/plugin"

	// Blank import registers the CockroachDB plugin via init()
	"tounilab.com/vessel/examples/plugin-example/cockroachdb"
	_ "tounilab.com/vessel/examples/plugin-example/cockroachdb"
)

func main() {
	ctx := context.Background()

	// Print available drivers (includes our plugin)
	for _, driver := range plugin.List() {
		log.Printf("Registered driver: %s\n", driver)
	}

	// Create CockroachDB config
	cfg := &cockroachdb.Config{
		Host:     "localhost",
		Port:     26257,
		User:     "root",
		Password: "password",
		Database: "testdb",
		SSLMode:  "require",
	}

	// Create database connection using NewDB
	// NewDB checks the plugin registry first, then falls back to built-in drivers
	database, err := db.NewDB(cfg, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Printf("Error closing database: %v\n", closeErr)
		}
	}()

	// Verify connection
	if err := database.Ping(ctx); err != nil {
		log.Printf("Ping failed: %v\n", err)
		return
	}
	log.Println("✅ Connected to CockroachDB")

	// Example: Insert a user
	result, err := database.Insert(ctx, "users", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
	}, nil)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
	} else {
		log.Printf("Inserted %d rows\n", result.RowsAffected)
	}

	// Example: Query users
	users, err := database.Get(ctx, "users", []string{"id", "name", "email"}, nil, nil, nil)
	if err != nil {
		log.Printf("Query failed: %v\n", err)
	} else {
		log.Printf("Found %d users:\n", len(users))
		for _, user := range users {
			log.Printf("  %v\n", user)
		}
	}

	// Example: Get connection pool stats
	stats, err := database.PoolStats()
	if err == nil {
		log.Printf("Pool stats: Open=%d, InUse=%d, Idle=%d\n",
			stats.OpenConnections, stats.InUse, stats.Idle)
	}

	log.Println("✅ Plugin example complete")
}
