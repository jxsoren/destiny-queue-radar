package web

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/engine"
)

func TestHealthz_AlwaysOK(t *testing.T) {
	e := newTestEnv(t)
	e.store.pingErr = errors.New("db down")

	rec := e.do(t, http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyz_OKWhenDatabaseAndPollerAreHealthy(t *testing.T) {
	e := newTestEnv(t)
	e.engine.lastTick = e.now.Add(-time.Second)

	rec := e.do(t, http.MethodGet, "/readyz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	got := decode[readiness](t, rec)
	if got.Status != "ok" || got.Database != "ok" || got.Poller != "ok" || got.Bungie != engine.APIOK {
		t.Errorf("body = %+v", got)
	}
}

func TestReadyz_UnavailableWhenDatabaseFails(t *testing.T) {
	e := newTestEnv(t)
	e.engine.lastTick = e.now
	e.store.pingErr = errors.New("connection refused")

	rec := e.do(t, http.MethodGet, "/readyz", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := decode[readiness](t, rec); got.Status != "unavailable" || got.Database != "unavailable" || got.Poller != "ok" {
		t.Errorf("body = %+v", got)
	}
}

func TestReadyz_UnavailableWhenPollerIsStale(t *testing.T) {
	for name, tick := range map[string]time.Time{
		"never ticked": {},
		"stalled":      time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC).Add(-pollerStaleAfter - time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t)
			e.engine.lastTick = tick

			rec := e.do(t, http.MethodGet, "/readyz", nil)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			if got := decode[readiness](t, rec); got.Poller != "stalled" || got.Database != "ok" {
				t.Errorf("body = %+v", got)
			}
		})
	}
}

// Bungie outages are reported but do not fail readiness: restarting cannot fix them.
func TestReadyz_BungieDownStillReady(t *testing.T) {
	e := newTestEnv(t)
	e.engine.lastTick = e.now
	e.engine.state = engine.APIDown

	rec := e.do(t, http.MethodGet, "/readyz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := decode[readiness](t, rec); got.Bungie != engine.APIDown {
		t.Errorf("bungie = %q", got.Bungie)
	}
}
