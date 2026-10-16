package web

import (
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

const maxNoteLen = 200

// playerJSON is a watched player as the dashboard sees it (also used in the SSE snapshot).
type playerJSON struct {
	MembershipType       int64         `json:"membershipType"`
	MembershipID         string        `json:"membershipId"`
	BungieName           string        `json:"bungieName"`
	EmblemPath           string        `json:"emblemPath"`
	EmblemBackgroundPath string        `json:"emblemBackgroundPath"`
	Status               status.Status `json:"status"`
	LastPolledAt         *time.Time    `json:"lastPolledAt"`
	LastSuccessAt        *time.Time    `json:"lastSuccessAt"`
	Note                 string        `json:"note"`
	Source               string        `json:"source"`
	AddedAt              time.Time     `json:"addedAt"`
}

func toPlayersJSON(list []store.WatchedPlayer) []playerJSON {
	out := make([]playerJSON, 0, len(list))
	for _, w := range list {
		out = append(out, playerJSON{
			MembershipType: w.MembershipType, MembershipID: w.MembershipID, BungieName: w.BungieName,
			EmblemPath: w.EmblemPath, EmblemBackgroundPath: w.EmblemBackgroundPath, Status: w.Status,
			LastPolledAt: w.LastPolledAt, LastSuccessAt: w.LastSuccessAt,
			Note: w.Note, Source: w.Source, AddedAt: w.AddedAt,
		})
	}
	return out
}

func (s *Server) handleWatchlist(w http.ResponseWriter, r *http.Request, u store.User) {
	list, err := s.d.Store.Watchlist(r.Context(), u.ID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlayersJSON(list))
}

type addWatchRequest struct {
	MembershipType int64  `json:"membershipType"`
	MembershipID   string `json:"membershipId"`
	BungieName     string `json:"bungieName"`
	Note           string `json:"note"`
	Source         string `json:"source"`
}

func (s *Server) handleAddWatch(w http.ResponseWriter, r *http.Request, u store.User) {
	var req addWatchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch {
	case req.MembershipType <= 0 || !isDigits(req.MembershipID):
		writeError(w, http.StatusBadRequest, "membershipType and a numeric membershipId are required")
		return
	case req.Source != store.SourceSearch && req.Source != store.SourceMatch:
		writeError(w, http.StatusBadRequest, `source must be "search" or "match"`)
		return
	case utf8.RuneCountInString(req.Note) > maxNoteLen:
		writeError(w, http.StatusBadRequest, "note is too long")
		return
	}
	ref := store.PlayerRef{MembershipType: req.MembershipType, MembershipID: req.MembershipID, BungieName: req.BungieName}
	err := s.d.Store.AddWatch(r.Context(), u.ID, ref, req.Note, req.Source)
	if errors.Is(err, store.ErrWatchlistFull) {
		writeError(w, http.StatusConflict, "your watchlist is full")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request, u store.User) {
	var req struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(w, r, &req); err != nil || utf8.RuneCountInString(req.Note) > maxNoteLen {
		writeError(w, http.StatusBadRequest, "invalid note")
		return
	}
	err := s.d.Store.UpdateNote(r.Context(), u.ID, r.PathValue("membershipId"), req.Note)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not on your watchlist")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveWatch(w http.ResponseWriter, r *http.Request, u store.User) {
	err := s.d.Store.RemoveWatch(r.Context(), u.ID, r.PathValue("membershipId"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not on your watchlist")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type searchResultJSON struct {
	MembershipType int64  `json:"membershipType"`
	MembershipID   string `json:"membershipId"`
	BungieName     string `json:"bungieName"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request, u store.User) {
	if !s.allowLookup(w, u) {
		return
	}
	name, code, err := bungie.ParseBungieName(r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "enter a Bungie Name like Guardian#1234")
		return
	}
	card, err := s.d.Bungie.SearchByBungieName(r.Context(), name, code)
	if bungie.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "no player with that Bungie Name")
		return
	}
	if err != nil {
		s.d.Log.Warn("search failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	display := bungie.FormatBungieName(name, code)
	if card.BungieGlobalDisplayName != "" {
		display = bungie.FormatBungieName(card.BungieGlobalDisplayName, card.BungieGlobalDisplayNameCode)
	}
	writeJSON(w, http.StatusOK, searchResultJSON{MembershipType: card.MembershipType, MembershipID: card.MembershipID, BungieName: display})
}

// allowLookup applies the per-user lookup rate limit, answering 429 when exceeded.
func (s *Server) allowLookup(w http.ResponseWriter, u store.User) bool {
	ok, wait := s.limiter.allow(u.ID)
	if !ok {
		w.Header().Set("Retry-After", retryAfterSeconds(wait))
		writeError(w, http.StatusTooManyRequests, "too many lookups; slow down")
	}
	return ok
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
