package store

import (
	"context"
	"sync"
	"testing"
)

func TestOpen_MigrationsAreIdempotent(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		s, err := open(ctx, cfg.Copy())
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		var n int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("open #%d: %d migrations recorded, want 2", i+1, n)
		}
		s.Close()
	}
}

func TestUpsertUser_InsertsThenUpdatesName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u1, err := s.UpsertUser(ctx, "4611", "Guardian#1993")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.UpsertUser(ctx, "4611", "Renamed#1993")
	if err != nil {
		t.Fatal(err)
	}
	if u1.ID == 0 || u1.ID != u2.ID {
		t.Errorf("ids = %d, %d; want same non-zero id", u1.ID, u2.ID)
	}
	if u2.BungieName != "Renamed#1993" || u2.BungieMembershipID != "4611" {
		t.Errorf("user = %+v", u2)
	}
}

func TestOpen_ConcurrentStartupIsSafe(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()
	const n = 4
	stores := make([]*Store, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			stores[i], errs[i] = open(ctx, cfg.Copy())
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("open #%d: %v", i, errs[i])
			continue
		}
		stores[i].Close()
	}
}
