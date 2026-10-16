package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/status"
)

func TestSavePollSuccess_StoresStatusAndEmblems(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("p1"), "", SourceSearch))

	at := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	started := at.Add(-4 * time.Minute)
	st := status.Status{State: status.InMatch, Activity: "Burnout", Mode: "Trials of Osiris", StartedAt: &started}
	if err := s.SavePollSuccess(ctx, "p1", st, "/e.jpg", "/bg.jpg", at); err != nil {
		t.Fatal(err)
	}
	p, err := s.Player(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Status.Equal(st) || p.EmblemPath != "/e.jpg" || p.EmblemBackgroundPath != "/bg.jpg" {
		t.Errorf("player = %+v", p)
	}
	if p.LastPolledAt == nil || !p.LastPolledAt.Equal(at) || p.LastSuccessAt == nil || !p.LastSuccessAt.Equal(at) {
		t.Errorf("timestamps = %v, %v", p.LastPolledAt, p.LastSuccessAt)
	}

	// Empty emblem paths keep the previous ones.
	if err := s.SavePollSuccess(ctx, "p1", status.Status{State: status.Offline}, "", "", at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Player(ctx, "p1")
	if p.EmblemBackgroundPath != "/bg.jpg" || p.Status.State != status.Offline {
		t.Errorf("after offline poll = %+v", p)
	}
}

func TestSavePollFailure_OnlyTouchesLastPolled(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("p1"), "", SourceSearch))
	ok := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	must(t, s.SavePollSuccess(ctx, "p1", status.Status{State: status.NotInPvP}, "", "", ok))

	failed := ok.Add(time.Minute)
	if err := s.SavePollFailure(ctx, "p1", failed); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Player(ctx, "p1")
	if p.LastPolledAt == nil || p.LastSuccessAt == nil {
		t.Fatalf("timestamps = %v, %v", p.LastPolledAt, p.LastSuccessAt)
	}
	if !p.LastPolledAt.Equal(failed) || !p.LastSuccessAt.Equal(ok) || p.Status.State != status.NotInPvP {
		t.Errorf("player = %+v", p)
	}
}

func TestPlayerMethods_UnknownPlayer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.Player(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Player: %v", err)
	}
	if err := s.SavePollSuccess(ctx, "nope", status.Status{}, "", "", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("SavePollSuccess: %v", err)
	}
	if err := s.SavePollFailure(ctx, "nope", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("SavePollFailure: %v", err)
	}
}

func TestDeleteUnwatchedPlayers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("kept"), "", SourceSearch))
	must(t, s.AddWatch(ctx, u.ID, ref("dropped"), "", SourceSearch))
	must(t, s.RemoveWatch(ctx, u.ID, "dropped"))

	// A cutoff in the past keeps the recently unwatched player.
	if n, err := s.DeleteUnwatchedPlayers(ctx, time.Now().Add(-time.Hour)); err != nil || n != 0 {
		t.Errorf("past cutoff: n=%d err=%v", n, err)
	}
	// A cutoff in the future deletes it, but never a watched player.
	n, err := s.DeleteUnwatchedPlayers(ctx, time.Now().Add(time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("future cutoff: n=%d err=%v", n, err)
	}
	if _, err := s.Player(ctx, "dropped"); !errors.Is(err, ErrNotFound) {
		t.Errorf("dropped still present: %v", err)
	}
	if _, err := s.Player(ctx, "kept"); err != nil {
		t.Errorf("kept missing: %v", err)
	}
}

func TestPlayer_NeverPolled(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("p1"), "", SourceSearch))
	p, err := s.Player(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status.State != status.Unknown || p.LastPolledAt != nil || p.LastSuccessAt != nil {
		t.Errorf("player = %+v", p)
	}
}
