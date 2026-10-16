package engine

import (
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	t0 := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	h := health{window: time.Minute}

	if got := h.state(t0); got != APIOK {
		t.Errorf("empty = %s", got)
	}

	// Under 4 samples never counts as degraded.
	h.record(t0, true, false)
	h.record(t0, true, false)
	h.record(t0, true, false)
	if got := h.state(t0); got != APIOK {
		t.Errorf("3 failures = %s, want ok (too few samples)", got)
	}
	h.record(t0, false, false)
	h.record(t0, true, false) // 4 of 5 failed
	if got := h.state(t0); got != APIDegraded {
		t.Errorf("4/5 failed = %s", got)
	}

	// Samples older than the window are forgotten.
	later := t0.Add(2 * time.Minute)
	if got := h.state(later); got != APIOK {
		t.Errorf("after window = %s", got)
	}

	// SystemDisabled means down until a success.
	h.record(later, true, true)
	if got := h.state(later); got != APIDown {
		t.Errorf("disabled = %s", got)
	}
	h.record(later, true, false)
	if got := h.state(later); got != APIDown {
		t.Errorf("failure after disabled = %s, want still down", got)
	}
	h.record(later, false, false)
	if got := h.state(later); got == APIDown {
		t.Errorf("success should clear down, got %s", got)
	}
}
