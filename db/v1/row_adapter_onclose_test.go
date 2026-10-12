//go:build test

package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRowsAdapterOnCloseRunsOnceAfterClose(t *testing.T) {
	calls := 0
	ra := &RowsAdapter{}
	ra.OnClose(func() { calls++ })
	assert.Equal(t, 0, calls)
	assert.NoError(t, ra.Close())
	assert.NoError(t, ra.Close())
	assert.Equal(t, 1, calls)
}

// A pooled adapter must not carry the previous query's hook into the next.
func TestRowsAdapterResetDropsOnClose(t *testing.T) {
	calls := 0
	ra := &RowsAdapter{}
	ra.OnClose(func() { calls++ })
	ra.reset()
	assert.NoError(t, ra.Close())
	assert.Equal(t, 0, calls)
}

// The manager registers its own cleanup on managed rows; a caller's hook must not
// replace it, or the query's context stays retained until shutdown.
func TestRowsAdapterOnCloseHooksCompose(t *testing.T) {
	var order []string
	ra := &RowsAdapter{}
	ra.OnClose(func() { order = append(order, "manager") })
	ra.OnClose(func() { order = append(order, "caller") })
	assert.NoError(t, ra.Close())
	assert.NoError(t, ra.Close())
	assert.Equal(t, []string{"caller", "manager"}, order)
}
