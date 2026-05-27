//go:build test

package v1_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/apex/log"
	apexjson "github.com/apex/log/handlers/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	v1 "tounilab.com/vessel/db/v1"
)

// SLOG ADAPTER TESTS

// TestSlogAdapter_BasicLogging tests that SlogAdapter correctly delegates logging calls.
func TestSlogAdapter_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	require.NotNil(t, adapter)

	adapter.Info("test message", "key", "value")

	// Verify JSON output contains the message and fields
	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "value", logEntry["key"])
}

// TestSlogAdapter_Debug tests debug-level logging.
func TestSlogAdapter_Debug(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	adapter.Debug("debug message", "level", "debug")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "debug message", logEntry["msg"])
}

// TestSlogAdapter_Info tests info-level logging.
func TestSlogAdapter_Info(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	adapter.Info("info message", "status", "ok")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "info message", logEntry["msg"])
}

// TestSlogAdapter_Warn tests warning-level logging.
func TestSlogAdapter_Warn(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	adapter.Warn("warning message", "code", 123)

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "warning message", logEntry["msg"])
}

// TestSlogAdapter_Error tests error-level logging.
func TestSlogAdapter_Error(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	adapter.Error("error message", "error", "something went wrong")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "error message", logEntry["msg"])
}

// TestSlogAdapter_With tests that With() returns a new Logger with context.
func TestSlogAdapter_With(t *testing.T) {
	var buf1 bytes.Buffer
	var buf2 bytes.Buffer

	handler1 := slog.NewJSONHandler(&buf1, nil)
	logger1 := slog.New(handler1)
	adapter1 := v1.NewSlogAdapter(logger1)

	// Create a logger with context
	handler2 := slog.NewJSONHandler(&buf2, nil)
	logger2 := slog.New(handler2)
	adapter2 := v1.NewSlogAdapter(logger2)
	adapterWithContext := adapter2.With("tenant_id", "company-123")

	adapter1.Info("message without context", "action", "test")
	adapterWithContext.Info("message with context", "action", "test")

	// Verify first log doesn't have tenant_id
	var logEntry1 map[string]any
	err := json.Unmarshal(buf1.Bytes(), &logEntry1)
	require.NoError(t, err)
	_, hasTenantID1 := logEntry1["tenant_id"]
	assert.False(t, hasTenantID1)

	// Verify second log has tenant_id
	var logEntry2 map[string]any
	err = json.Unmarshal(buf2.Bytes(), &logEntry2)
	require.NoError(t, err)
	tenantID, hasTenantID2 := logEntry2["tenant_id"]
	assert.True(t, hasTenantID2)
	assert.Equal(t, "company-123", tenantID)
}

// TestSlogAdapter_MultipleFields tests logging with multiple key-value pairs.
func TestSlogAdapter_MultipleFields(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	adapter.Info("database operation",
		"operation", "insert",
		"table", "users",
		"rows_affected", 5,
		"duration_ms", 42.5)

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "database operation", logEntry["msg"])
	assert.Equal(t, "insert", logEntry["operation"])
	assert.Equal(t, "users", logEntry["table"])
	assert.Equal(t, float64(5), logEntry["rows_affected"])
	assert.Equal(t, 42.5, logEntry["duration_ms"])
}

// TestSlogAdapter_WithNilLogger tests that NewSlogAdapter uses default logger when nil is passed.
func TestSlogAdapter_WithNilLogger(t *testing.T) {
	adapter := v1.NewSlogAdapter(nil)
	require.NotNil(t, adapter)

	// Should not panic
	adapter.Info("test message")
	adapter.Debug("debug message")
	adapter.Warn("warning message")
	adapter.Error("error message")
}

// TestSlogAdapter_ImplementsLoggerInterface verifies the adapter implements the Logger interface.
func TestSlogAdapter_ImplementsLoggerInterface(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	_ = v1.NewSlogAdapter(logger)
}

// TestSlogAdapter_ChainedWith tests chaining multiple With() calls.
func TestSlogAdapter_ChainedWith(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)
	chainedAdapter := adapter.
		With("tenant_id", "company-123").
		With("user_id", "user-456")

	chainedAdapter.Info("chained context message")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "company-123", logEntry["tenant_id"])
	assert.Equal(t, "user-456", logEntry["user_id"])
}

