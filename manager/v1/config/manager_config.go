package config

import "time"

const (
	DefaultWriteQueueSize  = 1000
	DefaultReadQueueSize   = 1000
	DefaultWriteWorkers    = 4
	DefaultReadWorkers     = 4
	DefaultHealthInterval  = 30 * time.Second
	DefaultWriteBatchRows  = 100
	DefaultWriteBatchDelay = 5 * time.Millisecond
)

// ManagerConfig holds global configuration for the database manager.
// Per-entry configuration overrides these defaults when specified.
//
//nolint:revive,tagalign
type ManagerConfig struct {
	HealthInterval       time.Duration `json:"health_interval,omitempty" yaml:"health_interval,omitempty" toml:"health_interval,omitempty"`
	WriteQueueSize       int           `json:"write_queue_size,omitempty" yaml:"write_queue_size,omitempty" toml:"write_queue_size,omitempty"`
	ReadQueueSize        int           `json:"read_queue_size,omitempty" yaml:"read_queue_size,omitempty" toml:"read_queue_size,omitempty"`
	WriteWorkers         int           `json:"write_workers,omitempty" yaml:"write_workers,omitempty" toml:"write_workers,omitempty"`
	ReadWorkers          int           `json:"read_workers,omitempty" yaml:"read_workers,omitempty" toml:"read_workers,omitempty"`
	WriteBatchingEnabled bool          `json:"write_batching_enabled,omitempty" yaml:"write_batching_enabled,omitempty" toml:"write_batching_enabled,omitempty"` //nolint:lll
	WriteBatchMaxRows    int           `json:"write_batch_max_rows,omitempty" yaml:"write_batch_max_rows,omitempty" toml:"write_batch_max_rows,omitempty"`       //nolint:lll
	WriteBatchMaxDelay   time.Duration `json:"write_batch_max_delay,omitempty" yaml:"write_batch_max_delay,omitempty" toml:"write_batch_max_delay,omitempty"`    //nolint:lll

	Entries []ConfigEntry `json:"entries" yaml:"entries" toml:"entries"`
}

func (mc *ManagerConfig) GetByName(name string) *ConfigEntry {
	for i := range mc.Entries {
		if mc.Entries[i].Name == name {
			return &mc.Entries[i]
		}
	}
	return nil
}

// Validate ensures all configuration entries are valid.
func (mc *ManagerConfig) Validate() error {
	for i := range mc.Entries {
		if err := mc.Entries[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// EntryWriteQueueSize returns the effective write queue size for an entry with fallback to global default.
func (mc *ManagerConfig) EntryWriteQueueSize(ce *ConfigEntry) int {
	if ce.WriteQueueSize != nil {
		return *ce.WriteQueueSize
	}
	if mc.WriteQueueSize > 0 {
		return mc.WriteQueueSize
	}
	return DefaultWriteQueueSize // Default fallback
}

// EntryReadQueueSize returns the effective read queue size for an entry with fallback to global default.
func (mc *ManagerConfig) EntryReadQueueSize(ce *ConfigEntry) int {
	if ce.ReadQueueSize != nil {
		return *ce.ReadQueueSize
	}
	if mc.ReadQueueSize > 0 {
		return mc.ReadQueueSize
	}
	return DefaultReadQueueSize // Default fallback
}

// EntryWriteWorkers returns the effective write workers count for an entry with fallback to global default.
func (mc *ManagerConfig) EntryWriteWorkers(ce *ConfigEntry) int {
	if ce.WriteWorkers != nil {
		return *ce.WriteWorkers
	}
	if mc.WriteWorkers > 0 {
		return mc.WriteWorkers
	}
	return DefaultWriteWorkers // Default fallback
}

// EntryReadWorkers returns the effective read workers count for an entry with fallback to global default.
func (mc *ManagerConfig) EntryReadWorkers(ce *ConfigEntry) int {
	if ce.ReadWorkers != nil {
		return *ce.ReadWorkers
	}
	if mc.ReadWorkers > 0 {
		return mc.ReadWorkers
	}
	return DefaultReadWorkers // Default fallback
}

// EntryHealthInterval returns the effective health check interval for an entry with fallback to global default.
func (mc *ManagerConfig) EntryHealthInterval(ce *ConfigEntry) time.Duration {
	if ce.HealthInterval != nil {
		return *ce.HealthInterval
	}
	if mc.HealthInterval > 0 {
		return mc.HealthInterval
	}
	return DefaultHealthInterval // Default fallback
}

// EntryPriority returns the effective priority for an entry with fallback to default (0).
func (mc *ManagerConfig) EntryPriority(ce *ConfigEntry) int {
	if ce.Priority != nil {
		return *ce.Priority
	}
	return 0 // Default priority if not set
}

// EntryWriteBatchingEnabled returns whether automatic insert batching is enabled for an entry.
func (mc *ManagerConfig) EntryWriteBatchingEnabled(ce *ConfigEntry) bool {
	if ce.WriteBatchingEnabled != nil {
		return *ce.WriteBatchingEnabled
	}
	return mc.WriteBatchingEnabled
}

// EntryWriteBatchMaxRows returns the effective maximum number of rows per automatic insert batch.
func (mc *ManagerConfig) EntryWriteBatchMaxRows(ce *ConfigEntry) int {
	if ce.WriteBatchMaxRows != nil && *ce.WriteBatchMaxRows > 0 {
		return *ce.WriteBatchMaxRows
	}
	if mc.WriteBatchMaxRows > 0 {
		return mc.WriteBatchMaxRows
	}
	return DefaultWriteBatchRows
}

// EntryWriteBatchMaxDelay returns the effective maximum wait before flushing an automatic insert batch.
func (mc *ManagerConfig) EntryWriteBatchMaxDelay(ce *ConfigEntry) time.Duration {
	if ce.WriteBatchMaxDelay != nil && *ce.WriteBatchMaxDelay > 0 {
		return *ce.WriteBatchMaxDelay
	}
	if mc.WriteBatchMaxDelay > 0 {
		return mc.WriteBatchMaxDelay
	}
	return DefaultWriteBatchDelay
}
