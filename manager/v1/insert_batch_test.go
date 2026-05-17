//go:build test

package v1

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/fabric/db/v1"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

type batchDB struct {
	mu sync.Mutex

	insertCalls  int
	insertsCalls int
	execCalls    int

	insertTables  []string
	insertsTables []string
	insertRows    []map[string]any
	insertsRows   [][]map[string]any

	insertsErr          error
	insertsRowsAffected int64
}

func (b *batchDB) Insert(
	_ context.Context,
	table string,
	data map[string]any,
	_ *options.QueryOptions,
) (*db.ExecResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.insertCalls++
	b.insertTables = append(b.insertTables, table)
	b.insertRows = append(b.insertRows, data)
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (b *batchDB) Inserts(
	_ context.Context,
	table string,
	data []map[string]any,
	_ *options.QueryOptions,
) (*db.ExecResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.insertsCalls++
	b.insertsTables = append(b.insertsTables, table)
	b.insertsRows = append(b.insertsRows, append([]map[string]any(nil), data...))
	if b.insertsErr != nil {
		return nil, b.insertsErr
	}
	rowsAffected := b.insertsRowsAffected
	if rowsAffected == 0 {
		rowsAffected = int64(len(data))
	}
	return &db.ExecResult{RowsAffected: rowsAffected}, nil
}

func (b *batchDB) Exec(_ context.Context, _ string, _ ...any) (*db.ExecResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.execCalls++
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (b *batchDB) calls() (insertCalls int, insertsCalls int, execCalls int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.insertCalls, b.insertsCalls, b.execCalls
}

func (b *batchDB) Get(
	context.Context,
	string,
	[]string,
	[]cdt.Join,
	cdt.Condition,
	*options.QueryOptions,
) ([]map[string]any, error) {
	return nil, nil
}

func (b *batchDB) GetRaw(
	context.Context,
	string,
	[]string,
	[]cdt.Join,
	cdt.Condition,
	*options.QueryOptions,
) (*db.RowsAdapter, error) {
	return nil, nil
}

func (b *batchDB) GetByID(
	context.Context,
	string,
	any,
	[]cdt.Join,
	*options.QueryOptions,
) ([]map[string]any, error) {
	return nil, nil
}

func (b *batchDB) GetByIDRaw(
	context.Context,
	string,
	any,
	[]cdt.Join,
	*options.QueryOptions,
) (*db.RowsAdapter, error) {
	return nil, nil
}

func (b *batchDB) Query(context.Context, string, ...any) ([]map[string]any, error) {
	return nil, nil
}

func (b *batchDB) QueryRaw(context.Context, string, ...any) (*db.RowsAdapter, error) {
	return nil, nil
}

func (b *batchDB) GetQuery(
	string,
	[]string,
	[]cdt.Join,
	cdt.Condition,
	*options.QueryOptions,
) (string, []any, error) {
	return "", nil, nil
}

func (b *batchDB) GetByIDQuery(string, any, []cdt.Join, *options.QueryOptions) (string, []any, error) {
	return "", nil, nil
}

func (b *batchDB) InsertQuery(string, map[string]any, *options.QueryOptions) (string, []any, error) {
	return "", nil, nil
}

func (b *batchDB) InsertsQuery(string, []map[string]any, *options.QueryOptions) (string, []any, error) {
	return "", nil, nil
}

func (b *batchDB) Update(
	context.Context,
	string,
	map[string]any,
	[]cdt.Join,
	cdt.Condition,
	*options.QueryOptions,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (b *batchDB) UpdateQuery(
	string,
	map[string]any,
	[]cdt.Join,
	cdt.Condition,
	*options.QueryOptions,
) (string, []any, error) {
	return "", nil, nil
}

func (b *batchDB) Delete(
	context.Context,
	string,
	[]cdt.Join,
	cdt.Condition,
	*options.QueryOptions,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (b *batchDB) DeleteQuery(string, []cdt.Join, cdt.Condition, *options.QueryOptions) (string, []any, error) {
	return "", nil, nil
}

func (b *batchDB) Explain(context.Context, string, ...any) (*db.RowsAdapter, error) {
	return nil, nil
}

func (b *batchDB) Begin(context.Context) (db.Tx, error) {
	return nil, nil
}

func (b *batchDB) WithTransaction(context.Context, func(db.Tx) error) error {
	return nil
}

func (b *batchDB) Ping(context.Context) error {
	return nil
}

func (b *batchDB) PoolStats() (*db.PoolStatistics, error) {
	return &db.PoolStatistics{}, nil
}

func (b *batchDB) Close() error {
	return nil
}

func TestWriteWorkerBatchingDisabledUsesSingleInsert(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, false, 10, time.Hour)
	defer cancel()

	de.wg.Add(1)
	go de.writeWorker(de.ctx, worker)

	q := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	worker.queue <- q
	resp := readBatchResponse(t, q)
	cancel()
	de.wg.Wait()

	require.NoError(t, resp.Error)
	assert.Equal(t, int64(1), resp.ExecData.RowsAffected)
	insertCalls, insertsCalls, _ := fake.calls()
	assert.Equal(t, 1, insertCalls)
	assert.Equal(t, 0, insertsCalls)
}

func TestWriteBatchingWorkerFlushesCompatibleInsertsAtMaxRows(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 2, time.Hour)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	q1 := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	q2 := insertQuery("users", map[string]any{"email": "b@example.com", "name": "B"})
	worker.queue <- q1
	worker.queue <- q2

	require.NoError(t, readBatchResponse(t, q1).Error)
	require.NoError(t, readBatchResponse(t, q2).Error)
	insertCalls, insertsCalls, _ := fake.calls()
	assert.Equal(t, 0, insertCalls)
	assert.Equal(t, 1, insertsCalls)
	assert.Len(t, fake.insertsRows[0], 2)
}

func TestWriteBatchingWorkerFlushesOnDelay(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 10, time.Millisecond)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	q := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	worker.queue <- q

	require.NoError(t, readBatchResponse(t, q).Error)
	_, insertsCalls, _ := fake.calls()
	assert.Equal(t, 1, insertsCalls)
}

func TestWriteBatchingWorkerFlushesOnIncompatibleTable(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 10, time.Hour)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	q1 := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	q2 := insertQuery("accounts", map[string]any{"email": "b@example.com", "name": "B"})
	worker.queue <- q1
	worker.queue <- q2

	require.NoError(t, readBatchResponse(t, q1).Error)
	cancel()
	require.NoError(t, readBatchResponse(t, q2).Error)
	_, insertsCalls, _ := fake.calls()
	assert.Equal(t, 2, insertsCalls)
}

