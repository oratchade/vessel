//go:build test

package retry_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"tounilab.com/fabric/pkg/retry"
)

// ExponentialBackoff tests

func TestExponentialBackoff_BasicProgression(t *testing.T) {
	eb := retry.NewExponentialBackoff(100*time.Millisecond, 10*time.Second, 2.0, -1, 0.0)

	// Verify sequence: 100ms, 200ms, 400ms, 800ms
	assert.Equal(t, 100*time.Millisecond, eb.NextDelay(0))
	assert.Equal(t, 200*time.Millisecond, eb.NextDelay(1))
	assert.Equal(t, 400*time.Millisecond, eb.NextDelay(2))
	assert.Equal(t, 800*time.Millisecond, eb.NextDelay(3))
}

func TestExponentialBackoff_CappedByMaxDelay(t *testing.T) {
	eb := retry.NewExponentialBackoff(100*time.Millisecond, 1*time.Second, 2.0, -1, 0.0)

	// Attempt 4 would be 1600ms, but capped at 1000ms
	assert.Equal(t, 1*time.Second, eb.NextDelay(4))
	assert.Equal(t, 1*time.Second, eb.NextDelay(10))
}

func TestExponentialBackoff_MaxAttempts(t *testing.T) {
	eb := retry.NewExponentialBackoff(100*time.Millisecond, 10*time.Second, 2.0, 3, 0.0)

	assert.Equal(t, 100*time.Millisecond, eb.NextDelay(0))
	assert.Equal(t, 200*time.Millisecond, eb.NextDelay(1))
	assert.Equal(t, 400*time.Millisecond, eb.NextDelay(2))

	// Should be exhausted at attempt 3
	assert.Equal(t, time.Duration(-1), eb.NextDelay(3))
}

// LinearBackoff tests

func TestLinearBackoff_BasicProgression(t *testing.T) {
	lb := retry.NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond, 10*time.Second, -1, 0.0)

	// 100ms, 150ms, 200ms, 250ms
	assert.Equal(t, 100*time.Millisecond, lb.NextDelay(0))
	assert.Equal(t, 150*time.Millisecond, lb.NextDelay(1))
	assert.Equal(t, 200*time.Millisecond, lb.NextDelay(2))
	assert.Equal(t, 250*time.Millisecond, lb.NextDelay(3))
}

func TestLinearBackoff_CappedByMaxDelay(t *testing.T) {
	lb := retry.NewLinearBackoff(100*time.Millisecond, 100*time.Millisecond, 500*time.Millisecond, -1, 0.0)

	assert.Equal(t, 100*time.Millisecond, lb.NextDelay(0))
	assert.Equal(t, 300*time.Millisecond, lb.NextDelay(2))
	assert.Equal(t, 500*time.Millisecond, lb.NextDelay(4))
	assert.Equal(t, 500*time.Millisecond, lb.NextDelay(5)) // Capped
}

func TestLinearBackoff_MaxAttempts(t *testing.T) {
	lb := retry.NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond, 10*time.Second, 4, 0.0)

	assert.Equal(t, 100*time.Millisecond, lb.NextDelay(0))
	assert.Equal(t, 150*time.Millisecond, lb.NextDelay(1))
	assert.Equal(t, 200*time.Millisecond, lb.NextDelay(2))
	assert.Equal(t, 250*time.Millisecond, lb.NextDelay(3))

	// Exhausted at attempt 4
	assert.Equal(t, time.Duration(-1), lb.NextDelay(4))
}

// FixedBackoff tests

func TestFixedBackoff_ConstantDelay(t *testing.T) {
	fb := retry.NewFixedBackoff(200*time.Millisecond, -1, 0.0)

	// All attempts should return the same delay
	for i := 0; i < 10; i++ {
		assert.Equal(t, 200*time.Millisecond, fb.NextDelay(i))
	}
}

func TestFixedBackoff_MaxAttempts(t *testing.T) {
	fb := retry.NewFixedBackoff(200*time.Millisecond, 3, 0.0)

	assert.Equal(t, 200*time.Millisecond, fb.NextDelay(0))
	assert.Equal(t, 200*time.Millisecond, fb.NextDelay(1))
	assert.Equal(t, 200*time.Millisecond, fb.NextDelay(2))

	// Exhausted at attempt 3
	assert.Equal(t, time.Duration(-1), fb.NextDelay(3))
}

// NoOpBackoff tests

func TestNoOpBackoff_NoRetries(t *testing.T) {
	nob := retry.NewNoOpBackoff()

	assert.Equal(t, time.Duration(-1), nob.NextDelay(0))
	assert.Equal(t, time.Duration(-1), nob.NextDelay(1))
	assert.Equal(t, time.Duration(-1), nob.NextDelay(100))
}

