package web

import (
	"testing"
	"time"
)

func TestUserLimiter(t *testing.T) {
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	l := newUserLimiter(2, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if ok, _ := l.allow(1); !ok {
			t.Fatalf("request %d denied", i+1)
		}
	}
	now = now.Add(20 * time.Second)
	ok, wait := l.allow(1)
	if ok || wait != 40*time.Second {
		t.Errorf("third request: ok=%v wait=%v, want denied with 40s", ok, wait)
	}
	if ok, _ := l.allow(2); !ok {
		t.Error("another user must have their own budget")
	}
	now = now.Add(41 * time.Second)
	if ok, _ := l.allow(1); !ok {
		t.Error("new window should allow again")
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "1", 400 * time.Millisecond: "1", 20 * time.Second: "20", 20*time.Second + time.Millisecond: "21"} {
		if got := retryAfterSeconds(d); got != want {
			t.Errorf("retryAfterSeconds(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestUserLimiter_SweepsExpiredWindowsOncePerWindow(t *testing.T) {
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	l := newUserLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	l.allow(1)
	l.allow(2)
	now = now.Add(2 * time.Minute)
	l.allow(3)
	if len(l.windows) != 1 {
		t.Errorf("windows = %d, want expired entries swept leaving 1", len(l.windows))
	}
}
