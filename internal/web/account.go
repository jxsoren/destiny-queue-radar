package web

import (
	"net/http"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// handleIndex is a placeholder until the frontend (Plan 3) is served from here.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!doctype html><title>Destiny Queue Radar</title>` +
		`<p>Destiny Queue Radar API is running. <a href="/auth/login">Sign in with Bungie</a></p>`))
}

type meJSON struct {
	ID             int64  `json:"id"`
	BungieName     string `json:"bungieName"`
	MembershipType int64  `json:"membershipType"`
	MembershipID   string `json:"membershipId"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u store.User) {
	writeJSON(w, http.StatusOK, meJSON{ID: u.ID, BungieName: u.BungieName,
		MembershipType: u.DestinyMembershipType, MembershipID: u.DestinyMembershipID})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookie); err == nil && c.Value != "" {
		if err := s.d.Store.DeleteSession(r.Context(), auth.HashToken(c.Value)); err != nil {
			s.internal(w, r, err)
			return
		}
	}
	http.SetCookie(w, auth.ClearSessionCookie())
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteMe removes the account, watchlist, sessions and tokens (spec §7.5).
func (s *Server) handleDeleteMe(w http.ResponseWriter, r *http.Request, u store.User) {
	if err := s.d.Store.DeleteUser(r.Context(), u.ID); err != nil {
		s.internal(w, r, err)
		return
	}
	http.SetCookie(w, auth.ClearSessionCookie())
	w.WriteHeader(http.StatusNoContent)
}
