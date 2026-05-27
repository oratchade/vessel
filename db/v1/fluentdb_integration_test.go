//go:build integration

package v1_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/vessel/db/v1"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

const (
	fluentUsersTable = "fluent_users"
	fluentPostsTable = "fluent_posts"
)

// IntegrationTest provides common setup for integration tests
type IntegrationTest struct {
	db  v1.DB
	ctx context.Context
}

// setupIntegration creates a test database connection
// Requires docker-compose.test.yml to be running:
// cd vessel && docker-compose -f docker-compose.test.yml up
func setupIntegration(t *testing.T) *IntegrationTest {
	t.Helper()

	dbType := strings.ToLower(os.Getenv("DB_TYPE"))
	if dbType != "" && dbType != "postgres" && dbType != "postgresql" {
		t.Skipf("PostgreSQL-only FluentDB scenario skipped because DB_TYPE=%s", dbType)
	}

	ctx := context.Background()

	cfg := &v1.PostgresConfig{
		User:           "test_user",
		Password:       "test_password",
		Host:           "localhost",
		Port:           5432,
		Database:       "test_db",
		SSLMode:        "disable",
		ConnectTimeout: 10 * time.Second,
		PoolMaxConns:   10,
		PoolMinConns:   2,
	}

	db, err := v1.NewDB(cfg, nil)
	if err != nil {
		if fluentStrict() || dbType != "" {
			require.NoError(t, err, "failed to connect to test database")
		}
		t.Skipf("PostgreSQL integration database unavailable: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		_ = db.Close()
		if fluentStrict() || dbType != "" {
			require.NoError(t, err, "failed to ping test database")
		}
		t.Skipf("PostgreSQL integration database unavailable: %v", err)
	}
	require.NotNil(t, db)

	setupFluentIntegrationSchema(t, ctx, db)

	return &IntegrationTest{db: db, ctx: ctx}
}

// cleanup closes the database connection
func (it *IntegrationTest) cleanup() error {
	if it.db != nil {
		return it.db.Close()
	}
	return nil
}

// clearTable removes all records from a table
func (it *IntegrationTest) clearTable(t *testing.T, table string) {
	t.Helper()

	if table != fluentUsersTable && table != fluentPostsTable {
		t.Fatalf("unknown FluentDB integration table %q", table)
	}
	_, err := it.db.Exec(it.ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
	require.NoError(t, err, "failed to clear table %s", table)
}

func setupFluentIntegrationSchema(t *testing.T, ctx context.Context, db v1.DB) {
	t.Helper()

	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INT,
			status VARCHAR(50) DEFAULT 'active'
		)`, fluentUsersTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			published BOOLEAN DEFAULT false,
			FOREIGN KEY (user_id) REFERENCES %s(id)
		)`, fluentPostsTable, fluentUsersTable),
	}

	for _, stmt := range statements {
		_, err := db.Exec(ctx, stmt)
		require.NoError(t, err, "failed to create FluentDB integration schema")
	}
}

// TestFluentDBSelectBasic tests basic SELECT operations
func TestFluentDBSelectBasic(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert test data
	result, err := v1.NewFluentDB(it.db).Insert().
		Into(fluentUsersTable).
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Set("age", 30).
		Exec(it.ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Test SELECT all columns
	rows, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "John Doe", rows[0]["name"])
	assert.Equal(t, "john@example.com", rows[0]["email"])
}

// TestFluentDBSelectWithWhere tests SELECT with WHERE conditions
func TestFluentDBSelectWithWhere(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert multiple test users
	for i, name := range []string{"Alice", "Bob", "Charlie"} {
		_, err := v1.NewFluentDB(it.db).Insert().
			Into(fluentUsersTable).
			Set("name", name).
			Set("email", name+"@example.com").
			Set("age", 25+i).
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Test SELECT with WHERE condition
	rows, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name", "age").
		Where(cdt.NewExpr().Column("age").Op(">").Value(25)).
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 2) // Bob (26) and Charlie (27)

	// Test SELECT with exact match
	rows, err = v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name", "email").
		Where(cdt.NewExpr().Column("name").Op("=").Value("Alice")).
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Alice", rows[0]["name"])
}

