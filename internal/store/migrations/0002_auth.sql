ALTER TABLE users
    ADD COLUMN destiny_membership_type INT NOT NULL DEFAULT 0,
    ADD COLUMN destiny_membership_id   TEXT NOT NULL DEFAULT '';

CREATE TABLE sessions (
    id_hash    BYTEA PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE oauth_tokens (
    user_id            BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    access_token       BYTEA NOT NULL,
    refresh_token      BYTEA NOT NULL,
    access_expires_at  TIMESTAMPTZ NOT NULL,
    refresh_expires_at TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
