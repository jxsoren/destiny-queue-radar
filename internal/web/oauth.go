package web

import (
	"errors"
	"net/http"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := auth.NewToken()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	http.SetCookie(w, auth.StateCookieFor(state))
	http.Redirect(w, r, auth.AuthorizeURL(s.d.OAuthClientID, state), http.StatusFound)
}

// handleCallback completes Bungie sign-in: verify state, exchange the code, identify the
// account, store the user and tokens, and start a session.
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	stateCookie := ""
	if c, err := r.Cookie(auth.StateCookie); err == nil {
		stateCookie = c.Value
	}
	// The state is single-use: clear it whether or not it matches.
	http.SetCookie(w, auth.ClearStateCookie())
	if !auth.StateMatches(stateCookie, q.Get("state")) {
		writeError(w, http.StatusBadRequest, "sign-in expired or was tampered with; try again")
		return
	}
	if q.Get("error") != "" {
		writeError(w, http.StatusBadRequest, "Bungie sign-in was cancelled")
		return
	}
	code := q.Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	ctx := r.Context()
	tokens, err := s.d.Tokens.Exchange(ctx, code)
	if errors.Is(err, auth.ErrGrantRejected) {
		writeError(w, http.StatusBadRequest, "Bungie rejected the sign-in code; try again")
		return
	}
	if err != nil {
		s.d.Log.Error("token exchange failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}
	me, err := s.d.Bungie.GetCurrentUser(ctx, tokens.AccessToken)
	if errors.Is(err, bungie.ErrNoDestinyAccount) {
		writeError(w, http.StatusForbidden, "this Bungie account has no Destiny 2 profile")
		return
	}
	if err != nil {
		s.d.Log.Error("current user lookup failed", "err", err)
		writeError(w, http.StatusBadGateway, "could not reach Bungie")
		return
	}

	bnetID := me.BungieNetMembershipID
	if bnetID == "" {
		bnetID = tokens.MembershipID
	}
	u, err := s.d.Store.UpsertUser(ctx, bnetID, me.BungieName)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	if err := s.d.Store.SetDestinyMembership(ctx, u.ID, me.Destiny.MembershipType, me.Destiny.MembershipID); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := s.d.Tokens.Persist(ctx, u.ID, tokens); err != nil {
		s.internal(w, r, err)
		return
	}
	// Rotate: a new sign-in invalidates any session the browser already had.
	if old, err := r.Cookie(auth.SessionCookie); err == nil && old.Value != "" {
		if err := s.d.Store.DeleteSession(ctx, auth.HashToken(old.Value)); err != nil {
			s.internal(w, r, err)
			return
		}
	}
	session, err := auth.NewToken()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	expires := s.now().Add(auth.SessionTTL)
	if err := s.d.Store.CreateSession(ctx, auth.HashToken(session), u.ID, expires); err != nil {
		s.internal(w, r, err)
		return
	}
	http.SetCookie(w, auth.SessionCookieFor(session, expires))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
