//go:build test

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/tests"
)

func TestConditionHelpers_ToSQL(t *testing.T) {
	dialect := tests.MockDialect{}

	tests := []struct {
		name string
		cond condition.Condition
		sql  string
		args []any
	}{
		{
			name: "is null",
			cond: condition.IsNull("deleted_at"),
			sql:  "`deleted_at` IS NULL",
		},
		{
			name: "qualified is not null",
			cond: condition.IsNotNull("tu.deleted_at"),
			sql:  "`tu`.`deleted_at` IS NOT NULL",
		},
		{
			name: "in",
			cond: condition.In("id", "a", "b"),
			sql:  "`id` IN (?, ?)",
			args: []any{"a", "b"},
		},
		{
			name: "not in",
			cond: condition.NotIn("id", 1, 2),
			sql:  "`id` NOT IN (?, ?)",
			args: []any{1, 2},
		},
		{
			name: "like",
			cond: condition.Like("name", "%alice%"),
			sql:  "`name` LIKE ?",
			args: []any{"%alice%"},
		},
		{
			name: "not like",
			cond: condition.NotLike("name", "%alice%"),
			sql:  "`name` NOT LIKE ?",
			args: []any{"%alice%"},
		},
		{
			name: "portable ilike",
			cond: condition.ILike("users.email", "%@example.com"),
			sql:  "LOWER(`users`.`email`) LIKE LOWER(?)",
			args: []any{"%@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := tt.cond.ToSQL(dialect, 1)
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestConditionHelpers_GroupedOr(t *testing.T) {
	dialect := tests.MockDialect{}
	cond := condition.NewAnd().Conditions(
		condition.NewOr().Conditions(
			condition.Like("name", "%alice%"),
			condition.Like("email", "%alice%"),
		),
		condition.IsNull("deleted_at"),
	)

	sql, args, err := cond.ToSQL(dialect, 1)
	require.NoError(t, err)
	assert.Equal(t, "((`name` LIKE ?) OR (`email` LIKE ?)) AND (`deleted_at` IS NULL)", sql)
	assert.Equal(t, []any{"%alice%", "%alice%"}, args)
}
