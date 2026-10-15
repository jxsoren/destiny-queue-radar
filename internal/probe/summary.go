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

// Summary is the human-readable view of one profile poll. It is also the
// index.jsonl schema, so field and JSON names are part of the recorded format.
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

	TransitoryPrivacy  bungie.Privacy `json:"transitoryPrivacy"`
	HasCurrentActivity bool           `json:"hasCurrentActivity"`
	TransitoryStart    *time.Time     `json:"transitoryStart,omitempty"`
	TransitoryEnd      *time.Time     `json:"transitoryEnd,omitempty"`
	Players            int            `json:"players,omitempty"`
	Opponents          int            `json:"opponents,omitempty"`

	ModeType   int      `json:"modeType,omitempty"`
	ModeTypes  []int    `json:"modeTypes,omitempty"`
	ModeHashes []uint32 `json:"modeHashes,omitempty"`
}

func Summarize(player string, p *bungie.Profile, l Lookup) Summary {
	s := Summary{
		Player:            player,
		Online:            p.ProfileTransitoryData.Data != nil,
		ActivitiesPrivacy: p.CharacterActivities.Privacy,
		TransitoryPrivacy: p.ProfileTransitoryData.Privacy,
	}
	if s.Online {
		td := p.ProfileTransitoryData.Data
		s.PartySize = len(td.PartyMembers)
		if ca := td.CurrentActivity; ca != nil {
			s.HasCurrentActivity = true
			s.TransitoryStart, s.TransitoryEnd = ca.StartTime, ca.EndTime
			s.Players, s.Opponents = ca.NumberOfPlayers, ca.NumberOfOpponents
		}
	}
	id, ok := p.ActiveCharacterID()
	if !ok {
		return s
	}
	a := p.CharacterActivities.Data[id]
	s.CharacterID = id
	s.Started = a.DateActivityStarted
	s.ModeType, s.ModeTypes, s.ModeHashes = a.CurrentActivityModeType, a.CurrentActivityModeTypes, a.CurrentActivityModeHashes
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

func privacyName(p bungie.Privacy) string {
	switch p {
	case bungie.PrivacyNone:
		return "none"
	case bungie.PrivacyPublic:
		return "public"
	case bungie.PrivacyPrivate:
		return "private"
	default:
		return fmt.Sprint(int(p))
	}
}

func (s Summary) String() string {
	started := "-"
	if !s.Started.IsZero() {
		started = s.Started.Local().Format("01-02 15:04:05")
	}
	return fmt.Sprintf("%-22s online=%t priv=act:%s/trans:%s activity=%q pvp=%t mode=%q cat=%s playlist=%q started=%s party=%d ca=%t players=%d opp=%d modeType=%d",
		s.Player, s.Online, privacyName(s.ActivitiesPrivacy), privacyName(s.TransitoryPrivacy), s.Activity, s.ActivityIsPvP, s.Mode, s.ModeCategory,
		s.Playlist, started, s.PartySize, s.HasCurrentActivity, s.Players, s.Opponents, s.ModeType)
}
