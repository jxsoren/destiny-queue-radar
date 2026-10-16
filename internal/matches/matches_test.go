package matches

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
)

type fakeBungie struct {
	mu        sync.Mutex
	chars     []string
	history   map[string][]bungie.HistoryActivity // by character
	reports   map[string]*bungie.PGCR
	failPGCR  map[string]bool
	pgcrCalls map[string]int
	gotToken  string
	gotCount  int
}

func (f *fakeBungie) GetProfile(_ context.Context, _ int64, _ string) (*bungie.Profile, error) {
	p := &bungie.Profile{}
	p.Characters.Data = map[string]bungie.Character{}
	for _, c := range f.chars {
		p.Characters.Data[c] = bungie.Character{CharacterID: c}
	}
	return p, nil
}

func (f *fakeBungie) ActivityHistory(_ context.Context, token string, _ int64, _, char string, mode, count int) ([]bungie.HistoryActivity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotToken = token
	f.gotCount = count
	if mode != bungie.ModeAllPvP {
		return nil, errors.New("wrong mode")
	}
	h := f.history[char]
	if len(h) > count {
		h = h[:count]
	}
	return h, nil
}

func (f *fakeBungie) GetPGCR(_ context.Context, id string) (*bungie.PGCR, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pgcrCalls[id]++
	if f.failPGCR[id] {
		return nil, errors.New("boom")
	}
	r, ok := f.reports[id]
	if !ok {
		return nil, errors.New("unknown report")
	}
	return r, nil
}

type lookup struct{}

func (lookup) Activity(h uint32) (manifest.Activity, bool) {
	names := map[uint32]string{100: "Burnout", 200: "Trials of Osiris"}
	n, ok := names[h]
	return manifest.Activity{Hash: h, Name: n}, ok
}

func hist(id string, at time.Time) bungie.HistoryActivity {
	return bungie.HistoryActivity{Period: at, ActivityDetails: bungie.ActivityDetails{InstanceID: id}}
}

func entry(id, name string, code, team int) bungie.PGCREntry {
	e := bungie.PGCREntry{Values: map[string]bungie.PGCRValue{}}
	e.Player.DestinyUserInfo.MembershipType = 3
	e.Player.DestinyUserInfo.MembershipID = id
	e.Player.DestinyUserInfo.BungieGlobalDisplayName = name
	e.Player.DestinyUserInfo.BungieGlobalDisplayNameCode = code
	e.Player.DestinyUserInfo.IconPath = "/" + id + ".jpg"
	if team >= 0 {
		var v bungie.PGCRValue
		v.Basic.Value = float64(team)
		e.Values["team"] = v
	}
	return e
}

func report(id string) *bungie.PGCR {
	return &bungie.PGCR{
		ActivityDetails: bungie.ActivityDetails{ReferenceID: 100, DirectorActivityHash: 200, InstanceID: id},
		Entries: []bungie.PGCREntry{
			entry("b", "Bravo", 2, 18),
			entry("a", "Alpha", 1, 17),
			entry("a", "Alpha", 1, 17), // same player on a second character
			entry("x", "", 0, 18),
		},
	}
}

func newFake() *fakeBungie {
	t0 := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	return &fakeBungie{
		chars: []string{"c1", "c2"},
		history: map[string][]bungie.HistoryActivity{
			"c1": {hist("m3", t0.Add(-1*time.Hour)), hist("m1", t0.Add(-3*time.Hour))},
			"c2": {hist("m2", t0.Add(-2*time.Hour)), hist("m3", t0.Add(-1*time.Hour))},
		},
		reports:   map[string]*bungie.PGCR{"m1": report("m1"), "m2": report("m2"), "m3": report("m3")},
		failPGCR:  map[string]bool{},
		pgcrCalls: map[string]int{},
	}
}

func ids(ms []Match) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.InstanceID)
	}
	return out
}

func TestRecent_MergesCharactersNewestFirstAndDedupes(t *testing.T) {
	f := newFake()
	s := New(f, f, f, lookup{})
	got, err := s.Recent(context.Background(), "tok", 3, "me", 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"m3", "m2", "m1"}; !slices.Equal(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
	if f.gotToken != "tok" {
		t.Errorf("history token = %q", f.gotToken)
	}
	if lim, _ := s.Recent(context.Background(), "tok", 3, "me", 2); !slices.Equal(ids(lim), []string{"m3", "m2"}) {
		t.Errorf("limit 2 = %v", ids(lim))
	}
}

func TestRecent_SkipsReportsThatFail(t *testing.T) {
	f := newFake()
	f.failPGCR["m2"] = true
	got, err := New(f, f, f, lookup{}).Recent(context.Background(), "", 3, "me", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), []string{"m3", "m1"}) {
		t.Errorf("ids = %v", ids(got))
	}
}

