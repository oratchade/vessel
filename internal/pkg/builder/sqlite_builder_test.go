//go:build test

package builder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/internal/pkg/builder"
	"tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

// TestSQLiteMySQLQueryBuilderSelect tests SQLite query building
func TestSQLiteMySQLQueryBuilderSelect(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	query, args, err := qb.Select("users", []string{"id", "name", "email"}, nil, nil, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "SELECT")
	assert.Contains(t, query, "`id`")
	assert.Contains(t, query, "`name`")
	assert.Contains(t, query, "`email`")
	assert.Contains(t, query, "FROM `users`")
	assert.Empty(t, args)
}

// TestSQLiteMySQLPlaceholders tests SQLite uses ? placeholders
func TestSQLiteMySQLPlaceholders(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewAnd().Conditions(
		cdt.NewExpr().Column("age").Op(">").Value(18),
		cdt.NewExpr().Column("status").Op("=").Value("active"),
	)

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "?")
	assert.NotContains(t, query, "$1")
	assert.NotContains(t, query, "@p1")
	assert.Len(t, args, 2)
}

// TestSQLiteMySQLInsert tests SQLite INSERT query
func TestSQLiteMySQLInsert(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]any{
		"id":    1,
		"name":  "John",
		"email": "john@example.com",
	}

	query, args, err := qb.Insert("users", data, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "INSERT INTO `users`")
	assert.Contains(t, query, "VALUES")
	assert.Len(t, args, 3)
}

// TestSQLiteMySQLUpdate tests SQLite UPDATE query
func TestSQLiteMySQLUpdate(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]any{
		"name":  "Jane",
		"email": "jane@example.com",
	}
	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Update("users", data, nil, condition, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE `users`")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 3)
}

// TestSQLiteMySQLDelete tests SQLite DELETE query
func TestSQLiteMySQLDelete(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Delete("users", nil, condition, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM `users`")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}
