package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func mustUser(t *testing.T, s *Store, id string) User {
	t.Helper()
	u, err := s.UpsertUser(context.Background(), id, "User"+id)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func ref(id string) PlayerRef {
	return PlayerRef{MembershipType: 3, MembershipID: id, BungieName: "Player" + id + "#0001"}
}

func TestAddWatch_ListsInAddedOrder(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	if err := s.AddWatch(ctx, u.ID, ref("p1"), "aimbot", SourceSearch); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWatch(ctx, u.ID, ref("p2"), "", SourceMatch); err != nil {
		t.Fatal(err)
	}
	list, err := s.Watchlist(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].MembershipID != "p1" || list[1].MembershipID != "p2" {
		t.Fatalf("list = %+v", list)
	}
	if list[0].Note != "aimbot" || list[0].Source != SourceSearch || list[0].BungieName != "Playerp1#0001" || list[0].MembershipType != 3 {
		t.Errorf("first = %+v", list[0])
	}
}

func TestAddWatch_ReAddUpdatesNoteWithoutDuplicating(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("p1"), "old", SourceSearch))
	if err := s.AddWatch(ctx, u.ID, ref("p1"), "new", SourceSearch); err != nil {
		t.Fatal(err)
	}
	list, _ := s.Watchlist(ctx, u.ID)
	if len(list) != 1 || list[0].Note != "new" {
		t.Errorf("list = %+v", list)
	}
}

func TestAddWatch_EnforcesCap(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	for i := 0; i < MaxWatchlist; i++ {
		if err := s.AddWatch(ctx, u.ID, ref(fmt.Sprintf("p%02d", i)), "", SourceSearch); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if err := s.AddWatch(ctx, u.ID, ref("overflow"), "", SourceSearch); !errors.Is(err, ErrWatchlistFull) {
		t.Errorf("err = %v, want ErrWatchlistFull", err)
	}
	// Re-adding an existing player at the cap is still allowed.
	if err := s.AddWatch(ctx, u.ID, ref("p00"), "note", SourceSearch); err != nil {
		t.Errorf("re-add at cap: %v", err)
	}
}

func TestAddWatch_UnknownUser(t *testing.T) {
	s := openTestStore(t)
	if err := s.AddWatch(context.Background(), 999, ref("p1"), "", SourceSearch); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateNoteAndRemoveWatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("p1"), "", SourceSearch))

	if err := s.UpdateNote(ctx, u.ID, "p1", "wallhacks"); err != nil {
		t.Fatal(err)
	}
	list, _ := s.Watchlist(ctx, u.ID)
	if list[0].Note != "wallhacks" {
		t.Errorf("note = %q", list[0].Note)
	}
	if err := s.UpdateNote(ctx, u.ID, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing: %v", err)
	}

	if err := s.RemoveWatch(ctx, u.ID, "p1"); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Watchlist(ctx, u.ID); len(list) != 0 {
		t.Errorf("list after remove = %+v", list)
	}
	if err := s.RemoveWatch(ctx, u.ID, "p1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second remove: %v", err)
	}
}

func TestWatchedByAndWatchersOf(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	a, b, c := mustUser(t, s, "a"), mustUser(t, s, "b"), mustUser(t, s, "c")
	must(t, s.AddWatch(ctx, a.ID, ref("p1"), "", SourceSearch))
	must(t, s.AddWatch(ctx, b.ID, ref("p1"), "", SourceSearch))
	must(t, s.AddWatch(ctx, b.ID, ref("p2"), "", SourceSearch))
	must(t, s.AddWatch(ctx, c.ID, ref("p3"), "", SourceSearch))

	players, err := s.WatchedBy(ctx, []int64{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 2 || players[0].MembershipID != "p1" || players[1].MembershipID != "p2" {
		t.Errorf("WatchedBy = %+v, want p1, p2 once each", players)
	}
	if none, _ := s.WatchedBy(ctx, nil); len(none) != 0 {
		t.Errorf("WatchedBy(nil) = %+v", none)
	}

	watchers, err := s.WatchersOf(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(watchers) != 2 || watchers[0] != a.ID || watchers[1] != b.ID {
		t.Errorf("WatchersOf(p1) = %v", watchers)
	}
}

func TestAddWatch_CapHoldsUnderConcurrency(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	for i := 0; i < MaxWatchlist-1; i++ {
		must(t, s.AddWatch(ctx, u.ID, ref(fmt.Sprintf("p%02d", i)), "", SourceSearch))
	}
	const n = 10
	// Pre-open pool connections so the goroutines truly overlap instead of queueing on dialing.
	var conns []*pgxpool.Conn
	for i := 0; i < int(s.pool.Config().MaxConns) && i < n; i++ {
		c, err := s.pool.Acquire(ctx)
		must(t, err)
		conns = append(conns, c)
	}
	for _, c := range conns {
		c.Release()
	}
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = s.AddWatch(ctx, u.ID, ref(fmt.Sprintf("new%02d", i)), "", SourceSearch)
		}(i)
	}
	close(start)
	wg.Wait()
	var ok, full int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrWatchlistFull):
			full++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || full != n-1 {
		t.Errorf("ok=%d full=%d, want 1 and %d", ok, full, n-1)
	}
	list, err := s.Watchlist(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != MaxWatchlist {
		t.Errorf("len(list) = %d, want %d", len(list), MaxWatchlist)
	}
}
