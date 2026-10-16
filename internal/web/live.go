package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/engine"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// handleRefresh queues an immediate poll of the user's watchlist (spec §4.4).
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request, u store.User) {
	if s.d.Engine.APIState() == engine.APIDown {
		writeError(w, http.StatusServiceUnavailable, "Bungie API is unavailable; statuses are frozen")
		return
	}
	err := s.d.Engine.RequestRefresh(r.Context(), u.ID)
	var cooldown *engine.CooldownError
	if errors.As(err, &cooldown) {
		w.Header().Set("Retry-After", retryAfterSeconds(cooldown.RetryAfter))
		writeError(w, http.StatusTooManyRequests, "refresh is on cooldown")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type snapshotJSON struct {
	Players   []playerJSON    `json:"players"`
	APIStatus engine.APIState `json:"apiStatus"`
}

// handleStream is the live dashboard feed (spec §6.3). It subscribes before reading the
// snapshot, so no change between the two is lost; at worst a change is sent twice.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, u store.User) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.internal(w, r, errors.New("response writer cannot flush"))
		return
	}
	conn, unsubscribe := s.d.Hub.Subscribe(u.ID)
	defer unsubscribe()

	list, err := s.d.Store.Watchlist(r.Context(), u.ID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := writeEvent(w, "snapshot", snapshotJSON{Players: toPlayersJSON(list), APIStatus: s.d.Engine.APIState()}); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(s.heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-conn.Done():
			return // dropped for falling behind; the client reconnects and gets a new snapshot
		case ev := <-conn.C:
			if err := writeEvent(w, ev.Type, ev.Data); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
		}
		flusher.Flush()
	}
}

func writeEvent(w io.Writer, name string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
	return err
}
