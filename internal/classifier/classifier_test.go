package classifier

import (
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
)

type fakeLookup struct{}

var activities = map[uint32]manifest.Activity{
	100:               {Hash: 100, Name: "Burnout", IsPvP: true},
	200:               {Hash: 200, Name: "Trials of Osiris", IsPvP: true},
	orbitActivityHash: {Hash: orbitActivityHash, Name: "Orbit"},
	300:               {Hash: 300, Name: "The Tower"},
	400:               {Hash: 400, Name: "Salvation's Edge"},
	500:               {Hash: 500, Name: "Mystery PvP", IsPvP: true},
	600:               {Hash: 600, Name: "Strange Place"},
}

var modes = map[uint32]manifest.Mode{
	84: {Hash: 84, Name: "Trials of Osiris", ModeType: 84, Category: manifest.CategoryPvP},
	87: {Hash: 87, Name: "Social", ModeType: 87, Category: manifest.CategoryNone},
	4:  {Hash: 4, Name: "Raid", ModeType: 4, Category: manifest.CategoryPvE},
	5:  {Hash: 5, Name: "AllPvP", ModeType: 5, Category: manifest.CategoryPvP},
	71: {Hash: 71, Name: "Clash: Quickplay", ModeType: 71, Category: manifest.CategoryPvP},
}

func (fakeLookup) Activity(h uint32) (manifest.Activity, bool) {
	a, ok := activities[h]
	return a, ok
}

func (fakeLookup) Mode(h uint32) (manifest.Mode, bool) {
	m, ok := modes[h]
	return m, ok
}

func (fakeLookup) ModeByType(t int) (manifest.Mode, bool) {
	for _, m := range modes {
		if m.ModeType == t {
			return m, true
		}
	}
	return manifest.Mode{}, false
}

var (
	now     = time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	started = now.Add(-4 * time.Minute)
)

// profile builds a profile with one character doing the given activity.
func profile(online bool, act, mode, playlist uint32) *bungie.Profile {
	p := &bungie.Profile{}
	p.CharacterActivities.Privacy = bungie.PrivacyPublic
	p.CharacterActivities.Data = map[string]bungie.CharacterActivities{
		"c1": {DateActivityStarted: started, CurrentActivityHash: act, CurrentActivityModeHash: mode, CurrentPlaylistActivityHash: playlist},
	}
	p.ProfileTransitoryData.Privacy = bungie.PrivacyPublic
	if online {
		p.ProfileTransitoryData.Data = &bungie.TransitoryData{}
	}
	return p
}

// classify runs Classify and fails the test if the profile was undecidable.
func classify(t *testing.T, c *Classifier, p *bungie.Profile, prev status.Status, at time.Time) status.Status {
	t.Helper()
	got, ok := c.Classify(p, prev, at)
	if !ok {
		t.Fatalf("Classify reported undecidable, want ok")
	}
	return got
}

func TestClassify_CurrentState(t *testing.T) {
	c := New(fakeLookup{})

	privateActs := profile(false, 0, 0, 0)
	privateActs.CharacterActivities.Privacy = bungie.PrivacyPrivate
	privateActs.CharacterActivities.Data = nil

	privateTransitory := profile(false, 100, 84, 200)
	privateTransitory.ProfileTransitoryData.Privacy = bungie.PrivacyPrivate

	flaggedButVisible := profile(true, 100, 84, 200) // smoke test: privacy=2 with data present
	flaggedButVisible.CharacterActivities.Privacy = bungie.PrivacyPrivate

	tests := []struct {
		name string
		p    *bungie.Profile
		want status.Status
	}{
		{"private activities, no data", privateActs, status.Status{State: status.Private}},
		{"private transitory", privateTransitory, status.Status{State: status.Private}},
		{"offline with stale PvP activity", profile(false, 100, 84, 200), status.Status{State: status.Offline}},
		{"in trials", profile(true, 100, 84, 200),
			status.Status{State: status.InMatch, Activity: "Burnout", Mode: "Trials of Osiris", Playlist: "Trials of Osiris", StartedAt: &started}},
		{"privacy flag but data visible", flaggedButVisible,
			status.Status{State: status.InMatch, Activity: "Burnout", Mode: "Trials of Osiris", Playlist: "Trials of Osiris", StartedAt: &started}},
		{"unknown mode, PvP activity", profile(true, 500, 999, 0),
			status.Status{State: status.InMatch, Activity: "Mystery PvP", StartedAt: &started}},
		{"orbit", profile(true, orbitActivityHash, 0, 0), status.Status{State: status.NotInPvP, Place: status.PlaceOrbit, Activity: "Orbit"}},
		{"no activity hash", profile(true, 0, 0, 0), status.Status{State: status.NotInPvP, Place: status.PlaceOrbit, Activity: "Orbit"}},
		{"tower", profile(true, 300, 87, 0), status.Status{State: status.NotInPvP, Place: status.PlaceSocial, Activity: "The Tower", Mode: "Social"}},
		{"raid", profile(true, 400, 4, 0), status.Status{State: status.NotInPvP, Place: status.PlacePvE, Activity: "Salvation's Edge", Mode: "Raid"}},
		{"online, no characters", func() *bungie.Profile { p := profile(true, 0, 0, 0); p.CharacterActivities.Data = nil; return p }(),
			status.Status{State: status.NotInPvP}},
	}
	for _, tt := range tests {
		if got := classify(t, c, tt.p, status.Status{}, now); !got.Equal(tt.want) {
			t.Errorf("%s:\n got  %+v\n want %+v", tt.name, got, tt.want)
		}
	}
}

