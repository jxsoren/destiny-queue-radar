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
		"c1": {DateActivityStarted: started, CurrentActivityHash: 100, CurrentActivityModeHash: 300, CurrentPlaylistActivityHash: 500,
			CurrentActivityModeType: 84, CurrentActivityModeTypes: []int{5, 84}, CurrentActivityModeHashes: []uint32{300, 301}},
		"c2": {DateActivityStarted: started.Add(-72 * time.Hour)},
	}
	ts := started.Add(time.Minute)
	p.ProfileTransitoryData.Privacy = bungie.PrivacyPublic
	p.ProfileTransitoryData.Data = &bungie.TransitoryData{
		PartyMembers:    []bungie.PartyMember{{}, {}, {}},
		CurrentActivity: &bungie.TransitoryActivity{StartTime: &ts, NumberOfPlayers: 6, NumberOfOpponents: 3},
	}
	return p
}

func TestSummarize_OnlineInMatch(t *testing.T) {
	p := onlineProfile()
	p.Characters.Data = map[string]bungie.Character{"c1": {ClassType: 2}}
	s := Summarize("Guardian#1993", p, lookup)
	if s.ClassType != 2 {
		t.Errorf("classType=%d", s.ClassType)
	}
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
	if !s.HasCurrentActivity || s.Players != 6 || s.Opponents != 3 || s.TransitoryStart == nil || s.TransitoryEnd != nil || s.TransitoryPrivacy != bungie.PrivacyPublic {
		t.Errorf("ca=%v players=%d opp=%d start=%v end=%v tpriv=%d", s.HasCurrentActivity, s.Players, s.Opponents, s.TransitoryStart, s.TransitoryEnd, s.TransitoryPrivacy)
	}
	if s.ModeType != 84 || len(s.ModeTypes) != 2 || s.ModeTypes[1] != 84 || len(s.ModeHashes) != 2 {
		t.Errorf("modeType=%d types=%v hashes=%v", s.ModeType, s.ModeTypes, s.ModeHashes)
	}
	out := s.String()
	for _, want := range []string{"Guardian#1993", "online=true", `activity="Burnout"`, "cat=PvP", "priv=act:public/trans:public", "class=Warlock", "ca=true players=6 opp=3 modeType=84"} {
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
	if s.Online || s.PartySize != 0 {
		t.Errorf("online=%v party=%d", s.Online, s.PartySize)
	}
	if s.ActivitiesPrivacy != bungie.PrivacyPrivate || s.CharacterID != "" {
		t.Errorf("privacy=%d char=%q", s.ActivitiesPrivacy, s.CharacterID)
	}
}

func TestSummarize_OnlineNoCurrentActivity(t *testing.T) {
	p := onlineProfile()
	p.ProfileTransitoryData.Data.CurrentActivity = nil
	s := Summarize("X#0001", p, lookup)
	if !s.Online || s.HasCurrentActivity || s.Players != 0 || s.Opponents != 0 || s.TransitoryStart != nil {
		t.Errorf("online=%v ca=%v players=%d", s.Online, s.HasCurrentActivity, s.Players)
	}
}

func TestSummarize_ZeroHashes(t *testing.T) {
	p := onlineProfile()
	p.CharacterActivities.Data = map[string]bungie.CharacterActivities{"c1": {}}
	s := Summarize("X#0001", p, lookup)
	if s.Activity != "" || s.Mode != "" || s.Playlist != "" || s.ModeCategory != manifest.CategoryNone {
		t.Errorf("activity=%q mode=%q playlist=%q cat=%v", s.Activity, s.Mode, s.Playlist, s.ModeCategory)
	}
}

func TestSummary_StringZeroStarted(t *testing.T) {
	out := Summary{Player: "X#0001"}.String()
	if !strings.Contains(out, "started=-") {
		t.Errorf("String() = %q", out)
	}
}
