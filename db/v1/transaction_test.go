//go:build test

package v1_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/db/v1"
)

func TestRunTransactionSuccessCommits(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	tx := v1.NewMockTx(ctrl)
	tx.EXPECT().Commit(ctx).Return(nil)

	err := v1.ExportRunTransaction(ctx, "test.WithTransaction", tx, func(got v1.Tx) error {
		assert.Same(t, tx, got)
		return nil
	})

	require.NoError(t, err)
}

func TestRunTransactionCallbackErrorRollsBack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	tx := v1.NewMockTx(ctrl)
	callbackErr := errors.New("callback failed")
	tx.EXPECT().Rollback(ctx).Return(nil)

	err := v1.ExportRunTransaction(ctx, "test.WithTransaction", tx, func(v1.Tx) error {
		return callbackErr
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, callbackErr)
}

func TestRunTransactionCallbackErrorIncludesRollbackFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	tx := v1.NewMockTx(ctrl)
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")
	tx.EXPECT().Rollback(ctx).Return(rollbackErr)

	err := v1.ExportRunTransaction(ctx, "test.WithTransaction", tx, func(v1.Tx) error {
		return callbackErr
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, callbackErr)
	assert.ErrorIs(t, err, rollbackErr)
	assert.Contains(t, err.Error(), "rollback failed")
}

func TestRunTransactionCallbackPanicRollsBackAndReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	tx := v1.NewMockTx(ctrl)
	tx.EXPECT().Rollback(ctx).Return(nil)

	err := v1.ExportRunTransaction(ctx, "test.WithTransaction", tx, func(v1.Tx) error {
		panic("boom")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction callback panicked: boom")
	assert.Contains(t, err.Error(), "goroutine")
}

func TestRunTransactionCommitFailureReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	tx := v1.NewMockTx(ctrl)
	commitErr := errors.New("commit failed")
	tx.EXPECT().Commit(ctx).Return(commitErr)

	err := v1.ExportRunTransaction(ctx, "test.WithTransaction", tx, func(v1.Tx) error {
		return nil
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, commitErr)
	assert.Contains(t, err.Error(), "failed to commit transaction")
}

func TestSQLiteWithTransactionPanicRollsBackAndReturnsError(t *testing.T) {
	ctx := context.Background()
	database, err := v1.NewDB(&v1.SQLiteConfig{
		FilePath:     ":memory:",
		CacheMode:    "shared",
		Mode:         "memory",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}, NoOpLogger{})
	require.NoError(t, err)
	defer database.Close() //nolint:errcheck

	_, err = database.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	err = database.WithTransaction(ctx, func(tx v1.Tx) error {
		_, execErr := tx.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Ada")
		require.NoError(t, execErr)
		panic("rollback me")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction callback panicked: rollback me")

	rows, err := database.Query(ctx, "SELECT name FROM users")
	require.NoError(t, err)
	assert.Empty(t, rows)
}