// TestFluentDBSelectOne tests One(it.ctx) method
func TestFluentDBSelectOne(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert test data
	_, err := v1.NewFluentDB(it.db).Insert().
		Into(fluentUsersTable).
		Set("name", "Test User").
		Set("email", "test@example.com").
		Set("age", 35).
		Exec(it.ctx)
	require.NoError(t, err)

	// Test One(it.ctx) returns single row
	row, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable).
		Where(cdt.NewExpr().Column("name").Op("=").Value("Test User")).
		One(it.ctx)
	require.NoError(t, err)
	assert.NotNil(t, row)
	assert.Equal(t, "Test User", row["name"])

	// Test One(it.ctx) with no results returns error
	_, err = v1.NewFluentDB(it.db).
		Select(fluentUsersTable).
		Where(cdt.NewExpr().Column("name").Op("=").Value("NonExistent")).
		One(it.ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows found")
}

// TestFluentDBSelectCount tests Count(it.ctx) method
func TestFluentDBSelectCount(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert multiple users
	for i := 0; i < 5; i++ {
		_, err := v1.NewFluentDB(it.db).Insert().
			Into(fluentUsersTable).
			Set("name", "User "+string(rune('A'+i))).
			Set("email", string(rune('a'+i))+"@example.com").
			Set("age", 20+i).
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Test COUNT all rows
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Test COUNT with WHERE condition
	count, err = v1.NewFluentDB(it.db).
		Select(fluentUsersTable).
		Where(cdt.NewExpr().Column("age").Op(">=").Value(23)).
		Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count) // Users with age 23, 24, 25
}

// TestFluentDBSelectOrderBy tests ORDER BY clause
func TestFluentDBSelectOrderBy(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert users in random order
	names := []string{"Charlie", "Alice", "Bob"}
	for i, name := range names {
		_, err := v1.NewFluentDB(it.db).Insert().
			Into(fluentUsersTable).
			Set("name", name).
			Set("email", name+"@example.com").
			Set("age", 30-i).
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Test ORDER BY ASC
	rows, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name").
		OrderBy("name", "ASC").
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
	assert.Equal(t, "Alice", rows[0]["name"])
	assert.Equal(t, "Bob", rows[1]["name"])
	assert.Equal(t, "Charlie", rows[2]["name"])

	// Test ORDER BY DESC
	rows, err = v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name").
		OrderBy("name", "DESC").
		Get(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, "Charlie", rows[0]["name"])
	assert.Equal(t, "Bob", rows[1]["name"])
	assert.Equal(t, "Alice", rows[2]["name"])
}

// TestFluentDBSelectLimitOffset tests LIMIT and OFFSET
func TestFluentDBSelectLimitOffset(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert 10 users
	for i := 0; i < 10; i++ {
		_, err := v1.NewFluentDB(it.db).Insert().
			Into(fluentUsersTable).
			Set("name", "User"+string(rune('A'+i))).
			Set("email", string(rune('a'+i))+"@example.com").
			Set("age", 20+i).
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Test LIMIT only
	rows, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name").
		OrderBy("id", "ASC").
		Limit(3).
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 3)

	// Test LIMIT with OFFSET
	rows, err = v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name").
		OrderBy("id", "ASC").
		Limit(3).
		Offset(3).
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
	// Verify we got different users (offset worked)
	assert.NotEqual(t, rows[0]["name"], "UserA")
}

// TestFluentDBInsertSingle tests single INSERT operation
func TestFluentDBInsertSingle(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Test INSERT with Values()
	result, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Values(map[string]any{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		}).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Verify data was inserted
	rows, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

// TestFluentDBInsertBulk tests bulk INSERT operation
func TestFluentDBInsertBulk(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Test bulk INSERT
	bulkData := []map[string]any{
		{"name": "Alice", "email": "alice.n@example.com", "age": 25},
		{"name": "Bob", "email": "bob.j@example.com", "age": 30},
		{"name": "Charlie", "email": "charlie.a@example.com", "age": 35},
	}

	result, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		ValuesBulk(bulkData).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.RowsAffected)

	// Verify all records were inserted
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestFluentDBInsertWithSet tests INSERT with Set() method
func TestFluentDBInsertWithSet(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Test INSERT with multiple Set() calls
	result, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "Test User").
		Set("email", "test@example.com").
		Set("age", 25).
		Set("status", "active").
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Verify all fields were set
	row, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name", "email", "age", "status").
		One(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, "Test User", row["name"])
	assert.Equal(t, "test@example.com", row["email"])
	assert.Equal(t, int32(25), row["age"])
	assert.Equal(t, "active", row["status"])
}

// TestFluentDBUpdateBasic tests basic UPDATE operation
func TestFluentDBUpdateBasic(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert test data
	_, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Set("age", 30).
		Exec(it.ctx)
	require.NoError(t, err)

	// Test UPDATE
	result, err := v1.NewFluentDB(it.db).
		Update(fluentUsersTable).
		Set("age", 31).
		Set("status", "inactive").
		Where(cdt.NewExpr().Column("name").Op("=").Value("John Doe")).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Verify update
	row, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "age", "status").
		Where(cdt.NewExpr().Column("name").Op("=").Value("John Doe")).
		One(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int32(31), row["age"])
	assert.Equal(t, "inactive", row["status"])
}

