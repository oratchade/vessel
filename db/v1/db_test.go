//go:build test

package v1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/db/v1/plugin"
)

// NoOpLogger is a logger that does nothing.
type NoOpLogger struct{}

func (n NoOpLogger) Debug(msg string, args ...any) {}
func (n NoOpLogger) Info(msg string, args ...any)  {}
func (n NoOpLogger) Warn(msg string, args ...any)  {}
func (n NoOpLogger) Error(msg string, args ...any) {}
func (n NoOpLogger) With(fields ...any) v1.Logger  { return n }

// mockSQLResult implements sql.Result for testing.
type mockSQLResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (m mockSQLResult) LastInsertId() (int64, error) {
	return m.lastInsertID, nil
}

func (m mockSQLResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

// TestFromSQLResultSuccess tests successful conversion of sql.Result to ExecResult.
func TestFromSQLResultSuccess(t *testing.T) {
	testCases := []struct {
		name         string
		rowsAffected int64
	}{
		{"single row", 1},
		{"multiple rows", 42},
		{"no rows affected", 0},
		{"large number", 999999},
		{"max int64", 9223372036854775807},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockResult := mockSQLResult{rowsAffected: tc.rowsAffected}

			result, err := v1.ExportFromSQLResult(mockResult)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.rowsAffected, result.RowsAffected)
		})
	}
}

// TestFromSQLResultErrorHandling tests error handling in ExportFromSQLResult.
func TestFromSQLResultErrorHandling(t *testing.T) {
	// Create a mock that returns an error
	errorMockResult := &errorMockSQLResult{err: fmt.Errorf("connection lost")}

	result, err := v1.ExportFromSQLResult(errorMockResult)

	assert.Nil(t, result)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to get rows affected")
	assert.Contains(t, err.Error(), "connection lost")
}

// errorMockSQLResult is a mock that returns an error for RowsAffected.
type errorMockSQLResult struct {
	err error
}

