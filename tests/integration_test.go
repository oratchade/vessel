//go:build integration

//nolint:testpackage
package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	v1 "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/pkg/query/condition"
)

// getEnv retrieves environment variable with fallback default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvUint16(key string, defaultVal uint16) uint16 {
	if val := os.Getenv(key); val != "" {
		parsed, err := strconv.Atoi(val)
		if err == nil {
			return uint16(parsed)
		}
	}
	return defaultVal
}

const (
	inactiveStatus = "inactive"
	activeStatus   = "active"
)

type TestDB struct {
	name    string
	config  any
	driver  string
	setupFn func(*testing.T, any) // Function to setup DB-specific schema
}

type integrationAvailability struct {
	checked bool
	err     error
}

type User struct {
	ID     int
	Name   string
	Email  string
	Age    int
	Status string
}

// List of test databases
//
//nolint:gochecknoglobals
var testDatabases = []TestDB{
	{
		name:   "SQLite",
		driver: "sqlite3",
		config: v1.SQLiteConfig{
			FilePath:        ":memory:",
			CacheMode:       "shared",
			Mode:            "memory",
			ForeignKeys:     true,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 0,
		},
		setupFn: setupSQLiteTestDB,
	},
	{
		name:   "MySQL",
		driver: "mysql",
		config: v1.MysqlConfig{
			User:            getEnv("DB_MYSQL_USER", "root"),
			Password:        getEnv("DB_MYSQL_PASSWORD", "root_password"),
			Host:            getEnv("DB_MYSQL_HOST", "localhost"),
			Port:            getEnvUint16("DB_MYSQL_PORT", 3306),
			Database:        getEnv("DB_MYSQL_DATABASE", "test_db"),
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
		setupFn: setupMySQLTestDB,
	},
	{
		name:   "PostgreSQL",
		driver: "postgres",
		config: v1.PostgresConfig{
			User:           getEnv("DB_POSTGRES_USER", "test_user"),
			Password:       getEnv("DB_POSTGRES_PASSWORD", "test_password"),
			Host:           getEnv("DB_POSTGRES_HOST", "localhost"),
			Port:           getEnvUint16("DB_POSTGRES_PORT", 5432),
			Database:       getEnv("DB_POSTGRES_DATABASE", "test_db"),
			SSLMode:        "disable",
			ConnectTimeout: 10 * time.Second,
			PoolMaxConns:   10,
			PoolMinConns:   2,
		},
		setupFn: setupPostgresTestDB,
	},
	{
		name:   "MSSQL",
		driver: "sqlserver",
		config: v1.MSSQLConfig{
			User:            getEnv("DB_MSSQL_USER", "sa"),
			Password:        getEnv("DB_MSSQL_PASSWORD", "TestPassword123!"),
			Host:            getEnv("DB_MSSQL_HOST", "localhost"),
			Port:            getEnvUint16("DB_MSSQL_PORT", 1433),
			Database:        getEnv("DB_MSSQL_DATABASE", "test_db"),
			Encrypt:         "disable",
			TrustServerCert: true,
			MaxOpenConns:    10,
			MaxIdleConns:    2,
		},
		setupFn: setupMSSQLTestDB,
	},
}

//nolint:gochecknoglobals
var integrationAvailabilityCache = struct {
	sync.Mutex
	byDriver map[string]integrationAvailability
}{
	byDriver: make(map[string]integrationAvailability),
}

// getFilteredDatabases returns test databases filtered by DB_TYPE environment variable
// If DB_TYPE is not set, returns all databases
// If DB_TYPE is set, returns only the matching database(s)
func getFilteredDatabases() []TestDB {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		return testDatabases
	}

	filtered := []TestDB{}
	for _, db := range testDatabases {
		switch dbType {
		case "sqlite", "sqlite3":
			if db.driver == "sqlite3" {
				filtered = append(filtered, db)
			}
		case "mysql":
			if db.driver == "mysql" {
				filtered = append(filtered, db)
			}
		case "postgres", "postgresql":
			if db.driver == "postgres" {
				filtered = append(filtered, db)
			}
		case "sqlserver", "mssql":
			if db.driver == "sqlserver" {
				filtered = append(filtered, db)
			}
		}
	}
	return filtered
}

