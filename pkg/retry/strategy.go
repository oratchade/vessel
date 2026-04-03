package retry

import (
	"math"
	"math/rand"
	"time"
)

// ExponentialBackoff implements an exponential backoff strategy.
// Delay = initialDelay * (baseMultiplier ^ attempt), capped at maxDelay
type ExponentialBackoff struct {
	initialDelay   time.Duration
	maxDelay       time.Duration
	baseMultiplier float64
	maxAttempts    int
	jitterFactor   float64 // 0.0 to 1.0, fraction of delay to randomize
	rng            *rand.Rand
}

// NewExponentialBackoff creates a new exponential backoff strategy.
// initialDelay: starting delay (e.g., 100ms)
// maxDelay: maximum delay between retries (e.g., 10s)
// baseMultiplier: multiplier for exponential growth (e.g., 2.0)
// maxAttempts: maximum number of retries (-1 for unlimited)
// jitterFactor: randomization factor 0.0-1.0 (e.g., 0.1 for ±10%)
func NewExponentialBackoff(
	initialDelay, maxDelay time.Duration,
	baseMultiplier float64,
	maxAttempts int,
	jitterFactor float64,
) *ExponentialBackoff {
	return &ExponentialBackoff{
		initialDelay:   initialDelay,
		maxDelay:       maxDelay,
		baseMultiplier: baseMultiplier,
		maxAttempts:    maxAttempts,
		jitterFactor:   jitterFactor,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // non-cryptographic jitter
	}
}

// NextDelay calculates the next delay using exponential backoff with jitter.
func (eb *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if eb.maxAttempts >= 0 && attempt >= eb.maxAttempts {
		return -1 // No more retries
	}

	// Calculate exponential delay: initialDelay * (baseMultiplier^attempt)
	exponentialDelay := float64(eb.initialDelay.Milliseconds()) * math.Pow(eb.baseMultiplier, float64(attempt))
	delay := time.Duration(exponentialDelay) * time.Millisecond

	// Cap at maxDelay
	if delay > eb.maxDelay {
		delay = eb.maxDelay
	}

	// Apply jitter
	if eb.jitterFactor > 0 {
		jitterAmount := time.Duration(float64(delay) * eb.jitterFactor)
		jitterRange := 2*jitterAmount.Milliseconds() + 1
		jitter := time.Duration(eb.rng.Int63n(jitterRange)-jitterAmount.Milliseconds()) * time.Millisecond
		delay += jitter
	}

	return delay
}

// LinearBackoff implements a linear backoff strategy.
// Delay = initialDelay + (increment * attempt), capped at maxDelay
type LinearBackoff struct {
	initialDelay time.Duration
	increment    time.Duration
	maxDelay     time.Duration
	maxAttempts  int
	jitterFactor float64
	rng          *rand.Rand
}

// NewLinearBackoff creates a new linear backoff strategy.
// initialDelay: starting delay (e.g., 100ms)
// increment: delay increase per attempt (e.g., 100ms)
// maxDelay: maximum delay between retries (e.g., 10s)
// maxAttempts: maximum number of retries (-1 for unlimited)
// jitterFactor: randomization factor 0.0-1.0
func NewLinearBackoff(
	initialDelay, increment, maxDelay time.Duration,
	maxAttempts int,
	jitterFactor float64,
) *LinearBackoff {
	return &LinearBackoff{
		initialDelay: initialDelay,
		increment:    increment,
		maxDelay:     maxDelay,
		maxAttempts:  maxAttempts,
		jitterFactor: jitterFactor,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // non-cryptographic jitter
	}
}

// NextDelay calculates the next delay using linear backoff with jitter.
func (lb *LinearBackoff) NextDelay(attempt int) time.Duration {
	if lb.maxAttempts >= 0 && attempt >= lb.maxAttempts {
		return -1 // No more retries
	}

	// Calculate linear delay: initialDelay + (increment * attempt)
	delay := lb.initialDelay + (time.Duration(attempt) * lb.increment)

	// Cap at maxDelay
	if delay > lb.maxDelay {
		delay = lb.maxDelay
	}

	// Apply jitter
	if lb.jitterFactor > 0 {
		jitterAmount := time.Duration(float64(delay) * lb.jitterFactor)
		jitterRange := 2*jitterAmount.Milliseconds() + 1
		jitter := time.Duration(lb.rng.Int63n(jitterRange)-jitterAmount.Milliseconds()) * time.Millisecond
		delay += jitter
	}

	return delay
}

// FixedBackoff implements a fixed delay strategy with jitter.
// Every attempt has the same delay (plus optional jitter).
type FixedBackoff struct {
	delay        time.Duration
	maxAttempts  int
	jitterFactor float64
	rng          *rand.Rand
}

// NewFixedBackoff creates a new fixed backoff strategy.
// delay: fixed delay between retries
// maxAttempts: maximum number of retries (-1 for unlimited)
// jitterFactor: randomization factor 0.0-1.0
func NewFixedBackoff(delay time.Duration, maxAttempts int, jitterFactor float64) *FixedBackoff {
	return &FixedBackoff{
		delay:        delay,
		maxAttempts:  maxAttempts,
		jitterFactor: jitterFactor,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // non-cryptographic jitter
	}
}

// NextDelay returns the fixed delay with optional jitter.
func (fb *FixedBackoff) NextDelay(attempt int) time.Duration {
	if fb.maxAttempts >= 0 && attempt >= fb.maxAttempts {
		return -1 // No more retries
	}

	delay := fb.delay

	// Apply jitter
	if fb.jitterFactor > 0 {
		jitterAmount := time.Duration(float64(delay) * fb.jitterFactor)
		jitterRange := 2*jitterAmount.Milliseconds() + 1
		jitter := time.Duration(fb.rng.Int63n(jitterRange)-jitterAmount.Milliseconds()) * time.Millisecond
		delay += jitter
	}

	return delay
}

// NoOpBackoff implements a strategy with no delay - fails immediately.
// Useful for testing or when you only want a single attempt.
type NoOpBackoff struct{}

// NewNoOpBackoff creates a strategy that never retries.
func NewNoOpBackoff() *NoOpBackoff {
	return &NoOpBackoff{}
}

// NextDelay always returns -1 (no retries).
func (nob *NoOpBackoff) NextDelay(attempt int) time.Duration {
	return -1
}
