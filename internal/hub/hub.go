// Package hub fans out live events to connected dashboards and tracks who is online.
package hub

import (
	"slices"
	"sync"
	"time"
)

type Event struct {
	Type string
	Data any
}

// Conn is one open dashboard connection.
type Conn struct {
	C    <-chan Event
	ch   chan Event
	done chan struct{}
	once sync.Once
}

// Done is closed when the connection is unsubscribed or dropped for falling behind.
func (c *Conn) Done() <-chan struct{} { return c.done }

func (c *Conn) close() { c.once.Do(func() { close(c.done) }) }

// Hub is safe for concurrent use.
type Hub struct {
	mu       sync.Mutex
	buffer   int
	conns    map[int64]map[*Conn]struct{}
	lastSeen map[int64]time.Time // users with no open connection, by disconnect time
	now      func() time.Time
}

// New returns a hub whose connections buffer up to buffer events before being dropped.
func New(buffer int) *Hub {
	return &Hub{
		buffer:   buffer,
		conns:    map[int64]map[*Conn]struct{}{},
		lastSeen: map[int64]time.Time{},
		now:      time.Now,
	}
}

// Subscribe opens a connection for the user. Call the returned func when the client goes away.
func (h *Hub) Subscribe(userID int64) (*Conn, func()) {
	ch := make(chan Event, h.buffer)
	c := &Conn{C: ch, ch: ch, done: make(chan struct{})}
	h.mu.Lock()
	if h.conns[userID] == nil {
		h.conns[userID] = map[*Conn]struct{}{}
	}
	h.conns[userID][c] = struct{}{}
	delete(h.lastSeen, userID)
	h.mu.Unlock()
	return c, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.removeLocked(userID, c)
	}
}

func (h *Hub) removeLocked(userID int64, c *Conn) {
	if conns, ok := h.conns[userID]; ok {
		if _, ok := conns[c]; ok {
			delete(conns, c)
			if len(conns) == 0 {
				delete(h.conns, userID)
				h.lastSeen[userID] = h.now()
			}
		}
	}
	c.close()
}

// SendTo delivers ev to every connection of the given users. A connection whose buffer is
// full is dropped; the client reconnects and receives a fresh snapshot.
func (h *Hub) SendTo(userIDs []int64, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range userIDs {
		for c := range h.conns[id] {
			h.deliverLocked(id, c, ev)
		}
	}
}

// Broadcast delivers ev to every connection.
func (h *Hub) Broadcast(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, conns := range h.conns {
		for c := range conns {
			h.deliverLocked(id, c, ev)
		}
	}
}

func (h *Hub) deliverLocked(userID int64, c *Conn, ev Event) {
	select {
	case c.ch <- ev:
	default:
		h.removeLocked(userID, c)
	}
}

// ActiveUsers returns users with an open connection or one closed less than grace ago, ascending.
func (h *Hub) ActiveUsers(grace time.Duration) []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	ids := make([]int64, 0, len(h.conns)+len(h.lastSeen))
	for id := range h.conns {
		ids = append(ids, id)
	}
	for id, seen := range h.lastSeen {
		if now.Sub(seen) < grace {
			ids = append(ids, id)
		} else {
			delete(h.lastSeen, id)
		}
	}
	slices.Sort(ids)
	return ids
}
