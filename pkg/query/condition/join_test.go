//go:build test

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/tests"
)

func TestJoin_ToSQL_OnClause(t *testing.T) {
	dialect := tests.MockDialect{}
	join := condition.Join{
		Type:  "INNER",
		Table: "orders",
		Alias: "",
		Conditions: condition.JoinCdts{
			{
				Left:  "user_id",
				Right: "id",
			},
		},
	}
	expected := "INNER JOIN `orders` ON `orders`.`user_id` = `orders`.`id`"
	got := join.ToSQL("orders", dialect)
	assert.Equal(t, expected, got)
}

func TestJoin_ToSQL_OnClause_DifferentTables(t *testing.T) {
	dialect := tests.MockDialect{}
	join := condition.Join{
		Type:  "LEFT",
		Table: "orders",
		Alias: "",
		Conditions: condition.JoinCdts{
			{
				Left:  "id",
				Right: "user_id",
			},
		},
	}
	expected := "LEFT JOIN `orders` ON `orders`.`id` = `orders`.`user_id`"
	got := join.ToSQL("orders", dialect)
	assert.Equal(t, expected, got)
}

func TestJoin_ToSQL_WithAlias(t *testing.T) {
	dialect := tests.MockDialect{}
	join := condition.Join{
		Type:  "RIGHT",
		Table: "payments",
		Alias: "p",
		Conditions: condition.JoinCdts{
			{
				Left:  "user_id",
				Right: "payer_id",
			},
		},
	}
	expected := "RIGHT JOIN `payments` AS `p` ON `payments`.`user_id` = `payments`.`payer_id`"
	got := join.ToSQL("payments", dialect)
	assert.Equal(t, expected, got)
}

func TestJoin_ToSQL_UsingClause(t *testing.T) {
	dialect := tests.MockDialect{}
	join := condition.Join{
		Type:  "INNER",
		Table: "orders",
		Alias: "",
		Conditions: condition.JoinCdts{
			{
				Left:  "id",
				Right: "id",
			},
		},
	}
	expected := "INNER JOIN `orders` USING (`id`)"
	got := join.ToSQL("orders", dialect)
	assert.Equal(t, expected, got)
}

func TestJoin_ToSQL_Complex(t *testing.T) {
	dialect := tests.MockDialect{}
	join := condition.Join{
		Type:  "FULL OUTER",
		Table: "users",
		Alias: "u",
		Conditions: condition.JoinCdts{
			{
				Left:  "id",
				Right: "user_id",
			},
		},
	}
	expected := "FULL OUTER JOIN `users` AS `u` ON `users`.`id` = `users`.`user_id`"
	got := join.ToSQL("users", dialect)
	assert.Equal(t, expected, got)
}

func TestJoin_ToSQL_MultipleJoins(t *testing.T) {
	dialect := tests.MockDialect{}
	join1 := condition.Join{
		Type:  "INNER",
		Table: "orders",
		Alias: "o",
		Conditions: condition.JoinCdts{
			{
				Left:  "id",
				Right: "user_id",
			},
		},
	}
	join2 := condition.Join{
		Type:  "LEFT",
		Table: "payments",
		Alias: "p",
		Conditions: condition.JoinCdts{
			{
				Left:  "id",
				Right: "order_id",
			},
		},
	}
	expected := "INNER JOIN `orders` AS `o` ON `users`.`id` = `orders`.`user_id`" +
		" LEFT JOIN `payments` AS `p` ON `orders`.`id` = `payments`.`order_id`"
	got := join1.ToSQL("users", dialect) + " " + join2.ToSQL("orders", dialect)
	assert.Equal(t, expected, got)
}

// func TestJoin_ToSQL_WithCondition(t *testing.T) {
// 	dialect := tests.MockDialect{}
// 	join := condition.Join{
// 		Type:  "INNER",
// 		Table: "orders",
// 		Alias: "",
// 		Left:  "user_id",
// 		Right: "id",
// 	}
// 	cond := condition.New("user_id = ?", 1)
// 	expected := "INNER JOIN `orders` ON `orders`.`id` = `orders`.`id` WHERE user_id = 1"
// 	got := join.ToSQL("orders", dialect) + " " + cond.ToSQL()
// 	require.Equal(t, expected, got)
// }
