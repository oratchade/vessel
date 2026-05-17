//go:build test

package builder_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/fabric/internal/pkg/builder"
	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
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
			name:     "qualified wildcard",
			input:    "table.*",
			expected: `"table".*`,
		},
		{
			name:     "qualified with multiple parts",
			input:    "db.schema.table.col",
			expected: `"db"."schema"."table"."col"`,
		},
		{
			name:     "escaped column and alias delimiters",
			input:    `weird"col AS alias"name`,
			expected: `"weird""col" AS "alias""name"`,
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
			name:     "function with alias",
			input:    "COUNT(*) AS count",
			expected: `COUNT(*) AS "count"`,
		},
		{
			name:     "function with lowercase alias",
			input:    "count(*) as count",
			expected: `count(*) AS "count"`,
		},
		{
			name:     "column with lowercase as alias",
			input:    "col as alias",
			expected: `"col" AS "alias"`,
		},
		{
			name:     "column with mixed case as alias",
			input:    "col aS alias",
			expected: `"col" AS "alias"`,
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
		{"qualified wildcard", "table.*", `"table".*`},
		{"multiple parts", "db.schema.table.col", `"db"."schema"."table"."col"`},
		{"with alias", "col AS alias", `"col" AS "alias"`},
		{"function with alias", "COUNT(*) AS count", `COUNT(*) AS "count"`},
		{"function with lowercase alias", "count(*) as count", `count(*) AS "count"`},
		{"with lowercase alias", "col as alias", `"col" AS "alias"`},
		{"with mixed case alias", "col aS alias", `"col" AS "alias"`},
		{"qualified with alias", "table.col AS alias", `"table"."col" AS "alias"`},
		{"whitespace tolerance", "  table .  col  AS   alias  ", `"table"."col" AS "alias"`},
		{"all columns", "*", "*"},
	}

	dialects := []struct {
		name       string
		dialect    cdt.SQLDialect
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

func TestBuilderEscapedIdentifiersInSelectClauses(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)
	having := "COUNT(*) > 1"

	sql, args, err := qb.Select(
		"user`table",
		[]string{"user`table.id`col AS alias`name", "COUNT(*) AS total", "user`table.*"},
		[]cdt.Join{{
			Type:  "INNER",
			Table: "org`table",
			Alias: "o`alias",
			Conditions: cdt.JoinCdts{{
				Left:  "org`id",
				Right: "id`col",
			}},
		}},
		&options.QueryOptions{
			GroupBy: []string{"user`table.id`col"},
			Having:  &having,
			OrderBy: []options.OrderBy{{Column: "user`table.id`col", Direction: "DESC"}},
		},
		cdt.NewExpr().Column("user`table.id`col").Op("=").Value(10),
	)

	require.NoError(t, err)
	assert.Equal(t, "SELECT `user``table`.`id``col` AS `alias``name`, COUNT(*) AS `total`, `user``table`.* FROM `user``table` INNER JOIN `org``table` AS `o``alias` ON `user``table`.`org``id` = `o``alias`.`id``col` WHERE `user``table`.`id``col` = ? GROUP BY `user``table`.`id``col` HAVING COUNT(*) > 1 ORDER BY `user``table`.`id``col` DESC;", sql)
	assert.Equal(t, []any{10}, args)
}

func TestSelectParameterizedHavingCondition(t *testing.T) {
	dialect := sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	sql, args, err := qb.Select(
		"users",
		[]string{"department", "COUNT(*) AS total"},
		nil,
		&options.QueryOptions{
			GroupBy: []string{"department"},
			HavingCondition: cdt.NewExpr().
				Column("COUNT(*)").
				Op(">").
				Value(3),
			OrderBy: []options.OrderBy{{Column: "department", Direction: "ASC"}},
		},
		cdt.NewExpr().Column("active").Op("=").Value(true),
	)

	require.NoError(t, err)
	assert.Equal(t, `SELECT "department", COUNT(*) AS "total" FROM "users" WHERE "active" = $1 GROUP BY "department" HAVING COUNT(*) > $2 ORDER BY "department" ASC;`, sql)
	assert.Equal(t, []any{true, 3}, args)
}

func TestMySQLMutationOrderByLimitWithoutJoins(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)
	limit := 5
	opts := &options.QueryOptions{
		Limit:   &limit,
		OrderBy: []options.OrderBy{{Column: "created_at", Direction: "DESC"}},
	}

	updateSQL, updateArgs, err := qb.Update(
		"users",
		map[string]any{"name": "Ada"},
		nil,
		cdt.NewExpr().Column("id").Op("=").Value(1),
		opts,
	)
	require.NoError(t, err)
	assert.Equal(t, "UPDATE `users` SET `name` = ? WHERE `id` = ? ORDER BY `created_at` DESC LIMIT ?;", updateSQL)
	assert.Equal(t, []any{"Ada", 1, 5}, updateArgs)

	deleteSQL, deleteArgs, err := qb.Delete(
		"users",
		nil,
		cdt.NewExpr().Column("active").Op("=").Value(false),
		opts,
	)
	require.NoError(t, err)
	assert.Equal(t, "DELETE FROM `users` WHERE `active` = ? ORDER BY `created_at` DESC LIMIT ?;", deleteSQL)
	assert.Equal(t, []any{false, 5}, deleteArgs)
}