// Jitter tests

func TestExponentialBackoff_WithJitter(t *testing.T) {
	eb := retry.NewExponentialBackoff(100*time.Millisecond, 10*time.Second, 2.0, -1, 0.2)

	baseDelay := 200 * time.Millisecond
	minExpected := time.Duration(float64(baseDelay) * 0.8)
	maxExpected := time.Duration(float64(baseDelay) * 1.2)

	for i := 0; i < 5; i++ {
		delay := eb.NextDelay(1)
		assert.True(t, delay >= minExpected && delay <= maxExpected,
			"Delay %v should be between %v and %v", delay, minExpected, maxExpected)
	}
}

func TestLinearBackoff_WithJitter(t *testing.T) {
	lb := retry.NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond, 10*time.Second, -1, 0.1)

	baseDelay := 200 * time.Millisecond
	minExpected := time.Duration(float64(baseDelay) * 0.9)
	maxExpected := time.Duration(float64(baseDelay) * 1.1)

	for i := 0; i < 5; i++ {
		delay := lb.NextDelay(2)
		assert.True(t, delay >= minExpected && delay <= maxExpected,
			"Delay %v should be between %v and %v", delay, minExpected, maxExpected)
	}
}

func TestFixedBackoff_WithJitter(t *testing.T) {
	fb := retry.NewFixedBackoff(200*time.Millisecond, -1, 0.15)

	baseDelay := 200 * time.Millisecond
	// Add a small tolerance for rounding
	minExpected := time.Duration(float64(baseDelay) * 0.84)
	maxExpected := time.Duration(float64(baseDelay) * 1.16)

	hasVariance := false
	previousDelay := time.Duration(0)

	for i := 0; i < 10; i++ {
		delay := fb.NextDelay(0)
		assert.True(t, delay >= minExpected && delay <= maxExpected,
			"Delay %v should be between %v and %v", delay, minExpected, maxExpected)

		if i > 0 && delay != previousDelay {
			hasVariance = true
		}
		previousDelay = delay
	}

	assert.True(t, hasVariance, "Expected some variance with jitter")
}

// Benchmark tests

func BenchmarkExponentialBackoff(b *testing.B) {
	eb := retry.NewExponentialBackoff(100*time.Millisecond, 30*time.Second, 2.0, -1, 0.1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eb.NextDelay(i % 100)
	}
}

func BenchmarkLinearBackoff(b *testing.B) {
	lb := retry.NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond, 10*time.Second, -1, 0.1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.NextDelay(i % 100)
	}
}

func BenchmarkFixedBackoff(b *testing.B) {
	fb := retry.NewFixedBackoff(200*time.Millisecond, -1, 0.1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.NextDelay(i % 100)
	}
}

// Do() function tests

