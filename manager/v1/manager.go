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

	db "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/manager/v1/config"
	"tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

// setupDBs initializes DBEntry maps for read-only and read-write databases
// based on the provided configuration.
func setupDBs(
	ctx context.Context,
	cfg *config.ManagerConfig,
	logger db.Logger,
) (map[string]*DBEntry, map[string]*DBEntry, error) {
	var err error
	readOnly := make(map[string]*DBEntry)
	readWrite := make(map[string]*DBEntry)

	if logger == nil {
		logger = &noOpLogger{}
	}

	logger.Info("Setting up database entries",
		"total_entries", len(cfg.Entries),
	)

	c := context.WithoutCancel(ctx)

	for _, entry := range cfg.Entries {
		switch entry.Type {
		case config.ReadOnly:
			readOnly[entry.Name], err = newDBEntry(c, cfg, &entry, logger)
			if err != nil {
				logger.Error("Failed to create read-only database entry",
					"entry_name", entry.Name,
					"error", err.Error(),
				)
				return nil, nil, fmt.Errorf("failed to create read-only DB entry: %w", err)
			}
		case config.ReadWrite:
			dbEntry, err := newDBEntry(c, cfg, &entry, logger)
			if err != nil {
				logger.Error("Failed to create read-write database entry",
					"entry_name", entry.Name,
					"error", err.Error(),
				)
				return nil, nil, fmt.Errorf("failed to create read-write DB entry: %w", err)
			}
			readWrite[entry.Name] = dbEntry
		default:
			// Unreachable when the config was validated, but never drop an
			// entry silently if setupDBs is reached with an unknown type.
			return nil, nil, fmt.Errorf("setupDBs: entry %q has unknown type %q", entry.Name, entry.Type)
		}
	}

	logger.Info("Database entries setup completed",
		"read_only_count", len(readOnly),
		"read_write_count", len(readWrite),
	)

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
	ReqUpsert     QueryRequest = "upsert"
	ReqUpserts    QueryRequest = "upserts"
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

type QueryData struct {
	Table      string
	ID         any
	Columns    []string
	Data       map[string]any
	BulkData   []map[string]any
	UpsertOpts *options.UpsertOptions
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

func newQueryResponseChannel() chan *QueryResponse {
	return make(chan *QueryResponse, 1)
}

func (dm *DBManager) enqueueWrite(
	ctx context.Context,
	operation string,
	q *Query,
) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}

	dbEntry := dm.readWriteEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-write database entries available")
	}

	q.ResponseCh = newQueryResponseChannel()
	if err := dbEntry.roundRobinQueueWrite(ctx, q); err != nil {
		return nil, fmt.Errorf("%s: failed to enqueue query: %w", operation, err)
	}

	return q.ResponseCh, nil
}

// HealthStatus represents the current health status of all database entries.
type HealthStatus struct {
	ReadOnlyHealthy  int // Number of healthy readonly entries
	ReadOnlyTotal    int // Total number of readonly entries
	ReadWriteHealthy int // Number of healthy readwrite entries
	ReadWriteTotal   int // Total number of readwrite entries
}

// DBManager manages multiple database connections.
type DBManager struct {
	HealthInterval time.Duration

	readOnlyEntries  map[string]*DBEntry
	readWriteEntries map[string]*DBEntry

	logger    db.Logger
	lifecycle lifecycle

	writeWorkerIdx *AtomicWrapCounter
	readWorkerIdx  *AtomicWrapCounter
	readEntryIdx   *AtomicWrapCounter
	writeEntryIdx  *AtomicWrapCounter
}

