//go:build test

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/tests"
)

func TestOr_ToSQL_AllValid(t *testing.T) {
	dialect := tests.MockDialect{}
	or := condition.NewOr().
		Conditions(
			condition.NewExpr().Column("status").Op("=").Value("active"),
			condition.NewExpr().Column("status").Op("=").Value("pending"),
		)
	sql, args, err := or.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "(`status` = ?) OR (`status` = ?)", sql)
	assert.Equal(t, []any{"active", "pending"}, args)
}

func TestOr_ToSQL_Empty(t *testing.T) {
	dialect := tests.MockDialect{}
	or := condition.NewOr()
	sql, args, err := or.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Empty(t, sql)
	assert.Empty(t, args)
}

func TestOr_ToSQL_InvalidChild(t *testing.T) {
	dialect := tests.MockDialect{}
	or := condition.NewOr().
		Conditions(
			condition.NewExpr().Column("status").Op("=").Value("active"),
			condition.NewExpr(), // Invalid
		)
	sql, args, err := or.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}
