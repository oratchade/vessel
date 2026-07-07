package v1

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/manager/v1/config"
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

	db     db.DB
	logger db.Logger

	healthy        atomic.Bool
	healthInterval time.Duration
	priority       int

	writeQueue     []*dbEntryWorker
	readQueue      []*dbEntryWorker
	writeWorkerIdx *AtomicWrapCounter
	readWorkerIdx  *AtomicWrapCounter

	writeBatchingEnabled bool
	writeBatchMaxRows    int
	writeBatchMaxDelay   time.Duration

	lifecycle lifecycle
	wg        sync.WaitGroup
	stopOnce  sync.Once
}

// newDBEntry creates a new DBEntry instance.
// It initializes the database connection, worker queues, and other settings
// based on the provided configuration.
func newDBEntry(
	ctx context.Context,
	mc *config.ManagerConfig,
	cfg *config.ConfigEntry,
	logger db.Logger,
) (*DBEntry, error) {
	// Ensure logger is never nil - use a no-op logger if not provided
	if logger == nil {
		logger = &noOpLogger{}
	}

	logger.Info("Creating database entry",
		"name", cfg.Name,
		"type", cfg.Type,
		"priority", mc.EntryPriority(cfg),
	)

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

	logger.Debug("Initialized worker queues",
		"name", cfg.Name,
		"write_workers", len(writeQueue),
		"read_workers", len(readQueue),
		"write_queue_size", mc.EntryWriteQueueSize(cfg),
		"read_queue_size", mc.EntryReadQueueSize(cfg),
		"write_batching_enabled", mc.EntryWriteBatchingEnabled(cfg),
		"write_batch_max_rows", mc.EntryWriteBatchMaxRows(cfg),
		"write_batch_max_delay_ms", mc.EntryWriteBatchMaxDelay(cfg).Milliseconds(),
	)

	//nolint:contextcheck
	dbInstance, err := db.NewDB(cfg.Config(), logger)
	if err != nil {
		logger.Error("Failed to create database instance",
			"name", cfg.Name,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("newDBEntry: failed to create DB instance: %w", err)
	}

	if err := dbInstance.Ping(ctx); err != nil {
		logger.Error("Failed to ping database",
			"name", cfg.Name,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("newDBEntry: failed to ping DB: %w", err)
	}

	logger.Info("Database entry created and ping successful",
		"name", cfg.Name,
		"health_interval_ms", mc.EntryHealthInterval(cfg).Milliseconds(),
	)

	var writeWorkerIdxCounter *AtomicWrapCounter
	if len(writeQueue) > 0 {
		c, err := NewAtomicWrapCounter(int64(len(writeQueue)))
		if err != nil {
			logger.Error("Failed to create write worker index counter",
				"name", cfg.Name,
				"queue_size", len(writeQueue),
				"error", err.Error(),
			)
			return nil, fmt.Errorf("newDBEntry: failed to create write worker counter: %w", err)
		}
		writeWorkerIdxCounter = c
	}

	var readWorkerIdxCounter *AtomicWrapCounter
	if len(readQueue) > 0 {
		c, err := NewAtomicWrapCounter(int64(len(readQueue)))
		if err != nil {
			logger.Error("Failed to create read worker index counter",
				"name", cfg.Name,
				"queue_size", len(readQueue),
				"error", err.Error(),
			)
			return nil, fmt.Errorf("newDBEntry: failed to create read worker counter: %w", err)
		}
		readWorkerIdxCounter = c
	}

	c, cancel, releaseCancel, cleanupCancel := newDBEntryContext(ctx)
	defer cleanupCancel()
	dbe := &DBEntry{
		ctx:                  c,
		cancel:               cancel,
		name:                 cfg.Name,
		dbType:               cfg.Type,
		db:                   dbInstance,
		logger:               logger,
		healthInterval:       mc.EntryHealthInterval(cfg),
		healthy:              atomic.Bool{},
		priority:             mc.EntryPriority(cfg),
		writeQueue:           writeQueue,
		readQueue:            readQueue,
		writeWorkerIdx:       writeWorkerIdxCounter,
		readWorkerIdx:        readWorkerIdxCounter,
		writeBatchingEnabled: mc.EntryWriteBatchingEnabled(cfg),
		writeBatchMaxRows:    mc.EntryWriteBatchMaxRows(cfg),
		writeBatchMaxDelay:   mc.EntryWriteBatchMaxDelay(cfg),
	}
	dbe.healthy.Store(true) // Start as healthy
	releaseCancel()
	return dbe, nil
}

func newDBEntryContext(parent context.Context) (context.Context, context.CancelFunc, func(), func()) {
	c, cancel := context.WithCancel(parent)
	cancelOnError := true
	release := func() {
		cancelOnError = false
	}
	cleanup := func() {
		if cancelOnError {
			cancel()
		}
	}
	return c, cancel, release, cleanup
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
func (de *DBEntry) start() {
	if !de.lifecycle.compareAndSwap(lifecycleCreated, lifecycleStarted) {
		return
	}

	de.logger.Info("Starting database entry workers",
		"name", de.name,
		"write_workers", len(de.writeQueue),
		"read_workers", len(de.readQueue),
	)

	for i := range de.writeQueue {
		de.wg.Add(1)
		go de.writeWorker(de.ctx, de.writeQueue[i])
	}
	for i := range de.readQueue {
		de.wg.Add(1)
		go de.readWorker(de.ctx, de.readQueue[i])
	}

	de.wg.Add(1)
	go de.healthCheck(de.ctx)

	de.logger.Debug("Database entry workers started and health check initiated",
		"name", de.name,
	)
}

// stop closes all worker goroutines and closes the database connection.
// It cancels the context first to allow workers to exit via ctx.Done(),
// then closes channels and the database.
func (de *DBEntry) stop() {
	de.stopOnce.Do(func() {
		de.logger.Info("Stopping database entry",
			"name", de.name,
		)

		state := de.lifecycle.load()
		if state == lifecycleCreated {
			de.lifecycle.store(lifecycleStopping)
		} else if !de.lifecycle.compareAndSwap(lifecycleStarted, lifecycleStopping) &&
			de.lifecycle.load() != lifecycleStopping {
			return
		}

		de.cancel()
		de.wg.Wait()
		_ = de.db.Close()
		de.lifecycle.store(lifecycleStopped)

		de.logger.Debug("Database entry stopped",
			"name", de.name,
		)
	})
}

// healthCheck periodically checks the health status of the database connection.
func (de *DBEntry) healthCheck(ctx context.Context) {
	defer de.wg.Done()

	ticker := time.NewTicker(de.healthInterval)
	defer ticker.Stop()

	failureCount := 0
	const maxFailures = 5 // Mark unhealthy after 5 consecutive failures

	de.logger.Info("Health check started",
		"name", de.name,
		"interval_ms", de.healthInterval.Milliseconds(),
		"max_failures", maxFailures,
	)

	for {
		select {
		case <-ticker.C:
			err := de.db.Ping(ctx)
			if err != nil {
				failureCount = de.handleHealthCheckFailure(
					failureCount,
					maxFailures,
					err,
				)
				continue
			}

			de.handleHealthCheckSuccess()
			failureCount = 0
		case <-ctx.Done():
			de.logger.Debug("Health check stopped",
				"name", de.name,
			)
			return
		}
	}
}

// processWriteRequest handles a write request and returns the response.
func (de *DBEntry) processWriteRequest(ctx context.Context, qd *Query) *QueryResponse {
	switch qd.Request {
	case ReqInsert:
		resp, err := de.db.Insert(
			ctx,
			qd.Data.Table,
			qd.Data.Data,
			qd.Data.Opts,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	case ReqInserts:
		resp, err := de.db.Inserts(
			ctx,
			qd.Data.Table,
			qd.Data.BulkData,
			qd.Data.Opts,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	case ReqUpsert:
		resp, err := de.db.Upsert(
			ctx,
			qd.Data.Table,
			qd.Data.Data,
			qd.Data.UpsertOpts,
			qd.Data.Opts,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	case ReqUpserts:
		resp, err := de.db.Upserts(
			ctx,
			qd.Data.Table,
			qd.Data.BulkData,
			qd.Data.UpsertOpts,
			qd.Data.Opts,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	case ReqUpdate:
		resp, err := de.db.Update(
			ctx,
			qd.Data.Table,
			qd.Data.Data,
			qd.Data.Joins,
			qd.Data.Conditions,
			qd.Data.Opts,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	case ReqDelete:
		resp, err := de.db.Delete(
			ctx,
			qd.Data.Table,
			qd.Data.Joins,
			qd.Data.Conditions,
			qd.Data.Opts,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	case ReqExec:
		resp, err := de.db.Exec(
			ctx,
			qd.Data.Query,
			qd.Data.Params...,
		)
		return &QueryResponse{ExecData: resp, Error: err}
	}
	return &QueryResponse{}
}

// writeWorker processes write queries from its queue and executes them against the database.
func (de *DBEntry) writeWorker(ctx context.Context, w *dbEntryWorker) {
	defer de.wg.Done()

	if de.writeBatchingEnabled {
		de.writeBatchingWorker(ctx, w)
		return
	}

	for {
		select {
		case qd, ok := <-w.queue:
			if !ok {
				return
			}
			response := de.processWriteRequest(ctx, qd)
			// Send response with timeout to prevent worker from blocking indefinitely
			de.sendResponseWithTimeout(ctx, qd, response)
		case <-ctx.Done():
			return
		}
	}
}

func (de *DBEntry) writeBatchingWorker(ctx context.Context, w *dbEntryWorker) {
	batch := &insertBatch{}
	var timer *time.Timer
	var timerC <-chan time.Time
	defer stopBatchTimer(timer)

	startTimer := func() {
		if timer != nil {
			stopBatchTimer(timer)
		}
		timer = time.NewTimer(de.writeBatchMaxDelay)
		timerC = timer.C
	}
	stopTimer := func() {
		if timer != nil {
			stopBatchTimer(timer)
			timer = nil
		}
		timerC = nil
	}
	flush := func(flushCtx context.Context) {
		stopTimer()
		batch.flush(flushCtx, de)
	}

	for {
		select {
		case qd, ok := <-w.queue:
			if !ok {
				flush(context.WithoutCancel(ctx))
				return
			}
			de.handleBatchingWrite(ctx, qd, batch, startTimer, flush)
		case <-timerC:
			flush(ctx)
		case <-ctx.Done():
			flush(context.WithoutCancel(ctx))
			return
		}
	}
}

func (de *DBEntry) handleBatchingWrite(
	ctx context.Context,
	qd *Query,
	batch *insertBatch,
	startTimer func(),
	flush func(context.Context),
) {
	if qd.Request != ReqInsert {
		flush(ctx)
		response := de.processWriteRequest(ctx, qd)
		de.sendResponseWithTimeout(ctx, qd, response)
		return
	}
	if !batch.compatible(qd) {
		flush(ctx)
	}
	if batch.empty() {
		startTimer()
	}
	batch.add(qd)
	if batch.len() >= de.writeBatchMaxRows {
		flush(ctx)
	}
}

func stopBatchTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// processReadRequest handles a read request and returns the response.
func (de *DBEntry) processReadRequest(ctx context.Context, qd *Query) *QueryResponse {
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
		return &QueryResponse{Data: resp, Error: err}
	case ReqGetRaw:
		resp, err := de.db.GetRaw(
			ctx,
			qd.Data.Table,
			qd.Data.Columns,
			qd.Data.Joins,
			qd.Data.Conditions,
			qd.Data.Opts,
		)
		return &QueryResponse{RawData: resp, Error: err}
	case ReqGetByID:
		resp, err := de.db.GetByID(
			ctx,
			qd.Data.Table,
			qd.Data.ID,
			qd.Data.Joins,
			qd.Data.Opts,
		)
		return &QueryResponse{Data: resp, Error: err}
	case ReqGetByIDRaw:
		resp, err := de.db.GetByIDRaw(
			ctx,
			qd.Data.Table,
			qd.Data.ID,
			qd.Data.Joins,
			qd.Data.Opts,
		)
		return &QueryResponse{RawData: resp, Error: err}
	case ReqQuery:
		resp, err := de.db.Query(
			ctx,
			qd.Data.Query,
			qd.Data.Params...,
		)
		return &QueryResponse{Data: resp, Error: err}
	case ReqQueryRaw:
		resp, err := de.db.QueryRaw(
			ctx,
			qd.Data.Query,
			qd.Data.Params...,
		)
		return &QueryResponse{RawData: resp, Error: err}
	}
	return &QueryResponse{}
}

// readWorker processes read queries from its queue and executes them against the database.
func (de *DBEntry) readWorker(ctx context.Context, w *dbEntryWorker) {
	defer de.wg.Done()

	for {
		select {
		case qd, ok := <-w.queue:
			if !ok {
				return
			}
			response := de.processReadRequest(ctx, qd)
			// Send response with timeout to prevent worker from blocking indefinitely
			de.sendResponseWithTimeout(ctx, qd, response)
		case <-ctx.Done():
			return
		}
	}
}

func (de *DBEntry) sendResponseWithTimeout(
	ctx context.Context,
	qd *Query,
	response *QueryResponse,
) {
	const responseSendTimeout = 5 * time.Second
	select {
	case qd.ResponseCh <- response:
		// Response sent successfully
	case <-time.After(responseSendTimeout):
		if qd.ResponseCh != nil && de.logger != nil {
			de.logger.Warn("Response send timeout",
				"name", de.name,
				"request_type", qd.Request,
				"reason", "consumer_not_reading",
			)
		}
	case <-ctx.Done():
		return
	}
}

// RoundRobinQueueWrite enqueues a write query to the appropriate worker queue based on round-robin distribution.
func (de *DBEntry) roundRobinQueueWrite(ctx context.Context, qd *Query) error {
	if err := de.lifecycle.runningError(); err != nil {
		return err
	}
	if de.writeWorkerIdx == nil {
		return fmt.Errorf("roundRobinQueueWrite: no write workers available")
	}
	idx := de.nextWriteWorkerIndex(qd)
	w := de.writeQueue[idx]

	select {
	case w.queue <- qd:
		return nil
	case <-de.ctx.Done():
		return fmt.Errorf("roundRobinQueueWrite: %w", ErrManagerClosed)
	case <-ctx.Done():
		return fmt.Errorf("roundRobinQueueWrite: context done: %w", ctx.Err())
	}
}

func (de *DBEntry) nextWriteWorkerIndex(qd *Query) int64 {
	if de.writeBatchingEnabled && qd != nil && qd.Request == ReqInsert && qd.Data != nil {
		key := qd.Data.Table + "\x00" + insertColumnsKey(qd.Data.Data) + "\x00" + fmt.Sprintf("%#v", qd.Data.Opts)
		mod := int64(len(de.writeQueue))
		var idx int64
		var position int64
		for _, b := range []byte(key) {
			idx = (idx*31 + int64(b) + position) % mod
			position++
		}
		return idx
	}
	return de.writeWorkerIdx.Next()
}

// RoundRobinQueueRead enqueues a read query to the appropriate worker queue based on round-robin distribution.
func (de *DBEntry) roundRobinQueueRead(ctx context.Context, qd *Query) error {
	if err := de.lifecycle.runningError(); err != nil {
		return err
	}
	if de.readWorkerIdx == nil {
		return fmt.Errorf("roundRobinQueueRead: no read workers available")
	}
	idx := de.readWorkerIdx.Next()
	w := de.readQueue[idx]

	select {
	case w.queue <- qd:
		return nil
	case <-de.ctx.Done():
		return fmt.Errorf("roundRobinQueueRead: %w", ErrManagerClosed)
	case <-ctx.Done():
		return fmt.Errorf("roundRobinQueueRead: context done: %w", ctx.Err())
	}
}

// noOpLogger is a minimal logger that does nothing, used when no logger is provided.
// This satisfies the db.Logger interface without any output.
type noOpLogger struct{}

// Debug does nothing.
func (nol *noOpLogger) Debug(msg string, args ...any) {}

// Info does nothing.
func (nol *noOpLogger) Info(msg string, args ...any) {}

// Warn does nothing.
func (nol *noOpLogger) Warn(msg string, args ...any) {}

// Error does nothing.
func (nol *noOpLogger) Error(msg string, args ...any) {}

// With returns itself, satisfying the Logger interface.
func (nol *noOpLogger) With(fields ...any) db.Logger {
	return nol
}

// handleHealthCheckFailure handles a failed health check and returns the updated failure count.
func (de *DBEntry) handleHealthCheckFailure(
	failureCount int,
	maxFailures int,
	err error,
) int {
	failureCount++
	wasHealthy := de.healthy.Load()

	// Classify the error for appropriate logging
	errorType, logLevel := db.ClassifyError(err)

	// Log at appropriate level based on error classification
	switch logLevel {
	case db.LogLevelError:
		de.logger.Error("Health check failed",
			"name", de.name,
			"failure_count", failureCount,
			"error_type", errorType,
			"error", err.Error(),
		)
	case db.LogLevelWarn:
		de.logger.Warn("Health check failed",
			"name", de.name,
			"failure_count", failureCount,
			"error_type", errorType,
			"error", err.Error(),
		)
	default:
		de.logger.Debug("Health check failed",
			"name", de.name,
			"failure_count", failureCount,
			"error_type", errorType,
			"error", err.Error(),
		)
	}

	if failureCount >= maxFailures && wasHealthy {
		de.healthy.Store(false)
		de.logger.Warn("Database entry marked unhealthy",
			"name", de.name,
			"consecutive_failures", failureCount,
			"priority", de.priority,
		)
	}

	return failureCount
}

// handleHealthCheckSuccess handles a successful health check.
func (de *DBEntry) handleHealthCheckSuccess() {
	wasUnhealthy := !de.healthy.Load()
	de.healthy.Store(true)

	if wasUnhealthy {
		de.logger.Info("Database entry recovered and marked healthy",
			"name", de.name,
			"priority", de.priority,
		)
	} else {
		de.logger.Debug("Health check passed",
			"name", de.name,
		)
	}
}
