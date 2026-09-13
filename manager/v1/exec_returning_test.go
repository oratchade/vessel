//go:build test

package v1

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/manager/v1/config"
)

type returningBatchDB struct {
	*batchDB

	mu      sync.Mutex
	queries []string
	args    [][]any
	rows    *db.RowsAdapter
}

func (r *returningBatchDB) ExecReturning(_ context.Context, query string, args ...any) (*db.RowsAdapter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries = append(r.queries, query)
	r.args = append(r.args, args)
	return r.rows, nil
}

func (r *returningBatchDB) execReturningRecords() ([]string, [][]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.queries...), append([][]any(nil), r.args...)
}

func newReturningTestEntry(database db.DB, batching bool) (*DBEntry, *dbEntryWorker, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	de := &DBEntry{
		ctx:                  ctx,
		cancel:               cancel,
		name:                 "test",
		db:                   database,
		logger:               &noOpLogger{},
		writeBatchingEnabled: batching,
		writeBatchMaxRows:    10,
		writeBatchMaxDelay:   time.Hour,
	}
	return de, &dbEntryWorker{queue: make(chan *Query, 100)}, cancel
}

func execReturningQuery(query string, args ...any) *Query {
	return &Query{
		Request:    ReqExecReturning,
		Data:       &QueryData{Query: query, Params: args},
		ResponseCh: make(chan *QueryResponse, 1),
	}
}

func TestExecReturningRequestConstant(t *testing.T) {
	assert.Equal(t, "execReturning", ReqExecReturning)
}

func TestWriteWorkerProcessesExecReturning(t *testing.T) {
	const query = `INSERT INTO "users" ("name") VALUES ($1) RETURNING "id";`
	rows := &db.RowsAdapter{}
	returning := &returningBatchDB{batchDB: &batchDB{}, rows: rows}

	tests := []struct {
		name     string
		database db.DB
		wantErr  string
	}{
		{name: "database implementing ReturningExecutor", database: returning},
		{
			name:     "database without ReturningExecutor",
			database: &batchDB{},
			wantErr:  `database entry "test" does not support RETURNING execution`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			de, worker, cancel := newReturningTestEntry(tt.database, false)
			defer cancel()
			de.wg.Add(1)
			go de.writeWorker(de.ctx, worker)

			q := execReturningQuery(query, "Ada")
			worker.queue <- q
			resp := readBatchResponse(t, q)
			cancel()
			de.wg.Wait()

			if tt.wantErr != "" {
				require.Error(t, resp.Error)
				assert.Contains(t, resp.Error.Error(), tt.wantErr)
				assert.Nil(t, resp.RawData)
				return
			}
			require.NoError(t, resp.Error)
			assert.Same(t, rows, resp.RawData)
			queries, args := returning.execReturningRecords()
			assert.Equal(t, []string{query}, queries)
			assert.Equal(t, [][]any{{"Ada"}}, args)
		})
	}
}

func TestWriteBatchingWorkerFlushesBeforeExecReturning(t *testing.T) {
	returning := &returningBatchDB{batchDB: &batchDB{}, rows: &db.RowsAdapter{}}
	de, worker, cancel := newReturningTestEntry(returning, true)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	insert := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	execReturning := execReturningQuery(`UPDATE "users" SET "name" = $1 RETURNING "id";`, "B")
	worker.queue <- insert
	worker.queue <- execReturning

	require.NoError(t, readBatchResponse(t, insert).Error)
	resp := readBatchResponse(t, execReturning)
	require.NoError(t, resp.Error)
	assert.Same(t, returning.rows, resp.RawData)
	_, insertsCalls, execCalls := returning.calls()
	assert.Equal(t, 1, insertsCalls)
	assert.Equal(t, 0, execCalls)
	queries, _ := returning.execReturningRecords()
	assert.Len(t, queries, 1)
}

func TestDBManagerExecReturningWithoutEntries(t *testing.T) {
	dm := &DBManager{}
	ctx := context.Background()

	ch, err := dm.ExecReturningAsync(ctx, `DELETE FROM "users" RETURNING "id"`)
	assert.Nil(t, ch)
	require.Error(t, err)

	rows, err := dm.ExecReturning(ctx, `DELETE FROM "users" RETURNING "id"`)
	assert.Nil(t, rows)
	require.Error(t, err)
}

// delayedReturningDB returns live rows from a one-connection SQLite pool only once
// released, so the response reaches the worker after the synchronous caller gave up.
type delayedReturningDB struct {
	db.DB

	started  chan struct{}
	release  chan struct{}
	returned chan struct{}
}

func (d *delayedReturningDB) ExecReturning(ctx context.Context, _ string, _ ...any) (*db.RowsAdapter, error) {
	close(d.started)
	<-d.release
	defer close(d.returned)
	return d.QueryRaw(ctx, "SELECT 1")
}

func TestDBManagerExecReturningClosesRowsAfterCallerStopsWaiting(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		cancel  bool
	}{
		{name: "caller cancels", timeout: defaultTimeout, cancel: true},
		{name: "default timeout", timeout: 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := defaultTimeout
			defaultTimeout = tt.timeout
			t.Cleanup(func() { defaultTimeout = previous })

			sqliteEntry, err := newDBEntry(context.Background(), &config.ManagerConfig{}, &config.ConfigEntry{
				Name:   "sqlite-one-conn",
				Type:   config.ReadWrite,
				SQLite: &db.SQLiteConfig{FilePath: ":memory:", MaxOpenConns: 1, MaxIdleConns: 1},
			}, &noOpLogger{})
			require.NoError(t, err)
			sqlite := sqliteEntry.db
			t.Cleanup(func() {
				sqliteEntry.cancel()
				_ = sqlite.Close()
			})

			delayed := &delayedReturningDB{
				DB:       sqlite,
				started:  make(chan struct{}),
				release:  make(chan struct{}),
				returned: make(chan struct{}),
			}
			de, worker, stopEntry := newReturningTestEntry(delayed, false)
			de.writeQueue = []*dbEntryWorker{worker}
			de.writeWorkerIdx, err = NewAtomicWrapCounter(int64(len(de.writeQueue)))
			require.NoError(t, err)
			de.lifecycle.store(lifecycleStarted)
			de.wg.Add(1)
			go de.writeWorker(de.ctx, worker)
			t.Cleanup(func() {
				stopEntry()
				de.wg.Wait()
			})

			dm := &DBManager{readWriteEntries: map[string]*DBEntry{de.name: de}}
			dm.lifecycle.store(lifecycleStarted)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				go func() {
					<-delayed.started
					cancel()
				}()
			}

			rows, err := dm.ExecReturning(ctx, `INSERT INTO "jobs" DEFAULT VALUES RETURNING "id"`)
			require.Error(t, err)
			assert.Nil(t, rows)

			close(delayed.release)
			<-delayed.returned

			execCtx, cancelExec := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelExec()
			_, err = sqlite.Exec(execCtx, "SELECT 1")
			require.NoError(t, err, "rows delivered after the caller stopped waiting must be closed")
		})
	}
}
