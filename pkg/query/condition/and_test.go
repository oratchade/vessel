//nolint:dupl
package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/tests"
)

func TestAnd_ToSQL_AllValid(t *testing.T) {
	dialect := tests.MockDialect{}
	and := condition.NewAnd().
		Conditions(
			condition.NewExpr().Column("age").Op(">").Value(18),
			condition.NewExpr().Column("score").Op(">=").Value(100),
		)
	sql, args, err := and.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "(age > ?) AND (score >= ?)", sql)
	assert.Equal(t, []any{18, 100}, args)
}

func TestAnd_ToSQL_Empty(t *testing.T) {
	dialect := tests.MockDialect{}
	and := condition.NewAnd()
	sql, args, err := and.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Empty(t, sql)
	assert.Empty(t, args)
}

func TestAnd_ToSQL_InvalidChild(t *testing.T) {
	dialect := tests.MockDialect{}
	and := condition.NewAnd().
		Conditions(
			condition.NewExpr().Column("age").Op(">").Value(18),
			condition.NewExpr(), // Invalid
		)
	sql, args, err := and.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}
