package web

import (
	"math"
	"strconv"
	"sync"
	"time"
)

// userLimiter allows each user `limit` requests per fixed window.
type userLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	windows map[int64]userWindow
}

type userWindow struct {
	start time.Time
	count int
}

func newUserLimiter(limit int, window time.Duration) *userLimiter {
	return &userLimiter{limit: limit, window: window, now: time.Now, windows: map[int64]userWindow{}}
}

// allow reports whether the user may make another request and, if not, how long until they may.
func (l *userLimiter) allow(userID int64) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w := l.windows[userID]
	if now.Sub(w.start) >= l.window {
		w = userWindow{start: now}
	}
	if w.count >= l.limit {
		return false, l.window - now.Sub(w.start)
	}
	w.count++
	l.windows[userID] = w
	if len(l.windows) > 10_000 {
		for id, old := range l.windows {
			if now.Sub(old.start) >= l.window {
				delete(l.windows, id)
			}
		}
	}
	return true, 0
}

// retryAfterSeconds formats a wait for the Retry-After header (whole seconds, at least 1).
func retryAfterSeconds(d time.Duration) string {
	return strconv.Itoa(max(1, int(math.Ceil(d.Seconds()))))
}
