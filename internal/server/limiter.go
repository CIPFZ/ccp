package server

import (
	"context"
	"errors"
	"time"
)

var errBusy = errors.New("busy")

type limiter struct {
	ch      chan struct{}
	timeout time.Duration
}

func newLimiter(limit int, timeout time.Duration) *limiter {
	if limit <= 0 {
		return nil
	}
	return &limiter{ch: make(chan struct{}, limit), timeout: timeout}
}

func (l *limiter) acquire(ctx context.Context) (func(), error) {
	if l == nil {
		return func() {}, nil
	}
	timer := time.NewTimer(l.timeout)
	defer timer.Stop()
	select {
	case l.ch <- struct{}{}:
		return func() { <-l.ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errBusy
	}
}
