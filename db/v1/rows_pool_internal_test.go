//go:build test

package v1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staleRowsProvider struct{}

func (staleRowsProvider) columns() ([]string, error) { return []string{"stale"}, nil }
func (staleRowsProvider) next() bool                 { return false }
func (staleRowsProvider) scan(dest ...any) error     { return nil }
func (staleRowsProvider) close() error               { return nil }
func (staleRowsProvider) err() error                 { return nil }

func TestRowsAdapterPoolAcquireFailureResetsAdapterBeforePooling(t *testing.T) {
	pool := NewRowsAdapterPool()
	stale := &RowsAdapter{provider: staleRowsProvider{}}
	pool.pool.Put(stale)

	adapter, err := pool.Acquire(struct{ invalid string }{invalid: "rows"})

	require.Error(t, err)
	assert.Nil(t, adapter)

	reused := pool.pool.Get().(*RowsAdapter) //nolint:forcetypeassert
	assert.Nil(t, reused.provider)
}

func TestRowsAdapterPoolAcquireFailureKeepsErrorContext(t *testing.T) {
	pool := NewRowsAdapterPool()

	adapter, err := pool.Acquire(fmt.Errorf("not rows"))

	require.Error(t, err)
	assert.Nil(t, adapter)
	assert.Contains(t, err.Error(), "RowsAdapterPool.Acquire")
	assert.Contains(t, err.Error(), "unsupported rows type")
}
