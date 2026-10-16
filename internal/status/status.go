// Package status defines the computed state of a watched player.
package status

import (
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

type State string

const (
	Unknown       State = ""
	InMatch       State = "IN_MATCH"
	LikelyQueuing State = "LIKELY_QUEUING"
	NotInPvP      State = "NOT_IN_PVP"
	Offline       State = "OFFLINE"
	Private       State = "PRIVATE"
	NotFound      State = "NOT_FOUND"
)

// Place refines NotInPvP: where the player is instead.
type Place string

const (
	PlaceNone   Place = ""
	PlaceOrbit  Place = "ORBIT"
	PlaceSocial Place = "SOCIAL"
	PlacePvE    Place = "PVE"
)

type Status struct {
	State           State      `json:"state"`
	Place           Place      `json:"place,omitempty"`
	Activity        string     `json:"activity,omitempty"`
	Mode            string     `json:"mode,omitempty"`
	Playlist        string     `json:"playlist,omitempty"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	LastPvPEndedAt  *time.Time `json:"lastPvpEndedAt,omitempty"`
	LastPvPPlaylist string     `json:"lastPvpPlaylist,omitempty"`
}

// Equal reports whether two statuses would look the same to a viewer.
func (s Status) Equal(o Status) bool {
	return s.State == o.State &&
		s.Place == o.Place &&
		s.Activity == o.Activity &&
		s.Mode == o.Mode &&
		s.Playlist == o.Playlist &&
		s.LastPvPPlaylist == o.LastPvPPlaylist &&
		timeEqual(s.StartedAt, o.StartedAt) &&
		timeEqual(s.LastPvPEndedAt, o.LastPvPEndedAt)
}

func timeEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// Classifier turns a freshly polled profile into a status. prev is the last stored status,
// which carries memory such as LastPvPEndedAt between polls.
//
// The bool is false when the profile did not contain enough to decide (a component was not
// returned). The caller must then keep the previous status and treat the poll as not successful.
type Classifier interface {
	Classify(p *bungie.Profile, prev Status, now time.Time) (Status, bool)
}
