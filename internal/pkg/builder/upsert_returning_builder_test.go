//go:build test

package builder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/internal/pkg/builder"
	"tounilab.com/vessel/internal/pkg/sqldialect"
	"tounilab.com/vessel/pkg/query/options"
)

func TestPostgresUpsertRendersReturningAfterConflictClause(t *testing.T) {
	b := builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{})
	returningID := &options.QueryOptions{Returning: []string{"id"}}
	row := map[string]any{"email": "a@example.com", "name": "Alice"}
	rows := []map[string]any{row, {"email": "b@example.com", "name": "Bob"}}
	doUpdate := &options.UpsertOptions{ConflictColumns: []string{"email"}, Action: options.UpsertDoUpdate}
	doNothing := &options.UpsertOptions{ConflictColumns: []string{"email"}, Action: options.UpsertDoNothing}

	tests := []struct {
		name  string
		build func() (string, []any, error)
		want  string
	}{
		{
			name:  "upsert do update",
			build: func() (string, []any, error) { return b.Upsert("users", row, doUpdate, returningID) },
			want:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name" RETURNING "id";`,
		},
		{
			name:  "upsert do nothing",
			build: func() (string, []any, error) { return b.Upsert("users", row, doNothing, returningID) },
			want:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") DO NOTHING RETURNING "id";`,
		},
		{
			name:  "bulk upsert do update",
			build: func() (string, []any, error) { return b.Upserts("users", rows, doUpdate, returningID) },
			want:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2), ($3, $4) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name" RETURNING "id";`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _, err := tt.build()

			require.NoError(t, err)
			assert.Equal(t, tt.want, query)
			assert.Equal(t, []string{"id"}, returningID.Returning, "caller options must not be mutated")
		})
	}
}

func TestUpsertReturningIgnoredWhereNotRendered(t *testing.T) {
	returningID := &options.QueryOptions{Returning: []string{"id"}}
	row := map[string]any{"email": "a@example.com", "name": "Alice"}
	doUpdate := &options.UpsertOptions{ConflictColumns: []string{"email"}, Action: options.UpsertDoUpdate}

	tests := []struct {
		name    string
		builder builder.QueryBuilder
	}{
		{name: "mysql", builder: builder.NewMySQLQueryBuilder(sqldialect.MySQLDialect{})},
		{name: "sqlite", builder: builder.NewSQLiteQueryBuilder(sqldialect.SQLiteDialect{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, wantArgs, err := tt.builder.Upsert("users", row, doUpdate, nil)
			require.NoError(t, err)

			got, gotArgs, err := tt.builder.Upsert("users", row, doUpdate, returningID)

			require.NoError(t, err)
			assert.Equal(t, want, got)
			assert.Equal(t, wantArgs, gotArgs)
		})
	}
}
