//go:build test

package builder_test

import (
	"testing"

	"tounilab.com/fabric/internal/pkg/builder"
	"tounilab.com/fabric/internal/pkg/sqldialect"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

func BenchmarkPostgresSelectBuild(b *testing.B) {
	qb := builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{})
	limit := 50
	for b.Loop() {
		_, _, _ = qb.Select(
			"users",
			[]string{"id", "email", "name"},
			nil,
			&options.QueryOptions{
				Limit:   &limit,
				OrderBy: []options.OrderBy{{Column: "created_at", Direction: "DESC"}},
			},
			cdt.NewExpr().Column("active").Op("=").Value(true),
		)
	}
}

func BenchmarkPostgresUpsertBuild(b *testing.B) {
	qb := builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{})
	for b.Loop() {
		_, _, _ = qb.Upsert(
			"users",
			map[string]any{"id": "u1", "email": "a@example.com", "name": "Ada"},
			&options.UpsertOptions{
				ConflictColumns: []string{"id"},
				Action:          options.UpsertDoUpdate,
			},
			nil,
		)
	}
}
