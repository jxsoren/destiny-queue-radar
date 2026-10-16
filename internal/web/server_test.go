package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/engine"
	"github.com/jxsoren/destiny-queue-radar/internal/hub"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

type fakeSession struct {
	userID  int64
	expires time.Time
}

type fakeStore struct {
	mu       sync.Mutex
	nextID   int64
	users    map[int64]store.User
	sessions map[string]fakeSession
	watch    map[int64][]store.WatchedPlayer
	addErr   error
	// onWatchlist, when set, runs at the start of Watchlist (outside the lock).
	onWatchlist func()
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[int64]store.User{}, sessions: map[string]fakeSession{}, watch: map[int64][]store.WatchedPlayer{}}
}

func (f *fakeStore) user(id int64) store.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.users[id]
}

func (f *fakeStore) SessionUser(_ context.Context, h []byte, now time.Time) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[string(h)]
	if !ok || !now.Before(s.expires) {
		return store.User{}, store.ErrNotFound
	}
	return f.users[s.userID], nil
}

func (f *fakeStore) CreateSession(_ context.Context, h []byte, userID int64, exp time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[string(h)] = fakeSession{userID: userID, expires: exp}
	return nil
}

func (f *fakeStore) DeleteSession(_ context.Context, h []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, string(h))
	return nil
}

func (f *fakeStore) UpsertUser(_ context.Context, bnetID, name string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, u := range f.users {
		if u.BungieMembershipID == bnetID {
			u.BungieName = name
			f.users[id] = u
			return u, nil
		}
	}
	f.nextID++
	u := store.User{ID: f.nextID, BungieMembershipID: bnetID, BungieName: name}
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeStore) SetDestinyMembership(_ context.Context, userID, mType int64, mID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[userID]
	if !ok {
		return store.ErrNotFound
	}
	u.DestinyMembershipType, u.DestinyMembershipID = mType, mID
	f.users[userID] = u
	return nil
}

func (f *fakeStore) DeleteUser(_ context.Context, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.users[userID]; !ok {
		return store.ErrNotFound
	}
	delete(f.users, userID)
	delete(f.watch, userID)
	for k, s := range f.sessions {
		if s.userID == userID {
			delete(f.sessions, k)
		}
	}
	return nil
}

func (f *fakeStore) AddWatch(_ context.Context, userID int64, p store.PlayerRef, note, source string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return f.addErr
	}
	list := f.watch[userID]
	for i := range list {
		if list[i].MembershipID == p.MembershipID {
			return nil
		}
	}
	f.watch[userID] = append(list, store.WatchedPlayer{Player: store.Player{PlayerRef: p}, Note: note, Source: source})
	return nil
}

