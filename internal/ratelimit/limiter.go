// Package ratelimit paces outbound requests across every worker.
package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"
)

// Limiter hands out evenly spaced request slots. It is safe for concurrent use.
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
	now      func() time.Time
	sleep    func(context.Context, time.Duration) error
}

// New returns a limiter allowing perSecond requests per second.
func New(perSecond float64) *Limiter {
	if !(perSecond > 0) || math.IsInf(perSecond, 0) {
		panic("ratelimit: perSecond must be finite and > 0")
	}
	interval := time.Duration(float64(time.Second) / perSecond)
	if interval <= 0 {
		panic("ratelimit: perSecond is too large")
	}
	return &Limiter{
		interval: interval,
		now:      time.Now,
		sleep:    sleepCtx,
	}
}

// Wait blocks until the caller may make one request, or ctx is done.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := l.now()
	slot := l.next
	if slot.Before(now) {
		slot = now
	}
	l.next = slot.Add(l.interval)
	l.mu.Unlock()

	if d := slot.Sub(now); d > 0 {
		return l.sleep(ctx, d)
	}
	return ctx.Err()
}

// PauseUntil delays every future slot until at least t. It does not delay slots already
// reserved by in-progress Wait calls.
func (l *Limiter) PauseUntil(t time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t.After(l.next) {
		l.next = t
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
