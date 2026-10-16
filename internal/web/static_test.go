package web

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

const testIndex = `<!doctype html><div id="root"></div>`

func newStaticEnv(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnvWith(t, func(d *Deps) {
		d.Static = fstest.MapFS{
			"index.html":         {Data: []byte(testIndex)},
			"assets/app-1a2b.js": {Data: []byte("console.log(1)")},
			"favicon.svg":        {Data: []byte("<svg/>")},
		}
	})
}

func TestStatic_RootServesIndexUncached(t *testing.T) {
	e := newStaticEnv(t)
	rec := e.do(t, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != testIndex {
		t.Fatalf("GET / = %d %q", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q", got)
	}
	want := map[string]string{
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "same-origin",
		"Content-Security-Policy": "default-src 'self'; img-src 'self' https://www.bungie.net data:; font-src 'self' data:; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestStatic_HistoryFallback(t *testing.T) {
	e := newStaticEnv(t)
	for _, p := range []string{"/board", "/some/deep/link", "/players.json"} {
		rec := e.do(t, http.MethodGet, p, nil)
		if rec.Code != http.StatusOK || rec.Body.String() != testIndex || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("GET %s = %d %q cache %q", p, rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestStatic_HashedAssetsAreImmutable(t *testing.T) {
	e := newStaticEnv(t)
	rec := e.do(t, http.MethodGet, "/assets/app-1a2b.js", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
		t.Fatalf("asset = %d %q", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestStatic_RootFilesRevalidate(t *testing.T) {
	e := newStaticEnv(t)
	rec := e.do(t, http.MethodGet, "/favicon.svg", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("favicon = %d cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestStatic_MissingAssetIs404NotIndex(t *testing.T) {
	e := newStaticEnv(t)
	rec := e.do(t, http.MethodGet, "/assets/old-9z9z.js", nil)
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "root") {
		t.Errorf("missing asset = %d %q", rec.Code, rec.Body)
	}
}

func TestStatic_APIAndAuthNeverFallBack(t *testing.T) {
	e := newStaticEnv(t)
	for _, p := range []string{"/api/nope", "/api", "/auth/nope", "/auth"} {
		rec := e.do(t, http.MethodGet, p, nil)
		if rec.Code != http.StatusNotFound || decode[map[string]string](t, rec)["error"] == "" {
			t.Errorf("GET %s = %d %q, want a JSON 404", p, rec.Code, rec.Body)
		}
	}
	if rec := e.do(t, http.MethodGet, "/api/me", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/me = %d, want 401 (real routes still win)", rec.Code)
	}
}

func TestIndex_PlaceholderWithoutStatic(t *testing.T) {
	e := newTestEnv(t)
	if rec := e.do(t, http.MethodGet, "/board", nil); rec.Code != http.StatusNotFound {
		t.Errorf("without Static, /board = %d, want 404", rec.Code)
	}
}

// A known API route used with the wrong method is a JSON 404 rather than a 405: the catch-all
// GET / pattern (see handleStatic) answers it, and /api never falls back to the SPA.
func TestStatic_WrongMethodOnAPIRouteIs404JSON(t *testing.T) {
	e := newStaticEnv(t)
	rec := e.do(t, http.MethodGet, "/api/refresh", nil)
	if rec.Code != http.StatusNotFound || decode[map[string]string](t, rec)["error"] == "" || strings.Contains(rec.Body.String(), "root") {
		t.Errorf("GET /api/refresh = %d %q, want a JSON 404", rec.Code, rec.Body)
	}
}
