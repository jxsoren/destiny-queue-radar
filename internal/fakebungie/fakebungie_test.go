package fakebungie

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/classifier"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
)

const vex = "4611686018400000002"

func client(t *testing.T, s *Server) *bungie.Client {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return bungie.NewClient("fake-key", srv.URL, srv.Client())
}

func TestProfilesClassifyThroughScriptedStates(t *testing.T) {
	ctx := context.Background()
	s := New()
	c := client(t, s)
	man, err := manifest.Load(ctx, c)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	cl := classifier.New(man)
	classify := func(prev status.Status) status.Status {
		t.Helper()
		p, err := c.GetProfile(ctx, 3, vex)
		if err != nil {
			t.Fatalf("profile: %v", err)
		}
		st, ok := cl.Classify(p, prev, time.Now())
		if !ok {
			t.Fatal("profile was undecidable")
		}
		return st
	}

	if st := classify(status.Status{}); st.State != status.NotInPvP || st.Place != status.PlaceOrbit {
		t.Errorf("orbit = %+v", st)
	}
	if err := s.SetState(vex, InMatch); err != nil {
		t.Fatal(err)
	}
	inMatch := classify(status.Status{})
	if inMatch.State != status.InMatch || inMatch.Playlist != "Competitive" || inMatch.Activity != "Burnout" || inMatch.StartedAt == nil {
		t.Errorf("match = %+v", inMatch)
	}
	s.SetState(vex, Orbit)
	if st := classify(inMatch); st.State != status.LikelyQueuing {
		t.Errorf("after the match = %+v, want LIKELY_QUEUING", st)
	}
	s.SetState(vex, PvE)
	if st := classify(status.Status{}); st.State != status.NotInPvP || st.Place != status.PlacePvE {
		t.Errorf("pve = %+v", st)
	}
	s.SetState(vex, Offline)
	if st := classify(status.Status{}); st.State != status.Offline {
		t.Errorf("offline = %+v", st)
	}
	s.SetState(vex, Private)
	if st := classify(status.Status{}); st.State != status.Private {
		t.Errorf("private = %+v", st)
	}

	p, err := c.GetProfile(ctx, 3, vex)
	if err != nil || p.Profile.Data == nil || p.Profile.Data.UserInfo.BungieGlobalDisplayName != "Vex" {
		t.Errorf("profile identity = %+v, %v", p, err)
	}
	if icon, _ := p.ActiveEmblem(); icon == "" {
		t.Error("profile has no emblem")
	}
	if _, err := c.GetProfile(ctx, 3, "999"); !bungie.IsNotFound(err) {
		t.Errorf("unknown player err = %v, want not found", err)
	}
	if err := s.SetState("999", Orbit); err == nil {
		t.Error("SetState on an unknown player should fail")
	}
}

func TestSearch(t *testing.T) {
	c := client(t, New())
	card, err := c.SearchByBungieName(context.Background(), "Vex", 1111)
	if err != nil || card.MembershipID != vex || card.MembershipType != 3 {
		t.Errorf("search = %+v, %v", card, err)
	}
	if _, err := c.SearchByBungieName(context.Background(), "Nobody", 1); !bungie.IsNotFound(err) {
		t.Errorf("miss err = %v", err)
	}
}

func TestRecentMatchesForTheTestUser(t *testing.T) {
	ctx := context.Background()
	c := client(t, New())
	man, err := manifest.Load(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := matches.New(c, c, c, man).Recent(ctx, "e2e-access", SelfType, SelfID, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(ms) != 2 || ms[0].InstanceID != LastMatchID || ms[1].InstanceID != OlderMatchID {
		t.Fatalf("matches = %+v", ms)
	}
	last := ms[0]
	if len(last.Players) != 6 || last.Playlist != "Competitive" || last.Activity != "Burnout" {
		t.Errorf("last match = %+v", last)
	}
	found := false
	for _, p := range last.Players {
		if p.MembershipID == vex && p.BungieName == "Vex#1111" && p.EmblemPath != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("Vex#1111 missing from the last lobby: %+v", last.Players)
	}
}

func TestSetStateOverHTTP(t *testing.T) {
	s := New()
	srv := httptest.NewServer(s)
	defer srv.Close()
	post := func(id, body string) int {
		resp, err := http.Post(srv.URL+"/fake/players/"+id+"/state", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := post(vex, `{"state":"match"}`); code != http.StatusNoContent {
		t.Errorf("set = %d", code)
	}
	if s.state(vex) != InMatch {
		t.Errorf("state = %q", s.state(vex))
	}
	if code := post(vex, `{"state":"dancing"}`); code != http.StatusBadRequest {
		t.Errorf("bad state = %d", code)
	}
	if code := post("999", `{"state":"orbit"}`); code != http.StatusNotFound {
		t.Errorf("unknown player = %d", code)
	}
}

// SetState races with profile and PGCR reads; run with -race.
func TestConcurrentStateChangesAndReads(t *testing.T) {
	s := New()
	srv := httptest.NewServer(s)
	defer srv.Close()
	get := func(path string) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Error(err)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d", path, resp.StatusCode)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				get("/Platform/Destiny2/3/Profile/" + vex + "/?components=100,200,204,1000")
				get("/Platform/Destiny2/Stats/PostGameCarnageReport/" + LastMatchID + "/")
			}
		}()
	}
	for _, st := range []State{InMatch, Orbit, PvE, Offline, Private} {
		for j := 0; j < 20; j++ {
			if err := s.SetState(vex, st); err != nil {
				t.Fatal(err)
			}
		}
	}
	wg.Wait()
}

func TestHistoryHasDurationsForEveryLobbyMember(t *testing.T) {
	ctx := context.Background()
	s := New()
	c := client(t, s)
	vexChar := s.players[vex].characterID()
	acts, err := c.ActivityHistory(ctx, "", 3, vex, vexChar, bungie.ModeAllPvP, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].ActivityDetails.InstanceID != LastMatchID {
		t.Fatalf("Vex's history (count 1) = %+v, want only the last match", acts)
	}
	last := s.lobbies[LastMatchID]
	if acts[0].Duration() != last.duration || !acts[0].EndedAt().Equal(last.period.Add(last.duration)) {
		t.Errorf("duration = %s, ended %s; want %s", acts[0].Duration(), acts[0].EndedAt(), last.duration)
	}
	// A cold start must not make the e2e board's first poll of Vex LIKELY_QUEUING.
	if ago := time.Since(acts[0].EndedAt()); ago <= 5*time.Minute {
		t.Errorf("last match ended %s ago; keep it outside the 5-minute queue window", ago)
	}

	all, err := c.ActivityHistory(ctx, "", SelfType, SelfID, "c", bungie.ModeAllPvP, 10)
	if err != nil || len(all) != 2 || all[0].ActivityDetails.InstanceID != LastMatchID || all[1].ActivityDetails.InstanceID != OlderMatchID {
		t.Errorf("tester's history = %+v, %v", all, err)
	}
	none, err := c.ActivityHistory(ctx, "", 3, "4611686018400000005", "c", bungie.ModeAllPvP, 10)
	if err != nil || len(none) != 1 {
		t.Errorf("Halfmoon's history = %+v, %v; want the one lobby they played", none, err)
	}
	if acts, err := c.ActivityHistory(ctx, "", 3, "4611686018400000007", "c", bungie.ModeAllPvP, 1); err != nil || len(acts) != 1 || acts[0].ActivityDetails.InstanceID != OlderMatchID {
		t.Errorf("Mire's history = %+v, %v", acts, err)
	}
}
