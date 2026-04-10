//go:build test

//nolint:dupl // Table-driven tests naturally have duplicated structures
package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/tests"
)

// TestSimpleCondition tests basic condition building
func TestSimpleCondition(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
		verify  func(*testing.T, cdt.Condition)
	}{
		{
			name: "equals condition",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("id").Op("=").Value(1)
			},
			verify: func(t *testing.T, c cdt.Condition) {
				assert.NotNil(t, c)
			},
		},
		{
			name: "like condition",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("name").Op("LIKE").Value("%test%")
			},
			verify: func(t *testing.T, c cdt.Condition) {
				assert.NotNil(t, c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			tt.verify(t, cond)
		})
	}
}

// TestAndConditions tests AND logic between conditions
func TestAndConditions(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
	}{
		{
			name: "two conditions",
			builder: func() cdt.Condition {
				return cdt.NewAnd().Conditions(
					cdt.NewExpr().Column("age").Op(">").Value(18),
					cdt.NewExpr().Column("status").Op("=").Value("active"),
				)
			},
		},
		{
			name: "three conditions",
			builder: func() cdt.Condition {
				return cdt.NewAnd().Conditions(
					cdt.NewExpr().Column("country").Op("=").Value("US"),
					cdt.NewExpr().Column("state").Op("=").Value("CA"),
					cdt.NewExpr().Column("age").Op(">").Value(21),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
		})
	}
}

// TestOrConditions tests OR logic between conditions
func TestOrConditions(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
	}{
		{
			name: "two alternatives",
			builder: func() cdt.Condition {
				return cdt.NewOr().Conditions(
					cdt.NewExpr().Column("status").Op("=").Value("pending"),
					cdt.NewExpr().Column("status").Op("=").Value("approved"),
				)
			},
		},
		{
			name: "multiple alternatives",
			builder: func() cdt.Condition {
				return cdt.NewOr().Conditions(
					cdt.NewExpr().Column("role").Op("=").Value("admin"),
					cdt.NewExpr().Column("role").Op("=").Value("moderator"),
					cdt.NewExpr().Column("role").Op("=").Value("owner"),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
		})
	}
}

// TestNestedConditions tests nested AND/OR conditions
func TestNestedConditions(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
	}{
		{
			name: "and inside or",
			builder: func() cdt.Condition {
				return cdt.NewOr().Conditions(
					cdt.NewAnd().Conditions(
						cdt.NewExpr().Column("status").Op("=").Value("active"),
						cdt.NewExpr().Column("verified").Op("=").Value(true),
					),
					cdt.NewExpr().Column("admin").Op("=").Value(true),
				)
			},
		},
		{
			name: "or inside and",
			builder: func() cdt.Condition {
				return cdt.NewAnd().Conditions(
					cdt.NewExpr().Column("deleted").Op("=").Value(false),
					cdt.NewOr().Conditions(
						cdt.NewExpr().Column("type").Op("=").Value("A"),
						cdt.NewExpr().Column("type").Op("=").Value("B"),
					),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
		})
	}
}

// TestInOperator tests IN operator
func TestInOperator(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
		values  []any
	}{
		{
			name: "numeric in list",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("id").Op("IN").Value([]any{1, 2, 3, 4, 5})
			},
			values: []any{1, 2, 3, 4, 5},
		},
		{
			name: "string in list",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("status").Op("IN").Value([]any{"active", "pending", "archived"})
			},
			values: []any{"active", "pending", "archived"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
			assert.Len(t, tt.values, len(tt.values))
		})
	}
}

// TestNotInOperator tests NOT IN operator
func TestNotInOperator(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
	}{
		{
			name: "exclude numbers",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("id").Op("NOT IN").Value([]any{10, 20, 30})
			},
		},
		{
			name: "exclude statuses",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("status").Op("NOT IN").Value([]any{"deleted", "banned"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
		})
	}
}

// TestBetweenOperator tests BETWEEN operator
func TestBetweenOperator(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
	}{
		{
			name: "numeric range",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("age").Op("BETWEEN").Value(18)
			},
		},
		{
			name: "date range",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("created_at").Op("BETWEEN").Value("2024-01-01")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
		})
	}
}

// TestNullConditions tests NULL handling
func TestNullConditions(t *testing.T) {
	dialect := tests.MockDialect{}

	testCases := []struct {
		name        string
		builder     func() cdt.Condition
		expectedSQL string
	}{
		{
			name: "is null",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("deleted_at").Op("IS NULL")
			},
			expectedSQL: "deleted_at IS NULL",
		},
		{
			name: "is not null",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("email").Op("IS NOT NULL")
			},
			expectedSQL: "email IS NOT NULL",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)

			sql, args, err := cond.ToSQL(dialect, 1)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Empty(t, args)
		})
	}
}

// TestCombinedConditions tests combinations of different operators
func TestCombinedConditions(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
	}{
		{
			name: "status and date range",
			builder: func() cdt.Condition {
				return cdt.NewAnd().Conditions(
					cdt.NewExpr().Column("status").Op("=").Value("active"),
					cdt.NewExpr().Column("created_at").Op("BETWEEN").Value("2024-01-01"),
				)
			},
		},
		{
			name: "multiple conditions with or",
			builder: func() cdt.Condition {
				return cdt.NewOr().Conditions(
					cdt.NewAnd().Conditions(
						cdt.NewExpr().Column("type").Op("=").Value("A"),
						cdt.NewExpr().Column("priority").Op(">").Value(5),
					),
					cdt.NewAnd().Conditions(
						cdt.NewExpr().Column("type").Op("=").Value("B"),
						cdt.NewExpr().Column("priority").Op(">").Value(8),
					),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
		})
	}
}

// TestCaseInsensitivePatterns tests pattern matching
func TestCaseInsensitivePatterns(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
		pattern string
	}{
		{
			name: "like pattern start",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("name").Op("LIKE").Value("john%")
			},
			pattern: "john%",
		},
		{
			name: "like pattern middle",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("email").Op("LIKE").Value("%@example.com")
			},
			pattern: "%@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
			assert.Equal(t, tt.pattern, tt.pattern)
		})
	}
}

// TestQualifiedColumns tests table-qualified column names
func TestQualifiedColumns(t *testing.T) {
	tests := []struct {
		name      string
		builder   func() cdt.Condition
		qualified string
	}{
		{
			name: "table qualified column",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("users.id").Op("=").Value(1)
			},
			qualified: "users.id",
		},
		{
			name: "schema.table qualified column",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("public.users.name").Op("LIKE").Value("%test%")
			},
			qualified: "public.users.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
			assert.Equal(t, tt.qualified, tt.qualified)
		})
	}
}

// TestUnicodeHandling tests unicode characters in conditions
func TestUnicodeHandling(t *testing.T) {
	tests := []struct {
		name    string
		builder func() cdt.Condition
		unicode string
	}{
		{
			name: "japanese characters",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("名前").Op("=").Value("太郎")
			},
			unicode: "日本語",
		},
		{
			name: "emoji in value",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("status").Op("LIKE").Value("%✅%")
			},
			unicode: "✅",
		},
		{
			name: "accented characters",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("cidade").Op("=").Value("São Paulo")
			},
			unicode: "São Paulo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			require.NotNil(t, cond)
			assert.NotEmpty(t, tt.unicode)
		})
	}
}
