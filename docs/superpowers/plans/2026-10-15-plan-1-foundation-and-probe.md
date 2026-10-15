# Plan 1: Foundation + Live API Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go foundation (config, Bungie API client, manifest lookup) and a `probe` command that records labeled, real Bungie profile responses. These answer the open questions in spec §5.5 and become test data for Plan 2.

**Architecture:** One Go module with small internal packages: `config` (env), `bungie` (HTTP client with retry and throttle handling, typed responses), `manifest` (activity and mode definitions in memory), and `probe` (summary + recorder). `cmd/probe` wires them together. Standard library only.

**Tech Stack:** Go 1.25, `net/http`, `encoding/json`, `httptest`, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-15-destiny-queue-radar-design.md`

**Roadmap:** Plan 1 (this) → run the probe and write findings → Plan 2 (backend service) → Plan 3 (frontend, fake-Bungie mode, E2E).

---

## File Structure

```
go.mod                                   module github.com/jxsoren/destiny-queue-radar
.gitignore                               + testdata/probe/
.github/workflows/ci.yml                 gofmt, vet, test
internal/config/config.go                env → Config
internal/config/config_test.go
internal/bungie/errors.go                error codes, APIError, TransportError, Is* helpers
internal/bungie/transport.go             Client, request/retry/throttle loop, generic call[T]
internal/bungie/transport_test.go
internal/bungie/profile.go               Profile types (components 100,200,204,1000), GetProfile, ActiveCharacterID
internal/bungie/profile_test.go
internal/bungie/search.go                ParseBungieName, FormatBungieName, SearchByBungieName
internal/bungie/search_test.go
internal/bungie/manifest.go              GetManifestInfo, GetFile
internal/bungie/manifest_test.go
internal/manifest/manifest.go            Load, Manifest.Activity/Mode, ModeCategory
internal/manifest/manifest_test.go
internal/probe/summary.go                Summary, Summarize
internal/probe/summary_test.go
internal/probe/recorder.go               Recorder (raw JSON + index.jsonl), Label
internal/probe/recorder_test.go
cmd/probe/main.go                        CLI wiring
docs/probe-runbook.md                    How to run the probe + findings template
```

Responsibilities: `bungie` knows only Bungie's HTTP API and JSON shapes. `manifest` turns definitions into lookups. `probe` is the only package that knows about recording. Plan 2 reuses `config`, `bungie` and `manifest` as-is.

---

### Task 1: Module scaffold + config

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Create the module**

```bash
cd ~/Dev/destiny-queue-radar
go mod init github.com/jxsoren/destiny-queue-radar
```

Expected: `go.mod` created containing `module github.com/jxsoren/destiny-queue-radar` and `go 1.25.x`.

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:

```go
package config

