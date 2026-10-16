package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/hub"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// --- fakes ---

type fakeStore struct {
	mu       sync.Mutex
	players  map[string]store.Player
	watchers map[string][]int64
	saves    map[string]int
	failures map[string]int
	err      error // returned by WatchedBy when set
}

func newFakeStore() *fakeStore {
	return &fakeStore{players: map[string]store.Player{}, watchers: map[string][]int64{}, saves: map[string]int{}, failures: map[string]int{}}
}

func (f *fakeStore) watch(userID int64, p store.Player) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.players[p.MembershipID] = p
	f.watchers[p.MembershipID] = append(f.watchers[p.MembershipID], userID)
}

func (f *fakeStore) WatchedBy(_ context.Context, userIDs []int64) ([]store.Player, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	want := map[int64]bool{}
	for _, id := range userIDs {
		want[id] = true
	}
	var out []store.Player
	for id, ws := range f.watchers {
		for _, u := range ws {
			if want[u] {
				out = append(out, f.players[id])
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MembershipID < out[j].MembershipID })
	return out, nil
}

func (f *fakeStore) WatchersOf(_ context.Context, id string) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.watchers[id]...), nil
}

func (f *fakeStore) Player(_ context.Context, id string) (store.Player, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.players[id]
	if !ok {
		return store.Player{}, store.ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) SavePollSuccess(_ context.Context, id string, st status.Status, emblem, bg string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.players[id]
	p.Status = st
	if emblem != "" {
		p.EmblemPath = emblem
	}
	if bg != "" {
		p.EmblemBackgroundPath = bg
	}
	p.LastPolledAt, p.LastSuccessAt = &at, &at
	f.players[id] = p
	f.saves[id]++
	return nil
}

func (f *fakeStore) SavePollFailure(_ context.Context, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.players[id]
	p.LastPolledAt = &at
	f.players[id] = p
	f.failures[id]++
	return nil
}

func (f *fakeStore) count(m map[string]int, id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return m[id]
}

func (f *fakeStore) get(id string) store.Player {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.players[id]
}

type fakeProfiles struct {
	mu    sync.Mutex
	errs  map[string]error
	calls map[string]int
}

func newFakeProfiles() *fakeProfiles {
	return &fakeProfiles{errs: map[string]error{}, calls: map[string]int{}}
}

func (f *fakeProfiles) GetProfile(_ context.Context, _ int64, id string) (*bungie.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[id]++
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	return &bungie.Profile{}, nil
}

func (f *fakeProfiles) callCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[id]
}

type sent struct {
	users []int64
	ev    hub.Event
}

type fakeHub struct {
	mu         sync.Mutex
	active     []int64
	sent       []sent
	broadcasts []hub.Event
}

func (f *fakeHub) ActiveUsers(time.Duration) []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.active...)
}

func (f *fakeHub) SendTo(users []int64, ev hub.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{users: users, ev: ev})
}

func (f *fakeHub) Broadcast(ev hub.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.broadcasts = append(f.broadcasts, ev)
}

func (f *fakeHub) sentEvents() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sent(nil), f.sent...)
}

func (f *fakeHub) broadcastEvents() []hub.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]hub.Event(nil), f.broadcasts...)
}

type classifyFunc func(*bungie.Profile, status.Status, time.Time) status.Status

func (f classifyFunc) Classify(p *bungie.Profile, prev status.Status, now time.Time) status.Status {
	return f(p, prev, now)
}

func always(st status.Status) classifyFunc {
	return func(*bungie.Profile, status.Status, time.Time) status.Status { return st }
}

// --- helpers ---

