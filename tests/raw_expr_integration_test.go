//go:build integration

//nolint:testpackage
package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/condition"
)

type rawExprSQL struct {
	jobsTable     string
	countersTable string
	now           string
	future        string
	past          string
}

func rawExprDialectSQL(driver string) rawExprSQL {
	switch driver {
	case "postgres":
		return rawExprSQL{
			jobsTable:     `CREATE TABLE raw_expr_jobs (id INTEGER PRIMARY KEY, expires_at TIMESTAMPTZ NOT NULL, completed_at TIMESTAMPTZ)`,
			countersTable: `CREATE TABLE raw_expr_counters (scope_key VARCHAR(64) PRIMARY KEY, n INTEGER NOT NULL)`,
			now:           "NOW()",
			future:        "NOW() + INTERVAL '1 hour'",
			past:          "NOW() - INTERVAL '1 hour'",
		}
	case "mysql":
		return rawExprSQL{
			jobsTable:     `CREATE TABLE raw_expr_jobs (id INT PRIMARY KEY, expires_at DATETIME NOT NULL, completed_at DATETIME NULL)`,
			countersTable: `CREATE TABLE raw_expr_counters (scope_key VARCHAR(64) PRIMARY KEY, n INT NOT NULL)`,
			now:           "NOW()",
			future:        "NOW() + INTERVAL 1 HOUR",
			past:          "NOW() - INTERVAL 1 HOUR",
		}
	case "sqlserver":
		return rawExprSQL{
			jobsTable:     `CREATE TABLE raw_expr_jobs (id INT PRIMARY KEY, expires_at DATETIME2 NOT NULL, completed_at DATETIME2 NULL)`,
			countersTable: `CREATE TABLE raw_expr_counters (scope_key VARCHAR(64) PRIMARY KEY, n INT NOT NULL)`,
			now:           "SYSDATETIME()",
			future:        "DATEADD(hour, 1, SYSDATETIME())",
			past:          "DATEADD(hour, -1, SYSDATETIME())",
		}
	default:
		return rawExprSQL{
			jobsTable:     `CREATE TABLE raw_expr_jobs (id INTEGER PRIMARY KEY, expires_at TEXT NOT NULL, completed_at TEXT)`,
			countersTable: `CREATE TABLE raw_expr_counters (scope_key TEXT PRIMARY KEY, n INTEGER NOT NULL)`,
			now:           "CURRENT_TIMESTAMP",
			future:        "datetime('now', '+1 hour')",
			past:          "datetime('now', '-1 hour')",
		}
	}
}

// TestIntegration_RawExprValues executes database-evaluated expressions in VALUES, SET, WHERE, and DO UPDATE SET.
func TestIntegration_RawExprValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer database.Close()

			ctx := context.Background()
			dialectSQL := rawExprDialectSQL(testDB.driver)
			for _, stmt := range []string{
				"DROP TABLE IF EXISTS raw_expr_jobs",
				"DROP TABLE IF EXISTS raw_expr_counters",
				dialectSQL.jobsTable,
				dialectSQL.countersTable,
			} {
				_, err := database.Exec(ctx, stmt)
				require.NoError(t, err, stmt)
			}
			fdb := v1.NewFluentDB(database)

			inserted, err := fdb.Insert().Into("raw_expr_jobs").ValuesBulk([]map[string]any{
				{"id": 1, "expires_at": v1.RawExpr(dialectSQL.future)},
				{"id": 2, "expires_at": v1.RawExpr(dialectSQL.past)},
			}).Exec(ctx)
			require.NoError(t, err)
			require.EqualValues(t, 2, inserted.RowsAffected)

			completed, err := fdb.Update("raw_expr_jobs").
				Set("completed_at", v1.RawExpr(dialectSQL.now)).
				Where(condition.NewExpr().Column("expires_at").Op(">").Value(v1.RawExpr(dialectSQL.now))).
				Exec(ctx)
			require.NoError(t, err)
			require.EqualValues(t, 1, completed.RowsAffected)

			rows, err := database.Get(ctx, "raw_expr_jobs", []string{"id"}, nil,
				condition.NewExpr().Column("completed_at").Op("IS NOT NULL"), nil)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, "1", fmt.Sprint(rows[0]["id"]))

			if testDB.driver == "sqlserver" {
				return // MSSQL upsert is unsupported.
			}
			for range 2 {
				_, err := fdb.Insert().Into("raw_expr_counters").
					Set("scope_key", "k").
					Set("n", 1).
					OnConflict("scope_key").
					DoUpdateSet(map[string]any{"n": v1.RawExpr("raw_expr_counters.n + 1")}).
					Exec(ctx)
				require.NoError(t, err)
			}
			counters, err := database.Get(ctx, "raw_expr_counters", []string{"n"}, nil,
				condition.NewExpr().Column("scope_key").Op("=").Value("k"), nil)
			require.NoError(t, err)
			require.Len(t, counters, 1)
			require.Equal(t, "2", fmt.Sprint(counters[0]["n"]))
		})
	}
}