import "testing"

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnv_DefaultsBaseURL(t *testing.T) {
	c, err := FromEnv(envFrom(map[string]string{"BUNGIE_API_KEY": "k"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BungieAPIKey != "k" {
		t.Errorf("api key = %q", c.BungieAPIKey)
	}
	if c.BungieBaseURL != "https://www.bungie.net" {
		t.Errorf("base url = %q", c.BungieBaseURL)
	}
}

func TestFromEnv_TrimsTrailingSlash(t *testing.T) {
	c, err := FromEnv(envFrom(map[string]string{
		"BUNGIE_API_KEY":  "k",
		"BUNGIE_BASE_URL": "http://localhost:9999/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BungieBaseURL != "http://localhost:9999" {
		t.Errorf("base url = %q", c.BungieBaseURL)
	}
}

func TestFromEnv_RequiresAPIKey(t *testing.T) {
	if _, err := FromEnv(envFrom(map[string]string{})); err == nil {
		t.Fatal("expected error when BUNGIE_API_KEY is missing")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL, `undefined: FromEnv`

- [ ] **Step 4: Write minimal implementation**

`internal/config/config.go`:

```go
// Package config loads runtime configuration from the environment.
package config

import (
	"errors"
	"strings"
)

const defaultBungieBaseURL = "https://www.bungie.net"

type Config struct {
	BungieAPIKey  string
	BungieBaseURL string
}

// FromEnv builds a Config using getenv (normally os.Getenv).
func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		BungieAPIKey:  getenv("BUNGIE_API_KEY"),
		BungieBaseURL: getenv("BUNGIE_BASE_URL"),
	}
	if c.BungieAPIKey == "" {
		return Config{}, errors.New("BUNGIE_API_KEY is required")
	}
	if c.BungieBaseURL == "" {
		c.BungieBaseURL = defaultBungieBaseURL
	}
	c.BungieBaseURL = strings.TrimRight(c.BungieBaseURL, "/")
	return c, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: `ok  github.com/jxsoren/destiny-queue-radar/internal/config`

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/config
git commit -m "Add Go module and env config"
```

---

### Task 2: Bungie transport (errors, retries, throttling)

**Files:**
- Create: `internal/bungie/errors.go`
- Create: `internal/bungie/transport.go`
- Test: `internal/bungie/transport_test.go`

Background: every Bungie response is an envelope `{"Response": ..., "ErrorCode": 1, "ErrorStatus": "Success", "Message": "...", "ThrottleSeconds": 0}`. `ErrorCode == 1` means success. Codes used here (check against https://bungie-net.github.io/multi/schema_Exceptions-PlatformErrorCodes.html): `5` SystemDisabled (maintenance), `1601` DestinyAccountNotFound. A non-zero `ThrottleSeconds` means "wait this long before retrying".

Retry policy:
- Throttled: wait `ThrottleSeconds`, retry.
- SystemDisabled: never retry (the caller pauses globally).
- HTTP 5xx or a network error: exponential backoff with jitter.
- Anything else: no retry.
- At most `maxRetries` (3) retries.

- [ ] **Step 1: Write the failing tests**

`internal/bungie/transport_test.go`:

```go
package bungie

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient points a Client at handler and records sleeps instead of sleeping.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("test-key", srv.URL, srv.Client())
	var slept []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return c, &slept
}

type echo struct {
	Value string `json:"value"`
}

func TestCall_DecodesResponseAndSendsAPIKey(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing X-API-Key header")
		}
		w.Write([]byte(`{"Response":{"value":"hi"},"ErrorCode":1,"ErrorStatus":"Success"}`))
	})
	got, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "hi" {
		t.Errorf("value = %q", got.Value)
	}
}

func TestCall_NotFoundIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"ErrorCode":1601,"ErrorStatus":"DestinyAccountNotFound","Message":"nope"}`))
	})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if hits.Load() != 1 || len(*slept) != 0 {
		t.Errorf("hits=%d sleeps=%d, want 1 and 0", hits.Load(), len(*slept))
	}
}

func TestCall_RetriesServerErrorsWithBackoff(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	got, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "ok" || hits.Load() != 3 {
		t.Errorf("value=%q hits=%d", got.Value, hits.Load())
	}
	if len(*slept) != 2 {
		t.Fatalf("sleeps = %v, want 2", *slept)
	}
	if (*slept)[1] <= c.baseBackoff {
		t.Errorf("second backoff %v should exceed base %v", (*slept)[1], c.baseBackoff)
	}
}

func TestCall_HonorsThrottleSeconds(t *testing.T) {
	var hits atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Write([]byte(`{"ErrorCode":36,"ErrorStatus":"ThrottleLimitExceededMomentarily","ThrottleSeconds":2}`))
			return
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != 2*time.Second {
		t.Errorf("sleeps = %v, want [2s]", *slept)
	}
}

func TestCall_GivesUpAfterMaxRetries(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err == nil {
		t.Fatal("expected error")
	}
	if got, want := hits.Load(), int32(c.maxRetries+1); got != want {
		t.Errorf("hits = %d, want %d", got, want)
	}
}

func TestCall_SystemDisabledIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"ErrorCode":5,"ErrorStatus":"SystemDisabled","Message":"maintenance"}`))
	})
	_, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil)
	if !IsSystemDisabled(err) {
		t.Fatalf("expected system-disabled error, got %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

func TestCall_SendsJSONBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method=%s content-type=%q", r.Method, r.Header.Get("Content-Type"))
		}
		w.Write([]byte(`{"Response":{"value":"posted"},"ErrorCode":1}`))
	})
	got, err := call[echo](context.Background(), c, http.MethodPost, "/Platform/Test/", echo{Value: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "posted" {
		t.Errorf("value = %q", got.Value)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bungie/`
Expected: FAIL, `undefined: NewClient`, `undefined: call`, etc.

- [ ] **Step 3: Write errors.go**

`internal/bungie/errors.go`:

```go
package bungie

import (
	"errors"
	"fmt"
)

// Bungie PlatformErrorCodes used by this app.
const (
	CodeSuccess         = 1
	CodeSystemDisabled  = 5
	CodeAccountNotFound = 1601
)

var ErrPlayerNotFound = errors.New("bungie: player not found")

// APIError is a non-success Bungie envelope, or an undecodable HTTP response.
type APIError struct {
	Code            int
	Status          string
	Message         string
	ThrottleSeconds int
	HTTPStatus      int
}

func (e *APIError) Error() string {
	if e.Code == 0 {
		return fmt.Sprintf("bungie: http %d: %s", e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("bungie: error %d %s: %s", e.Code, e.Status, e.Message)
}

// TransportError wraps network-level failures (DNS, connection reset, timeout).
type TransportError struct {
	Path string
	Err  error
}

func (e *TransportError) Error() string { return fmt.Sprintf("bungie: request %s: %v", e.Path, e.Err) }
func (e *TransportError) Unwrap() error { return e.Err }

func IsSystemDisabled(err error) bool { return hasCode(err, CodeSystemDisabled) }

func IsNotFound(err error) bool {
	return errors.Is(err, ErrPlayerNotFound) || hasCode(err, CodeAccountNotFound)
}

func IsThrottled(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.ThrottleSeconds > 0
}

func hasCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}
```

- [ ] **Step 4: Write transport.go**

`internal/bungie/transport.go`:

```go
// Package bungie is a typed client for the Bungie.net Platform API.
package bungie

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey      string
	baseURL     string
	http        *http.Client
	maxRetries  int
	baseBackoff time.Duration
	sleep       func(context.Context, time.Duration) error
}

func NewClient(apiKey, baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		apiKey:      apiKey,
		baseURL:     strings.TrimRight(baseURL, "/"),
		http:        hc,
		maxRetries:  3,
		baseBackoff: 500 * time.Millisecond,
		sleep:       sleepCtx,
	}
}

type envelope struct {
	Response        json.RawMessage `json:"Response"`
	ErrorCode       int             `json:"ErrorCode"`
	ErrorStatus     string          `json:"ErrorStatus"`
	Message         string          `json:"Message"`
	ThrottleSeconds int             `json:"ThrottleSeconds"`
}

// call performs a Platform request and decodes the envelope's Response into T.
func call[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var out T
	raw, err := c.callRaw(ctx, method, path, body)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("bungie: decode %s response: %w", path, err)
	}
	return out, nil
}

// callRaw performs a Platform request with retries and returns the raw Response JSON.
func (c *Client) callRaw(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("bungie: marshal %s body: %w", path, err)
		}
		payload = b
	}
	for attempt := 0; ; attempt++ {
		raw, err := c.callOnce(ctx, method, path, payload)
		if err == nil {
			return raw, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		wait, retry := c.retryDelay(err, attempt)
		if !retry {
			return nil, err
		}
		if err := c.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

func (c *Client) callOnce(ctx context.Context, method, path string, payload []byte) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("bungie: build request %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &TransportError{Path: path, Err: err}
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, &APIError{HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("decode %s: %v", path, err)}
	}
	if env.ErrorCode != CodeSuccess {
		return nil, &APIError{
			Code:            env.ErrorCode,
			Status:          env.ErrorStatus,
			Message:         env.Message,
			ThrottleSeconds: env.ThrottleSeconds,
			HTTPStatus:      resp.StatusCode,
		}
	}
	return env.Response, nil
}

func (c *Client) retryDelay(err error, attempt int) (time.Duration, bool) {
	if attempt >= c.maxRetries {
		return 0, false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Code == CodeSystemDisabled:
			return 0, false
		case apiErr.ThrottleSeconds > 0:
			return time.Duration(apiErr.ThrottleSeconds) * time.Second, true
		case apiErr.HTTPStatus >= 500:
			return c.backoff(attempt), true
		default:
			return 0, false
		}
	}
	var tErr *TransportError
	if errors.As(err, &tErr) {
		return c.backoff(attempt), true
	}
	return 0, false
}

// backoff returns base*2^attempt plus up to 50% jitter.
func (c *Client) backoff(attempt int) time.Duration {
	d := c.baseBackoff << attempt
	return d + time.Duration(rand.Int64N(int64(d)/2+1))
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/bungie/`
Expected: `ok`

- [ ] **Step 6: Commit**

```bash
git add internal/bungie
git commit -m "Add Bungie transport with retry and throttle handling"
```

---

### Task 3: Profile endpoint + active character

**Files:**
- Create: `internal/bungie/profile.go`
- Test: `internal/bungie/profile_test.go`

Background: `GET /Platform/Destiny2/{membershipType}/Profile/{membershipId}/?components=100,200,204,1000`. Each component comes back as `{"data": ..., "privacy": N}`, where privacy `1` = Public and `2` = Private. Per-character components (`characters`, `characterActivities`) have `data` as a map keyed by character ID. Component 1000 (`profileTransitoryData`) has no `data` when the player is offline.

- [ ] **Step 1: Write the failing tests**

`internal/bungie/profile_test.go`:

```go
package bungie

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const profileFixture = `{
  "ErrorCode": 1,
  "Response": {
    "profile": {"privacy": 1, "data": {
      "userInfo": {"membershipType": 3, "membershipId": "4611", "bungieGlobalDisplayName": "Guardian", "bungieGlobalDisplayNameCode": 1993},
      "dateLastPlayed": "2026-10-15T18:00:00Z",
      "characterIds": ["c1", "c2"]
    }},
    "characters": {"privacy": 1, "data": {
      "c1": {"characterId": "c1", "dateLastPlayed": "2026-10-15T18:00:00Z", "emblemPath": "/e1.jpg", "emblemBackgroundPath": "/e1bg.jpg", "classType": 1},
      "c2": {"characterId": "c2", "dateLastPlayed": "2026-10-01T10:00:00Z", "emblemPath": "/e2.jpg", "emblemBackgroundPath": "/e2bg.jpg", "classType": 2}
    }},
    "characterActivities": {"privacy": 1, "data": {
      "c1": {"dateActivityStarted": "2026-10-15T18:05:00Z", "currentActivityHash": 111, "currentActivityModeHash": 222, "currentActivityModeType": 69, "currentPlaylistActivityHash": 333},
      "c2": {"dateActivityStarted": "2026-10-01T10:00:00Z", "currentActivityHash": 0}
    }},
    "profileTransitoryData": {"privacy": 1, "data": {
      "partyMembers": [{"membershipId": "4611", "displayName": "Guardian", "status": 11}],
      "currentActivity": {"startTime": "2026-10-15T18:05:10Z", "numberOfPlayers": 6, "numberOfOpponents": 3}
    }}
  }
}`

func TestGetProfile_RequestsComponentsAndDecodes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Platform/Destiny2/3/Profile/4611/") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("components"); got != ProfileComponents {
			t.Errorf("components = %q", got)
		}
		w.Write([]byte(profileFixture))
	})
	p, err := c.GetProfile(context.Background(), 3, "4611")
	if err != nil {
		t.Fatal(err)
	}
	if p.Profile.Data.UserInfo.BungieGlobalDisplayName != "Guardian" {
		t.Errorf("display name = %q", p.Profile.Data.UserInfo.BungieGlobalDisplayName)
	}
	if p.Characters.Data["c1"].EmblemBackgroundPath != "/e1bg.jpg" {
		t.Errorf("emblem bg = %q", p.Characters.Data["c1"].EmblemBackgroundPath)
	}
	if a := p.CharacterActivities.Data["c1"]; a.CurrentActivityModeHash != 222 || a.CurrentPlaylistActivityHash != 333 {
		t.Errorf("activities = %+v", a)
	}
	if p.ProfileTransitoryData.Data == nil || len(p.ProfileTransitoryData.Data.PartyMembers) != 1 {
		t.Errorf("transitory = %+v", p.ProfileTransitoryData.Data)
	}
	if p.CharacterActivities.Privacy != PrivacyPublic {
		t.Errorf("privacy = %d", p.CharacterActivities.Privacy)
	}
	if len(p.Raw) == 0 || !strings.Contains(string(p.Raw), "profileTransitoryData") {
		t.Errorf("Raw not populated")
	}
}

func TestGetProfile_OfflineHasNoTransitoryData(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{"profileTransitoryData":{"privacy":1}}}`))
	})
	p, err := c.GetProfile(context.Background(), 3, "4611")
	if err != nil {
		t.Fatal(err)
	}
	if p.ProfileTransitoryData.Data != nil {
		t.Errorf("expected nil transitory data for offline player")
	}
}

