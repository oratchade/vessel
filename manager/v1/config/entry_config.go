package config

import (
	"fmt"
	"time"

	db "tounilab.com/vessel/db/v1"
)

type DBType = string

const (
	ReadOnly  DBType = "readonly"
	ReadWrite DBType = "readwrite"
)

// ConfigEntry holds a database configuration for a specific database engine.
// Exactly one of MySQL, Postgres, SQLite, or MSSQL must be set.
// Per-entry resource and operational settings override global defaults if specified.
//
//nolint:revive,tagalign
type ConfigEntry struct {
	Name string `json:"name" yaml:"name" toml:"name"`
	Type DBType `json:"type" yaml:"type" toml:"type"`

	MySQL    *db.MysqlConfig    `json:"mysql,omitempty" yaml:"mysql,omitempty" toml:"mysql,omitempty"`
	Postgres *db.PostgresConfig `json:"postgres,omitempty" yaml:"postgres,omitempty" toml:"postgres,omitempty"`
	SQLite   *db.SQLiteConfig   `json:"sqlite,omitempty" yaml:"sqlite,omitempty" toml:"sqlite,omitempty"`
	MSSQL    *db.MSSQLConfig    `json:"mssql,omitempty" yaml:"mssql,omitempty" toml:"mssql,omitempty"`

	WriteQueueSize *int           `json:"write_queue_size,omitempty" yaml:"write_queue_size,omitempty" toml:"write_queue_size,omitempty"`
	ReadQueueSize  *int           `json:"read_queue_size,omitempty" yaml:"read_queue_size,omitempty" toml:"read_queue_size,omitempty"`
	WriteWorkers   *int           `json:"write_workers,omitempty" yaml:"write_workers,omitempty" toml:"write_workers,omitempty"`
	ReadWorkers    *int           `json:"read_workers,omitempty" yaml:"read_workers,omitempty" toml:"read_workers,omitempty"`
	HealthInterval *time.Duration `json:"health_interval,omitempty" yaml:"health_interval,omitempty" toml:"health_interval,omitempty"`
	Priority       *int           `json:"priority,omitempty" yaml:"priority,omitempty" toml:"priority,omitempty"`

	WriteBatchingEnabled *bool          `json:"write_batching_enabled,omitempty" yaml:"write_batching_enabled,omitempty" toml:"write_batching_enabled,omitempty"` //nolint:lll
	WriteBatchMaxRows    *int           `json:"write_batch_max_rows,omitempty" yaml:"write_batch_max_rows,omitempty" toml:"write_batch_max_rows,omitempty"`       //nolint:lll
	WriteBatchMaxDelay   *time.Duration `json:"write_batch_max_delay,omitempty" yaml:"write_batch_max_delay,omitempty" toml:"write_batch_max_delay,omitempty"`    //nolint:lll
}

// Config returns the database configuration interface if one is set.
func (ce *ConfigEntry) Config() db.DBConfig {
	switch {
	case ce.MySQL != nil:
		return *ce.MySQL
	case ce.Postgres != nil:
		return *ce.Postgres
	case ce.SQLite != nil:
		return *ce.SQLite
	case ce.MSSQL != nil:
		return *ce.MSSQL
	default:
		return nil
	}
}

// Validate ensures exactly one database configuration is set.
func (ce *ConfigEntry) Validate() error {
	count := 0
	if ce.MySQL != nil {
		count++
	}
	if ce.Postgres != nil {
		count++
	}
	if ce.SQLite != nil {
		count++
	}
	if ce.MSSQL != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("ConfigEntry %q must have exactly one database config, got %d", ce.Name, count)
	}
	return nil
}
