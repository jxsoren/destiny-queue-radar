package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUser_RoundTripsDestinyMembership(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	if u.DestinyMembershipType != 0 || u.DestinyMembershipID != "" {
		t.Errorf("new user = %+v", u)
	}
	must(t, s.SetDestinyMembership(ctx, u.ID, 3, "4611"))
	got, err := s.User(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DestinyMembershipType != 3 || got.DestinyMembershipID != "4611" || got.BungieName != u.BungieName {
		t.Errorf("user = %+v", got)
	}
	again, err := s.UpsertUser(ctx, u.BungieMembershipID, "Renamed#0001")
	if err != nil || again.DestinyMembershipID != "4611" {
		t.Errorf("UpsertUser should keep the destiny membership: %+v, %v", again, err)
	}
}

func TestUserMethods_UnknownUser(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.User(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("User: %v", err)
	}
	if err := s.SetDestinyMembership(ctx, 999, 3, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetDestinyMembership: %v", err)
	}
	if err := s.DeleteUser(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteUser: %v", err)
	}
}

func TestDeleteUser_RemovesWatchesAndRestartsCleanupClock(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	must(t, s.AddWatch(ctx, u.ID, ref("p1"), "", SourceSearch))
	// Pretend the player was last touched long ago.
	if _, err := s.pool.Exec(ctx, `UPDATE players SET updated_at = now() - interval '40 days'`); err != nil {
		t.Fatal(err)
	}

	must(t, s.DeleteUser(ctx, u.ID))
	if _, err := s.User(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("user still present: %v", err)
	}
	// The player is unwatched now, but its grace period restarted at deletion.
	n, err := s.DeleteUnwatchedPlayers(ctx, time.Now().Add(-30*24*time.Hour))
	if err != nil || n != 0 {
		t.Errorf("cleanup removed %d players (err %v); grace period should restart at account deletion", n, err)
	}
}
