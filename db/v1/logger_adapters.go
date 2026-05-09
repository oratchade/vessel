// Package v1 provides database abstraction interfaces and implementations for multiple database engines.
package v1

import (
	"log/slog"

	"github.com/apex/log"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

// logLevel constants are used by adapter logWithFields dispatch switches.
const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

// SlogAdapter adapts Go's standard library log/slog.Logger to the Fabric Logger interface.
// This allows using slog with Fabric's database manager without modification.
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new Fabric Logger adapter wrapping slog.Logger.
// If logger is nil, it uses slog.Default().
//
// Example:
//
//	import "log/slog"
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	adapter := NewSlogAdapter(logger)
//	dbManager, err := NewDBManager(ctx, configPath, adapter)
func NewSlogAdapter(logger *slog.Logger) Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogAdapter{logger: logger}
}

// Debug logs a debug-level message with key-value pairs.
func (a *SlogAdapter) Debug(msg string, args ...any) {
	a.logger.Debug(msg, args...)
}

// Info logs an info-level message with key-value pairs.
func (a *SlogAdapter) Info(msg string, args ...any) {
	a.logger.Info(msg, args...)
}

// Warn logs a warning-level message with key-value pairs.
func (a *SlogAdapter) Warn(msg string, args ...any) {
	a.logger.Warn(msg, args...)
}

// Error logs an error-level message with key-value pairs.
func (a *SlogAdapter) Error(msg string, args ...any) {
	a.logger.Error(msg, args...)
}

// With returns a new Logger with additional context fields.
// The fields are added to all subsequent log messages.
func (a *SlogAdapter) With(fields ...any) Logger {
	return &SlogAdapter{logger: a.logger.With(fields...)}
}

// LogrusAdapter adapts github.com/sirupsen/logrus.Logger or logrus.Entry to the Fabric Logger interface.
// This allows using logrus with Fabric's database manager without modification.
//
// Example:
//
//	import "github.com/sirupsen/logrus"
//	logger := logrus.New()
//	adapter := NewLogrusAdapter(logger)
//	dbManager, err := NewDBManager(ctx, configPath, adapter)
type LogrusAdapter struct {
	entry *logrus.Entry
}

// NewLogrusAdapter creates a new Fabric Logger adapter wrapping logrus.Logger or logrus.Entry.
// If a *logrus.Logger is passed, it wraps it with a new Entry.
func NewLogrusAdapter(logger any) Logger {
	var entry *logrus.Entry

	// Handle both *logrus.Logger and *logrus.Entry
	switch v := logger.(type) {
	case *logrus.Logger:
		entry = logrus.NewEntry(v)
	case *logrus.Entry:
		entry = v
	default:
		// Fallback: create a default logger if something unexpected is passed
		entry = logrus.NewEntry(logrus.New())
	}

	return &LogrusAdapter{entry: entry}
}

// Debug logs a debug-level message with key-value pairs.
func (a *LogrusAdapter) Debug(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelDebug)
}

// Info logs an info-level message with key-value pairs.
func (a *LogrusAdapter) Info(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelInfo)
}

// Warn logs a warning-level message with key-value pairs.
func (a *LogrusAdapter) Warn(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelWarn)
}

// Error logs an error-level message with key-value pairs.
func (a *LogrusAdapter) Error(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelError)
}

// With returns a new Logger with additional context fields.
// For logrus, this uses WithFields.
func (a *LogrusAdapter) With(fields ...any) Logger {
	fieldsMap := argsToFields(fields)
	entry := a.entry.WithFields(fieldsMap)
	return &LogrusAdapter{entry: entry}
}

// logWithFields logs a message at the specified level with key-value pairs.
func (a *LogrusAdapter) logWithFields(msg string, args []any, level string) {
	fieldsMap := argsToFields(args)
	entry := a.entry.WithFields(fieldsMap)
	switch level {
	case logLevelDebug:
		entry.Debug(msg)
	case logLevelInfo:
		entry.Info(msg)
	case logLevelWarn:
		entry.Warn(msg)
	case logLevelError:
		entry.Error(msg)
	}
}

// ZapAdapter adapts go.uber.org/zap.Logger to the Fabric Logger interface.
// This allows using zap with Fabric's database manager without modification.
//
// Example:
//
//	import "go.uber.org/zap"
//	logger, _ := zap.NewProduction()
//	adapter := NewZapAdapter(logger)
//	dbManager, err := NewDBManager(ctx, configPath, adapter)
type ZapAdapter struct {
	logger *zap.Logger
}

