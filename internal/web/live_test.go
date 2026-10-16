package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/engine"
	"github.com/jxsoren/destiny-queue-radar/internal/hub"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

func TestRefresh(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")

	if rec := e.do(t, http.MethodPost, "/api/refresh", nil, cookie); rec.Code != http.StatusAccepted {
		t.Fatalf("refresh = %d", rec.Code)
	}
	if len(e.engine.refreshed) != 1 || e.engine.refreshed[0] != u.ID {
		t.Errorf("refreshed = %v", e.engine.refreshed)
	}

	e.engine.refreshErr = &engine.CooldownError{RetryAfter: 19500 * time.Millisecond}
	rec := e.do(t, http.MethodPost, "/api/refresh", nil, cookie)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "20" {
		t.Errorf("cooldown = %d Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}

	e.engine.refreshErr = errors.New("db down")
	if rec := e.do(t, http.MethodPost, "/api/refresh", nil, cookie); rec.Code != http.StatusInternalServerError {
		t.Errorf("error = %d", rec.Code)
	}

	e.engine.refreshErr = nil
	e.engine.state = engine.APIDown
	before := len(e.engine.refreshed)
	if rec := e.do(t, http.MethodPost, "/api/refresh", nil, cookie); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("bungie down = %d", rec.Code)
	}
	if len(e.engine.refreshed) != before {
		t.Error("refresh requested while Bungie is down; the cooldown would be wasted")
	}
}

type sseEvent struct {
	name string
	data string
}

// readEvents parses SSE frames from the stream onto a channel; comments arrive as name ":".
func readEvents(body *bufio.Reader) <-chan sseEvent {
	ch := make(chan sseEvent, 16)
	go func() {
		defer close(ch)
		var ev sseEvent
		for {
			line, err := body.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\n")
			switch {
			case strings.HasPrefix(line, ":"):
				ch <- sseEvent{name: ":"}
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			case line == "" && ev.name != "":
				ch <- ev
				ev = sseEvent{}
			}
		}
	}()
	return ch
}

func next(t *testing.T, ch <-chan sseEvent, skipPings bool) sseEvent {
	t.Helper()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("stream closed")
			}
			if skipPings && ev.name == ":" {
				continue
			}
			return ev
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for an SSE event")
		}
	}
}

func TestStream_SnapshotThenLiveEventsAndHeartbeats(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.store.AddWatch(context.Background(), u.ID, store.PlayerRef{MembershipType: 3, MembershipID: "777", BungieName: "Ghost#0880"}, "", store.SourceSearch)
	e.engine.state = engine.APIDegraded
	e.s.heartbeat = 20 * time.Millisecond

	srv := httptest.NewServer(e.s)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	req.AddCookie(cookie)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d content-type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	events := readEvents(bufio.NewReader(resp.Body))

	snap := next(t, events, true)
	if snap.name != "snapshot" {
		t.Fatalf("first event = %+v", snap)
	}
	var s snapshotJSON
	if err := json.Unmarshal([]byte(snap.data), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Players) != 1 || s.Players[0].MembershipID != "777" || s.APIStatus != engine.APIDegraded {
		t.Errorf("snapshot = %+v", s)
	}

	e.hub.SendTo([]int64{u.ID}, hub.Event{Type: engine.EventPlayerUpdate, Data: engine.PlayerUpdate{MembershipID: "777"}})
	upd := next(t, events, true)
	if upd.name != engine.EventPlayerUpdate || !strings.Contains(upd.data, `"membershipId":"777"`) {
		t.Errorf("update = %+v", upd)
	}

	// Nothing else is sent, so the next frame must be a heartbeat comment.
	if ping := next(t, events, false); ping.name != ":" {
		t.Errorf("expected a heartbeat, got %+v", ping)
	}

	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for len(e.hub.ActiveUsers(0)) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("subscription not released after the client disconnected")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStream_RequiresSession(t *testing.T) {
	e := newTestEnv(t)
	if rec := e.do(t, http.MethodGet, "/api/stream", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d", rec.Code)
	}
}

// openStream connects to /api/stream and returns the response (the caller closes it).
func openStream(t *testing.T, ctx context.Context, srv *httptest.Server, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	req.AddCookie(cookie)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestStream_PerUserCap(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	srv := httptest.NewServer(e.s)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < maxStreamsPerUser; i++ {
		resp := openStream(t, ctx, srv, cookie)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("stream %d = %d", i+1, resp.StatusCode)
		}
	}
	resp := openStream(t, ctx, srv, cookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("stream %d = %d, want 429", maxStreamsPerUser+1, resp.StatusCode)
	}

	// Closing one frees a slot.
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for {
		e.s.streamsMu.Lock()
		n := e.s.streams[1]
		e.s.streamsMu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stream count = %d after all clients disconnected", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStream_LogoutWhenSessionEnds(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.s.heartbeat = 20 * time.Millisecond
	srv := httptest.NewServer(e.s)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv, cookie)
	defer resp.Body.Close()
	events := readEvents(bufio.NewReader(resp.Body))
	if snap := next(t, events, true); snap.name != "snapshot" {
		t.Fatalf("first event = %+v", snap)
	}

	e.store.DeleteSession(context.Background(), auth.HashToken(cookie.Value))
	if ev := next(t, events, true); ev.name != "logout" {
		t.Fatalf("event = %+v, want logout", ev)
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Error("stream kept sending after logout")
		}
	case <-time.After(2 * time.Second):
		t.Error("stream not closed after logout")
	}
}

func TestStream_EventBetweenSubscribeAndSnapshotIsDelivered(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.store.onWatchlist = func() {
		e.hub.SendTo([]int64{u.ID}, hub.Event{Type: engine.EventPlayerUpdate, Data: engine.PlayerUpdate{MembershipID: "777"}})
	}
	srv := httptest.NewServer(e.s)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv, cookie)
	defer resp.Body.Close()
	events := readEvents(bufio.NewReader(resp.Body))
	if snap := next(t, events, true); snap.name != "snapshot" {
		t.Fatalf("first event = %+v", snap)
	}
	if upd := next(t, events, true); upd.name != engine.EventPlayerUpdate {
		t.Errorf("event published before the snapshot was lost: %+v", upd)
	}
}

func TestStream_EndsWhenDepsDoneCloses(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	done := make(chan struct{})
	e.s.d.Done = done
	srv := httptest.NewServer(e.s)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv, cookie)
	defer resp.Body.Close()
	events := readEvents(bufio.NewReader(resp.Body))
	if snap := next(t, events, true); snap.name != "snapshot" {
		t.Fatalf("first event = %+v", snap)
	}
	close(done)
	select {
	case _, ok := <-events:
		if ok {
			t.Error("unexpected event after Done closed")
		}
	case <-time.After(2 * time.Second):
		t.Error("stream still open after Done closed")
	}
}