func integrationStrict() bool {
	switch strings.ToLower(os.Getenv("FABRIC_INTEGRATION_STRICT")) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func integrationDBExplicitlyRequested() bool {
	return os.Getenv("DB_TYPE") != ""
}

func connectIntegrationDB(t *testing.T, testDB TestDB) v1.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	integrationAvailabilityCache.Lock()
	cached := integrationAvailabilityCache.byDriver[testDB.driver]
	integrationAvailabilityCache.Unlock()
	if cached.checked && cached.err != nil {
		handleIntegrationUnavailable(t, testDB, cached.err)
	}

	if testDB.driver == "sqlserver" {
		if err := ensureMSSQLTestDatabase(testDB.config.(v1.MSSQLConfig)); err != nil {
			cacheIntegrationAvailability(testDB.driver, err)
			handleIntegrationUnavailable(t, testDB, err)
		}
	}

	var database v1.DB
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		database, err = v1.NewDB(testDB.config.(v1.DBConfig), nil)
		if err == nil {
			if pingErr := database.Ping(context.Background()); pingErr == nil {
				cacheIntegrationAvailability(testDB.driver, nil)
				return database
			} else {
				_ = database.Close()
				err = pingErr
			}
		}
		time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
	}

	cacheIntegrationAvailability(testDB.driver, err)
	handleIntegrationUnavailable(t, testDB, err)
	return nil
}

func cacheIntegrationAvailability(driver string, err error) {
	integrationAvailabilityCache.Lock()
	defer integrationAvailabilityCache.Unlock()
	integrationAvailabilityCache.byDriver[driver] = integrationAvailability{
		checked: true,
		err:     err,
	}
}

func handleIntegrationUnavailable(t *testing.T, testDB TestDB, err error) {
	t.Helper()

	envPrefix := strings.ToUpper(testDB.driver)
	if testDB.driver == "sqlserver" {
		envPrefix = "MSSQL"
	}
	msg := fmt.Sprintf(
		"%s integration database unavailable: %v. Set DB_%s_* env vars or start the local test service",
		testDB.name,
		err,
		envPrefix,
	)
	if integrationStrict() || integrationDBExplicitlyRequested() {
		t.Fatalf("%s", msg)
	}
	t.Skip(msg)
}

func ensureMSSQLTestDatabase(cfg v1.MSSQLConfig) error {
	masterCfg := cfg
	masterCfg.Database = "master"

	masterDB, err := sql.Open(masterCfg.Driver(), masterCfg.DSN())
	if err != nil {
		return fmt.Errorf("open MSSQL master connection: %w", err)
	}
	defer func() { _ = masterDB.Close() }()

	for attempt := 1; attempt <= 10; attempt++ {
		if err = masterDB.Ping(); err == nil {
			break
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("ping MSSQL master: %w", err)
	}

	dbName := strings.ReplaceAll(cfg.Database, "'", "''")
	identifier := strings.ReplaceAll(cfg.Database, "]", "]]")
	stmt := fmt.Sprintf("IF DB_ID(N'%s') IS NULL CREATE DATABASE [%s]", dbName, identifier)
	if _, err := masterDB.Exec(stmt); err != nil {
		return fmt.Errorf("ensure MSSQL database %q exists: %w", cfg.Database, err)
	}
	return nil
}

// TestSimpleSQLite tests basic SQLite functionality in isolation
func TestSimpleSQLite(t *testing.T) {
	config := v1.SQLiteConfig{
		FilePath:     ":memory:",
		CacheMode:    "shared",
		Mode:         "memory",
		ForeignKeys:  true,
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}

	// Create database
	database, err := v1.NewDB(config, nil)
	if err != nil {
		t.Fatalf("Failed to create SQLite database: %v", err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create table
	createStmt := `CREATE TABLE IF NOT EXISTS test_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	)`
	_, err = database.Exec(ctx, createStmt)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	t.Logf("Table created successfully")

	// Insert data
	data := []map[string]any{
		{"name": "Alice", "email": "alice@test.com"},
		{"name": "Bob", "email": "bob@test.com"},
	}
	result, err := database.Inserts(ctx, "test_users", data, nil)
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}
	t.Logf("Inserted %d rows", result.RowsAffected)

	// Query data
	users, err := database.Get(ctx, "test_users", []string{"id", "name", "email"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get users: %v", err)
	}
	t.Logf("Retrieved %d users", len(users))

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
		for _, user := range users {
			t.Logf("User: %v", user)
		}
	}
}

// TestIntegration_GetAllUsers tests basic GET without WHERE clause
//
//nolint:errcheck
func TestIntegration_GetAllUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			users, err := database.Get(ctx, "users", []string{"id", "name", "email", "age", "status"}, nil, nil, nil)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected to find users, but found none")
			}
		})
	}
}