// TestSlogAdapter_WithOddNumberOfArgs tests handling of odd number of arguments.
func TestSlogAdapter_WithOddNumberOfArgs(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	adapter := v1.NewSlogAdapter(logger)

	// This should not panic even with odd number of args
	// slog handles this gracefully by logging the orphaned key
	adapter.Info("test message", "key_without_value")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "test message", logEntry["msg"])
}

// LOGRUS ADAPTER TESTS

// TestLogrusAdapter_BasicLogging tests that LogrusAdapter correctly delegates logging calls.
func TestLogrusAdapter_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	adapter := v1.NewLogrusAdapter(logger)
	require.NotNil(t, adapter)

	adapter.Info("test message", "key", "value")

	// Verify JSON output contains the message
	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "test message", logEntry["msg"])
}

// TestLogrusAdapter_Debug tests debug-level logging.
func TestLogrusAdapter_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel)

	adapter := v1.NewLogrusAdapter(logger)
	adapter.Debug("debug message", "level", "debug")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "debug message", logEntry["msg"])
}

// TestLogrusAdapter_Warn tests warning-level logging.
func TestLogrusAdapter_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	adapter := v1.NewLogrusAdapter(logger)
	adapter.Warn("warning message", "code", 123)

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "warning message", logEntry["msg"])
}

// TestLogrusAdapter_Error tests error-level logging.
func TestLogrusAdapter_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	adapter := v1.NewLogrusAdapter(logger)
	adapter.Error("error message", "error", "something went wrong")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "error message", logEntry["msg"])
}

// TestLogrusAdapter_With tests that With() returns a new Logger with context.
func TestLogrusAdapter_With(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	adapter := v1.NewLogrusAdapter(logger)
	adapterWithContext := adapter.With("tenant_id", "company-123")

	adapterWithContext.Info("message with context", "action", "test")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "message with context", logEntry["msg"])
	assert.Equal(t, "company-123", logEntry["tenant_id"])
}

// TestLogrusAdapter_ImplementsLoggerInterface verifies the adapter implements the Logger interface.
func TestLogrusAdapter_ImplementsLoggerInterface(t *testing.T) {
	logger := logrus.New()
	_ = v1.NewLogrusAdapter(logger)
}

// TestLogrusAdapter_ChainedWith tests chaining multiple With() calls.
func TestLogrusAdapter_ChainedWith(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})

	adapter := v1.NewLogrusAdapter(logger)
	chainedAdapter := adapter.
		With("tenant_id", "company-123").
		With("user_id", "user-456")

	chainedAdapter.Info("chained context message")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "company-123", logEntry["tenant_id"])
	assert.Equal(t, "user-456", logEntry["user_id"])
}

// ZAP ADAPTER TESTS

// TestZapAdapter_BasicLogging tests that ZapAdapter correctly delegates logging calls.
func TestZapAdapter_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		TimeKey:    "time",
		NameKey:    "logger",
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.InfoLevel)
	logger := zap.New(core)

	adapter := v1.NewZapAdapter(logger)
	require.NotNil(t, adapter)

	adapter.Info("test message", "key", "value")

	// Verify JSON output contains the message
	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "test message", logEntry["msg"])
}

// TestZapAdapter_Debug tests debug-level logging.
func TestZapAdapter_Debug(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		TimeKey:    "time",
		NameKey:    "logger",
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.DebugLevel)
	logger := zap.New(core)

	adapter := v1.NewZapAdapter(logger)
	adapter.Debug("debug message", "level", "debug")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "debug message", logEntry["msg"])
}

// TestZapAdapter_Warn tests warning-level logging.
func TestZapAdapter_Warn(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		TimeKey:    "time",
		NameKey:    "logger",
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.WarnLevel)
	logger := zap.New(core)

	adapter := v1.NewZapAdapter(logger)
	adapter.Warn("warning message", "code", 123)

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "warning message", logEntry["msg"])
}

// TestZapAdapter_Error tests error-level logging.
func TestZapAdapter_Error(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		TimeKey:    "time",
		NameKey:    "logger",
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.ErrorLevel)
	logger := zap.New(core)

	adapter := v1.NewZapAdapter(logger)
	adapter.Error("error message", "error", "something went wrong")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "error message", logEntry["msg"])
}

