package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
)

type historyCall struct {
	token, membershipID, characterID string
	mode, count                      int
}

// fakeHistory serves per-character activity history.
type fakeHistory struct {
	mu    sync.Mutex
	acts  map[string][]bungie.HistoryActivity // by character ID
	errs  map[string]error                    // by character ID
	block bool                                // wait for ctx to be cancelled
	calls []historyCall
}

func newFakeHistory() *fakeHistory {
	return &fakeHistory{acts: map[string][]bungie.HistoryActivity{}, errs: map[string]error{}}
}

func (f *fakeHistory) ActivityHistory(ctx context.Context, token string, _ int64, membershipID, characterID string, mode, count int) ([]bungie.HistoryActivity, error) {
	f.mu.Lock()
	f.calls = append(f.calls, historyCall{token, membershipID, characterID, mode, count})
	block := f.block
	acts, err := f.acts[characterID], f.errs[characterID]
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return acts, err
}

func (f *fakeHistory) callsFor(membershipID string) []historyCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []historyCall
	for _, c := range f.calls {
		if c.membershipID == membershipID {
			out = append(out, c)
		}
	}
	return out
}

func pvp(start time.Time, dur time.Duration, playlist uint32) bungie.HistoryActivity {
	a := bungie.HistoryActivity{Period: start, ActivityDetails: bungie.ActivityDetails{DirectorActivityHash: playlist, Mode: 84}}
	a.Values = map[string]bungie.PGCRValue{"activityDurationSeconds": {}}
	v := a.Values["activityDurationSeconds"]
	v.Basic.Value = dur.Seconds()
	a.Values["activityDurationSeconds"] = v
	return a
}

// withCharacters builds a profile whose characters were last played at the given times.
func withCharacters(lastPlayed map[string]time.Time) *bungie.Profile {
	p := &bungie.Profile{}
	p.Characters.Privacy = bungie.PrivacyPublic
	p.Characters.Data = map[string]bungie.Character{}
	for id, t := range lastPlayed {
		p.Characters.Data[id] = bungie.Character{CharacterID: id, DateLastPlayed: t}
	}
	return p
}

// keepPvP returns st plus whatever last-PvP memory the engine handed the classifier.
func keepPvP(st status.Status) classifyFunc {
	return func(_ *bungie.Profile, prev status.Status, _ time.Time) (status.Status, bool) {
		st.LastPvPEndedAt, st.LastPvPPlaylist = prev.LastPvPEndedAt, prev.LastPvPPlaylist
		return st, true
	}
}

var t0 = time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)

type coldStartRig struct {
	st *fakeStore
	pr *fakeProfiles
	hi *fakeHistory
	e  *Engine
}

func newColdStartRig(c classifyFunc) *coldStartRig {
	r := &coldStartRig{st: newFakeStore(), pr: newFakeProfiles(), hi: newFakeHistory()}
	r.e = New(testConfig(), r.st, r.pr, r.hi, c, &fakeHub{active: []int64{1}}, quietLog())
	r.e.now = func() time.Time { return t0 }
	return r
}

// pollAll refreshes the active set and synchronously polls every player due (or every player,
// after forcing them due).
func (r *coldStartRig) pollAll(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := r.e.refreshActiveSet(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.e.mu.Lock()
	for _, en := range r.e.entries {
		en.lastPolled = time.Time{}
	}
	r.e.mu.Unlock()
	jobs := make(chan string, 16)
	r.e.dispatch(jobs)
	close(jobs)
	for id := range jobs {
		r.e.poll(ctx, id)
	}
}

func TestColdStart_SeedsLatestPvPFromUpToThreeCharacters(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}))
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{
		"c1": t0.Add(-time.Hour), "c2": t0.Add(-2 * time.Hour), "c3": t0.Add(-3 * time.Hour),
		"c4": t0.Add(-24 * time.Hour), // least recently played: skipped
	})
	// c1 started later but c2 ran longer and ended last: the end time decides.
	r.hi.acts["c1"] = []bungie.HistoryActivity{pvp(t0.Add(-20*time.Minute), 5*time.Minute, 200)}
	r.hi.acts["c2"] = []bungie.HistoryActivity{pvp(t0.Add(-30*time.Minute), 27*time.Minute, 300)}
	r.hi.acts["c4"] = []bungie.HistoryActivity{pvp(t0.Add(-2*time.Minute), time.Minute, 400)}

	r.pollAll(t, context.Background())

	calls := r.hi.callsFor("p1")
	if len(calls) != 3 {
		t.Fatalf("history calls = %+v, want 3", calls)
	}
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c.characterID] = true
		if c.mode != bungie.ModeAllPvP || c.count != 1 || c.token != "" {
			t.Errorf("call = %+v, want AllPvP, count 1, no token", c)
		}
	}
	if !seen["c1"] || !seen["c2"] || !seen["c3"] {
		t.Errorf("characters = %v, want the 3 most recently played", seen)
	}
	got := r.st.get("p1").Status
	want := t0.Add(-3 * time.Minute)
	if got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(want) || got.LastPvPPlaylist != "playlist 300" {
		t.Errorf("saved last PvP = %v %q, want %v %q", got.LastPvPEndedAt, got.LastPvPPlaylist, want, "playlist 300")
	}
	if got.State != status.NotInPvP {
		t.Errorf("state = %s", got.State)
	}
}

