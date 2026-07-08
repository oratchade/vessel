//go:build test

package v1

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/manager/v1/config"
)

// recordingLogger captures log messages so tests can assert which layer logged.
type recordingLogger struct {
	mu   sync.Mutex
	msgs []string
}

func (r *recordingLogger) record(msg string) {
	r.mu.Lock()
	r.msgs = append(r.msgs, msg)
	r.mu.Unlock()
}

func (r *recordingLogger) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.msgs...)
}

func (r *recordingLogger) Debug(msg string, _ ...any) { r.record(msg) }
func (r *recordingLogger) Info(msg string, _ ...any)  { r.record(msg) }
func (r *recordingLogger) Warn(msg string, _ ...any)  { r.record(msg) }
func (r *recordingLogger) Error(msg string, _ ...any) { r.record(msg) }
func (r *recordingLogger) With(_ ...any) db.Logger    { return r }

// TestNewDBEntryPassesLoggerToDriver ensures the manager's logger reaches the
// underlying DB driver, so driver-level logging (slow queries, query errors,
// transaction lifecycle) is not silenced when databases run through the manager.
func TestNewDBEntryPassesLoggerToDriver(t *testing.T) {
	logger := &recordingLogger{}

	mc := &config.ManagerConfig{}
	ce := &config.ConfigEntry{
		Name:   "sqlite-mem",
		Type:   config.ReadWrite,
		SQLite: &db.SQLiteConfig{FilePath: ":memory:"},
	}

	entry, err := newDBEntry(context.Background(), mc, ce, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = entry.db.Close() })

	// Trigger a driver-level error; the driver logs it via its SafeLogger only
	// if a non-nil logger was passed through to db.NewDB.
	_, err = entry.db.Get(context.Background(), "no_such_table", []string{"*"}, nil, nil, nil)
	require.Error(t, err)

	driverLogged := false
	for _, msg := range logger.messages() {
		if strings.Contains(msg, "sqlite.") {
			driverLogged = true
			break
		}
	}
	require.True(t, driverLogged,
		"expected a driver-level log entry (message containing \"sqlite.\"); "+
			"manager did not pass its logger to the DB layer. Got: %v", logger.messages())
}
