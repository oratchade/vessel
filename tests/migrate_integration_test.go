//go:build integration

package tests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	migrate "tounilab.com/vessel/migrate/v1"
)

// routineMigration is a statement body with inner semicolons, which goose must
// send whole: it only works when the StatementBegin/End markers are honoured.
func routineMigration(driver, name string) (up, drop string) {
	switch driver {
	case "postgres":
		return fmt.Sprintf("CREATE FUNCTION %s() RETURNS integer AS $$\nBEGIN\n    PERFORM 1;\n    RETURN 1;\nEND;\n$$ LANGUAGE plpgsql;", name),
			fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", name)
	case "mysql":
		return fmt.Sprintf("CREATE PROCEDURE %s()\nBEGIN\n    SELECT 1;\n    SELECT 2;\nEND;", name),
			fmt.Sprintf("DROP PROCEDURE IF EXISTS %s", name)
	case "sqlserver":
		return fmt.Sprintf("CREATE PROCEDURE %s AS\nBEGIN\n    SELECT 1;\n    SELECT 2;\nEND;", name),
			fmt.Sprintf("DROP PROCEDURE IF EXISTS %s", name)
	default: // sqlite
		return fmt.Sprintf("CREATE TRIGGER %s AFTER INSERT ON %s_items\nBEGIN\n    SELECT 1;\n    SELECT 2;\nEND;", name, strings.TrimSuffix(name, "_routine")),
			fmt.Sprintf("DROP TRIGGER IF EXISTS %s", name)
	}
}

type migrationFixture struct {
	fsys         fstest.MapFS
	versionTable string
	cleanup      []string
}

// newMigrationFixture names every object after the test, so parallel dialects and
// reruns never meet in the shared test database.
func newMigrationFixture(t *testing.T, driver string) migrationFixture {
	t.Helper()
	prefix := fmt.Sprintf("vm_%d", time.Now().UnixNano()%1_000_000_000)
	routineUp, routineDrop := routineMigration(driver, prefix+"_routine")
	versionTable := prefix + "_version"
	return migrationFixture{
		fsys: fstest.MapFS{
			"00001_items.sql": {Data: []byte(fmt.Sprintf(
				"-- +goose Up\nCREATE TABLE %s_items (id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL);\n", prefix))},
			"00002_items_name.sql": {Data: []byte(fmt.Sprintf(
				"-- +goose Up\nCREATE INDEX %s_items_name ON %s_items (name);\n", prefix, prefix))},
			"00003_routine.sql": {Data: []byte(
				"-- +goose Up\n-- +goose StatementBegin\n" + routineUp + "\n-- +goose StatementEnd\n")},
		},
		versionTable: versionTable,
		// Dropping the table drops its index and, on SQLite, its trigger.
		cleanup: []string{routineDrop, "DROP TABLE IF EXISTS " + prefix + "_items", "DROP TABLE IF EXISTS " + versionTable},
	}
}

func (f migrationFixture) drop(t *testing.T, conn v1.DB) {
	t.Helper()
	for _, stmt := range f.cleanup {
		if _, err := conn.Exec(context.Background(), stmt); err != nil {
			t.Logf("cleanup %q: %v", stmt, err)
		}
	}
}

func migrationConfig(testDB TestDB) v1.DBConfig {
	if cfg, ok := testDB.config.(v1.SQLiteConfig); ok {
		// The matrix's SQLite is a shared-cache in-memory database, which a second
		// pool (the Migrator's) does not share with connectIntegrationDB's.
		return v1.SQLiteConfig{FilePath: cfg.FilePath, CacheMode: cfg.CacheMode, Mode: cfg.Mode}
	}
	cfg, _ := testDB.config.(v1.DBConfig)
	return cfg
}

func TestIntegrationMigrate(t *testing.T) {
	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			conn := connectIntegrationDB(t, testDB)
			ctx := context.Background()
			cfg := migrationConfig(testDB)

			t.Run("applies pending migrations once", func(t *testing.T) {
				fixture := newMigrationFixture(t, testDB.driver)
				t.Cleanup(func() { fixture.drop(t, conn) })

				m, err := migrate.New(cfg, fixture.fsys, migrate.WithVersionTable(fixture.versionTable))
				require.NoError(t, err)
				t.Cleanup(func() { assert.NoError(t, m.Close()) })

				results, err := m.Up(ctx)
				require.NoError(t, err)
				assert.Len(t, results, 3)

				again, err := m.Up(ctx)
				require.NoError(t, err)
				assert.Empty(t, again)

				version, err := m.Version(ctx)
				require.NoError(t, err)
				assert.Equal(t, int64(3), version)

				statuses, err := m.Status(ctx)
				require.NoError(t, err)
				require.Len(t, statuses, 3)
				for _, s := range statuses {
					assert.Equal(t, migrate.StateApplied, s.State, s.Source)
				}
			})

			t.Run("stops at a failing migration", func(t *testing.T) {
				fixture := newMigrationFixture(t, testDB.driver)
				t.Cleanup(func() { fixture.drop(t, conn) })
				fixture.fsys["00004_broken.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE (;\n")}

				m, err := migrate.New(cfg, fixture.fsys, migrate.WithVersionTable(fixture.versionTable))
				require.NoError(t, err)
				t.Cleanup(func() { assert.NoError(t, m.Close()) })

				_, err = m.Up(ctx)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "00004_broken.sql")

				version, err := m.Version(ctx)
				require.NoError(t, err)
				assert.Equal(t, int64(3), version)
			})
		})
	}
}

// Two processes migrating one database at once, as two API replicas or a retried
// provisioning job would, must apply each migration exactly once. Only the dialects
// with a goose locker are covered: SQLite and MSSQL leave this to the caller.
func TestIntegrationMigrateConcurrentUp(t *testing.T) {
	for _, testDB := range getFilteredDatabases() {
		if testDB.driver != "postgres" && testDB.driver != "mysql" {
			continue
		}
		t.Run(testDB.name, func(t *testing.T) {
			conn := connectIntegrationDB(t, testDB)
			ctx := context.Background()
			fixture := newMigrationFixture(t, testDB.driver)
			// Hold the first migration open for a second before its CREATE TABLE, so the
			// runners are certain to overlap: without a lock, the others reach the same
			// CREATE TABLE and fail with "already exists".
			sleep := map[string]string{"postgres": "SELECT pg_sleep(1);", "mysql": "DO SLEEP(1);"}[testDB.driver]
			first := fixture.fsys["00001_items.sql"]
			first.Data = []byte(strings.Replace(string(first.Data), "-- +goose Up\n", "-- +goose Up\n"+sleep+"\n", 1))
			t.Cleanup(func() {
				fixture.drop(t, conn)
				if testDB.driver == "mysql" {
					_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS goose_lock")
				}
			})

			const runners = 3
			var (
				wg      sync.WaitGroup
				mu      sync.Mutex
				applied int
				errs    []error
			)
			for range runners {
				wg.Add(1)
				go func() {
					defer wg.Done()
					m, err := migrate.New(migrationConfig(testDB), fixture.fsys, migrate.WithVersionTable(fixture.versionTable))
					if err != nil {
						mu.Lock()
						errs = append(errs, err)
						mu.Unlock()
						return
					}
					defer func() { _ = m.Close() }()
					results, err := m.Up(ctx)
					mu.Lock()
					defer mu.Unlock()
					applied += len(results)
					if err != nil {
						errs = append(errs, err)
					}
				}()
			}
			wg.Wait()

			assert.Empty(t, errs)
			assert.Equal(t, 3, applied, "each migration applied by exactly one runner")
		})
	}
}
