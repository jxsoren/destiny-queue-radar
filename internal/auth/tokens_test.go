package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/cryptobox"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

type memTokens struct {
	mu         sync.Mutex
	rows       map[int64]store.Tokens
	beforeSave func() // simulates events (like a request cancel) arriving just before a save
	failSave   bool
}

func (m *memTokens) SaveTokens(ctx context.Context, t store.Tokens) error {
	if m.beforeSave != nil {
		m.beforeSave()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.failSave {
		return errors.New("save failed")
	}
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
	calls       atomic.Int32
	status      int
	errCode     string // JSON error body for status
	omitRefresh bool   // refresh grant responses carry no refresh_token
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
			if ts.errCode != "" {
				json.NewEncoder(w).Encode(map[string]string{"error": ts.errCode})
			}
			return
		}
		r.ParseForm()
		access := "access-for-" + r.Form.Get("grant_type")
		body := map[string]any{
			"access_token": access, "refresh_token": "refresh-new", "expires_in": 3600,
			"refresh_expires_in": 7776000, "membership_id": "bnet-1",
		}
		if ts.omitRefresh && r.Form.Get("grant_type") == "refresh_token" {
			delete(body, "refresh_token")
			delete(body, "refresh_expires_in")
		}
		json.NewEncoder(w).Encode(body)
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

	rejected := &tokenServer{status: http.StatusBadRequest, errCode: "invalid_grant"}
	tm2, _, _ := newManager(t, rejected)
	if _, err := tm2.Exchange(ctx, "bad-code"); !errors.Is(err, ErrGrantRejected) {
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

func TestPersist_SurvivesCancelledRequest(t *testing.T) {
	ts := &tokenServer{}
	tm, mem, now := newManager(t, ts)
	ctx := context.Background()
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	*now = now.Add(time.Hour)

	cctx, cancel := context.WithCancel(ctx)
	mem.beforeSave = cancel // the request is cancelled after the refresh response, before the save
	got, err := tm.AccessToken(cctx, 1)
	if err != nil || got != "access-for-refresh_token" {
		t.Fatalf("AccessToken = %q, %v", got, err)
	}
	mem.beforeSave = nil
	again, err := tm.AccessToken(ctx, 1)
	if err != nil || again != got || ts.calls.Load() != 2 {
		t.Errorf("refreshed token not persisted: %q %v calls=%d", again, err, ts.calls.Load())
	}
}

func TestAccessToken_PersistFailureStillReturnsTokenWithoutLoggingIt(t *testing.T) {
	var buf strings.Builder
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	ts := &tokenServer{}
	tm, mem, now := newManager(t, ts)
	ctx := context.Background()
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	*now = now.Add(time.Hour)
	mem.failSave = true
	got, err := tm.AccessToken(ctx, 1)
	if err != nil || got != "access-for-refresh_token" {
		t.Fatalf("AccessToken = %q, %v", got, err)
	}
	if !strings.Contains(buf.String(), "save failed") || strings.Contains(buf.String(), "access-for") || strings.Contains(buf.String(), "refresh-new") {
		t.Errorf("log = %q", buf.String())
	}
}

func TestRefresh_KeepsRefreshTokenWhenResponseOmitsIt(t *testing.T) {
	ts := &tokenServer{omitRefresh: true}
	tm, mem, now := newManager(t, ts)
	ctx := context.Background()
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	before := mem.rows[1]
	*now = now.Add(time.Hour)
	if _, err := tm.AccessToken(ctx, 1); err != nil {
		t.Fatal(err)
	}
	after := mem.rows[1]
	if !bytes.Equal(after.RefreshToken, before.RefreshToken) || !after.RefreshExpiresAt.Equal(before.RefreshExpiresAt) {
		t.Error("refresh token or its expiry was overwritten")
	}
	if bytes.Equal(after.AccessToken, before.AccessToken) {
		t.Error("access token was not updated")
	}
}

func TestExchange_RequiresRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"access_token":"a","expires_in":3600,"membership_id":"m"}`))
	}))
	t.Cleanup(srv.Close)
	box, _ := cryptobox.New(bytes.Repeat([]byte{7}, 32))
	tm := NewTokenManager(&memTokens{rows: map[int64]store.Tokens{}}, box, "55818", "shh", srv.URL, srv.Client())
	_, err := tm.Exchange(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "Confidential") {
		t.Errorf("err = %v", err)
	}
}

func TestTokenEndpointErrors(t *testing.T) {
	ctx := context.Background()
	unauth := &tokenServer{status: http.StatusUnauthorized}
	tm, _, _ := newManager(t, unauth)
	if _, err := tm.Exchange(ctx, "c"); !errors.Is(err, ErrClientConfig) {
		t.Errorf("401: %v", err)
	}
	badClient := &tokenServer{status: http.StatusBadRequest, errCode: "invalid_client"}
	tm, _, _ = newManager(t, badClient)
	if _, err := tm.Exchange(ctx, "c"); !errors.Is(err, ErrClientConfig) {
		t.Errorf("invalid_client: %v", err)
	}
	badCode := &tokenServer{status: http.StatusBadRequest, errCode: "invalid_grant"}
	tm, _, _ = newManager(t, badCode)
	if _, err := tm.Exchange(ctx, "c"); !errors.Is(err, ErrGrantRejected) {
		t.Errorf("authorization_code invalid_grant: %v", err)
	}
	emptyBody := &tokenServer{status: http.StatusBadRequest}
	tm, _, _ = newManager(t, emptyBody)
	if _, err := tm.Exchange(ctx, "c"); err == nil || errors.Is(err, ErrReauthRequired) || errors.Is(err, ErrClientConfig) || errors.Is(err, ErrGrantRejected) {
		t.Errorf("400 with empty body: %v", err)
	}
	boom := &tokenServer{status: http.StatusInternalServerError}
	tm, _, _ = newManager(t, boom)
	err := func() error { _, err := tm.Exchange(ctx, "c"); return err }()
	if err == nil || errors.Is(err, ErrClientConfig) || errors.Is(err, ErrGrantRejected) || errors.Is(err, ErrReauthRequired) || !strings.Contains(err.Error(), "500") {
		t.Errorf("500: %v", err)
	}

	ts := &tokenServer{}
	tm, _, now := newManager(t, ts)
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	*now = now.Add(time.Hour)
	ts.status, ts.errCode = http.StatusBadRequest, "invalid_grant"
	if _, err := tm.AccessToken(ctx, 1); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("refresh invalid_grant: %v", err)
	}
}

func TestAccessToken_UnreadableStoredTokensNeedReauth(t *testing.T) {
	ts := &tokenServer{}
	tm, mem, _ := newManager(t, ts)
	ctx := context.Background()
	resp, _ := tm.Exchange(ctx, "code")
	tm.Persist(ctx, 1, resp)
	row := mem.rows[1]
	row.AccessToken = append([]byte(nil), row.AccessToken...)
	row.AccessToken[len(row.AccessToken)-1] ^= 1
	mem.rows[1] = row
	if _, err := tm.AccessToken(ctx, 1); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("err = %v", err)
	}
}

func TestAccessToken_LockRespectsContext(t *testing.T) {
	ts := &tokenServer{}
	tm, _, _ := newManager(t, ts)
	tm.lock(1) <- struct{}{} // someone else holds the user's lock
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := tm.AccessToken(ctx, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
}

func TestTokenResponse_RedactsTokens(t *testing.T) {
	r := TokenResponse{AccessToken: "SECRET-A", RefreshToken: "SECRET-R", MembershipID: "m"}
	for _, out := range []string{fmt.Sprintf("%v", r), fmt.Sprintf("%+v", r), fmt.Sprintf("%#v", r), fmt.Sprint(&r)} {
		if strings.Contains(out, "SECRET") {
			t.Errorf("leaked: %s", out)
		}
	}
	var buf strings.Builder
	slog.New(slog.NewTextHandler(&buf, nil)).Info("x", "resp", r)
	if strings.Contains(buf.String(), "SECRET") {
		t.Errorf("slog leaked: %s", buf.String())
	}
}
