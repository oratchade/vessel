//go:build test

package v1

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/manager/v1/config"
	"tounilab.com/vessel/pkg/query/options"
)

// ctxRecordingDB records the context each driver call runs with. The first call
// blocks until its context ends, as against a stalled database; later ones return.
type ctxRecordingDB struct {
	db.DB
	calls   atomic.Int32
	started chan struct{}
	ended   chan error
}

func newCtxRecordingDB() *ctxRecordingDB {
	return &ctxRecordingDB{started: make(chan struct{}), ended: make(chan error, 1)}
}

func (d *ctxRecordingDB) run(ctx context.Context) error {
	if d.calls.Add(1) > 1 {
		return nil
	}
	close(d.started)
	<-ctx.Done()
	d.ended <- ctx.Err()
	return ctx.Err()
}

func (d *ctxRecordingDB) Query(ctx context.Context, _ string, _ ...any) ([]map[string]any, error) {
	return nil, d.run(ctx)
}

func (d *ctxRecordingDB) QueryRaw(ctx context.Context, _ string, _ ...any) (*db.RowsAdapter, error) {
	return nil, d.run(ctx)
}

// startStubManager runs one read or write worker around database.
func startStubManager(t *testing.T, database db.DB, write, batching bool) (*DBManager, *DBEntry) {
	t.Helper()
	de, worker, stopEntry := newReturningTestEntry(database, batching)
	counter, err := NewAtomicWrapCounter(1)
	require.NoError(t, err)
	dm := &DBManager{}
	if write {
		de.writeQueue, de.writeWorkerIdx = []*dbEntryWorker{worker}, counter
		dm.readWriteEntries = map[string]*DBEntry{de.name: de}
	} else {
		de.readQueue, de.readWorkerIdx = []*dbEntryWorker{worker}, counter
		dm.readOnlyEntries = map[string]*DBEntry{de.name: de}
	}
	de.lifecycle.store(lifecycleStarted)
	dm.lifecycle.store(lifecycleStarted)
	de.wg.Add(1)
	if write {
		go de.writeWorker(de.ctx, worker)
	} else {
		go de.readWorker(de.ctx, worker)
	}
	t.Cleanup(func() {
		stopEntry()
		de.wg.Wait()
	})
	return dm, de
}

func requireEnded(t *testing.T, d *ctxRecordingDB, want error, why string) {
	t.Helper()
	select {
	case got := <-d.ended:
		require.ErrorIs(t, got, want, why)
	case <-time.After(2 * time.Second):
		t.Fatal(why + ": the driver call was never cancelled")
	}
}

// Cancelling the caller cancels the driver call, so the worker is free again (#82).
func TestCallerCancellationReachesTheDriver(t *testing.T) {
	stub := newCtxRecordingDB()
	dm, _ := startStubManager(t, stub, false, false)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-stub.started; cancel() }()
	_, err := dm.Query(ctx, "SELECT 1")
	require.Error(t, err)
	requireEnded(t, stub, context.Canceled, "the caller's cancellation")

	// The worker takes the next query.
	_, err = dm.Query(context.Background(), "SELECT 1")
	require.NoError(t, err)
}

// A caller with no deadline waits at most defaultTimeout; the query must stop then too.
func TestDefaultTimeoutBoundsExecution(t *testing.T) {
	previous := defaultTimeout
	defaultTimeout = 50 * time.Millisecond
	t.Cleanup(func() { defaultTimeout = previous })

	stub := newCtxRecordingDB()
	dm, _ := startStubManager(t, stub, false, false)
	_, err := dm.Query(context.Background(), "SELECT 1")
	require.Error(t, err)
	requireEnded(t, stub, context.DeadlineExceeded, "the default timeout")
}

// Raw rows cannot take a default deadline, since they outlive the call; a caller who
// stops waiting cancels the query instead.
func TestRawQueryStopsWhenTheCallerStopsWaiting(t *testing.T) {
	previous := defaultTimeout
	defaultTimeout = 50 * time.Millisecond
	t.Cleanup(func() { defaultTimeout = previous })

	stub := newCtxRecordingDB()
	dm, _ := startStubManager(t, stub, false, false)
	rows, err := dm.QueryRaw(context.Background(), "SELECT 1")
	require.Error(t, err)
	assert.Nil(t, rows)
	requireEnded(t, stub, context.Canceled, "a raw query whose caller stopped waiting")
}

