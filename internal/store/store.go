// Package store persists users, watchlists and cached player status in Postgres.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxWatchlist is the most players one user may watch.
const MaxWatchlist = 50

var (
	ErrNotFound      = errors.New("store: not found")
	ErrWatchlistFull = errors.New("store: watchlist is full")
)

type Store struct {
	pool *pgxpool.Pool
}

// Open connects to databaseURL and applies pending migrations.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: parse database url: %w", err)
	}
	return open(ctx, cfg)
}

func open(ctx context.Context, cfg *pgxpool.Config) (*Store, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

type User struct {
	ID                 int64
	BungieMembershipID string
	BungieName         string
}

// UpsertUser creates the user for a Bungie.net membership, or updates their display name.
func (s *Store) UpsertUser(ctx context.Context, bungieMembershipID, bungieName string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (bungie_membership_id, bungie_name) VALUES ($1, $2)
		ON CONFLICT (bungie_membership_id) DO UPDATE SET bungie_name = EXCLUDED.bungie_name
		RETURNING id, bungie_membership_id, bungie_name`,
		bungieMembershipID, bungieName).Scan(&u.ID, &u.BungieMembershipID, &u.BungieName)
	if err != nil {
		return User{}, fmt.Errorf("store: upsert user: %w", err)
	}
	return u, nil
}