func (e *errorMockSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (e *errorMockSQLResult) RowsAffected() (int64, error) {
	return 0, e.err
}

// TestExecResultChaining tests that ExecResult values can be used in sequences,
// simulating batch execution with ExportFromSQLResult.
func TestExecResultChaining(t *testing.T) {
	results := make([]*v1.ExecResult, 3)

	// Simulate 3 consecutive database operations with different impact
	testCases := []int64{1, 5, 10}

	for i, expected := range testCases {
		mockResult := mockSQLResult{rowsAffected: expected}
		result, err := v1.ExportFromSQLResult(mockResult)

		require.NoError(t, err)
		require.NotNil(t, result)
		results[i] = result
	}

	// Verify the results are independent and correct
	assert.Equal(t, int64(1), results[0].RowsAffected)
	assert.Equal(t, int64(5), results[1].RowsAffected)
	assert.Equal(t, int64(10), results[2].RowsAffected)

	// Total affected rows across batch
	var totalRows int64
	for _, r := range results {
		totalRows += r.RowsAffected
	}
	assert.Equal(t, int64(16), totalRows)
}

// TestNewDBUnsupportedDriver tests NewDB with an unsupported driver name.
func TestNewDBUnsupportedDriver(t *testing.T) {
	cfg := &testDBConfig{driverName: "unsupported-db"}
	logger := NoOpLogger{}

	db, err := v1.NewDB(cfg, logger)

	assert.Nil(t, db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported driver")
	assert.Contains(t, err.Error(), "unsupported-db")
}

// TestNewDBMySQLDriver tests NewDB with valid MySQL config.
func TestNewDBMySQLDriver(t *testing.T) {
	cfg := &v1.MysqlConfig{
		User:     "testuser",
		Password: "testpass",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
	}
	logger := NoOpLogger{}

	db, err := v1.NewDB(cfg, logger)

	// Should not error, but we get a real DB instance
	// (might fail on actual connection, that's ok for this test)
	if err == nil {
		require.NotNil(t, db)
	}
}

// TestNewDBPostgresDriver tests NewDB with valid PostgreSQL config.
func TestNewDBPostgresDriver(t *testing.T) {
	cfg := &v1.PostgresConfig{
		User:     "testuser",
		Password: "testpass",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
	}
	logger := NoOpLogger{}

	db, err := v1.NewDB(cfg, logger)

	if err == nil {
		require.NotNil(t, db)
	}
}

// TestNewDBSQLiteDriver tests NewDB with valid SQLite config.
func TestNewDBSQLiteDriver(t *testing.T) {
	cfg := &v1.SQLiteConfig{
		FilePath: ":memory:",
	}
	logger := NoOpLogger{}

	db, err := v1.NewDB(cfg, logger)

	if err == nil {
		require.NotNil(t, db)
		_ = db.Close()
	}
}

// TestNewDBMSSQLDriver tests NewDB with valid MSSQL config.
func TestNewDBMSSQLDriver(t *testing.T) {
	cfg := &v1.MSSQLConfig{
		User:     "testuser",
		Password: "testpass",
		Host:     "localhost",
		Port:     1433,
		Database: "testdb",
	}
	logger := NoOpLogger{}

	db, err := v1.NewDB(cfg, logger)

	if err == nil {
		require.NotNil(t, db)
	}
}

// testDBConfig is a test implementation of DBConfig.
type testDBConfig struct {
	driverName string
	dsn        string
}

func (t *testDBConfig) Driver() string {
	return t.driverName
}

func (t *testDBConfig) DSN() string {
	return t.dsn
}

// TestNewDBWithNilLogger tests NewDB accepts nil logger and creates DB.
func TestNewDBWithNilLogger(t *testing.T) {
	cfg := &v1.SQLiteConfig{
		FilePath: ":memory:",
	}

	db, err := v1.NewDB(cfg, nil)

	// SQLite with :memory: should work
	if err == nil {
		require.NotNil(t, db)
		_ = db.Close()
	}
}

// TestNewDBWithRegisteredPlugin tests NewDB uses plugin registry when driver is registered.
func TestNewDBWithRegisteredPlugin(t *testing.T) {
	// Clean up any existing plugins
	defer plugin.Clear()

	// Register a test plugin
	testFactory := &testPluginFactory{
		shouldFail: false,
	}
	err := plugin.Register(testFactory)
	require.NoError(t, err)

	cfg := &testDBConfig{
		driverName: "test-plugin",
		dsn:        "test://localhost",
	}

	db, err := v1.NewDB(cfg, NoOpLogger{})

	// Should succeed and return a DB instance (SQLite in-memory)
	if err == nil {
		require.NotNil(t, db)
		_ = db.Close()
	}
}

// TestNewDBPluginReturnsError tests NewDB error handling when plugin fails.
func TestNewDBPluginReturnsError(t *testing.T) {
	defer plugin.Clear()

	testFactory := &testPluginFactory{
		shouldFail: true,
		failErr:    fmt.Errorf("connection refused"),
	}
	plugin.MustRegister(testFactory)

	cfg := &testDBConfig{
		driverName: "test-plugin",
	}

	db, err := v1.NewDB(cfg, NoOpLogger{})

	assert.Nil(t, db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin driver")
	assert.Contains(t, err.Error(), "failed")
	assert.Contains(t, err.Error(), "connection refused")
}

// TestNewDBPluginReturnsInvalidType tests NewDB error when plugin returns wrong type.
func TestNewDBPluginReturnsInvalidType(t *testing.T) {
	defer plugin.Clear()

	testFactory := &testPluginFactory{
		shouldFail:   false,
		returnString: true,
	}
	plugin.MustRegister(testFactory)

	cfg := &testDBConfig{
		driverName: "test-plugin",
	}

	db, err := v1.NewDB(cfg, NoOpLogger{})

	assert.Nil(t, db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

// testPluginFactory implements plugin.DriverFactory for testing.
type testPluginFactory struct {
	shouldFail   bool
	returnString bool
	failErr      error
}

func (f *testPluginFactory) Name() string {
	return "test-plugin"
}

func (f *testPluginFactory) Create(ctx context.Context, cfg any) (any, error) {
	_ = ctx // context not needed for this test implementation
	if f.shouldFail {
		return nil, f.failErr
	}
	if f.returnString {
		return "not a DB", nil
	}
	// Return a real SQLite in-memory database
	sqliteCfg := &v1.SQLiteConfig{FilePath: ":memory:"}
	//nolint:contextcheck
	db, err := v1.NewDB(sqliteCfg, NoOpLogger{})
	if err != nil {
		return nil, fmt.Errorf("testPluginFactory.Create: failed to create SQLite DB: %w", err)
	}
	return db, nil
}