func TestMatch_BuildsLobbyAndCaches(t *testing.T) {
	f := newFake()
	s := New(f, f, f, lookup{})
	m, err := s.Match(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if m.InstanceID != "m1" || m.Activity != "Burnout" || m.Playlist != "Trials of Osiris" {
		t.Errorf("match = %+v", m)
	}
	want := []Participant{
		{MembershipType: 3, MembershipID: "a", BungieName: "Alpha#0001", EmblemPath: "/a.jpg", Team: 17},
		{MembershipType: 3, MembershipID: "b", BungieName: "Bravo#0002", EmblemPath: "/b.jpg", Team: 18},
		{MembershipType: 3, MembershipID: "x", BungieName: "", EmblemPath: "/x.jpg", Team: 18},
	}
	if !slices.Equal(m.Players, want) {
		t.Errorf("players = %+v\nwant %+v", m.Players, want)
	}
	s.Match(context.Background(), "m1")
	if f.pgcrCalls["m1"] != 1 {
		t.Errorf("report fetched %d times, want 1 (cached)", f.pgcrCalls["m1"])
	}
}

func TestMatch_CacheEvictsOldest(t *testing.T) {
	f := newFake()
	s := New(f, f, f, lookup{})
	s.cacheSize = 2
	ctx := context.Background()
	s.Match(ctx, "m1")
	s.Match(ctx, "m2")
	s.Match(ctx, "m1") // m1 is now most recently used
	s.Match(ctx, "m3") // evicts m2
	s.Match(ctx, "m1")
	s.Match(ctx, "m2")
	if f.pgcrCalls["m1"] != 1 || f.pgcrCalls["m2"] != 2 {
		t.Errorf("calls = %v, want m1 cached and m2 refetched", f.pgcrCalls)
	}
}

func TestRecent_AllReportsFailReturnsError(t *testing.T) {
	f := newFake()
	f.failPGCR["m1"], f.failPGCR["m2"], f.failPGCR["m3"] = true, true, true
	got, err := New(f, f, f, lookup{}).Recent(context.Background(), "", 3, "me", 10)
	if err == nil || got != nil {
		t.Errorf("got %v, %v; want nil and an error", got, err)
	}
}

func TestRecent_CancelledContextReturnsError(t *testing.T) {
	f := newFake()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(f, f, f, lookup{}).Recent(ctx, "", 3, "me", 10); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRecent_ClampsLimit(t *testing.T) {
	for limit, want := range map[int]int{0: 10, -1: 10, 26: 25, 1000: 25} {
		f := newFake()
		if _, err := New(f, f, f, lookup{}).Recent(context.Background(), "", 3, "me", limit); err != nil {
			t.Fatal(err)
		}
		if f.gotCount != want {
			t.Errorf("limit %d requested %d activities, want %d", limit, f.gotCount, want)
		}
	}
	f := newFake()
	New(f, f, f, lookup{}).Recent(context.Background(), "", 3, "me", 25)
	if f.gotCount != 25 {
		t.Errorf("limit 25 requested %d", f.gotCount)
	}
}

func TestMatch_CallerCannotMutateCache(t *testing.T) {
	f := newFake()
	s := New(f, f, f, lookup{})
	m, _ := s.Match(context.Background(), "m1")
	m.Players[0].BungieName = "Mutated"
	m2, _ := s.Match(context.Background(), "m1")
	if m2.Players[0].BungieName != "Alpha#0001" {
		t.Errorf("cache was mutated: %q", m2.Players[0].BungieName)
	}
}

func TestMatch_UnresolvedNamesAreNotCached(t *testing.T) {
	f := newFake()
	r := report("m9")
	r.ActivityDetails.ReferenceID = 999 // map not in the manifest yet
	f.reports["m9"] = r
	s := New(f, f, f, lookup{})
	m, err := s.Match(context.Background(), "m9")
	if err != nil || m.Activity != "" {
		t.Fatalf("m = %+v, err = %v", m, err)
	}
	s.Match(context.Background(), "m9")
	if f.pgcrCalls["m9"] != 2 {
		t.Errorf("fetched %d times, want 2 (not cached)", f.pgcrCalls["m9"])
	}
}

func TestConcurrentUse(t *testing.T) {
	f := newFake()
	s := New(f, f, f, lookup{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() { defer wg.Done(); s.Match(context.Background(), "m1") }()
		go func() { defer wg.Done(); s.Recent(context.Background(), "", 3, "me", 10) }()
	}
	wg.Wait()
}

func TestMatch_ZeroPlaylistHashIsCached(t *testing.T) {
	f := newFake()
	r := report("m8")
	r.ActivityDetails.DirectorActivityHash = 0
	f.reports["m8"] = r
	s := New(f, f, f, lookup{})
	s.Match(context.Background(), "m8")
	s.Match(context.Background(), "m8")
	if f.pgcrCalls["m8"] != 1 {
		t.Errorf("fetched %d times, want 1 (cached)", f.pgcrCalls["m8"])
	}
}