func TestActiveCharacterID_PicksMostRecentActivity(t *testing.T) {
	p := &Profile{}
	p.CharacterActivities.Data = map[string]CharacterActivities{
		"old": {DateActivityStarted: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		"new": {DateActivityStarted: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)},
	}
	id, ok := p.ActiveCharacterID()
	if !ok || id != "new" {
		t.Errorf("got %q, %v", id, ok)
	}
}

func TestActiveCharacterID_NoData(t *testing.T) {
	if _, ok := (&Profile{}).ActiveCharacterID(); ok {
		t.Error("expected ok=false with no activity data")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bungie/ -run 'Profile|ActiveCharacter'`
Expected: FAIL, `undefined: ProfileComponents`, `undefined: Profile`

- [ ] **Step 3: Write the implementation**

`internal/bungie/profile.go`:

```go
package bungie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProfileComponents: 100 Profiles, 200 Characters, 204 CharacterActivities, 1000 Transitory.
const ProfileComponents = "100,200,204,1000"

// Privacy is Bungie's ComponentPrivacySetting.
type Privacy int

const (
	PrivacyNone    Privacy = 0
	PrivacyPublic  Privacy = 1
	PrivacyPrivate Privacy = 2
)

type single[T any] struct {
	Data    *T      `json:"data"`
	Privacy Privacy `json:"privacy"`
}

type perCharacter[T any] struct {
	Data    map[string]T `json:"data"`
	Privacy Privacy      `json:"privacy"`
}

type UserInfo struct {
	MembershipType              int64  `json:"membershipType"`
	MembershipID                string `json:"membershipId"`
	BungieGlobalDisplayName     string `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode int    `json:"bungieGlobalDisplayNameCode"`
}

type ProfileData struct {
	UserInfo       UserInfo  `json:"userInfo"`
	DateLastPlayed time.Time `json:"dateLastPlayed"`
	CharacterIDs   []string  `json:"characterIds"`
}

type Character struct {
	CharacterID          string    `json:"characterId"`
	DateLastPlayed       time.Time `json:"dateLastPlayed"`
	EmblemPath           string    `json:"emblemPath"`
	EmblemBackgroundPath string    `json:"emblemBackgroundPath"`
	ClassType            int       `json:"classType"`
}

type CharacterActivities struct {
	DateActivityStarted         time.Time `json:"dateActivityStarted"`
	CurrentActivityHash         uint32    `json:"currentActivityHash"`
	CurrentActivityModeHash     uint32    `json:"currentActivityModeHash"`
	CurrentActivityModeType     int       `json:"currentActivityModeType"`
	CurrentActivityModeHashes   []uint32  `json:"currentActivityModeHashes"`
	CurrentActivityModeTypes    []int     `json:"currentActivityModeTypes"`
	CurrentPlaylistActivityHash uint32    `json:"currentPlaylistActivityHash"`
}

type PartyMember struct {
	MembershipID string `json:"membershipId"`
	DisplayName  string `json:"displayName"`
	Status       int    `json:"status"`
}

type TransitoryActivity struct {
	StartTime         *time.Time `json:"startTime"`
	EndTime           *time.Time `json:"endTime"`
	NumberOfPlayers   int        `json:"numberOfPlayers"`
	NumberOfOpponents int        `json:"numberOfOpponents"`
}

type TransitoryData struct {
	PartyMembers    []PartyMember       `json:"partyMembers"`
	CurrentActivity *TransitoryActivity `json:"currentActivity"`
}

type Profile struct {
	Profile               single[ProfileData]               `json:"profile"`
	Characters            perCharacter[Character]           `json:"characters"`
	CharacterActivities   perCharacter[CharacterActivities] `json:"characterActivities"`
	ProfileTransitoryData single[TransitoryData]            `json:"profileTransitoryData"`

	// Raw is the undecoded Response JSON, kept for recording test data.
	Raw json.RawMessage `json:"-"`
}

func (c *Client) GetProfile(ctx context.Context, membershipType int64, membershipID string) (*Profile, error) {
	path := fmt.Sprintf("/Platform/Destiny2/%d/Profile/%s/?components=%s", membershipType, membershipID, ProfileComponents)
	raw, err := c.callRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("bungie: decode profile: %w", err)
	}
	p.Raw = raw
	return &p, nil
}

// ActiveCharacterID returns the character whose current activity started most recently.
func (p *Profile) ActiveCharacterID() (string, bool) {
	var bestID string
	var best time.Time
	for id, a := range p.CharacterActivities.Data {
		if bestID == "" || a.DateActivityStarted.After(best) {
			bestID, best = id, a.DateActivityStarted
		}
	}
	return bestID, bestID != ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bungie/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/bungie/profile.go internal/bungie/profile_test.go
git commit -m "Add GetProfile with activity and transitory components"
```

---

### Task 4: Bungie Name parsing + player search

**Files:**
- Create: `internal/bungie/search.go`
- Test: `internal/bungie/search_test.go`

Background: `POST /Platform/Destiny2/SearchDestinyPlayerByBungieName/-1/` with body `{"displayName":"Guardian","displayNameCode":1993}` returns an array of user info cards, one per platform. With Cross Save, the card whose `membershipType == crossSaveOverride` is the real one. When `crossSaveOverride == 0`, use the first card. Codes are 4 digits and may have leading zeros (`#0880` is code 880).

- [ ] **Step 1: Write the failing tests**

`internal/bungie/search_test.go`:

```go
package bungie

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestParseBungieName(t *testing.T) {
	tests := []struct {
		in       string
		name     string
		code     int
		wantFail bool
	}{
		{in: "Guardian#1993", name: "Guardian", code: 1993},
		{in: "Ghost™#0880", name: "Ghost™", code: 880},
		{in: "Lucky Strike#7173", name: "Lucky Strike", code: 7173},
		{in: "  Spaces#0001  ", name: "Spaces", code: 1},
		{in: "Has#Hash#1234", name: "Has#Hash", code: 1234},
		{in: "NoCode", wantFail: true},
		{in: "#1234", wantFail: true},
		{in: "Short#123", wantFail: true},
		{in: "Letters#12a4", wantFail: true},
	}
	for _, tt := range tests {
		name, code, err := ParseBungieName(tt.in)
		if tt.wantFail {
			if err == nil {
				t.Errorf("%q: expected error", tt.in)
			}
			continue
		}
		if err != nil || name != tt.name || code != tt.code {
			t.Errorf("%q: got (%q, %d, %v), want (%q, %d)", tt.in, name, code, err, tt.name, tt.code)
		}
	}
}

func TestFormatBungieName_PadsCode(t *testing.T) {
	if got := FormatBungieName("Ghost™", 880); got != "Ghost™#0880" {
		t.Errorf("got %q", got)
	}
}

func TestSearchByBungieName_PicksCrossSavePrimary(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/Platform/Destiny2/SearchDestinyPlayerByBungieName/-1/" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["displayName"] != "Guardian" || body["displayNameCode"] != float64(1993) {
			t.Errorf("body = %v", body)
		}
		w.Write([]byte(`{"ErrorCode":1,"Response":[
			{"membershipType":2,"membershipId":"psn","crossSaveOverride":3,"bungieGlobalDisplayName":"Guardian","bungieGlobalDisplayNameCode":1993},
			{"membershipType":3,"membershipId":"steam","crossSaveOverride":3,"bungieGlobalDisplayName":"Guardian","bungieGlobalDisplayNameCode":1993}
		]}`))
	})
	card, err := c.SearchByBungieName(context.Background(), "Guardian", 1993)
	if err != nil {
		t.Fatal(err)
	}
	if card.MembershipID != "steam" || card.MembershipType != 3 {
		t.Errorf("card = %+v", card)
	}
}

func TestSearchByBungieName_NoCrossSaveUsesFirst(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":[{"membershipType":3,"membershipId":"only","crossSaveOverride":0}]}`))
	})
	card, err := c.SearchByBungieName(context.Background(), "X", 1)
	if err != nil || card.MembershipID != "only" {
		t.Errorf("card=%+v err=%v", card, err)
	}
}

