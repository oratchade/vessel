//go:build test

package builder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/fabric/internal/pkg/builder"
	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

func TestPostgresMutationReturningPreview(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)
	opts := &options.QueryOptions{Returning: []string{"id", "name"}}

	insertSQL, insertArgs, err := qb.Insert("users", map[string]any{"name": "Ada"}, opts)
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "users" ("name") VALUES ($1) RETURNING "id", "name";`, insertSQL)
	assert.Equal(t, []any{"Ada"}, insertArgs)

	insertsSQL, insertsArgs, err := qb.Inserts("users", []map[string]any{{"name": "Ada"}, {"name": "Grace"}}, opts)
	require.NoError(t, err)
	assert.Equal(t, `INSERT INTO "users" ("name") VALUES ($1), ($2) RETURNING "id", "name";`, insertsSQL)
	assert.Equal(t, []any{"Ada", "Grace"}, insertsArgs)

	updateSQL, updateArgs, err := qb.Update(
		"users",
		map[string]any{"name": "Ada"},
		nil,
		cdt.NewExpr().Column("id").Op("=").Value(1),
		opts,
	)
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "users" SET "name" = $1 WHERE "id" = $2 RETURNING "id", "name";`, updateSQL)
	assert.Equal(t, []any{"Ada", 1}, updateArgs)

	deleteSQL, deleteArgs, err := qb.Delete("users", nil, cdt.NewExpr().Column("id").Op("=").Value(1), opts)
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "users" WHERE "id" = $1 RETURNING "id", "name";`, deleteSQL)
	assert.Equal(t, []any{1}, deleteArgs)
}

func TestMSSQLMutationOutputPreview(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)
	opts := &options.QueryOptions{Returning: []string{"id"}}

	insertSQL, insertArgs, err := qb.Insert("users", map[string]any{"name": "Ada"}, opts)
	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO [users] ([name]) OUTPUT inserted.[id] VALUES (@p1);", insertSQL)
	assert.Equal(t, []any{"Ada"}, insertArgs)

	updateSQL, updateArgs, err := qb.Update(
		"users",
		map[string]any{"name": "Ada"},
		nil,
		cdt.NewExpr().Column("id").Op("=").Value(1),
		opts,
	)
	require.NoError(t, err)
	assert.Equal(t, "UPDATE [users] SET [name] = @p1 OUTPUT inserted.[id] WHERE [id] = @p2;", updateSQL)
	assert.Equal(t, []any{"Ada", 1}, updateArgs)

	deleteSQL, deleteArgs, err := qb.Delete("users", nil, cdt.NewExpr().Column("id").Op("=").Value(1), opts)
	require.NoError(t, err)
	assert.Equal(t, "DELETE FROM [users] OUTPUT deleted.[id] WHERE [id] = @p1;", deleteSQL)
	assert.Equal(t, []any{1}, deleteArgs)
}

func TestMySQLMutationReturningPreviewIgnored(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)
	opts := &options.QueryOptions{Returning: []string{"id"}}

	sql, args, err := qb.Insert("users", map[string]any{"name": "Ada"}, opts)

	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO `users` (`name`) VALUES (?);", sql)
	assert.Equal(t, []any{"Ada"}, args)
}