// TestZapAdapter_With tests that With() returns a new Logger with context.
func TestZapAdapter_With(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		TimeKey:    "time",
		NameKey:    "logger",
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.InfoLevel)
	logger := zap.New(core)

	adapter := v1.NewZapAdapter(logger)
	adapterWithContext := adapter.With("tenant_id", "company-123")

	adapterWithContext.Info("message with context", "action", "test")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "message with context", logEntry["msg"])
}

// TestZapAdapter_ImplementsLoggerInterface verifies the adapter implements the Logger interface.
func TestZapAdapter_ImplementsLoggerInterface(t *testing.T) {
	logger := zap.NewNop()
	_ = v1.NewZapAdapter(logger)
}

// TestZapAdapter_ChainedWith tests chaining multiple With() calls.
func TestZapAdapter_ChainedWith(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		TimeKey:    "time",
		NameKey:    "logger",
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.InfoLevel)
	logger := zap.New(core)

	adapter := v1.NewZapAdapter(logger)
	chainedAdapter := adapter.
		With("tenant_id", "company-123").
		With("user_id", "user-456")

	chainedAdapter.Info("chained context message")

	var logEntry map[string]any
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "chained context message", logEntry["msg"])
}

// APEX ADAPTER TESTS

// TestApexAdapter_BasicLogging tests that ApexAdapter correctly delegates logging calls.
func TestApexAdapter_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := &log.Logger{
		Handler: apexjson.New(&buf),
		Level:   log.InfoLevel,
	}

	adapter := v1.NewApexAdapter(logger)
	require.NotNil(t, adapter)

	adapter.Info("test message", "key", "value")

	// Verify JSON output contains the message
	logOutput := buf.String()
	assert.Contains(t, logOutput, "test message")
}

// TestApexAdapter_Debug tests debug-level logging.
func TestApexAdapter_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := &log.Logger{
		Handler: apexjson.New(&buf),
		Level:   log.DebugLevel,
	}

	adapter := v1.NewApexAdapter(logger)
	adapter.Debug("debug message", "level", "debug")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "debug message")
}

// TestApexAdapter_Warn tests warning-level logging.
func TestApexAdapter_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := &log.Logger{
		Handler: apexjson.New(&buf),
		Level:   log.WarnLevel,
	}

	adapter := v1.NewApexAdapter(logger)
	adapter.Warn("warning message", "code", 123)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "warning message")
}

// TestApexAdapter_Error tests error-level logging.
func TestApexAdapter_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &log.Logger{
		Handler: apexjson.New(&buf),
		Level:   log.ErrorLevel,
	}

	adapter := v1.NewApexAdapter(logger)
	adapter.Error("error message", "error", "something went wrong")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "error message")
}

// TestApexAdapter_With tests that With() returns a new Logger with context.
func TestApexAdapter_With(t *testing.T) {
	var buf bytes.Buffer
	logger := &log.Logger{
		Handler: apexjson.New(&buf),
		Level:   log.InfoLevel,
	}

	adapter := v1.NewApexAdapter(logger)
	adapterWithContext := adapter.With("tenant_id", "company-123")

	adapterWithContext.Info("message with context", "action", "test")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "message with context")
	assert.Contains(t, logOutput, "company-123")
}

// TestApexAdapter_ImplementsLoggerInterface verifies the adapter implements the Logger interface.
func TestApexAdapter_ImplementsLoggerInterface(t *testing.T) {
	logger := &log.Logger{
		Handler: apexjson.New(bytes.NewBuffer(nil)),
	}
	_ = v1.NewApexAdapter(logger)
}

// TestApexAdapter_ChainedWith tests chaining multiple With() calls.
func TestApexAdapter_ChainedWith(t *testing.T) {
	var buf bytes.Buffer
	logger := &log.Logger{
		Handler: apexjson.New(&buf),
		Level:   log.InfoLevel,
	}

	adapter := v1.NewApexAdapter(logger)
	chainedAdapter := adapter.
		With("tenant_id", "company-123").
		With("user_id", "user-456")

	chainedAdapter.Info("chained context message")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "chained context message")
	assert.Contains(t, logOutput, "company-123")
	assert.Contains(t, logOutput, "user-456")
}
