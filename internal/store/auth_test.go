package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokens_UpsertAndLoad(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	exp := time.Date(2026, 10, 15, 19, 0, 0, 0, time.UTC)

	if _, err := s.Tokens(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no tokens yet: %v", err)
	}
	must(t, s.SaveTokens(ctx, Tokens{UserID: u.ID, AccessToken: []byte("a1"), RefreshToken: []byte("r1"), AccessExpiresAt: exp, RefreshExpiresAt: exp.Add(time.Hour)}))
	must(t, s.SaveTokens(ctx, Tokens{UserID: u.ID, AccessToken: []byte("a2"), RefreshToken: []byte("r2"), AccessExpiresAt: exp.Add(time.Minute), RefreshExpiresAt: exp.Add(2 * time.Hour)}))

	got, err := s.Tokens(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.AccessToken, []byte("a2")) || !bytes.Equal(got.RefreshToken, []byte("r2")) ||
		!got.AccessExpiresAt.Equal(exp.Add(time.Minute)) || !got.RefreshExpiresAt.Equal(exp.Add(2*time.Hour)) {
		t.Errorf("tokens = %+v", got)
	}
}

func TestSessions_LifecycleAndExpiry(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	now := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	live, expired := []byte("live-hash"), []byte("old-hash")
	must(t, s.CreateSession(ctx, live, u.ID, now.Add(time.Hour)))
	must(t, s.CreateSession(ctx, expired, u.ID, now.Add(-time.Minute)))

	got, err := s.SessionUser(ctx, live, now)
	if err != nil || got.ID != u.ID {
		t.Fatalf("live session: %+v, %v", got, err)
	}
	if _, err := s.SessionUser(ctx, expired, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session: %v", err)
	}
	if _, err := s.SessionUser(ctx, []byte("unknown"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown session: %v", err)
	}

	n, err := s.DeleteExpiredSessions(ctx, now)
	if err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions = %d, %v; want 1", n, err)
	}
	must(t, s.DeleteSession(ctx, live))
	must(t, s.DeleteSession(ctx, live)) // deleting twice is fine
	if _, err := s.SessionUser(ctx, live, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted session: %v", err)
	}
}

func TestDeleteUser_CascadesSessionsAndTokens(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u := mustUser(t, s, "u1")
	now := time.Now()
	must(t, s.CreateSession(ctx, []byte("h"), u.ID, now.Add(time.Hour)))
	must(t, s.SaveTokens(ctx, Tokens{UserID: u.ID, AccessToken: []byte("a"), RefreshToken: []byte("r"), AccessExpiresAt: now, RefreshExpiresAt: now}))
	must(t, s.DeleteUser(ctx, u.ID))
	if _, err := s.SessionUser(ctx, []byte("h"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("session survived user deletion: %v", err)
	}
	if _, err := s.Tokens(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("tokens survived user deletion: %v", err)
	}
}