func TestSearchByBungieName_EmptyIsNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":[]}`))
	})
	_, err := c.SearchByBungieName(context.Background(), "Nobody", 1)
	if !IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bungie/ -run 'BungieName'`
Expected: FAIL, `undefined: ParseBungieName`

- [ ] **Step 3: Write the implementation**

`internal/bungie/search.go`:

```go
package bungie

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type UserInfoCard struct {
	MembershipType              int64  `json:"membershipType"`
	MembershipID                string `json:"membershipId"`
	CrossSaveOverride           int64  `json:"crossSaveOverride"`
	BungieGlobalDisplayName     string `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode int    `json:"bungieGlobalDisplayNameCode"`
}

// ParseBungieName splits "Name#1234" into its display name and 4-digit code.
func ParseBungieName(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	i := strings.LastIndex(s, "#")
	if i <= 0 {
		return "", 0, fmt.Errorf("bungie name %q: expected Name#1234", s)
	}
	name, codeStr := s[:i], s[i+1:]
	if len(codeStr) != 4 {
		return "", 0, fmt.Errorf("bungie name %q: code must be 4 digits", s)
	}
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return "", 0, fmt.Errorf("bungie name %q: code must be numeric", s)
	}
	return name, code, nil
}

func FormatBungieName(name string, code int) string {
	return fmt.Sprintf("%s#%04d", name, code)
}

type searchBody struct {
	DisplayName     string `json:"displayName"`
	DisplayNameCode int    `json:"displayNameCode"`
}

// SearchByBungieName resolves a Bungie Name to the player's primary (Cross Save) membership.
func (c *Client) SearchByBungieName(ctx context.Context, name string, code int) (UserInfoCard, error) {
	cards, err := call[[]UserInfoCard](ctx, c, http.MethodPost,
		"/Platform/Destiny2/SearchDestinyPlayerByBungieName/-1/", searchBody{DisplayName: name, DisplayNameCode: code})
	if err != nil {
		return UserInfoCard{}, err
	}
	if len(cards) == 0 {
		return UserInfoCard{}, fmt.Errorf("%s: %w", FormatBungieName(name, code), ErrPlayerNotFound)
	}
	for _, card := range cards {
		if card.CrossSaveOverride != 0 && card.MembershipType == card.CrossSaveOverride {
			return card, nil
		}
	}
	return cards[0], nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bungie/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/bungie/search.go internal/bungie/search_test.go
git commit -m "Add Bungie Name parsing and player search"
```

---

### Task 5: Manifest download + activity/mode lookup

**Files:**
- Create: `internal/bungie/manifest.go`
- Test: `internal/bungie/manifest_test.go`
- Create: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

Background: `GET /Platform/Destiny2/Manifest/` returns `version` and `jsonWorldComponentContentPaths.en.<DefinitionName>` paths. Each path is a plain JSON file (not wrapped in an envelope) on the same host, mapping hash strings to definitions. We need:
- `DestinyActivityDefinition`: `hash`, `displayProperties.name`, `isPvP`, `placeHash`
- `DestinyActivityModeDefinition`: `hash`, `displayProperties.name`, `modeType`, `activityModeCategory` (0 None, 1 PvE, 2 PvP, 3 PvECompetitive)

- [ ] **Step 1: Write the failing bungie test**

`internal/bungie/manifest_test.go`:

```go
package bungie

import (
	"context"
	"net/http"
	"testing"
)

func TestGetManifestInfo(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{"version":"v123","jsonWorldComponentContentPaths":{"en":{
			"DestinyActivityDefinition":"/common/act.json",
			"DestinyActivityModeDefinition":"/common/mode.json"}}}}`))
	})
	info, err := c.GetManifestInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := ManifestInfo{Version: "v123", ActivityDefPath: "/common/act.json", ActivityModeDefPath: "/common/mode.json"}
	if info != want {
		t.Errorf("got %+v, want %+v", info, want)
	}
}

