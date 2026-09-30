//go:build test

package v1_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
	migrate "tounilab.com/vessel/migrate/v1"
)

type unknownConfig struct{}

func (unknownConfig) Driver() string { return "clickhouse" }
func (unknownConfig) DSN() string    { return "" }

func sqliteConfig(t *testing.T) db.SQLiteConfig {
	t.Helper()
	return db.SQLiteConfig{FilePath: filepath.Join(t.TempDir(), "migrate.db")}
}

// Two ordinary migrations, the second a trigger whose body holds semicolons.
func baseMigrations() fstest.MapFS {
	return fstest.MapFS{
		"00001_create_items.sql": {Data: []byte(`-- +goose Up
CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL, updated INTEGER NOT NULL DEFAULT 0);
`)},
		"00002_touch_trigger.sql": {Data: []byte(`-- +goose Up
-- +goose StatementBegin
CREATE TRIGGER items_touch AFTER UPDATE OF name ON items
BEGIN
    UPDATE items SET updated = updated + 1 WHERE id = NEW.id;
END;
-- +goose StatementEnd
`)},
	}
}

func TestNewRejectsWhatItCannotMigrate(t *testing.T) {
	_, err := migrate.New(unknownConfig{}, baseMigrations())
	assert.ErrorIs(t, err, migrate.ErrUnsupportedDriver)

	_, err = migrate.New(nil, baseMigrations())
	assert.Error(t, err)

	_, err = migrate.New(sqliteConfig(t), nil)
	assert.Error(t, err)

	_, err = migrate.New(sqliteConfig(t), fstest.MapFS{})
	assert.ErrorIs(t, err, migrate.ErrNoMigrations)

	// goose reads the applied-at timestamps back, which MySQL returns as bytes
	// without parseTime. Refused before any connection is opened.
	_, err = migrate.New(db.MysqlConfig{Host: "localhost", Port: 3306, Database: "x"}, baseMigrations())
	assert.ErrorIs(t, err, migrate.ErrMySQLParseTime)
}

func TestUpAppliesPendingMigrationsOnce(t *testing.T) {
	ctx := context.Background()
	m, err := migrate.New(sqliteConfig(t), baseMigrations())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	version, err := m.Version(ctx)
	require.NoError(t, err)
	assert.Zero(t, version)

	results, err := m.Up(ctx)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, int64(1), results[0].Version)
	assert.Equal(t, "00001_create_items.sql", results[0].Source)
	assert.Equal(t, "00002_touch_trigger.sql", results[1].Source)

	again, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Empty(t, again, "a second Up applies nothing")

	version, err = m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), version)

	statuses, err := m.Status(ctx)
	require.NoError(t, err)
	require.Len(t, statuses, 2)
	for _, s := range statuses {
		assert.Equal(t, migrate.StateApplied, s.State, s.Source)
		assert.False(t, s.AppliedAt.IsZero(), s.Source)
	}
}

func TestUpStopsAtAFailingMigrationAndNamesIt(t *testing.T) {
	ctx := context.Background()
	fsys := baseMigrations()
	fsys["00003_broken.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE broken (;\n")}
	fsys["00004_after.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE after_broken (id INTEGER);\n")}

	m, err := migrate.New(sqliteConfig(t), fsys)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	results, err := m.Up(ctx)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "00003_broken.sql"), "error names the file: %v", err)
	assert.Len(t, results, 2, "the migrations before the failure are reported as applied")

	version, err := m.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), version, "the version stays at the last migration that succeeded")

	statuses, err := m.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, migrate.StatePending, statuses[2].State)
	assert.Equal(t, migrate.StatePending, statuses[3].State)
}

func TestNoTransactionMigration(t *testing.T) {
	ctx := context.Background()
	fsys := baseMigrations()
	fsys["00003_vacuum.sql"] = &fstest.MapFile{Data: []byte("-- +goose NO TRANSACTION\n-- +goose Up\nVACUUM;\n")}

	m, err := migrate.New(sqliteConfig(t), fsys)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	// VACUUM fails inside a transaction, so this passes only if the annotation is honoured.
	results, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestWithVersionTable(t *testing.T) {
	ctx := context.Background()
	cfg := sqliteConfig(t)

	m, err := migrate.New(cfg, baseMigrations(), migrate.WithVersionTable("schema_history"))
	require.NoError(t, err)
	_, err = m.Up(ctx)
	require.NoError(t, err)
	require.NoError(t, m.Close())

	conn, err := db.NewDB(cfg, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	rows, err := conn.QueryRaw(ctx, "SELECT COUNT(*) FROM schema_history WHERE version_id > 0")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var applied int
	require.NoError(t, rows.Scan(&applied))
	assert.Equal(t, 2, applied)
}

// goose providers include Go migrations registered anywhere in the process by default.
// One registered for another database must never run against this one.
func TestUpIgnoresGloballyRegisteredGoMigrations(t *testing.T) {
	ran := false
	goose.AddNamedMigrationContext("00003_other_database.go",
		func(context.Context, *sql.Tx) error { ran = true; return nil }, nil)
	t.Cleanup(goose.ResetGlobalMigrations)

	m, err := migrate.New(sqliteConfig(t), baseMigrations())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	results, err := m.Up(context.Background())
	require.NoError(t, err)
	assert.Len(t, results, 2, "only the supplied SQL migrations run")
	assert.False(t, ran, "the globally registered Go migration must not run")

	version, err := m.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), version)
}

func TestErrorsAreDistinct(t *testing.T) {
	assert.False(t, errors.Is(migrate.ErrNoMigrations, migrate.ErrUnsupportedDriver))
}
