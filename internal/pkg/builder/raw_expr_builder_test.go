//go:build test

package builder_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/internal/pkg/builder"
	"tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

var postgresPlaceholder = regexp.MustCompile(`\$(\d+)`)

type rawExprDialect struct {
	name        string
	qb          builder.QueryBuilder
	left, right string
	placeholder string
	upsert      bool
}

// expect converts a PostgreSQL-shaped expected query to this dialect's quoting and placeholders.
func (d rawExprDialect) expect(postgres string) string {
	return postgresPlaceholder.ReplaceAllString(convertQuotedExpected(postgres, d.left, d.right), d.placeholder)
}

func rawExprDialects() []rawExprDialect {
	return []rawExprDialect{
		{
			name: "postgres", qb: builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{}),
			left: `"`, right: `"`, placeholder: "$$${1}", upsert: true,
		},
		{
			name: "mysql", qb: builder.NewMySQLQueryBuilder(sqldialect.MySQLDialect{}),
			left: "`", right: "`", placeholder: "?", upsert: true,
		},
		{
			name: "sqlite", qb: builder.NewSQLiteQueryBuilder(sqldialect.SQLiteDialect{}),
			left: "`", right: "`", placeholder: "?", upsert: true,
		},
		{
			name: "mssql", qb: builder.NewMSSQLQueryBuilder(sqldialect.MSSQLDialect{}),
			left: "[", right: "]", placeholder: "@p${1}",
		},
	}
}

func TestRawExprValuesRenderInline(t *testing.T) {
	for _, d := range rawExprDialects() {
		t.Run(d.name, func(t *testing.T) {
			t.Run("insert", func(t *testing.T) {
				query, args, err := d.qb.Insert("jobs", map[string]any{
					"completed_at": cdt.RawExpr("NOW()"),
					"id":           7,
					"name":         "a",
				}, nil)
				require.NoError(t, err)
				assert.Equal(t, d.expect(`INSERT INTO "jobs" ("completed_at", "id", "name") VALUES (NOW(), $1, $2);`), query)
				assert.Equal(t, []any{7, "a"}, args)
			})

			t.Run("bulk insert mixes raw and bound rows", func(t *testing.T) {
				query, args, err := d.qb.Inserts("jobs", []map[string]any{
					{"created_at": cdt.RawExpr("NOW()"), "id": 1},
					{"created_at": "2026-01-01", "id": 2},
				}, nil)
				require.NoError(t, err)
				assert.Equal(t, d.expect(`INSERT INTO "jobs" ("created_at", "id") VALUES (NOW(), $1), ($2, $3);`), query)
				assert.Equal(t, []any{1, "2026-01-01", 2}, args)
			})

			t.Run("update set and where", func(t *testing.T) {
				query, args, err := d.qb.Update("jobs", map[string]any{
					"attempts":     cdt.RawExpr("attempts + 1"),
					"completed_at": cdt.RawExpr("NOW()"),
					"status":       "done",
				}, nil, cdt.NewExpr().Column("expires_at").Op(">").Value(cdt.RawExpr("NOW()")), nil)
				require.NoError(t, err)
				assert.Equal(t, d.expect(
					`UPDATE "jobs" SET "attempts" = attempts + 1, "completed_at" = NOW(), "status" = $1 WHERE "expires_at" > NOW();`,
				), query)
				assert.Equal(t, []any{"done"}, args)
			})

			t.Run("update numbering continues into where", func(t *testing.T) {
				query, args, err := d.qb.Update("jobs", map[string]any{
					"completed_at": cdt.RawExpr("NOW()"),
					"status":       "done",
				}, nil, cdt.NewExpr().Column("id").Op("=").Value(5), nil)
				require.NoError(t, err)
				assert.Equal(t, d.expect(`UPDATE "jobs" SET "completed_at" = NOW(), "status" = $1 WHERE "id" = $2;`), query)
				assert.Equal(t, []any{"done", 5}, args)
			})
		})
	}
}

