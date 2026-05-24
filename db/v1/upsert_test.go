//go:build test

package v1_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/fabric/db/v1"
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
