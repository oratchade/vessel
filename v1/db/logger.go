package db

// Logger is a minimal structured logging interface used by DB implementations.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(fields ...any) Logger
}