func TestMutationOrderByLimitUnsupportedDialects(t *testing.T) {
	limit := 5
	opts := &options.QueryOptions{Limit: &limit}

	cases := []struct {
		name string
		qb   builder.QueryBuilder
	}{
		{name: "postgres", qb: builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{})},
		{name: "sqlite", qb: builder.NewSQLiteQueryBuilder(sqldialect.SQLiteDialect{})},
		{name: "mssql", qb: builder.NewMSSQLQueryBuilder(sqldialect.MSSQLDialect{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.qb.Update(
				"users",
				map[string]any{"name": "Ada"},
				nil,
				cdt.NewExpr().Column("id").Op("=").Value(1),
				opts,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "does not support OrderBy or Limit")
		})
	}
}

func TestMySQLMutationOrderByLimitWithJoinReturnsError(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)
	limit := 5

	_, _, err := qb.Update(
		"users",
		map[string]any{"name": "Ada"},
		[]cdt.Join{{
			Type:  "INNER",
			Table: "profiles",
			Conditions: cdt.JoinCdts{{
				Left:  "id",
				Right: "user_id",
			}},
		}},
		cdt.NewExpr().Column("active").Op("=").Value(true),
		&options.QueryOptions{Limit: &limit},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support OrderBy or Limit with joined")
}

