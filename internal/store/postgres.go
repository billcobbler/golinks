package store

// PostgresStore implements Store using a PostgreSQL database.
// Uses the pgx/v5 stdlib shim so all code remains standard database/sql.

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/billcobbler/golinks/internal/models"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/001_initial.postgres.sql
var pgMigration001 string

//go:embed migrations/002_auth.postgres.sql
var pgMigration002 string

// PostgresStore wraps a *sql.DB connected to PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgres opens a Postgres connection and runs migrations.
func NewPostgres(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres db: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	s := &PostgresStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}
	return s, nil
}

func (s *PostgresStore) migrate() error {
	for _, m := range []string{pgMigration001, pgMigration002} {
		if _, err := s.db.Exec(m); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) Close() error { return s.db.Close() }

// ─── Link operations ──────────────────────────────────────────────────────────

func (s *PostgresStore) GetLink(shortname string) (*models.Link, error) {
	row := s.db.QueryRow(`
		SELECT id, shortname, target_url, description, is_pattern,
		       created_by, created_at, updated_at, click_count, last_clicked
		FROM links WHERE shortname = $1`, shortname)
	link, err := scanLinkPG(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: shortname}
	}
	return link, err
}

func (s *PostgresStore) GetLinkByPath(path string) (*models.Link, string, error) {
	path = strings.TrimPrefix(path, "/")
	link, err := s.GetLink(path)
	if err == nil {
		return link, "", nil
	}
	if !IsNotFound(err) {
		return nil, "", err
	}

	rows, err := s.db.Query(`
		SELECT id, shortname, target_url, description, is_pattern,
		       created_by, created_at, updated_at, click_count, last_clicked
		FROM links WHERE is_pattern = true ORDER BY LENGTH(shortname) DESC`)
	if err != nil {
		return nil, "", fmt.Errorf("prefix match query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		candidate, err := scanLinkPGRows(rows)
		if err != nil {
			return nil, "", err
		}
		prefix := candidate.Shortname + "/"
		if strings.HasPrefix(path, prefix) {
			return candidate, strings.TrimPrefix(path, prefix), nil
		}
	}
	return nil, "", &ErrNotFound{Shortname: path}
}

func (s *PostgresStore) ListLinks(search string, offset, limit int) (*models.ListResult, error) {
	if limit <= 0 {
		limit = 50
	}
	var total int
	if search != "" {
		like := "%" + search + "%"
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM links WHERE shortname ILIKE $1 OR description ILIKE $1 OR target_url ILIKE $1`,
			like).Scan(&total); err != nil {
			return nil, fmt.Errorf("list links count: %w", err)
		}
	} else {
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&total); err != nil {
			return nil, fmt.Errorf("list links count: %w", err)
		}
	}

	var rows *sql.Rows
	var err error
	if search != "" {
		like := "%" + search + "%"
		rows, err = s.db.Query(`
			SELECT id, shortname, target_url, description, is_pattern,
			       created_by, created_at, updated_at, click_count, last_clicked
			FROM links
			WHERE shortname ILIKE $1 OR description ILIKE $1 OR target_url ILIKE $1
			ORDER BY shortname ASC LIMIT $2 OFFSET $3`, like, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, shortname, target_url, description, is_pattern,
			       created_by, created_at, updated_at, click_count, last_clicked
			FROM links ORDER BY shortname ASC LIMIT $1 OFFSET $2`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list links query: %w", err)
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link, err := scanLinkPGRows(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return &models.ListResult{Links: links, Total: total, Offset: offset, Limit: limit}, nil
}

func (s *PostgresStore) CreateLink(link *models.Link) error {
	now := time.Now().UTC()
	err := s.db.QueryRow(`
		INSERT INTO links (shortname, target_url, description, is_pattern, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		link.Shortname, link.TargetURL, link.Description, link.IsPattern,
		link.CreatedBy, now, now).Scan(&link.ID)
	if err != nil {
		if isPostgresConflictError(err) {
			return &ErrConflict{Shortname: link.Shortname}
		}
		return fmt.Errorf("create link: %w", err)
	}
	link.CreatedAt, link.UpdatedAt = now, now
	return nil
}

func (s *PostgresStore) UpdateLink(link *models.Link) error {
	now := time.Now().UTC()
	result, err := s.db.Exec(`
		UPDATE links SET target_url=$1, description=$2, is_pattern=$3, updated_at=$4
		WHERE shortname=$5`,
		link.TargetURL, link.Description, link.IsPattern, now, link.Shortname)
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

func (s *PostgresStore) DeleteLink(shortname string) error {
	result, err := s.db.Exec(`DELETE FROM links WHERE shortname = $1`, shortname)
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

func (s *PostgresStore) RecordClick(linkID int64, referrer, userAgent string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	now := time.Now().UTC()
	if _, err = tx.Exec(`INSERT INTO click_events (link_id, clicked_at, referrer, user_agent) VALUES ($1,$2,$3,$4)`,
		linkID, now, referrer, userAgent); err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	if _, err = tx.Exec(`UPDATE links SET click_count=click_count+1, last_clicked=$1 WHERE id=$2`, now, linkID); err != nil {
		return fmt.Errorf("update click count: %w", err)
	}
	return tx.Commit()
}

func (s *PostgresStore) GetStats() (*models.Stats, error) {
	stats := &models.Stats{}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&stats.TotalLinks); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(click_count),0) FROM links`).Scan(&stats.TotalClicks); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id,shortname,target_url,description,is_pattern,created_by,created_at,updated_at,click_count,last_clicked
		FROM links ORDER BY click_count DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		link, err := scanLinkPGRows(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		stats.TopLinks = append(stats.TopLinks, link)
	}
	rows.Close()

	clickRows, err := s.db.Query(`
		SELECT ce.id,ce.link_id,l.shortname,ce.clicked_at,ce.referrer,ce.user_agent
		FROM click_events ce JOIN links l ON l.id=ce.link_id
		ORDER BY ce.clicked_at DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer clickRows.Close()
	for clickRows.Next() {
		var ce models.ClickEvent
		if err := clickRows.Scan(&ce.ID, &ce.LinkID, &ce.Shortname, &ce.ClickedAt, &ce.Referrer, &ce.UserAgent); err != nil {
			return nil, err
		}
		stats.RecentClicks = append(stats.RecentClicks, &ce)
	}
	return stats, nil
}

func (s *PostgresStore) PruneAnalytics(olderThan time.Time) (int64, error) {
	result, err := s.db.Exec(`DELETE FROM click_events WHERE clicked_at < $1`, olderThan.UTC())
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// ─── Export / Import ──────────────────────────────────────────────────────────

func (s *PostgresStore) ExportLinks() ([]*models.Link, error) {
	rows, err := s.db.Query(`
		SELECT id,shortname,target_url,description,is_pattern,created_by,created_at,updated_at,click_count,last_clicked
		FROM links ORDER BY shortname ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []*models.Link
	for rows.Next() {
		link, err := scanLinkPGRows(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, nil
}

func (s *PostgresStore) ImportLinks(links []*models.Link, overwrite bool) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	now := time.Now().UTC()
	count := 0
	for _, link := range links {
		if overwrite {
			_, err = tx.Exec(`
				INSERT INTO links (shortname,target_url,description,is_pattern,created_by,created_at,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7)
				ON CONFLICT(shortname) DO UPDATE SET
				  target_url=EXCLUDED.target_url, description=EXCLUDED.description,
				  is_pattern=EXCLUDED.is_pattern, updated_at=$7`,
				link.Shortname, link.TargetURL, link.Description, link.IsPattern, link.CreatedBy, now, now)
		} else {
			_, err = tx.Exec(`
				INSERT INTO links (shortname,target_url,description,is_pattern,created_by,created_at,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
				link.Shortname, link.TargetURL, link.Description, link.IsPattern, link.CreatedBy, now, now)
		}
		if err != nil {
			return count, fmt.Errorf("import link %q: %w", link.Shortname, err)
		}
		count++
	}
	return count, tx.Commit()
}

// ─── Auth methods (delegate to SQLiteStore pattern) ───────────────────────────
// Postgres auth methods follow the same logic as SQLite but use $N placeholders
// and native time.Time scanning (no string parsing needed).

func (s *PostgresStore) CreateUser(u *models.User) error {
	err := s.db.QueryRow(`
		INSERT INTO users (username,email,password,provider,provider_id,created_at)
		VALUES ($1,$2,$3,$4,$5,NOW()) RETURNING id`,
		nullStringPG(u.Username), nullStringPG(u.Email), nullStringPG(u.Password),
		u.Provider, nullStringPG(u.ProviderID)).Scan(&u.ID)
	if err != nil {
		if isPostgresConflictError(err) {
			return fmt.Errorf("user already exists")
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetUserByUsername(username string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id,COALESCE(username,''),COALESCE(email,''),COALESCE(password,''),
		       provider,COALESCE(provider_id,''),created_at
		FROM users WHERE username=$1`, username)
	return scanUserPG(row)
}

func (s *PostgresStore) GetUserByProvider(provider, providerID string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id,COALESCE(username,''),COALESCE(email,''),COALESCE(password,''),
		       provider,COALESCE(provider_id,''),created_at
		FROM users WHERE provider=$1 AND provider_id=$2`, provider, providerID)
	return scanUserPG(row)
}

func (s *PostgresStore) GetUserByEmail(email string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id,COALESCE(username,''),COALESCE(email,''),COALESCE(password,''),
		       provider,COALESCE(provider_id,''),created_at
		FROM users WHERE email=$1`, email)
	return scanUserPG(row)
}

func (s *PostgresStore) CountUsers() (int, error) {
	var n int
	return n, s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
}

func (s *PostgresStore) CreateSession(sess *models.Session) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (id,user_id,username,created_at,expires_at,last_seen)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		sess.ID, sess.UserID, sess.Username, sess.CreatedAt, sess.ExpiresAt, sess.LastSeen)
	return err
}

func (s *PostgresStore) GetSession(tokenHash string) (*models.Session, error) {
	var sess models.Session
	err := s.db.QueryRow(`
		SELECT id,user_id,username,created_at,expires_at,last_seen
		FROM sessions WHERE id=$1`, tokenHash).
		Scan(&sess.ID, &sess.UserID, &sess.Username, &sess.CreatedAt, &sess.ExpiresAt, &sess.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: "session"}
	}
	return &sess, err
}

func (s *PostgresStore) TouchSession(tokenHash string, lastSeen time.Time) error {
	_, err := s.db.Exec(`UPDATE sessions SET last_seen=$1 WHERE id=$2`, lastSeen, tokenHash)
	return err
}

func (s *PostgresStore) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id=$1`, tokenHash)
	return err
}

func (s *PostgresStore) PruneSessions(now time.Time) (int64, error) {
	result, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func (s *PostgresStore) CreateAPIToken(t *models.APIToken) error {
	return s.db.QueryRow(`
		INSERT INTO api_tokens (user_id,token_hash,label,created_at)
		VALUES ($1,$2,$3,NOW()) RETURNING id`,
		t.UserID, t.TokenHash, t.Label).Scan(&t.ID)
}

func (s *PostgresStore) GetAPIToken(tokenHash string) (*models.APIToken, error) {
	var t models.APIToken
	err := s.db.QueryRow(`
		SELECT id,user_id,token_hash,label,created_at,last_used
		FROM api_tokens WHERE token_hash=$1`, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Label, &t.CreatedAt, &t.LastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: "api_token"}
	}
	if err != nil {
		return nil, err
	}
	_, _ = s.db.Exec(`UPDATE api_tokens SET last_used=NOW() WHERE token_hash=$1`, tokenHash)
	return &t, nil
}

func (s *PostgresStore) DeleteAPIToken(id int64) error {
	result, err := s.db.Exec(`DELETE FROM api_tokens WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return &ErrNotFound{Shortname: "api_token"}
	}
	return nil
}

func (s *PostgresStore) ListAPITokens(userID int64) ([]*models.APIToken, error) {
	rows, err := s.db.Query(`
		SELECT id,user_id,token_hash,label,created_at,last_used
		FROM api_tokens WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []*models.APIToken
	for rows.Next() {
		var t models.APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Label, &t.CreatedAt, &t.LastUsed); err != nil {
			return nil, err
		}
		tokens = append(tokens, &t)
	}
	return tokens, nil
}

func (s *PostgresStore) CreateOAuthState(token string, expiresAt time.Time) error {
	_, err := s.db.Exec(`INSERT INTO oauth_state (token,expires_at) VALUES ($1,$2)`, token, expiresAt)
	return err
}

func (s *PostgresStore) ConsumeOAuthState(token string, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var expiresAt time.Time
	if err := tx.QueryRow(`SELECT expires_at FROM oauth_state WHERE token=$1`, token).Scan(&expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ErrNotFound{Shortname: "oauth_state"}
		}
		return err
	}
	if now.After(expiresAt) {
		return fmt.Errorf("oauth state expired")
	}
	if _, err := tx.Exec(`DELETE FROM oauth_state WHERE token=$1`, token); err != nil {
		return err
	}
	return tx.Commit()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type pgScanner interface{ Scan(...any) error }

func scanLinkPG(s *sql.Row) (*models.Link, error) {
	var l models.Link
	err := s.Scan(&l.ID, &l.Shortname, &l.TargetURL, &l.Description, &l.IsPattern,
		&l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.ClickCount, &l.LastClicked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	return &l, err
}

func scanLinkPGRows(rows *sql.Rows) (*models.Link, error) {
	var l models.Link
	err := rows.Scan(&l.ID, &l.Shortname, &l.TargetURL, &l.Description, &l.IsPattern,
		&l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.ClickCount, &l.LastClicked)
	return &l, err
}

func scanUserPG(row *sql.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Password,
		&u.Provider, &u.ProviderID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: "user"}
	}
	return &u, err
}

func isPostgresConflictError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

// nullStringPG converts an empty Go string to nil for nullable Postgres columns.
func nullStringPG(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
