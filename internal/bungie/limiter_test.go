package bungie

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type fakeLimiter struct {
	waits   int
	pauses  []time.Time
	waitErr error
}

func (f *fakeLimiter) Wait(context.Context) error {
	f.waits++
	return f.waitErr
}

func (f *fakeLimiter) PauseUntil(t time.Time) { f.pauses = append(f.pauses, t) }

func TestLimiter_WaitsBeforeEveryAttempt(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	lim := &fakeLimiter{}
	c.SetLimiter(lim)
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if lim.waits != 3 {
		t.Errorf("waits = %d, want 3 (one per attempt)", lim.waits)
	}
}

func TestLimiter_ThrottlePausesLimiterInsteadOfSleeping(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Write([]byte(`{"ErrorCode":36,"ErrorStatus":"ThrottleLimitExceededMomentarily","ThrottleSeconds":2}`))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	lim := &fakeLimiter{}
	c.SetLimiter(lim)
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if len(lim.pauses) != 1 || !lim.pauses[0].Equal(now.Add(2*time.Second)) {
		t.Errorf("pauses = %v, want [now+2s]", lim.pauses)
	}
	if len(*slept) != 0 {
		t.Errorf("client slept %v; the limiter should absorb throttle waits", *slept)
	}
	if lim.waits != 2 {
		t.Errorf("waits = %d, want 2", lim.waits)
	}
}

func TestLimiter_FinalAttemptThrottleStillPauses(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":36,"ErrorStatus":"ThrottleLimitExceededMomentarily","ThrottleSeconds":2}`))
	})
	c.maxRetries = 0
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	lim := &fakeLimiter{}
	c.SetLimiter(lim)
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err == nil {
		t.Fatal("expected error")
	}
	if len(lim.pauses) != 1 || !lim.pauses[0].Equal(now.Add(2*time.Second)) {
		t.Errorf("pauses = %v, want [now+2s]", lim.pauses)
	}
}

func TestLimiter_Bare429PausesLimiter(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("slow down"))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	lim := &fakeLimiter{}
	c.SetLimiter(lim)
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if len(lim.pauses) != 1 {
		t.Errorf("pauses = %v, want exactly 1", lim.pauses)
	}
	if len(*slept) != 0 {
		t.Errorf("client slept %v; the limiter should absorb the wait", *slept)
	}
	if lim.waits != 2 {
		t.Errorf("waits = %d, want 2", lim.waits)
	}
}

func TestLimiter_WaitErrorAbortsWithoutRequest(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { hits.Add(1) })
	boom := errors.New("cancelled")
	c.SetLimiter(&fakeLimiter{waitErr: boom})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if !errors.Is(err, boom) || hits.Load() != 0 {
		t.Errorf("err=%v hits=%d, want limiter error and no request", err, hits.Load())
	}
}

func TestLimiter_GetFileWaits(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	lim := &fakeLimiter{}
	c.SetLimiter(lim)
	var out map[string]any
	if err := c.GetFile(context.Background(), "/f.json", &out); err != nil {
		t.Fatal(err)
	}
	if lim.waits != 1 {
		t.Errorf("waits = %d, want 1", lim.waits)
	}
}
