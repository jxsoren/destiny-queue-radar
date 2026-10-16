//go:build e2e

package web

import (
	"net/http"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/fakebungie"
)

func TestE2ELogin_SignsInTheFakeUser(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(t, http.MethodGet, "/auth/e2e-login", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("e2e login = %d Location %q", rec.Code, rec.Header().Get("Location"))
	}
	c := cookieNamed(rec, auth.SessionCookie)
	if c == nil || c.Value == "" {
		t.Fatal("no session cookie")
	}
	me := decode[meJSON](t, e.do(t, http.MethodGet, "/api/me", nil, c))
	if me.BungieName != fakebungie.SelfName || me.MembershipID != fakebungie.SelfID || me.MembershipType != fakebungie.SelfType {
		t.Errorf("me = %+v", me)
	}
	if got := e.tokens.persisted[me.ID].AccessToken; got != "e2e-access" {
		t.Errorf("access token = %q, want placeholder tokens so recent matches work", got)
	}
}