func TestMutationOffsetReturnsExplicitError(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)
	offset := 10

	_, _, err := qb.Delete(
		"users",
		nil,
		cdt.NewExpr().Column("active").Op("=").Value(false),
		&options.QueryOptions{Offset: &offset},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support Offset")
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
			query, args, err := qb.Inserts(tt.table, tt.data, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, query)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

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
			query, args, err := qb.Inserts(tt.table, tt.data, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, query)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

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
			query, args, err := qb.Inserts(tt.table, tt.data, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, query)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestInserts_EmptyDataError(t *testing.T) {
	dialect := sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	_, _, err := qb.Inserts("users", []map[string]any{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no data provided")
}

// SQLiteTestDialect is a test helper that implements cdt.SQLDialect for SQLite testing.
type SQLiteTestDialect struct {
	*sqldialect.MySQLDialect
}

// _ ensures SQLiteTestDialect implements cdt.SQLDialect
var _ cdt.SQLDialect = (*SQLiteTestDialect)(nil)

// ==================== MySQL JOIN Tests ====================

// TestMysqlUpdateWithJoinAndCondition tests UPDATE with JOIN and WHERE condition
func TestMysqlUpdateWithJoinAndCondition(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]any{
		"status": "active",
	}
	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "orders",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("orders.amount").Op(">").Value(100)

	query, args, err := qb.Update("users", data, joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		"UPDATE `users` INNER JOIN `orders` ON `users`.`id` = `orders`.`user_id` SET `status` = ? WHERE `orders`.`amount` > ?;",
		query,
	)
	assert.Len(t, args, 2) // 1 for SET, 1 for WHERE condition
}

// TestMysqlDeleteWithJoinAndCondition tests DELETE with JOIN and WHERE condition
func TestMysqlDeleteWithJoinAndCondition(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	joins := []cdt.Join{
		{
			Type:  "LEFT",
			Table: "subscriptions",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("subscriptions.status").Op("=").Value("canceled")

	query, args, err := qb.Delete("users", joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		"DELETE `users` FROM `users` LEFT JOIN `subscriptions` ON `users`.`id` = `subscriptions`.`user_id` WHERE `subscriptions`.`status` = ?;",
		query,
	)
	assert.Len(t, args, 1)
}

// ==================== PostgreSQL JOIN Tests ====================

// TestPostgresUpdateWithJoin tests PostgreSQL UPDATE with JOIN using FROM clause
func TestPostgresUpdateWithJoin(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	data := map[string]any{
		"status": "verified",
	}
	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "accounts",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("accounts.verified").Op("=").Value(true)

	query, args, err := qb.Update("users", data, joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		`UPDATE "users" SET "status" = $1 FROM "accounts" WHERE "users"."id" = "accounts"."user_id" AND "accounts"."verified" = $2;`,
		query,
	)
	assert.Len(t, args, 2)
}

// TestPostgresDeleteWithJoin tests PostgreSQL DELETE with JOIN using USING clause
func TestPostgresDeleteWithJoin(t *testing.T) {
	dialect := &sqldialect.PostgresDialect{}
	qb := builder.NewPostgresQueryBuilder(dialect)

	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "logs",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("logs.action").Op("=").Value("delete_request")

	query, args, err := qb.Delete("users", joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		`DELETE FROM "users" USING "logs" WHERE "users"."id" = "logs"."user_id" AND "logs"."action" = $1;`,
		query,
	)
	assert.Len(t, args, 1)
}

// ==================== MSSQL JOIN Tests ====================

// TestMSSQLUpdateWithJoin tests MSSQL UPDATE with JOIN
func TestMSSQLUpdateWithJoin(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	data := map[string]any{
		"department": "Sales",
	}
	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "teams",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("teams.active").Op("=").Value(true)

	query, args, err := qb.Update("users", data, joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		"UPDATE [users] SET [department] = @p1 FROM [users] INNER JOIN [teams] ON [users].[id] = [teams].[user_id] WHERE [teams].[active] = @p2;",
		query,
	)
	assert.Len(t, args, 2)
}

// TestMSSQLDeleteWithJoin tests MSSQL DELETE with JOIN
func TestMSSQLDeleteWithJoin(t *testing.T) {
	dialect := &sqldialect.MSSQLDialect{}
	qb := builder.NewMSSQLQueryBuilder(dialect)

	joins := []cdt.Join{
		{
			Type:  "LEFT",
			Table: "departments",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "dept_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("departments.status").Op("=").Value("inactive")

	query, args, err := qb.Delete("users", joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		"DELETE [users] FROM [users] LEFT JOIN [departments] ON [users].[id] = [departments].[dept_id] WHERE [departments].[status] = @p1;",
		query,
	)
	assert.Len(t, args, 1)
}

// ==================== SQLite JOIN Tests ====================

// TestSQLiteUpdateWithJoin tests SQLite UPDATE with JOIN (should work)
func TestSQLiteUpdateWithJoin(t *testing.T) {
	dialect := &sqldialect.SQLiteDialect{}
	qb := builder.NewSQLiteQueryBuilder(dialect)

	data := map[string]any{
		"active": 1,
	}
	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "profiles",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("profiles.verified").Op("=").Value(1)

	query, args, err := qb.Update("users", data, joins, condition, nil)

	require.NoError(t, err)
	assert.Equal(
		t,
		"UPDATE `users` SET `active` = ? FROM `profiles` WHERE `users`.`id` = `profiles`.`user_id` AND `profiles`.`verified` = ?;",
		query,
	)
	assert.Len(t, args, 2)
}

// TestSQLiteDeleteWithJoinReturnsError tests SQLite DELETE with JOINs returns error
func TestSQLiteDeleteWithJoinReturnsError(t *testing.T) {
	dialect := &SQLiteTestDialect{MySQLDialect: &sqldialect.MySQLDialect{}}
	qb := builder.NewSQLiteQueryBuilder(dialect)

	joins := []cdt.Join{
		{
			Type:  "INNER",
			Table: "archive",
			Conditions: cdt.JoinCdts{
				{Left: "id", Right: "user_id"},
			},
		},
	}
	condition := cdt.NewExpr().Column("archive.status").Op("=").Value("old")

	query, args, err := qb.Delete("users", joins, condition, nil)

	// Should return an error for SQLite
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DELETE with JOINs is not supported in SQLite")
	assert.Empty(t, query)
	assert.Nil(t, args)
}

// TestSQLiteDeleteWithoutJoinWorks tests SQLite DELETE without JOINs still works
func TestSQLiteDeleteWithoutJoinWorks(t *testing.T) {
	dialect := &sqldialect.SQLiteDialect{}
	qb := builder.NewSQLiteQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("id").Op("=").Value(123)

	query, args, err := qb.Delete("users", nil, condition, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM `users`")
	assert.Contains(t, query, "WHERE")
	assert.Len(t, args, 1)
}

// ==================== Backwards Compatibility Tests ====================

// TestUpdateBackwardsCompatibilityWithNilJoins tests UPDATE with nil joins works
func TestUpdateBackwardsCompatibilityWithNilJoins(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	data := map[string]any{
		"email": "new@example.com",
	}
	condition := cdt.NewExpr().Column("id").Op("=").Value(42)

	query, args, err := qb.Update("users", data, nil, condition, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "UPDATE `users`")
	assert.NotContains(t, query, "JOIN")
	assert.Len(t, args, 2)
}

// TestDeleteBackwardsCompatibilityWithNilJoins tests DELETE with nil joins works
func TestDeleteBackwardsCompatibilityWithNilJoins(t *testing.T) {
	dialect := &sqldialect.MySQLDialect{}
	qb := builder.NewMySQLQueryBuilder(dialect)

	condition := cdt.NewExpr().Column("status").Op("=").Value("inactive")

	query, args, err := qb.Delete("users", nil, condition, nil)

	require.NoError(t, err)
	assert.Contains(t, query, "DELETE FROM `users`")
	assert.NotContains(t, query, "JOIN")
	assert.Len(t, args, 1)
}
