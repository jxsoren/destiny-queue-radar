// Package web serves the REST API, the live event stream and Bungie sign-in (spec §6).
package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/engine"
	"github.com/jxsoren/destiny-queue-radar/internal/hub"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// Store is the subset of *store.Store the web layer uses.
type Store interface {
	SessionUser(ctx context.Context, idHash []byte, now time.Time) (store.User, error)
	CreateSession(ctx context.Context, idHash []byte, userID int64, expiresAt time.Time) error
	DeleteSession(ctx context.Context, idHash []byte) error
	UpsertUser(ctx context.Context, bungieMembershipID, bungieName string) (store.User, error)
	SetDestinyMembership(ctx context.Context, userID, membershipType int64, membershipID string) error
	DeleteUser(ctx context.Context, userID int64) error
	AddWatch(ctx context.Context, userID int64, p store.PlayerRef, note, source string) error
	UpdateNote(ctx context.Context, userID int64, membershipID, note string) error
	RemoveWatch(ctx context.Context, userID int64, membershipID string) error
	Watchlist(ctx context.Context, userID int64) ([]store.WatchedPlayer, error)
}

// Engine is the subset of *engine.Engine the web layer uses.
type Engine interface {
	RequestRefresh(ctx context.Context, userID int64) error
	APIState() engine.APIState
}

// Hub is the subset of *hub.Hub the web layer uses.
type Hub interface {
	Subscribe(userID int64) (*hub.Conn, func())
}

// Bungie is the subset of *bungie.Client the web layer uses.
type Bungie interface {
	SearchByBungieName(ctx context.Context, name string, code int) (bungie.UserInfoCard, error)
	GetCurrentUser(ctx context.Context, accessToken string) (bungie.CurrentUser, error)
}

// Tokens is implemented by *auth.TokenManager.
type Tokens interface {
	Exchange(ctx context.Context, code string) (auth.TokenResponse, error)
	Persist(ctx context.Context, userID int64, r auth.TokenResponse) error
	AccessToken(ctx context.Context, userID int64) (string, error)
}

// Matches is implemented by *matches.Service.
type Matches interface {
	Recent(ctx context.Context, token string, membershipType int64, membershipID string, limit int) ([]matches.Match, error)
	Match(ctx context.Context, instanceID string) (matches.Match, error)
}

var (
	_ Store   = (*store.Store)(nil)
	_ Engine  = (*engine.Engine)(nil)
	_ Hub     = (*hub.Hub)(nil)
	_ Bungie  = (*bungie.Client)(nil)
	_ Tokens  = (*auth.TokenManager)(nil)
	_ Matches = (*matches.Service)(nil)
)

type Deps struct {
	Store         Store
	Engine        Engine
	Hub           Hub
	Bungie        Bungie
	Tokens        Tokens
	Matches       Matches
	OAuthClientID string
	Log           *slog.Logger
}

const (
	requestedWithHeader = "X-Requested-With"
	requestedWithValue  = "dqr"
	lookupsPerMinute    = 30 // search and match lookups per user (spec §7.5)
)

type Server struct {
	d         Deps
	mux       *http.ServeMux
	limiter   *userLimiter
	now       func() time.Time
	heartbeat time.Duration
}

func New(d Deps) *Server {
	s := &Server{d: d, mux: http.NewServeMux(), now: time.Now, heartbeat: 25 * time.Second}
	s.limiter = newUserLimiter(lookupsPerMinute, time.Minute)
	s.limiter.now = func() time.Time { return s.now() }
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /auth/callback", s.handleCallback)
	s.mux.HandleFunc("POST /auth/logout", s.mutating(s.handleLogout))
	s.mux.HandleFunc("GET /api/me", s.authed(s.handleMe))
	s.mux.HandleFunc("DELETE /api/me", s.mutating(s.authed(s.handleDeleteMe)))
}

type userHandler func(w http.ResponseWriter, r *http.Request, u store.User)

// authed resolves the session cookie to a user, or answers 401.
func (s *Server) authed(h userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.SessionCookie)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "sign in required")
			return
		}
		u, err := s.d.Store.SessionUser(r.Context(), auth.HashToken(c.Value), s.now())
		if errors.Is(err, store.ErrNotFound) {
			http.SetCookie(w, auth.ClearSessionCookie())
			writeError(w, http.StatusUnauthorized, "sign in required")
			return
		}
		if err != nil {
			s.internal(w, r, err)
			return
		}
		h(w, r, u)
	}
}

// mutating rejects state-changing requests without the custom header (CSRF protection, spec §7.3).
func (s *Server) mutating(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(requestedWithHeader) != requestedWithValue {
			writeError(w, http.StatusForbidden, "missing "+requestedWithHeader+" header")
			return
		}
		h(w, r)
	}
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, err error) {
	s.d.Log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
