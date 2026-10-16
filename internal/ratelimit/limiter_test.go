package ratelimit

import (
	"context"
	"math"
	"slices"
	"sync"
	"testing"
	"time"
)

// newTestLimiter uses a frozen clock and records sleeps instead of sleeping.
func newTestLimiter(perSecond float64) (*Limiter, *[]time.Duration, time.Time) {
	l := New(perSecond)
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	var slept []time.Duration
	l.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return l, &slept, now
}

func TestWait_SpacesRequestsEvenly(t *testing.T) {
	l, slept, _ := newTestLimiter(10) // one slot per 100ms
	for i := 0; i < 3; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}
	if len(*slept) != 2 || (*slept)[0] != want[0] || (*slept)[1] != want[1] {
		t.Errorf("sleeps = %v, want %v (first call is immediate)", *slept, want)
	}
}

func TestPauseUntil_DelaysNextSlot(t *testing.T) {
	l, slept, now := newTestLimiter(10)
	l.PauseUntil(now.Add(5 * time.Second))
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != 5*time.Second {
		t.Errorf("sleeps = %v, want [5s]", *slept)
	}
}

func TestPauseUntil_EarlierThanNextSlotIsIgnored(t *testing.T) {
	l, slept, now := newTestLimiter(1)                   // one slot per second
	if err := l.Wait(context.Background()); err != nil { // immediate; next slot at +1s
		t.Fatal(err)
	}
	l.PauseUntil(now.Add(500 * time.Millisecond))
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != time.Second {
		t.Errorf("sleeps = %v, want [1s]", *slept)
	}
}

func TestWait_CancelledContext(t *testing.T) {
	l := New(1000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWait_RealSleepHonorsCancel(t *testing.T) {
	l := New(0.001) // one slot per 1000s
	l.Wait(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := l.Wait(ctx); err == nil {
		t.Fatal("expected deadline error")
	}
	if time.Since(start) > time.Second {
		t.Error("Wait did not return promptly on cancel")
	}
}

func TestNew_PanicsOnInvalidRate(t *testing.T) {
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1), 1e12} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%v): expected panic", rate)
				}
			}()
			New(rate)
		}()
	}
}

func TestWait_ConcurrentCallersGetDistinctSlots(t *testing.T) {
	l := New(10)
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	var mu sync.Mutex
	var slept []time.Duration
	l.sleep = func(_ context.Context, d time.Duration) error {
		mu.Lock()
		slept = append(slept, d)
		mu.Unlock()
		return nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Wait(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	slices.Sort(slept)
	var want []time.Duration // the zero-delay call records nothing
	for i := 1; i < 8; i++ {
		want = append(want, time.Duration(i)*100*time.Millisecond)
	}
	if !slices.Equal(slept, want) {
		t.Errorf("delays = %v, want %v", slept, want)
	}
}
