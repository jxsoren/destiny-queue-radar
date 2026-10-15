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