// TestFluentDBUpdateMultiple tests UPDATE with multiple rows
func TestFluentDBUpdateMultiple(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert multiple users
	for i := 0; i < 5; i++ {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Set("name", "User"+string(rune('A'+i))).
			Set("email", string(rune('a'+i))+"@example.com").
			Set("age", 20+i).
			Set("status", "active").
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Update multiple records
	result, err := v1.NewFluentDB(it.db).
		Update(fluentUsersTable).
		Set("status", "inactive").
		Where(cdt.NewExpr().Column("age").Op(">").Value(21)).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Greater(t, result.RowsAffected, int64(1))

	// Verify updates
	count, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable).
		Where(cdt.NewExpr().Column("status").Op("=").Value("inactive")).
		Count(it.ctx)
	require.NoError(t, err)
	assert.Greater(t, count, int64(1))
}

// TestFluentDBDeleteBasic tests basic DELETE operation
func TestFluentDBDeleteBasic(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert test data
	_, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	// Insert another user to ensure we only delete the right one
	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "Jane Doe").
		Set("email", "jane@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	// Test DELETE
	result, err := v1.NewFluentDB(it.db).
		Delete().
		From(fluentUsersTable).
		Where(cdt.NewExpr().Column("name").Op("=").Value("John Doe")).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Verify deletion
	count, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable).
		Where(cdt.NewExpr().Column("name").Op("=").Value("John Doe")).
		Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Verify other row still exists
	count, err = v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestFluentDBDeleteMultiple tests DELETE with multiple rows
func TestFluentDBDeleteMultiple(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert multiple users
	for i := 0; i < 5; i++ {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Set("name", "User"+string(rune('A'+i))).
			Set("email", string(rune('a'+i))+"@example.com").
			Set("age", 20+i).
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Delete multiple records
	result, err := v1.NewFluentDB(it.db).
		Delete().
		From(fluentUsersTable).
		Where(cdt.NewExpr().Column("age").Op("<").Value(23)).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Greater(t, result.RowsAffected, int64(0))

	// Verify remaining records
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Less(t, count, int64(5))
}

// TestFluentDBTransactionCommit tests transaction with successful commit
func TestFluentDBTransactionCommit(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Create a transaction and commit successfully
	tx, err := it.db.Begin(it.ctx)
	require.NoError(t, err)

	// Insert within transaction
	result, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		WithTx(tx).
		Set("name", "TX User 1").
		Set("email", "tx1@example.com").
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Commit transaction
	err = tx.Commit(it.ctx)
	require.NoError(t, err)

	// Verify data was committed
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestFluentDBTransactionRollback tests transaction with rollback
func TestFluentDBTransactionRollback(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert outside transaction
	_, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "Before TX").
		Set("email", "before@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	// Create a transaction and rollback
	tx, err := it.db.Begin(it.ctx)
	require.NoError(t, err)

	// Insert within transaction
	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		WithTx(tx).
		Set("name", "TX User Rollback").
		Set("email", "txrollback@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	// Rollback transaction
	err = tx.Rollback(it.ctx)
	require.NoError(t, err)

	// Verify only original data exists
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	row, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).One(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, "Before TX", row["name"])
}

// TestFluentDBTransactionNested tests nested transaction-like operations
func TestFluentDBTransactionNested(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)
	defer it.clearTable(t, fluentPostsTable)

	// Create a transaction with multiple operations
	tx, err := it.db.Begin(it.ctx)
	require.NoError(t, err)

	// Insert user
	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		WithTx(tx).
		Set("name", "Blog Author").
		Set("email", "author@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	userRow, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "id").
		WithTx(tx).
		Where(cdt.NewExpr().Column("email").Op("=").Value("author@example.com")).
		One(it.ctx)
	require.NoError(t, err)
	userID := userRow["id"]

	// Insert posts for the user
	postsToInsert := []map[string]any{
		{"user_id": userID, "title": "Post 1", "content": "Content 1", "published": true},
		{"user_id": userID, "title": "Post 2", "content": "Content 2", "published": true},
	}

	postResult, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentPostsTable).
		WithTx(tx).
		ValuesBulk(postsToInsert).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), postResult.RowsAffected)

	// Commit transaction
	err = tx.Commit(it.ctx)
	require.NoError(t, err)

	// Verify all data was committed
	userCount, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), userCount)

	postCount, err := v1.NewFluentDB(it.db).Select(fluentPostsTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), postCount)
}

