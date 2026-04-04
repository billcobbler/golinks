-- Migration 001: initial schema

CREATE TABLE IF NOT EXISTS links (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    shortname    TEXT     NOT NULL UNIQUE,
    target_url   TEXT     NOT NULL,
    description  TEXT     NOT NULL DEFAULT '',
    is_pattern   INTEGER  NOT NULL DEFAULT 0,
    created_by   TEXT     NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    click_count  INTEGER  NOT NULL DEFAULT 0,
    last_clicked DATETIME
);

CREATE INDEX IF NOT EXISTS idx_links_shortname   ON links(shortname);
CREATE INDEX IF NOT EXISTS idx_links_click_count ON links(click_count DESC);

-- Pattern links are queried by prefix; index on is_pattern + shortname helps.
CREATE INDEX IF NOT EXISTS idx_links_pattern ON links(is_pattern, shortname);

CREATE TABLE IF NOT EXISTS click_events (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    link_id    INTEGER  NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    referrer   TEXT     NOT NULL DEFAULT '',
    user_agent TEXT     NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_click_events_link_id    ON click_events(link_id);
CREATE INDEX IF NOT EXISTS idx_click_events_clicked_at ON click_events(clicked_at);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER  PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

INSERT OR IGNORE INTO schema_migrations (version) VALUES (1);
