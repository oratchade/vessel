package examples

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tounilab.com/vessel/pkg/retry"
)

// Example: Query Retry with Replica Failover
//
// This example demonstrates how to use the retry package to automatically
// try the next healthy replica when a query fails on the current replica.

// Replica represents a database replica with health status
type Replica struct {
	ID      string
	Host    string
	Port    int
	Healthy bool
	LastErr error
	mu      sync.RWMutex
}

// ReplicaPool manages a set of replicas and tracks their health
type ReplicaPool struct {
	mu       sync.RWMutex
	replicas []*Replica
	current  int // index of current replica
}

// NewReplicaPool creates a new replica pool
func NewReplicaPool(replicas []*Replica) *ReplicaPool {
	return &ReplicaPool{
		replicas: replicas,
		current:  0,
	}
}

// NextHealthyReplica gets the next healthy replica, cycling through the list
func (p *ReplicaPool) NextHealthyReplica() *Replica {
	p.mu.Lock()
	defer p.mu.Unlock()

	startIdx := p.current
	for {
		if p.replicas[p.current].Healthy {
			return p.replicas[p.current]
		}
		p.current = (p.current + 1) % len(p.replicas)
		if p.current == startIdx {
			// Cycled through all replicas, none are healthy
			return nil
		}
	}
}

// GetReplica returns the replica at the given index
func (p *ReplicaPool) GetReplica(idx int) *Replica {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if idx < 0 || idx >= len(p.replicas) {
		return nil
	}
	return p.replicas[idx]
}

// MarkUnhealthy marks a replica as unhealthy with an error
func (p *ReplicaPool) MarkUnhealthy(replica *Replica, err error) {
	replica.mu.Lock()
	replica.Healthy = false
	replica.LastErr = err
	replica.mu.Unlock()

	fmt.Printf("Replica %s marked unhealthy: %v\n", replica.ID, err)
}

// MarkHealthy marks a replica as healthy
func (p *ReplicaPool) MarkHealthy(replica *Replica) {
	replica.mu.Lock()
	replica.Healthy = true
	replica.LastErr = nil
	replica.mu.Unlock()

	fmt.Printf("Replica %s marked healthy\n", replica.ID)
}

// QueryWithReplica executes a query on a replica with automatic failover
//
// This function demonstrates the key pattern:
// 1. Use DoWithResult() with a custom function
// 2. Inside the function, select next healthy replica
// 3. Execute the query
// 4. If it fails, the retry mechanism will call this function again
// 5. The next iteration will try the next replica
func (p *ReplicaPool) QueryWithReplica(ctx context.Context, query string, strategy retry.Strategy) (string, error) {
	// Use the retry package with a wrapper function that handles replica selection
	result, err := retry.DoWithResult(ctx, strategy, func() (string, error) {
		// Get the next healthy replica
		replica := p.NextHealthyReplica()
		if replica == nil {
			return "", fmt.Errorf("no healthy replicas available")
		}

		// Execute query on the replica
		result, err := executeQuery(ctx, replica, query)
		if err != nil {
			// Mark replica as unhealthy on failure
			p.MarkUnhealthy(replica, err)
			return "", fmt.Errorf("replica query failed: %w", err)
		}

		// Query succeeded, mark replica as healthy
		p.MarkHealthy(replica)
		return result, nil
	})
	if err != nil {
		return "", fmt.Errorf("query with replica failed: %w", err)
	}
	return result, nil
}

// executeQuery simulates executing a query on a replica
func executeQuery(ctx context.Context, replica *Replica, query string) (string, error) {
	// In real implementation, this would connect to the replica and execute the query
	// For now, simulate a network call
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context canceled: %w", ctx.Err())
	case <-time.After(10 * time.Millisecond):
		return fmt.Sprintf("Result from %s: %s", replica.Host, query), nil
	}
}

// Example usage function
//
// This demonstrates how to use the replica pool with the retry package.
// The retry package handles:
// - Backoff between replica attempts (exponential, linear, etc.)
// - Jitter to prevent thundering herd
// - Context cancellation and timeouts
// - Error aggregation
//
// Your code handles:
// - Replica selection logic
// - Health tracking
// - Which replica to try next
func ExampleReplicaFailover() error {
	// Create replica pool
	replicas := []*Replica{
		{ID: "replica-1", Host: "db1.example.com", Port: 5432, Healthy: true},
		{ID: "replica-2", Host: "db2.example.com", Port: 5432, Healthy: true},
		{ID: "replica-3", Host: "db3.example.com", Port: 5432, Healthy: true},
	}
	pool := NewReplicaPool(replicas)

	// Set up retry strategy
	// Linear backoff: 100ms -> 200ms -> 300ms -> ...
	// Max 3 attempts per unique replica
	strategy := retry.NewLinearBackoff(
		100*time.Millisecond, // initialDelay
		1*time.Second,        // maxDelay
		100*time.Millisecond, // increment
		3,                    // maxAttempts per replica
		0.1,                  // jitterFactor (10%)
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute query with automatic replica failover
	result, err := pool.QueryWithReplica(ctx, "SELECT * FROM users", strategy)
	if err != nil {
		fmt.Printf("Query failed after retries: %v\n", err)
		return err
	}

	fmt.Printf("Query succeeded: %s\n", result)
	return nil
}

// ExampleAdvancedReplicaFailover demonstrates nested retry loops with replica failover.
//
// For even more resilience, wrap the replica selection in a second retry loop:
// - Outer loop: retries the entire replica set (exponential backoff)
// - Inner loop: retries individual replicas (linear backoff)
// - Automatic health tracking on failures
// - Context-aware cancellation at all levels
func ExampleAdvancedReplicaFailover() error {
	// Create replica pool
	replicas := []*Replica{
		{ID: "replica-1", Host: "db1.example.com", Port: 5432, Healthy: true},
		{ID: "replica-2", Host: "db2.example.com", Port: 5432, Healthy: true},
		{ID: "replica-3", Host: "db3.example.com", Port: 5432, Healthy: true},
	}
	pool := NewReplicaPool(replicas)

	// Outer strategy: retry the entire replica set
	// Use exponential backoff with longer delays between set attempts
	outerStrategy := retry.NewExponentialBackoff(
		1*time.Second,  // initialDelay
		30*time.Second, // maxDelay
		2.0,            // baseMultiplier
		3,              // maxAttempts
		0.1,            // jitterFactor
	)

	// Inner strategy: retry individual replicas within the set
	// Use linear backoff with shorter delays between replica attempts
	innerStrategy := retry.NewLinearBackoff(
		100*time.Millisecond, // initialDelay
		1*time.Second,        // maxDelay
		100*time.Millisecond, // increment
		3,                    // maxAttempts
		0.1,                  // jitterFactor
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Outer loop: retry the entire replica set
	result, err := retry.DoWithResult(ctx, outerStrategy, func() (string, error) {
		// Inner loop: try replicas within the set
		return pool.QueryWithReplica(ctx, "SELECT * FROM critical_data", innerStrategy)
	})
	if err != nil {
		return fmt.Errorf("query failed after outer retries: %w", err)
	}

	fmt.Printf("Query succeeded with nested retries: %s\n", result)
	return nil
}
