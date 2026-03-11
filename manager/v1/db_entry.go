// Package v1 provides database manager entrypoint and implementation for multiple database engines management.
package v1

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/manager/v1/config"
)

// dbEntryWorker represents a worker that processes queries for a DBEntry. Each worker has its own queue of QueryData.
type dbEntryWorker struct {
	queue chan *Query
}

// DBEntry holds resolved runtime configuration for a database.
// All values are resolved from ConfigEntry with global defaults applied.
type DBEntry struct {
	ctx    context.Context
	cancel context.CancelFunc

	name   string
	dbType config.DBType

	db db.DB

	healthy        atomic.Bool
	healthInterval time.Duration
	priority       int

	writeQueue     []*dbEntryWorker
	readQueue      []*dbEntryWorker
	writeWorkerIdx AtomicWrapCounter
	readWorkerIdx  AtomicWrapCounter
}

// newDBEntry creates a new DBEntry instance.
// It initializes the database connection, worker queues, and other settings based on the provided configuration.
func newDBEntry(ctx context.Context, mc *config.ManagerConfig, cfg *config.ConfigEntry) (*DBEntry, error) {
	writeQueue := make([]*dbEntryWorker, mc.EntryWriteWorkers(cfg))
	for i := range writeQueue {
		writeQueue[i] = &dbEntryWorker{
			queue: make(chan *Query, mc.EntryWriteQueueSize(cfg)),
		}
	}

	readQueue := make([]*dbEntryWorker, mc.EntryReadWorkers(cfg))
	for i := range readQueue {
		readQueue[i] = &dbEntryWorker{
			queue: make(chan *Query, mc.EntryReadQueueSize(cfg)),
		}
	}

	//nolint:contextcheck
	db, err := db.NewDB(cfg.Config(), nil)
	if err != nil {
		return nil, fmt.Errorf("newDBEntry: failed to create DB instance: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("newDBEntry: failed to ping DB: %w", err)
	}

	c, cancel := context.WithCancel(ctx)
	dbe := &DBEntry{
		ctx:            c,
		cancel:         cancel,
		name:           cfg.Name,
		dbType:         cfg.Type,
		db:             db,
		healthInterval: mc.EntryHealthInterval(cfg),
		priority:       mc.EntryPriority(cfg),
		writeQueue:     writeQueue,
		readQueue:      readQueue,
		writeWorkerIdx: *NewAtomicWrapCounter(int64(len(writeQueue))),
		readWorkerIdx:  *NewAtomicWrapCounter(int64(len(readQueue))),
	}
	return dbe, nil
}

// Priority returns the priority of the DBEntry, which is used for query routing and load balancing decisions.
func (de *DBEntry) Priority() int {
	return de.priority
}

// Health returns the health status of the DBEntry.
func (de *DBEntry) Health() bool {
	return de.healthy.Load()
}

// start launches worker goroutines for processing read and write queries.
func (de *DBEntry) start(ctx context.Context) {
	for i := range de.writeQueue {
		go de.writeWorker(ctx, de.writeQueue[i])
	}
	for i := range de.readQueue {
		go de.readWorker(ctx, de.readQueue[i])
	}

	go de.healthCheck(ctx)
}

// stop closes all worker goroutines and closes the database connection.
func (de *DBEntry) stop() {
	for i := range de.writeQueue {
		close(de.writeQueue[i].queue)
	}
	for i := range de.readQueue {
		close(de.readQueue[i].queue)
	}

	_ = de.db.Close()
	de.cancel()
}

// healthCheck periodically checks the health status of the database connection.
func (de *DBEntry) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(de.healthInterval)
	defer ticker.Stop()

	failureCount := 0
	const maxFailures = 5 // Mark unhealthy after 5 consecutive failures

	for {
		select {
		case <-ticker.C:
			err := de.db.Ping(ctx)
			if err != nil {
				failureCount++
				if failureCount >= maxFailures {
					de.healthy.Store(false)
				}

				continue
			}
			// Success: reset failure count and mark healthy
			failureCount = 0
			de.healthy.Store(true)
		case <-ctx.Done():
			return
		}
	}
}

