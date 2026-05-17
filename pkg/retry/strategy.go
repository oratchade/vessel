package retry

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
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
	mu             sync.Mutex
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
	initialDelay = normalizeDuration(initialDelay)
	maxDelay = normalizeMaxDelay(initialDelay, maxDelay)
	baseMultiplier = normalizeMultiplier(baseMultiplier)
	maxAttempts = normalizeMaxAttempts(maxAttempts)
	jitterFactor = normalizeJitter(jitterFactor)
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

	delay = eb.applyJitter(delay)

	return delay
}

// Validate checks that the strategy is configured with normalized, safe values.
func (eb *ExponentialBackoff) Validate() error {
	return validateBackoffConfig(eb.initialDelay, eb.maxDelay, eb.maxAttempts, eb.jitterFactor, eb.baseMultiplier)
}

func (eb *ExponentialBackoff) applyJitter(delay time.Duration) time.Duration {
	if eb.jitterFactor <= 0 || delay <= 0 {
		return delay
	}
	jitterAmount := time.Duration(float64(delay) * eb.jitterFactor)
	if jitterAmount <= 0 {
		return delay
	}
	jitterRange := int64(2*jitterAmount + 1)
	eb.mu.Lock()
	jitter := time.Duration(eb.rng.Int63n(jitterRange) - int64(jitterAmount))
	eb.mu.Unlock()
	return delay + jitter
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
	mu           sync.Mutex
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
	initialDelay = normalizeDuration(initialDelay)
	increment = normalizeDuration(increment)
	maxDelay = normalizeMaxDelay(initialDelay, maxDelay)
	maxAttempts = normalizeMaxAttempts(maxAttempts)
	jitterFactor = normalizeJitter(jitterFactor)
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

	delay = lb.applyJitter(delay)

	return delay
}

// Validate checks that the strategy is configured with normalized, safe values.
func (lb *LinearBackoff) Validate() error {
	return validateBackoffConfig(lb.initialDelay, lb.maxDelay, lb.maxAttempts, lb.jitterFactor, 1)
}

func (lb *LinearBackoff) applyJitter(delay time.Duration) time.Duration {
	if lb.jitterFactor <= 0 || delay <= 0 {
		return delay
	}
	jitterAmount := time.Duration(float64(delay) * lb.jitterFactor)
	if jitterAmount <= 0 {
		return delay
	}
	jitterRange := int64(2*jitterAmount + 1)
	lb.mu.Lock()
	jitter := time.Duration(lb.rng.Int63n(jitterRange) - int64(jitterAmount))
	lb.mu.Unlock()
	return delay + jitter
}

// FixedBackoff implements a fixed delay strategy with jitter.
// Every attempt has the same delay (plus optional jitter).
type FixedBackoff struct {
	delay        time.Duration
	maxAttempts  int
	jitterFactor float64
	rng          *rand.Rand
	mu           sync.Mutex
}

// NewFixedBackoff creates a new fixed backoff strategy.
// delay: fixed delay between retries
// maxAttempts: maximum number of retries (-1 for unlimited)
// jitterFactor: randomization factor 0.0-1.0
func NewFixedBackoff(delay time.Duration, maxAttempts int, jitterFactor float64) *FixedBackoff {
	delay = normalizeDuration(delay)
	maxAttempts = normalizeMaxAttempts(maxAttempts)
	jitterFactor = normalizeJitter(jitterFactor)
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

	delay = fb.applyJitter(delay)

	return delay
}

// Validate checks that the strategy is configured with normalized, safe values.
func (fb *FixedBackoff) Validate() error {
	return validateBackoffConfig(fb.delay, fb.delay, fb.maxAttempts, fb.jitterFactor, 1)
}

func (fb *FixedBackoff) applyJitter(delay time.Duration) time.Duration {
	if fb.jitterFactor <= 0 || delay <= 0 {
		return delay
	}
	jitterAmount := time.Duration(float64(delay) * fb.jitterFactor)
	if jitterAmount <= 0 {
		return delay
	}
	jitterRange := int64(2*jitterAmount + 1)
	fb.mu.Lock()
	jitter := time.Duration(fb.rng.Int63n(jitterRange) - int64(jitterAmount))
	fb.mu.Unlock()
	return delay + jitter
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

func normalizeDuration(delay time.Duration) time.Duration {
	if delay < 0 {
		return 0
	}
	return delay
}

func normalizeMaxDelay(initialDelay, maxDelay time.Duration) time.Duration {
	maxDelay = normalizeDuration(maxDelay)
	if maxDelay < initialDelay {
		return initialDelay
	}
	return maxDelay
}

func normalizeMultiplier(multiplier float64) float64 {
	if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		return 1
	}
	return multiplier
}

func normalizeMaxAttempts(maxAttempts int) int {
	if maxAttempts < -1 {
		return -1
	}
	return maxAttempts
}

func normalizeJitter(jitterFactor float64) float64 {
	if jitterFactor < 0 || math.IsNaN(jitterFactor) {
		return 0
	}
	if jitterFactor > 1 || math.IsInf(jitterFactor, 0) {
		return 1
	}
	return jitterFactor
}

func validateBackoffConfig(
	initialDelay time.Duration,
	maxDelay time.Duration,
	maxAttempts int,
	jitterFactor float64,
	baseMultiplier float64,
) error {
	if initialDelay < 0 {
		return fmt.Errorf("initial delay cannot be negative")
	}
	if maxDelay < initialDelay {
		return fmt.Errorf("max delay cannot be less than initial delay")
	}
	if maxAttempts < -1 {
		return fmt.Errorf("max attempts must be -1 or greater")
	}
	if jitterFactor < 0 || jitterFactor > 1 || math.IsNaN(jitterFactor) {
		return fmt.Errorf("jitter factor must be between 0 and 1")
	}
	if baseMultiplier <= 0 || math.IsNaN(baseMultiplier) || math.IsInf(baseMultiplier, 0) {
		return fmt.Errorf("base multiplier must be positive and finite")
	}
	return nil
}
