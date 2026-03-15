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

// TestMysqlQueryBuilderSelect tests MySQL query building
func TestMysqlQueryBuilderSelect(t *testing.T) {
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

// TestMysqlPlaceholders tests MySQL uses ? placeholders
func TestMysqlPlaceholders(t *testing.T) {
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

// TestMysqlInsert tests MySQL INSERT query
func TestMysqlInsert(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"id":    1,
		"name":  "John",
		"email": "john@example.com",
	}

	query, args, err := qb.Insert("users", data)

	require.NoError(t, err)
	assert.Contains(t, query, "INSERT INTO `users`")
	assert.Contains(t, query, "VALUES")
	assert.Len(t, args, 3)
}

// TestMysqlUpdate tests MySQL UPDATE query
func TestMysqlUpdate(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"name":  "Jane",
		"email": "jane@example.com",
	}
	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Update("users", data, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE `users`")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 3)
}

// TestMysqlDelete tests MySQL DELETE query
func TestMysqlDelete(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Delete("users", nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM `users`")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}

// TestMysqlIdentifierQuoting tests MySQL uses backticks
func TestMysqlIdentifierQuoting(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	query, _, err := qb.Select("users", []string{"user_id", "first_name"}, nil, nil, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "`user_id`")
	assert.Contains(t, query, "`first_name`")
	assert.NotContains(t, query, "[user_id]")
	assert.NotContains(t, query, "\"user_id\"")
}

// TestMysqlSelectWithLimit tests LIMIT clause
func TestMysqlSelectWithLimit(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	limit := 10
	opts := &options.QueryOptions{
		Limit: &limit,
	}

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, opts, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT ?")
	assert.Len(t, args, 1)
	assert.Equal(t, 10, args[0])
}

// TestMysqlSelectWithOffset tests LIMIT with OFFSET
func TestMysqlSelectWithOffset(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	limit := 10
	offset := 20
	opts := &options.QueryOptions{
		Limit:  &limit,
		Offset: &offset,
	}

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, opts, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT ? OFFSET ?")
	assert.Len(t, args, 2)
	assert.Equal(t, 10, args[0])
	assert.Equal(t, 20, args[1])
}

// TestMysqlSelectWithOrderBy tests ORDER BY clause
func TestMysqlSelectWithOrderBy(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	opts := &options.QueryOptions{
		OrderBy: []options.OrderBy{
			{Column: "id", Direction: "DESC"},
			{Column: "name", Direction: "ASC"},
		},
	}

	query, _, err := qb.Select("users", []string{"id", "name"}, nil, opts, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "ORDER BY")
	assert.Contains(t, query, "DESC")
	assert.Contains(t, query, "ASC")
}

// TestMysqlComplexCondition tests AND/OR conditions
func TestMysqlComplexCondition(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewAnd().Conditions(
		cdt.NewExpr().Column("age").Op(">").Value(21),
		cdt.NewOr().Conditions(
			cdt.NewExpr().Column("status").Op("=").Value("active"),
			cdt.NewExpr().Column("status").Op("=").Value("pending"),
		),
	)

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "WHERE")
	assert.NotEmpty(t, args)
	assert.Len(t, args, 3)
}

// TestMysqlInCondition tests IN operator
func TestMysqlInCondition(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewIn().Column("status").Values("active", "pending", "inactive")

	query, args, err := qb.Select("users", []string{"id", "name"}, nil, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "IN")
	assert.Len(t, args, 3)
}

// TestMysqlUpdateWithoutJoin tests UPDATE without JOINs (backwards compatibility)
func TestMysqlUpdateWithoutJoin(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"status": "inactive",
	}
	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	query, args, err := qb.Update("users", data, nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE `users`")
	assert.Contains(t, query, "SET `status` = ?")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 2)
}

// TestMysqlUpdateWithSingleJoin tests UPDATE with single JOIN
func TestMysqlUpdateWithSingleJoin(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"status": "active",
	}
	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "orders",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("orders.status").Op("=").Value("completed")

	query, args, err := qb.Update("users", data, joins, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE `users`")
	assert.Contains(t, query, "SET")
	assert.Contains(t, query, "INNER JOIN")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 2) // 1 from SET, 1 from WHERE
}

// TestMysqlUpdateWithMultipleJoins tests UPDATE with multiple JOINs
func TestMysqlUpdateWithMultipleJoins(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]interface{}{
		"level": "premium",
	}
	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "orders",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
		{
			Type:  "LEFT",
			Table: "payments",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("orders.total").Op(">").Value(1000)

	query, args, err := qb.Update("users", data, joins, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "INNER JOIN `orders`")
	assert.Contains(t, query, "LEFT JOIN `payments`")
	assert.Len(t, args, 2)
}

// TestMysqlDeleteWithoutJoin tests DELETE without JOINs (backwards compatibility)
func TestMysqlDeleteWithoutJoin(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("status").Op("=").Value("deleted")

	query, args, err := qb.Delete("users", nil, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM `users`")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}

// TestMysqlDeleteWithSingleJoin tests DELETE with single JOIN
func TestMysqlDeleteWithSingleJoin(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "orders",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("orders.status").Op("=").Value("canceled")

	query, args, err := qb.Delete("users", joins, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM `users`")
	assert.Contains(t, query, "INNER JOIN")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}

// TestMysqlDeleteWithMultipleJoins tests DELETE with multiple JOINs
func TestMysqlDeleteWithMultipleJoins(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "orders",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
		{
			Type:  "LEFT",
			Table: "accounts",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("orders.status").Op("=").Value("refunded")

	query, args, err := qb.Delete("users", joins, condition)

	require.NoError(t, err)
	assert.Contains(t, query, "INNER JOIN `orders`")
	assert.Contains(t, query, "LEFT JOIN `accounts`")
	assert.Len(t, args, 1)
}
