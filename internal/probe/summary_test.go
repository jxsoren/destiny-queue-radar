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
	s := Summarize("Kwivor#1993", onlineProfile(), lookup)
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
	for _, want := range []string{"Kwivor#1993", "online=true", `activity="Burnout"`, "cat=PvP"} {
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
