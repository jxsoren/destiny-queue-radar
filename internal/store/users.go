package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const userColumns = `id, bungie_membership_id, bungie_name, destiny_membership_type, destiny_membership_id`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.BungieMembershipID, &u.BungieName, &u.DestinyMembershipType, &u.DestinyMembershipID)
	return u, err
}

func (s *Store) User(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: user: %w", err)
	}
	return u, nil
}

// SetDestinyMembership records which Destiny account the user plays on (for recent matches).
func (s *Store) SetDestinyMembership(ctx context.Context, userID, membershipType int64, membershipID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET destiny_membership_type = $2, destiny_membership_id = $3 WHERE id = $1`,
		userID, membershipType, membershipID)
	if err != nil {
		return fmt.Errorf("store: set destiny membership: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes the user, their watchlist, sessions and tokens. Their watched players'
// cleanup grace period restarts now, as it would after removing each watch by hand.
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback(ctx)
	// Lock the user row first, then players: the same order AddWatch uses, so the two can't deadlock.
	var id int64
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: lock user: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE players SET updated_at = now()
		WHERE membership_id IN (SELECT membership_id FROM watch_entries WHERE user_id = $1)`, userID); err != nil {
		return fmt.Errorf("store: touch players: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("store: delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit delete user: %w", err)
	}
	return nil
}