func TestWriteBatchingWorkerDoesNotBatchDifferentColumns(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 10, time.Hour)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	q1 := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	q2 := insertQuery("users", map[string]any{"email": "b@example.com", "age": 42})
	worker.queue <- q1
	worker.queue <- q2

	require.NoError(t, readBatchResponse(t, q1).Error)
	cancel()
	require.NoError(t, readBatchResponse(t, q2).Error)
	_, insertsCalls, _ := fake.calls()
	assert.Equal(t, 2, insertsCalls)
}

func TestWriteBatchingWorkerFlushesBeforeDirectWrite(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 10, time.Hour)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	insert := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	exec := &Query{
		Request:    ReqExec,
		Data:       &QueryData{Query: "UPDATE users SET name = ?", Params: []any{"B"}},
		ResponseCh: make(chan *QueryResponse, 1),
	}
	worker.queue <- insert
	worker.queue <- exec

	require.NoError(t, readBatchResponse(t, insert).Error)
	require.NoError(t, readBatchResponse(t, exec).Error)
	_, insertsCalls, execCalls := fake.calls()
	assert.Equal(t, 1, insertsCalls)
	assert.Equal(t, 1, execCalls)
}

func TestWriteBatchingWorkerFansOutBatchError(t *testing.T) {
	batchErr := errors.New("batch failed")
	fake := &batchDB{insertsErr: batchErr}
	de, worker, cancel := newBatchTestEntry(fake, true, 2, time.Hour)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	q1 := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	q2 := insertQuery("users", map[string]any{"email": "b@example.com", "name": "B"})
	worker.queue <- q1
	worker.queue <- q2

	assert.ErrorIs(t, readBatchResponse(t, q1).Error, batchErr)
	assert.ErrorIs(t, readBatchResponse(t, q2).Error, batchErr)
}

func TestWriteBatchingWorkerFlushesOnStop(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 10, time.Hour)
	worker.queue = make(chan *Query)
	go de.writeBatchingWorker(de.ctx, worker)

	q := insertQuery("users", map[string]any{"email": "a@example.com", "name": "A"})
	sent := make(chan struct{})
	go func() {
		worker.queue <- q
		close(sent)
	}()
	<-sent
	cancel()

	require.NoError(t, readBatchResponse(t, q).Error)
	_, insertsCalls, _ := fake.calls()
	assert.Equal(t, 1, insertsCalls)
}

func TestWriteBatchingWorkerConcurrentInsertProducers(t *testing.T) {
	fake := &batchDB{}
	de, worker, cancel := newBatchTestEntry(fake, true, 100, 2*time.Millisecond)
	defer cancel()
	go de.writeBatchingWorker(de.ctx, worker)

	const total = 50
	queries := make([]*Query, total)
	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			q := insertQuery("users", map[string]any{"email": idx, "name": "user"})
			queries[idx] = q
			worker.queue <- q
		}(i)
	}
	wg.Wait()

	for _, q := range queries {
		require.NoError(t, readBatchResponse(t, q).Error)
	}
	_, insertsCalls, _ := fake.calls()
	assert.GreaterOrEqual(t, insertsCalls, 1)
}

func newBatchTestEntry(
	fake *batchDB,
	enabled bool,
	maxRows int,
	maxDelay time.Duration,
) (*DBEntry, *dbEntryWorker, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := &dbEntryWorker{queue: make(chan *Query, 100)}
	de := &DBEntry{
		ctx:                  ctx,
		cancel:               cancel,
		name:                 "test",
		db:                   fake,
		logger:               &noOpLogger{},
		writeBatchingEnabled: enabled,
		writeBatchMaxRows:    maxRows,
		writeBatchMaxDelay:   maxDelay,
	}
	return de, worker, cancel
}

func insertQuery(table string, data map[string]any) *Query {
	return &Query{
		Request: ReqInsert,
		Data: &QueryData{
			Table: table,
			Data:  data,
		},
		ResponseCh: make(chan *QueryResponse, 1),
	}
}

func readBatchResponse(t *testing.T, q *Query) *QueryResponse {
	t.Helper()
	select {
	case resp := <-q.ResponseCh:
		return resp
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for batch response")
		return nil
	}
}
