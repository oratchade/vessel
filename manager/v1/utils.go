package v1

import (
	"fmt"
	"sync/atomic"
)

type AtomicWrapCounter struct {
	value int64
	max   int64
}

// ErrInvalidCounterMax is returned when attempting to create an AtomicWrapCounter with invalid max value.
var ErrInvalidCounterMax = fmt.Errorf("invalid counter max: must be > 0")

// NewAtomicWrapCounter creates a new AtomicWrapCounter with the given max value.
// Returns an error if max is <= 0, as this would cause division by zero in Next() or Get().
func NewAtomicWrapCounter(max int64) (*AtomicWrapCounter, error) {
	if max <= 0 {
		return nil, fmt.Errorf("%w (got %d)", ErrInvalidCounterMax, max)
	}
	return &AtomicWrapCounter{max: max}, nil
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
