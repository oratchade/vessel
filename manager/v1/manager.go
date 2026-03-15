// Package v1 provides database manager entrypoint and implementation for multiple database engines management.
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/manager/v1/config"
	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

// setupDBs initializes DBEntry maps for read-only and read-write databases based on the provided configuration.
func setupDBs(ctx context.Context, cfg *config.ManagerConfig) (map[string]*DBEntry, map[string]*DBEntry, error) {
	var err error
	readOnly := make(map[string]*DBEntry)
	readWrite := make(map[string]*DBEntry)

	c := context.WithoutCancel(ctx)

	for _, entry := range cfg.Entries {
		switch entry.Type {
		case config.ReadOnly:
			readOnly[entry.Name], err = newDBEntry(c, cfg, &entry)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create read-only DB entry: %w", err)
			}
		case config.ReadWrite:
			readWrite[entry.Name], err = newDBEntry(c, cfg, &entry)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create read-write DB entry: %w", err)
			}
		}
	}

	return readOnly, readWrite, nil
}

// QueryRequest represents a type of database query.
type QueryRequest = string

const (
	ReqGet        QueryRequest = "get"
	ReqGetRaw     QueryRequest = "getRaw"
	ReqGetByID    QueryRequest = "getById"
	ReqGetByIDRaw QueryRequest = "getByIdRaw"
	ReqInsert     QueryRequest = "insert"
	ReqInserts    QueryRequest = "inserts"
	ReqUpdate     QueryRequest = "update"
	ReqDelete     QueryRequest = "delete"
	ReqQuery      QueryRequest = "query"
	ReqQueryRaw   QueryRequest = "queryRaw"
	ReqExec       QueryRequest = "exec"
)

// Query represents a database query request with its parameters and a channel for the response.
type Query struct {
	Request    QueryRequest
	Data       *QueryData
	ResponseCh chan *QueryResponse
}

// DBEntry represents a single database connection entry with its configuration, worker queues, and other settings.
type QueryData struct {
	Table      string
	ID         any
	Columns    []string
	Data       map[string]any
	BulkData   []map[string]any
	Joins      []condition.Join
	Conditions condition.Condition
	Opts       *options.QueryOptions

	Query  string
	Params []any
}

// QueryResponse represents the response for a database query.
type QueryResponse struct {
	RequestID string
	Data      []map[string]any
	RawData   *db.RowsAdapter
	ExecData  *db.ExecResult
	Error     error
}

// DBManager manages multiple database connections.
type DBManager struct {
	HealthInterval time.Duration

	readOnlyEntries  map[string]*DBEntry
	readWriteEntries map[string]*DBEntry

	writeWorkerIdx AtomicWrapCounter
	readWorkerIdx  AtomicWrapCounter
}

// NewDBManager creates a new DBManager instance by loading
// the configuration from the specified path and setting up the database entries.
func NewDBManager(ctx context.Context, configPath string) (*DBManager, error) {
	var err error

	cfg, err := (&DBManager{}).loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	readOnly, readWrite, err := setupDBs(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup DB entries: %w", err)
	}

	return &DBManager{
		HealthInterval:   cfg.HealthInterval,
		readOnlyEntries:  readOnly,
		readWriteEntries: readWrite,
		writeWorkerIdx:   *NewAtomicWrapCounter(int64(len(readWrite))),
		readWorkerIdx:    *NewAtomicWrapCounter(int64(len(readOnly))),
	}, nil
}

// loadConfig loads the configuration from the specified path.
func (dm *DBManager) loadConfig(path string) (*config.ManagerConfig, error) {
	// Prevent directory traversal
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("loadConfig: invalid config path: contains directory traversal")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("loadConfig: failed to read config file: %w", err)
	}

	cfg := &config.ManagerConfig{}
	ext := filepath.Ext(cleanPath)

	switch ext {
	case ".json":
		err = json.Unmarshal(data, cfg)

	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, cfg)

	case ".toml":
		err = toml.Unmarshal(data, cfg)
	}

	return cfg, fmt.Errorf("loadConfig: failed to load configuration provided: %w", err)
}

// Start initializes the database entries and starts their worker routines.
func (dm *DBManager) Start(ctx context.Context) {
	for _, entry := range dm.readOnlyEntries {
		entry.start(ctx)
	}
	for _, entry := range dm.readWriteEntries {
		entry.start(ctx)
	}
}

