//go:build test

package builder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/fabric/internal/pkg/builder"
	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
)

// TestMSSQLQueryBuilderSelect tests MSSQL query building
func TestMSSQLQueryBuilderSelect(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	query, args, err := qb.Select("users", []string{"id", "name", "email"}, nil, nil, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "SELECT")
	assert.Contains(t, query, "[id]")
	assert.Contains(t, query, "[name]")
	assert.Contains(t, query, "[email]")
	assert.Contains(t, query, "FROM [users]")
	assert.Empty(t, args)
}

// TestMSSQLPlaceholders tests MSSQL uses @p1, @p2 placeholders
func TestMSSQLPlaceholders(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	condition := cdt.NewAnd().Conditions(
		cdt.NewExpr().Column("age").Op(">").Value(18),
		cdt.NewExpr().Column("status").Op("=").Value("active"),
	)

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "@p1")
	assert.Contains(t, query, "@p2")
	assert.NotContains(t, query, "?")
	assert.NotContains(t, query, "$1")
	assert.Len(t, args, 2)
}

// TestMSSQLInsert tests MSSQL INSERT query
func TestMSSQLInsert(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"id":    1,
		"name":  "John",
		"email": "john@example.com",
	}

	query, args, err := qb.Insert("users", data)

	require.NoError(t, err)
	assert.Contains(t, query, "INSERT INTO [users]")
	assert.Contains(t, query, "VALUES")
	assert.Len(t, args, 3)
}

// TestMSSQLUpdate tests MSSQL UPDATE query
func TestMSSQLUpdate(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"name":  "Jane",
		"email": "jane@example.com",
	}
	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Update("users", data, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE [users]")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 3)
}

// TestMSSQLDelete tests MSSQL DELETE query
func TestMSSQLDelete(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Delete("users", condition)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM [users]")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}
