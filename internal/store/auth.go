package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Tokens are a user's Bungie OAuth tokens, already encrypted by the caller.
type Tokens struct {
	UserID           int64
	AccessToken      []byte
	RefreshToken     []byte
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

func (s *Store) SaveTokens(ctx context.Context, t Tokens) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_tokens (user_id, access_token, refresh_token, access_expires_at, refresh_expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			access_expires_at = EXCLUDED.access_expires_at,
			refresh_expires_at = EXCLUDED.refresh_expires_at,
			updated_at = now()`,
		t.UserID, t.AccessToken, t.RefreshToken, t.AccessExpiresAt, t.RefreshExpiresAt)
	if err != nil {
		return fmt.Errorf("store: save tokens: %w", err)
	}
	return nil
}

func (s *Store) Tokens(ctx context.Context, userID int64) (Tokens, error) {
	t := Tokens{UserID: userID}
	err := s.pool.QueryRow(ctx, `
		SELECT access_token, refresh_token, access_expires_at, refresh_expires_at
		FROM oauth_tokens WHERE user_id = $1`, userID).
		Scan(&t.AccessToken, &t.RefreshToken, &t.AccessExpiresAt, &t.RefreshExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tokens{}, ErrNotFound
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("store: tokens: %w", err)
	}
	return t, nil
}

// CreateSession stores a session by the hash of its cookie token (never the token itself).
func (s *Store) CreateSession(ctx context.Context, idHash []byte, userID int64, expiresAt time.Time) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (id_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		idHash, userID, expiresAt); err != nil {
		return fmt.Errorf("store: create session: %w", err)
	}
	return nil
}

// SessionUser returns the user of an unexpired session, or ErrNotFound.
func (s *Store) SessionUser(ctx context.Context, idHash []byte, now time.Time) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE id = (SELECT user_id FROM sessions WHERE id_hash = $1 AND expires_at > $2)`, idHash, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: session user: %w", err)
	}
	return u, nil
}

// DeleteSession removes a session; deleting one that does not exist is not an error.
func (s *Store) DeleteSession(ctx context.Context, idHash []byte) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id_hash = $1`, idHash); err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("store: delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
