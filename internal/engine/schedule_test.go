package engine

import (
	"slices"
	"testing"
	"time"
)

func TestPickDue(t *testing.T) {
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	interval := time.Minute
	entries := map[string]*entry{
		"fresh":    {lastPolled: now.Add(-10 * time.Second)},
		"stale":    {lastPolled: now.Add(-2 * time.Minute)},
		"staler":   {lastPolled: now.Add(-5 * time.Minute)},
		"never":    {},
		"busy":     {lastPolled: now.Add(-time.Hour), inFlight: true},
		"manual":   {lastPolled: now.Add(-20 * time.Second), manualAt: now},
		"manual-b": {lastPolled: now.Add(-30 * time.Second), manualAt: now},
	}

	got := pickDue(entries, now, interval, 10)
	want := []string{"manual-b", "manual", "never", "staler", "stale"}
	if !slices.Equal(got, want) {
		t.Errorf("pickDue = %v, want %v", got, want)
	}

	if got := pickDue(entries, now, interval, 2); !slices.Equal(got, []string{"manual-b", "manual"}) {
		t.Errorf("capped = %v", got)
	}
	if got := pickDue(entries, now, interval, 0); len(got) != 0 {
		t.Errorf("max 0 = %v", got)
	}
}

func TestPickDue_TiesBreakOnID(t *testing.T) {
	now := time.Now()
	same := now.Add(-time.Hour)
	entries := map[string]*entry{"b": {lastPolled: same}, "a": {lastPolled: same}, "c": {lastPolled: same}}
	for i := 0; i < 20; i++ {
		if got := pickDue(entries, now, time.Minute, 3); !slices.Equal(got, []string{"a", "b", "c"}) {
			t.Fatalf("got %v", got)
		}
	}
}
