//go:build test

package v1_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	migrate "tounilab.com/vessel/migrate/v1"
)

// recordingHandler keeps every record at or above its level, with attributes from
// both the record and With.
type recordingHandler struct {
	level   slog.Level
	mu      *sync.Mutex
	records *[]slog.Record
	attrs   []slog.Attr
}

func newRecordingHandler(level slog.Level) *recordingHandler {
	return &recordingHandler{level: level, mu: &sync.Mutex{}, records: &[]slog.Record{}}
}

func (h *recordingHandler) Enabled(_ context.Context, level slog.Level) bool { return level >= h.level }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	r = r.Clone()
	r.AddAttrs(h.attrs...)
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

func attr(r slog.Record, key string) any {
	var v any
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v = a.Value.Any()
			return false
		}
		return true
	})
	return v
}

func migrateWith(t *testing.T, handler slog.Handler) {
	t.Helper()
	logger := slog.New(handler).With("database", "items_db")
	m, err := migrate.New(sqliteConfig(t), baseMigrations(), migrate.WithSlog(logger))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })
	_, err = m.Up(context.Background())
	require.NoError(t, err)
}

func TestWithSlogReportsProgressAndKeepsStatementsAtDebug(t *testing.T) {
	handler := newRecordingHandler(slog.LevelDebug)
	migrateWith(t, handler)

	var completed []string
	var statements, runs int
	for _, r := range *handler.records {
		// The application's own attributes reach every record.
		assert.Equal(t, "items_db", attr(r, "database"), r.Message)
		switch r.Message {
		case "migration completed":
			assert.Equal(t, slog.LevelInfo, r.Level)
			completed = append(completed, attr(r, "source").(string))
		case "executing statement":
			// A baseline migration must not put a whole schema in the Info log.
			assert.Equal(t, slog.LevelDebug, r.Level)
			statements++
		case "successfully migrated database":
			assert.Equal(t, slog.LevelInfo, r.Level)
			assert.Equal(t, int64(2), attr(r, "current_version"))
			runs++
		}
	}
	assert.Equal(t, []string{"00001_create_items.sql", "00002_touch_trigger.sql"}, completed)
	assert.Equal(t, 2, statements)
	assert.Equal(t, 1, runs)
}

func TestWithSlogDropsStatementsBelowTheHandlersLevel(t *testing.T) {
	handler := newRecordingHandler(slog.LevelInfo)
	migrateWith(t, handler)

	require.NotEmpty(t, *handler.records)
	for _, r := range *handler.records {
		assert.NotEqual(t, "executing statement", r.Message, "a statement reached an Info-level handler")
	}
}