// queued is a player in Orbit whose transitory activity counts a matchmaking lobby.
func queued(players int) *bungie.Profile {
	p := profile(true, orbitActivityHash, 0, 0)
	p.ProfileTransitoryData.Data.CurrentActivity = &bungie.TransitoryActivity{NumberOfPlayers: players}
	return p
}

func TestClassify_Queued(t *testing.T) {
	c := New(fakeLookup{})
	tower := profile(true, 300, 87, 0)
	tower.ProfileTransitoryData.Data.CurrentActivity = &bungie.TransitoryActivity{NumberOfPlayers: 12}
	idle := profile(true, orbitActivityHash, 0, 0)
	idle.ProfileTransitoryData.Data.CurrentActivity = &bungie.TransitoryActivity{}

	tests := []struct {
		name string
		p    *bungie.Profile
		want status.State
	}{
		{"orbit, lobby of 1", queued(1), status.Queued},
		{"orbit, lobby of 9", queued(9), status.Queued},
		{"orbit, no players", idle, status.NotInPvP},
		{"orbit, no transitory activity", profile(true, orbitActivityHash, 0, 0), status.NotInPvP},
		{"tower counts its own instance", tower, status.NotInPvP},
	}
	for _, tt := range tests {
		if got := classify(t, c, tt.p, status.Status{}, now); got.State != tt.want {
			t.Errorf("%s: state = %s, want %s", tt.name, got.State, tt.want)
		}
	}
	if got := classify(t, c, queued(1), status.Status{}, now); got.Place != status.PlaceOrbit || got.Activity != "Orbit" {
		t.Errorf("queued place=%s activity=%q, want ORBIT/Orbit", got.Place, got.Activity)
	}
}

// Bungie sometimes omits transitory data for a player in a match (seen once, Supremacy).
func TestClassify_MatchWithoutTransitory(t *testing.T) {
	c := New(fakeLookup{})
	orbit := status.Status{State: status.NotInPvP, Place: status.PlaceOrbit, Activity: "Orbit"}
	old := profile(false, 100, 84, 200)
	old.CharacterActivities.Data["c1"] = bungie.CharacterActivities{DateActivityStarted: now.Add(-31 * time.Minute), CurrentActivityHash: 100, CurrentActivityModeHash: 84}

	tests := []struct {
		name string
		prev status.Status
		p    *bungie.Profile
		want status.State
	}{
		{"queued → match", status.Status{State: status.Queued}, profile(false, 100, 84, 200), status.InMatch},
		{"orbit → match", orbit, profile(false, 100, 84, 200), status.InMatch},
		{"likely queuing → match", status.Status{State: status.LikelyQueuing}, profile(false, 100, 84, 200), status.InMatch},
		{"match too old to trust", orbit, old, status.Offline},
		{"already offline", status.Status{State: status.Offline}, profile(false, 100, 84, 200), status.Offline},
		{"first poll", status.Status{}, profile(false, 100, 84, 200), status.Offline},
		{"orbit activity", orbit, profile(false, orbitActivityHash, 0, 0), status.Offline},
	}
	for _, tt := range tests {
		got := classify(t, c, tt.p, tt.prev, now)
		if got.State != tt.want {
			t.Errorf("%s: state = %s, want %s", tt.name, got.State, tt.want)
		}
		if tt.want == status.InMatch && (got.StartedAt == nil || !got.StartedAt.Equal(started) || got.Playlist != "Trials of Osiris") {
			t.Errorf("%s: %+v, want the match's details", tt.name, got)
		}
	}
}

