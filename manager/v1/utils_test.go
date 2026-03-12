//go:build test

package v1_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/manager/v1"
)

// TestNewAtomicWrapCounterPanic tests that New panics on invalid max value.
func TestNewAtomicWrapCounterPanic(t *testing.T) {
	// Test zero max panics
	assert.Panics(t, func() {
		v1.NewAtomicWrapCounter(0)
	}, "should panic with max=0")

	// Test negative max panics
	assert.Panics(t, func() {
		v1.NewAtomicWrapCounter(-1)
	}, "should panic with max=-1")
}

// TestNewAtomicWrapCounterValid tests valid counter creation.
func TestNewAtomicWrapCounterValid(t *testing.T) {
	counter := v1.NewAtomicWrapCounter(10)
	require.NotNil(t, counter)

	// Initial value should be 0
	val := counter.Get()
	assert.Equal(t, int64(0), val)
}

// TestAtomicWrapCounterNext tests wrap-around behavior.
func TestAtomicWrapCounterNext(t *testing.T) {
	counter := v1.NewAtomicWrapCounter(5)

	// Test sequence: should wrap at max
	expected := []int64{1, 2, 3, 4, 0, 1, 2}
	for _, exp := range expected {
		val := counter.Next()
		assert.Equal(t, exp, val)
	}
}

// TestAtomicWrapCounterGet returns current value without incrementing.
func TestAtomicWrapCounterGet(t *testing.T) {
	counter := v1.NewAtomicWrapCounter(10)

	// Get before any increments
	val1 := counter.Get()
	val2 := counter.Get()
	assert.Equal(t, val1, val2, "Get should not change value")

	// Get after Next
	counter.Next()
	val3 := counter.Get()
	assert.Equal(t, int64(1), val3)
}

// TestAtomicWrapCounterReset tests the Reset function.
func TestAtomicWrapCounterReset(t *testing.T) {
	counter := v1.NewAtomicWrapCounter(10)

	// Increment a few times
	counter.Next()
	counter.Next()
	counter.Next()

	// Verify values have changed
	val := counter.Get()
	assert.NotEqual(t, int64(0), val)

	// Reset and verify
	counter.Reset()
	resetVal := counter.Get()
	assert.Equal(t, int64(0), resetVal)
}

// TestAtomicWrapCounterConcurrency tests thread-safe behavior.
func TestAtomicWrapCounterConcurrency(t *testing.T) {
	counter := v1.NewAtomicWrapCounter(10)
	const goroutines = 50
	const increments = 100

	var wg sync.WaitGroup

	// Launch concurrent goroutines incrementing the counter
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				counter.Next()
			}
		}()
	}

	wg.Wait()

	// Total increments should be goroutines * increments
	// The value wraps so we check that something was incremented
	final := counter.Get()
	assert.True(t, final >= 0 && final < 10, "Final value should be valid wrapped value")
}

// TestAtomicWrapCounterWrapDistribution tests even distribution across wrap values.
func TestAtomicWrapCounterWrapDistribution(t *testing.T) {
	const max = 5
	counter := v1.NewAtomicWrapCounter(int64(max))

	distribution := make(map[int64]int)

	// Collect values and check distribution
	for i := 0; i < 1000; i++ {
		val := counter.Next()
		distribution[val]++
	}

	// Should have seen all values 0 to max-1
	for i := int64(0); i < int64(max); i++ {
		assert.Greater(t, distribution[i], 0, "Should see value %d", i)
	}

	// No values >= max should appear
	for i := int64(max); i < int64(max+5); i++ {
		assert.Equal(t, 0, distribution[i], "Should not see value %d", i)
	}
}
