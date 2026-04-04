package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/billcobbler/golinks/internal/models"
)

//go:embed migrations/001_initial.sql
var migration001 string

//go:embed migrations/002_auth.sql
var migration002 string

// SQLiteStore implements Store using a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLite opens (or creates) a SQLite database at the given path and runs
// any pending migrations.
func NewSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// Limit to a single open connection. SQLite does not handle true concurrent
	// writes, and for :memory: databases each new connection gets an independent
	// empty database — so a pool of >1 would silently lose all data between calls.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// SQLite performs best with WAL mode for concurrent reads.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) migrate() error {
	for _, m := range []string{migration001, migration002} {
		if _, err := s.db.Exec(m); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// ─── Link operations ──────────────────────────────────────────────────────────

// GetLink returns a link by its exact shortname.
func (s *SQLiteStore) GetLink(shortname string) (*models.Link, error) {
	row := s.db.QueryRow(`
		SELECT id, shortname, target_url, description, is_pattern,
		       created_by, created_at, updated_at, click_count, last_clicked
		FROM links WHERE shortname = ?`, shortname)

	link, err := scanLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: shortname}
	}
	return link, err
}

// GetLinkByPath resolves a URL path to a link using exact-then-prefix matching.
// It returns the matched link and any remaining path suffix (for pattern substitution).
//
// Resolution order:
//  1. Exact match on the full path.
//  2. Longest-prefix match among pattern links whose shortname is a prefix of path.
func (s *SQLiteStore) GetLinkByPath(path string) (*models.Link, string, error) {
	// Trim leading slash.
	path = strings.TrimPrefix(path, "/")

	// 1. Exact match (non-pattern or pattern used without extra path).
	link, err := s.GetLink(path)
	if err == nil {
		return link, "", nil
	}
	if !IsNotFound(err) {
		return nil, "", err
	}

	// 2. Prefix match — find the longest pattern shortname that is a prefix of path.
	rows, err := s.db.Query(`
		SELECT id, shortname, target_url, description, is_pattern,
		       created_by, created_at, updated_at, click_count, last_clicked
		FROM links
		WHERE is_pattern = 1
		ORDER BY LENGTH(shortname) DESC`)
	if err != nil {
		return nil, "", fmt.Errorf("prefix match query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		candidate, err := scanLink(rows)
		if err != nil {
			return nil, "", err
		}
		prefix := candidate.Shortname + "/"
		if strings.HasPrefix(path, prefix) {
			remaining := strings.TrimPrefix(path, prefix)
			return candidate, remaining, nil
		}
		// Also match if path exactly equals the shortname (handled by step 1,
		// but belt-and-suspenders for pattern links used bare).
	}

	return nil, "", &ErrNotFound{Shortname: path}
}

// ListLinks returns a paginated, optionally filtered list of links.
func (s *SQLiteStore) ListLinks(search string, offset, limit int) (*models.ListResult, error) {
	if limit <= 0 {
		limit = 50
	}

	// With MaxOpenConns(1) we have exactly one connection. Scan the count
	// immediately so the connection is released before the rows query runs —
	// holding a *sql.Row while calling db.Query would deadlock.
	var total int
	if search != "" {
		like := "%" + search + "%"
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM links WHERE shortname LIKE ? OR description LIKE ? OR target_url LIKE ?`,
			like, like, like).Scan(&total); err != nil {
			return nil, fmt.Errorf("list links count: %w", err)
		}
	} else {
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&total); err != nil {
			return nil, fmt.Errorf("list links count: %w", err)
		}
	}

	var (
		rows *sql.Rows
		err  error
	)
	if search != "" {
		like := "%" + search + "%"
		rows, err = s.db.Query(`
			SELECT id, shortname, target_url, description, is_pattern,
			       created_by, created_at, updated_at, click_count, last_clicked
			FROM links
			WHERE shortname LIKE ? OR description LIKE ? OR target_url LIKE ?
			ORDER BY shortname ASC
			LIMIT ? OFFSET ?`, like, like, like, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, shortname, target_url, description, is_pattern,
			       created_by, created_at, updated_at, click_count, last_clicked
			FROM links
			ORDER BY shortname ASC
			LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list links query: %w", err)
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}

	return &models.ListResult{
		Links:  links,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

// CreateLink inserts a new link. Returns ErrConflict if shortname already exists.
func (s *SQLiteStore) CreateLink(link *models.Link) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(`
		INSERT INTO links (shortname, target_url, description, is_pattern, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		link.Shortname, link.TargetURL, link.Description, boolToInt(link.IsPattern),
		link.CreatedBy, now.Format(time.RFC3339), now.Format(time.RFC3339))

	if err != nil {
		if isSQLiteConstraintError(err) {
			return &ErrConflict{Shortname: link.Shortname}
		}
		return fmt.Errorf("create link: %w", err)
	}

	id, _ := result.LastInsertId()
	link.ID = id
	link.CreatedAt = now
	link.UpdatedAt = now
	return nil
}

// UpdateLink updates a link's mutable fields.
func (s *SQLiteStore) UpdateLink(link *models.Link) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(`
		UPDATE links SET target_url=?, description=?, is_pattern=?, updated_at=?
		WHERE shortname=?`,
		link.TargetURL, link.Description, boolToInt(link.IsPattern),
		now.Format(time.RFC3339), link.Shortname)
	if err != nil {
		return fmt.Errorf("update link: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return &ErrNotFound{Shortname: link.Shortname}
	}

	link.UpdatedAt = now
	return nil
}

// DeleteLink removes a link and its click history by shortname.
func (s *SQLiteStore) DeleteLink(shortname string) error {
	result, err := s.db.Exec(`DELETE FROM links WHERE shortname = ?`, shortname)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return &ErrNotFound{Shortname: shortname}
	}
	return nil
}

// ─── Analytics ────────────────────────────────────────────────────────────────

// RecordClick inserts a click event and bumps the link's counter.
func (s *SQLiteStore) RecordClick(linkID int64, referrer, userAgent string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.Exec(`
		INSERT INTO click_events (link_id, clicked_at, referrer, user_agent)
		VALUES (?, ?, ?, ?)`, linkID, now, referrer, userAgent)
	if err != nil {
		return fmt.Errorf("insert click event: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE links SET click_count = click_count + 1, last_clicked = ?
		WHERE id = ?`, now, linkID)
	if err != nil {
		return fmt.Errorf("update click count: %w", err)
	}

	return tx.Commit()
}

// GetStats returns aggregate analytics data.
func (s *SQLiteStore) GetStats() (*models.Stats, error) {
	stats := &models.Stats{}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&stats.TotalLinks); err != nil {
		return nil, fmt.Errorf("count links: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(click_count), 0) FROM links`).Scan(&stats.TotalClicks); err != nil {
		return nil, fmt.Errorf("sum clicks: %w", err)
	}

	// Top 10 most-clicked links.
	rows, err := s.db.Query(`
		SELECT id, shortname, target_url, description, is_pattern,
		       created_by, created_at, updated_at, click_count, last_clicked
		FROM links ORDER BY click_count DESC LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("top links query: %w", err)
	}
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		stats.TopLinks = append(stats.TopLinks, link)
	}
	rows.Close() // must close before next query — defer would be too late with MaxOpenConns(1)

	// 20 most recent click events.
	clickRows, err := s.db.Query(`
		SELECT ce.id, ce.link_id, l.shortname, ce.clicked_at, ce.referrer, ce.user_agent
		FROM click_events ce
		JOIN links l ON l.id = ce.link_id
		ORDER BY ce.clicked_at DESC LIMIT 20`)
	if err != nil {
		return nil, fmt.Errorf("recent clicks query: %w", err)
	}
	defer clickRows.Close()
	for clickRows.Next() {
		var ce models.ClickEvent
		var clickedAtStr string
		if err := clickRows.Scan(&ce.ID, &ce.LinkID, &ce.Shortname, &clickedAtStr, &ce.Referrer, &ce.UserAgent); err != nil {
			return nil, fmt.Errorf("scan click event: %w", err)
		}
		ce.ClickedAt, _ = time.Parse(time.RFC3339, clickedAtStr)
		stats.RecentClicks = append(stats.RecentClicks, &ce)
	}

	return stats, nil
}

// PruneAnalytics deletes click events older than olderThan. Returns rows deleted.
func (s *SQLiteStore) PruneAnalytics(olderThan time.Time) (int64, error) {
	result, err := s.db.Exec(
		`DELETE FROM click_events WHERE clicked_at < ?`,
		olderThan.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("prune analytics: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// ─── Export / Import ──────────────────────────────────────────────────────────

// ExportLinks returns all links ordered by shortname.
func (s *SQLiteStore) ExportLinks() ([]*models.Link, error) {
	rows, err := s.db.Query(`
		SELECT id, shortname, target_url, description, is_pattern,
		       created_by, created_at, updated_at, click_count, last_clicked
		FROM links ORDER BY shortname ASC`)
	if err != nil {
		return nil, fmt.Errorf("export query: %w", err)
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, nil
}

// ImportLinks inserts or updates links from an export payload.
// If overwrite is true, existing links are replaced; otherwise they are skipped.
// Returns the number of links imported.
func (s *SQLiteStore) ImportLinks(links []*models.Link, overwrite bool) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin import tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	count := 0
	now := time.Now().UTC().Format(time.RFC3339)

	for _, link := range links {
		if overwrite {
			_, err = tx.Exec(`
				INSERT INTO links (shortname, target_url, description, is_pattern, created_by, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(shortname) DO UPDATE SET
					target_url  = excluded.target_url,
					description = excluded.description,
					is_pattern  = excluded.is_pattern,
					updated_at  = ?`,
				link.Shortname, link.TargetURL, link.Description, boolToInt(link.IsPattern),
				link.CreatedBy, now, now, now)
		} else {
			_, err = tx.Exec(`
				INSERT OR IGNORE INTO links (shortname, target_url, description, is_pattern, created_by, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				link.Shortname, link.TargetURL, link.Description, boolToInt(link.IsPattern),
				link.CreatedBy, now, now)
		}
		if err != nil {
			return count, fmt.Errorf("import link %q: %w", link.Shortname, err)
		}
		count++
	}

	return count, tx.Commit()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanLink(s scanner) (*models.Link, error) {
	var link models.Link
	var createdAtStr, updatedAtStr string
	var lastClickedStr *string
	var isPattern int

	err := s.Scan(
		&link.ID, &link.Shortname, &link.TargetURL, &link.Description, &isPattern,
		&link.CreatedBy, &createdAtStr, &updatedAtStr, &link.ClickCount, &lastClickedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scan link: %w", err)
	}

	link.IsPattern = isPattern != 0
	link.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	link.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)

	if lastClickedStr != nil {
		t, _ := time.Parse(time.RFC3339, *lastClickedStr)
		link.LastClicked = &t
	}

	return &link, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isSQLiteConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint failed")
}