// NewDBManager creates a new DBManager instance by loading
// the configuration from the specified path and setting up the database entries.
//
// Environment variable expansion in config files is opt-in.
// Use WithEnvVars, WithEnvPrefix, or WithEnvFile to enable it.
// Without any EnvOption, no expansion occurs (secure by default).
func NewDBManager(ctx context.Context, configPath string, logger db.Logger, envOpts ...EnvOption) (*DBManager, error) {
	var err error

	cfg, err := loadConfig(configPath, envOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	// Use a no-op logger if none is provided
	if logger == nil {
		logger = &noOpLogger{}
	}

	readOnly, readWrite, err := setupDBs(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to setup database entries",
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to setup DB entries: %w", err)
	}

	if len(readOnly) == 0 && len(readWrite) == 0 {
		return nil, fmt.Errorf("NewDBManager: no database entries available")
	}

	writeWorkerIdxCounter, readWorkerIdxCounter,
		readEntryIdxCounter, writeEntryIdxCounter, err := getRoundRobinCounters(readOnly, readWrite, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create round robin counters: %w", err)
	}

	dm := &DBManager{
		HealthInterval:   cfg.HealthInterval,
		readOnlyEntries:  readOnly,
		readWriteEntries: readWrite,
		logger:           logger,
		writeWorkerIdx:   writeWorkerIdxCounter,
		readWorkerIdx:    readWorkerIdxCounter,
		readEntryIdx:     readEntryIdxCounter,
		writeEntryIdx:    writeEntryIdxCounter,
	}

	logger.Info("Database manager created successfully",
		"read_only_entries", len(readOnly),
		"read_write_entries", len(readWrite),
		"health_interval_ms", cfg.HealthInterval.Milliseconds(),
	)

	return dm, nil
}

func getRoundRobinCounters(
	readOnly map[string]*DBEntry,
	readWrite map[string]*DBEntry,
	logger db.Logger,
) (
	*AtomicWrapCounter,
	*AtomicWrapCounter,
	*AtomicWrapCounter,
	*AtomicWrapCounter,
	error,
) {
	// Create round-robin counters for load balancing across workers and entries.
	// Counters are only created if there are entries to distribute across.
	var writeWorkerIdxCounter *AtomicWrapCounter
	if len(readWrite) > 0 {
		c, err := NewAtomicWrapCounter(int64(len(readWrite)))
		if err != nil {
			logger.Error("Failed to create write worker index counter",
				"count", len(readWrite),
				"error", err.Error(),
			)
			return nil, nil, nil, nil, fmt.Errorf("NewDBManager: failed to create write worker counter: %w", err)
		}
		writeWorkerIdxCounter = c
	}

	var readWorkerIdxCounter *AtomicWrapCounter
	if len(readOnly) > 0 {
		c, err := NewAtomicWrapCounter(int64(len(readOnly)))
		if err != nil {
			logger.Error("Failed to create read worker index counter",
				"count", len(readOnly),
				"error", err.Error(),
			)
			return nil, nil, nil, nil, fmt.Errorf("NewDBManager: failed to create read worker counter: %w", err)
		}
		readWorkerIdxCounter = c
	}

	var readEntryIdxCounter *AtomicWrapCounter
	if len(readOnly) > 0 {
		c, err := NewAtomicWrapCounter(int64(len(readOnly)))
		if err != nil {
			logger.Error("Failed to create read entry index counter",
				"count", len(readOnly),
				"error", err.Error(),
			)
			return nil, nil, nil, nil, fmt.Errorf("NewDBManager: failed to create read entry counter: %w", err)
		}
		readEntryIdxCounter = c
	}

	var writeEntryIdxCounter *AtomicWrapCounter
	if len(readWrite) > 0 {
		c, err := NewAtomicWrapCounter(int64(len(readWrite)))
		if err != nil {
			logger.Error("Failed to create write entry index counter",
				"count", len(readWrite),
				"error", err.Error(),
			)
			return nil, nil, nil, nil, fmt.Errorf("NewDBManager: failed to create write entry counter: %w", err)
		}
		writeEntryIdxCounter = c
	}

	return writeWorkerIdxCounter, readWorkerIdxCounter, readEntryIdxCounter, writeEntryIdxCounter, nil
}

// validateConfigPath validates and resolves the config path to prevent directory traversal.
func validateConfigPath(path string) (string, error) {
	// Reject absolute paths — only relative paths within the working directory are allowed.
	// filepath.Join silently discards the base when path is absolute, defeating the traversal check below.
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("loadConfig: config path must be relative, got absolute path: %s", path)
	}

	// Resolve the path to absolute and ensure it's within the working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("loadConfig: failed to get working directory: %w", err)
	}

	// Resolve the config path relative to current directory
	absPath, err := filepath.Abs(filepath.Join(wd, path))
	if err != nil {
		return "", fmt.Errorf("loadConfig: failed to resolve config path: %w", err)
	}

	// Verify the resolved path is within the working directory (no directory traversal)
	rel, err := filepath.Rel(wd, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("loadConfig: invalid config path: path escapes working directory")
	}

	return absPath, nil
}

