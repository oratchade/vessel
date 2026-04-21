//go:build test

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/tests"
)

func TestBetween_ToSQL_AllFieldsSet(t *testing.T) {
	dialect := tests.MockDialect{}
	between := condition.NewBetween().Column("age").From(18).To(30)
	sql, args, err := between.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "age BETWEEN ? AND ?", sql)
	assert.Equal(t, []any{18, 30}, args)
}

func TestBetween_ToSQL_MissingColumn(t *testing.T) {
	dialect := tests.MockDialect{}
	between := condition.NewBetween().From(18).To(30)
	sql, args, err := between.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestBetween_ToSQL_MissingFrom(t *testing.T) {
	dialect := tests.MockDialect{}
	between := condition.NewBetween().Column("age").To(30)
	sql, args, err := between.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestBetween_ToSQL_MissingTo(t *testing.T) {
	dialect := tests.MockDialect{}
	between := condition.NewBetween().Column("age").From(18)
	sql, args, err := between.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestBetween_ToSQL_AllMissing(t *testing.T) {
	dialect := tests.MockDialect{}
	between := condition.NewBetween()
	sql, args, err := between.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}
