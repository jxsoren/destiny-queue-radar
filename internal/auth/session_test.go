package auth

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewToken_IsRandomAndURLSafe(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewToken()
	if a == b || len(a) != 43 || strings.ContainsAny(a, "+/=") {
		t.Errorf("tokens %q, %q", a, b)
	}
}

func TestHashToken(t *testing.T) {
	if !bytes.Equal(HashToken("x"), HashToken("x")) || len(HashToken("x")) != 32 || bytes.Equal(HashToken("x"), HashToken("y")) {
		t.Error("HashToken must be a deterministic 32-byte digest")
	}
}

func TestCookies(t *testing.T) {
	exp := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	c := SessionCookieFor("tok", exp)
	if c.Name != SessionCookie || c.Value != "tok" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || !c.Expires.Equal(exp) {
		t.Errorf("session cookie = %+v", c)
	}
	if c := ClearSessionCookie(); c.Name != SessionCookie || c.MaxAge >= 0 {
		t.Errorf("clear cookie = %+v", c)
	}
	s := StateCookieFor("st")
	if s.Name != StateCookie || s.Value != "st" || !s.HttpOnly || !s.Secure || s.MaxAge != 600 || s.Path != "/" {
		t.Errorf("state cookie = %+v", s)
	}
	if c := ClearStateCookie(); c.Name != StateCookie || c.MaxAge >= 0 || c.Path != "/" {
		t.Errorf("clear state cookie = %+v", c)
	}
}

func TestStateMatches(t *testing.T) {
	if !StateMatches("abc", "abc") {
		t.Error("equal states should match")
	}
	for _, tc := range [][2]string{{"", ""}, {"abc", ""}, {"abc", "abd"}} {
		if StateMatches(tc[0], tc[1]) {
			t.Errorf("StateMatches(%q, %q) = true", tc[0], tc[1])
		}
	}
}

func TestAuthorizeURL(t *testing.T) {
	u, err := url.Parse(AuthorizeURL("55818", "st"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "www.bungie.net" || u.Path != "/en/OAuth/Authorize" || q.Get("client_id") != "55818" || q.Get("response_type") != "code" || q.Get("state") != "st" {
		t.Errorf("url = %s", u)
	}
}

func TestSessionCookieName_UsesHostPrefix(t *testing.T) {
	if SessionCookie != "__Host-dqr_session" {
		t.Errorf("SessionCookie = %q", SessionCookie)
	}
}

func TestCookieNames_UseHostPrefix(t *testing.T) {
	for _, n := range []string{SessionCookie, StateCookie} {
		if !strings.HasPrefix(n, "__Host-") {
			t.Errorf("%q lacks the __Host- prefix", n)
		}
	}
}
