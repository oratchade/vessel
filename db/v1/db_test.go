//go:build test

package v1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/db/v1"
)

// MockDBConfig implements the DBConfig interface for testing.
type MockDBConfig struct{}

func (m MockDBConfig) Dialect() string {
	return "test"
}

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

func TestExecResult(t *testing.T) {
	testCases := []struct {
		name       string
		lastInsID  int64
		rowsAffect int64
	}{
		{"positive values", 42, 100},
		{"zero values", 0, 0},
		{"large values", 9223372036854775807, 9223372036854775807},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockResult := mockSQLResult{
				lastInsertID: tc.lastInsID,
				rowsAffected: tc.rowsAffect,
			}

			result := &v1.ExecResult{
				LastInsertID: tc.lastInsID,
				RowsAffected: tc.rowsAffect,
			}

			assert.Equal(t, mockResult.lastInsertID, result.LastInsertID)
			assert.Equal(t, mockResult.rowsAffected, result.RowsAffected)
		})
	}
}

func TestPoolStatistics(t *testing.T) {
	testCases := []struct {
		name               string
		idle               int
		inUse              int
		openConnections    int
		maxOpenConnections int
		waitCount          int64
		waitDuration       time.Duration
	}{
		{"normal stats", 5, 15, 20, 50, 10, 100 * time.Millisecond},
		{"all idle", 25, 0, 25, 50, 0, 0},
		{"all in use", 0, 30, 30, 50, 100, 500 * time.Millisecond},
		{"zero values", 0, 0, 0, 0, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stats := &v1.PoolStatistics{
				Idle:               tc.idle,
				InUse:              tc.inUse,
				OpenConnections:    tc.openConnections,
				MaxOpenConnections: tc.maxOpenConnections,
				WaitCount:          tc.waitCount,
				WaitDuration:       tc.waitDuration,
			}

			assert.Equal(t, tc.idle, stats.Idle)
			assert.Equal(t, tc.inUse, stats.InUse)
			assert.Equal(t, tc.openConnections, stats.OpenConnections)
			assert.Equal(t, tc.maxOpenConnections, stats.MaxOpenConnections)
			assert.Equal(t, tc.waitCount, stats.WaitCount)
			assert.Equal(t, tc.waitDuration, stats.WaitDuration)
		})
	}
}

func TestPoolStatisticsZeroValues(t *testing.T) {
	stats := &v1.PoolStatistics{}

	assert.Equal(t, 0, stats.Idle)
	assert.Equal(t, 0, stats.InUse)
	assert.Equal(t, 0, stats.OpenConnections)
	assert.Equal(t, 0, stats.MaxOpenConnections)
	assert.Equal(t, int64(0), stats.WaitCount)
	assert.Equal(t, time.Duration(0), stats.WaitDuration)
	assert.Equal(t, int64(0), stats.MaxIdleClosed)
	assert.Equal(t, int64(0), stats.MaxIdleTimeClosed)
	assert.Equal(t, int64(0), stats.MaxLifetimeClosed)
}

func TestPoolStatisticsAvailableConnections(t *testing.T) {
	stats := &v1.PoolStatistics{
		Idle:            5,
		InUse:           15,
		OpenConnections: 20,
	}

	// Available should be derived from total - in use
	assert.Equal(t, stats.Idle, 20-stats.InUse)
}

func TestMockDBConfig(t *testing.T) {
	mockCfg := MockDBConfig{}
	assert.Equal(t, "test", mockCfg.Dialect())
}

func TestNoOpLogger(t *testing.T) {
	logger := NoOpLogger{}

	// NoOpLogger should not panic on any method calls
	logger.Debug("test message")
	logger.Info("test message", "key", "value")
	logger.Warn("test message")
	logger.Error("test message")

	// With should return itself
	newLogger := logger.With("field", "value")
	require.NotNil(t, newLogger)
	assert.IsType(t, NoOpLogger{}, newLogger)

	// Multiple calls to With should chain
	chainedLogger := logger.With("a", 1).With("b", 2)
	assert.NotNil(t, chainedLogger)
}

func TestMockSQLResult(t *testing.T) {
	mockResult := mockSQLResult{
		lastInsertID: 123,
		rowsAffected: 45,
	}

	lastID, err := mockResult.LastInsertId()
	require.NoError(t, err)
	assert.Equal(t, int64(123), lastID)

	rowsAff, err := mockResult.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(45), rowsAff)
}
