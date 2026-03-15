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

// TestPostgresQueryBuilderSelect tests PostgreSQL query building
func TestPostgresQueryBuilderSelect(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	query, args, err := qb.Select("users", []string{"id", "name", "email"}, nil, nil, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "SELECT")
	assert.Contains(t, query, "\"id\"")
	assert.Contains(t, query, "\"name\"")
	assert.Contains(t, query, "\"email\"")
	assert.Contains(t, query, "FROM \"users\"")
	assert.Empty(t, args)
}

// TestPostgresPlaceholders tests PostgreSQL uses $1, $2 placeholders
func TestPostgresPlaceholders(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	condition := cdt.NewAnd().Conditions(
		cdt.NewExpr().Column("age").Op(">").Value(18),
		cdt.NewExpr().Column("status").Op("=").Value("active"),
	)

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "$1")
	assert.Contains(t, query, "$2")
	assert.NotContains(t, query, "?")
	assert.NotContains(t, query, "@p1")
	assert.Len(t, args, 2)
}

// TestPostgresInsert tests PostgreSQL INSERT query
func TestPostgresInsert(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	data := map[string]interface{}{
		"id":    1,
		"name":  "John",
		"email": "john@example.com",
	}

	query, args, err := qb.Insert("users", data)

	require.NoError(t, err)
	assert.Contains(t, query, "INSERT INTO \"users\"")
	assert.Contains(t, query, "VALUES")
	assert.Len(t, args, 3)
}

// TestPostgresUpdate tests PostgreSQL UPDATE query
func TestPostgresUpdate(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	data := map[string]interface{}{
		"name":  "Jane",
		"email": "jane@example.com",
	}
	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Update("users", data, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE \"users\"")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 3)
}

// TestPostgresDelete tests PostgreSQL DELETE query
func TestPostgresDelete(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Delete("users", nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM \"users\"")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}

// TestPostgresSelectWithLimit tests LIMIT clause
func TestPostgresSelectWithLimit(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	limit := 10
	opts := &options.QueryOptions{
		Limit: &limit,
	}

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, opts, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT $1")
	assert.Len(t, args, 1)
	assert.Equal(t, 10, args[0])
}