func TestColdStart_SeededBeforeClassifying(t *testing.T) {
	var gotPrev status.Status
	r := newColdStartRig(func(_ *bungie.Profile, prev status.Status, _ time.Time) (status.Status, bool) {
		gotPrev = prev
		return status.Status{State: status.LikelyQueuing, LastPvPEndedAt: prev.LastPvPEndedAt, LastPvPPlaylist: prev.LastPvPPlaylist}, true
	})
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
	r.hi.acts["c1"] = []bungie.HistoryActivity{pvp(t0.Add(-10*time.Minute), 8*time.Minute, 200)}

	r.pollAll(t, context.Background())

	if gotPrev.LastPvPEndedAt == nil || !gotPrev.LastPvPEndedAt.Equal(t0.Add(-2*time.Minute)) || gotPrev.State != status.Unknown {
		t.Errorf("classifier saw prev = %+v, want the seeded end on an Unknown state", gotPrev)
	}
}

func TestColdStart_CalledOncePerProcessEvenWhenNothingFound(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}))
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})

	r.pollAll(t, context.Background())
	r.pollAll(t, context.Background())

	// Leaving and re-entering the active set does not retry either.
	r.e.hub.(*fakeHub).active = nil
	r.pollAll(t, context.Background())
	r.e.hub.(*fakeHub).active = []int64{1}
	r.pollAll(t, context.Background())

	if n := r.pr.callCount("p1"); n != 3 {
		t.Fatalf("profile polls = %d, want 3", n)
	}
	if calls := r.hi.callsFor("p1"); len(calls) != 1 {
		t.Errorf("history calls = %d, want 1", len(calls))
	}
	if got := r.st.get("p1").Status; got.LastPvPEndedAt != nil {
		t.Errorf("LastPvPEndedAt = %v, want nil (no PvP found)", got.LastPvPEndedAt)
	}
}

func TestColdStart_SkippedWhenLastPvPKnown(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}))
	p := player("p1")
	ended := t0.Add(-time.Hour)
	p.Status = status.Status{State: status.Offline, LastPvPEndedAt: &ended, LastPvPPlaylist: "Control"}
	r.st.watch(1, p)
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
	r.hi.acts["c1"] = []bungie.HistoryActivity{pvp(t0.Add(-10*time.Minute), 8*time.Minute, 200)}

	r.pollAll(t, context.Background())
	r.pollAll(t, context.Background())

	if calls := r.hi.callsFor("p1"); len(calls) != 0 {
		t.Errorf("history calls = %+v, want none", calls)
	}
	if got := r.st.get("p1").Status; got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(ended) || got.LastPvPPlaylist != "Control" {
		t.Errorf("last PvP = %v %q, want the stored one", got.LastPvPEndedAt, got.LastPvPPlaylist)
	}
}

func TestColdStart_HistoryErrorsDoNotFailThePoll(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.Offline}))
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0, "c2": t0.Add(-time.Hour), "c3": t0.Add(-2 * time.Hour)})
	r.hi.errs["c1"] = &bungie.TransportError{Path: "/x", Err: errors.New("reset")}
	r.hi.errs["c2"] = &bungie.APIError{Code: 1665, Status: "DestinyPrivacyRestriction"}
	r.hi.acts["c3"] = []bungie.HistoryActivity{pvp(t0.Add(-3*time.Hour), 10*time.Minute, 200)}

	r.pollAll(t, context.Background())

	if len(r.hi.callsFor("p1")) != 3 {
		t.Errorf("history calls = %d, want 3 (a failing character is skipped)", len(r.hi.callsFor("p1")))
	}
	if r.st.count(r.st.saves, "p1") != 1 || r.st.count(r.st.failures, "p1") != 0 {
		t.Errorf("saves=%d failures=%d, want a successful poll", r.st.count(r.st.saves, "p1"), r.st.count(r.st.failures, "p1"))
	}
	got := r.st.get("p1").Status
	if got.State != status.Offline || got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(t0.Add(-170*time.Minute)) {
		t.Errorf("status = %+v, want OFFLINE seeded from c3", got)
	}
}

func TestColdStart_HistoryFailuresDoNotAffectAPIHealth(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}))
	for i := range 8 {
		id := fmt.Sprintf("p%d", i)
		r.st.watch(1, player(id))
		r.pr.profiles[id] = withCharacters(map[string]time.Time{"c" + id: t0})
		r.hi.errs["c"+id] = &bungie.TransportError{Path: "/x", Err: errors.New("reset")}
	}
	r.pollAll(t, context.Background())

	for i := range 8 {
		id := fmt.Sprintf("p%d", i)
		if len(r.hi.callsFor(id)) != 1 || r.st.count(r.st.saves, id) != 1 {
			t.Errorf("%s: history calls=%d saves=%d", id, len(r.hi.callsFor(id)), r.st.count(r.st.saves, id))
		}
	}
	if got := r.e.APIState(); got != APIOK {
		t.Errorf("api state = %s, want ok: history failures are not API health samples", got)
	}
}

