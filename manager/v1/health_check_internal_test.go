//go:build test

package v1

import (
	"context"
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
