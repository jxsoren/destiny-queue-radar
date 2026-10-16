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

const (
	SourceSearch = "search"
	SourceMatch  = "match"
)

// PlayerRef identifies a Destiny player.
type PlayerRef struct {
	MembershipType int64
	MembershipID   string
	BungieName     string
}

// Player is the shared, cached state of one watched player.
type Player struct {
	PlayerRef
	EmblemPath           string
	EmblemBackgroundPath string
	Status               status.Status
	LastPolledAt         *time.Time
	LastSuccessAt        *time.Time
}

// WatchedPlayer is a Player as it appears on one user's watchlist.
type WatchedPlayer struct {
	Player
	Note    string
	Source  string
	AddedAt time.Time
}

const playerColumns = `p.membership_id, p.membership_type, p.bungie_name, p.emblem_path,
	p.emblem_background_path, p.status, p.last_polled_at, p.last_success_at`

// scanPlayer scans playerColumns, followed by any extra destinations.
func scanPlayer(row pgx.Row, p *Player, extra ...any) error {
	var rawStatus []byte
	dest := append([]any{
		&p.MembershipID, &p.MembershipType, &p.BungieName, &p.EmblemPath,
		&p.EmblemBackgroundPath, &rawStatus, &p.LastPolledAt, &p.LastSuccessAt,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return err
	}
	if err := json.Unmarshal(rawStatus, &p.Status); err != nil {
		return fmt.Errorf("store: decode status: %w", err)
	}
	return nil
}

// AddWatch puts p on the user's watchlist (or updates the note if already there).
// It returns ErrWatchlistFull at MaxWatchlist and ErrNotFound for an unknown user.
func (s *Store) AddWatch(ctx context.Context, userID int64, p PlayerRef, note, source string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the user row so concurrent adds can't both slip under the cap.
	var id int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("store: lock user: %w", err)
	}
	var others int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM watch_entries WHERE user_id = $1 AND membership_id <> $2`,
		userID, p.MembershipID).Scan(&others); err != nil {
		return fmt.Errorf("store: count watchlist: %w", err)
	}
	if others >= MaxWatchlist {
		return ErrWatchlistFull
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO players (membership_id, membership_type, bungie_name) VALUES ($1, $2, $3)
		ON CONFLICT (membership_id) DO UPDATE SET
			membership_type = EXCLUDED.membership_type,
			bungie_name = COALESCE(NULLIF(EXCLUDED.bungie_name, ''), players.bungie_name),
			-- Touching updated_at stops a concurrent DeleteUnwatchedPlayers from removing this
			-- player before the watch entry below is inserted.
			updated_at = now()`,
		p.MembershipID, p.MembershipType, p.BungieName); err != nil {
		return fmt.Errorf("store: upsert player: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO watch_entries (user_id, membership_id, note, source) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, membership_id) DO UPDATE SET note = EXCLUDED.note`,
		userID, p.MembershipID, note, source); err != nil {
		return fmt.Errorf("store: add watch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit watch: %w", err)
	}
	return nil
}

func (s *Store) UpdateNote(ctx context.Context, userID int64, membershipID, note string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE watch_entries SET note = $3 WHERE user_id = $1 AND membership_id = $2`,
		userID, membershipID, note)
	if err != nil {
		return fmt.Errorf("store: update note: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveWatch takes a player off the user's watchlist. The player's updated_at is touched so
// DeleteUnwatchedPlayers measures its grace period from the player's last change (this removal
// or any later poll result).
func (s *Store) RemoveWatch(ctx context.Context, userID int64, membershipID string) error {
	tag, err := s.pool.Exec(ctx, `
		WITH removed AS (
			DELETE FROM watch_entries WHERE user_id = $1 AND membership_id = $2 RETURNING membership_id
		)
		UPDATE players SET updated_at = now() WHERE membership_id IN (SELECT membership_id FROM removed)`,
		userID, membershipID)
	if err != nil {
		return fmt.Errorf("store: remove watch: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Watchlist returns the user's watched players in the order they were added.
func (s *Store) Watchlist(ctx context.Context, userID int64) ([]WatchedPlayer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+playerColumns+`, w.note, w.source, w.added_at
		FROM watch_entries w JOIN players p ON p.membership_id = w.membership_id
		WHERE w.user_id = $1
		ORDER BY w.added_at, p.membership_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: watchlist: %w", err)
	}
	defer rows.Close()
	var out []WatchedPlayer
	for rows.Next() {
		var wp WatchedPlayer
		if err := scanPlayer(rows, &wp.Player, &wp.Note, &wp.Source, &wp.AddedAt); err != nil {
			return nil, fmt.Errorf("store: scan watchlist: %w", err)
		}
		out = append(out, wp)
	}
	return out, rows.Err()
}

// WatchedBy returns every distinct player on any of the given users' watchlists.
func (s *Store) WatchedBy(ctx context.Context, userIDs []int64) ([]Player, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+playerColumns+` FROM players p
		WHERE EXISTS (
			SELECT 1 FROM watch_entries w WHERE w.membership_id = p.membership_id AND w.user_id = ANY($1)
		)
		ORDER BY p.membership_id`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("store: watched by: %w", err)
	}
	defer rows.Close()
	var out []Player
	for rows.Next() {
		var p Player
		if err := scanPlayer(rows, &p); err != nil {
			return nil, fmt.Errorf("store: scan player: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// WatchersOf returns the IDs of users watching the player, ascending.
func (s *Store) WatchersOf(ctx context.Context, membershipID string) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id FROM watch_entries WHERE membership_id = $1 ORDER BY user_id`, membershipID)
	if err != nil {
		return nil, fmt.Errorf("store: watchers of: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("store: scan watchers: %w", err)
	}
	return ids, nil
}