func TestGetFile_DecodesPlainJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/common/act.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"1":{"hash":1}}`))
	})
	var out map[string]struct {
		Hash uint32 `json:"hash"`
	}
	if err := c.GetFile(context.Background(), "/common/act.json", &out); err != nil {
		t.Fatal(err)
	}
	if out["1"].Hash != 1 {
		t.Errorf("out = %+v", out)
	}
}

func TestGetFile_Non200IsError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	var out map[string]any
	if err := c.GetFile(context.Background(), "/missing.json", &out); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/bungie/ -run 'Manifest|GetFile'`
Expected: FAIL, `undefined: ManifestInfo`

- [ ] **Step 3: Write bungie/manifest.go**

`internal/bungie/manifest.go`:

```go
package bungie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ManifestInfo struct {
	Version             string
	ActivityDefPath     string
	ActivityModeDefPath string
}

type manifestResponse struct {
	Version string `json:"version"`
	Paths   map[string]struct {
		Activity     string `json:"DestinyActivityDefinition"`
		ActivityMode string `json:"DestinyActivityModeDefinition"`
	} `json:"jsonWorldComponentContentPaths"`
}

func (c *Client) GetManifestInfo(ctx context.Context) (ManifestInfo, error) {
	m, err := call[manifestResponse](ctx, c, http.MethodGet, "/Platform/Destiny2/Manifest/", nil)
	if err != nil {
		return ManifestInfo{}, err
	}
	en, ok := m.Paths["en"]
	if !ok || en.Activity == "" || en.ActivityMode == "" {
		return ManifestInfo{}, fmt.Errorf("bungie: manifest %s missing en activity definitions", m.Version)
	}
	return ManifestInfo{Version: m.Version, ActivityDefPath: en.Activity, ActivityModeDefPath: en.ActivityMode}, nil
}

// GetFile downloads a non-enveloped JSON file (e.g. a manifest definition) and decodes it into out.
func (c *Client) GetFile(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("bungie: build request %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return &TransportError{Path: path, Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &APIError{HTTPStatus: resp.StatusCode, Message: "fetch " + path}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("bungie: decode %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run bungie tests**

Run: `go test ./internal/bungie/`
Expected: `ok`

- [ ] **Step 5: Write the failing manifest test**

`internal/manifest/manifest_test.go`:

```go
package manifest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

type fakeSource struct {
	info  bungie.ManifestInfo
	files map[string]any
}

func (f fakeSource) GetManifestInfo(context.Context) (bungie.ManifestInfo, error) { return f.info, nil }