func TestColdStart_SkippedForNotFoundPrivateAndNoCharacters(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP}))
		r.st.watch(1, player("gone"))
		r.pr.errs["gone"] = &bungie.APIError{Code: bungie.CodeAccountNotFound}
		r.pollAll(t, context.Background())
		if r.st.get("gone").Status.State != status.NotFound || len(r.hi.callsFor("gone")) != 0 {
			t.Errorf("state=%s history calls=%d", r.st.get("gone").Status.State, len(r.hi.callsFor("gone")))
		}
	})
	t.Run("private", func(t *testing.T) {
		r := newColdStartRig(keepPvP(status.Status{State: status.Private}))
		r.st.watch(1, player("p1"))
		r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
		r.hi.acts["c1"] = []bungie.HistoryActivity{pvp(t0.Add(-10*time.Minute), 8*time.Minute, 200)}
		r.pollAll(t, context.Background())
		if len(r.hi.callsFor("p1")) != 0 || r.st.get("p1").Status.LastPvPEndedAt != nil {
			t.Errorf("history calls=%d status=%+v, want none for a private player", len(r.hi.callsFor("p1")), r.st.get("p1").Status)
		}
	})
	t.Run("private, then public, still gets its one call", func(t *testing.T) {
		st := status.Status{State: status.Private}
		r := newColdStartRig(func(_ *bungie.Profile, prev status.Status, _ time.Time) (status.Status, bool) {
			out := st
			out.LastPvPEndedAt, out.LastPvPPlaylist = prev.LastPvPEndedAt, prev.LastPvPPlaylist
			return out, true
		})
		r.st.watch(1, player("p1"))
		r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
		r.pollAll(t, context.Background())
		st = status.Status{State: status.Offline}
		r.pollAll(t, context.Background())
		if len(r.hi.callsFor("p1")) != 1 {
			t.Errorf("history calls = %d, want 1 once the player is public", len(r.hi.callsFor("p1")))
		}
	})
	t.Run("no characters", func(t *testing.T) {
		r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP}))
		r.st.watch(1, player("p1"))
		r.pollAll(t, context.Background()) // fakeProfiles' default profile has no characters
		if len(r.hi.callsFor("p1")) != 0 || r.st.count(r.st.saves, "p1") != 1 {
			t.Errorf("history calls=%d saves=%d", len(r.hi.callsFor("p1")), r.st.count(r.st.saves, "p1"))
		}
	})
	t.Run("undecidable profile", func(t *testing.T) {
		r := newColdStartRig(undecidable())
		r.st.watch(1, player("p1"))
		r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
		r.pollAll(t, context.Background())
		if len(r.hi.callsFor("p1")) != 0 {
			t.Errorf("history calls = %d, want none for an undecidable poll", len(r.hi.callsFor("p1")))
		}
	})
}

func TestColdStart_InMatchSeedsThePreviousMatch(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.InMatch, Playlist: "Trials"}))
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
	r.hi.acts["c1"] = []bungie.HistoryActivity{pvp(t0.Add(-20*time.Minute), 9*time.Minute, 200)}

	r.pollAll(t, context.Background())

	got := r.st.get("p1").Status
	if got.State != status.InMatch || got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(t0.Add(-11*time.Minute)) || got.LastPvPPlaylist != "playlist 200" {
		t.Errorf("status = %+v, want IN_MATCH with the previous match recorded", got)
	}
}

func TestColdStart_FutureEndIsClampedToNow(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}))
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0})
	r.hi.acts["c1"] = []bungie.HistoryActivity{pvp(t0.Add(-time.Minute), 10*time.Minute, 200)} // clock skew

	r.pollAll(t, context.Background())

	if got := r.st.get("p1").Status.LastPvPEndedAt; got == nil || !got.Equal(t0) {
		t.Errorf("LastPvPEndedAt = %v, want now", got)
	}
}

func TestColdStart_CancelledDuringHistoryDoesNotSave(t *testing.T) {
	r := newColdStartRig(keepPvP(status.Status{State: status.NotInPvP, Place: status.PlaceOrbit}))
	r.st.watch(1, player("p1"))
	r.pr.profiles["p1"] = withCharacters(map[string]time.Time{"c1": t0, "c2": t0.Add(-time.Hour)})
	r.hi.block = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.pollAll(t, ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for len(r.hi.callsFor("p1")) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("history never called")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("poll did not return after cancel")
	}
	if n := len(r.hi.callsFor("p1")); n != 1 {
		t.Errorf("history calls = %d, want 1 (stop after cancel)", n)
	}
	if r.st.count(r.st.saves, "p1") != 0 || r.st.count(r.st.failures, "p1") != 0 {
		t.Error("cancelled poll must not save")
	}
}