// TestIntegration_GetWithWhere tests GET with WHERE condition
//
//nolint:cyclop,errcheck
func TestIntegration_GetWithWhere(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			cond := condition.NewExpr().Column("age").Op(">").Value(25)
			users, err := database.Get(ctx, "users", []string{"id", "name", "email", "age", "status"}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}

			for _, user := range users {
				// Different databases return different types for integers
				var age int64
				switch v := user["age"].(type) {
				case int64:
					age = v
				case int32:
					age = int64(v)
				case int16:
					age = int64(v)
				case int:
					age = int64(v)
				case float64:
					age = int64(v)
				default:
					t.Fatalf("Unexpected age type: %T", user["age"])
				}
				if age <= 25 {
					t.Errorf("Expected age > 25, got %d", age)
				}
			}
		})
	}
}

// TestIntegration_BulkInsert tests bulk INSERT (Inserts) operations
//
//nolint:errcheck
func TestIntegration_BulkInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			data := []map[string]any{
				{
					"name":   "Grace Lee",
					"email":  "grace@example.com",
					"age":    27,
					"status": activeStatus,
				},
				{
					"name":   "Henry Brown",
					"email":  "henry@example.com",
					"age":    31,
					"status": activeStatus,
				},
			}

			result, err := database.Inserts(ctx, "users", data, nil)
			if err != nil {
				t.Fatalf("Inserts failed: %v", err)
			}

			if result.RowsAffected != 2 {
				t.Errorf("Expected 2 rows affected, got %d", result.RowsAffected)
			}
		})
	}
}

// TestIntegration_Update tests UPDATE operations
//
//nolint:errcheck
func TestIntegration_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			cond := condition.NewExpr().Column("name").Op("=").Value("Alice Johnson")
			result, err := database.Update(ctx, "users", map[string]any{
				"age": 29,
			}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Update failed: %v", err)
			}

			if result.RowsAffected < 1 {
				t.Errorf("Expected at least 1 row affected, got %d", result.RowsAffected)
			}
		})
	}
}

// TestIntegration_MultipleConditions tests queries with multiple WHERE conditions
//
//nolint:cyclop,errcheck
func TestIntegration_MultipleConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			cond := condition.NewAnd().Conditions(
				condition.NewExpr().Column("age").Op(">=").Value(25),
				condition.NewExpr().Column("age").Op("<=").Value(40),
				condition.NewExpr().Column("status").Op("=").Value(activeStatus),
			)
			users, err := database.Get(ctx, "users", []string{"id", "name", "email", "age"}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}

			for _, user := range users {
				// Different databases return different types for integers
				var age int64
				switch v := user["age"].(type) {
				case int64:
					age = v
				case int32:
					age = int64(v)
				case int16:
					age = int64(v)
				case int:
					age = int64(v)
				case float64:
					age = int64(v)
				default:
					t.Fatalf("Unexpected age type: %T", user["age"])
				}
				if age < 25 || age > 40 {
					t.Errorf("Expected age between 25 and 40, got %d", age)
				}
			}
		})
	}
}

// TestIntegration_Delete tests DELETE operations
//
//nolint:errcheck
func TestIntegration_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			cond := condition.NewExpr().Column("status").Op("=").Value(inactiveStatus)
			result, err := database.Delete(ctx, "users", nil, cond, nil)
			if err != nil {
				t.Fatalf("Delete failed: %v", err)
			}

			if result.RowsAffected < 1 {
				t.Errorf("Expected at least 1 row affected, got %d", result.RowsAffected)
			}
		})
	}
}

