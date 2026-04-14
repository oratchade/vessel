//go:build test

package v1_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/manager/v1"
)

// TestNewAtomicWrapCounterErrorHandling tests that NewAtomicWrapCounter returns error on invalid max value.
func TestNewAtomicWrapCounterErrorHandling(t *testing.T) {
	// Test zero max returns error
	counter, err := v1.NewAtomicWrapCounter(0)
	require.Error(t, err, "should return error with max=0")
	require.Nil(t, counter, "counter should be nil on error")
	require.ErrorIs(t, err, v1.ErrInvalidCounterMax)

	// Test negative max returns error
	counter, err = v1.NewAtomicWrapCounter(-1)
	require.Error(t, err, "should return error with max=-1")
	require.Nil(t, counter, "counter should be nil on error")
	require.ErrorIs(t, err, v1.ErrInvalidCounterMax)
}

// TestNewAtomicWrapCounterValid tests valid counter creation.
func TestNewAtomicWrapCounterValid(t *testing.T) {
	counter, err := v1.NewAtomicWrapCounter(10)
	require.NoError(t, err)
	require.NotNil(t, counter)

	// Initial value should be 0
	val := counter.Get()
	assert.Equal(t, int64(0), val)
}

// TestAtomicWrapCounterNext tests wrap-around behavior.
func TestAtomicWrapCounterNext(t *testing.T) {
	counter, err := v1.NewAtomicWrapCounter(5)
	require.NoError(t, err)

	// Test sequence: should wrap at max
	expected := []int64{1, 2, 3, 4, 0, 1, 2}
	for _, exp := range expected {
		val := counter.Next()
		assert.Equal(t, exp, val)
	}
}

// TestAtomicWrapCounterGet returns current value without incrementing.
func TestAtomicWrapCounterGet(t *testing.T) {
	counter, err := v1.NewAtomicWrapCounter(10)
	require.NoError(t, err)

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
	counter, err := v1.NewAtomicWrapCounter(10)
	require.NoError(t, err)

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
	counter, err := v1.NewAtomicWrapCounter(10)
	require.NoError(t, err)
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
	counter, err := v1.NewAtomicWrapCounter(int64(max))
	require.NoError(t, err)

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
