// Package auth implements Bungie OAuth sign-in: authorization state, sessions and tokens.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	SessionCookie = "dqr_session"
	StateCookie   = "dqr_oauth_state"
	SessionTTL    = 30 * 24 * time.Hour

	stateTTL      = 10 * time.Minute
	authorizeBase = "https://www.bungie.net/en/OAuth/Authorize"
)

// NewToken returns 32 random bytes, base64url-encoded without padding (43 characters).
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the SHA-256 of a session token; only the hash is stored server-side.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// AuthorizeURL is where the browser is sent to sign in with Bungie. Bungie redirects back to the
// redirect URL registered on the application.
func AuthorizeURL(clientID, state string) string {
	q := url.Values{"client_id": {clientID}, "response_type": {"code"}, "state": {state}}
	return authorizeBase + "?" + q.Encode()
}

func SessionCookieFor(token string, expires time.Time) *http.Cookie {
	return &http.Cookie{Name: SessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

func ClearSessionCookie() *http.Cookie {
	return &http.Cookie{Name: SessionCookie, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

// StateCookieFor holds the OAuth state between /auth/login and /auth/callback. SameSite=Lax
// still sends it on Bungie's top-level redirect back to us.
func StateCookieFor(state string) *http.Cookie {
	return &http.Cookie{Name: StateCookie, Value: state, Path: "/auth/", MaxAge: int(stateTTL.Seconds()),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

func ClearStateCookie() *http.Cookie {
	return &http.Cookie{Name: StateCookie, Path: "/auth/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

// StateMatches compares the state cookie with the callback's state parameter in constant time.
func StateMatches(cookieValue, queryValue string) bool {
	return cookieValue != "" && subtle.ConstantTimeCompare([]byte(cookieValue), []byte(queryValue)) == 1
}