func TestClassify_Transitions(t *testing.T) {
	c := New(fakeLookup{})
	inTrials := status.Status{State: status.InMatch, Activity: "Burnout", Mode: "Trials of Osiris", Playlist: "Trials of Osiris", StartedAt: &started}
	endedRecently := now.Add(-2 * time.Minute)
	endedLongAgo := now.Add(-6 * time.Minute)

	tests := []struct {
		name      string
		prev      status.Status
		p         *bungie.Profile
		wantState status.State
		wantEnded *time.Time
	}{
		{"match → orbit is likely queuing", inTrials, profile(true, orbitActivityHash, 0, 0), status.LikelyQueuing, &now},
		{"match → tower is likely queuing", inTrials, profile(true, 300, 87, 0), status.LikelyQueuing, &now},
		{"match → PvE is not queuing", inTrials, profile(true, 400, 4, 0), status.NotInPvP, &now},
		{"match → offline records the end", inTrials, profile(false, orbitActivityHash, 0, 0), status.Offline, &now},
		{"match without transitory stays in match", inTrials, profile(false, 100, 84, 200), status.InMatch, nil},
		{"match → queued records the end", inTrials, queued(1), status.Queued, &now},
		{"queued → orbit with a recent match is likely queuing", status.Status{State: status.Queued, LastPvPEndedAt: &endedRecently, LastPvPPlaylist: "Trials of Osiris"},
			profile(true, orbitActivityHash, 0, 0), status.LikelyQueuing, &endedRecently},
		{"still within window", status.Status{State: status.LikelyQueuing, LastPvPEndedAt: &endedRecently, LastPvPPlaylist: "Trials of Osiris"},
			profile(true, orbitActivityHash, 0, 0), status.LikelyQueuing, &endedRecently},
		{"window expired", status.Status{State: status.LikelyQueuing, LastPvPEndedAt: &endedLongAgo, LastPvPPlaylist: "Trials of Osiris"},
			profile(true, orbitActivityHash, 0, 0), status.NotInPvP, &endedLongAgo},
		{"match → match keeps playing", inTrials, profile(true, 100, 84, 200), status.InMatch, nil},
	}
	for _, tt := range tests {
		got := classify(t, c, tt.p, tt.prev, now)
		if got.State != tt.wantState {
			t.Errorf("%s: state = %s, want %s", tt.name, got.State, tt.wantState)
		}
		switch {
		case tt.wantEnded == nil && got.LastPvPEndedAt != nil:
			t.Errorf("%s: LastPvPEndedAt = %v, want nil", tt.name, got.LastPvPEndedAt)
		case tt.wantEnded != nil && (got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(*tt.wantEnded)):
			t.Errorf("%s: LastPvPEndedAt = %v, want %v", tt.name, got.LastPvPEndedAt, *tt.wantEnded)
		}
		if tt.wantEnded != nil && got.LastPvPPlaylist != "Trials of Osiris" {
			t.Errorf("%s: LastPvPPlaylist = %q", tt.name, got.LastPvPPlaylist)
		}
	}
}

func TestClassify_EndedMatchFallsBackToModeName(t *testing.T) {
	c := New(fakeLookup{})
	prev := status.Status{State: status.InMatch, Mode: "Trials of Osiris"} // no playlist resolved
	got := classify(t, c, profile(true, orbitActivityHash, 0, 0), prev, now)
	if got.LastPvPPlaylist != "Trials of Osiris" {
		t.Errorf("LastPvPPlaylist = %q, want the mode name", got.LastPvPPlaylist)
	}
}

