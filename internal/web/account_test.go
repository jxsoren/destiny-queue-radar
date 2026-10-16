package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
)

func TestIndex(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(t, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/auth/login") {
		t.Errorf("index = %d %q", rec.Code, rec.Body.String())
	}
}

func TestMe_RequiresSession(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(t, http.MethodGet, "/api/me", nil)
	if rec.Code != http.StatusUnauthorized || decode[map[string]string](t, rec)["error"] == "" {
		t.Errorf("no session = %d %s", rec.Code, rec.Body)
	}
}

func TestMe_ExpiredSessionClearsCookie(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.now = e.now.Add(2 * time.Hour) // the test session lasts 1h
	rec := e.do(t, http.MethodGet, "/api/me", nil, cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d", rec.Code)
	}
	if c := cookieNamed(rec, auth.SessionCookie); c == nil || c.MaxAge >= 0 {
		t.Errorf("session cookie not cleared: %+v", c)
	}
}

func TestMe_ReturnsUser(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	rec := e.do(t, http.MethodGet, "/api/me", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d %s", rec.Code, rec.Body)
	}
	got := decode[meJSON](t, rec)
	if got.ID != u.ID || got.BungieName != "Guardian#1993" || got.MembershipType != 3 || got.MembershipID != "d-Guardian#1993" {
		t.Errorf("me = %+v", got)
	}
}

func TestMutatingRequestsNeedCSRFHeader(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	req := httptest.NewRequest(http.MethodDelete, "/api/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
	if _, ok := e.store.users[1]; !ok {
		t.Error("user deleted without CSRF header")
	}
}

func TestLogout(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	rec := e.do(t, http.MethodPost, "/auth/logout", nil, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}
	if c := cookieNamed(rec, auth.SessionCookie); c == nil || c.MaxAge >= 0 {
		t.Error("session cookie not cleared")
	}
	if rec := e.do(t, http.MethodGet, "/api/me", nil, cookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("session still valid after logout: %d", rec.Code)
	}
	// Logging out without a session is harmless.
	if rec := e.do(t, http.MethodPost, "/auth/logout", nil); rec.Code != http.StatusNoContent {
		t.Errorf("anonymous logout = %d", rec.Code)
	}
}

func TestDeleteMe(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	rec := e.do(t, http.MethodDelete, "/api/me", nil, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d %s", rec.Code, rec.Body)
	}
	if _, ok := e.store.users[u.ID]; ok {
		t.Error("user not deleted")
	}
	if c := cookieNamed(rec, auth.SessionCookie); c == nil || c.MaxAge >= 0 {
		t.Error("session cookie not cleared")
	}
}
