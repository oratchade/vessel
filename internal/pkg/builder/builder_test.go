package builder_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"tounilab.com/db-connector/internal/pkg/builder"
	"tounilab.com/db-connector/internal/pkg/sqldialect"
	"tounilab.com/db-connector/pkg/query/condition"
)

func convertQuotedExpected(base, left, right string) string {
	parts := strings.Split(base, `"`)
	if len(parts) == 1 {
		return base
	}
	var b strings.Builder
	for i, p := range parts {
		b.WriteString(p)
		if i < len(parts)-1 {
			// even index -> left quote, odd -> right quote (we alternate)
			if i%2 == 0 {
				b.WriteString(left)
			} else {
				b.WriteString(right)
			}
		}
	}
	return b.String()
}

func TestSanitizeColumn(t *testing.T) {
	dialect := sqldialect.PostgresDialect{}

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple column",
			input:    "col",
			expected: `"col"`,
		},
		{
			name:     "qualified column table.column",
			input:    "table.col",
			expected: `"table"."col"`,
		},
		{
			name:     "qualified with multiple parts",
			input:    "db.schema.table.col",
			expected: `"db"."schema"."table"."col"`,
		},
		{
			name:     "column with AS alias",
			input:    "col AS alias",
			expected: `"col" AS "alias"`,
		},
		{
			name:     "qualified column with AS alias",
			input:    "table.col AS alias",
			expected: `"table"."col" AS "alias"`,
		},
		{
			name:     "whitespace tolerance",
			input:    "  table .  col  AS   alias  ",
			expected: `"table"."col" AS "alias"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := builder.ExportSanitizeColumn(dialect, tc.input)
			assert.Equal(t, tc.expected, got, "sanitizeColumn(%q)", tc.input)
		})
	}
}

func TestSanitizeColumn_MultipleDialects(t *testing.T) {
	baseDialect := sqldialect.PostgresDialect{}

	cases := []struct {
		name     string
		input    string
		expected string // expected for postgres (double quotes)
	}{
		{"simple column", "col", `"col"`},
		{"qualified column", "table.col", `"table"."col"`},
		{"multiple parts", "db.schema.table.col", `"db"."schema"."table"."col"`},
		{"with alias", "col AS alias", `"col" AS "alias"`},
		{"qualified with alias", "table.col AS alias", `"table"."col" AS "alias"`},
		{"whitespace tolerance", "  table .  col  AS   alias  ", `"table"."col" AS "alias"`},
		{"all columns", "*", "*"},
	}

	dialects := []struct {
		name       string
		dialect    condition.SQLDialect
		leftQuote  string
		rightQuote string
	}{
		{"postgres", sqldialect.PostgresDialect{}, `"`, `"`},
		{"sqlite", sqldialect.MySQLDialect{}, "`", "`"},
		{"mysql", sqldialect.MySQLDialect{}, "`", "`"},
		{"mssql", sqldialect.MSSQLDialect{}, "[", "]"},
	}

	for _, d := range dialects {
		t.Run(d.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					// compute expected for this dialect by converting base (double-quoted) expectation
					expected := convertQuotedExpected(tc.expected, d.leftQuote, d.rightQuote)
					got := builder.ExportSanitizeColumn(d.dialect, tc.input)
					assert.Equal(t, expected, got, "dialect=%s input=%q", d.name, tc.input)
				})
			}
		})
	}

	// sanity: ensure sanitizeColumn still works with explicit Postgres dialect variable used elsewhere
	for _, tc := range cases {
		got := builder.ExportSanitizeColumn(baseDialect, tc.input)
		assert.Equal(t, tc.expected, got, "postgres sanitizeColumn(%q)", tc.input)
	}
}

func TestInserts_MySQL(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	tests := []struct {
		name          string
		table         string
		data          []map[string]any
		expectedQuery string
		expectedArgs  []any
	}{
		{
			name:          "single row",
			table:         "users",
			data:          []map[string]any{{"id": 1, "name": "Alice"}},
			expectedQuery: "INSERT INTO `users` (`id`, `name`) VALUES (?, ?);",
			expectedArgs:  []any{1, "Alice"},
		},
		{
			name:  "multiple rows",
			table: "users",
			data: []map[string]any{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
			expectedQuery: "INSERT INTO `users` (`id`, `name`) VALUES (?, ?), (?, ?);",
			expectedArgs:  []any{1, "Alice", 2, "Bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args, err := qb.Inserts(tt.table, tt.data)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, query)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

//nolint:dupl
func TestInserts_Postgres(t *testing.T) {
	dialect := sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	tests := []struct {
		name          string
		table         string
		data          []map[string]any
		expectedQuery string
		expectedArgs  []any
	}{
		{
			name:  "multiple rows",
			table: "users",
			data: []map[string]any{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
			expectedQuery: `INSERT INTO "users" ("id", "name") VALUES ($1, $2), ($3, $4);`,
			expectedArgs:  []any{1, "Alice", 2, "Bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args, err := qb.Inserts(tt.table, tt.data)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, query)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

//nolint:dupl
func TestInserts_MSSQL(t *testing.T) {
	dialect := sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	tests := []struct {
		name          string
		table         string
		data          []map[string]any
		expectedQuery string
		expectedArgs  []any
	}{
		{
			name:  "multiple rows",
			table: "users",
			data: []map[string]any{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
			expectedQuery: "INSERT INTO [users] ([id], [name]) VALUES (@p1, @p2), (@p3, @p4);",
			expectedArgs:  []any{1, "Alice", 2, "Bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args, err := qb.Inserts(tt.table, tt.data)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, query)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestInserts_EmptyDataError(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	_, _, err := qb.Inserts("users", []map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no data provided")
}
