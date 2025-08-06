package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/tests"
)

func TestIn_ToSQL_AllValid(t *testing.T) {
	dialect := tests.MockDialect{}
	in := condition.NewIn().Column("status").Values("active", "pending", "archived")
	sql, args, err := in.ToSQL(dialect, 1)
	assert.NoError(t, err)
	assert.Equal(t, "status IN (?, ?, ?)", sql)
	assert.Equal(t, []any{"active", "pending", "archived"}, args)
}

func TestIn_ToSQL_EmptyValues(t *testing.T) {
	dialect := tests.MockDialect{}
	in := condition.NewIn().Column("status")
	sql, args, err := in.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}

func TestIn_ToSQL_MissingColumn(t *testing.T) {
	dialect := tests.MockDialect{}
	in := condition.NewIn().Values(1, 2, 3)
	sql, args, err := in.ToSQL(dialect, 1)
	assert.Error(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)
}
