package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// ErrReauthRequired means the user must sign in with Bungie again.
var ErrReauthRequired = errors.New("auth: sign in again")

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	MembershipID     string `json:"membership_id"` // Bungie.net membership
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
	locks        sync.Map // userID → *sync.Mutex; one refresh at a time per user
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
	return tm.request(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}})
}

// Persist encrypts and stores a token response for the user.
func (tm *TokenManager) Persist(ctx context.Context, userID int64, r TokenResponse) error {
	access, err := tm.box.Encrypt([]byte(r.AccessToken))
	if err != nil {
		return err
	}
	refresh, err := tm.box.Encrypt([]byte(r.RefreshToken))
	if err != nil {
		return err
	}
	now := tm.now()
	return tm.store.SaveTokens(ctx, store.Tokens{
		UserID:           userID,
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  now.Add(time.Duration(r.ExpiresIn) * time.Second),
		RefreshExpiresAt: now.Add(time.Duration(r.RefreshExpiresIn) * time.Second),
	})
}

// AccessToken returns a usable access token for the user, refreshing it when needed.
// It returns ErrReauthRequired when the user has no tokens or the refresh token has expired.
func (tm *TokenManager) AccessToken(ctx context.Context, userID int64) (string, error) {
	mu := tm.lock(userID)
	mu.Lock()
	defer mu.Unlock()

	t, err := tm.store.Tokens(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrReauthRequired
	}
	if err != nil {
		return "", err
	}
	now := tm.now()
	if now.Add(refreshSkew).Before(t.AccessExpiresAt) {
		access, err := tm.box.Decrypt(t.AccessToken)
		if err != nil {
			return "", fmt.Errorf("auth: decrypt access token: %w", err)
		}
		return string(access), nil
	}
	if !now.Before(t.RefreshExpiresAt) {
		return "", ErrReauthRequired
	}
	refresh, err := tm.box.Decrypt(t.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("auth: decrypt refresh token: %w", err)
	}
	r, err := tm.request(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {string(refresh)}})
	if err != nil {
		return "", err
	}
	if err := tm.Persist(ctx, userID, r); err != nil {
		return "", err
	}
	return r.AccessToken, nil
}

func (tm *TokenManager) lock(userID int64) *sync.Mutex {
	m, _ := tm.locks.LoadOrStore(userID, &sync.Mutex{})
	return m.(*sync.Mutex)
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
	switch {
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized:
		return TokenResponse{}, ErrReauthRequired
	case resp.StatusCode/100 != 2:
		return TokenResponse{}, fmt.Errorf("auth: token endpoint returned %d", resp.StatusCode)
	}
	var out TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return TokenResponse{}, fmt.Errorf("auth: decode token response: %w", err)
	}
	if out.AccessToken == "" {
		return TokenResponse{}, errors.New("auth: token response has no access_token")
	}
	return out, nil
}