// TestIntegration_TransactionCommit tests transaction commit
//
//nolint:errcheck
func TestIntegration_TransactionCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			err := database.WithTransaction(ctx, func(tx v1.Tx) error {
				_, err := tx.Insert(ctx, "users", map[string]any{
					"name":   "Iris Taylor",
					"email":  "iris@example.com",
					"age":    26,
					"status": activeStatus,
				}, nil)
				if err != nil {
					return fmt.Errorf("Insert failed: %w", err)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Transaction failed: %v", err)
			}

			// Verify inserted
			users, err := database.Get(
				ctx,
				"users",
				[]string{"name", "email"},
				nil,
				condition.NewExpr().Column("email").Op("=").Value("iris@example.com"),
				nil,
			)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected to find inserted user after transaction commit")
			}
		})
	}
}

// TestIntegration_GetByID tests GetByID operation
//
//nolint:errcheck
func TestIntegration_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			users, err := database.GetByID(ctx, "users", 1, nil, nil)
			if err != nil {
				t.Fatalf("GetByID failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected to find user with ID 1")
			}
		})
	}
}

// TestIntegration_ConditionalQuery tests query with conditional errors
//
//nolint:errcheck
func TestIntegration_ConditionalQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			// Get users with valid condition
			users, err := database.Get(
				ctx,
				"users",
				[]string{"id", "name"},
				nil,
				condition.NewExpr().Column("age").Op(">").Value(30),
				nil,
			)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}

			// Verify results are valid
			for _, user := range users {
				if user["id"] == nil || user["name"] == nil {
					t.Error("Expected valid user data")
				}
			}
		})
	}
}

// TestIntegration_SingleInsert tests single row INSERT operation
//
//nolint:errcheck
func TestIntegration_SingleInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()
			result, err := database.Insert(ctx, "users", map[string]any{
				"name":   "Frank Wilson",
				"email":  "frank@example.com",
				"age":    35,
				"status": activeStatus,
			}, nil)
			if err != nil {
				t.Fatalf("Insert failed: %v", err)
			}

			if result.RowsAffected != 1 {
				t.Errorf("Expected 1 row affected, got %d", result.RowsAffected)
			}

			// Verify the record was inserted
			users, err := database.Get(
				ctx,
				"users",
				[]string{"email"},
				nil,
				condition.NewExpr().Column("email").Op("=").Value("frank@example.com"),
				nil,
			)
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}

			if len(users) != 1 {
				t.Errorf("Expected 1 inserted user, found %d", len(users))
			}
		})
	}
}

// TestIntegration_TransactionRollback tests transaction rollback
//
//nolint:errcheck
func TestIntegration_TransactionRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Get initial count
			initialUsers, err := database.Get(ctx, "users", []string{"id"}, nil, nil, nil)
			if err != nil {
				t.Fatalf("Initial Get failed: %v", err)
			}
			initialCount := len(initialUsers)

			// Attempt transaction that will rollback (simulate error)
			err = database.WithTransaction(ctx, func(tx v1.Tx) error {
				// Insert a record
				_, errInsert := tx.Insert(ctx, "users", map[string]any{
					"name":   "George Taylor",
					"email":  "george@example.com",
					"age":    32,
					"status": activeStatus,
				}, nil)
				if errInsert != nil {
					return fmt.Errorf("Insert failed: %w", errInsert)
				}
				// Force rollback by returning error
				return fmt.Errorf("simulated error for rollback")
			})

			// err should not be nil (transaction failed)
			if err == nil {
				t.Error("Expected transaction error")
			}

			// Verify rollback happened - count should remain same
			finalUsers, err := database.Get(ctx, "users", []string{"id"}, nil, nil, nil)
			if err != nil {
				t.Fatalf("Final Get failed: %v", err)
			}

			if len(finalUsers) != initialCount {
				t.Errorf("Expected %d users after rollback, got %d", initialCount, len(finalUsers))
			}
		})
	}
}

