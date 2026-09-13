//go:build test

package builder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/internal/pkg/builder"
	"tounilab.com/vessel/internal/pkg/sqldialect"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

func TestUpsertConflictPredicates(t *testing.T) {
	single := map[string]any{"email": "a@example.com", "name": "Alice"}
	bulk := []map[string]any{
		{"email": "a@example.com", "name": "Alice"},
		{"email": "b@example.com", "name": "Bob"},
	}
	pg := builder.NewPostgresQueryBuilder(sqldialect.PostgresDialect{})
	lite := builder.NewSQLiteQueryBuilder(sqldialect.SQLiteDialect{})
	my := builder.NewMySQLQueryBuilder(sqldialect.MySQLDialect{})

	tenant := func() cdt.Condition { return cdt.NewExpr().Column("tenant_id").Op("=").Value("t1") }
	version := func() cdt.Condition { return cdt.NewExpr().Column("users.version").Op("<").Value(5) }

	tests := []struct {
		name     string
		qb       builder.QueryBuilder
		bulk     bool
		opts     options.UpsertOptions
		wantSQL  string
		wantArgs []any
		wantErr  string
	}{
		{
			name: "postgres target predicate only",
			qb:   pg,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				TargetWhere:     cdt.IsNull("deleted_at"),
			},
			wantSQL:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") WHERE "deleted_at" IS NULL DO UPDATE SET "name" = excluded."name";`,
			wantArgs: []any{"a@example.com", "Alice"},
		},
		{
			name: "postgres update predicate only",
			qb:   pg,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				UpdateWhere:     version(),
			},
			wantSQL:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name" WHERE "users"."version" < $3;`,
			wantArgs: []any{"a@example.com", "Alice", 5},
		},
		{
			name: "postgres both predicates with update values keep textual arg order",
			qb:   pg,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				UpdateValues:    map[string]any{"name": "Updated"},
				TargetWhere:     tenant(),
				UpdateWhere:     version(),
			},
			wantSQL:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") WHERE "tenant_id" = $3 DO UPDATE SET "name" = $4 WHERE "users"."version" < $5;`,
			wantArgs: []any{"a@example.com", "Alice", "t1", "Updated", 5},
		},
		{
			name: "postgres bulk both predicates",
			qb:   pg,
			bulk: true,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				UpdateValues:    map[string]any{"name": "Updated"},
				TargetWhere:     tenant(),
				UpdateWhere:     version(),
			},
			wantSQL:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2), ($3, $4) ON CONFLICT ("email") WHERE "tenant_id" = $5 DO UPDATE SET "name" = $6 WHERE "users"."version" < $7;`,
			wantArgs: []any{"a@example.com", "Alice", "b@example.com", "Bob", "t1", "Updated", 5},
		},
		{
			name: "postgres do nothing with target predicate",
			qb:   pg,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoNothing,
				TargetWhere:     tenant(),
			},
			wantSQL:  `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") WHERE "tenant_id" = $3 DO NOTHING;`,
			wantArgs: []any{"a@example.com", "Alice", "t1"},
		},
		{
			name: "sqlite both predicates",
			qb:   lite,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				UpdateValues:    map[string]any{"name": "Updated"},
				TargetWhere:     cdt.IsNull("deleted_at"),
				UpdateWhere:     version(),
			},
			wantSQL:  "INSERT INTO `users` (`email`, `name`) VALUES (?, ?) ON CONFLICT (`email`) WHERE `deleted_at` IS NULL DO UPDATE SET `name` = ? WHERE `users`.`version` < ?;",
			wantArgs: []any{"a@example.com", "Alice", "Updated", 5},
		},
		{
			name: "sqlite rejects bound target predicate",
			qb:   lite,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoNothing,
				TargetWhere:     tenant(),
			},
			wantErr: "SQLite cannot match a bound TargetWhere value",
		},
		{
			name: "sqlite bulk rejects bound target predicate",
			qb:   lite,
			bulk: true,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				TargetWhere:     tenant(),
			},
			wantErr: "SQLite cannot match a bound TargetWhere value",
		},
		{
			name: "sqlite bulk do nothing with target predicate",
			qb:   lite,
			bulk: true,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoNothing,
				TargetWhere:     cdt.IsNull("deleted_at"),
			},
			wantSQL:  "INSERT INTO `users` (`email`, `name`) VALUES (?, ?), (?, ?) ON CONFLICT (`email`) WHERE `deleted_at` IS NULL DO NOTHING;",
			wantArgs: []any{"a@example.com", "Alice", "b@example.com", "Bob"},
		},
		{
			name: "update predicate requires do update",
			qb:   pg,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoNothing,
				UpdateWhere:     version(),
			},
			wantErr: "UpdateWhere requires DO UPDATE",
		},
		{
			name: "bulk update predicate requires do update",
			qb:   lite,
			bulk: true,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoNothing,
				UpdateWhere:     version(),
			},
			wantErr: "UpdateWhere requires DO UPDATE",
		},
		{
			name: "mysql rejects target predicate",
			qb:   my,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoNothing,
				TargetWhere:     tenant(),
			},
			wantErr: "MySQL does not support conflict predicates",
		},
		{
			name: "mysql bulk rejects update predicate",
			qb:   my,
			bulk: true,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				UpdateWhere:     version(),
			},
			wantErr: "MySQL does not support conflict predicates",
		},
		{
			name: "invalid target predicate propagates",
			qb:   pg,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				TargetWhere:     cdt.NewExpr(),
			},
			wantErr: "invalid expression",
		},
		{
			name: "invalid update predicate propagates",
			qb:   lite,
			bulk: true,
			opts: options.UpsertOptions{
				ConflictColumns: []string{"email"},
				Action:          options.UpsertDoUpdate,
				UpdateWhere:     cdt.NewExpr(),
			},
			wantErr: "invalid expression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				query string
				args  []any
				err   error
			)
			if tt.bulk {
				query, args, err = tt.qb.Upserts("users", bulk, &tt.opts, nil)
			} else {
				query, args, err = tt.qb.Upsert("users", single, &tt.opts, nil)
			}
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSQL, query)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}
