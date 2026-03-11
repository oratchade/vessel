package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	db "tounilab.com/fabric/db/v1"
)

type User struct {
	ID     int
	Name   string
	Email  string
	Age    int
	Status string
}

//nolint:forbidigo,cyclop
func main() {
	cfgs := []db.DBConfig{
		db.PostgresConfig{
			User:           "test_user",
			Password:       "test_password",
			Host:           "localhost",
			Port:           5432,
			Database:       "test_db",
			SSLMode:        "disable",
			ConnectTimeout: 10 * time.Second,
			PoolMaxConns:   10,
			PoolMinConns:   2,
		},
		db.MysqlConfig{
			User:            "root",
			Password:        "root_password",
			Host:            "localhost",
			Port:            3306,
			Database:        "test_db",
			Charset:         "utf8mb4",
			ParseTime:       true,
			Loc:             "Local",
			Timeout:         10 * time.Second,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			MaxOpenConns:    10,
			MaxIdleConns:    2,
			ConnMaxLifetime: 0,
		},
		db.SQLiteConfig{
			FilePath:        ":memory:",
			CacheMode:       "shared",
			Mode:            "memory",
			ForeignKeys:     true,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 0,
		},
	}

	for _, cfg := range cfgs {
		d, err := db.NewDB(cfg, nil)
		if err != nil {
			panic(err)
		}
		defer func() { _ = d.Close() }()

		if strings.Contains(cfg.DSN(), `&mode="memory"`) {
			if err := setupSQLiteTestDB(d); err != nil {
				panic(fmt.Sprintf("Failed to set up SQLite test DB: %v", err))
			}
		}

		users, err := d.Get(
			context.Background(),
			"users",
			[]string{"id", "name", "email", "age", "status"},
			nil,
			nil,
			nil,
		)
		if err != nil {
			panic(fmt.Sprintf("Get failed: %v", err))
		}

		if len(users) == 0 {
			panic("Expected to find users, but found none")
		}

		for _, u := range users {
			fmt.Printf("User: %+v\n", u)
		}

		usersRaw, err := d.GetRaw(
			context.Background(),
			"users",
			[]string{"*"},
			nil,
			nil,
			nil,
		)
		if err != nil {
			panic(fmt.Sprintf("GetRaw failed: %v", err))
		}

		uList, err := db.ScanRowsTo[User](context.Background(), usersRaw)
		if err != nil {
			panic(fmt.Sprintf("ScanRowsTo failed: %v", err))
		}

		for _, u := range uList {
			fmt.Printf("User: %+v\n", u)
		}
	}
}

func setupSQLiteTestDB(database db.DB) error {
	ctx := context.Background()

	// Create tables
	schema := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			age INTEGER,
			status TEXT DEFAULT 'active'
		)`,
		`CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT,
			published INTEGER DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, stmt := range schema {
		if _, err := database.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create tables in sqlite: %w", err)
		}
	}

	// Seed data
	users := []map[string]any{
		{"name": "Alice Johnson", "email": "alice@example.com", "age": 28, "status": "active"},
		{"name": "Bob Smith", "email": "bob@example.com", "age": 34, "status": "active"},
		{"name": "Charlie Davis", "email": "charlie@example.com", "age": 45, "status": "inactive"},
		{"name": "Diana Wilson", "email": "diana@example.com", "age": 29, "status": "active"},
		{"name": "Eve Martinez", "email": "eve@example.com", "age": 31, "status": "active"},
	}

	if _, err := database.Inserts(ctx, "users", users, nil); err != nil {
		return fmt.Errorf("failed to insert users: %w", err)
	}

	return nil
}