// TestIntegration_RawQuery tests raw SQL query execution
//
//nolint:errcheck
func TestIntegration_RawQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			rawQuery := "SELECT id, name FROM users LIMIT 1"
			if testDB.driver == "sqlserver" {
				rawQuery = "SELECT TOP 1 id, name FROM users"
			}

			// Test raw query execution (basic)
			rows, err := database.QueryRaw(ctx, rawQuery)
			if err != nil {
				t.Fatalf("QueryRaw failed: %v", err)
			}

			list, err := v1.ScanRowsTo[User](context.Background(), rows)
			if err != nil {
				panic(fmt.Sprintf("ScanRowsTo failed: %v", err))
			}

			if len(list) != 1 {
				t.Errorf("Expected 1 user, found %d", len(list))
			}

			// Also test with Get which will validate the query works
			users, err := database.Get(
				ctx,
				"users",
				[]string{"id", "name", "email"},
				nil,
				condition.NewExpr().Column("age").Op(">").Value(30),
				nil,
			)
			if err != nil {
				t.Fatalf("Verification Get failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected at least one result matching the query")
			}
		})
	}
}

// TestIntegration_OrConditions tests OR conditions
//
//nolint:errcheck,staticcheck
func TestIntegration_OrConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Get users with OR condition: age < 30 OR status = "inactive"
			cond := condition.NewOr().Conditions(
				condition.NewExpr().Column("age").Op("<").Value(30),
				condition.NewExpr().Column("status").Op("=").Value(inactiveStatus),
			)
			users, err := database.Get(ctx, "users", []string{"id", "name", "age", "status"}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Get with OR failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected at least one result from OR condition")
			}

			// Verify results match the condition
			for _, user := range users {
				age := getInt64(user["age"])
				status := user["status"].(string)
				isYoung := age < 30
				isInactive := status == inactiveStatus
				if !(isYoung || isInactive) {
					t.Errorf("Result doesn't match OR condition: age=%d, status=%s", age, status)
				}
			}
		})
	}
}

// TestIntegration_ComplexNestedConditions tests complex nested AND/OR conditions
//
//nolint:errcheck,staticcheck
func TestIntegration_ComplexNestedConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Complex condition: (age >= 28 AND age <= 35) OR status = "inactive"
			cond := condition.NewOr().Conditions(
				condition.NewAnd().Conditions(
					condition.NewExpr().Column("age").Op(">=").Value(28),
					condition.NewExpr().Column("age").Op("<=").Value(35),
				),
				condition.NewExpr().Column("status").Op("=").Value(inactiveStatus),
			)

			users, err := database.Get(ctx, "users", []string{"id", "name", "age", "status"}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Complex condition Get failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected results from complex condition")
			}

			// Verify all results match the condition
			for _, user := range users {
				age := getInt64(user["age"])
				status := user["status"].(string)
				inRange := age >= 28 && age <= 35
				isInactive := status == inactiveStatus
				if !(inRange || isInactive) {
					t.Errorf("Result doesn't match complex condition: age=%d, status=%s", age, status)
				}
			}
		})
	}
}

// TestIntegration_UpdateMultipleRows tests updating multiple rows
//
//nolint:errcheck
func TestIntegration_UpdateMultipleRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Update all users with age > 30 to have status "senior"
			cond := condition.NewExpr().Column("age").Op(">").Value(30)
			result, err := database.Update(ctx, "users", map[string]any{
				"status": "senior",
			}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Update failed: %v", err)
			}

			if result.RowsAffected < 1 {
				t.Errorf("Expected at least 1 row affected, got %d", result.RowsAffected)
			}

			// Verify the updates
			seniors, err := database.Get(
				ctx,
				"users",
				[]string{"id", "status"},
				nil,
				condition.NewExpr().Column("status").Op("=").Value("senior"),
				nil,
			)
			if err != nil {
				t.Fatalf("Verification Get failed: %v", err)
			}

			if len(seniors) != int(result.RowsAffected) {
				t.Errorf("Expected %d senior users, found %d", result.RowsAffected, len(seniors))
			}
		})
	}
}

// TestIntegration_DeleteMultipleRows tests deleting multiple rows
//
//nolint:errcheck
func TestIntegration_DeleteMultipleRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Get initial count
			initialUsers, err := database.Get(ctx, "users", []string{"id"}, nil, nil, nil)
			if err != nil {
				t.Fatalf("Initial Get failed: %v", err)
			}
			initialCount := len(initialUsers)

			// Delete all users with age > 40
			cond := condition.NewExpr().Column("age").Op(">").Value(40)
			result, err := database.Delete(ctx, "users", nil, cond, nil)
			if err != nil {
				t.Fatalf("Delete failed: %v", err)
			}

			// Verify deletion
			finalUsers, err := database.Get(ctx, "users", []string{"id"}, nil, nil, nil)
			if err != nil {
				t.Fatalf("Final Get failed: %v", err)
			}

			expectedFinalCount := initialCount - int(result.RowsAffected)
			if len(finalUsers) != expectedFinalCount {
				t.Errorf("Expected %d users after delete, got %d", expectedFinalCount, len(finalUsers))
			}
		})
	}
}

