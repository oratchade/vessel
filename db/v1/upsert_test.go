//go:build test

package v1_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/options"
)

func TestFluentUpsertSQLite(t *testing.T) {
	ctx := context.Background()
	database, err := v1.NewDB(&v1.SQLiteConfig{
		FilePath:     ":memory:",
		CacheMode:    "shared",
		Mode:         "memory",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}, NoOpLogger{})
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	_, err = database.Exec(ctx, "CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	result, err := v1.NewFluentDB(database).
		Insert().
		Into("users").
		Set("id", "u1").
		Set("name", "Ada").
		OnConflict("id").
		DoUpdate("name").
		Exec(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	result, err = v1.NewFluentDB(database).
		Insert().
		Into("users").
		Set("id", "u1").
		Set("name", "Grace").
		OnConflict("id").
		DoUpdate("name").
		Upsert(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)

	rows, err := database.Query(ctx, "SELECT name FROM users WHERE id = ?", "u1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Grace", rows[0]["name"])
}

func TestUpsertsSQLite(t *testing.T) {
	ctx := context.Background()
	database, err := v1.NewDB(&v1.SQLiteConfig{
		FilePath:     ":memory:",
		CacheMode:    "shared",
		Mode:         "memory",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}, NoOpLogger{})
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	_, err = database.Exec(ctx, "CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	result, err := database.Upserts(
		ctx,
		"users",
		[]map[string]any{
			{"id": "u1", "name": "Ada"},
			{"id": "u2", "name": "Linus"},
		},
		&options.UpsertOptions{
			ConflictColumns: []string{"id"},
			Action:          options.UpsertDoUpdate,
			UpdateColumns:   []string{"name"},
		},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowsAffected)

	result, err = database.Upserts(
		ctx,
		"users",
		[]map[string]any{
			{"id": "u1", "name": "Grace"},
			{"id": "u3", "name": "Ken"},
		},
		&options.UpsertOptions{
			ConflictColumns: []string{"id"},
			Action:          options.UpsertDoUpdate,
			UpdateColumns:   []string{"name"},
		},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowsAffected)

	rows, err := database.Query(ctx, "SELECT id, name FROM users ORDER BY id")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "Grace", rows[0]["name"])
	assert.Equal(t, "Linus", rows[1]["name"])
	assert.Equal(t, "Ken", rows[2]["name"])
}

func TestFluentUpsertsSQLite(t *testing.T) {
	ctx := context.Background()
	database, err := v1.NewDB(&v1.SQLiteConfig{
		FilePath:     ":memory:",
		CacheMode:    "shared",
		Mode:         "memory",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}, NoOpLogger{})
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	_, err = database.Exec(ctx, "CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	result, err := v1.NewFluentDB(database).
		Insert().
		Into("users").
		ValuesBulk([]map[string]any{
			{"id": "u1", "name": "Ada"},
			{"id": "u2", "name": "Linus"},
		}).
		OnConflict("id").
		DoUpdate("name").
		Exec(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowsAffected)

	result, err = v1.NewFluentDB(database).
		Insert().
		Into("users").
		ValuesBulk([]map[string]any{
			{"id": "u1", "name": "Grace"},
			{"id": "u3", "name": "Ken"},
		}).
		OnConflict("id").
		DoUpdate("name").
		Upserts(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.RowsAffected)

	rows, err := database.Query(ctx, "SELECT id, name FROM users ORDER BY id")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "Grace", rows[0]["name"])
	assert.Equal(t, "Linus", rows[1]["name"])
	assert.Equal(t, "Ken", rows[2]["name"])
}

func TestFluentUpsertQuerySQLite(t *testing.T) {
	database, err := v1.NewDB(&v1.SQLiteConfig{FilePath: ":memory:", Mode: "memory"}, NoOpLogger{})
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	query, args, err := v1.NewFluentDB(database).
		Insert().
		Into("users").
		Set("id", "u1").
		Set("name", "Ada").
		OnConflict("id").
		DoNothing().
		Query()

	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO `users` (`id`, `name`) VALUES (?, ?) ON CONFLICT (`id`) DO NOTHING;", query)
	assert.Equal(t, []any{"u1", "Ada"}, args)
}

func TestFluentUpsertsQuerySQLite(t *testing.T) {
	database, err := v1.NewDB(&v1.SQLiteConfig{FilePath: ":memory:", Mode: "memory"}, NoOpLogger{})
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	query, args, err := v1.NewFluentDB(database).
		Insert().
		Into("users").
		ValuesBulk([]map[string]any{
			{"id": "u1", "name": "Ada"},
			{"id": "u2", "name": "Linus"},
		}).
		OnConflict("id").
		DoNothing().
		Query()

	require.NoError(t, err)
	assert.Equal(
		t,
		"INSERT INTO `users` (`id`, `name`) VALUES (?, ?), (?, ?) ON CONFLICT (`id`) DO NOTHING;",
		query,
	)
	assert.Equal(t, []any{"u1", "Ada", "u2", "Linus"}, args)
}
