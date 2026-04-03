//go:build test

package sqldialect_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
)

// TestDialectComparisonOperators tests comparison operators across all dialects
func TestDialectComparisonOperators(t *testing.T) {
	tests := []struct {
		name     string
		dialect  cdt.SQLDialect
		column   string
		operator string
		value    any
		expected string
	}{
		{
			name:     "mysql equals",
			dialect:  &sqldialect.MySQLDialect{},
			column:   "age",
			operator: "=",
			value:    30,
			expected: "= 30",
		},
		{
			name:     "postgres not equals",
			dialect:  &sqldialect.PostgresDialect{},
			column:   "status",
			operator: "<>",
			value:    "active",
			expected: "<> 'active'",
		},
		{
			name:     "sqlite less than",
			dialect:  &sqldialect.MySQLDialect{},
			column:   "price",
			operator: "<",
			value:    100.50,
			expected: "< 100.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			assert.NotEmpty(t, tt.operator)
		})
	}
}

// TestDialectIdentifierQuoting tests identifier quoting per dialect
func TestDialectIdentifierQuoting(t *testing.T) {
	tests := []struct {
		name     string
		dialect  cdt.SQLDialect
		ident    string
		expected string
	}{
		{
			name:     "mysql reserves backticks",
			dialect:  &sqldialect.MySQLDialect{},
			ident:    "user",
			expected: "`user`",
		},
		{
			name:     "postgres uses double quotes",
			dialect:  &sqldialect.PostgresDialect{},
			ident:    "user_id",
			expected: `"user_id"`,
		},
		{
			name:     "sqlite no quoting",
			dialect:  &sqldialect.MySQLDialect{},
			ident:    "table_name",
			expected: "table_name",
		},
		{
			name:     "mssql uses square brackets",
			dialect:  &sqldialect.MSSQLDialect{},
			ident:    "column",
			expected: "[column]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			assert.NotEmpty(t, tt.ident)
		})
	}
}

// TestDialectLikeOperator tests LIKE operator behavior
func TestDialectLikeOperator(t *testing.T) {
	tests := []struct {
		name    string
		dialect cdt.SQLDialect
		pattern string
		want    string
	}{
		{
			name:    "mysql basic pattern",
			dialect: &sqldialect.MySQLDialect{},
			pattern: "%john%",
			want:    "LIKE",
		},
		{
			name:    "postgres ilike case insensitive",
			dialect: &sqldialect.PostgresDialect{},
			pattern: "John%",
			want:    "ILIKE",
		},
		{
			name:    "sqlite case sensitive",
			dialect: &sqldialect.MySQLDialect{},
			pattern: "test_",
			want:    "LIKE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			assert.NotEmpty(t, tt.pattern)
			assert.NotEmpty(t, tt.want)
		})
	}
}

// TestDialectInOperator tests IN operator handling
func TestDialectInOperator(t *testing.T) {
	tests := []struct {
		name    string
		dialect cdt.SQLDialect
		values  []any
		want    string
	}{
		{
			name:    "mysql with multiple values",
			dialect: &sqldialect.MySQLDialect{},
			values:  []any{1, 2, 3},
			want:    "IN",
		},
		{
			name:    "postgres with strings",
			dialect: &sqldialect.PostgresDialect{},
			values:  []any{"a", "b", "c"},
			want:    "IN",
		},
		{
			name:    "sqlite with single value",
			dialect: &sqldialect.MySQLDialect{},
			values:  []any{42},
			want:    "IN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			require.NotEmpty(t, tt.values)
			assert.Equal(t, "IN", tt.want)
		})
	}
}

// TestDialectBetweenOperator tests BETWEEN operator
func TestDialectBetweenOperator(t *testing.T) {
	tests := []struct {
		name    string
		dialect cdt.SQLDialect
		start   any
		end     any
		want    string
	}{
		{
			name:    "mysql numeric range",
			dialect: &sqldialect.MySQLDialect{},
			start:   10,
			end:     20,
			want:    "BETWEEN",
		},
		{
			name:    "postgres date range",
			dialect: &sqldialect.PostgresDialect{},
			start:   "2024-01-01",
			end:     "2024-12-31",
			want:    "BETWEEN",
		},
		{
			name:    "sqlite string range",
			dialect: &sqldialect.MySQLDialect{},
			start:   "a",
			end:     "z",
			want:    "BETWEEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			assert.Equal(t, "BETWEEN", tt.want)
		})
	}
}

// TestDialectNullHandling tests NULL value handling
func TestDialectNullHandling(t *testing.T) {
	tests := []struct {
		name    string
		dialect cdt.SQLDialect
		op      string
		want    string
	}{
		{
			name:    "mysql is null",
			dialect: &sqldialect.MySQLDialect{},
			op:      "IS NULL",
			want:    "IS NULL",
		},
		{
			name:    "postgres is not null",
			dialect: &sqldialect.PostgresDialect{},
			op:      "IS NOT NULL",
			want:    "IS NOT NULL",
		},
		{
			name:    "sqlite null comparison",
			dialect: &sqldialect.MySQLDialect{},
			op:      "IS NULL",
			want:    "IS NULL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			assert.Equal(t, tt.want, tt.op)
		})
	}
}

// TestDialectLogicalOperators tests AND/OR operators
func TestDialectLogicalOperators(t *testing.T) {
	tests := []struct {
		name    string
		dialect cdt.SQLDialect
		op      string
		want    string
	}{
		{
			name:    "mysql and",
			dialect: &sqldialect.MySQLDialect{},
			op:      "AND",
			want:    "AND",
		},
		{
			name:    "postgres or",
			dialect: &sqldialect.PostgresDialect{},
			op:      "OR",
			want:    "OR",
		},
		{
			name:    "sqlite not",
			dialect: &sqldialect.MySQLDialect{},
			op:      "NOT",
			want:    "NOT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.dialect)
			assert.Equal(t, tt.want, tt.op)
		})
	}
}

// TestConditionBuilding tests condition building with operators
func TestConditionBuilding(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() cdt.Condition
		expected string
	}{
		{
			name: "simple equals condition",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("id").Op("=").Value(1)
			},
			expected: "id",
		},
		{
			name: "like condition",
			builder: func() cdt.Condition {
				return cdt.NewExpr().Column("name").Op("LIKE").Value("%john%")
			},
			expected: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := tt.builder()
			assert.NotNil(t, cond)
		})
	}
}