func (f *fakeStore) RemoveWatch(_ context.Context, userID int64, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.watch[userID]
	for i, w := range list {
		if w.MembershipID == id {
			f.watch[userID] = append(list[:i:i], list[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) Watchlist(_ context.Context, userID int64) ([]store.WatchedPlayer, error) {
	if f.onWatchlist != nil {
		f.onWatchlist()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.WatchedPlayer(nil), f.watch[userID]...), nil
}

type fakeEngine struct {
	mu         sync.Mutex
	state      engine.APIState
	refreshErr error
	refreshed  []int64
	rechecked  []store.PlayerRef
}

func (f *fakeEngine) RequestRefresh(_ context.Context, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshed = append(f.refreshed, userID)
	return f.refreshErr
}

func (f *fakeEngine) Recheck(membershipType int64, membershipID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rechecked = append(f.rechecked, store.PlayerRef{MembershipType: membershipType, MembershipID: membershipID})
}

func (f *fakeEngine) APIState() engine.APIState {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state == "" {
		return engine.APIOK
	}
	return f.state
}

type fakeBungie struct {
	cards      map[string]bungie.UserInfoCard
	profiles   map[string]*bungie.Profile // keyed by membershipId
	profileErr error
	searchErr  error
	current    bungie.CurrentUser
	currentErr error
	gotToken   string
}

func (f *fakeBungie) SearchByBungieName(_ context.Context, name string, code int) (bungie.UserInfoCard, error) {
	if f.searchErr != nil {
		return bungie.UserInfoCard{}, f.searchErr
	}
	c, ok := f.cards[bungie.FormatBungieName(name, code)]
	if !ok {
		return bungie.UserInfoCard{}, bungie.ErrPlayerNotFound
	}
	return c, nil
}

func (f *fakeBungie) GetProfile(_ context.Context, _ int64, id string) (*bungie.Profile, error) {
	if f.profileErr != nil {
		return nil, f.profileErr
	}
	p, ok := f.profiles[id]
	if !ok {
		return nil, bungie.ErrPlayerNotFound
	}
	return p, nil
}

func (f *fakeBungie) GetCurrentUser(_ context.Context, token string) (bungie.CurrentUser, error) {
	f.gotToken = token
	return f.current, f.currentErr
}

type fakeTokens struct {
	mu          sync.Mutex
	exchange    auth.TokenResponse
	exchangeErr error
	gotCode     string
	persisted   map[int64]auth.TokenResponse
	access      string
	accessErr   error
}

func (f *fakeTokens) Exchange(_ context.Context, code string) (auth.TokenResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotCode = code
	return f.exchange, f.exchangeErr
}

func (f *fakeTokens) Persist(_ context.Context, userID int64, r auth.TokenResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.persisted[userID] = r
	return nil
}

func (f *fakeTokens) AccessToken(_ context.Context, _ int64) (string, error) {
	return f.access, f.accessErr
}

type fakeMatches struct {
	recent    []matches.Match
	recentErr error
	byID      map[string]matches.Match
	gotToken  string
	gotMember string
}

func (f *fakeMatches) Recent(_ context.Context, token string, _ int64, membershipID string, _ int) ([]matches.Match, error) {
	f.gotToken, f.gotMember = token, membershipID
	return f.recent, f.recentErr
}

func (f *fakeMatches) Match(_ context.Context, id string) (matches.Match, error) {
	m, ok := f.byID[id]
	if !ok {
		return matches.Match{}, bungie.ErrPlayerNotFound
	}
	return m, nil
}

type testEnv struct {
	s       *Server
	store   *fakeStore
	engine  *fakeEngine
	bungie  *fakeBungie
	tokens  *fakeTokens
	matches *fakeMatches
	hub     *hub.Hub
	now     time.Time
}

func newTestEnv(t *testing.T) *testEnv { return newTestEnvWith(t, nil) }

// newTestEnvWith is newTestEnv with a hook that adjusts the server's Deps before New.
func newTestEnvWith(t *testing.T, adjust func(*Deps)) *testEnv {
	t.Helper()
	e := &testEnv{
		store:   newFakeStore(),
		engine:  &fakeEngine{},
		bungie:  &fakeBungie{cards: map[string]bungie.UserInfoCard{}, profiles: map[string]*bungie.Profile{}},
		tokens:  &fakeTokens{persisted: map[int64]auth.TokenResponse{}},
		matches: &fakeMatches{byID: map[string]matches.Match{}},
		hub:     hub.New(8),
		now:     time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC),
	}
	d := Deps{
		Store: e.store, Engine: e.engine, Hub: e.hub, Bungie: e.bungie, Tokens: e.tokens, Matches: e.matches,
		OAuthClientID: "55818",
		Log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if adjust != nil {
		adjust(&d)
	}
	e.s = New(d)
	e.s.now = func() time.Time { return e.now }
	return e
}

// login creates a user with a Destiny membership and a live session, returning its cookie.
func (e *testEnv) login(t *testing.T, name string) (store.User, *http.Cookie) {
	t.Helper()
	ctx := context.Background()
	u, _ := e.store.UpsertUser(ctx, "bnet-"+name, name)
	e.store.SetDestinyMembership(ctx, u.ID, 3, "d-"+name)
	tok, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	e.store.CreateSession(ctx, auth.HashToken(tok), u.ID, e.now.Add(time.Hour))
	return e.store.user(u.ID), &http.Cookie{Name: auth.SessionCookie, Value: tok}
}

// do sends a request through the server. Non-GET requests carry the CSRF header.
func (e *testEnv) do(t *testing.T, method, target string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, rdr)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if method != http.MethodGet {
		req.Header.Set(requestedWithHeader, requestedWithValue)
	}
	rec := httptest.NewRecorder()
	e.s.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// addProfile registers the profile Bungie returns for a membership ID.
func (e *testEnv) addProfile(mType int64, id, name string, code int) {
	p := &bungie.Profile{}
	p.Profile.Data = &bungie.ProfileData{UserInfo: bungie.UserInfo{
		MembershipType: mType, MembershipID: id, BungieGlobalDisplayName: name, BungieGlobalDisplayNameCode: code}}
	e.bungie.profiles[id] = p
}
