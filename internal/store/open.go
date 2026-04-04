package store

import "strings"

// Open returns a Store backed by SQLite or Postgres depending on the DSN.
//
//   - Any path ending in ".db" or not starting with "postgres" → SQLite
//   - "postgres://…" or "postgresql://…"                       → Postgres
func Open(dsn string) (Store, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return NewPostgres(dsn)
	}
	return NewSQLite(dsn)
}
