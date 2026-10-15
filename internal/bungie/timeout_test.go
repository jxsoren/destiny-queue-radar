package bungie

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func slowHandler(attempts *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}
}

func TestCall_PerAttemptTimeoutIsRetriedAsTransportError(t *testing.T) {
	var attempts atomic.Int32
	c, _ := newTestClient(t, slowHandler(&attempts))
	c.requestTimeout = 20 * time.Millisecond
	ctx := context.Background()
	_, err := call[echo](ctx, c, http.MethodGet, "/Platform/Slow/", nil)
	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("err = %v, want *TransportError", err)
	}
	if got := int(attempts.Load()); got != c.maxRetries+1 {
		t.Errorf("attempts = %d, want %d", got, c.maxRetries+1)
	}
	if ctx.Err() != nil {
		t.Errorf("parent ctx cancelled: %v", ctx.Err())
	}
}

func TestGetFile_RespectsFileTimeout(t *testing.T) {
	var attempts atomic.Int32
	c, _ := newTestClient(t, slowHandler(&attempts))
	c.fileTimeout = 20 * time.Millisecond
	var out map[string]any
	if err := c.GetFile(context.Background(), "/slow.json", &out); err == nil {
		t.Fatal("expected timeout error")
	}
}
