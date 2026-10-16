package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// recentMatchLimit is how many recent PvP matches the picker shows (spec §6.2).
const recentMatchLimit = 10

type participantJSON struct {
	matches.Participant
	Watched bool `json:"watched"`
	IsSelf  bool `json:"isSelf"`
}

type matchJSON struct {
	InstanceID string            `json:"instanceId"`
	Period     time.Time         `json:"period"`
	Activity   string            `json:"activity"`
	Playlist   string            `json:"playlist"`
	Players    []participantJSON `json:"players"`
}

func toMatchJSON(m matches.Match, self string, watched map[string]bool) matchJSON {
	out := matchJSON{InstanceID: m.InstanceID, Period: m.Period, Activity: m.Activity, Playlist: m.Playlist,
		Players: make([]participantJSON, 0, len(m.Players))}
	for _, p := range m.Players {
		out.Players = append(out.Players, participantJSON{Participant: p,
			Watched: watched[p.MembershipID], IsSelf: p.MembershipID == self})
	}
	return out
}

func (s *Server) watchedSet(r *http.Request, u store.User) (map[string]bool, error) {
	list, err := s.d.Store.Watchlist(r.Context(), u.ID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(list))
	for _, w := range list {
		set[w.MembershipID] = true
	}
	return set, nil
}

func (s *Server) handleRecentMatches(w http.ResponseWriter, r *http.Request, u store.User) {
	if u.DestinyMembershipID == "" {
		writeError(w, http.StatusConflict, "no Destiny 2 account linked; sign in again")
		return
	}
	if !s.allowLookup(w, u) {
		return
	}
	token, err := s.d.Tokens.AccessToken(r.Context(), u.ID)
	if errors.Is(err, auth.ErrReauthRequired) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in again to see your matches", "reauth": true})
		return
	}
	if err != nil {
		s.d.Log.Warn("access token unavailable", "user", u.ID, "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	recent, err := s.d.Matches.Recent(r.Context(), token, u.DestinyMembershipType, u.DestinyMembershipID, recentMatchLimit)
	if err != nil {
		s.d.Log.Warn("recent matches failed", "user", u.ID, "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	watched, err := s.watchedSet(r, u)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	out := make([]matchJSON, 0, len(recent))
	for _, m := range recent {
		out = append(out, toMatchJSON(m, u.DestinyMembershipID, watched))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMatch(w http.ResponseWriter, r *http.Request, u store.User) {
	id := r.PathValue("instanceId")
	if !isDigits(id) {
		writeError(w, http.StatusBadRequest, "invalid match id")
		return
	}
	if !s.allowLookup(w, u) {
		return
	}
	m, err := s.d.Matches.Match(r.Context(), id)
	if bungie.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "match not found")
		return
	}
	if err != nil {
		s.d.Log.Warn("match lookup failed", "instance", id, "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	watched, err := s.watchedSet(r, u)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMatchJSON(m, u.DestinyMembershipID, watched))
}