// Stop gracefully shuts down all database entries and their worker routines.
func (dm *DBManager) Stop() {
	for _, entry := range dm.readOnlyEntries {
		entry.stop()
	}
	for _, entry := range dm.readWriteEntries {
		entry.stop()
	}
}

// readOnlyEntry returns a read-only DBEntry using a hybrid priority + health + round-robin selection strategy.
//
// Selection Strategy:
//  1. Health-First: Prioritizes healthy entries (determined by health check)
//  2. Priority-Based: Among healthy entries, selects the entry group with the highest priority
//  3. Load Balanced: If multiple healthy entries share the same highest priority, distributes
//     queries using round-robin within that priority tier
//  4. Fallback: If no healthy entries exist, falls back to unhealthy entries by priority
//
// Use Cases:
//   - Primary healthy: Always routes to primary database
//   - Primary unhealthy: Automatically routes to healthy replicas
//   - All unhealthy: Routes to highest priority entry (monitoring can alert)
//   - Multiple same-priority: Balances load among healthy replicas
//
// Example Configuration:
//
//	entries:
//	  - name: primary-db
//	    priority: 100        # Always preferred if healthy
//	    type: read-only
//	  - name: replica-1
//	    priority: 50         # Used if primary is unhealthy
//	    type: read-only
//	  - name: replica-2
//	    priority: 50         # Load-balanced with replica-1 if healthy
//	    type: read-only
//
// Returns nil if no read-only entries are configured.
func (dm *DBManager) readOnlyEntry() *DBEntry {
	entries := dm.readOnlyEntries
	if len(entries) == 0 {
		return nil
	}

	if len(entries) == 1 {
		for _, entry := range entries {
			return entry
		}
	}

	// First, try to select from healthy entries
	selected := dm.selectHealthyEntry(entries)
	if selected != nil {
		return selected
	}

	// Fallback: select from all entries if no healthy ones available
	return dm.selectByPriorityAndRoundRobin(entries, &dm.readWorkerIdx)
}

// readWriteEntry returns a read-write DBEntry using a hybrid priority + health + round-robin selection strategy.
//
// Selection Strategy:
//  1. Health-First: Prioritizes healthy entries (determined by health check)
//  2. Priority-Based: Among healthy entries, selects the entry group with the highest priority
//  3. Load Balanced: If multiple healthy entries share the same highest priority, distributes
//     queries using round-robin within that priority tier
//  4. Fallback: If no healthy entries exist, falls back to unhealthy entries by priority
//
// Use Cases:
//   - Primary healthy: Always routes writes to primary database
//   - Primary unhealthy: Automatically routes writes to healthy secondaries
//   - All unhealthy: Routes to highest priority entry (monitoring can alert)
//   - Multiple same-priority: Balances write load among healthy entries
//
// Example Configuration:
//
//	entries:
//	  - name: primary-writer
//	    priority: 100        # Always preferred if healthy
//	    type: read-write
//	  - name: secondary-writer
//	    priority: 50         # Used if primary is unhealthy
//	    type: read-write
//
// Returns nil if no read-write entries are configured.
func (dm *DBManager) readWriteEntry() *DBEntry {
	entries := dm.readWriteEntries
	if len(entries) == 0 {
		return nil
	}

	if len(entries) == 1 {
		for _, entry := range entries {
			return entry
		}
	}

	// First, try to select from healthy entries
	selected := dm.selectHealthyEntry(entries)
	if selected != nil {
		return selected
	}

	// Fallback: select from all entries if no healthy ones available
	return dm.selectByPriorityAndRoundRobin(entries, &dm.writeWorkerIdx)
}

// healthOnly filters the provided entries map and returns a slice of only the healthy entries.
func (dm *DBManager) healthyOnly(entries map[string]*DBEntry) []*DBEntry {
	if len(entries) == 0 {
		return nil
	}

	// Collect healthy entries only
	var healthyEntries []*DBEntry
	for _, entry := range entries {
		if entry.Health() {
			healthyEntries = append(healthyEntries, entry)
		}
	}

	return healthyEntries
}

