package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/cryptobox"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// refreshSkew refreshes access tokens a little before they expire.
const refreshSkew = 60 * time.Second

const (
	kindAccess  = "access"
	kindRefresh = "refresh"
)

// tokenAAD binds an encrypted token to its owner and type, so a stored value cannot be moved to
// another user's row or swapped between the access and refresh columns.
func tokenAAD(userID int64, kind string) []byte {
	return []byte(fmt.Sprintf("dqr/oauth-token/v1|%d|%s", userID, kind))
}

// ErrReauthRequired means the user must sign in with Bungie again.
var ErrReauthRequired = errors.New("auth: sign in again")

// ErrGrantRejected means Bungie rejected the authorization code (expired, reused or invalid).
var ErrGrantRejected = errors.New("auth: authorization code rejected")

// ErrClientConfig means Bungie rejected this server's client ID or secret.
var ErrClientConfig = errors.New("auth: Bungie rejected the client credentials")

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	MembershipID     string `json:"membership_id"` // Bungie.net membership
}

// String redacts the tokens so a response can't leak through fmt.
func (r TokenResponse) String() string {
	return fmt.Sprintf("TokenResponse{membership_id=%s tokens=[redacted]}", r.MembershipID)
}

// GoString redacts the tokens for %#v.
func (r TokenResponse) GoString() string { return r.String() }

// LogValue redacts the tokens for slog.
func (r TokenResponse) LogValue() slog.Value {
	return slog.StringValue(r.String())
}

// TokenStore is implemented by *store.Store.
type TokenStore interface {
	SaveTokens(ctx context.Context, t store.Tokens) error
	Tokens(ctx context.Context, userID int64) (store.Tokens, error)
}

// TokenManager exchanges, stores (encrypted) and refreshes Bungie OAuth tokens for a
// confidential client.
type TokenManager struct {
	store        TokenStore
	box          *cryptobox.Box
	clientID     string
	clientSecret string
	tokenURL     string
	http         *http.Client
	now          func() time.Time
	// locks maps userID to a 1-slot channel: one refresh at a time per user, abandonable via ctx.
	// The map is bounded by the number of users. It assumes a single process; multiple instances
	// would need a database lock instead. The lock is not reentrant: never acquire it from inside
	// AccessToken or from the token transport.
	locks sync.Map
}

func NewTokenManager(st TokenStore, box *cryptobox.Box, clientID, clientSecret, bungieBaseURL string, hc *http.Client) *TokenManager {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &TokenManager{
		store:        st,
		box:          box,
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     strings.TrimRight(bungieBaseURL, "/") + "/Platform/App/OAuth/Token/",
		http:         hc,
		now:          time.Now,
	}
}

// Exchange swaps an authorization code from the OAuth callback for tokens.
func (tm *TokenManager) Exchange(ctx context.Context, code string) (TokenResponse, error) {
	r, err := tm.request(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}})
	if err != nil {
		return TokenResponse{}, err
	}
	if r.RefreshToken == "" || r.RefreshExpiresIn <= 0 {
		return TokenResponse{}, errors.New("auth: Bungie returned no refresh token; the Bungie app must be a Confidential client")
	}
	return r, nil
}

// Persist encrypts and stores a token response for the user.
func (tm *TokenManager) Persist(ctx context.Context, userID int64, r TokenResponse) error {
	return tm.persist(ctx, userID, r, nil)
}

// persist stores r. When prev is set and r carries no refresh token, the previous refresh token
// and its expiry are kept rather than overwritten.
func (tm *TokenManager) persist(ctx context.Context, userID int64, r TokenResponse, prev *store.Tokens) error {
	access, err := tm.box.Encrypt([]byte(r.AccessToken), tokenAAD(userID, kindAccess))
	if err != nil {
		return err
	}
	now := tm.now()
	row := store.Tokens{
		UserID:          userID,
		AccessToken:     access,
		AccessExpiresAt: now.Add(time.Duration(r.ExpiresIn) * time.Second),
	}
	if r.RefreshToken == "" && prev != nil {
		row.RefreshToken = prev.RefreshToken
		row.RefreshExpiresAt = prev.RefreshExpiresAt
	} else {
		refresh, err := tm.box.Encrypt([]byte(r.RefreshToken), tokenAAD(userID, kindRefresh))
		if err != nil {
			return err
		}
		row.RefreshToken = refresh
		row.RefreshExpiresAt = now.Add(time.Duration(r.RefreshExpiresIn) * time.Second)
	}
	return tm.store.SaveTokens(ctx, row)
}

// AccessToken returns a usable access token for the user, refreshing it when needed.
// It returns ErrReauthRequired when the user has no tokens or the refresh token has expired.
func (tm *TokenManager) AccessToken(ctx context.Context, userID int64) (string, error) {
	lock := tm.lock(userID)
	select {
	case lock <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-lock }()

	t, err := tm.store.Tokens(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrReauthRequired
	}
	if err != nil {
		return "", err
	}
	now := tm.now()
	if now.Add(refreshSkew).Before(t.AccessExpiresAt) {
		access, err := tm.box.Decrypt(t.AccessToken, tokenAAD(userID, kindAccess))
		if err != nil {
			slog.Default().Warn("auth: stored access token unreadable", "user_id", userID, "err", err)
			return "", fmt.Errorf("%w: stored tokens unreadable", ErrReauthRequired)
		}
		return string(access), nil
	}
	if !now.Before(t.RefreshExpiresAt) {
		return "", ErrReauthRequired
	}
	refresh, err := tm.box.Decrypt(t.RefreshToken, tokenAAD(userID, kindRefresh))
	if err != nil {
		slog.Default().Warn("auth: stored refresh token unreadable", "user_id", userID, "err", err)
		return "", fmt.Errorf("%w: stored tokens unreadable", ErrReauthRequired)
	}
	r, err := tm.request(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {string(refresh)}})
	if err != nil {
		return "", err
	}
	// Bungie rotates refresh tokens, so the new tokens must be saved even if the caller's request
	// was cancelled meanwhile. A failed save still returns the fresh access token.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := tm.persist(pctx, userID, r, &t); err != nil {
		slog.Default().Error("auth: persist refreshed tokens failed", "user_id", userID, "err", err)
	}
	return r.AccessToken, nil
}

func (tm *TokenManager) lock(userID int64) chan struct{} {
	m, _ := tm.locks.LoadOrStore(userID, make(chan struct{}, 1))
	return m.(chan struct{})
}

func (tm *TokenManager) request(ctx context.Context, form url.Values) (TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tm.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("auth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(tm.clientID, tm.clientSecret)

	resp, err := tm.http.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("auth: token request: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
	}()
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&e)
		switch {
		case resp.StatusCode == http.StatusUnauthorized || (resp.StatusCode == http.StatusBadRequest &&
			(e.Error == "invalid_client" || e.Error == "unauthorized_client")):
			return TokenResponse{}, fmt.Errorf("%w (status %d)", ErrClientConfig, resp.StatusCode)
		case resp.StatusCode == http.StatusBadRequest && e.Error == "invalid_grant":
			if form.Get("grant_type") == "refresh_token" {
				return TokenResponse{}, ErrReauthRequired
			}
			return TokenResponse{}, ErrGrantRejected
		}
		return TokenResponse{}, fmt.Errorf("auth: token endpoint returned %d", resp.StatusCode)
	}
	var out TokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return TokenResponse{}, fmt.Errorf("auth: decode token response: %w", err)
	}
	if out.AccessToken == "" {
		return TokenResponse{}, errors.New("auth: token response has no access_token")
	}
	return out, nil
}