// loadConfig loads the configuration from the specified path.
// Security: Path must not escape the current working directory (prevents directory traversal).
func loadConfig(path string, envOpts []EnvOption) (*config.ManagerConfig, error) {
	// Validate and resolve the config path
	absPath, err := validateConfigPath(path)
	if err != nil {
		return nil, err
	}

	//nolint:gosec // G304: Path is validated above to prevent directory traversal
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("loadConfig: failed to read config file: %w", err)
	}

	// Expand ${VAR} and ${VAR:default} patterns if env options are provided
	data = expandEnvVars(data, newEnvResolver(envOpts))

	cfg := &config.ManagerConfig{}
	ext := filepath.Ext(absPath)

	switch ext {
	case ".json":
		err = json.Unmarshal(data, cfg)

	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, cfg)

	case ".toml":
		err = toml.Unmarshal(data, cfg)

	default:
		return nil, fmt.Errorf("loadConfig: unsupported file extension: %s", ext)
	}

	if err != nil {
		return nil, fmt.Errorf("loadConfig: failed to load configuration: %w", err)
	}

	return cfg, nil
}

// Start initializes the database entries and starts their worker routines.
func (dm *DBManager) Start() {
	if !dm.lifecycle.compareAndSwap(lifecycleCreated, lifecycleStarted) {
		return
	}

	if dm.logger != nil {
		dm.logger.Info("Starting database manager",
			"read_only_entries", len(dm.readOnlyEntries),
			"read_write_entries", len(dm.readWriteEntries),
		)
	}

	for name, entry := range dm.readOnlyEntries {
		if dm.logger != nil {
			dm.logger.Debug("Starting read-only entry",
				"entry_name", name,
				"priority", entry.Priority(),
			)
		}
		entry.start()
	}
	for name, entry := range dm.readWriteEntries {
		if dm.logger != nil {
			dm.logger.Debug("Starting read-write entry",
				"entry_name", name,
				"priority", entry.Priority(),
			)
		}
		entry.start()
	}

	if dm.logger != nil {
		dm.logger.Info("Database manager started successfully",
			"total_entries", len(dm.readOnlyEntries)+len(dm.readWriteEntries),
		)
	}
}

// Stop gracefully shuts down all database entries and their worker routines.
func (dm *DBManager) Stop() {
	if !dm.beginStop() {
		return
	}

	if dm.logger != nil {
		dm.logger.Info("Stopping database manager",
			"read_only_entries", len(dm.readOnlyEntries),
			"read_write_entries", len(dm.readWriteEntries),
		)
	}

	for name, entry := range dm.readOnlyEntries {
		if dm.logger != nil {
			dm.logger.Debug("Stopping read-only entry",
				"entry_name", name,
			)
		}
		entry.stop()
	}
	for name, entry := range dm.readWriteEntries {
		if dm.logger != nil {
			dm.logger.Debug("Stopping read-write entry",
				"entry_name", name,
			)
		}
		entry.stop()
	}

	if dm.logger != nil {
		dm.logger.Info("Database manager stopped successfully")
	}

	dm.lifecycle.store(lifecycleStopped)
}

func (dm *DBManager) ensureRunning() error {
	return dm.lifecycle.runningError()
}

