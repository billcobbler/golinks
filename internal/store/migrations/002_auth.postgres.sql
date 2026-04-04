-- Migration 002: authentication tables (PostgreSQL)

CREATE TABLE IF NOT EXISTS users (
    id          SERIAL       PRIMARY KEY,
    username    TEXT         UNIQUE,
    email       TEXT         UNIQUE,
    password    TEXT,
    provider    TEXT         NOT NULL DEFAULT 'local',
    provider_id TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider
    ON users(provider, provider_id)
    WHERE provider_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT         PRIMARY KEY,
    user_id    INTEGER      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username   TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ  NOT NULL,
    last_seen  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS api_tokens (
    id         SERIAL       PRIMARY KEY,
    user_id    INTEGER      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT         NOT NULL UNIQUE,
    label      TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_used  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS oauth_state (
    token      TEXT         PRIMARY KEY,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ  NOT NULL
);

INSERT INTO schema_migrations (version) VALUES (2) ON CONFLICT DO NOTHING;