func testConfig() Config {
	return Config{
		Interval:        time.Hour,
		Grace:           time.Minute,
		Workers:         2,
		RefreshCooldown: 30 * time.Second,
		RefreshSkip:     10 * time.Second,
		DownProbe:       time.Minute,
		Tick:            5 * time.Millisecond,
	}
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func player(id string) store.Player {
	return store.Player{PlayerRef: store.PlayerRef{MembershipType: 3, MembershipID: id}}
}

func run(t *testing.T, e *Engine) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// --- tests ---

func TestEngine_PollsOnlyActiveUsersPlayersAndPublishesChange(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	st.watch(2, player("p2")) // user 2 is offline
	e := New(testConfig(), st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	run(t, e)

	eventually(t, "p1 saved", func() bool { return st.count(st.saves, "p1") == 1 })
	if pr.callCount("p2") != 0 {
		t.Error("p2 polled although its only watcher is offline")
	}
	if got := st.get("p1").Status.State; got != status.InMatch {
		t.Errorf("stored state = %s", got)
	}
	eventually(t, "update sent", func() bool { return len(h.sentEvents()) == 1 })
	ev := h.sentEvents()[0]
	upd, ok := ev.ev.Data.(PlayerUpdate)
	if ev.ev.Type != EventPlayerUpdate || !ok || upd.MembershipID != "p1" || upd.Status.State != status.InMatch {
		t.Errorf("event = %+v", ev.ev)
	}
	if len(ev.users) != 1 || ev.users[0] != 1 {
		t.Errorf("sent to %v, want [1]", ev.users)
	}
}

func TestEngine_NoEventWhenStatusUnchanged(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	cfg := testConfig()
	cfg.Interval = 10 * time.Millisecond
	e := New(cfg, st, pr, always(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}), h, quietLog())
	run(t, e)

	eventually(t, "3 polls", func() bool { return st.count(st.saves, "p1") >= 3 })
	if n := len(h.sentEvents()); n != 1 {
		t.Errorf("events = %d, want 1 (only the first poll changed status)", n)
	}
}

func TestEngine_NotFoundIsStoredAsStatus(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("gone"))
	pr.errs["gone"] = &bungie.APIError{Code: bungie.CodeAccountNotFound}
	e := New(testConfig(), st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	run(t, e)

	eventually(t, "not found saved", func() bool { return st.get("gone").Status.State == status.NotFound })
	if got := e.APIState(); got != APIOK {
		t.Errorf("api state = %s; not-found is not an API failure", got)
	}
}

func TestEngine_TransportErrorRecordsFailureOnly(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	pr.errs["p1"] = &bungie.TransportError{Path: "/x", Err: errors.New("connection reset")}
	e := New(testConfig(), st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	run(t, e)

	eventually(t, "failure saved", func() bool { return st.count(st.failures, "p1") == 1 })
	if st.count(st.saves, "p1") != 0 || len(h.sentEvents()) != 0 {
		t.Error("a failed poll must not save a status or publish")
	}
}

func TestEngine_SystemDisabledBroadcastsDown(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	pr.errs["p1"] = &bungie.APIError{Code: bungie.CodeSystemDisabled}
	e := New(testConfig(), st, pr, always(status.Status{}), h, quietLog())
	run(t, e)

	eventually(t, "down broadcast", func() bool { return len(h.broadcastEvents()) > 0 })
	ev := h.broadcastEvents()[0]
	if upd, ok := ev.Data.(APIStatusUpdate); ev.Type != EventAPIStatus || !ok || upd.Bungie != APIDown {
		t.Errorf("broadcast = %+v", ev)
	}
	if e.APIState() != APIDown {
		t.Errorf("APIState = %s", e.APIState())
	}
}

func TestDispatch_WhileDownOnlyProbesOncePerInterval(t *testing.T) {
	st, h := newFakeStore(), &fakeHub{}
	e := New(testConfig(), st, newFakeProfiles(), always(status.Status{}), h, quietLog())
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	e.now = func() time.Time { return now }
	for _, id := range []string{"a", "b", "c"} {
		e.entries[id] = &entry{player: player(id)}
	}
	e.apiState = APIDown
	jobs := make(chan string, 5)

	e.lastProbe = now.Add(-10 * time.Second)
	e.dispatch(jobs)
	if len(jobs) != 0 {
		t.Fatalf("dispatched %d while down within probe interval", len(jobs))
	}
	e.lastProbe = now.Add(-2 * time.Minute)
	e.dispatch(jobs)
	if len(jobs) != 1 {
		t.Errorf("dispatched %d, want 1 probe", len(jobs))
	}
}

func TestRequestRefresh_Cooldown(t *testing.T) {
	st := newFakeStore()
	st.watch(1, player("p1"))
	e := New(testConfig(), st, newFakeProfiles(), always(status.Status{}), &fakeHub{}, quietLog())
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	e.now = func() time.Time { return now }

	if err := e.RequestRefresh(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Second)
	err := e.RequestRefresh(context.Background(), 1)
	var cd *CooldownError
	if !errors.As(err, &cd) || cd.RetryAfter != 20*time.Second {
		t.Fatalf("err = %v, want CooldownError{20s}", err)
	}
	now = now.Add(20 * time.Second)
	if err := e.RequestRefresh(context.Background(), 1); err != nil {
		t.Errorf("after cooldown: %v", err)
	}
}

func TestRequestRefresh_SkipsRecentlyPolled(t *testing.T) {
	st := newFakeStore()
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	old, recent := now.Add(-time.Minute), now.Add(-5*time.Second)
	pOld, pRecent := player("old"), player("recent")
	pOld.LastPolledAt, pRecent.LastPolledAt = &old, &recent
	st.watch(1, pOld)
	st.watch(1, pRecent)
	e := New(testConfig(), st, newFakeProfiles(), always(status.Status{}), &fakeHub{}, quietLog())
	e.now = func() time.Time { return now }

	if err := e.RequestRefresh(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if e.entries["old"] == nil || e.entries["old"].manualAt.IsZero() {
		t.Error("old player should be marked for manual refresh")
	}
	if e.entries["recent"] == nil || !e.entries["recent"].manualAt.IsZero() {
		t.Error("recently polled player should be skipped")
	}
}

func TestActiveEmblem(t *testing.T) {
	p := &bungie.Profile{}
	p.Characters.Data = map[string]bungie.Character{
		"c1": {EmblemPath: "/1.jpg", EmblemBackgroundPath: "/1bg.jpg", DateLastPlayed: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		"c2": {EmblemPath: "/2.jpg", EmblemBackgroundPath: "/2bg.jpg", DateLastPlayed: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)},
	}
	// No activity data: fall back to the most recently played character.
	if e, bg := activeEmblem(p); e != "/2.jpg" || bg != "/2bg.jpg" {
		t.Errorf("fallback = %q, %q", e, bg)
	}
	p.CharacterActivities.Data = map[string]bungie.CharacterActivities{
		"c1": {DateActivityStarted: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)},
	}
	if e, _ := activeEmblem(p); e != "/1.jpg" {
		t.Errorf("active = %q", e)
	}
	if e, bg := activeEmblem(nil); e != "" || bg != "" {
		t.Error("nil profile should give empty paths")
	}
}

// --- hardening tests ---

func TestRequestRefresh_PollsAlreadyPolledPlayer(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	cfg := testConfig()
	cfg.RefreshSkip, cfg.RefreshCooldown = 0, 0
	e := New(cfg, st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	run(t, e)

	eventually(t, "first poll", func() bool { return pr.callCount("p1") == 1 })
	eventually(t, "manual poll", func() bool {
		_ = e.RequestRefresh(context.Background(), 1)
		return pr.callCount("p1") >= 2
	})
}

func TestRequestRefresh_PollsForInactiveUser(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{} // nobody active
	st.watch(1, player("p1"))
	e := New(testConfig(), st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	run(t, e)

	if err := e.RequestRefresh(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	eventually(t, "manual poll of inactive user's player", func() bool { return pr.callCount("p1") >= 1 })
}

func TestRequestRefresh_StoreErrorDoesNotConsumeCooldown(t *testing.T) {
	st := newFakeStore()
	st.watch(1, player("p1"))
	e := New(testConfig(), st, newFakeProfiles(), always(status.Status{}), &fakeHub{}, quietLog())
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	e.now = func() time.Time { return now }

	st.mu.Lock()
	st.err = errors.New("db down")
	st.mu.Unlock()
	if err := e.RequestRefresh(context.Background(), 1); err == nil {
		t.Fatal("want error")
	}
	st.mu.Lock()
	st.err = nil
	st.mu.Unlock()
	if err := e.RequestRefresh(context.Background(), 1); err != nil {
		t.Errorf("cooldown consumed by failed refresh: %v", err)
	}
}

func TestRequestRefresh_SkipsInFlight(t *testing.T) {
	st := newFakeStore()
	st.watch(1, player("p1"))
	e := New(testConfig(), st, newFakeProfiles(), always(status.Status{}), &fakeHub{}, quietLog())
	e.entries["p1"] = &entry{player: player("p1"), inFlight: true}
	if err := e.RequestRefresh(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if !e.entries["p1"].manualAt.IsZero() {
		t.Error("in-flight player must not be queued again")
	}
}

func TestRecordHealth_ConsecutiveFailuresGoDownAndDelayProbe(t *testing.T) {
	e := New(testConfig(), newFakeStore(), newFakeProfiles(), always(status.Status{}), &fakeHub{}, quietLog())
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	for i := 0; i < downAfterFailures; i++ {
		e.recordHealth(now, &bungie.TransportError{Path: "/x", Err: errors.New("reset")})
	}
	if e.APIState() != APIDown {
		t.Fatalf("state = %s, want down", e.APIState())
	}
	if !e.lastProbe.Equal(now) {
		t.Errorf("lastProbe = %v, want %v", e.lastProbe, now)
	}
}

func TestRecordHealth_IgnoresThrottlingAndNotFound(t *testing.T) {
	h := &fakeHub{}
	e := New(testConfig(), newFakeStore(), newFakeProfiles(), always(status.Status{}), h, quietLog())
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		e.recordHealth(now, &bungie.APIError{Code: 36, ThrottleSeconds: 5})
		e.recordHealth(now, &bungie.APIError{Code: bungie.CodeAccountNotFound})
	}
	if e.APIState() != APIOK || len(h.broadcastEvents()) != 0 {
		t.Errorf("state = %s, broadcasts = %d", e.APIState(), len(h.broadcastEvents()))
	}
}

func TestEngine_RecoversFromDown(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("a"))
	st.watch(1, player("b"))
	cfg := testConfig()
	cfg.DownProbe = 10 * time.Millisecond
	e := New(cfg, st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	e.apiState = APIDown // before Run starts
	e.health.down = true
	run(t, e)

	eventually(t, "ok broadcast", func() bool {
		for _, ev := range h.broadcastEvents() {
			if u, ok := ev.Data.(APIStatusUpdate); ok && u.Bungie == APIOK {
				return true
			}
		}
		return false
	})
	eventually(t, "both players polled", func() bool { return pr.callCount("a") >= 1 && pr.callCount("b") >= 1 })
}

func TestEngine_FiveFailuresGoDown(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		st.watch(1, player(id))
		pr.errs[id] = &bungie.TransportError{Path: "/x", Err: errors.New("reset")}
	}
	e := New(testConfig(), st, pr, always(status.Status{}), h, quietLog())
	run(t, e)
	eventually(t, "down", func() bool { return e.APIState() == APIDown })
}

func TestNew_ClampsZeroConfig(t *testing.T) {
	st, pr, h := newFakeStore(), newFakeProfiles(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	e := New(Config{}, st, pr, always(status.Status{State: status.InMatch}), h, quietLog())
	if e.cfg.Workers < 1 || e.cfg.Tick <= 0 {
		t.Fatalf("cfg = %+v", e.cfg)
	}
	run(t, e)
	eventually(t, "poll with zero config", func() bool { return st.count(st.saves, "p1") >= 1 })
}

// blockingProfiles blocks every call until release is closed or ctx is done.
type blockingProfiles struct {
	release chan struct{}
	mu      sync.Mutex
	cur     int
	max     int
	calls   int
}

func (b *blockingProfiles) GetProfile(ctx context.Context, _ int64, _ string) (*bungie.Profile, error) {
	b.mu.Lock()
	b.calls++
	b.cur++
	b.max = max(b.max, b.cur)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.cur--
		b.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.release:
		return &bungie.Profile{}, nil
	}
}

func (b *blockingProfiles) stats() (calls, cur, max int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls, b.cur, b.max
}

func TestEngine_ShutdownWithPollsInFlight(t *testing.T) {
	st, h := newFakeStore(), &fakeHub{active: []int64{1}}
	st.watch(1, player("p1"))
	bp := &blockingProfiles{release: make(chan struct{})}
	e := New(testConfig(), st, bp, always(status.Status{State: status.InMatch}), h, quietLog())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(done)
	}()
	eventually(t, "poll in flight", func() bool { _, cur, _ := bp.stats(); return cur == 1 })
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if st.count(st.saves, "p1") != 0 || st.count(st.failures, "p1") != 0 || len(h.sentEvents()) != 0 {
		t.Error("cancelled poll must not save or publish")
	}
}

func TestEngine_WorkerConcurrencyCap(t *testing.T) {
	st, h := newFakeStore(), &fakeHub{active: []int64{1}}
	for _, id := range []string{"a", "b", "c", "d", "e", "f"} {
		st.watch(1, player(id))
	}
	bp := &blockingProfiles{release: make(chan struct{})}
	e := New(testConfig(), st, bp, always(status.Status{State: status.InMatch}), h, quietLog())
	run(t, e)

	eventually(t, "two concurrent polls", func() bool { _, cur, _ := bp.stats(); return cur == 2 })
	close(bp.release)
	eventually(t, "all polled", func() bool { c, _, _ := bp.stats(); return c == 6 })
	if _, _, m := bp.stats(); m > 2 {
		t.Errorf("max concurrent = %d, want <= 2", m)
	}
}