func (dm *DBManager) beginStop() bool {
	switch dm.lifecycle.load() {
	case lifecycleStopped, lifecycleStopping:
		return false
	case lifecycleCreated:
		dm.lifecycle.store(lifecycleStopping)
		return true
	case lifecycleStarted:
		return dm.lifecycle.compareAndSwap(lifecycleStarted, lifecycleStopping)
	default:
		return false
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
//
//nolint:dupl
func (dm *DBManager) readOnlyEntry() *DBEntry {
	entries := dm.readOnlyEntries
	if len(entries) == 0 {
		if dm.logger != nil {
			dm.logger.Debug("No read-only entries available")
		}
		return nil
	}

	if len(entries) == 1 {
		for _, entry := range entries {
			if dm.logger != nil {
				dm.logger.Debug("Selected read-only entry",
					"entry_name", entry.name,
					"reason", "only_entry",
				)
			}
			return entry
		}
	}

	// First, try to select from healthy entries
	selected := dm.selectHealthyEntry(entries, dm.readEntryIdx)
	if selected != nil {
		if dm.logger != nil {
			dm.logger.Debug("Selected read-only entry from healthy entries",
				"entry_name", selected.name,
				"priority", selected.Priority(),
				"healthy", true,
			)
		}
		return selected
	}

	// Fallback: select from all entries if no healthy ones available
	fallback := selectByPriorityAndRoundRobin(entries, dm.readEntryIdx)
	if dm.logger != nil {
		dm.logger.Warn("No healthy read-only entries available, selecting from unhealthy entries",
			"selected_entry", func() string {
				if fallback == nil {
					return "none"
				}
				return fallback.name
			}(),
			"reason", "all_unhealthy",
		)
	}
	return fallback
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
//
//nolint:dupl
func (dm *DBManager) readWriteEntry() *DBEntry {
	entries := dm.readWriteEntries
	if len(entries) == 0 {
		if dm.logger != nil {
			dm.logger.Debug("No read-write entries available")
		}
		return nil
	}

	if len(entries) == 1 {
		for _, entry := range entries {
			if dm.logger != nil {
				dm.logger.Debug("Selected read-write entry",
					"entry_name", entry.name,
					"reason", "only_entry",
				)
			}
			return entry
		}
	}

	// First, try to select from healthy entries
	selected := dm.selectHealthyEntry(entries, dm.writeEntryIdx)
	if selected != nil {
		if dm.logger != nil {
			dm.logger.Debug("Selected read-write entry from healthy entries",
				"entry_name", selected.name,
				"priority", selected.Priority(),
				"healthy", true,
			)
		}
		return selected
	}

	// Fallback: select from all entries if no healthy ones available
	fallback := selectByPriorityAndRoundRobin(entries, dm.writeEntryIdx)
	if dm.logger != nil {
		dm.logger.Warn("No healthy read-write entries available, selecting from unhealthy entries",
			"selected_entry", func() string {
				if fallback == nil {
					return "none"
				}
				return fallback.name
			}(),
			"reason", "all_unhealthy",
		)
	}
	return fallback
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
// counter is used for round-robin distribution among equal-priority entries; callers must
// pass the correct counter for their entry type (readEntryIdx for reads, writeEntryIdx for writes).
func (dm *DBManager) selectHealthyEntry(entries map[string]*DBEntry, counter *AtomicWrapCounter) *DBEntry {
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
	// Uses the caller-supplied counter so reads and writes each track their own index.
	idx := counter.Next() % int64(len(priorityEntries))
	return priorityEntries[idx]
}

// selectByPriorityAndRoundRobin selects an entry from the provided map using priority and round-robin,
// without considering health status. Used as a fallback when no healthy entries are available.
func selectByPriorityAndRoundRobin(entries map[string]*DBEntry, counter *AtomicWrapCounter) *DBEntry {
	if len(entries) == 0 {
		return nil
	}

	if len(entries) == 1 {
		for _, entry := range entries {
			return entry
		}
	}

	entriesList, maxPriority := getEntriesListAndMaxPriority(entries)

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

	// Multiple entries with same max priority: use round-robin.
	// selectByPriorityAndRoundRobin requires counter to be non-nil when called.
	if counter == nil {
		return priorityEntries[0]
	}
	idx := counter.Next() % int64(len(priorityEntries))
	return priorityEntries[idx]
}

// getEntriesListAndMaxPriority converts the entries map to a slice and finds the maximum priority among them.
func getEntriesListAndMaxPriority(entries map[string]*DBEntry) ([]*DBEntry, int) {
	// Convert map to slice to work with entries
	entriesList := make([]*DBEntry, len(entries))
	i := 0
	for _, entry := range entries {
		entriesList[i] = entry
		i++
	}

	// Find the maximum priority among all entries
	var maxPriority int
	for _, entry := range entriesList {
		if entry.Priority() > maxPriority {
			maxPriority = entry.Priority()
		}
	}

	return entriesList, maxPriority
}

// GetAsync fetches data from the database asynchronously based on the specified table, columns, and conditions.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Get() instead.
func (dm *DBManager) GetAsync(
	ctx context.Context,
	table string,
	columns []string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("GetAsync: %w", err)
	}

	dbEntry := dm.readEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-only database entries available")
	}

	q := &Query{
		Request: ReqGet,
		Data: &QueryData{
			Table:      table,
			Columns:    columns,
			Joins:      joins,
			Conditions: cond,
			Opts:       opts,
		},
		ResponseCh: newQueryResponseChannel(),
	}

	if err := dbEntry.roundRobinQueueRead(ctx, q); err != nil {
		return nil, fmt.Errorf("GetAsync: failed to enqueue query: %w", err)
	}

	return q.ResponseCh, nil
}

// GetRawAsync fetches raw data from the database asynchronously based on the specified table, columns, and conditions.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use GetRaw() instead.
func (dm *DBManager) GetRawAsync(
	ctx context.Context,
	table string,
	columns []string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("GetRawAsync: %w", err)
	}

	dbEntry := dm.readEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-only database entries available")
	}

	q := &Query{
		Request: ReqGetRaw,
		Data: &QueryData{
			Table:      table,
			Columns:    columns,
			Joins:      joins,
			Conditions: cond,
			Opts:       opts,
		},
		ResponseCh: newQueryResponseChannel(),
	}

	if err := dbEntry.roundRobinQueueRead(ctx, q); err != nil {
		return nil, fmt.Errorf("GetRawAsync: failed to enqueue query: %w", err)
	}

	return q.ResponseCh, nil
}

