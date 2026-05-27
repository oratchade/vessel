//go:build test

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/tests"
)

func TestNot_ToSQL_Valid(t *testing.T) {
	dialect := tests.MockDialect{}
	not := condition.NewNot().Condition(condition.NewExpr().Column("active").Op("=").Value(false))
	sql, args, err := not.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "NOT (`active` = ?)", sql)
	assert.Equal(t, []any{false}, args)
}

func TestNot_ToSQL_InvalidChild(t *testing.T) {
	dialect := tests.MockDialect{}
	not := condition.NewNot().Condition(condition.NewExpr())
	sql, args, err := not.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}
