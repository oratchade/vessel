package retry

// Package retry provides configurable retry logic with various backoff strategies.

import (
	"context"
	"fmt"
	"time"
)

// Strategy defines how to calculate the next retry delay.
type Strategy interface {
	// NextDelay returns the delay before the next retry attempt.
	// attempt is zero-indexed (0 for first retry, 1 for second, etc.).
	// Returns a negative duration to indicate no more retries should be attempted.
	NextDelay(attempt int) time.Duration
}

// Do executes fn with retries according to the provided strategy.
// It retries when fn returns an error.
// Returns the error if all retries are exhausted, or nil if successful.
func Do(ctx context.Context, strategy Strategy, fn func() error) error {
	attempt := 0
	for {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry: context done: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		// Get next delay
		delay := strategy.NextDelay(attempt)
		if delay < 0 {
			return fmt.Errorf("retry: exhausted after %d attempts: %w", attempt, err)
		}

		// Wait before retrying
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry: context done during delay: %w", ctx.Err())
		}

		attempt++
	}
}

// DoWithResult executes fn with retries and returns the result.
// It retries when fn returns an error.
// Returns the result and error; error is non-nil if all retries exhausted.
func DoWithResult[T any](ctx context.Context, strategy Strategy, fn func() (T, error)) (T, error) {
	var zero T
	attempt := 0
	for {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry: context done: %w", ctx.Err())
		default:
		}

		// Execute the function
		result, err := fn()
		if err == nil {
			return result, nil // Success
		}

		// Get next delay
		delay := strategy.NextDelay(attempt)
		if delay < 0 {
			return zero, fmt.Errorf("retry: exhausted after %d attempts: %w", attempt, err)
		}

		// Wait before retrying
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return zero, fmt.Errorf("retry: context done during delay: %w", ctx.Err())
		}

		attempt++
	}
}
