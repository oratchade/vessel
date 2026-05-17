package v1

import (
	"errors"
	"sync/atomic"
)

// ErrManagerNotStarted is returned when work is submitted before Start has run.
var ErrManagerNotStarted = errors.New("database manager is not started")

// ErrManagerClosed is returned when work is submitted after shutdown begins.
var ErrManagerClosed = errors.New("database manager is closed")

type lifecycleState int32

const (
	lifecycleCreated lifecycleState = iota
	lifecycleStarted
	lifecycleStopping
	lifecycleStopped
)

type lifecycle struct {
	state atomic.Int32
}

func (l *lifecycle) load() lifecycleState {
	return lifecycleState(l.state.Load())
}

func (l *lifecycle) compareAndSwap(oldState, newState lifecycleState) bool {
	return l.state.CompareAndSwap(int32(oldState), int32(newState))
}

func (l *lifecycle) store(state lifecycleState) {
	l.state.Store(int32(state))
}

func (l *lifecycle) runningError() error {
	switch l.load() {
	case lifecycleStarted:
		return nil
	case lifecycleCreated:
		return ErrManagerNotStarted
	case lifecycleStopping, lifecycleStopped:
		return ErrManagerClosed
	default:
		return ErrManagerClosed
	}
}