// TestFluentDBTransactionErrorHandling tests transaction with error handling
func TestFluentDBTransactionErrorHandling(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Create a transaction that will rollback on error
	tx, err := it.db.Begin(it.ctx)
	require.NoError(t, err)

	// Insert first user
	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		WithTx(tx).
		Set("name", "User 1").
		Set("email", "user1@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	// Simulate error by trying to insert duplicate email (unique constraint)
	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		WithTx(tx).
		Set("name", "User 2").
		Set("email", "user1@example.com"). // Duplicate email
		Exec(it.ctx)
	assert.Error(t, err) // Expected error

	// Rollback transaction
	_ = tx.Rollback(it.ctx)
	// Rollback should succeed even after error

	// Verify no users were inserted (entire transaction rolled back)
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// TestFluentDBSelectWithJoin tests SELECT with JOIN
func TestFluentDBSelectWithJoin(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentPostsTable)
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert user
	_, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Exec(it.ctx)
	require.NoError(t, err)

	userRow, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "id").
		Where(cdt.NewExpr().Column("email").Op("=").Value("john@example.com")).
		One(it.ctx)
	require.NoError(t, err)
	userID := userRow["id"]

	// Insert posts
	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentPostsTable).
		ValuesBulk([]map[string]any{
			{"user_id": userID, "title": "Post 1", "published": true},
			{"user_id": userID, "title": "Post 2", "published": true},
		}).
		Exec(it.ctx)
	require.NoError(t, err)

	// Test SELECT with JOIN
	join := cdt.Join{
		Type:       "INNER",
		Table:      fluentPostsTable,
		Conditions: cdt.JoinCdts{{Left: "id", Right: "user_id"}},
	}

	rows, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, fluentUsersTable+".name", fluentPostsTable+".title").
		Join(join).
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 2) // Two posts from one user
}