// selectHealthyEntry selects an entry from the provided map using priority and round-robin,
// considering only healthy entries. Returns nil if no healthy entries are available.
func (dm *DBManager) selectHealthyEntry(entries map[string]*DBEntry) *DBEntry {
	// Collect healthy entries only
	healthyEntries := dm.healthyOnly(entries)

	if len(healthyEntries) == 0 {
		return nil // No healthy entries available
	}

	if len(healthyEntries) == 1 {
		return healthyEntries[0]
	}

	// Find the maximum priority among healthy entries
	var maxPriority int
	for _, entry := range healthyEntries {
		if entry.Priority() > maxPriority {
			maxPriority = entry.Priority()
		}
	}

	// Collect all healthy entries with the maximum priority
	var priorityEntries []*DBEntry
	for _, entry := range healthyEntries {
		if entry.Priority() == maxPriority {
			priorityEntries = append(priorityEntries, entry)
		}
	}

	if len(priorityEntries) == 1 {
		return priorityEntries[0]
	}

	// Multiple healthy entries with same max priority: use round-robin
	// Note: This uses a simple approach; in production, separate counters per entry type might be better
	idx := dm.readWorkerIdx.Next() % int64(len(priorityEntries))
	return priorityEntries[idx]
}

// selectByPriorityAndRoundRobin selects an entry from the provided map using priority and round-robin,
// without considering health status. Used as a fallback when no healthy entries are available.
func (dm *DBManager) selectByPriorityAndRoundRobin(entries map[string]*DBEntry, counter *AtomicWrapCounter) *DBEntry {
	if len(entries) == 0 {
		return nil
	}

	if len(entries) == 1 {
		for _, entry := range entries {
			return entry
		}
	}

	// Convert map to slice to work with entries
	var entriesList []*DBEntry
	for _, entry := range entries {
		entriesList = append(entriesList, entry)
	}

	// Find the maximum priority among all entries
	var maxPriority int
	for _, entry := range entriesList {
		if entry.Priority() > maxPriority {
			maxPriority = entry.Priority()
		}
	}

	// Collect all entries with the maximum priority
	var priorityEntries []*DBEntry
	for _, entry := range entriesList {
		if entry.Priority() == maxPriority {
			priorityEntries = append(priorityEntries, entry)
		}
	}

	if len(priorityEntries) == 1 {
		return priorityEntries[0]
	}

	// Multiple entries with same max priority: use round-robin
	idx := counter.Next() % int64(len(priorityEntries))
	return priorityEntries[idx]
}

