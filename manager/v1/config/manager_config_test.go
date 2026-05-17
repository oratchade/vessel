//go:build test

package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/manager/v1/config"
)

func TestManagerConfigGetByName(t *testing.T) {
	mc := &config.ManagerConfig{
		Entries: []config.ConfigEntry{
			{Name: "db1", MySQL: &db.MysqlConfig{}},
			{Name: "db2", Postgres: &db.PostgresConfig{}},
			{Name: "db3", SQLite: &db.SQLiteConfig{}},
		},
	}

	testCases := []struct {
		name           string
		searchName     string
		expectedFound  bool
		expectedDBType string
	}{
		{"find db1", "db1", true, "mysql"},
		{"find db2", "db2", true, "postgres"},
		{"find db3", "db3", true, "sqlite"},
		{"not found", "db4", false, ""},
		{"empty string", "", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := mc.GetByName(tc.searchName)
			if tc.expectedFound {
				assert.NotNil(t, entry)
				assert.Equal(t, tc.searchName, entry.Name)
			} else {
				assert.Nil(t, entry)
			}
		})
	}
}

func TestManagerConfigValidate(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.ManagerConfig
		expectedErr bool
	}{
		{
			name: "all entries valid",
			config: &config.ManagerConfig{
				Entries: []config.ConfigEntry{
					{Name: "db1", MySQL: &db.MysqlConfig{}},
					{Name: "db2", Postgres: &db.PostgresConfig{}},
				},
			},
			expectedErr: false,
		},
		{
			name: "one entry invalid (no DB config)",
			config: &config.ManagerConfig{
				Entries: []config.ConfigEntry{
					{Name: "db1", MySQL: &db.MysqlConfig{}},
					{Name: "db2"}, // Invalid - no DB config
				},
			},
			expectedErr: true,
		},
		{
			name: "empty entries valid",
			config: &config.ManagerConfig{
				Entries: []config.ConfigEntry{},
			},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test 3-tier fallback: entry override → global default → hardcoded default
func TestManagerConfigEntryWriteQueueSize(t *testing.T) {
	testCases := []struct {
		name              string
		globalSize        int
		entryOverride     *int
		expectedQueueSize int
	}{
		{
			name:              "entry override takes precedence",
			globalSize:        2000,
			entryOverride:     func() *int { i := 500; return &i }(),
			expectedQueueSize: 500,
		},
		{
			name:              "global default used when no entry override",
			globalSize:        2000,
			entryOverride:     nil,
			expectedQueueSize: 2000,
		},
		{
			name:              "hardcoded default when global is zero",
			globalSize:        0,
			entryOverride:     nil,
			expectedQueueSize: config.DefaultWriteQueueSize,
		},
		{
			name:              "hardcoded default when global is negative",
			globalSize:        -1,
			entryOverride:     nil,
			expectedQueueSize: config.DefaultWriteQueueSize,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &config.ManagerConfig{
				WriteQueueSize: tc.globalSize,
			}
			entry := &config.ConfigEntry{
				WriteQueueSize: tc.entryOverride,
				MySQL:          &db.MysqlConfig{},
			}

			size := mc.EntryWriteQueueSize(entry)
			assert.Equal(t, tc.expectedQueueSize, size)
		})
	}
}

func TestManagerConfigEntryWriteBatchingEnabled(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name     string
		global   bool
		entry    *bool
		expected bool
	}{
		{name: "default disabled"},
		{name: "global enabled", global: true, expected: true},
		{name: "entry override enabled", entry: &enabled, expected: true},
		{name: "entry override disabled", global: true, entry: &disabled, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &config.ManagerConfig{WriteBatchingEnabled: tt.global}
			entry := &config.ConfigEntry{WriteBatchingEnabled: tt.entry}

			assert.Equal(t, tt.expected, mc.EntryWriteBatchingEnabled(entry))
		})
	}
}

func TestManagerConfigEntryWriteBatchMaxRows(t *testing.T) {
	entryRows := 25

	tests := []struct {
		name     string
		global   int
		entry    *int
		expected int
	}{
		{name: "default", expected: config.DefaultWriteBatchRows},
		{name: "global", global: 50, expected: 50},
		{name: "entry override", global: 50, entry: &entryRows, expected: 25},
		{name: "ignore negative entry", global: 50, entry: func() *int { v := -1; return &v }(), expected: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &config.ManagerConfig{WriteBatchMaxRows: tt.global}
			entry := &config.ConfigEntry{WriteBatchMaxRows: tt.entry}

			assert.Equal(t, tt.expected, mc.EntryWriteBatchMaxRows(entry))
		})
	}
}

func TestManagerConfigEntryWriteBatchMaxDelay(t *testing.T) {
	entryDelay := 2 * time.Millisecond

	tests := []struct {
		name     string
		global   time.Duration
		entry    *time.Duration
		expected time.Duration
	}{
		{name: "default", expected: config.DefaultWriteBatchDelay},
		{name: "global", global: 10 * time.Millisecond, expected: 10 * time.Millisecond},
		{name: "entry override", global: 10 * time.Millisecond, entry: &entryDelay, expected: entryDelay},
		{
			name:     "ignore negative entry",
			global:   10 * time.Millisecond,
			entry:    func() *time.Duration { v := -time.Millisecond; return &v }(),
			expected: 10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &config.ManagerConfig{WriteBatchMaxDelay: tt.global}
			entry := &config.ConfigEntry{WriteBatchMaxDelay: tt.entry}

			assert.Equal(t, tt.expected, mc.EntryWriteBatchMaxDelay(entry))
		})
	}
}

