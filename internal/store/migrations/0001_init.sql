CREATE TABLE users (
    id                   BIGSERIAL PRIMARY KEY,
    bungie_membership_id TEXT NOT NULL UNIQUE,
    bungie_name          TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE players (
    membership_id          TEXT PRIMARY KEY,
    membership_type        INT NOT NULL,
    bungie_name            TEXT NOT NULL DEFAULT '',
    emblem_path            TEXT NOT NULL DEFAULT '',
    emblem_background_path TEXT NOT NULL DEFAULT '',
    status                 JSONB NOT NULL DEFAULT '{}',
    last_polled_at         TIMESTAMPTZ,
    last_success_at        TIMESTAMPTZ,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE watch_entries (
    user_id       BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    membership_id TEXT NOT NULL REFERENCES players (membership_id) ON DELETE CASCADE,
    note          TEXT NOT NULL DEFAULT '',
    source        TEXT NOT NULL CHECK (source IN ('search', 'match')),
    added_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, membership_id)
);

CREATE INDEX watch_entries_membership_idx ON watch_entries (membership_id);