func TestDo_Success(t *testing.T) {
	calls := 0
	strategy := retry.NewFixedBackoff(1*time.Millisecond, -1, 0.0)

	err := retry.Do(context.Background(), strategy, func() error {
		calls++
		return nil // Success on first attempt
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	calls := 0
	strategy := retry.NewFixedBackoff(1*time.Millisecond, -1, 0.0)

	err := retry.Do(context.Background(), strategy, func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("temporary error")
		}
		return nil // Success on third attempt
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestDo_ExhaustedRetries(t *testing.T) {
	calls := 0
	strategy := retry.NewFixedBackoff(1*time.Millisecond, 2, 0.0)

	err := retry.Do(context.Background(), strategy, func() error {
		calls++
		return fmt.Errorf("always fails")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, calls) // Called 3 times (attempts 0, 1, 2)
	assert.Contains(t, err.Error(), "exhausted after 2 attempts")
}

func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	strategy := retry.NewFixedBackoff(100*time.Millisecond, -1, 0.0)

	cancel() // Cancel immediately

	err := retry.Do(ctx, strategy, func() error {
		return fmt.Errorf("should not be called")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context done")
}

func TestDo_ContextCanceledDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	strategy := retry.NewFixedBackoff(500*time.Millisecond, -1, 0.0)

	calls := 0
	go func() {
		time.Sleep(50 * time.Millisecond) // Cancel during delay
		cancel()
	}()

	err := retry.Do(ctx, strategy, func() error {
		calls++
		return fmt.Errorf("always fails")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.Contains(t, err.Error(), "context done during delay")
}

// DoWithResult() function tests

func TestDoWithResult_Success(t *testing.T) {
	calls := 0
	strategy := retry.NewFixedBackoff(1*time.Millisecond, -1, 0.0)

	result, err := retry.DoWithResult(context.Background(), strategy, func() (string, error) {
		calls++
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 1, calls)
}

func TestDoWithResult_SuccessAfterRetries(t *testing.T) {
	calls := 0
	strategy := retry.NewFixedBackoff(1*time.Millisecond, -1, 0.0)

	result, err := retry.DoWithResult(context.Background(), strategy, func() (string, error) {
		calls++
		if calls < 3 {
			return "", fmt.Errorf("temporary error")
		}
		return "final result", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "final result", result)
	assert.Equal(t, 3, calls)
}

func TestDoWithResult_ExhaustedRetries(t *testing.T) {
	calls := 0
	strategy := retry.NewFixedBackoff(1*time.Millisecond, 2, 0.0)

	result, err := retry.DoWithResult(context.Background(), strategy, func() (int, error) {
		calls++
		return 0, fmt.Errorf("always fails")
	})

	assert.Error(t, err)
	assert.Equal(t, 0, result)
	assert.Equal(t, 3, calls) // Called 3 times (attempts 0, 1, 2)
	assert.Contains(t, err.Error(), "exhausted after 2 attempts")
}

func TestDoWithResult_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	strategy := retry.NewFixedBackoff(100*time.Millisecond, -1, 0.0)

	cancel() // Cancel immediately

	result, err := retry.DoWithResult(ctx, strategy, func() (string, error) {
		return "", fmt.Errorf("should not be called")
	})

	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Contains(t, err.Error(), "context done")
}

func TestDoWithResult_ContextCanceledDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	strategy := retry.NewFixedBackoff(500*time.Millisecond, -1, 0.0)

	calls := 0
	go func() {
		time.Sleep(50 * time.Millisecond) // Cancel during delay
		cancel()
	}()

	result, err := retry.DoWithResult(ctx, strategy, func() (string, error) {
		calls++
		return "", fmt.Errorf("always fails")
	})

	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Equal(t, 1, calls)
	assert.Contains(t, err.Error(), "context done during delay")
}

// Integration tests

func TestIntegration_ExponentialBackoffWithDo(t *testing.T) {
	attempts := []time.Duration{}
	startTime := time.Now()
	strategy := retry.NewExponentialBackoff(10*time.Millisecond, 100*time.Millisecond, 2.0, 4, 0.0)

	err := retry.Do(context.Background(), strategy, func() error {
		attempts = append(attempts, time.Since(startTime))
		if len(attempts) < 4 {
			return fmt.Errorf("retry")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 4, len(attempts))

	// Verify timing is increasing (roughly)
	assert.True(t, attempts[1] > attempts[0])
	assert.True(t, attempts[2] > attempts[1])
	assert.True(t, attempts[3] > attempts[2])
}

func TestIntegration_LinearBackoffWithDoWithResult(t *testing.T) {
	type TestResult struct {
		Value   string
		Attempt int
	}

	attempt := 0
	strategy := retry.NewLinearBackoff(5*time.Millisecond, 5*time.Millisecond, 50*time.Millisecond, -1, 0.0)

	result, err := retry.DoWithResult(context.Background(), strategy, func() (TestResult, error) {
		attempt++
		if attempt < 3 {
			return TestResult{}, fmt.Errorf("not ready")
		}
		return TestResult{Value: "ready", Attempt: attempt}, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "ready", result.Value)
	assert.Equal(t, 3, result.Attempt)
	assert.Equal(t, 3, attempt)
}

func TestIntegration_TimeoutContext(t *testing.T) {
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	strategy := retry.NewFixedBackoff(30*time.Millisecond, -1, 0.0)

	err := retry.Do(ctx, strategy, func() error {
		calls++
		return fmt.Errorf("always fails")
	})

	assert.Error(t, err)
	// Should get at least 1 call, possibly 2 depending on timing
	assert.True(t, calls >= 1 && calls <= 3)
	assert.Contains(t, err.Error(), "context")
}

// Edge case tests

func TestEdgeCase_ZeroMaxAttempts(t *testing.T) {
	strategy := retry.NewFixedBackoff(1*time.Millisecond, 0, 0.0)
	calls := 0

	err := retry.Do(context.Background(), strategy, func() error {
		calls++
		return fmt.Errorf("error")
	})

	assert.Error(t, err)
	// With maxAttempts=0, NextDelay(0) returns -1 immediately
	assert.Equal(t, 1, calls) // Still called once (first attempt)
	assert.Contains(t, err.Error(), "exhausted")
}

func TestEdgeCase_LargeJitterFactor(t *testing.T) {
	// Jitter factor > 1.0 can produce very large variance
	strategy := retry.NewExponentialBackoff(100*time.Millisecond, 1*time.Second, 2.0, -1, 0.5)

	// Should still return valid delays
	for i := 0; i < 5; i++ {
		delay := strategy.NextDelay(i)
		// With reasonable jitter, we should get positive delays or -1
		assert.True(t, delay >= 0 || delay == -1, "Delay %v should be non-negative or -1", delay)
	}
}