func TestManagerConfigEntryReadQueueSize(t *testing.T) {
	testCases := []struct {
		name          string
		globalSize    int
		entryOverride *int
		expectedSize  int
	}{
		{
			name:          "entry override takes precedence",
			globalSize:    2000,
			entryOverride: func() *int { i := 3000; return &i }(),
			expectedSize:  3000,
		},
		{
			name:          "global default used when no entry override",
			globalSize:    1500,
			entryOverride: nil,
			expectedSize:  1500,
		},
		{
			name:          "hardcoded default when global is zero",
			globalSize:    0,
			entryOverride: nil,
			expectedSize:  config.DefaultReadQueueSize,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &config.ManagerConfig{ReadQueueSize: tc.globalSize}
			entry := &config.ConfigEntry{ReadQueueSize: tc.entryOverride}

			size := mc.EntryReadQueueSize(entry)
			assert.Equal(t, tc.expectedSize, size)
		})
	}
}

func TestManagerConfigEntryWriteWorkers(t *testing.T) {
	testCases := []struct {
		name            string
		globalWorkers   int
		entryOverride   *int
		expectedWorkers int
	}{
		{
			name:            "entry override takes precedence",
			globalWorkers:   8,
			entryOverride:   func() *int { i := 16; return &i }(),
			expectedWorkers: 16,
		},
		{
			name:            "global default used when no entry override",
			globalWorkers:   6,
			entryOverride:   nil,
			expectedWorkers: 6,
		},
		{
			name:            "hardcoded default when global is zero",
			globalWorkers:   0,
			entryOverride:   nil,
			expectedWorkers: config.DefaultWriteWorkers,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &config.ManagerConfig{WriteWorkers: tc.globalWorkers}
			entry := &config.ConfigEntry{WriteWorkers: tc.entryOverride}

			workers := mc.EntryWriteWorkers(entry)
			assert.Equal(t, tc.expectedWorkers, workers)
		})
	}
}

func TestManagerConfigEntryReadWorkers(t *testing.T) {
	testCases := []struct {
		name            string
		globalWorkers   int
		entryOverride   *int
		expectedWorkers int
	}{
		{
			name:            "entry override takes precedence",
			globalWorkers:   4,
			entryOverride:   func() *int { i := 12; return &i }(),
			expectedWorkers: 12,
		},
		{
			name:            "global default used when no entry override",
			globalWorkers:   8,
			entryOverride:   nil,
			expectedWorkers: 8,
		},
		{
			name:            "hardcoded default when global is zero",
			globalWorkers:   0,
			entryOverride:   nil,
			expectedWorkers: config.DefaultReadWorkers,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &config.ManagerConfig{ReadWorkers: tc.globalWorkers}
			entry := &config.ConfigEntry{ReadWorkers: tc.entryOverride}

			workers := mc.EntryReadWorkers(entry)
			assert.Equal(t, tc.expectedWorkers, workers)
		})
	}
}

func TestManagerConfigEntryHealthInterval(t *testing.T) {
	testCases := []struct {
		name             string
		globalInterval   time.Duration
		entryOverride    *time.Duration
		expectedInterval time.Duration
	}{
		{
			name:             "entry override takes precedence",
			globalInterval:   30 * time.Second,
			entryOverride:    func() *time.Duration { d := 10 * time.Second; return &d }(),
			expectedInterval: 10 * time.Second,
		},
		{
			name:             "global default used when no entry override",
			globalInterval:   60 * time.Second,
			entryOverride:    nil,
			expectedInterval: 60 * time.Second,
		},
		{
			name:             "hardcoded default when global is zero",
			globalInterval:   0,
			entryOverride:    nil,
			expectedInterval: config.DefaultHealthInterval,
		},
		{
			name:             "hardcoded default when global is negative",
			globalInterval:   -1 * time.Second,
			entryOverride:    nil,
			expectedInterval: config.DefaultHealthInterval,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &config.ManagerConfig{HealthInterval: tc.globalInterval}
			entry := &config.ConfigEntry{HealthInterval: tc.entryOverride}

			interval := mc.EntryHealthInterval(entry)
			assert.Equal(t, tc.expectedInterval, interval)
		})
	}
}

// Test all resolvers work together
func TestManagerConfigMultipleFallbacks(t *testing.T) {
	// Setup: global config with some values, entry with partial overrides
	globalWorkers := 2
	globalQueueSize := 500
	entryWorkers := 8 // Override

	mc := &config.ManagerConfig{
		WriteWorkers:   globalWorkers,
		WriteQueueSize: globalQueueSize,
		HealthInterval: 45 * time.Second,
	}

	entry := &config.ConfigEntry{
		WriteWorkers: &entryWorkers,
		// ReadQueueSize: nil (uses global)
		// HealthInterval: nil (uses global)
	}

	// Entry override should be used for WriteWorkers
	assert.Equal(t, 8, mc.EntryWriteWorkers(entry))

	// Global should be used for WriteQueueSize (no entry override)
	assert.Equal(t, 500, mc.EntryWriteQueueSize(entry))

	// Health interval should use global
	assert.Equal(t, 45*time.Second, mc.EntryHealthInterval(entry))

	// Reads should use hardcoded defaults (not set in global or entry)
	assert.Equal(t, config.DefaultReadWorkers, mc.EntryReadWorkers(entry))
	assert.Equal(t, config.DefaultReadQueueSize, mc.EntryReadQueueSize(entry))
}