// GetByIDAsync fetches a single record from the database asynchronously based on the specified table and ID.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use GetByID() instead.
func (dm *DBManager) GetByIDAsync(
	ctx context.Context,
	table string,
	id any,
	joins []condition.Join,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("GetByIDAsync: %w", err)
	}

	dbEntry := dm.readEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-only database entries available")
	}

	q := &Query{
		Request: ReqGetByID,
		Data: &QueryData{
			Table: table,
			ID:    id,
			Joins: joins,
			Opts:  opts,
		},
		ResponseCh: newQueryResponseChannel(),
	}

	if err := dbEntry.roundRobinQueueRead(ctx, q); err != nil {
		return nil, fmt.Errorf("GetByIDAsync: failed to enqueue query: %w", err)
	}

	return q.ResponseCh, nil
}

// GetByIDRawAsync fetches a single record from the database asynchronously based on the specified table and ID.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use GetByIDRaw() instead.
func (dm *DBManager) GetByIDRawAsync(
	ctx context.Context,
	table string,
	id any,
	joins []condition.Join,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("GetByIDRawAsync: %w", err)
	}

	dbEntry := dm.readEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-only database entries available")
	}

	q := &Query{
		Request: ReqGetByIDRaw,
		Data: &QueryData{
			Table: table,
			ID:    id,
			Joins: joins,
			Opts:  opts,
		},
		ResponseCh: newQueryResponseChannel(),
	}

	if err := dbEntry.roundRobinQueueRead(ctx, q); err != nil {
		return nil, fmt.Errorf("GetByIDRawAsync: failed to enqueue query: %w", err)
	}

	return q.ResponseCh, nil
}

// InsertAsync adds a new record to the specified table in the database asynchronously.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Insert() instead.
func (dm *DBManager) InsertAsync(
	ctx context.Context,
	table string,
	data map[string]any,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqInsert,
		Data: &QueryData{
			Table: table,
			Data:  data,
			Opts:  opts,
		},
	}
	return dm.enqueueWrite(ctx, "InsertAsync", q)
}

// InsertsAsync adds multiple new records to the specified table in the database asynchronously.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Inserts() instead.
func (dm *DBManager) InsertsAsync(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqInserts,
		Data: &QueryData{
			Table:    table,
			BulkData: data,
			Opts:     opts,
		},
	}
	return dm.enqueueWrite(ctx, "InsertsAsync", q)
}

// UpsertAsync inserts a record or applies conflict behavior asynchronously.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Upsert() instead.
func (dm *DBManager) UpsertAsync(
	ctx context.Context,
	table string,
	data map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqUpsert,
		Data: &QueryData{
			Table:      table,
			Data:       data,
			UpsertOpts: upsertOpts,
			Opts:       opts,
		},
	}
	return dm.enqueueWrite(ctx, "UpsertAsync", q)
}

// UpsertsAsync inserts multiple records or applies conflict behavior asynchronously.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Upserts() instead.
func (dm *DBManager) UpsertsAsync(
	ctx context.Context,
	table string,
	data []map[string]any,
	upsertOpts *options.UpsertOptions,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqUpserts,
		Data: &QueryData{
			Table:      table,
			BulkData:   data,
			UpsertOpts: upsertOpts,
			Opts:       opts,
		},
	}
	return dm.enqueueWrite(ctx, "UpsertsAsync", q)
}

// UpdateAsync updates an existing record in the database asynchronously.
// Updates are based on the specified table, data, and conditions.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Update() instead.
func (dm *DBManager) UpdateAsync(
	ctx context.Context,
	table string,
	data map[string]any,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqUpdate,
		Data: &QueryData{
			Table:      table,
			Data:       data,
			Joins:      joins,
			Conditions: cond,
			Opts:       opts,
		},
	}
	return dm.enqueueWrite(ctx, "UpdateAsync", q)
}