// TestIntegration_GetByIDRaw tests raw version of GetByID
//
//nolint:errcheck
func TestIntegration_GetByIDRaw(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Get raw rows for user ID 1
			usersRaw, err := database.GetByIDRaw(ctx, "users", 1, nil, nil)
			if err != nil {
				t.Fatalf("GetByIDRaw failed: %v", err)
			}
			u, err := v1.ScanRowsTo[User](context.Background(), usersRaw)
			if err != nil {
				t.Fatalf("ScanRowsTo failed: %v", err)
			}

			if len(u) == 0 {
				t.Error("Expected to find user with ID 1 using GetByIDRaw")
			}

			// Verify with normal GetByID that the user exists
			users, err := database.GetByID(ctx, "users", 1, nil, nil)
			if err != nil {
				t.Fatalf("GetByID verification failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected to find user with ID 1")
			}
		})
	}
}

// TestIntegration_NotEqualOperator tests NOT EQUAL (!=) operator
//
//nolint:errcheck
func TestIntegration_NotEqualOperator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Get all users with status != 'inactive'
			cond := condition.NewExpr().Column("status").Op("!=").Value(inactiveStatus)
			users, err := database.Get(ctx, "users", []string{"id", "name", "status"}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Get with != failed: %v", err)
			}

			if len(users) == 0 {
				t.Error("Expected at least one active user")
			}

			// Verify all results have status != 'inactive'
			for _, user := range users {
				if user["status"].(string) == inactiveStatus {
					t.Error("Found inactive user in results with != condition")
				}
			}
		})
	}
}

// TestIntegration_InOperator tests IN operator with multiple values
//
//nolint:errcheck
func TestIntegration_InOperator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			// Setup test data
			testDB.setupFn(t, database)

			ctx := context.Background()

			// Get users with specific names using IN operator
			cond := condition.NewIn().Column("name").Values("Alice Johnson", "Bob Smith")
			users, err := database.Get(ctx, "users", []string{"id", "name"}, nil, cond, nil)
			if err != nil {
				t.Fatalf("Get with IN failed: %v", err)
			}

			if len(users) != 2 {
				t.Errorf("Expected 2 users with IN condition, got %d", len(users))
			}
		})
	}
}

// Helper function to safely convert value to int64
func getInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case int16:
		return int64(val)
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// Setup functions for each database

// setupSQLiteTestDB creates and seeds SQLite database
func setupSQLiteTestDB(t *testing.T, database any) {
	db := database.(v1.DB)
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
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}
	}

	// Seed data
	users := []map[string]any{
		{"name": "Alice Johnson", "email": "alice@example.com", "age": 28, "status": activeStatus},
		{"name": "Bob Smith", "email": "bob@example.com", "age": 34, "status": activeStatus},
		{"name": "Charlie Davis", "email": "charlie@example.com", "age": 45, "status": inactiveStatus},
		{"name": "Diana Wilson", "email": "diana@example.com", "age": 29, "status": activeStatus},
		{"name": "Eve Martinez", "email": "eve@example.com", "age": 31, "status": activeStatus},
	}

	if _, err := db.Inserts(ctx, "users", users, nil); err != nil {
		t.Fatalf("Failed to insert users: %v", err)
	}
}

