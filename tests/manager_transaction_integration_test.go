//go:build integration

//nolint:testpackage
package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	manager "tounilab.com/vessel/manager/v1"
	"tounilab.com/vessel/manager/v1/config"
	"tounilab.com/vessel/pkg/query/condition"
)

const (
	managerTxTable   = "manager_tx_accounts"
	managerTxEntry   = "primary"
	managerTxTimeout = 10 * time.Second
)

// startIntegrationManager starts a DBManager with a single readwrite entry pointing at testDB.
// It also returns a direct connection for setup and verification outside the manager.
func startIntegrationManager(t *testing.T, testDB TestDB) (*manager.DBManager, v1.DB) {
	t.Helper()

	raw := connectIntegrationDB(t, testDB)
	t.Cleanup(func() { _ = raw.Close() })

	ctx := context.Background()
	_, _ = raw.Exec(ctx, "DROP TABLE IF EXISTS "+managerTxTable)
	_, err := raw.Exec(ctx, "CREATE TABLE "+managerTxTable+" (id INTEGER PRIMARY KEY, balance INTEGER NOT NULL)")
	require.NoError(t, err)

	entry := config.ConfigEntry{Name: managerTxEntry, Type: config.ReadWrite}
	switch cfg := testDB.config.(type) {
	case v1.PostgresConfig:
		entry.Postgres = &cfg
	case v1.SQLiteConfig:
		entry.SQLite = &cfg
	case v1.MysqlConfig:
		entry.MySQL = &cfg
	case v1.MSSQLConfig:
		entry.MSSQL = &cfg
	default:
		t.Fatalf("unsupported integration config %T", testDB.config)
	}

	data, err := json.Marshal(config.ManagerConfig{Entries: []config.ConfigEntry{entry}})
	require.NoError(t, err)

	// NewDBManager only accepts config paths relative to the working directory.
	file, err := os.CreateTemp(".", "manager-tx-*.json")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(file.Name()) })
	_, err = file.Write(data)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	dm, err := manager.NewDBManager(ctx, file.Name(), nil)
	require.NoError(t, err)
	dm.Start()
	t.Cleanup(dm.Stop)

	return dm, raw
}

func managerTxRowExists(t *testing.T, raw v1.DB, id int) bool {
	t.Helper()
	rows, err := raw.Get(context.Background(), managerTxTable, []string{"id"}, nil,
		condition.NewExpr().Column("id").Op("=").Value(id), nil)
	require.NoError(t, err)
	return len(rows) > 0
}

func TestIntegration_ManagerWithTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	for _, testDB := range getFilteredDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			dm, raw := startIntegrationManager(t, testDB)
			ctx := context.Background()

			insert := func(tx v1.Tx, id int) error {
				_, err := tx.Insert(ctx, managerTxTable, map[string]any{"id": id, "balance": 10}, nil)
				return err
			}

			t.Run("commits when callback succeeds", func(t *testing.T) {
				err := dm.WithTransaction(ctx, managerTxEntry, func(tx v1.Tx) error { return insert(tx, 1) })
				require.NoError(t, err)
				assert.True(t, managerTxRowExists(t, raw, 1))
			})

			t.Run("rolls back when callback returns an error", func(t *testing.T) {
				sentinel := errors.New("abort")
				err := dm.WithTransaction(ctx, managerTxEntry, func(tx v1.Tx) error {
					if err := insert(tx, 2); err != nil {
						return err
					}
					return sentinel
				})
				require.ErrorIs(t, err, sentinel)
				assert.False(t, managerTxRowExists(t, raw, 2))
			})

			t.Run("rolls back and returns an error when callback panics", func(t *testing.T) {
				err := dm.WithTransaction(ctx, managerTxEntry, func(tx v1.Tx) error {
					if err := insert(tx, 3); err != nil {
						return err
					}
					panic("boom")
				})
				require.ErrorContains(t, err, "panicked")
				assert.False(t, managerTxRowExists(t, raw, 3))
			})
		})
	}
}

// TestIntegration_ManagerWithTransactionRowLock proves that a SELECT ... FOR UPDATE inside
// DBManager.WithTransaction serialises a concurrent read-modify-write on the same row:
// the second transaction blocks on the lock and then reads the first one's committed value.
//
//nolint:gocognit,cyclop
func TestIntegration_ManagerWithTransactionRowLock(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	const (
		lockSQL   = "SELECT balance FROM " + managerTxTable + " WHERE id = 1 FOR UPDATE"
		updateSQL = "UPDATE " + managerTxTable + " SET balance = balance + 1 WHERE id = 1"
	)

	for _, testDB := range getFilteredDatabases() {
		if testDB.driver != "postgres" {
			continue
		}
		t.Run(testDB.name, func(t *testing.T) {
			dm, raw := startIntegrationManager(t, testDB)
			ctx, cancel := context.WithTimeout(context.Background(), managerTxTimeout)
			defer cancel()

			_, err := raw.Exec(ctx, "INSERT INTO "+managerTxTable+" (id, balance) VALUES (1, 10)")
			require.NoError(t, err)

			locked := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseLock := func() { releaseOnce.Do(func() { close(release) }) }
			defer releaseLock()

			tx1Done := make(chan error, 1)
			go func() {
				tx1Done <- dm.WithTransaction(ctx, managerTxEntry, func(tx v1.Tx) error {
					if _, err := tx.Query(ctx, lockSQL); err != nil {
						return err
					}
					close(locked)
					<-release
					_, err := tx.Exec(ctx, updateSQL)
					return err
				})
			}()

			select {
			case <-locked:
			case err := <-tx1Done:
				t.Fatalf("first transaction ended before taking the lock: %v", err)
			case <-ctx.Done():
				t.Fatal("timed out waiting for the first transaction to take the lock")
			}

			var seenByTx2 string
			tx2Done := make(chan error, 1)
			go func() {
				tx2Done <- dm.WithTransaction(ctx, managerTxEntry, func(tx v1.Tx) error {
					rows, err := tx.Query(ctx, lockSQL)
					if err != nil {
						return err
					}
					if len(rows) != 1 {
						return fmt.Errorf("expected 1 locked row, got %d", len(rows))
					}
					seenByTx2 = fmt.Sprint(rows[0]["balance"])
					_, err = tx.Exec(ctx, updateSQL)
					return err
				})
			}()

			waitForPostgresLockWaiter(ctx, t, raw)
			select {
			case err := <-tx2Done:
				t.Fatalf("second transaction finished while the row lock was held: %v", err)
			default:
			}

			releaseLock()
			require.NoError(t, <-tx1Done)
			require.NoError(t, <-tx2Done)

			assert.Equal(t, "11", seenByTx2, "second transaction must read the first one's committed value")
			rows, err := raw.Query(ctx, "SELECT balance FROM "+managerTxTable+" WHERE id = 1")
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, "12", fmt.Sprint(rows[0]["balance"]))
		})
	}
}

// waitForPostgresLockWaiter blocks until some backend in the current database is waiting on a lock.
func waitForPostgresLockWaiter(ctx context.Context, t *testing.T, raw v1.DB) {
	t.Helper()
	const waitingSQL = "SELECT count(*) AS waiting FROM pg_stat_activity " +
		"WHERE datname = current_database() AND wait_event_type = 'Lock'"
	for {
		rows, err := raw.Query(ctx, waitingSQL)
		require.NoError(t, err)
		if len(rows) == 1 && fmt.Sprint(rows[0]["waiting"]) != "0" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for the second transaction to block on the row lock")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
