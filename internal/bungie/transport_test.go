package bungie

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient points a Client at handler and records sleeps instead of sleeping.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("test-key", srv.URL, srv.Client())
	var slept []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return c, &slept
}

type echo struct {
	Value string `json:"value"`
}

func TestCall_DecodesResponseAndSendsAPIKey(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing X-API-Key header")
		}
		w.Write([]byte(`{"Response":{"value":"hi"},"ErrorCode":1,"ErrorStatus":"Success"}`))
	})
	got, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "hi" {
		t.Errorf("value = %q", got.Value)
	}
}

func TestCall_NotFoundIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"ErrorCode":1601,"ErrorStatus":"DestinyAccountNotFound","Message":"nope"}`))
	})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if hits.Load() != 1 || len(*slept) != 0 {
		t.Errorf("hits=%d sleeps=%d, want 1 and 0", hits.Load(), len(*slept))
	}
}

func TestCall_RetriesServerErrorsWithBackoff(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	got, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "ok" || hits.Load() != 3 {
		t.Errorf("value=%q hits=%d", got.Value, hits.Load())
	}
	if len(*slept) != 2 {
		t.Fatalf("sleeps = %v, want 2", *slept)
	}
	b := c.baseBackoff
	if (*slept)[0] < b || (*slept)[0] > b*3/2 {
		t.Errorf("first backoff %v not in [%v, %v]", (*slept)[0], b, b*3/2)
	}
	if (*slept)[1] < 2*b || (*slept)[1] > 3*b {
		t.Errorf("second backoff %v not in [%v, %v]", (*slept)[1], 2*b, 3*b)
	}
}

func TestCall_HonorsThrottleSeconds(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Write([]byte(`{"ErrorCode":36,"ErrorStatus":"ThrottleLimitExceededMomentarily","ThrottleSeconds":2}`))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != 2*time.Second {
		t.Errorf("sleeps = %v, want [2s]", *slept)
	}
}

func TestCall_GivesUpAfterMaxRetries(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err == nil {
		t.Fatal("expected error")
	}
	if got, want := hits.Load(), int32(c.maxRetries+1); got != want {
		t.Errorf("hits = %d, want %d", got, want)
	}
}

func TestCall_SystemDisabledIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"ErrorCode":5,"ErrorStatus":"SystemDisabled","Message":"maintenance"}`))
	})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if !IsSystemDisabled(err) {
		t.Fatalf("expected system-disabled error, got %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

func TestCall_SendsJSONBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method=%s content-type=%q", r.Method, r.Header.Get("Content-Type"))
		}
		var in echo
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Value != "x" {
			t.Errorf("body decode err=%v value=%q, want x", err, in.Value)
		}
		w.Write([]byte(`{"Response":{"value":"posted"},"ErrorCode":1}`))
	})
	got, err := call[echo](context.Background(), c, http.MethodPost, "/Platform/Test/", echo{Value: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "posted" {
		t.Errorf("value = %q", got.Value)
	}
}

func TestCall_DefinitiveErrorWithHTTP500IsNotRetried(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ErrorCode":1601,"ErrorStatus":"DestinyAccountNotFound","Message":"nope"}`))
	})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if hits.Load() != 1 || len(*slept) != 0 {
		t.Errorf("hits=%d sleeps=%d, want 1 and 0", hits.Load(), len(*slept))
	}
}

func TestCall_CapsThrottleWait(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Write([]byte(`{"ErrorCode":36,"ErrorStatus":"Throttle","ThrottleSeconds":600}`))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != 60*time.Second {
		t.Errorf("sleeps = %v, want [60s]", *slept)
	}
}

func TestCall_Retries429WithUndecodableBody(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("slow down"))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 || len(*slept) != 1 {
		t.Errorf("hits=%d sleeps=%d, want 2 and 1", hits.Load(), len(*slept))
	}
}

func TestCall_CancelledContextStopsRetries(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.sleep = func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}
	_, err := call[echo](ctx, c, http.MethodGet, "/Platform/Test/", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

func TestCall_EmptyEnvelopeHasMessage(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message == "" {
		t.Fatalf("err = %v, want APIError with message", err)
	}
}
