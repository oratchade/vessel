//go:build integration

//nolint:testpackage
package tests

import (
	"context"
	"strings"
	"testing"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/condition"
)

const (
	upsertWhereRelationships = "upsert_where_relationships"
	upsertWhereClaims        = "upsert_where_claims"
)

func upsertWhereSchema(driver string) []string {
	switch driver {
	case "sqlite", "postgres":
		textType, idType := "TEXT", "INTEGER PRIMARY KEY AUTOINCREMENT"
		if driver == "postgres" {
			textType, idType = "VARCHAR(64)", "SERIAL PRIMARY KEY"
		}
		return []string{
			"DROP TABLE IF EXISTS " + upsertWhereRelationships,
			"DROP TABLE IF EXISTS " + upsertWhereClaims,
			"CREATE TABLE " + upsertWhereRelationships + " (id " + idType + ", source_id " + textType +
				" NOT NULL, target_id " + textType + " NOT NULL, label " + textType + ", deleted_at " + textType + ")",
			"CREATE UNIQUE INDEX " + upsertWhereRelationships + "_live ON " + upsertWhereRelationships +
				" (source_id, target_id) WHERE deleted_at IS NULL",
			"CREATE TABLE " + upsertWhereClaims + " (scope_key " + textType + " PRIMARY KEY, owner " + textType +
				" NOT NULL, expires_at BIGINT NOT NULL)",
		}
	default:
		return nil
	}
}

// TestIntegration_UpsertConflictPredicates covers ON CONFLICT target and DO UPDATE predicates.
func TestIntegration_UpsertConflictPredicates(t *testing.T) {
	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectIntegrationDB(t, testDB)
			defer func() { _ = database.Close() }()
			ctx := context.Background()

			schema := upsertWhereSchema(testDB.driver)
			if schema == nil {
				_, err := v1.NewFluentDB(database).Insert().Into(upsertWhereClaims).
					Set("scope_key", "k1").
					OnConflict("scope_key").
					TargetWhere(condition.IsNull("deleted_at")).
					DoNothing().
					Upsert(ctx)
				if err == nil || !(strings.Contains(err.Error(), "MySQL does not support conflict predicates") ||
					strings.Contains(err.Error(), "MSSQL upsert is not supported")) {
					t.Fatalf("expected unsupported conflict predicate error, got %v", err)
				}
				return
			}
			for _, stmt := range schema {
				if _, err := database.Exec(ctx, stmt); err != nil {
					t.Fatalf("schema statement %q failed: %v", stmt, err)
				}
			}

			t.Run("partial unique index target", func(t *testing.T) {
				testUpsertTargetWhere(ctx, t, database)
			})
			t.Run("conditional do update", func(t *testing.T) {
				testUpsertUpdateWhere(ctx, t, database)
			})
		})
	}
}

func testUpsertTargetWhere(ctx context.Context, t *testing.T, database v1.DB) {
	t.Helper()
	fluent := v1.NewFluentDB(database)
	_, err := fluent.Insert().Into(upsertWhereRelationships).ValuesBulk([]map[string]any{
		{"source_id": "s1", "target_id": "t1", "label": "deleted", "deleted_at": "2026-01-01"},
		{"source_id": "s1", "target_id": "t1", "label": "live"},
		{"source_id": "s2", "target_id": "t2", "label": "deleted", "deleted_at": "2026-01-01"},
	}).Exec(ctx)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	upsert := func(source, target, label string) *v1.InsertBuilder {
		return fluent.Insert().Into(upsertWhereRelationships).
			Set("source_id", source).
			Set("target_id", target).
			Set("label", label).
			OnConflict("source_id", "target_id").
			DoUpdate("label")
	}

	if _, err := upsert("s1", "t1", "no-predicate").Upsert(ctx); err == nil {
		t.Fatalf("expected ON CONFLICT without the index predicate to be rejected")
	}
	if _, err := upsert("s1", "t1", "updated").TargetWhere(condition.IsNull("deleted_at")).Upsert(ctx); err != nil {
		t.Fatalf("upsert with TargetWhere failed: %v", err)
	}
	if _, err := upsert("s2", "t2", "revived").TargetWhere(condition.IsNull("deleted_at")).Upsert(ctx); err != nil {
		t.Fatalf("upsert over soft-deleted duplicate failed: %v", err)
	}

	live, err := fluent.Select(upsertWhereRelationships, "source_id", "label").
		Where(condition.IsNull("deleted_at")).
		OrderByAsc("source_id").
		Get(ctx)
	if err != nil {
		t.Fatalf("select live rows failed: %v", err)
	}
	if len(live) != 2 || live[0]["label"] != "updated" || live[1]["label"] != "revived" {
		t.Fatalf("unexpected live rows: %#v", live)
	}
	deleted, err := fluent.Select(upsertWhereRelationships, "label").
		Where(condition.IsNotNull("deleted_at")).
		Get(ctx)
	if err != nil {
		t.Fatalf("select deleted rows failed: %v", err)
	}
	if len(deleted) != 2 || deleted[0]["label"] != "deleted" || deleted[1]["label"] != "deleted" {
		t.Fatalf("soft-deleted rows were modified: %#v", deleted)
	}
}

func testUpsertUpdateWhere(ctx context.Context, t *testing.T, database v1.DB) {
	t.Helper()
	fluent := v1.NewFluentDB(database)
	_, err := fluent.Insert().Into(upsertWhereClaims).
		Set("scope_key", "k1").Set("owner", "first").Set("expires_at", 100).
		Exec(ctx)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	claim := func(cutoff int) *v1.ExecResult {
		t.Helper()
		result, err := fluent.Insert().Into(upsertWhereClaims).
			Set("scope_key", "k1").Set("owner", "second").Set("expires_at", 200).
			OnConflict("scope_key").
			DoUpdateSet(map[string]any{"owner": "second", "expires_at": 200}).
			UpdateWhere(condition.NewExpr().Column(upsertWhereClaims + ".expires_at").Op("<=").Value(cutoff)).
			Upsert(ctx)
		if err != nil {
			t.Fatalf("conditional upsert failed: %v", err)
		}
		return result
	}
	owner := func() any {
		t.Helper()
		rows, err := fluent.Select(upsertWhereClaims, "owner").
			Where(condition.NewExpr().Column("scope_key").Op("=").Value("k1")).
			Get(ctx)
		if err != nil || len(rows) != 1 {
			t.Fatalf("select claim failed: rows=%v err=%v", rows, err)
		}
		return rows[0]["owner"]
	}

	if result := claim(50); result.RowsAffected != 0 {
		t.Fatalf("expected no-op when predicate is false, got %d rows affected", result.RowsAffected)
	}
	if got := owner(); got != "first" {
		t.Fatalf("claim changed while predicate was false: %v", got)
	}
	if result := claim(150); result.RowsAffected != 1 {
		t.Fatalf("expected update when predicate is true, got %d rows affected", result.RowsAffected)
	}
	if got := owner(); got != "second" {
		t.Fatalf("claim was not taken over: %v", got)
	}
}
