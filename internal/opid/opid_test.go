package opid

import (
	"context"
	"regexp"
	"testing"
)

func TestNew_IsShortHexAndUnique(t *testing.T) {
	a, b := New(), New()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(a) {
		t.Errorf("New() = %q, want 16 hex characters", a)
	}
	if a == b {
		t.Errorf("two IDs are equal: %q", a)
	}
}

func TestWithFrom_RoundTrip(t *testing.T) {
	if got := From(context.Background()); got != "" {
		t.Errorf("From(empty) = %q, want empty", got)
	}
	if got := From(With(context.Background(), "abc")); got != "abc" {
		t.Errorf("From = %q, want abc", got)
	}
}
