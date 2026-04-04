-- Migration 002: authentication tables
-- Users, sessions, API tokens, and OAuth state.
-- All tables use IF NOT EXISTS so this migration is idempotent.

CREATE TABLE IF NOT EXISTS users (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    username    TEXT     UNIQUE,                          -- NULL for OAuth-only users
    email       TEXT     UNIQUE,                          -- NULL for local-only users
    password    TEXT,                                     -- bcrypt hash; NULL for OAuth users
    provider    TEXT     NOT NULL DEFAULT 'local',        -- 'local' | 'google' | 'github'
    provider_id TEXT,                                     -- OAuth subject; NULL for local users
    created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Ensure (provider, provider_id) is unique when both are set.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider
    ON users(provider, provider_id)
    WHERE provider_id IS NOT NULL;

-- Sessions: the id is SHA-256(raw_token) stored hex-encoded.
-- Username is denormalised here so middleware doesn't need a second query.
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT     PRIMARY KEY,   -- SHA-256(raw_token), hex
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username   TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at DATETIME NOT NULL,
    last_seen  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- API tokens: used by CLI and browser extension.
-- The id is SHA-256(raw_token) hex-encoded; the raw token is shown once on creation.
CREATE TABLE IF NOT EXISTS api_tokens (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT     NOT NULL UNIQUE,   -- SHA-256(raw_token), hex
    label      TEXT     NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used  DATETIME
);

-- Short-lived nonces for OAuth CSRF protection.
CREATE TABLE IF NOT EXISTS oauth_state (
    token      TEXT     PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at DATETIME NOT NULL
);

INSERT OR IGNORE INTO schema_migrations (version) VALUES (2);
