package v1

import "sync/atomic"

type AtomicWrapCounter struct {
	value int64
	max   int64
}

func NewAtomicWrapCounter(max int64) *AtomicWrapCounter {
	if max <= 0 {
		panic("max must be > 0")
	}
	return &AtomicWrapCounter{max: max}
}

// Next increments and wraps around automatically.
func (c *AtomicWrapCounter) Next() int64 {
	newVal := atomic.AddInt64(&c.value, 1)
	return newVal % c.max
}

// Get returns the current value modulo max.
func (c *AtomicWrapCounter) Get() int64 {
	return atomic.LoadInt64(&c.value) % c.max
}

// Reset sets the counter back to zero.
func (c *AtomicWrapCounter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}
