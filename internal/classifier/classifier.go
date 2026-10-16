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
//
// Rules, on top of current():
//   - A profile that cannot tell us whether the player is online (transitory component
//     not returned) leaves the previous status unchanged, so nothing flaps.
//   - Leaving a match, or starting a different match back to back, records
//     LastPvPEndedAt and LastPvPPlaylist.
//   - LIKELY_QUEUING needs the previous state to be IN_MATCH or LIKELY_QUEUING, the player
//     in Orbit or a known social space, and the last match to have ended under queueWindow ago.
//     A detour through PvE or an unknown activity therefore ends the queuing guess.
func (c *Classifier) Classify(p *bungie.Profile, prev status.Status, now time.Time) status.Status {
	next, known := c.current(p)
	if !known {
		return prev
	}
	next.LastPvPEndedAt, next.LastPvPPlaylist = prev.LastPvPEndedAt, prev.LastPvPPlaylist

	if prev.State == status.InMatch {
		switch {
		case next.State != status.InMatch:
			ended := now
			next.LastPvPEndedAt = &ended
			next.LastPvPPlaylist = playlistOf(prev)
		case prev.StartedAt != nil && next.StartedAt != nil && !prev.StartedAt.Equal(*next.StartedAt):
			ended := *next.StartedAt
			next.LastPvPEndedAt = &ended
			next.LastPvPPlaylist = playlistOf(prev)
		}
	}
	if next.State == status.NotInPvP &&
		(prev.State == status.InMatch || prev.State == status.LikelyQueuing) &&
		(next.Place == status.PlaceOrbit || next.Place == status.PlaceSocial) &&
		next.LastPvPEndedAt != nil && now.Sub(*next.LastPvPEndedAt) < queueWindow {
		next.State = status.LikelyQueuing
	}
	return next
}

// playlistOf names a finished match: its playlist, else its mode.
func playlistOf(s status.Status) string {
	if s.Playlist != "" {
		return s.Playlist
	}
	return s.Mode
}

// current classifies the profile on its own, without memory of earlier polls. The bool is
// false when the profile does not say enough to classify (a component was not returned).
//   - PRIVATE needs a private flag and no data, for either component.
//   - OFFLINE needs a public transitory component with no data; it is checked before
//     activities because offline players keep stale activity data.
//   - IN_MATCH if the primary mode, any parent mode, or (absent a PvE mode) the activity is PvP.
//   - Otherwise NOT_IN_PVP, with a Place only for Orbit, known PvE modes and known social modes.
func (c *Classifier) current(p *bungie.Profile) (status.Status, bool) {
	acts, tp := p.CharacterActivities, p.ProfileTransitoryData
	if (tp.Privacy == bungie.PrivacyPrivate && tp.Data == nil) ||
		(acts.Privacy == bungie.PrivacyPrivate && len(acts.Data) == 0) {
		return status.Status{State: status.Private}, true
	}
	if tp.Data == nil {
		if tp.Privacy == bungie.PrivacyPublic {
			return status.Status{State: status.Offline}, true
		}
		return status.Status{}, false
	}
	id, ok := p.ActiveCharacterID()
	if !ok {
		return status.Status{State: status.NotInPvP}, true
	}
	a := acts.Data[id]
	act, hasAct := c.lookup.Activity(a.CurrentActivityHash)
	mode, hasMode := c.lookup.Mode(a.CurrentActivityModeHash)
	playlist, _ := c.lookup.Activity(a.CurrentPlaylistActivityHash)
	st := status.Status{Activity: act.Name, Mode: mode.Name, Playlist: playlist.Name}

	if c.isPvP(a, act, hasAct, mode, hasMode) {
		st.State = status.InMatch
		if !a.DateActivityStarted.IsZero() {
			started := a.DateActivityStarted
			st.StartedAt = &started
		}
		return st, true
	}

	st.State = status.NotInPvP
	switch {
	case a.CurrentActivityHash == 0 || a.CurrentActivityHash == orbitActivityHash:
		st.Place = status.PlaceOrbit
		st.Activity = "Orbit"
	case hasMode && (mode.Category == manifest.CategoryPvE || mode.Category == manifest.CategoryPvECompetitive):
		st.Place = status.PlacePvE
	case hasMode && mode.Category == manifest.CategoryNone:
		st.Place = status.PlaceSocial
	}
	return st, true
}

func (c *Classifier) isPvP(a bungie.CharacterActivities, act manifest.Activity, hasAct bool, mode manifest.Mode, hasMode bool) bool {
	if hasMode && mode.Category == manifest.CategoryPvP {
		return true
	}
	for _, h := range a.CurrentActivityModeHashes {
		if m, ok := c.lookup.Mode(h); ok && m.Category == manifest.CategoryPvP {
			return true
		}
	}
	pve := hasMode && (mode.Category == manifest.CategoryPvE || mode.Category == manifest.CategoryPvECompetitive)
	return hasAct && act.IsPvP && !pve
}
