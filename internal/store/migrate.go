package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// migrate applies every embedded migration not yet recorded in schema_migrations, in filename order.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("store: list migrations: %w", err)
	}
	sort.Strings(files)
	for _, f := range files {
		version := strings.TrimSuffix(path.Base(f), ".sql")
		if err := applyMigration(ctx, pool, version, f); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, version, file string) error {
	sql, err := migrations.ReadFile(file)
	if err != nil {
		return fmt.Errorf("store: read %s: %w", file, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin %s: %w", version, err)
	}
	defer tx.Rollback(ctx)

	var applied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
		return fmt.Errorf("store: check %s: %w", version, err)
	}
	if applied {
		return nil
	}
	// Simple protocol allows multiple statements in one migration file.
	if _, err := tx.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("store: apply %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("store: record %s: %w", version, err)
	}
	return tx.Commit(ctx)
}
