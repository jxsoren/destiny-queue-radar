package web

import (
	"context"
	"net/http"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/engine"
)

const (
	// pollerStaleAfter is how long the scheduler may go without looping before readiness fails.
	// It loops every second normally; a database stall in the active-set query can hold it longer.
	pollerStaleAfter = 30 * time.Second
	pingTimeout      = 2 * time.Second
)

type readiness struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Poller   string `json:"poller"`
	// Bungie is informational: an outage there does not fail readiness, since restarting
	// or withholding traffic from this server cannot fix it.
	Bungie engine.APIState `json:"bungie"`
}

// handleHealthz is the liveness check: the process is up and serving HTTP.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz is the readiness check: the database answers and the poller is looping.
// Failures log at Warn, not Error: uptime monitors probe this every minute, and they alert.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	rd := readiness{Status: "ok", Database: "ok", Poller: "ok", Bungie: s.d.Engine.APIState()}

	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()
	if err := s.d.Store.Ping(ctx); err != nil {
		s.d.Log.WarnContext(r.Context(), "readiness: database ping failed", "err", err)
		rd.Database, rd.Status = "unavailable", "unavailable"
	}
	if last := s.d.Engine.LastTick(); last.IsZero() || s.now().Sub(last) > pollerStaleAfter {
		s.d.Log.WarnContext(r.Context(), "readiness: poller stalled", "last_tick", last)
		rd.Poller, rd.Status = "stalled", "unavailable"
	}

	code := http.StatusOK
	if rd.Status != "ok" {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, code, rd)
}
