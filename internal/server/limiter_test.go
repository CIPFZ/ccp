package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLimiterAllowsWithinCapacity(t *testing.T) {
	limiter := newLimiter(1, 10*time.Millisecond)
	release, err := limiter.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestLimiterReturnsBusyWhenFull(t *testing.T) {
	limiter := newLimiter(1, 5*time.Millisecond)
	release, err := limiter.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, err = limiter.acquire(context.Background())
	if !errors.Is(err, errBusy) {
		t.Fatalf("err=%v", err)
	}
}
