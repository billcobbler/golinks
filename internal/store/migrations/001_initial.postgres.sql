-- Migration 001: initial schema (PostgreSQL)

CREATE TABLE IF NOT EXISTS links (
    id           SERIAL       PRIMARY KEY,
    shortname    TEXT         NOT NULL UNIQUE,
    target_url   TEXT         NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    is_pattern   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_by   TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    click_count  BIGINT       NOT NULL DEFAULT 0,
    last_clicked TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_links_shortname   ON links(shortname);
CREATE INDEX IF NOT EXISTS idx_links_click_count ON links(click_count DESC);
CREATE INDEX IF NOT EXISTS idx_links_pattern     ON links(is_pattern, shortname);

CREATE TABLE IF NOT EXISTS click_events (
    id         SERIAL       PRIMARY KEY,
    link_id    INTEGER      NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    referrer   TEXT         NOT NULL DEFAULT '',
    user_agent TEXT         NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_click_events_link_id    ON click_events(link_id);
CREATE INDEX IF NOT EXISTS idx_click_events_clicked_at ON click_events(clicked_at);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER      PRIMARY KEY,
    applied_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO schema_migrations (version) VALUES (1) ON CONFLICT DO NOTHING;