// DeleteAsync removes records from the database asynchronously based on the specified table and conditions.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Delete() instead.
func (dm *DBManager) DeleteAsync(
	ctx context.Context,
	table string,
	joins []condition.Join,
	cond condition.Condition,
	opts *options.QueryOptions,
) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqDelete,
		Data: &QueryData{
			Table:      table,
			Joins:      joins,
			Conditions: cond,
			Opts:       opts,
		},
	}
	return dm.enqueueWrite(ctx, "DeleteAsync", q)
}

// QueryAsync executes a raw query against the database asynchronously and returns the results.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Query() instead.
func (dm *DBManager) QueryAsync(ctx context.Context, query string, args ...any) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("QueryAsync: %w", err)
	}

	dbEntry := dm.readEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-only database entries available")
	}

	q := &Query{
		Request: ReqQuery,
		Data: &QueryData{
			Query:  query,
			Params: args,
		},
		ResponseCh: newQueryResponseChannel(),
	}

	if err := dbEntry.roundRobinQueueRead(ctx, q); err != nil {
		return nil, fmt.Errorf("QueryAsync: failed to enqueue query: %w", err)
	}

	return q.ResponseCh, nil
}

// QueryRawAsync executes a raw query against the database asynchronously and returns the results.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use QueryRaw() instead.
func (dm *DBManager) QueryRawAsync(ctx context.Context, query string, args ...any) (<-chan *QueryResponse, error) {
	if err := dm.ensureRunning(); err != nil {
		return nil, fmt.Errorf("QueryRawAsync: %w", err)
	}

	dbEntry := dm.readEntry()
	if dbEntry == nil {
		return nil, fmt.Errorf("no read-only database entries available")
	}

	q := &Query{
		Request: ReqQueryRaw,
		Data: &QueryData{
			Query:  query,
			Params: args,
		},
		ResponseCh: newQueryResponseChannel(),
	}

	if err := dbEntry.roundRobinQueueRead(ctx, q); err != nil {
		return nil, fmt.Errorf("QueryRawAsync: failed to enqueue query: %w", err)
	}

	return q.ResponseCh, nil
}

func (dm *DBManager) readEntry() *DBEntry {
	dbEntry := dm.readOnlyEntry()
	if dbEntry == nil {
		dbEntry = dm.readWriteEntry()
	}
	return dbEntry
}

// ExecAsync executes a raw query against the database asynchronously and returns the execution result.
// This method returns a channel that will receive the result and an error.
// Returns an error immediately if no entries are available or if context is already canceled.
// For synchronous access, use Exec() instead.
func (dm *DBManager) ExecAsync(ctx context.Context, query string, args ...any) (<-chan *QueryResponse, error) {
	q := &Query{
		Request: ReqExec,
		Data: &QueryData{
			Query:  query,
			Params: args,
		},
	}
	return dm.enqueueWrite(ctx, "ExecAsync", q)
}

func (dm *DBManager) HealthStatus() HealthStatus {
	status := HealthStatus{
		ReadOnlyTotal:  len(dm.readOnlyEntries),
		ReadWriteTotal: len(dm.readWriteEntries),
	}

	for _, entry := range dm.readOnlyEntries {
		if entry.Health() {
			status.ReadOnlyHealthy++
		}
	}

	for _, entry := range dm.readWriteEntries {
		if entry.Health() {
			status.ReadWriteHealthy++
		}
	}

	return status
}

// Ping checks the health status of all database entries and returns detailed health information.
// It checks every entry in both readonly and readwrite maps and returns a HealthStatus
// containing counts of healthy vs. total entries.
//
// Returns an error if an entire entry category is unhealthy (e.g., all readonly entries down),
// allowing for graceful degradation when isolated entries fail.
func (dm *DBManager) Ping(ctx context.Context) (HealthStatus, error) {
	status := dm.HealthStatus()

	// Fail if readonly entries are configured but all are unhealthy
	if status.ReadOnlyTotal > 0 && status.ReadOnlyHealthy == 0 {
		return status, fmt.Errorf(
			"all readonly entries unhealthy (%d/%d healthy)",
			status.ReadOnlyHealthy,
			status.ReadOnlyTotal,
		)
	}

	// Fail if readwrite entries are configured but all are unhealthy
	if status.ReadWriteTotal > 0 && status.ReadWriteHealthy == 0 {
		return status, fmt.Errorf(
			"all readwrite entries unhealthy (%d/%d healthy)",
			status.ReadWriteHealthy,
			status.ReadWriteTotal,
		)
	}

	return status, nil
}
