//go:build test

package v1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
	v1 "tounilab.com/vessel/manager/v1"
	"tounilab.com/vessel/pkg/retry"
)

func retryTestConfig() *v1.QueryWithRetryConfig {
	return &v1.QueryWithRetryConfig{
		Strategy: retry.NewFixedBackoff(time.Millisecond, 2, 0),
	}
}

// TestQueryWithRetrySuccess ensures a successful query returns a nil error.
func TestQueryWithRetrySuccess(t *testing.T) {
	dm := &v1.DBManager{}

	rows, err := dm.QueryWithRetry(context.Background(), retryTestConfig(),
		func(_ context.Context, _ int) ([]map[string]any, error) {
			return []map[string]any{{"id": 1}}, nil
		})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0]["id"])
}

// TestQueryWithRetryFailure ensures an exhausted retry returns the wrapped error
// even when no logger is configured.
func TestQueryWithRetryFailure(t *testing.T) {
	dm := &v1.DBManager{}
	sentinel := errors.New("boom")

	_, err := dm.QueryWithRetry(context.Background(), retryTestConfig(),
		func(_ context.Context, _ int) ([]map[string]any, error) {
			return nil, sentinel
		})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "query with retry failed")
}

// TestQueryWithRetryRecovers ensures a failure followed by a success returns nil error.
func TestQueryWithRetryRecovers(t *testing.T) {
	dm := &v1.DBManager{}
	calls := 0

	rows, err := dm.QueryWithRetry(context.Background(), retryTestConfig(),
		func(_ context.Context, _ int) ([]map[string]any, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("transient")
			}
			return []map[string]any{{"ok": true}}, nil
		})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, calls)
}

// TestQueryWithRetryPassesAttemptNumbers ensures the callback receives the
// 1-indexed attempt number on every invocation.
func TestQueryWithRetryPassesAttemptNumbers(t *testing.T) {
	dm := &v1.DBManager{}
	var attempts []int

	_, err := dm.QueryWithRetry(context.Background(), retryTestConfig(),
		func(_ context.Context, attempt int) ([]map[string]any, error) {
			attempts = append(attempts, attempt)
			return nil, errors.New("always fails")
		})

	require.Error(t, err)
	assert.Equal(t, []int{1, 2, 3}, attempts)
}

// TestExecWithRetrySuccess ensures a successful exec returns a nil error.
func TestExecWithRetrySuccess(t *testing.T) {
	dm := &v1.DBManager{}

	result, err := dm.ExecWithRetry(context.Background(), retryTestConfig(),
		func(_ context.Context) (*db.ExecResult, error) {
			return &db.ExecResult{RowsAffected: 3}, nil
		})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(3), result.RowsAffected)
}

// TestExecWithRetryFailure ensures an exhausted retry returns the wrapped error.
func TestExecWithRetryFailure(t *testing.T) {
	dm := &v1.DBManager{}
	sentinel := errors.New("boom")

	_, err := dm.ExecWithRetry(context.Background(), retryTestConfig(),
		func(_ context.Context) (*db.ExecResult, error) {
			return nil, sentinel
		})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "exec with retry failed")
}