// TestFluentDBConcurrentOperations tests concurrent FluentDB operations
func TestFluentDBConcurrentOperations(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert initial users concurrently
	done := make(chan error, 3)

	go func() {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Set("name", "User A").
			Set("email", "usera@example.com").
			Exec(it.ctx)
		done <- err
	}()

	go func() {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Set("name", "User B").
			Set("email", "userb@example.com").
			Exec(it.ctx)
		done <- err
	}()

	go func() {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Set("name", "User C").
			Set("email", "userc@example.com").
			Exec(it.ctx)
		done <- err
	}()

	// Wait for all operations
	for i := 0; i < 3; i++ {
		err := <-done
		require.NoError(t, err)
	}

	// Verify all were inserted
	count, err := v1.NewFluentDB(it.db).Select(fluentUsersTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestFluentDBSelectWithMultipleConditions tests SELECT with multiple WHERE conditions
func TestFluentDBSelectWithMultipleConditions(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert test users
	testUsers := []map[string]any{
		{"name": "Alice", "email": "alice.n@example.com", "age": 25, "status": "active"},
		{"name": "Bob", "email": "bob.j@example.com", "age": 30, "status": "active"},
		{"name": "Charlie", "email": "charlie.a@example.com", "age": 35, "status": "inactive"},
		{"name": "Diana", "email": "diana.w@example.com", "age": 28, "status": "active"},
	}

	for _, user := range testUsers {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Values(user).
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Test multiple conditions (age > 25 AND status = 'active')
	rows, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name", "age").
		Where(cdt.NewExpr().Column("age").Op(">").Value(25)).
		Where(cdt.NewExpr().Column("status").Op("=").Value("active")).
		Get(it.ctx)
	require.NoError(t, err)

	// Should return Bob (30) and Diana (28), but not Alice (25)
	assert.Len(t, rows, 2)
	names := make(map[string]bool)
	for _, row := range rows {
		names[row["name"].(string)] = true
	}
	assert.True(t, names["Bob"])
	assert.True(t, names["Diana"])
	assert.False(t, names["Alice"])
}

// TestFluentDBUpdateWithSetMap tests UPDATE with SetMap
func TestFluentDBUpdateWithSetMap(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert test data
	_, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "John Doe").
		Set("email", "john@example.com").
		Set("age", 30).
		Set("status", "active").
		Exec(it.ctx)
	require.NoError(t, err)

	// Update with SetMap
	dataMap := map[string]any{
		"age":    31,
		"status": "inactive",
		"name":   "Jane Doe",
	}

	result, err := v1.NewFluentDB(it.db).
		Update(fluentUsersTable).
		SetMap(dataMap).
		Where(cdt.NewExpr().Column("email").Op("=").Value("john@example.com")).
		Exec(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Verify all fields were updated
	row, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable).
		Where(cdt.NewExpr().Column("email").Op("=").Value("john@example.com")).
		One(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, "Jane Doe", row["name"])
	assert.Equal(t, int32(31), row["age"])
	assert.Equal(t, "inactive", row["status"])
}

// TestFluentDBComplexWorkflow tests a realistic workflow with all operations
func TestFluentDBComplexWorkflow(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentPostsTable)
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// 1. Create users
	_, err := v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "Alice").
		Set("email", "alice.n@example.com").
		Set("age", 28).
		Exec(it.ctx)
	require.NoError(t, err)

	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentUsersTable).
		Set("name", "Bob").
		Set("email", "bob.j@example.com").
		Set("age", 32).
		Exec(it.ctx)
	require.NoError(t, err)

	aliceRow, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "id").
		Where(cdt.NewExpr().Column("email").Op("=").Value("alice.n@example.com")).
		One(it.ctx)
	require.NoError(t, err)
	aliceID := aliceRow["id"]

	bobRow, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "id").
		Where(cdt.NewExpr().Column("email").Op("=").Value("bob.j@example.com")).
		One(it.ctx)
	require.NoError(t, err)
	bobID := bobRow["id"]

	// 2. Create posts for users
	tx, err := it.db.Begin(it.ctx)
	require.NoError(t, err)

	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentPostsTable).
		WithTx(tx).
		Set("user_id", aliceID).
		Set("title", "Alice's First Post").
		Set("content", "Hello World").
		Set("published", true).
		Exec(it.ctx)
	require.NoError(t, err)

	_, err = v1.NewFluentDB(it.db).
		Insert().
		Into(fluentPostsTable).
		WithTx(tx).
		Set("user_id", bobID).
		Set("title", "Bob's Post").
		Set("content", "Hello from Bob").
		Set("published", false).
		Exec(it.ctx)
	require.NoError(t, err)

	err = tx.Commit(it.ctx)
	require.NoError(t, err)

	// 3. Query published posts
	publishedPosts, err := v1.NewFluentDB(it.db).
		Select(fluentPostsTable, "title", "user_id").
		Where(cdt.NewExpr().Column("published").Op("=").Value(true)).
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, publishedPosts, 1)

	// 4. Update user age
	_, err = v1.NewFluentDB(it.db).
		Update(fluentUsersTable).
		Set("age", 29).
		Where(cdt.NewExpr().Column("name").Op("=").Value("Alice")).
		Exec(it.ctx)
	require.NoError(t, err)

	// 5. Verify final state
	users, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name", "age").
		OrderBy("name", "ASC").
		Get(it.ctx)
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, int32(29), users[0]["age"].(int32))

	// 6. Delete unpublished posts
	_, err = v1.NewFluentDB(it.db).
		Delete().
		From(fluentPostsTable).
		Where(cdt.NewExpr().Column("published").Op("=").Value(false)).
		Exec(it.ctx)
	require.NoError(t, err)

	// 7. Verify deletion
	postCount, err := v1.NewFluentDB(it.db).Select(fluentPostsTable).Count(it.ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), postCount)
}

// TestFluentDBSelectGetRaw tests GetRaw(it.ctx) returns a valid RowsAdapter
func TestFluentDBSelectGetRaw(t *testing.T) {
	it := setupIntegration(t)
	defer it.cleanup()
	defer it.clearTable(t, fluentUsersTable)

	it.clearTable(t, fluentUsersTable)

	// Insert multiple users
	for i := 0; i < 10; i++ {
		_, err := v1.NewFluentDB(it.db).
			Insert().
			Into(fluentUsersTable).
			Set("name", "User"+string(rune('A'+i))).
			Set("email", string(rune('a'+i))+"@example.com").
			Exec(it.ctx)
		require.NoError(t, err)
	}

	// Test GetRaw(it.ctx) returns a valid RowsAdapter
	rows, err := v1.NewFluentDB(it.db).
		Select(fluentUsersTable, "name", "email").
		OrderBy("id", "ASC").
		Limit(5).
		GetRaw(it.ctx)
	require.NoError(t, err)
	require.NotNil(t, rows)

	// ScanRowsTo automatically closes the adapter
	type User struct {
		Name  string
		Email string
	}
	users, err := v1.ScanRowsTo[User](it.ctx, rows)
	require.NoError(t, err)
	require.True(t, len(users) > 0, "should have scanned users")
}