// setupMySQLTestDB creates and seeds MySQL database
//
//nolint:dupl
func setupMySQLTestDB(t *testing.T, database any) {
	db := database.(v1.DB)
	ctx := context.Background()

	// Drop and create fresh tables
	schema := []string{
		"DROP TABLE IF EXISTS comments",
		"DROP TABLE IF EXISTS posts",
		"DROP TABLE IF EXISTS users",
		`CREATE TABLE users (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INT,
			status VARCHAR(50) DEFAULT 'active'
		)`,
		`CREATE TABLE posts (
			id INT PRIMARY KEY AUTO_INCREMENT,
			user_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published INT DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE comments (
			id INT PRIMARY KEY AUTO_INCREMENT,
			post_id INT NOT NULL,
			user_id INT NOT NULL,
			content TEXT NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Logf("Schema statement: %s", stmt)
			t.Fatalf("Failed to create table: %v", err)
		}
	}

	// Seed data
	users := []map[string]any{
		{"name": "Alice Johnson", "email": "alice@example.com", "age": 28, "status": activeStatus},
		{"name": "Bob Smith", "email": "bob@example.com", "age": 34, "status": activeStatus},
		{"name": "Charlie Davis", "email": "charlie@example.com", "age": 45, "status": inactiveStatus},
		{"name": "Diana Wilson", "email": "diana@example.com", "age": 29, "status": activeStatus},
		{"name": "Eve Martinez", "email": "eve@example.com", "age": 31, "status": activeStatus},
	}

	if _, err := db.Inserts(ctx, "users", users, nil); err != nil {
		t.Fatalf("Failed to insert users: %v", err)
	}
}

// setupPostgresTestDB creates and seeds PostgreSQL database
//
//nolint:dupl
func setupPostgresTestDB(t *testing.T, database any) {
	db := database.(v1.DB)
	ctx := context.Background()

	// Drop and create fresh tables
	schema := []string{
		"DROP TABLE IF EXISTS comments CASCADE",
		"DROP TABLE IF EXISTS posts CASCADE",
		"DROP TABLE IF EXISTS users CASCADE",
		`CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INT,
			status VARCHAR(50) DEFAULT 'active'
		)`,
		`CREATE TABLE posts (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published INT DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE comments (
			id SERIAL PRIMARY KEY,
			post_id INT NOT NULL,
			user_id INT NOT NULL,
			content TEXT NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Logf("Schema statement: %s", stmt)
			t.Fatalf("Failed to create table: %v", err)
		}
	}

	// Seed data
	users := []map[string]any{
		{"name": "Alice Johnson", "email": "alice@example.com", "age": 28, "status": activeStatus},
		{"name": "Bob Smith", "email": "bob@example.com", "age": 34, "status": activeStatus},
		{"name": "Charlie Davis", "email": "charlie@example.com", "age": 45, "status": inactiveStatus},
		{"name": "Diana Wilson", "email": "diana@example.com", "age": 29, "status": activeStatus},
		{"name": "Eve Martinez", "email": "eve@example.com", "age": 31, "status": activeStatus},
	}

	if _, err := db.Inserts(ctx, "users", users, nil); err != nil {
		t.Fatalf("Failed to insert users: %v", err)
	}
}

// setupMSSQLTestDB creates and seeds MSSQL database
//
//nolint:dupl
func setupMSSQLTestDB(t *testing.T, database any) {
	db := database.(v1.DB)
	ctx := context.Background()

	// Drop and create fresh tables
	schema := []string{
		"DROP TABLE IF EXISTS comments",
		"DROP TABLE IF EXISTS posts",
		"DROP TABLE IF EXISTS users",
		`CREATE TABLE users (
			id INT PRIMARY KEY IDENTITY(1,1),
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INT,
			status VARCHAR(50) DEFAULT 'active'
		)`,
		`CREATE TABLE posts (
			id INT PRIMARY KEY IDENTITY(1,1),
			user_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published INT DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE comments (
			id INT PRIMARY KEY IDENTITY(1,1),
			post_id INT NOT NULL,
			user_id INT NOT NULL,
			content TEXT NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Logf("Schema statement: %s", stmt)
			t.Fatalf("Failed to create table: %v", err)
		}
	}

	// Seed data
	users := []map[string]any{
		{"name": "Alice Johnson", "email": "alice@example.com", "age": 28, "status": activeStatus},
		{"name": "Bob Smith", "email": "bob@example.com", "age": 34, "status": activeStatus},
		{"name": "Charlie Davis", "email": "charlie@example.com", "age": 45, "status": inactiveStatus},
		{"name": "Diana Wilson", "email": "diana@example.com", "age": 29, "status": activeStatus},
		{"name": "Eve Martinez", "email": "eve@example.com", "age": 31, "status": activeStatus},
	}

	if _, err := db.Inserts(ctx, "users", users, nil); err != nil {
		t.Fatalf("Failed to insert users: %v", err)
	}
}
