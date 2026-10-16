package hub

import (
	"slices"
	"testing"
	"time"
)

func newTestHub(buffer int) (*Hub, *time.Time) {
	h := New(buffer)
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }
	return h, &now
}

func receive(t *testing.T, c *Conn) Event {
	t.Helper()
	select {
	case ev := <-c.C:
		return ev
	default:
		t.Fatal("expected an event")
		return Event{}
	}
}

func assertEmpty(t *testing.T, c *Conn) {
	t.Helper()
	select {
	case ev := <-c.C:
		t.Fatalf("unexpected event %+v", ev)
	default:
	}
}

func TestSendTo_OnlyReachesTargetUsers(t *testing.T) {
	h, _ := newTestHub(4)
	a, _ := h.Subscribe(1)
	a2, _ := h.Subscribe(1) // second tab, same user
	b, _ := h.Subscribe(2)

	h.SendTo([]int64{1}, Event{Type: "player_update", Data: "x"})
	if ev := receive(t, a); ev.Type != "player_update" || ev.Data != "x" {
		t.Errorf("a got %+v", ev)
	}
	receive(t, a2)
	assertEmpty(t, b)
}

func TestBroadcast_ReachesEveryone(t *testing.T) {
	h, _ := newTestHub(4)
	a, _ := h.Subscribe(1)
	b, _ := h.Subscribe(2)
	h.Broadcast(Event{Type: "api_status"})
	receive(t, a)
	receive(t, b)
}

func TestSlowConnectionIsDropped(t *testing.T) {
	h, _ := newTestHub(1)
	c, _ := h.Subscribe(1)
	h.SendTo([]int64{1}, Event{Type: "one"})
	h.SendTo([]int64{1}, Event{Type: "two"}) // buffer full → dropped
	select {
	case <-c.Done():
	default:
		t.Fatal("slow connection should be dropped")
	}
	if ev := receive(t, c); ev.Type != "one" {
		t.Errorf("buffered event = %+v", ev)
	}
	h.SendTo([]int64{1}, Event{Type: "three"})
	assertEmpty(t, c)
}

func TestActiveUsers_GracePeriodAfterDisconnect(t *testing.T) {
	h, now := newTestHub(4)
	_, unsubA := h.Subscribe(1)
	h.Subscribe(2)

	if got := h.ActiveUsers(5 * time.Minute); !slices.Equal(got, []int64{1, 2}) {
		t.Fatalf("connected = %v", got)
	}
	unsubA()
	unsubA() // idempotent
	*now = now.Add(4 * time.Minute)
	if got := h.ActiveUsers(5 * time.Minute); !slices.Equal(got, []int64{1, 2}) {
		t.Errorf("within grace = %v", got)
	}
	*now = now.Add(2 * time.Minute)
	if got := h.ActiveUsers(5 * time.Minute); !slices.Equal(got, []int64{2}) {
		t.Errorf("after grace = %v", got)
	}
}

func TestActiveUsers_DroppedConnectionStartsGrace(t *testing.T) {
	h, now := newTestHub(1) // one slot: the second send overflows and drops
	h.Subscribe(1)
	h.SendTo([]int64{1}, Event{Type: "x"})
	h.SendTo([]int64{1}, Event{Type: "y"})
	if got := h.ActiveUsers(time.Minute); !slices.Equal(got, []int64{1}) {
		t.Errorf("right after drop = %v", got)
	}
	*now = now.Add(2 * time.Minute)
	if got := h.ActiveUsers(time.Minute); len(got) != 0 {
		t.Errorf("after grace = %v", got)
	}
}

func TestNew_ClampsBufferToOne(t *testing.T) {
	h := New(0)
	c, _ := h.Subscribe(1)
	h.SendTo([]int64{1}, Event{Type: "x"})
	select {
	case <-c.Done():
		t.Fatal("connection dropped; buffer 0 should clamp to 1")
	default:
	}
	if ev := receive(t, c); ev.Type != "x" {
		t.Errorf("got %+v", ev)
	}
}
