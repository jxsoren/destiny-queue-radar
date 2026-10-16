package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
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

// acquireStream reserves one of the user's stream slots; the caller must call the returned
// release func when the stream ends.
func (s *Server) acquireStream(userID int64) (release func(), ok bool) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()
	if s.streams[userID] >= maxStreamsPerUser {
		return nil, false
	}
	s.streams[userID]++
	return func() {
		s.streamsMu.Lock()
		defer s.streamsMu.Unlock()
		if s.streams[userID]--; s.streams[userID] <= 0 {
			delete(s.streams, userID)
		}
	}, true
}

// stillSignedIn re-checks that the request's session still belongs to u. Transient store
// errors count as "yes" so a database blip does not log everyone out.
func (s *Server) stillSignedIn(r *http.Request, u store.User) bool {
	c, err := r.Cookie(auth.SessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	cur, err := s.d.Store.SessionUser(r.Context(), auth.HashToken(c.Value), s.now())
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	return err != nil || cur.ID == u.ID
}

// handleStream is the live dashboard feed (spec §6.3). It subscribes before reading the
// snapshot, so no change between the two is lost; at worst a change is sent twice.
// Every write has a deadline, each user has a stream cap, and the session is re-checked on
// every heartbeat so a logout or account deletion ends the stream.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, u store.User) {
	release, ok := s.acquireStream(u.ID)
	if !ok {
		writeError(w, http.StatusTooManyRequests, "too many open streams")
		return
	}
	defer release()

	conn, unsubscribe := s.d.Hub.Subscribe(u.ID)
	defer unsubscribe()

	list, err := s.d.Store.Watchlist(r.Context(), u.ID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	rc := http.NewResponseController(w)
	defer rc.SetWriteDeadline(time.Time{}) // don't leave a stale deadline on a keep-alive connection
	// send writes one frame under a fresh write deadline (unsupported deadlines are ignored)
	// and flushes it, so a stalled client cannot hold the handler forever.
	send := func(write func() error) error {
		_ = rc.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
		if err := write(); err != nil {
			return err
		}
		return rc.Flush()
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if send(func() error {
		return writeEvent(w, "snapshot", snapshotJSON{Players: toPlayersJSON(list), APIStatus: s.d.Engine.APIState()})
	}) != nil {
		return
	}

	heartbeat := time.NewTicker(s.heartbeat)
	defer heartbeat.Stop()
	for {
		var err error
		select {
		case <-r.Context().Done():
			return
		case <-s.d.Done: // server shutting down (nil channel: never)
			return
		case <-conn.Done():
			return // dropped for falling behind; the client reconnects and gets a new snapshot
		case ev := <-conn.C:
			err = send(func() error { return writeEvent(w, ev.Type, ev.Data) })
		case <-heartbeat.C:
			if !s.stillSignedIn(r, u) {
				send(func() error { return writeEvent(w, "logout", struct{}{}) })
				return
			}
			err = send(func() error { _, err := io.WriteString(w, ": ping\n\n"); return err })
		}
		if err != nil {
			return
		}
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
