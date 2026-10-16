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
}

var modes = map[uint32]manifest.Mode{
	84: {Hash: 84, Name: "Trials of Osiris", Category: manifest.CategoryPvP},
	87: {Hash: 87, Name: "Social", Category: manifest.CategoryNone},
	4:  {Hash: 4, Name: "Raid", Category: manifest.CategoryPvE},
}

func (fakeLookup) Activity(h uint32) (manifest.Activity, bool) {
	a, ok := activities[h]
	return a, ok
}

func (fakeLookup) Mode(h uint32) (manifest.Mode, bool) {
	m, ok := modes[h]
	return m, ok
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
		if got := c.Classify(tt.p, status.Status{}, now); !got.Equal(tt.want) {
			t.Errorf("%s:\n got  %+v\n want %+v", tt.name, got, tt.want)
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
		{"match → offline records the end", inTrials, profile(false, 100, 84, 200), status.Offline, &now},
		{"still within window", status.Status{State: status.LikelyQueuing, LastPvPEndedAt: &endedRecently, LastPvPPlaylist: "Trials of Osiris"},
			profile(true, orbitActivityHash, 0, 0), status.LikelyQueuing, &endedRecently},
		{"window expired", status.Status{State: status.LikelyQueuing, LastPvPEndedAt: &endedLongAgo, LastPvPPlaylist: "Trials of Osiris"},
			profile(true, orbitActivityHash, 0, 0), status.NotInPvP, &endedLongAgo},
		{"match → match keeps playing", inTrials, profile(true, 100, 84, 200), status.InMatch, nil},
	}
	for _, tt := range tests {
		got := c.Classify(tt.p, tt.prev, now)
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
	got := c.Classify(profile(true, orbitActivityHash, 0, 0), prev, now)
	if got.LastPvPPlaylist != "Trials of Osiris" {
		t.Errorf("LastPvPPlaylist = %q, want the mode name", got.LastPvPPlaylist)
	}
}
