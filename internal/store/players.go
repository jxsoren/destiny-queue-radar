package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jxsoren/destiny-queue-radar/internal/status"
)

func (s *Store) Player(ctx context.Context, membershipID string) (Player, error) {
	var p Player
	err := scanPlayer(s.pool.QueryRow(ctx,
		`SELECT `+playerColumns+` FROM players p WHERE p.membership_id = $1`, membershipID), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return Player{}, ErrNotFound
	}
	if err != nil {
		return Player{}, fmt.Errorf("store: player: %w", err)
	}
	return p, nil
}

// SavePollSuccess stores a successful poll. Empty emblem paths keep the previous ones.
func (s *Store) SavePollSuccess(ctx context.Context, membershipID string, st status.Status, emblemPath, emblemBackgroundPath string, at time.Time) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("store: encode status: %w", err)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE players SET
			status = $2,
			emblem_path = COALESCE(NULLIF($3, ''), emblem_path),
			emblem_background_path = COALESCE(NULLIF($4, ''), emblem_background_path),
			last_polled_at = $5,
			last_success_at = $5,
			updated_at = now()
		WHERE membership_id = $1`,
		membershipID, string(raw), emblemPath, emblemBackgroundPath, at)
	if err != nil {
		return fmt.Errorf("store: save poll: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SavePollFailure records a failed poll attempt without touching the last known status.
func (s *Store) SavePollFailure(ctx context.Context, membershipID string, at time.Time) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE players SET last_polled_at = $2 WHERE membership_id = $1`, membershipID, at)
	if err != nil {
		return fmt.Errorf("store: save poll failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUnwatchedPlayers removes players nobody watches whose last change is before cutoff.
func (s *Store) DeleteUnwatchedPlayers(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM players p
		WHERE p.updated_at < $1
		  AND NOT EXISTS (SELECT 1 FROM watch_entries w WHERE w.membership_id = p.membership_id)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: delete unwatched players: %w", err)
	}
	return tag.RowsAffected(), nil
}