func TestRawExprUpsertValues(t *testing.T) {
	upsertOpts := &options.UpsertOptions{
		ConflictColumns: []string{"scope_key"},
		Action:          options.UpsertDoUpdate,
		UpdateValues:    map[string]any{"n": cdt.RawExpr("counters.n + 1"), "updated_at": "later"},
	}
	onConflict := ` ON CONFLICT ("scope_key") DO UPDATE SET "created_at" = excluded."created_at", ` +
		`"n" = counters.n + 1, "updated_at" = $%s;`
	mysqlUpdate := " ON DUPLICATE KEY UPDATE `created_at` = VALUES(`created_at`), `n` = counters.n + 1, `updated_at` = ?;"

	for _, d := range rawExprDialects() {
		if !d.upsert {
			continue
		}
		t.Run(d.name, func(t *testing.T) {
			expect := func(values, lastIndex string) string {
				insert := `INSERT INTO "counters" ("created_at", "n", "scope_key") VALUES ` + values
				if d.name == "mysql" {
					return d.expect(insert) + mysqlUpdate
				}
				return d.expect(insert + fmtIndex(onConflict, lastIndex))
			}

			t.Run("single", func(t *testing.T) {
				query, args, err := d.qb.Upsert(
					"counters",
					map[string]any{"created_at": cdt.RawExpr("NOW()"), "n": 1, "scope_key": "k"},
					upsertOpts,
					nil,
				)
				require.NoError(t, err)
				assert.Equal(t, expect(`(NOW(), $1, $2)`, "3"), query)
				assert.Equal(t, []any{1, "k", "later"}, args)
			})

			t.Run("bulk", func(t *testing.T) {
				query, args, err := d.qb.Upserts("counters", []map[string]any{
					{"created_at": cdt.RawExpr("NOW()"), "n": 1, "scope_key": "a"},
					{"created_at": cdt.RawExpr("NOW()"), "n": 2, "scope_key": "b"},
				}, upsertOpts, nil)
				require.NoError(t, err)
				assert.Equal(t, expect(`(NOW(), $1, $2), (NOW(), $3, $4)`, "5"), query)
				assert.Equal(t, []any{1, "a", 2, "b", "later"}, args)
			})
		})
	}
}

func fmtIndex(template, index string) string {
	return regexp.MustCompile(`\$%s`).ReplaceAllLiteralString(template, "$"+index)
}

func TestRawExprEmptyValueErrors(t *testing.T) {
	empty := cdt.RawExpr("  ")
	idIsOne := func() cdt.Condition { return cdt.NewExpr().Column("id").Op("=").Value(1) }

	for _, d := range rawExprDialects() {
		t.Run(d.name, func(t *testing.T) {
			cases := []struct {
				name       string
				upsertOnly bool
				build      func() error
			}{
				{name: "insert", build: func() error {
					_, _, err := d.qb.Insert("jobs", map[string]any{"a": empty}, nil)
					return err
				}},
				{name: "bulk insert", build: func() error {
					_, _, err := d.qb.Inserts("jobs", []map[string]any{{"a": 1}, {"a": empty}}, nil)
					return err
				}},
				{name: "update set", build: func() error {
					_, _, err := d.qb.Update("jobs", map[string]any{"a": empty}, nil, idIsOne(), nil)
					return err
				}},
				{name: "update where", build: func() error {
					cond := cdt.NewExpr().Column("id").Op("=").Value(empty)
					_, _, err := d.qb.Update("jobs", map[string]any{"a": 1}, nil, cond, nil)
					return err
				}},
				{name: "upsert update value", upsertOnly: true, build: func() error {
					_, _, err := d.qb.Upsert("jobs", map[string]any{"id": 1, "a": 1}, &options.UpsertOptions{
						ConflictColumns: []string{"id"},
						Action:          options.UpsertDoUpdate,
						UpdateValues:    map[string]any{"a": empty},
					}, nil)
					return err
				}},
			}

			for _, tc := range cases {
				if tc.upsertOnly && !d.upsert {
					continue
				}
				t.Run(tc.name, func(t *testing.T) {
					err := tc.build()
					require.Error(t, err)
					assert.Contains(t, err.Error(), "raw expression")
				})
			}
		})
	}
}
