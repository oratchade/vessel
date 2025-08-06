package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/tests"
)

func TestExpr_ToSQL_AllFieldsSet(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr().Column("age").Op(">").Value(18)
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "age > ?", sql)
	assert.Equal(t, []any{18}, args)
}

func TestExpr_ToSQL_MissingColumn(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr().Op("=").Value(42)
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestExpr_ToSQL_MissingOperator(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr().Column("score").Value(100)
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestExpr_ToSQL_MissingValue(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr().Column("name").Op("=")
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestExpr_ToSQL_AllMissing(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr()
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestExpr_ToSQL_NilValue(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr().Column("active").Op("=").Value(nil)
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestExpr_ToSQL_DifferentOperators(t *testing.T) {
	dialect := tests.MockDialect{}
	expr := condition.NewExpr().Column("salary").Op(">=").Value(5000)
	sql, args, err := expr.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "salary >= ?", sql)
	assert.Equal(t, []any{5000}, args)

	expr2 := condition.NewExpr().Column("status").Op("!=").Value("inactive")
	sql2, args2, err2 := expr2.ToSQL(dialect, 1)
	assert.NoError(t, err2)
	assert.Equal(t, "status != ?", sql2)
	assert.Equal(t, []any{"inactive"}, args2)
}
