//go:build test

package v1_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

func newUpsertWhereSQLite(t *testing.T, schema ...string) v1.DB {
	t.Helper()
	database, err := v1.NewDB(&v1.SQLiteConfig{
		FilePath:     ":memory:",
		Mode:         "memory",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}, NoOpLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	for _, stmt := range schema {
		_, err := database.Exec(context.Background(), stmt)
		require.NoError(t, err)
	}
	return database
}

func TestFluentUpsertTargetWherePartialIndexSQLite(t *testing.T) {
	ctx := context.Background()
	database := newUpsertWhereSQLite(t,
		`CREATE TABLE relationships (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			label TEXT,
			deleted_at TEXT
		)`,
		`CREATE UNIQUE INDEX relationships_live ON relationships (source_id, target_id) WHERE deleted_at IS NULL`,
		`INSERT INTO relationships (source_id, target_id, label, deleted_at) VALUES
			('s1', 't1', 'deleted', '2026-01-01'),
			('s1', 't1', 'live', NULL),
			('s2', 't2', 'deleted', '2026-01-01')`,
	)
	upsert := func(source, target, label string) *v1.InsertBuilder {
		return v1.NewFluentDB(database).Insert().Into("relationships").
			Set("source_id", source).
			Set("target_id", target).
			Set("label", label).
			OnConflict("source_id", "target_id").
			TargetWhere(cdt.IsNull("deleted_at")).
			DoUpdate("label")
	}

	_, err := upsert("s1", "t1", "updated").Upsert(ctx)
	require.NoError(t, err)
	_, err = upsert("s2", "t2", "revived").Exec(ctx)
	require.NoError(t, err)

	rows, err := database.Query(ctx,
		"SELECT source_id, label FROM relationships WHERE deleted_at IS NULL ORDER BY source_id")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "updated", rows[0]["label"])
	assert.Equal(t, "revived", rows[1]["label"])

	deleted, err := database.Query(ctx, "SELECT label FROM relationships WHERE deleted_at IS NOT NULL")
	require.NoError(t, err)
	require.Len(t, deleted, 2)
	for _, row := range deleted {
		assert.Equal(t, "deleted", row["label"])
	}
}

func TestFluentUpsertUpdateWhereSQLite(t *testing.T) {
	ctx := context.Background()
	database := newUpsertWhereSQLite(t,
		`CREATE TABLE claims (scope_key TEXT PRIMARY KEY, owner TEXT NOT NULL, expires_at INTEGER NOT NULL)`,
		`INSERT INTO claims (scope_key, owner, expires_at) VALUES ('k1', 'first', 100)`,
	)
	claim := func(cutoff int) (*v1.ExecResult, error) {
		return v1.NewFluentDB(database).Insert().Into("claims").
			Set("scope_key", "k1").
			Set("owner", "second").
			Set("expires_at", 200).
			OnConflict("scope_key").
			DoUpdateSet(map[string]any{"owner": "second", "expires_at": 200}).
			UpdateWhere(cdt.NewExpr().Column("claims.expires_at").Op("<=").Value(cutoff)).
			Exec(ctx)
	}

	result, err := claim(50)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.RowsAffected)
	rows, err := database.Query(ctx, "SELECT owner, expires_at FROM claims WHERE scope_key = ?", "k1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "first", rows[0]["owner"])

	result, err = claim(150)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected)
	rows, err = database.Query(ctx, "SELECT owner, expires_at FROM claims WHERE scope_key = ?", "k1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "second", rows[0]["owner"])
}

func TestFluentUpsertPredicatesCallOrderSQLite(t *testing.T) {
	database := newUpsertWhereSQLite(t)
	expected := "INSERT INTO `claims` (`owner`, `scope_key`) VALUES (?, ?) ON CONFLICT (`scope_key`) WHERE `deleted_at` IS NULL DO UPDATE SET `owner` = excluded.`owner` WHERE `claims`.`expires_at` <= ?;"
	expiry := cdt.NewExpr().Column("claims.expires_at").Op("<=").Value(10)

	predicatesFirst, args, err := v1.NewFluentDB(database).Insert().Into("claims").
		UpdateWhere(expiry).
		TargetWhere(cdt.IsNull("deleted_at")).
		Set("scope_key", "k1").
		Set("owner", "o").
		DoUpdate("owner").
		OnConflict("scope_key").
		Query()
	require.NoError(t, err)
	assert.Equal(t, expected, predicatesFirst)
	assert.Equal(t, []any{"o", "k1", 10}, args)

	_, _, err = v1.NewFluentDB(database).Insert().Into("claims").
		Set("scope_key", "k1").
		UpdateWhere(expiry).
		Query()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert action is required")
}