func (f fakeSource) GetFile(_ context.Context, path string, out any) error {
	b, err := json.Marshal(f.files[path])
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func newFake() fakeSource {
	return fakeSource{
		info: bungie.ManifestInfo{Version: "v1", ActivityDefPath: "/a.json", ActivityModeDefPath: "/m.json"},
		files: map[string]any{
			"/a.json": map[string]any{
				"100": map[string]any{"hash": 100, "displayProperties": map[string]any{"name": "Burnout"}, "isPvP": true, "placeHash": 7},
				"200": map[string]any{"hash": 200, "displayProperties": map[string]any{"name": "Orbit"}, "isPvP": false},
			},
			"/m.json": map[string]any{
				"300": map[string]any{"hash": 300, "displayProperties": map[string]any{"name": "Trials of Osiris"}, "modeType": 84, "activityModeCategory": 2},
				"400": map[string]any{"hash": 400, "displayProperties": map[string]any{"name": "Raid"}, "modeType": 4, "activityModeCategory": 1},
			},
		},
	}
}

func TestLoad_BuildsLookups(t *testing.T) {
	m, err := Load(context.Background(), newFake())
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "v1" {
		t.Errorf("version = %q", m.Version)
	}
	a, ok := m.Activity(100)
	if !ok || a.Name != "Burnout" || !a.IsPvP || a.PlaceHash != 7 {
		t.Errorf("activity = %+v ok=%v", a, ok)
	}
	mode, ok := m.Mode(300)
	if !ok || mode.Name != "Trials of Osiris" || mode.Category != CategoryPvP || mode.ModeType != 84 {
		t.Errorf("mode = %+v ok=%v", mode, ok)
	}
	if raid, _ := m.Mode(400); raid.Category != CategoryPvE {
		t.Errorf("raid category = %v", raid.Category)
	}
	if _, ok := m.Activity(999); ok {
		t.Error("unknown hash should not be found")
	}
}

func TestModeCategory_String(t *testing.T) {
	for cat, want := range map[ModeCategory]string{
		CategoryNone: "None", CategoryPvE: "PvE", CategoryPvP: "PvP", CategoryPvECompetitive: "PvECompetitive", 9: "ModeCategory(9)",
	} {
		if got := cat.String(); got != want {
			t.Errorf("%d: got %q want %q", int(cat), got, want)
		}
	}
}
```

- [ ] **Step 6: Run to verify it fails**

Run: `go test ./internal/manifest/`
Expected: FAIL, `undefined: Load`

- [ ] **Step 7: Write manifest/manifest.go**

`internal/manifest/manifest.go`:

```go
// Package manifest holds the Destiny activity and activity-mode definitions needed to label player status.
package manifest

import (
	"context"
	"fmt"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

// ModeCategory is Bungie's DestinyActivityModeCategory.
type ModeCategory int

const (
	CategoryNone           ModeCategory = 0
	CategoryPvE            ModeCategory = 1
	CategoryPvP            ModeCategory = 2
	CategoryPvECompetitive ModeCategory = 3
)

func (c ModeCategory) String() string {
	switch c {
	case CategoryNone:
		return "None"
	case CategoryPvE:
		return "PvE"
	case CategoryPvP:
		return "PvP"
	case CategoryPvECompetitive:
		return "PvECompetitive"
	default:
		return fmt.Sprintf("ModeCategory(%d)", int(c))
	}
}

type Activity struct {
	Hash      uint32
	Name      string
	IsPvP     bool
	PlaceHash uint32
}

type Mode struct {
	Hash     uint32
	Name     string
	ModeType int
	Category ModeCategory
}

type Manifest struct {
	Version    string
	activities map[uint32]Activity
	modes      map[uint32]Mode
}

// Source is implemented by *bungie.Client.
type Source interface {
	GetManifestInfo(ctx context.Context) (bungie.ManifestInfo, error)
	GetFile(ctx context.Context, path string, out any) error
}

type displayProperties struct {
	Name string `json:"name"`
}

type activityDef struct {
	Hash              uint32            `json:"hash"`
	DisplayProperties displayProperties `json:"displayProperties"`
	IsPvP             bool              `json:"isPvP"`
	PlaceHash         uint32            `json:"placeHash"`
}

type modeDef struct {
	Hash                 uint32            `json:"hash"`
	DisplayProperties    displayProperties `json:"displayProperties"`
	ModeType             int               `json:"modeType"`
	ActivityModeCategory int               `json:"activityModeCategory"`
}

// Load downloads the current English activity and mode definitions.
func Load(ctx context.Context, src Source) (*Manifest, error) {
	info, err := src.GetManifestInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("manifest info: %w", err)
	}
	var acts map[string]activityDef
	if err := src.GetFile(ctx, info.ActivityDefPath, &acts); err != nil {
		return nil, fmt.Errorf("activity definitions: %w", err)
	}
	var modes map[string]modeDef
	if err := src.GetFile(ctx, info.ActivityModeDefPath, &modes); err != nil {
		return nil, fmt.Errorf("activity mode definitions: %w", err)
	}

	m := &Manifest{
		Version:    info.Version,
		activities: make(map[uint32]Activity, len(acts)),
		modes:      make(map[uint32]Mode, len(modes)),
	}
	for _, d := range acts {
		m.activities[d.Hash] = Activity{Hash: d.Hash, Name: d.DisplayProperties.Name, IsPvP: d.IsPvP, PlaceHash: d.PlaceHash}
	}
	for _, d := range modes {
		m.modes[d.Hash] = Mode{Hash: d.Hash, Name: d.DisplayProperties.Name, ModeType: d.ModeType, Category: ModeCategory(d.ActivityModeCategory)}
	}
	return m, nil
}

func (m *Manifest) Activity(hash uint32) (Activity, bool) {
	a, ok := m.activities[hash]
	return a, ok
}

func (m *Manifest) Mode(hash uint32) (Mode, bool) {
	mode, ok := m.modes[hash]
	return mode, ok
}
```

- [ ] **Step 8: Run all tests**

Run: `go test ./...`
Expected: all `ok`

- [ ] **Step 9: Commit**

```bash
git add internal/bungie/manifest.go internal/bungie/manifest_test.go internal/manifest
git commit -m "Add manifest download and activity/mode lookup"
```

---

### Task 6: Probe summary

**Files:**
- Create: `internal/probe/summary.go`
- Test: `internal/probe/summary_test.go`

A `Summary` is the human-readable view of one profile poll. It's what you watch in the terminal during the probe session and what goes into `index.jsonl`.

- [ ] **Step 1: Write the failing tests**

`internal/probe/summary_test.go`:

```go
package probe

import (
	"strings"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
)

type fakeLookup struct {
	acts  map[uint32]manifest.Activity
	modes map[uint32]manifest.Mode
}

func (f fakeLookup) Activity(h uint32) (manifest.Activity, bool) {
	a, ok := f.acts[h]
	return a, ok
}

func (f fakeLookup) Mode(h uint32) (manifest.Mode, bool) {
	m, ok := f.modes[h]
	return m, ok
}

var lookup = fakeLookup{
	acts: map[uint32]manifest.Activity{
		100: {Hash: 100, Name: "Burnout", IsPvP: true},
		500: {Hash: 500, Name: "Trials of Osiris", IsPvP: true},
	},
	modes: map[uint32]manifest.Mode{
		300: {Hash: 300, Name: "Trials of Osiris", Category: manifest.CategoryPvP},
	},
}

func onlineProfile() *bungie.Profile {
	p := &bungie.Profile{}
	started := time.Date(2026, 10, 15, 18, 5, 0, 0, time.UTC)
	p.CharacterActivities.Privacy = bungie.PrivacyPublic
	p.CharacterActivities.Data = map[string]bungie.CharacterActivities{
		"c1": {DateActivityStarted: started, CurrentActivityHash: 100, CurrentActivityModeHash: 300, CurrentPlaylistActivityHash: 500},
		"c2": {DateActivityStarted: started.Add(-72 * time.Hour)},
	}
	p.ProfileTransitoryData.Data = &bungie.TransitoryData{PartyMembers: []bungie.PartyMember{{}, {}, {}}}
	return p
}

func TestSummarize_OnlineInMatch(t *testing.T) {
	s := Summarize("Guardian#1993", onlineProfile(), lookup)
	if !s.Online || s.PartySize != 3 || s.CharacterID != "c1" {
		t.Errorf("online=%v party=%d char=%q", s.Online, s.PartySize, s.CharacterID)
	}
	if s.Activity != "Burnout" || !s.ActivityIsPvP {
		t.Errorf("activity=%q pvp=%v", s.Activity, s.ActivityIsPvP)
	}
	if s.Mode != "Trials of Osiris" || s.ModeCategory != manifest.CategoryPvP {
		t.Errorf("mode=%q cat=%v", s.Mode, s.ModeCategory)
	}
	if s.Playlist != "Trials of Osiris" {
		t.Errorf("playlist=%q", s.Playlist)
	}
	out := s.String()
	for _, want := range []string{"Guardian#1993", "online=true", `activity="Burnout"`, "cat=PvP"} {
		if !strings.Contains(out, want) {
			t.Errorf("String() = %q, missing %q", out, want)
		}
	}
}

func TestSummarize_Offline(t *testing.T) {
	p := onlineProfile()
	p.ProfileTransitoryData.Data = nil
	s := Summarize("X#0001", p, lookup)
	if s.Online || s.PartySize != 0 {
		t.Errorf("online=%v party=%d", s.Online, s.PartySize)
	}
}

func TestSummarize_UnknownHashes(t *testing.T) {
	p := onlineProfile()
	p.CharacterActivities.Data = map[string]bungie.CharacterActivities{
		"c1": {DateActivityStarted: time.Now(), CurrentActivityHash: 42, CurrentActivityModeHash: 43},
	}
	s := Summarize("X#0001", p, lookup)
	if s.Activity != "unknown(42)" || s.Mode != "unknown(43)" || s.Playlist != "" {
		t.Errorf("activity=%q mode=%q playlist=%q", s.Activity, s.Mode, s.Playlist)
	}
}

func TestSummarize_PrivateActivities(t *testing.T) {
	p := &bungie.Profile{}
	p.CharacterActivities.Privacy = bungie.PrivacyPrivate
	s := Summarize("X#0001", p, lookup)
	if s.ActivitiesPrivacy != bungie.PrivacyPrivate || s.CharacterID != "" {
		t.Errorf("privacy=%d char=%q", s.ActivitiesPrivacy, s.CharacterID)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/probe/`
Expected: FAIL, `undefined: Summarize`

- [ ] **Step 3: Write the implementation**

`internal/probe/summary.go`:

```go
// Package probe records live Bungie profile snapshots for exploring player status.
package probe

import (
	"fmt"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
)

// Lookup is implemented by *manifest.Manifest.
type Lookup interface {
	Activity(hash uint32) (manifest.Activity, bool)
	Mode(hash uint32) (manifest.Mode, bool)
}

type Summary struct {
	Player            string                `json:"player"`
	Online            bool                  `json:"online"`
	ActivitiesPrivacy bungie.Privacy        `json:"activitiesPrivacy"`
	CharacterID       string                `json:"characterId,omitempty"`
	ActivityHash      uint32                `json:"activityHash"`
	Activity          string                `json:"activity"`
	ActivityIsPvP     bool                  `json:"activityIsPvP"`
	ModeHash          uint32                `json:"modeHash"`
	Mode              string                `json:"mode"`
	ModeCategory      manifest.ModeCategory `json:"modeCategory"`
	PlaylistHash      uint32                `json:"playlistHash"`
	Playlist          string                `json:"playlist"`
	Started           time.Time             `json:"started"`
	PartySize         int                   `json:"partySize"`
}

func Summarize(player string, p *bungie.Profile, l Lookup) Summary {
	s := Summary{
		Player:            player,
		Online:            p.ProfileTransitoryData.Data != nil,
		ActivitiesPrivacy: p.CharacterActivities.Privacy,
	}
	if s.Online {
		s.PartySize = len(p.ProfileTransitoryData.Data.PartyMembers)
	}
	id, ok := p.ActiveCharacterID()
	if !ok {
		return s
	}
	a := p.CharacterActivities.Data[id]
	s.CharacterID = id
	s.Started = a.DateActivityStarted
	s.ActivityHash, s.ModeHash, s.PlaylistHash = a.CurrentActivityHash, a.CurrentActivityModeHash, a.CurrentPlaylistActivityHash
	s.Activity, s.ActivityIsPvP = activityName(l, a.CurrentActivityHash)
	s.Playlist, _ = activityName(l, a.CurrentPlaylistActivityHash)
	s.Mode, s.ModeCategory = modeName(l, a.CurrentActivityModeHash)
	return s
}

func activityName(l Lookup, hash uint32) (string, bool) {
	if hash == 0 {
		return "", false
	}
	if a, ok := l.Activity(hash); ok {
		return a.Name, a.IsPvP
	}
	return fmt.Sprintf("unknown(%d)", hash), false
}

func modeName(l Lookup, hash uint32) (string, manifest.ModeCategory) {
	if hash == 0 {
		return "", manifest.CategoryNone
	}
	if m, ok := l.Mode(hash); ok {
		return m.Name, m.Category
	}
	return fmt.Sprintf("unknown(%d)", hash), manifest.CategoryNone
}

func (s Summary) String() string {
	return fmt.Sprintf("%-22s online=%t privacy=%d activity=%q pvp=%t mode=%q cat=%s playlist=%q started=%s party=%d",
		s.Player, s.Online, s.ActivitiesPrivacy, s.Activity, s.ActivityIsPvP, s.Mode, s.ModeCategory,
		s.Playlist, s.Started.Local().Format("15:04:05"), s.PartySize)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/probe/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/probe/summary.go internal/probe/summary_test.go
git commit -m "Add probe profile summary"
```

---

### Task 7: Probe recorder + label

**Files:**
- Create: `internal/probe/recorder.go`
- Test: `internal/probe/recorder_test.go`

The recorder writes each raw response to `<dir>/<player-slug>/<timestamp>.json` and appends one line to `<dir>/index.jsonl` with `{time, label, file, summary}`. The label is whatever the operator last typed (for example `P1 orbit after comp`), so each snapshot records what was actually happening.

- [ ] **Step 1: Write the failing tests**

`internal/probe/recorder_test.go`:

```go
package probe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorder_WritesRawFileAndIndexLine(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 15, 18, 5, 7, 123_000_000, time.UTC)
	s := Summary{Player: "Ghost™#0880", Activity: "Orbit"}
	if err := r.Record(at, "P1 orbit", s, []byte(`{"raw":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	rel := filepath.Join("Ghost__0880", "20261015T180507.123Z.json")
	raw, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil || string(raw) != `{"raw":true}` {
		t.Fatalf("raw file: %q, %v", raw, err)
	}

	f, err := os.Open(filepath.Join(dir, "index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("index.jsonl is empty")
	}
	var line indexLine
	if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line.Label != "P1 orbit" || line.File != rel || line.Summary.Activity != "Orbit" || !line.Time.Equal(at) {
		t.Errorf("line = %+v", line)
	}
}

func TestRecorder_AppendsAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		r, err := NewRecorder(dir)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 10, 15, 18, 0, i, 0, time.UTC)
		if err := r.Record(at, "", Summary{Player: "A#0001"}, []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		r.Close()
	}
	b, _ := os.ReadFile(filepath.Join(dir, "index.jsonl"))
	if n := bytes.Count(b, []byte("\n")); n != 2 {
		t.Errorf("index lines = %d, want 2", n)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Guardian#1993":        "Guardian_1993",
		"Ghost™#0880":         "Ghost__0880",
		"Lucky Strike#7173": "Lucky_Strike_7173",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	var l Label
	if l.Get() != "" {
		t.Error("zero label should be empty")
	}
	l.Set("  P1 in comp  \n")
	if l.Get() != "P1 in comp" {
		t.Errorf("label = %q", l.Get())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/probe/ -run 'Recorder|Slug|Label'`
Expected: FAIL, `undefined: NewRecorder`

- [ ] **Step 3: Write the implementation**

`internal/probe/recorder.go`:

```go
package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

type indexLine struct {
	Time    time.Time `json:"time"`
	Label   string    `json:"label"`
	File    string    `json:"file"`
	Summary Summary   `json:"summary"`
}

// Recorder saves raw profile responses plus an index.jsonl describing each one.
type Recorder struct {
	dir   string
	mu    sync.Mutex
	index *os.File
}

func NewRecorder(dir string) (*Recorder, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "index.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	return &Recorder{dir: dir, index: f}, nil
}

func (r *Recorder) Record(at time.Time, label string, s Summary, raw []byte) error {
	rel := filepath.Join(slug(s.Player), at.UTC().Format("20060102T150405.000Z")+".json")
	full := filepath.Join(r.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("create player dir: %w", err)
	}
	if err := os.WriteFile(full, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	line, err := json.Marshal(indexLine{Time: at, Label: label, File: rel, Summary: s})
	if err != nil {
		return fmt.Errorf("marshal index line: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.index.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append index: %w", err)
	}
	return nil
}

func (r *Recorder) Close() error { return r.index.Close() }

func slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Label is the operator's current description of what the probed players are doing.
type Label struct {
	mu sync.Mutex
	v  string
}

func (l *Label) Set(v string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.v = strings.TrimSpace(v)
}

func (l *Label) Get() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.v
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/probe/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/probe/recorder.go internal/probe/recorder_test.go
git commit -m "Add probe recorder and operator label"
```

---

### Task 8: `cmd/probe` CLI + runbook

**Files:**
- Create: `cmd/probe/main.go`
- Create: `docs/probe-runbook.md`
- Modify: `.gitignore`

`cmd/probe` is thin wiring with no logic of its own, so it has no unit tests. It's verified by building it and running `-h`.

- [ ] **Step 1: Write main.go**

`cmd/probe/main.go`:

```go
// Command probe polls real Destiny 2 profiles and records labeled raw responses.
//
//	BUNGIE_API_KEY=... go run ./cmd/probe -players "Guardian#1993,Ghost™#0880"
//
// Type a label and press Enter at any time to tag the following snapshots.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/config"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/probe"
)

type target struct {
	name string
	card bungie.UserInfoCard
}

func main() {
	players := flag.String("players", "", `comma-separated Bungie Names, e.g. "Guardian#1993,Ghost™#0880"`)
	interval := flag.Duration("interval", 15*time.Second, "poll interval")
	out := flag.String("out", "testdata/probe", "output directory")
	flag.Parse()
	if *players == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(strings.Split(*players, ","), *interval, *out); err != nil {
		log.Fatal(err)
	}
}

func run(names []string, interval time.Duration, out string) error {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := bungie.NewClient(cfg.BungieAPIKey, cfg.BungieBaseURL, nil)
	log.Println("loading manifest (downloads tens of MB)...")
	m, err := manifest.Load(ctx, client)
	if err != nil {
		return err
	}
	log.Printf("manifest %s loaded", m.Version)

	var targets []target
	for _, raw := range names {
		name, code, err := bungie.ParseBungieName(raw)
		if err != nil {
			return err
		}
		card, err := client.SearchByBungieName(ctx, name, code)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", raw, err)
		}
		targets = append(targets, target{name: bungie.FormatBungieName(name, code), card: card})
		log.Printf("resolved %s → type=%d id=%s", raw, card.MembershipType, card.MembershipID)
	}

	rec, err := probe.NewRecorder(out)
	if err != nil {
		return err
	}
	defer rec.Close()

	var label probe.Label
	go readLabels(os.Stdin, &label)
	log.Printf("polling %d players every %s → %s", len(targets), interval, out)
	log.Println(`type a label + Enter to tag snapshots (e.g. "P1 orbit after comp"); Ctrl+C to stop`)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		pollAll(ctx, client, m, rec, label.Get(), targets)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func pollAll(ctx context.Context, client *bungie.Client, m *manifest.Manifest, rec *probe.Recorder, label string, targets []target) {
	now := time.Now()
	for _, t := range targets {
		p, err := client.GetProfile(ctx, t.card.MembershipType, t.card.MembershipID)
		if err != nil {
			log.Printf("%s: %v", t.name, err)
			continue
		}
		s := probe.Summarize(t.name, p, m)
		if err := rec.Record(now, label, s, p.Raw); err != nil {
			log.Printf("%s: record: %v", t.name, err)
		}
		fmt.Printf("%s [%s] %s\n", now.Format("15:04:05"), label, s)
	}
}

func readLabels(r io.Reader, l *probe.Label) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l.Set(sc.Text())
		log.Printf("label → %q", l.Get())
	}
}
```

- [ ] **Step 2: Build and check the help output**

Run: `go build ./... && go run ./cmd/probe -h`
Expected: usage text listing `-interval`, `-out`, `-players`; exit status 0 or 2.

Run: `go run ./cmd/probe`
Expected: usage text and `exit status 2`.

- [ ] **Step 3: Ignore raw captures**

Raw captures contain real membership IDs. Plan 2 curates and anonymizes the ones it needs into `internal/classifier/testdata/`.

`.gitignore` should contain:

```
.superpowers/
testdata/probe/
```

- [ ] **Step 4: Write the runbook**

`docs/probe-runbook.md`:

````markdown
# Probe Runbook (Step 0)

Goal: confirm how Bungie's API reports each player state before writing the classifier (spec §5.5).

## Setup

1. Register an app at https://www.bungie.net/en/Application (OAuth type can be "Not applicable" for the probe). Copy the API key.
2. `export BUNGIE_API_KEY=...`
3. Get 1–3 consenting players (you plus friends) with public profiles, and ideally one with a **private** profile.

## Run

```bash
go run ./cmd/probe -players "You#1234,Friend#5678,PrivateFriend#0001" -interval 15s
```

Before each scenario, type its label into the probe terminal and press Enter, e.g. `P1 orbit`.

## Scenarios (label → what to do)

| Label | Action | Hold for |
|---|---|---|
| `P1 offline` | Fully signed out of D2 | 1 min |
| `P1 orbit` | Sitting in Orbit | 1 min |
| `P1 orbit queuing comp` | In Orbit, matchmaking for Competitive | Until match found |
| `P1 loading comp` | Loading into the match | — |
| `P1 in comp` | In a Competitive match | Whole match |
| `P1 post-game` | Post-game screen / back in Orbit | 2 min |
| `P1 tower` | In the Tower | 1 min |
| `P1 pve` | Patrol or strike | 1 min |
| `P1 trials` | One Trials match (if live) | Whole match |
| `P1 fireteam` | Repeat `orbit` and `in comp` in a fireteam with P2 | — |
| `P3 private` | Private player doing anything | 1 min |

## Questions to answer (write the answers in Findings)

1. Can Orbit and social spaces (Tower etc.) be told apart from PvE? Which fields: activity hash/name, mode hash/category, `isPvP`?
2. When a match ends, how many polls until the activity changes? Is there a post-game state?
3. Does anything change between "in Orbit" and "in Orbit, queuing"? (Expected: no.)
4. What does an offline player look like (transitory data missing? stale activity hash?)
5. What does a private player look like (which components have `privacy: 2`, which have data)?
6. Is `currentActivityModeHash` or `currentPlaylistActivityHash` the better source for the playlist (Comp vs Control vs Trials)?
7. Did we see any throttling at this request rate?

## Findings

Write the findings to `docs/superpowers/specs/2026-10-15-probe-findings.md` with one section per question above, quoting `index.jsonl` lines as evidence. Plan 2's classifier is designed from that document.
````

- [ ] **Step 5: Commit**

```bash
git add cmd/probe docs/probe-runbook.md .gitignore
git commit -m "Add probe command and runbook"
```

---

### Task 9: CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

`.github/workflows/ci.yml`:

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: gofmt
        run: test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
      - name: vet
        run: go vet ./...
      - name: test
        run: go test -race ./...
```

- [ ] **Step 2: Run the same checks locally**

Run: `test -z "$(gofmt -l .)" && go vet ./... && go test -race ./...`
Expected: no gofmt output, vet clean, all packages `ok`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "Add CI workflow"
```

---

## After this plan

1. Run the probe following `docs/probe-runbook.md` (needs a real API key and players; **done by a person, not an agent**).
2. Write `docs/superpowers/specs/2026-10-15-probe-findings.md`.
3. Write Plan 2 (backend service) from the spec + findings.

### Carry-overs for Plan 2 (from Task 2 code review)

- **The global rate limiter must cover retries.** `bungie.Client` retries internally, so a limiter checked only before `GetProfile` doesn't see retries or throttle waits. Plan 2 should add an optional `limiter interface{ Wait(context.Context) error }` to `Client`, called before every attempt, *or* stop retrying throttles inside the client and surface them to the poller so it can pause everything at once (as it already does for SystemDisabled).
- Confirm with the probe which HTTP status Bungie uses for 1601 and privacy errors. The client already avoids retrying 5xx responses that carry a definitive error code.
- **Manifest reloads:** build a new `*manifest.Manifest` and publish it with `atomic.Pointer`. Readers take one snapshot per operation. A failed reload is logged and retried later, and the current manifest is never replaced with nil.
- **Orbit and social spaces have no mode:** `currentActivityModeHash` is 0 or absent there, so `Mode(0)` returns `ok=false` and the classifier falls back to the activity name. For PvP, use mode category first, with `Activity.IsPvP` as a fallback. The probe confirms this.
- **Test construction of `bungie.Profile`:** the component wrappers `single[T]`/`perCharacter[T]` are unexported, so other packages can't build `bungie.Profile` literals. Plan 2's classifier tests should either export them (e.g. `Component[T]`, `CharacterComponent[T]`) or decode fixture JSON. Also, `UserInfoCard` and `UserInfo` overlap; do not add a third identity type.