// countingDB counts driver calls and answers at once.
type countingDB struct {
	db.DB
	calls atomic.Int32
}

func (d *countingDB) Query(context.Context, string, ...any) ([]map[string]any, error) {
	d.calls.Add(1)
	return nil, nil
}

// A query whose caller gave up while it waited in the queue is not run.
func TestQueryAbandonedWhileQueuedIsNotRun(t *testing.T) {
	counting := &countingDB{}
	de, worker, stopEntry := newReturningTestEntry(counting, false)
	t.Cleanup(stopEntry)
	counter, err := NewAtomicWrapCounter(1)
	require.NoError(t, err)
	de.readQueue, de.readWorkerIdx = []*dbEntryWorker{worker}, counter
	de.lifecycle.store(lifecycleStarted)
	dm := &DBManager{readOnlyEntries: map[string]*DBEntry{de.name: de}}
	dm.lifecycle.store(lifecycleStarted)

	// No worker yet: the query waits in the queue while its caller gives up.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = dm.Query(ctx, "SELECT 1")
	require.Error(t, err)
	require.Len(t, worker.queue, 1)

	de.wg.Add(1)
	go de.readWorker(de.ctx, worker)
	require.Eventually(t, func() bool { return len(worker.queue) == 0 }, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	assert.Zero(t, counting.calls.Load(), "a query nobody waits for was run")
	stopEntry()
	de.wg.Wait()
}

// Raw rows are read after the worker has moved on; the query's context must stay
// alive until they are closed.
func TestRawRowsOutliveTheWorker(t *testing.T) {
	sqliteEntry, err := newDBEntry(context.Background(), &config.ManagerConfig{}, &config.ConfigEntry{
		Name:   "sqlite-rows",
		Type:   config.ReadWrite,
		SQLite: &db.SQLiteConfig{FilePath: ":memory:", MaxOpenConns: 1, MaxIdleConns: 1},
	}, &noOpLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { sqliteEntry.cancel(); _ = sqliteEntry.db.Close() })

	dm, _ := startStubManager(t, sqliteEntry.db, false, false)
	rows, err := dm.QueryRaw(context.Background(), "SELECT 1 UNION ALL SELECT 2")
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond) // the worker is long done
	n := 0
	for rows.Next() {
		n++
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, 2, n)
	require.NoError(t, rows.Close())
}

// Shutdown still cancels a query in flight, whatever its caller's context.
func TestShutdownCancelsAQueryInFlight(t *testing.T) {
	stub := newCtxRecordingDB()
	dm, de := startStubManager(t, stub, false, false)
	_, err := dm.QueryAsync(context.Background(), "SELECT 1")
	require.NoError(t, err)
	<-stub.started
	de.cancel()
	requireEnded(t, stub, context.Canceled, "shutdown")
}

// batchCtxDB records whether a batch's context had ended when it ran.
type batchCtxDB struct {
	db.DB
	ranWith chan error
}

func (d *batchCtxDB) Inserts(ctx context.Context, _ string, _ []map[string]any, _ *options.QueryOptions) (*db.ExecResult, error) {
	d.ranWith <- ctx.Err()
	return &db.ExecResult{RowsAffected: 2}, nil
}

// One batch carries several callers' rows: no single caller can cancel it.
func TestOneCallerDoesNotCancelABatch(t *testing.T) {
	batch := &batchCtxDB{ranWith: make(chan error, 1)}
	dm, de := startStubManager(t, batch, true, true)
	de.writeBatchMaxRows = 2

	ctx, cancel := context.WithCancel(context.Background())
	gone, err := dm.InsertAsync(ctx, "jobs", map[string]any{"n": 1}, nil)
	require.NoError(t, err)
	cancel()
	_, err = dm.InsertAsync(context.Background(), "jobs", map[string]any{"n": 2}, nil)
	require.NoError(t, err)
	select {
	case err := <-batch.ranWith:
		require.NoError(t, err, "the batch ran with a cancelled context")
	case <-time.After(2 * time.Second):
		t.Fatal("the batch never ran")
	}
	<-gone
}