// NewZapAdapter creates a new Fabric Logger adapter wrapping zap.Logger.
func NewZapAdapter(logger *zap.Logger) Logger {
	return &ZapAdapter{logger: logger}
}

// Debug logs a debug-level message with key-value pairs.
func (a *ZapAdapter) Debug(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelDebug)
}

// Info logs an info-level message with key-value pairs.
func (a *ZapAdapter) Info(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelInfo)
}

// Warn logs a warning-level message with key-value pairs.
func (a *ZapAdapter) Warn(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelWarn)
}

// Error logs an error-level message with key-value pairs.
func (a *ZapAdapter) Error(msg string, args ...any) {
	a.logWithFields(msg, args, logLevelError)
}

// With returns a new Logger with additional context fields.
func (a *ZapAdapter) With(fields ...any) Logger {
	// Convert key-value pairs to zap fields
	zapFields := argsToZapFields(fields)
	newLogger := a.logger.With(zapFields...)
	return &ZapAdapter{logger: newLogger}
}

// logWithFields logs a message at the specified level with key-value pairs.
func (a *ZapAdapter) logWithFields(msg string, args []any, level string) {
	zapFields := argsToZapFields(args)
	switch level {
	case logLevelDebug:
		a.logger.Debug(msg, zapFields...)
	case logLevelInfo:
		a.logger.Info(msg, zapFields...)
	case logLevelWarn:
		a.logger.Warn(msg, zapFields...)
	case logLevelError:
		a.logger.Error(msg, zapFields...)
	}
}

// argsToZapFields converts alternating key-value pairs to zap.Field slice.
func argsToZapFields(args []any) []zap.Field {
	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				fields = append(fields, zap.Any(key, args[i+1]))
			}
		}
	}
	return fields
}

// ApexAdapter adapts github.com/apex/log.Logger to the Fabric Logger interface.
// This allows using Apex log with Fabric's database manager without modification.
//
// Example:
//
//	import "github.com/apex/log"
//	logger := log.New()
//	adapter := NewApexAdapter(logger)
//	dbManager, err := NewDBManager(ctx, configPath, adapter)
type ApexAdapter struct {
	logger *log.Logger
	entry  *log.Entry
}

// NewApexAdapter creates a new Fabric Logger adapter wrapping apex/log.Logger.
func NewApexAdapter(logger *log.Logger) Logger {
	return &ApexAdapter{
		logger: logger,
		entry:  nil,
	}
}

// Debug logs a debug-level message with key-value pairs.
func (a *ApexAdapter) Debug(msg string, args ...any) {
	entry := a.getEntry()
	entry = entry.WithFields(log.Fields(argsToFields(args)))
	entry.Debug(msg)
}

// Info logs an info-level message with key-value pairs.
func (a *ApexAdapter) Info(msg string, args ...any) {
	entry := a.getEntry()
	entry = entry.WithFields(log.Fields(argsToFields(args)))
	entry.Info(msg)
}

// Warn logs a warning-level message with key-value pairs.
func (a *ApexAdapter) Warn(msg string, args ...any) {
	entry := a.getEntry()
	entry = entry.WithFields(log.Fields(argsToFields(args)))
	entry.Warn(msg)
}

// Error logs an error-level message with key-value pairs.
func (a *ApexAdapter) Error(msg string, args ...any) {
	entry := a.getEntry()
	entry = entry.WithFields(log.Fields(argsToFields(args)))
	entry.Error(msg)
}

// With returns a new Logger with additional context fields.
func (a *ApexAdapter) With(fields ...any) Logger {
	entry := a.getEntry()
	entry = entry.WithFields(log.Fields(argsToFields(fields)))
	newAdapter := &ApexAdapter{
		logger: a.logger,
		entry:  entry,
	}
	return newAdapter
}

// getEntry returns the current entry, or creates a new one from the logger if none exists.
func (a *ApexAdapter) getEntry() *log.Entry {
	if a.entry != nil {
		return a.entry
	}
	return log.NewEntry(a.logger)
}

// argsToFields converts alternating key-value pairs to a map for logging libraries.
func argsToFields(args []any) map[string]any {
	fields := make(map[string]any)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				fields[key] = args[i+1]
			}
		}
	}
	return fields
}
