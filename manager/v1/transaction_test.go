//go:build test

package v1

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
)

func newTxTestManager(state lifecycleState, readWrite, readOnly map[string]*DBEntry) *DBManager {
	dm := &DBManager{readWriteEntries: readWrite, readOnlyEntries: readOnly, logger: &noOpLogger{}}
	dm.lifecycle.store(state)
	return dm
}

func newTxTestEntry(name string, database db.DB, healthy bool) *DBEntry {
	de := &DBEntry{name: name, db: database, logger: &noOpLogger{}}
	de.healthy.Store(healthy)
	return de
}

func delegateToCallback(tx db.Tx) func(context.Context, func(db.Tx) error, ...db.TransactionOptions) error {
	return func(_ context.Context, fn func(db.Tx) error, _ ...db.TransactionOptions) error {
		return fn(tx)
	}
}

func TestWithTransactionRunsEveryStatementOnReadWriteEntryTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	primary := db.NewMockDB(ctrl)
	replica := db.NewMockDB(ctrl) // no expectations: any call fails the test
	tx := db.NewMockTx(ctrl)
	opts := db.TransactionOptions{Isolation: sql.LevelSerializable}

	primary.EXPECT().WithTransaction(gomock.Any(), gomock.Any(), opts).DoAndReturn(delegateToCallback(tx))
	tx.EXPECT().Query(gomock.Any(), "SELECT balance FROM accounts WHERE id = 1 FOR UPDATE").Return(nil, nil)
	tx.EXPECT().Exec(gomock.Any(), "UPDATE accounts SET balance = 0 WHERE id = 1").Return(&db.ExecResult{}, nil)

	dm := newTxTestManager(lifecycleStarted,
		map[string]*DBEntry{"primary": newTxTestEntry("primary", primary, true)},
		map[string]*DBEntry{"replica": newTxTestEntry("replica", replica, true)},
	)

	ctx := context.Background()
	err := dm.WithTransaction(ctx, "primary", func(tx db.Tx) error {
		if _, err := tx.Query(ctx, "SELECT balance FROM accounts WHERE id = 1 FOR UPDATE"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE accounts SET balance = 0 WHERE id = 1")
		return err
	}, opts)

	require.NoError(t, err)
}

func TestWithTransactionPropagatesCallbackError(t *testing.T) {
	ctrl := gomock.NewController(t)
	primary := db.NewMockDB(ctrl)
	tx := db.NewMockTx(ctrl)
	sentinel := errors.New("callback failed")

	primary.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).DoAndReturn(delegateToCallback(tx))

	dm := newTxTestManager(lifecycleStarted,
		map[string]*DBEntry{"primary": newTxTestEntry("primary", primary, true)}, nil)

	err := dm.WithTransaction(context.Background(), "primary", func(db.Tx) error { return sentinel })

	require.ErrorIs(t, err, sentinel)
}

func TestWithTransactionRejectsBeforeTouchingDatabase(t *testing.T) {
	noop := func(db.Tx) error { return nil }

	tests := []struct {
		name    string
		state   lifecycleState
		entry   string
		fn      func(db.Tx) error
		wantErr error
	}{
		{name: "not started", state: lifecycleCreated, entry: "primary", fn: noop, wantErr: ErrManagerNotStarted},
		{name: "stopped", state: lifecycleStopped, entry: "primary", fn: noop, wantErr: ErrManagerClosed},
		{name: "unknown entry", state: lifecycleStarted, entry: "missing", fn: noop, wantErr: ErrEntryNotFound},
		{name: "empty entry name", state: lifecycleStarted, entry: "", fn: noop, wantErr: ErrEntryNotFound},
		{name: "readonly entry", state: lifecycleStarted, entry: "replica", fn: noop, wantErr: ErrEntryReadOnly},
		{name: "unhealthy entry", state: lifecycleStarted, entry: "down", fn: noop, wantErr: ErrEntryUnhealthy},
		{name: "nil callback", state: lifecycleStarted, entry: "primary", fn: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// No expectations on any mock: reaching a database fails the test.
			dm := newTxTestManager(tt.state,
				map[string]*DBEntry{
					"primary": newTxTestEntry("primary", db.NewMockDB(ctrl), true),
					"down":    newTxTestEntry("down", db.NewMockDB(ctrl), false),
				},
				map[string]*DBEntry{"replica": newTxTestEntry("replica", db.NewMockDB(ctrl), true)},
			)

			err := dm.WithTransaction(context.Background(), tt.entry, tt.fn)

			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}
