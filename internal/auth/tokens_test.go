package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/cryptobox"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

type memTokens struct {
	mu   sync.Mutex
	rows map[int64]store.Tokens
}

func (m *memTokens) SaveTokens(_ context.Context, t store.Tokens) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[t.UserID] = t
	return nil
}

func (m *memTokens) Tokens(_ context.Context, id int64) (store.Tokens, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.rows[id]
	if !ok {
		return store.Tokens{}, store.ErrNotFound
	}
	return t, nil
}

type tokenServer struct {
	calls  atomic.Int32
	status int
}

func (ts *tokenServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ts.calls.Add(1)
		if r.URL.Path != "/Platform/App/OAuth/Token/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if id, secret, ok := r.BasicAuth(); !ok || id != "55818" || secret != "shh" {
			t.Errorf("basic auth = %q/%q/%v", id, secret, ok)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		if ts.status != 0 {
			w.WriteHeader(ts.status)
			return
		}
		r.ParseForm()
		access := "access-for-" + r.Form.Get("grant_type")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": access, "refresh_token": "refresh-new", "expires_in": 3600,
			"refresh_expires_in": 7776000, "membership_id": "bnet-1",
		})
	}
}

func newManager(t *testing.T, ts *tokenServer) (*TokenManager, *memTokens, *time.Time) {
	t.Helper()
	srv := httptest.NewServer(ts.handler(t))
	t.Cleanup(srv.Close)
	box, err := cryptobox.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	mem := &memTokens{rows: map[int64]store.Tokens{}}
	tm := NewTokenManager(mem, box, "55818", "shh", srv.URL, srv.Client())
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	tm.now = func() time.Time { return now }
	return tm, mem, &now
}

func TestExchange_PersistAndReuseAccessToken(t *testing.T) {
	ts := &tokenServer{}
	tm, mem, _ := newManager(t, ts)
	ctx := context.Background()

	resp, err := tm.Exchange(ctx, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken != "access-for-authorization_code" || resp.MembershipID != "bnet-1" {
		t.Errorf("resp = %+v", resp)
	}
	if err := tm.Persist(ctx, 1, resp); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(mem.rows[1].AccessToken, []byte("access-for")) {
		t.Error("access token stored in plaintext")
	}
	got, err := tm.AccessToken(ctx, 1)
	if err != nil || got != "access-for-authorization_code" {
		t.Errorf("AccessToken = %q, %v", got, err)
	}
	if ts.calls.Load() != 1 {
		t.Errorf("token endpoint calls = %d, want 1 (no refresh needed)", ts.calls.Load())
	}
}

func TestAccessToken_RefreshesWhenExpired(t *testing.T) {
	ts := &tokenServer{}
	tm, _, now := newManager(t, ts)
	ctx := context.Background()
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)

	*now = now.Add(time.Hour) // access token (3600s) is past its refresh skew
	got, err := tm.AccessToken(ctx, 1)
	if err != nil || got != "access-for-refresh_token" {
		t.Fatalf("AccessToken = %q, %v", got, err)
	}
	// The refreshed token was persisted: no further refresh right away.
	again, _ := tm.AccessToken(ctx, 1)
	if again != got || ts.calls.Load() != 2 {
		t.Errorf("again=%q calls=%d, want reuse and 2 calls", again, ts.calls.Load())
	}
}

func TestAccessToken_ReauthCases(t *testing.T) {
	ctx := context.Background()

	ts := &tokenServer{}
	tm, _, now := newManager(t, ts)
	if _, err := tm.AccessToken(ctx, 1); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("no tokens: %v", err)
	}

	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	*now = now.Add(91 * 24 * time.Hour) // refresh token (90 days) expired too
	if _, err := tm.AccessToken(ctx, 1); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("refresh expired: %v", err)
	}

	rejected := &tokenServer{status: http.StatusBadRequest}
	tm2, _, _ := newManager(t, rejected)
	if _, err := tm2.Exchange(ctx, "bad-code"); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("rejected grant: %v", err)
	}
}

func TestAccessToken_ConcurrentCallersRefreshOnce(t *testing.T) {
	ts := &tokenServer{}
	tm, _, now := newManager(t, ts)
	ctx := context.Background()
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	*now = now.Add(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tm.AccessToken(ctx, 1); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := ts.calls.Load(); got != 2 {
		t.Errorf("token endpoint calls = %d, want 2 (1 exchange + 1 refresh)", got)
	}
}

func TestTokens_BoundToUserAndKind(t *testing.T) {
	ts := &tokenServer{}
	tm, mem, _ := newManager(t, ts)
	ctx := context.Background()
	resp, err := tm.Exchange(ctx, "code")
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.Persist(ctx, 1, resp); err != nil {
		t.Fatal(err)
	}

	// A row copied to another user must not decrypt as that user's token.
	row := mem.rows[1]
	row.UserID = 2
	mem.rows[2] = row
	if got, err := tm.AccessToken(ctx, 2); err == nil {
		t.Errorf("user 2 got a token from user 1's row: %q", got)
	}

	// Swapping the access and refresh ciphertexts must not decrypt either.
	swapped := mem.rows[1]
	swapped.AccessToken, swapped.RefreshToken = swapped.RefreshToken, swapped.AccessToken
	mem.rows[1] = swapped
	if got, err := tm.AccessToken(ctx, 1); err == nil {
		t.Errorf("swapped access/refresh decrypted: %q", got)
	}
}
