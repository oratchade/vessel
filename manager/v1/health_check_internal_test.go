//go:build test

package v1

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	db "tounilab.com/vessel/db/v1"
)

// stalledPingDB is a database that stops answering without closing its connection:
// a ping blocks until whoever waits gives up.
type stalledPingDB struct {
	db.DB
}

func (stalledPingDB) Ping(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// A ping with no deadline blocked the health loop forever on a stalled database:
// no failure was ever counted, so the entry stayed healthy through the outage
// (#81). Each ping is now bounded by the health interval.
func TestHealthCheckCountsAStalledPingAsAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	de := &DBEntry{
		ctx:            ctx,
		name:           "stalled",
		db:             stalledPingDB{},
		logger:         &recordingLogger{},
		healthInterval: 10 * time.Millisecond,
	}
	de.healthy.Store(true)
	de.wg.Add(1)
	go de.healthCheck(ctx)

	require.Eventually(t, func() bool { return !de.Health() }, 2*time.Second, 5*time.Millisecond,
		"a database whose ping never returns stayed healthy")

	cancel()
	de.wg.Wait()
}

// deafPingDB never returns from a ping until released, whatever its context says:
// some drivers block past cancellation (go-mssqldb waits for the server to
// acknowledge it).
type deafPingDB struct {
	db.DB
	release  chan struct{}
	inFlight atomic.Int32
	maxSeen  atomic.Int32
}

func (d *deafPingDB) Ping(context.Context) error {
	n := d.inFlight.Add(1)
	for {
		seen := d.maxSeen.Load()
		if n <= seen || d.maxSeen.CompareAndSwap(seen, n) {
			break
		}
	}
	<-d.release
	d.inFlight.Add(-1)
	return nil
}

// A driver that ignores its context must not hold the health check: the wait is
// bounded by the check itself, and a stuck ping is never joined by another (#81).
func TestHealthCheckIsBoundedWhenTheDriverIgnoresItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deaf := &deafPingDB{release: make(chan struct{})}
	de := &DBEntry{
		ctx:            ctx,
		name:           "deaf",
		db:             deaf,
		logger:         &recordingLogger{},
		healthInterval: 10 * time.Millisecond,
	}
	de.healthy.Store(true)
	de.wg.Add(1)
	go de.healthCheck(ctx)

	require.Eventually(t, func() bool { return !de.Health() }, 2*time.Second, 5*time.Millisecond,
		"a database whose driver ignores cancellation stayed healthy")
	require.Equal(t, int32(1), deaf.maxSeen.Load(), "pings piled up behind a stuck one")

	cancel()
	de.wg.Wait() // the loop ends even though the ping has not returned
	close(deaf.release)
}
