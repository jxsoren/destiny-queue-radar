// Package classifier turns a polled profile into a player status (spec §5).
// The rules are provisional until the probe findings confirm how Bungie reports each state.
package classifier

import (
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
)

const (
	// queueWindow is how long after a PvP match ends a player in Orbit or a social space
	// counts as likely re-queuing.
	queueWindow = 5 * time.Minute
	// orbitActivityHash is the Orbit activity.
	orbitActivityHash uint32 = 82913930
)

// Lookup is implemented by *manifest.Manifest.
type Lookup interface {
	Activity(hash uint32) (manifest.Activity, bool)
	Mode(hash uint32) (manifest.Mode, bool)
}

type Classifier struct {
	lookup Lookup
}

var _ status.Classifier = (*Classifier)(nil)

func New(l Lookup) *Classifier { return &Classifier{lookup: l} }

// Classify implements status.Classifier.
func (c *Classifier) Classify(p *bungie.Profile, prev status.Status, now time.Time) status.Status {
	next := c.current(p)
	next.LastPvPEndedAt, next.LastPvPPlaylist = prev.LastPvPEndedAt, prev.LastPvPPlaylist

	if prev.State == status.InMatch && next.State != status.InMatch {
		ended := now
		next.LastPvPEndedAt = &ended
		next.LastPvPPlaylist = prev.Playlist
		if next.LastPvPPlaylist == "" {
			next.LastPvPPlaylist = prev.Mode
		}
	}
	if next.State == status.NotInPvP &&
		(next.Place == status.PlaceOrbit || next.Place == status.PlaceSocial) &&
		next.LastPvPEndedAt != nil && now.Sub(*next.LastPvPEndedAt) < queueWindow {
		next.State = status.LikelyQueuing
	}
	return next
}

// current classifies the profile on its own, without memory of earlier polls.
func (c *Classifier) current(p *bungie.Profile) status.Status {
	acts := p.CharacterActivities
	if p.ProfileTransitoryData.Privacy == bungie.PrivacyPrivate ||
		(acts.Privacy == bungie.PrivacyPrivate && len(acts.Data) == 0) {
		return status.Status{State: status.Private}
	}
	// Offline players keep their last activity in characterActivities, so check this first.
	if p.ProfileTransitoryData.Data == nil {
		return status.Status{State: status.Offline}
	}
	id, ok := p.ActiveCharacterID()
	if !ok {
		return status.Status{State: status.NotInPvP}
	}
	a := acts.Data[id]
	act, hasAct := c.lookup.Activity(a.CurrentActivityHash)
	mode, hasMode := c.lookup.Mode(a.CurrentActivityModeHash)
	playlist, _ := c.lookup.Activity(a.CurrentPlaylistActivityHash)
	st := status.Status{Activity: act.Name, Mode: mode.Name, Playlist: playlist.Name}

	if (hasMode && mode.Category == manifest.CategoryPvP) || (!hasMode && hasAct && act.IsPvP) {
		st.State = status.InMatch
		if !a.DateActivityStarted.IsZero() {
			started := a.DateActivityStarted
			st.StartedAt = &started
		}
		return st
	}

	st.State = status.NotInPvP
	switch {
	case a.CurrentActivityHash == 0 || a.CurrentActivityHash == orbitActivityHash:
		st.Place = status.PlaceOrbit
		st.Activity = "Orbit"
	case hasMode && (mode.Category == manifest.CategoryPvE || mode.Category == manifest.CategoryPvECompetitive):
		st.Place = status.PlacePvE
	default:
		st.Place = status.PlaceSocial
	}
	return st
}
