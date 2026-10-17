package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

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
			Source: w.Source, AddedAt: w.AddedAt,
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
	// BungieName is accepted for compatibility but ignored: identity comes from Bungie.
	BungieName string `json:"bungieName"`
	Source     string `json:"source"`
}

// validMembershipTypes are the Destiny 2 platforms a player can be on: Xbox, PSN, Steam,
// Stadia and Epic.
var validMembershipTypes = map[int64]bool{1: true, 2: true, 3: true, 5: true, 6: true}

// handleAddWatch never trusts client-supplied identity: the player row is shared by every
// user who watches them, so type and name come from Bungie's own answer.
func (s *Server) handleAddWatch(w http.ResponseWriter, r *http.Request, u store.User) {
	var req addWatchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		badBody(w, err)
		return
	}
	switch {
	case !validMembershipTypes[req.MembershipType] || !isDigits(req.MembershipID):
		writeError(w, http.StatusBadRequest, "membershipType and a numeric membershipId are required")
		return
	case req.Source != store.SourceSearch && req.Source != store.SourceMatch:
		writeError(w, http.StatusBadRequest, `source must be "search" or "match"`)
		return
	}
	if !s.allowLookup(w, u) {
		return
	}
	prof, err := s.d.Bungie.GetProfile(r.Context(), req.MembershipType, req.MembershipID)
	switch {
	case bungie.IsNotFound(err):
		writeError(w, http.StatusNotFound, "player not found")
		return
	case bungie.IsSystemDisabled(err), bungie.IsThrottled(err):
		s.d.Log.WarnContext(r.Context(), "profile lookup unavailable", "err", err)
		writeError(w, http.StatusServiceUnavailable, "Bungie API is unavailable")
		return
	case err != nil:
		s.d.Log.WarnContext(r.Context(), "profile lookup failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	// Private or missing profile data: keep the (Bungie-confirmed) ID, leave the name empty.
	ref := store.PlayerRef{MembershipType: req.MembershipType, MembershipID: req.MembershipID}
	if d := prof.Profile.Data; d != nil {
		if d.UserInfo.MembershipID != req.MembershipID || !validMembershipTypes[d.UserInfo.MembershipType] {
			s.d.Log.WarnContext(r.Context(), "profile does not match the request", "asked", req.MembershipID, "got", d.UserInfo.MembershipID)
			writeError(w, http.StatusBadGateway, "unexpected response from Bungie")
			return
		}
		ref.MembershipType, ref.MembershipID = d.UserInfo.MembershipType, d.UserInfo.MembershipID
		if n := d.UserInfo.BungieGlobalDisplayName; n != "" {
			ref.BungieName = bungie.FormatBungieName(n, d.UserInfo.BungieGlobalDisplayNameCode)
		}
	}
	err = s.d.Store.AddWatch(r.Context(), u.ID, ref, "", req.Source)
	if errors.Is(err, store.ErrWatchlistFull) {
		writeError(w, http.StatusConflict, "your watchlist is full")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	// The player is Bungie-verified, so a stored NOT_FOUND is stale: poll right away.
	s.d.Engine.Recheck(ref.MembershipType, ref.MembershipID)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleRemoveWatch(w http.ResponseWriter, r *http.Request, u store.User) {
	id := r.PathValue("membershipId")
	if !isDigits(id) {
		writeError(w, http.StatusBadRequest, "invalid membership id")
		return
	}
	err := s.d.Store.RemoveWatch(r.Context(), u.ID, id)
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

// searchEmblemTimeout bounds the optional emblem lookup so a slow profile never delays a search.
// It is a variable only so tests can shorten it.
var searchEmblemTimeout = 3 * time.Second

type searchResultJSON struct {
	MembershipType int64  `json:"membershipType"`
	MembershipID   string `json:"membershipId"`
	BungieName     string `json:"bungieName"`
	EmblemPath     string `json:"emblemPath"`
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
		s.d.Log.WarnContext(r.Context(), "search failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	display := bungie.FormatBungieName(name, code)
	if card.BungieGlobalDisplayName != "" {
		display = bungie.FormatBungieName(card.BungieGlobalDisplayName, card.BungieGlobalDisplayNameCode)
	}
	// The emblem is a nicety: a failed profile lookup still answers the search.
	var emblem string
	pctx, cancel := context.WithTimeout(r.Context(), searchEmblemTimeout)
	defer cancel()
	if prof, err := s.d.Bungie.GetProfile(pctx, card.MembershipType, card.MembershipID); err == nil {
		emblem, _ = prof.ActiveEmblem()
	} else if !bungie.IsNotFound(err) && !errors.Is(err, context.DeadlineExceeded) {
		s.d.Log.WarnContext(r.Context(), "search emblem lookup failed", "err", err)
	}
	writeJSON(w, http.StatusOK, searchResultJSON{MembershipType: card.MembershipType, MembershipID: card.MembershipID,
		BungieName: display, EmblemPath: emblem})
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

// isDigits reports whether s is a 1-19 digit ID (the most a 64-bit Bungie ID can hold).
func isDigits(s string) bool {
	if s == "" || len(s) > 19 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
