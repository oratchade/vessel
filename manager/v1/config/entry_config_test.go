//go:build test

package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/manager/v1/config"
)

func TestConfigEntryValidateExactlyOneDB(t *testing.T) {
	testCases := []struct {
		name        string
		entry       *config.ConfigEntry
		expectedErr bool
		errMsg      string
	}{
		{
			name: "valid with MySQL only",
			entry: &config.ConfigEntry{
				Name:  "mysql-db",
				Type:  config.ReadWrite,
				MySQL: &db.MysqlConfig{},
			},
			expectedErr: false,
		},
		{
			name: "valid with Postgres only",
			entry: &config.ConfigEntry{
				Name:     "pg-db",
				Type:     config.ReadOnly,
				Postgres: &db.PostgresConfig{},
			},
			expectedErr: false,
		},
		{
			name: "valid with SQLite only",
			entry: &config.ConfigEntry{
				Name:   "sqlite-db",
				Type:   config.ReadOnly,
				SQLite: &db.SQLiteConfig{},
			},
			expectedErr: false,
		},
		{
			name: "valid with MSSQL only",
			entry: &config.ConfigEntry{
				Name:  "mssql-db",
				Type:  config.ReadWrite,
				MSSQL: &db.MSSQLConfig{},
			},
			expectedErr: false,
		},
		{
			name: "invalid with no DB config",
			entry: &config.ConfigEntry{
				Name: "no-db",
				Type: config.ReadOnly,
			},
			expectedErr: true,
			errMsg:      "exactly one database config",
		},
		{
			name: "invalid with MySQL and Postgres",
			entry: &config.ConfigEntry{
				Name:     "dual-db",
				Type:     config.ReadWrite,
				MySQL:    &db.MysqlConfig{},
				Postgres: &db.PostgresConfig{},
			},
			expectedErr: true,
			errMsg:      "exactly one database config",
		},
		{
			name: "invalid with all DB configs",
			entry: &config.ConfigEntry{
				Name:     "all-db",
				Type:     config.ReadWrite,
				MySQL:    &db.MysqlConfig{},
				Postgres: &db.PostgresConfig{},
				SQLite:   &db.SQLiteConfig{},
				MSSQL:    &db.MSSQLConfig{},
			},
			expectedErr: true,
			errMsg:      "exactly one database config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigEntryConfigReturnsCorrectType(t *testing.T) {
	testCases := []struct {
		name       string
		entry      *config.ConfigEntry
		expectNil  bool
		expectType string
	}{
		{
			name: "returns MySQL config",
			entry: &config.ConfigEntry{
				Name:  "mysql-db",
				MySQL: &db.MysqlConfig{Host: "localhost"},
			},
			expectNil:  false,
			expectType: "mysql",
		},
		{
			name: "returns Postgres config",
			entry: &config.ConfigEntry{
				Name:     "pg-db",
				Postgres: &db.PostgresConfig{Host: "localhost"},
			},
			expectNil:  false,
			expectType: "postgres",
		},
		{
			name: "returns SQLite config",
			entry: &config.ConfigEntry{
				Name:   "sqlite-db",
				SQLite: &db.SQLiteConfig{FilePath: "/tmp/test.db"},
			},
			expectNil:  false,
			expectType: "sqlite",
		},
		{
			name: "returns MSSQL config",
			entry: &config.ConfigEntry{
				Name:  "mssql-db",
				MSSQL: &db.MSSQLConfig{Host: "localhost"},
			},
			expectNil:  false,
			expectType: "sqlserver",
		},
		{
			name: "returns nil when no config set",
			entry: &config.ConfigEntry{
				Name: "no-db",
			},
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.entry.Config()
			if tc.expectNil {
				assert.Nil(t, cfg)
			} else {
				assert.NotNil(t, cfg)
				// Verify it implements DBConfig by checking Driver method
				assert.Equal(t, tc.expectType, cfg.Driver())
			}
		})
	}
}

func TestConfigEntryType(t *testing.T) {
	entry := &config.ConfigEntry{
		Name:  "test-db",
		Type:  config.ReadOnly,
		MySQL: &db.MysqlConfig{},
	}

	assert.Equal(t, config.ReadOnly, entry.Type)
	entry.Type = config.ReadWrite
	assert.Equal(t, config.ReadWrite, entry.Type)
}

func TestConfigEntryOverrideSettings(t *testing.T) {
	priority := 100
	interval := 10 * time.Second
	queueSize := 2000
	workers := 8

	entry := &config.ConfigEntry{
		Name:           "test-db",
		Type:           config.ReadWrite,
		MySQL:          &db.MysqlConfig{},
		Priority:       &priority,
		HealthInterval: &interval,
		ReadQueueSize:  &queueSize,
		WriteWorkers:   &workers,
	}

	assert.Equal(t, &priority, entry.Priority)
	assert.Equal(t, &interval, entry.HealthInterval)
	assert.Equal(t, &queueSize, entry.ReadQueueSize)
	assert.Equal(t, &workers, entry.WriteWorkers)
}

func TestConfigEntryNilOverrides(t *testing.T) {
	entry := &config.ConfigEntry{
		Name:     "test-db",
		Type:     config.ReadOnly,
		Postgres: &db.PostgresConfig{},
	}

	assert.Nil(t, entry.Priority)
	assert.Nil(t, entry.HealthInterval)
	assert.Nil(t, entry.ReadQueueSize)
	assert.Nil(t, entry.WriteWorkers)
}
