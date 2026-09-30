//go:build test

package v1_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
	migrate "tounilab.com/vessel/migrate/v1"
)

type logEntry struct {
	level string
	msg   string
	args  []any
}

// recordingLogger is a db.Logger that keeps every entry, including the fields added
// through With.
type recordingLogger struct {
	mu      *sync.Mutex
	entries *[]logEntry
	fields  []any
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{mu: &sync.Mutex{}, entries: &[]logEntry{}}
}

func (l *recordingLogger) record(level, msg string, args []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, logEntry{level: level, msg: msg, args: append(append([]any{}, l.fields...), args...)})
}

func (l *recordingLogger) Debug(msg string, args ...any) { l.record("debug", msg, args) }
func (l *recordingLogger) Info(msg string, args ...any)  { l.record("info", msg, args) }
func (l *recordingLogger) Warn(msg string, args ...any)  { l.record("warn", msg, args) }
func (l *recordingLogger) Error(msg string, args ...any) { l.record("error", msg, args) }

func (l *recordingLogger) With(fields ...any) db.Logger {
	return &recordingLogger{mu: l.mu, entries: l.entries, fields: append(append([]any{}, l.fields...), fields...)}
}

func field(args []any, key string) any {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == key {
			return args[i+1]
		}
	}
	return nil
}

func TestWithLoggerReportsProgressAndKeepsStatementsAtDebug(t *testing.T) {
	logger := newRecordingLogger()
	m, err := migrate.New(sqliteConfig(t), baseMigrations(), migrate.WithLogger(logger))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	_, err = m.Up(context.Background())
	require.NoError(t, err)

	var completed []string
	var statements, runs int
	for _, e := range *logger.entries {
		switch e.msg {
		case "migration completed":
			assert.Equal(t, "info", e.level)
			completed = append(completed, field(e.args, "source").(string))
		case "executing statement":
			// A baseline migration must not put a whole schema in the Info log.
			assert.Equal(t, "debug", e.level)
			statements++
		case "successfully migrated database":
			assert.Equal(t, "info", e.level)
			assert.Equal(t, int64(2), field(e.args, "current_version"))
			runs++
		}
	}
	assert.Equal(t, []string{"00001_create_items.sql", "00002_touch_trigger.sql"}, completed)
	assert.Equal(t, 2, statements)
	assert.Equal(t, 1, runs)
}