// GetAsync fetches data from the database asynchronously based on the specified table, columns, and conditions.
// This method returns a channel that will receive the result.
// For synchronous access, use Get() instead.
func (dm *DBManager) GetAsync(
	ctx context.Context,
	table string,
	columns []string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqGet,
		Data: &QueryData{
			Table:      table,
			Columns:    columns,
			Joins:      joins,
			Conditions: cond,
			Opts:       opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-only database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueRead(ctx, q)

	return q.ResponseCh
}

// GetRawAsync fetches raw data from the database asynchronously based on the specified table, columns, and conditions.
// This method returns a channel that will receive the result.
// For synchronous access, use GetRaw() instead.
func (dm *DBManager) GetRawAsync(
	ctx context.Context,
	table string,
	columns []string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqGetRaw,
		Data: &QueryData{
			Table:      table,
			Columns:    columns,
			Joins:      joins,
			Conditions: cond,
			Opts:       opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-only database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueRead(ctx, q)

	return q.ResponseCh
}

// GetByIDAsync fetches a single record from the database asynchronously based on the specified table and ID.
// This method returns a channel that will receive the result.
// For synchronous access, use GetByID() instead.
func (dm *DBManager) GetByIDAsync(
	ctx context.Context,
	table string,
	id any,
	joins []condition.Join,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqGetByID,
		Data: &QueryData{
			Table: table,
			ID:    id,
			Joins: joins,
			Opts:  opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-only database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueRead(ctx, q)

	return q.ResponseCh
}

// GetByIDRawAsync fetches a single record from the database asynchronously based on the specified table and ID.
// This method returns a channel that will receive the result.
// For synchronous access, use GetByIDRaw() instead.
func (dm *DBManager) GetByIDRawAsync(
	ctx context.Context,
	table string,
	id any,
	joins []condition.Join,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqGetByIDRaw,
		Data: &QueryData{
			Table: table,
			ID:    id,
			Joins: joins,
			Opts:  opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-only database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueRead(ctx, q)

	return q.ResponseCh
}

// InsertAsync adds a new record to the specified table in the database asynchronously.
// This method returns a channel that will receive the result.
// For synchronous access, use Insert() instead.
func (dm *DBManager) InsertAsync(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqInsert,
		Data: &QueryData{
			Table: table,
			Data:  data,
			Opts:  opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readWriteEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-write database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueWrite(ctx, q)

	return q.ResponseCh
}

// InsertsAsync adds multiple new records to the specified table in the database asynchronously.
// This method returns a channel that will receive the result.
// For synchronous access, use Inserts() instead.
func (dm *DBManager) InsertsAsync(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqInserts,
		Data: &QueryData{
			Table:    table,
			BulkData: data,
			Opts:     opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readWriteEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-write database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueWrite(ctx, q)

	return q.ResponseCh
}

// UpdateAsync updates an existing record in the database asynchronously based on the specified table, data, and conditions.
// This method returns a channel that will receive the result.
// For synchronous access, use Update() instead.
func (dm *DBManager) UpdateAsync(
	ctx context.Context,
	table string,
	data map[string]any,
	cond condition.Condition,
	joins []condition.Join,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqUpdate,
		Data: &QueryData{
			Table:      table,
			Data:       data,
			Conditions: cond,
			Opts:       opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readWriteEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-write database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueWrite(ctx, q)

	return q.ResponseCh
}

// DeleteAsync removes records from the database asynchronously based on the specified table and conditions.
// This method returns a channel that will receive the result.
// For synchronous access, use Delete() instead.
func (dm *DBManager) DeleteAsync(
	ctx context.Context,
	table string,
	cond condition.Condition,
	joins []condition.Join,
	opts *options.QueryOptions,
) <-chan *QueryResponse {
	q := &Query{
		Request: ReqDelete,
		Data: &QueryData{
			Table:      table,
			Conditions: cond,
			Opts:       opts,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readWriteEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-write database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueWrite(ctx, q)

	return q.ResponseCh
}

// QueryAsync executes a raw query against the database asynchronously and returns the results.
// This method returns a channel that will receive the result.
// For synchronous access, use Query() instead.
func (dm *DBManager) QueryAsync(ctx context.Context, query string, args ...any) <-chan *QueryResponse {
	q := &Query{
		Request: ReqQuery,
		Data: &QueryData{
			Query:  query,
			Params: args,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-only database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueRead(ctx, q)

	return q.ResponseCh
}

// QueryRawAsync executes a raw query against the database asynchronously and returns the results.
// This method returns a channel that will receive the result.
// For synchronous access, use QueryRaw() instead.
func (dm *DBManager) QueryRawAsync(ctx context.Context, query string, args ...any) <-chan *QueryResponse {
	q := &Query{
		Request: ReqQueryRaw,
		Data: &QueryData{
			Query:  query,
			Params: args,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-only database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueRead(ctx, q)

	return q.ResponseCh
}

// ExecAsync executes a raw query against the database asynchronously and returns the execution result.
// This method returns a channel that will receive the result.
// For synchronous access, use Exec() instead.
func (dm *DBManager) ExecAsync(ctx context.Context, query string, args ...any) <-chan *QueryResponse {
	q := &Query{
		Request: ReqExec,
		Data: &QueryData{
			Query:  query,
			Params: args,
		},
		ResponseCh: make(chan *QueryResponse),
	}

	dbEntry := dm.readWriteEntry()
	if dbEntry == nil {
		go func() {
			q.ResponseCh <- &QueryResponse{Error: fmt.Errorf("no read-write database entries available")}
			close(q.ResponseCh)
		}()
		return q.ResponseCh
	}
	_ = dbEntry.roundRobinQueueWrite(ctx, q)

	return q.ResponseCh
}

// Ping checks the database connection synchronously.
func (dm *DBManager) Ping(ctx context.Context) error {
	entry := dm.readOnlyEntry()
	if entry == nil {
		return fmt.Errorf("no read-only database entries configured")
	}
	return entry.db.Ping(ctx)
}