func TestClassify_Hardening(t *testing.T) {
	c := New(fakeLookup{})
	inTrials := status.Status{State: status.InMatch, Activity: "Burnout", Mode: "Trials of Osiris", Playlist: "Trials of Osiris", StartedAt: &started}

	t.Run("missing transitory component keeps prev", func(t *testing.T) {
		p := profile(false, 100, 84, 200)
		p.ProfileTransitoryData.Privacy = bungie.PrivacyNone
		got, ok := c.Classify(p, inTrials, now)
		if ok || !got.Equal(inTrials) {
			t.Errorf("got %+v ok=%v, want prev %+v and ok=false", got, ok, inTrials)
		}
	})
	t.Run("missing activities component keeps prev", func(t *testing.T) {
		p := profile(true, 100, 84, 200)
		p.CharacterActivities.Privacy = bungie.PrivacyNone
		p.CharacterActivities.Data = nil
		got, ok := c.Classify(p, inTrials, now)
		if ok || !got.Equal(inTrials) || got.LastPvPEndedAt != nil {
			t.Errorf("got %+v ok=%v, want prev %+v, ok=false, no end", got, ok, inTrials)
		}
	})
	t.Run("older StartedAt does not record an end", func(t *testing.T) {
		p := profile(true, 100, 84, 200)
		a := p.CharacterActivities.Data["c1"]
		a.DateActivityStarted = started.Add(-time.Hour)
		p.CharacterActivities.Data["c1"] = a
		got := classify(t, c, p, inTrials, now)
		if got.LastPvPEndedAt != nil {
			t.Errorf("LastPvPEndedAt = %v, want nil", got.LastPvPEndedAt)
		}
	})
	t.Run("transitory flagged private but data visible", func(t *testing.T) {
		p := profile(true, 100, 84, 200)
		p.ProfileTransitoryData.Privacy = bungie.PrivacyPrivate
		if got := classify(t, c, p, status.Status{}, now); got.State != status.InMatch {
			t.Errorf("state = %s, want IN_MATCH", got.State)
		}
	})
	t.Run("match, PvE, then orbit is not queuing", func(t *testing.T) {
		afterPvE := classify(t, c, profile(true, 400, 4, 0), inTrials, now)
		got := classify(t, c, profile(true, orbitActivityHash, 0, 0), afterPvE, now.Add(time.Minute))
		if got.State != status.NotInPvP {
			t.Errorf("state = %s, want NOT_IN_PVP", got.State)
		}
	})
	t.Run("orbit then tower stays queuing", func(t *testing.T) {
		inOrbit := classify(t, c, profile(true, orbitActivityHash, 0, 0), inTrials, now)
		got := classify(t, c, profile(true, 300, 87, 0), inOrbit, now.Add(time.Minute))
		if got.State != status.LikelyQueuing {
			t.Errorf("state = %s, want LIKELY_QUEUING", got.State)
		}
	})
	t.Run("queuing to match keeps end", func(t *testing.T) {
		ended := now.Add(-time.Minute)
		prev := status.Status{State: status.LikelyQueuing, LastPvPEndedAt: &ended, LastPvPPlaylist: "Trials of Osiris"}
		got := classify(t, c, profile(true, 100, 84, 200), prev, now)
		if got.State != status.InMatch || got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(ended) {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("unknown activity and mode is not social, never queuing", func(t *testing.T) {
		got := classify(t, c, profile(true, 999, 998, 0), inTrials, now)
		if got.State != status.NotInPvP || got.Place != status.PlaceNone {
			t.Errorf("got %+v, want NOT_IN_PVP with no place", got)
		}
	})
	t.Run("parent mode hash resolves to PvP", func(t *testing.T) {
		p := profile(true, 999, 998, 0)
		a := p.CharacterActivities.Data["c1"]
		a.CurrentActivityModeHashes = []uint32{998, 5}
		p.CharacterActivities.Data["c1"] = a
		if got := classify(t, c, p, status.Status{}, now); got.State != status.InMatch {
			t.Errorf("state = %s, want IN_MATCH", got.State)
		}
	})
	t.Run("social-category mode but PvP activity", func(t *testing.T) {
		if got := classify(t, c, profile(true, 500, 87, 0), status.Status{}, now); got.State != status.InMatch {
			t.Errorf("state = %s, want IN_MATCH", got.State)
		}
	})
	t.Run("back-to-back matches record an end", func(t *testing.T) {
		next := now.Add(-time.Minute)
		p := profile(true, 100, 84, 200)
		a := p.CharacterActivities.Data["c1"]
		a.DateActivityStarted = next
		p.CharacterActivities.Data["c1"] = a
		got := classify(t, c, p, inTrials, now)
		if got.State != status.InMatch || got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(next) || got.LastPvPPlaylist != "Trials of Osiris" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("match to private records an end", func(t *testing.T) {
		p := profile(false, 0, 0, 0)
		p.ProfileTransitoryData.Privacy = bungie.PrivacyPrivate
		got := classify(t, c, p, inTrials, now)
		if got.State != status.Private || got.LastPvPEndedAt == nil || !got.LastPvPEndedAt.Equal(now) {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("window boundary is exclusive", func(t *testing.T) {
		ended := now.Add(-5 * time.Minute)
		prev := status.Status{State: status.LikelyQueuing, LastPvPEndedAt: &ended, LastPvPPlaylist: "Trials of Osiris"}
		if got := classify(t, c, profile(true, orbitActivityHash, 0, 0), prev, now); got.State != status.NotInPvP {
			t.Errorf("state = %s, want NOT_IN_PVP", got.State)
		}
	})
}

// Cold start: the engine seeds LastPvPEndedAt from activity history before the first
// classification (prev.State is Unknown).
func TestClassify_ColdStartUsingHistory(t *testing.T) {
	c := New(fakeLookup{})
	seeded := func(state status.State, ago time.Duration) status.Status {
		ended := now.Add(-ago)
		return status.Status{State: state, LastPvPEndedAt: &ended, LastPvPPlaylist: "Trials of Osiris"}
	}
	tests := []struct {
		name      string
		prev      status.Status
		p         *bungie.Profile
		wantState status.State
	}{
		{"ended 2m ago, in orbit, is likely queuing", seeded(status.Unknown, 2*time.Minute), profile(true, orbitActivityHash, 0, 0), status.LikelyQueuing},
		{"ended 2m ago, in the tower, is likely queuing", seeded(status.Unknown, 2*time.Minute), profile(true, 300, 87, 0), status.LikelyQueuing},
		{"ended 6m ago is not queuing", seeded(status.Unknown, 6*time.Minute), profile(true, orbitActivityHash, 0, 0), status.NotInPvP},
		{"ended 2m ago, in PvE, is not queuing", seeded(status.Unknown, 2*time.Minute), profile(true, 400, 4, 0), status.NotInPvP},
		{"ended 2m ago, unknown place, is not queuing", seeded(status.Unknown, 2*time.Minute), profile(true, 999, 998, 0), status.NotInPvP},
		{"ended 2m ago, offline", seeded(status.Unknown, 2*time.Minute), profile(false, 0, 0, 0), status.Offline},
		{"ended 2m ago, back in a match", seeded(status.Unknown, 2*time.Minute), profile(true, 100, 84, 200), status.InMatch},
		{"no history is not queuing", status.Status{}, profile(true, orbitActivityHash, 0, 0), status.NotInPvP},
		// Only a first classification qualifies: a known earlier state that was not PvP stays so.
		{"prev NOT_IN_PVP does not qualify", seeded(status.NotInPvP, 2*time.Minute), profile(true, orbitActivityHash, 0, 0), status.NotInPvP},
		{"prev OFFLINE does not qualify", seeded(status.Offline, 2*time.Minute), profile(true, orbitActivityHash, 0, 0), status.NotInPvP},
		{"prev PRIVATE does not qualify", seeded(status.Private, 2*time.Minute), profile(true, orbitActivityHash, 0, 0), status.NotInPvP},
	}
	for _, tt := range tests {
		got := classify(t, c, tt.p, tt.prev, now)
		if got.State != tt.wantState {
			t.Errorf("%s: state = %s, want %s", tt.name, got.State, tt.wantState)
		}
		if !timePtrEqual(got.LastPvPEndedAt, tt.prev.LastPvPEndedAt) || got.LastPvPPlaylist != tt.prev.LastPvPPlaylist {
			t.Errorf("%s: last PvP = %v %q, want the seeded %v %q", tt.name, got.LastPvPEndedAt, got.LastPvPPlaylist, tt.prev.LastPvPEndedAt, tt.prev.LastPvPPlaylist)
		}
	}
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func TestHistoryPlaylist(t *testing.T) {
	c := New(fakeLookup{})
	act := func(director uint32, mode int) bungie.HistoryActivity {
		return bungie.HistoryActivity{ActivityDetails: bungie.ActivityDetails{DirectorActivityHash: director, Mode: mode}}
	}
	tests := []struct {
		name string
		a    bungie.HistoryActivity
		want string
	}{
		{"playlist from the director activity", act(200, 84), "Trials of Osiris"},
		{"unknown playlist falls back to the mode", act(999, 71), "Clash: Quickplay"},
		{"no playlist hash falls back to the mode", act(0, 71), "Clash: Quickplay"},
		{"nothing resolves", act(999, 999), ""},
	}
	for _, tt := range tests {
		if got := c.HistoryPlaylist(tt.a); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