// writeWorker processes write queries from its queue and executes them against the database.
func (de *DBEntry) writeWorker(ctx context.Context, w *dbEntryWorker) {
	for {
		select {
		case qd := <-w.queue:
			switch qd.Request {
			case ReqInsert:
				resp, err := de.db.Insert(
					ctx,
					qd.Data.Table,
					qd.Data.Data,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{ExecData: resp, Error: err}
			case ReqInserts:
				resp, err := de.db.Inserts(
					ctx,
					qd.Data.Table,
					qd.Data.BulkData,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{ExecData: resp, Error: err}
			case ReqUpdate:
				resp, err := de.db.Update(
					ctx,
					qd.Data.Table,
					qd.Data.Data,
					qd.Data.Conditions,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{ExecData: resp, Error: err}
			case ReqDelete:
				resp, err := de.db.Delete(
					ctx,
					qd.Data.Table,
					qd.Data.Conditions,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{ExecData: resp, Error: err}
			case ReqExec:
				resp, err := de.db.Exec(
					ctx,
					qd.Data.Query,
					qd.Data.Params...,
				)
				qd.ResponseCh <- &QueryResponse{ExecData: resp, Error: err}
			}
		case <-ctx.Done():
			return
		}
	}
}

// readWorker processes read queries from its queue and executes them against the database.
func (de *DBEntry) readWorker(ctx context.Context, w *dbEntryWorker) {
	for {
		select {
		case qd := <-w.queue:
			switch qd.Request {
			case ReqGet:
				resp, err := de.db.Get(
					ctx,
					qd.Data.Table,
					qd.Data.Columns,
					qd.Data.Joins,
					qd.Data.Conditions,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{Data: resp, Error: err}
			case ReqGetRaw:
				resp, err := de.db.GetRaw(
					ctx,
					qd.Data.Table,
					qd.Data.Columns,
					qd.Data.Joins,
					qd.Data.Conditions,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{RawData: resp, Error: err}
			case ReqGetByID:
				resp, err := de.db.GetByID(
					ctx,
					qd.Data.Table,
					qd.Data.ID,
					qd.Data.Joins,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{Data: resp, Error: err}
			case ReqGetByIDRaw:
				resp, err := de.db.GetByIDRaw(
					ctx,
					qd.Data.Table,
					qd.Data.ID,
					qd.Data.Joins,
					qd.Data.Opts,
				)
				qd.ResponseCh <- &QueryResponse{RawData: resp, Error: err}
			case ReqQuery:
				resp, err := de.db.Query(
					ctx,
					qd.Data.Query,
					qd.Data.Params...,
				)
				qd.ResponseCh <- &QueryResponse{Data: resp, Error: err}
			case ReqQueryRaw:
				resp, err := de.db.QueryRaw(
					ctx,
					qd.Data.Query,
					qd.Data.Params...,
				)
				qd.ResponseCh <- &QueryResponse{RawData: resp, Error: err}
			}
		case <-ctx.Done():
			return
		}
	}
}

// RoundRobinQueueWrite enqueues a write query to the appropriate worker queue based on round-robin distribution.
func (de *DBEntry) roundRobinQueueWrite(ctx context.Context, qd *Query) error {
	idx := de.writeWorkerIdx.Next()
	w := de.writeQueue[idx]

	select {
	case w.queue <- qd:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("roundRobinQueueWrite: context done: %w", ctx.Err())
	}
}

// RoundRobinQueueRead enqueues a read query to the appropriate worker queue based on round-robin distribution.
func (de *DBEntry) roundRobinQueueRead(ctx context.Context, qd *Query) error {
	idx := de.readWorkerIdx.Next()
	w := de.readQueue[idx]

	select {
	case w.queue <- qd:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("roundRobinQueueRead: context done: %w", ctx.Err())
	}
}
