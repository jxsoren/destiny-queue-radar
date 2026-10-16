package web

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

func TestLogin_RedirectsToBungieWithState(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(t, http.MethodGet, "/auth/login", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("code = %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || loc.Host != "www.bungie.net" || loc.Query().Get("client_id") != "55818" {
		t.Fatalf("location = %v", loc)
	}
	c := cookieNamed(rec, auth.StateCookie)
	if c == nil || c.Value == "" || c.Value != loc.Query().Get("state") {
		t.Errorf("state cookie %+v does not match state param %q", c, loc.Query().Get("state"))
	}
}

func callback(e *testEnv, t *testing.T, query string, state string) *http.Response {
	t.Helper()
	var cookies []*http.Cookie
	if state != "" {
		cookies = append(cookies, &http.Cookie{Name: auth.StateCookie, Value: state})
	}
	return e.do(t, http.MethodGet, "/auth/callback?"+query, nil, cookies...).Result()
}

func TestCallback_RejectsMissingOrWrongState(t *testing.T) {
	e := newTestEnv(t)
	if r := callback(e, t, "code=c&state=abc", ""); r.StatusCode != http.StatusBadRequest {
		t.Errorf("no state cookie = %d", r.StatusCode)
	}
	if r := callback(e, t, "code=c&state=abc", "xyz"); r.StatusCode != http.StatusBadRequest {
		t.Errorf("wrong state = %d", r.StatusCode)
	}
	if e.tokens.gotCode != "" {
		t.Error("code was exchanged despite a bad state")
	}
}

func TestCallback_SignsInAndCreatesSession(t *testing.T) {
	e := newTestEnv(t)
	e.tokens.exchange = auth.TokenResponse{AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 3600, MembershipID: "bnet-1"}
	e.bungie.current = bungie.CurrentUser{
		BungieNetMembershipID: "bnet-1", BungieName: "Guardian#1993",
		Destiny: bungie.UserInfoCard{MembershipType: 3, MembershipID: "4611"},
	}
	r := callback(e, t, "code=the-code&state=st", "st")
	if r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != "/" {
		t.Fatalf("callback = %d, location %q", r.StatusCode, r.Header.Get("Location"))
	}
	if e.tokens.gotCode != "the-code" || e.bungie.gotToken != "acc" {
		t.Errorf("code=%q token=%q", e.tokens.gotCode, e.bungie.gotToken)
	}
	u := e.store.user(1)
	if u.BungieMembershipID != "bnet-1" || u.BungieName != "Guardian#1993" || u.DestinyMembershipType != 3 || u.DestinyMembershipID != "4611" {
		t.Errorf("user = %+v", u)
	}
	if e.tokens.persisted[1].AccessToken != "acc" {
		t.Error("tokens not persisted")
	}
	var session *http.Cookie
	for _, c := range r.Cookies() {
		switch {
		case c.Name == auth.SessionCookie:
			session = c
		case c.Name == auth.StateCookie && c.MaxAge >= 0:
			t.Error("state cookie not cleared")
		}
	}
	if session == nil {
		t.Fatal("no session cookie")
	}
	if rec := e.do(t, http.MethodGet, "/api/me", nil, session); rec.Code != http.StatusOK {
		t.Errorf("new session rejected: %d", rec.Code)
	}
}

func TestCallback_Failures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(e *testEnv)
		query string
		want  int
	}{
		{"user cancelled", func(e *testEnv) {}, "error=access_denied&state=st", http.StatusBadRequest},
		{"missing code", func(e *testEnv) {}, "state=st", http.StatusBadRequest},
		{"code rejected", func(e *testEnv) { e.tokens.exchangeErr = auth.ErrGrantRejected }, "code=c&state=st", http.StatusBadRequest},
		{"token endpoint down", func(e *testEnv) { e.tokens.exchangeErr = errors.New("boom") }, "code=c&state=st", http.StatusBadGateway},
		{"no destiny account", func(e *testEnv) { e.bungie.currentErr = bungie.ErrNoDestinyAccount }, "code=c&state=st", http.StatusForbidden},
		{"bungie down", func(e *testEnv) { e.bungie.currentErr = errors.New("boom") }, "code=c&state=st", http.StatusBadGateway},
	}
	for _, tt := range tests {
		e := newTestEnv(t)
		e.tokens.exchange = auth.TokenResponse{AccessToken: "acc"}
		tt.setup(e)
		if r := callback(e, t, tt.query, "st"); r.StatusCode != tt.want {
			t.Errorf("%s: code = %d, want %d", tt.name, r.StatusCode, tt.want)
		}
	}
}
