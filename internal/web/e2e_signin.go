//go:build e2e

// This file is compiled only with -tags e2e. It adds GET /auth/e2e-login, which signs in the
// fake Bungie server's test user without OAuth, for Playwright. It must never ship.

package web

import (
	"net/http"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/fakebungie"
)

const e2eBungieNetID = "e2e-bnet-1"

func init() {
	extraRoutes = append(extraRoutes, func(s *Server) {
		s.mux.HandleFunc("GET /auth/e2e-login", s.handleE2ELogin)
	})
}

// handleE2ELogin creates (or reuses) the test user, stores placeholder tokens so recent matches
// work against the fake Bungie server, starts a session and redirects to the dashboard.
func (s *Server) handleE2ELogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, err := s.d.Store.UpsertUser(ctx, e2eBungieNetID, fakebungie.SelfName)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	if err := s.d.Store.SetDestinyMembership(ctx, u.ID, fakebungie.SelfType, fakebungie.SelfID); err != nil {
		s.internal(w, r, err)
		return
	}
	tokens := auth.TokenResponse{AccessToken: "e2e-access", RefreshToken: "e2e-refresh",
		ExpiresIn: 3600, RefreshExpiresIn: 86400, MembershipID: e2eBungieNetID}
	if err := s.d.Tokens.Persist(ctx, u.ID, tokens); err != nil {
		s.internal(w, r, err)
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		s.internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
