package v1

import (
	"fmt"
	"runtime"
	"sync"
)

// RowsAdapterPoolStats provides statistics about RowsAdapter pool usage.
type RowsAdapterPoolStats struct {
	Allocated int
	Available int
}

// RowsAdapterPool manages a pool of reusable RowsAdapter instances to reduce allocation pressure.
//
// This pool uses sync.Pool internally to efficiently recycle RowsAdapter objects,
// which is particularly useful in high-throughput database query scenarios.
//
// Usage:
//
//	pool := NewRowsAdapterPool()
//	adapter, err := pool.Acquire(rows)
//	if err != nil { ... }
//	defer pool.Release(adapter)
//
// The pool automatically resets adapters when they are released, making them ready for reuse.
// If you prefer automatic cleanup, use ManagedRowsAdapter instead.
type RowsAdapterPool struct {
	pool *sync.Pool
	mu   sync.Mutex
	// Stats tracking (optional, enables Stats() calls)
	statsEnabled bool
	allocated    int
	released     int
}

// NewRowsAdapterPool creates a new pool for managing RowsAdapter instances.
func NewRowsAdapterPool() *RowsAdapterPool {
	return &RowsAdapterPool{
		pool: &sync.Pool{
			New: func() any {
				return &RowsAdapter{}
			},
		},
		statsEnabled: false,
	}
}

// NewRowsAdapterPoolWithStats creates a new pool with statistics tracking enabled.
// Note: Enabling stats has minimal performance overhead due to atomic operations.
func NewRowsAdapterPoolWithStats() *RowsAdapterPool {
	return &RowsAdapterPool{
		pool: &sync.Pool{
			New: func() any {
				return &RowsAdapter{}
			},
		},
		statsEnabled: true,
	}
}

// Acquire retrieves a RowsAdapter from the pool and initializes it with the provided rows.
// If no pooled adapter is available, a new one is created.
func (rap *RowsAdapterPool) Acquire(rows any) (*RowsAdapter, error) {
	ra := rap.pool.Get().(*RowsAdapter) //nolint:forcetypeassert
	if rap.statsEnabled {
		rap.mu.Lock()
		rap.allocated++
		rap.mu.Unlock()
	}

	// Initialize the adapter with the provided rows
	if err := ra.initRowsAdapterSource(rows); err != nil {
		ra.reset()
		// Return to pool before returning error
		rap.pool.Put(ra)
		return nil, fmt.Errorf("RowsAdapterPool.Acquire: %w", err)
	}

	return ra, nil
}

// Release returns a RowsAdapter to the pool after resetting its state.
// The adapter MUST be fully consumed and closed before calling Release.
// The pool does NOT call Close() - that's the caller's responsibility.
func (rap *RowsAdapterPool) Release(adapter *RowsAdapter) {
	if adapter == nil {
		return
	}

	if rap.statsEnabled {
		rap.mu.Lock()
		rap.released++
		rap.mu.Unlock()
	}

	// Reset the adapter state for reuse
	adapter.reset()

	// Return to pool
	rap.pool.Put(adapter)
}

// Stats returns current pool statistics if stats tracking is enabled.
// Returns (0, 0) if stats tracking was not enabled at pool creation time.
func (rap *RowsAdapterPool) Stats() RowsAdapterPoolStats {
	if !rap.statsEnabled {
		return RowsAdapterPoolStats{}
	}

	rap.mu.Lock()
	defer rap.mu.Unlock()

	// Estimate available in pool (imprecise due to sync.Pool internals)
	available := rap.released - (rap.allocated - rap.released)
	if available < 0 {
		available = 0
	}

	return RowsAdapterPoolStats{
		Allocated: rap.allocated,
		Available: available,
	}
}

// ManagedRowsAdapter is a RowsAdapter wrapper that automatically closes resources.
//
// This type is useful when you want to guarantee cleanup without explicit defer calls.
// It implements a pattern similar to Go's sql.Row vs sql.Rows.
//
// Usage:
//
//	managed, err := WrapManagedRowsAdapter(rows)
//	if err != nil { ... }
//
//	// Automatically closes when managed goes out of scope (via finalizer)
//	// Or explicitly:
//	managed.Close()
//
// WARNING: ManagedRowsAdapter relies on finalizers for cleanup in the absence of explicit Close().
// Finalizers are not guaranteed to run immediately, so explicit Close() is still recommended.
type ManagedRowsAdapter struct {
	adapter *RowsAdapter
	closed  bool
	mu      sync.Mutex
}

// WrapManagedRowsAdapter wraps a RowsAdapter in a ManagedRowsAdapter for automatic cleanup.
func WrapManagedRowsAdapter(rows any) (*ManagedRowsAdapter, error) {
	adapter, err := newRowsAdapter(rows)
	if err != nil {
		return nil, err
	}

	mra := &ManagedRowsAdapter{
		adapter: adapter,
		closed:  false,
	}

	// Set a finalizer to ensure cleanup if Close() is not explicitly called
	runtime.SetFinalizer(mra, (*ManagedRowsAdapter).Close)

	return mra, nil
}

// Adapter returns the underlying RowsAdapter for use in iteration and scanning.
// The returned adapter must not be modified or closed directly.
func (mra *ManagedRowsAdapter) Adapter() *RowsAdapter {
	mra.mu.Lock()
	defer mra.mu.Unlock()

	if mra.closed {
		return nil
	}

	return mra.adapter
}

// Close closes the underlying RowsAdapter and marks this wrapper as closed.
// Subsequent calls to Close() are safe (idempotent).
func (mra *ManagedRowsAdapter) Close() error {
	mra.mu.Lock()
	defer mra.mu.Unlock()

	if mra.closed || mra.adapter == nil {
		return nil
	}

	mra.closed = true
	err := mra.adapter.close()
	if err != nil {
		return fmt.Errorf("ManagedRowsAdapter.Close: %w", err)
	}

	// Cancel the finalizer since we've explicitly closed
	runtime.SetFinalizer(mra, nil)

	return nil
}

// IsClosed returns whether this ManagedRowsAdapter has been closed.
func (mra *ManagedRowsAdapter) IsClosed() bool {
	mra.mu.Lock()
	defer mra.mu.Unlock()

	return mra.closed
}
