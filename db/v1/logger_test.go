//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/db/v1"
)

// TestLoggerInterface verifies Logger interface contract
func TestLoggerInterface(t *testing.T) {
	var _ v1.Logger = (*NoOpLogger)(nil)
}

// TestLoggerDebugMethod tests debug logging interface
func TestLoggerDebugMethod(t *testing.T) {
	logger := NoOpLogger{}
	// Should not panic
	logger.Debug("test message")
	logger.Debug("test message", "key", "value")
	logger.Debug("test message", "key1", "value1", "key2", "value2")
}

// TestLoggerInfoMethod tests info logging interface
func TestLoggerInfoMethod(t *testing.T) {
	logger := NoOpLogger{}
	// Should not panic
	logger.Info("test message")
	logger.Info("test message", "key", "value")
	logger.Info("test message", "key1", "value1", "key2", "value2")
}

// TestLoggerWarnMethod tests warn logging interface
func TestLoggerWarnMethod(t *testing.T) {
	logger := NoOpLogger{}
	// Should not panic
	logger.Warn("test message")
	logger.Warn("test message", "key", "value")
	logger.Warn("test message", "key1", "value1", "key2", "value2")
}

// TestLoggerErrorMethod tests error logging interface
func TestLoggerErrorMethod(t *testing.T) {
	logger := NoOpLogger{}
	// Should not panic
	logger.Error("test message")
	logger.Error("test message", "key", "value")
	logger.Error("test message", "key1", "value1", "key2", "value2")
}

// TestLoggerWithMethod tests With chaining
func TestLoggerWithMethod(t *testing.T) {
	logger := NoOpLogger{}
	newLogger := logger.With("key", "value")
	require.NotNil(t, newLogger)
	assert.IsType(t, NoOpLogger{}, newLogger)
}

// TestLoggerWithChaining tests multiple With calls
func TestLoggerWithChaining(t *testing.T) {
	logger := NoOpLogger{}
	chained := logger.With("a", 1).With("b", 2).With("c", 3)
	require.NotNil(t, chained)
	assert.IsType(t, NoOpLogger{}, chained)
}

// TestLoggerWithEmptyArgs tests With with no arguments
func TestLoggerWithEmptyArgs(t *testing.T) {
	logger := NoOpLogger{}
	newLogger := logger.With()
	require.NotNil(t, newLogger)
}

// TestLoggerWithVariousTypes tests With with different argument types
func TestLoggerWithVariousTypes(t *testing.T) {
	logger := NoOpLogger{}

	newLogger := logger.With(
		"string", "value",
		"int", 42,
		"float", 3.14,
		"bool", true,
		"nil", nil,
	)
	require.NotNil(t, newLogger)
}

// TestLoggerImplementationContract tests that NoOpLogger implements Logger
func TestLoggerImplementationContract(t *testing.T) {
	var logger v1.Logger = NoOpLogger{}

	// Test all methods exist and can be called
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	newLogger := logger.With("key", "value")
	assert.NotNil(t, newLogger)
}

// TestLoggerWithReturnType tests that With returns a Logger
func TestLoggerWithReturnType(t *testing.T) {
	logger := NoOpLogger{}
	result := logger.With("key", "value")

	assert.NotNil(t, result)
	assert.IsType(t, NoOpLogger{}, result)
}

// CustomTestLogger implementation for testing
type CustomTestLogger struct {
	calls map[string]int
}

func (c *CustomTestLogger) Debug(msg string, args ...interface{}) {
	c.calls["Debug"]++
}

func (c *CustomTestLogger) Info(msg string, args ...interface{}) {
	c.calls["Info"]++
}

func (c *CustomTestLogger) Warn(msg string, args ...interface{}) {
	c.calls["Warn"]++
}

func (c *CustomTestLogger) Error(msg string, args ...interface{}) {
	c.calls["Error"]++
}

func (c *CustomTestLogger) With(fields ...interface{}) v1.Logger {
	return c
}

// TestLoggerInterfaceImplementation tests custom logger implementation
func TestLoggerInterfaceImplementation(t *testing.T) {
	logger := &CustomTestLogger{
		calls: make(map[string]int),
	}

	var _ v1.Logger = logger

	logger.Debug("test")
	assert.Equal(t, 1, logger.calls["Debug"])

	logger.Info("test")
	assert.Equal(t, 1, logger.calls["Info"])

	logger.Warn("test")
	assert.Equal(t, 1, logger.calls["Warn"])

	logger.Error("test")
	assert.Equal(t, 1, logger.calls["Error"])
}

// TestLoggerMultipleCalls tests multiple calls to each method
func TestLoggerMultipleCalls(t *testing.T) {
	logger := &CustomTestLogger{
		calls: make(map[string]int),
	}

	for i := 0; i < 5; i++ {
		logger.Debug("message")
	}
	assert.Equal(t, 5, logger.calls["Debug"])

	for i := 0; i < 3; i++ {
		logger.Info("message")
	}
	assert.Equal(t, 3, logger.calls["Info"])

	for i := 0; i < 7; i++ {
		logger.Warn("message")
	}
	assert.Equal(t, 7, logger.calls["Warn"])

	for i := 0; i < 2; i++ {
		logger.Error("message")
	}
	assert.Equal(t, 2, logger.calls["Error"])
}
