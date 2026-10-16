// Package matches lists a user's recent PvP matches with everyone who played in them, so
// players can be added to a watchlist straight from a lobby.
package matches

import (
	"container/list"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
)

// Profiles, History and Reports are implemented by *bungie.Client (Reports by the client
// pointed at the stats host).
type Profiles interface {
	GetProfile(ctx context.Context, membershipType int64, membershipID string) (*bungie.Profile, error)
}

type History interface {
	ActivityHistory(ctx context.Context, token string, membershipType int64, membershipID, characterID string, mode, count int) ([]bungie.HistoryActivity, error)
}

type Reports interface {
	GetPGCR(ctx context.Context, instanceID string) (*bungie.PGCR, error)
}

// Lookup is implemented by *manifest.Manifest.
type Lookup interface {
	Activity(hash uint32) (manifest.Activity, bool)
}

type Participant struct {
	MembershipType int64  `json:"membershipType"`
	MembershipID   string `json:"membershipId"`
	BungieName     string `json:"bungieName"` // empty for players without a Bungie Name
	EmblemPath     string `json:"emblemPath"`
	Team           int    `json:"team"` // -1 when the activity has no teams
}

// Match is one activity instance. Matches are shared from a cache; treat them as read-only.
type Match struct {
	InstanceID string        `json:"instanceId"`
	Period     time.Time     `json:"period"`
	Activity   string        `json:"activity"` // the map
	Playlist   string        `json:"playlist"`
	Players    []Participant `json:"players"`
}

type Service struct {
	profiles Profiles
	history  History
	reports  Reports
	lookup   Lookup

	mu        sync.Mutex
	cacheSize int
	order     *list.List // front = most recently used; values are Match
	cache     map[string]*list.Element
}

func New(p Profiles, h History, r Reports, l Lookup) *Service {
	return &Service{profiles: p, history: h, reports: r, lookup: l, cacheSize: 500,
		order: list.New(), cache: map[string]*list.Element{}}
}

// Recent returns up to limit of the player's most recent PvP matches across all characters,
// newest first. token may be empty when the player's history is public. Matches whose report
// cannot be fetched are left out.
func (s *Service) Recent(ctx context.Context, token string, membershipType int64, membershipID string, limit int) ([]Match, error) {
	p, err := s.profiles.GetProfile(ctx, membershipType, membershipID)
	if err != nil {
		return nil, fmt.Errorf("matches: load profile: %w", err)
	}
	chars := make([]string, 0, len(p.Characters.Data))
	for id := range p.Characters.Data {
		chars = append(chars, id)
	}
	sort.Strings(chars)

	var acts []bungie.HistoryActivity
	for _, c := range chars {
		h, err := s.history.ActivityHistory(ctx, token, membershipType, membershipID, c, bungie.ModeAllPvP, limit)
		if err != nil {
			return nil, fmt.Errorf("matches: history for character %s: %w", c, err)
		}
		acts = append(acts, h...)
	}
	sort.SliceStable(acts, func(i, j int) bool { return acts[i].Period.After(acts[j].Period) })

	seen := map[string]bool{}
	var instanceIDs []string
	for _, a := range acts {
		id := a.ActivityDetails.InstanceID
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		instanceIDs = append(instanceIDs, id)
		if len(instanceIDs) == limit {
			break
		}
	}

	results := make([]*Match, len(instanceIDs))
	var wg sync.WaitGroup
	for i, id := range instanceIDs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m, err := s.Match(ctx, id); err == nil {
				results[i] = &m
			}
		}()
	}
	wg.Wait()

	out := make([]Match, 0, len(results))
	for _, m := range results {
		if m != nil {
			out = append(out, *m)
		}
	}
	return out, nil
}

// Match returns one activity instance's participants. Reports never change, so they are cached.
func (s *Service) Match(ctx context.Context, instanceID string) (Match, error) {
	if m, ok := s.cached(instanceID); ok {
		return m, nil
	}
	r, err := s.reports.GetPGCR(ctx, instanceID)
	if err != nil {
		return Match{}, err
	}
	m := s.build(instanceID, r)
	s.remember(instanceID, m)
	return m, nil
}

func (s *Service) build(instanceID string, r *bungie.PGCR) Match {
	m := Match{InstanceID: instanceID, Period: r.Period}
	if a, ok := s.lookup.Activity(r.ActivityDetails.ReferenceID); ok {
		m.Activity = a.Name
	}
	if a, ok := s.lookup.Activity(r.ActivityDetails.DirectorActivityHash); ok {
		m.Playlist = a.Name
	}
	seen := map[string]bool{}
	for _, e := range r.Entries {
		u := e.Player.DestinyUserInfo
		if seen[u.MembershipID] { // one entry per character played
			continue
		}
		seen[u.MembershipID] = true
		name := ""
		if u.BungieGlobalDisplayName != "" {
			name = bungie.FormatBungieName(u.BungieGlobalDisplayName, u.BungieGlobalDisplayNameCode)
		}
		m.Players = append(m.Players, Participant{
			MembershipType: u.MembershipType,
			MembershipID:   u.MembershipID,
			BungieName:     name,
			EmblemPath:     u.IconPath,
			Team:           e.Team(),
		})
	}
	sort.SliceStable(m.Players, func(i, j int) bool {
		a, b := m.Players[i], m.Players[j]
		if a.Team != b.Team {
			return a.Team < b.Team
		}
		return a.MembershipID < b.MembershipID
	})
	return m
}

func (s *Service) cached(id string) (Match, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.cache[id]
	if !ok {
		return Match{}, false
	}
	s.order.MoveToFront(el)
	return el.Value.(Match), true
}

func (s *Service) remember(id string, m Match) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.cache[id]; ok {
		s.order.MoveToFront(el)
		return
	}
	s.cache[id] = s.order.PushFront(m)
	for s.order.Len() > s.cacheSize {
		oldest := s.order.Back()
		s.order.Remove(oldest)
		delete(s.cache, oldest.Value.(Match).InstanceID)
	}
}
