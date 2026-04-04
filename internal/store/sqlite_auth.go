package store

// Auth-related Store methods for SQLiteStore.
// Implements: users, sessions, api_tokens, oauth_state.

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/billcobbler/golinks/internal/models"
)

// ─── Users ────────────────────────────────────────────────────────────────────

func (s *SQLiteStore) CreateUser(u *models.User) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		INSERT INTO users (username, email, password, provider, provider_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		nullString(u.Username), nullString(u.Email), nullString(u.Password),
		u.Provider, nullString(u.ProviderID), now)
	if err != nil {
		if isSQLiteConstraintError(err) {
			return fmt.Errorf("user already exists")
		}
		return fmt.Errorf("create user: %w", err)
	}
	id, _ := result.LastInsertId()
	u.ID = id
	return nil
}

func (s *SQLiteStore) GetUserByUsername(username string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, COALESCE(username,''), COALESCE(email,''), COALESCE(password,''),
		       provider, COALESCE(provider_id,''), created_at
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByProvider(provider, providerID string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, COALESCE(username,''), COALESCE(email,''), COALESCE(password,''),
		       provider, COALESCE(provider_id,''), created_at
		FROM users WHERE provider = ? AND provider_id = ?`, provider, providerID)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByEmail(email string) (*models.User, error) {
	row := s.db.QueryRow(`
		SELECT id, COALESCE(username,''), COALESCE(email,''), COALESCE(password,''),
		       provider, COALESCE(provider_id,''), created_at
		FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func (s *SQLiteStore) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var createdAtStr string
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Password,
		&u.Provider, &u.ProviderID, &createdAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: "user"}
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &u, nil
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

func (s *SQLiteStore) CreateSession(sess *models.Session) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, user_id, username, created_at, expires_at, last_seen)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.Username,
		sess.CreatedAt.UTC().Format(time.RFC3339),
		sess.ExpiresAt.UTC().Format(time.RFC3339),
		sess.LastSeen.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSession(tokenHash string) (*models.Session, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, username, created_at, expires_at, last_seen
		FROM sessions WHERE id = ?`, tokenHash)

	var sess models.Session
	var createdStr, expiresStr, lastSeenStr string
	err := row.Scan(&sess.ID, &sess.UserID, &sess.Username,
		&createdStr, &expiresStr, &lastSeenStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: "session"}
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	sess.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
	sess.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
	return &sess, nil
}

func (s *SQLiteStore) TouchSession(tokenHash string, lastSeen time.Time) error {
	_, err := s.db.Exec(`UPDATE sessions SET last_seen = ? WHERE id = ?`,
		lastSeen.UTC().Format(time.RFC3339), tokenHash)
	return err
}

func (s *SQLiteStore) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, tokenHash)
	return err
}

func (s *SQLiteStore) PruneSessions(now time.Time) (int64, error) {
	result, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`,
		now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("prune sessions: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// ─── API Tokens ───────────────────────────────────────────────────────────────

func (s *SQLiteStore) CreateAPIToken(t *models.APIToken) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		INSERT INTO api_tokens (user_id, token_hash, label, created_at)
		VALUES (?, ?, ?, ?)`,
		t.UserID, t.TokenHash, t.Label, now)
	if err != nil {
		return fmt.Errorf("create api token: %w", err)
	}
	id, _ := result.LastInsertId()
	t.ID = id
	return nil
}

func (s *SQLiteStore) GetAPIToken(tokenHash string) (*models.APIToken, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, token_hash, label, created_at, last_used
		FROM api_tokens WHERE token_hash = ?`, tokenHash)

	var t models.APIToken
	var createdStr string
	var lastUsedStr *string
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Label, &createdStr, &lastUsedStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ErrNotFound{Shortname: "api_token"}
	}
	if err != nil {
		return nil, fmt.Errorf("get api token: %w", err)
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	if lastUsedStr != nil {
		ts, _ := time.Parse(time.RFC3339, *lastUsedStr)
		t.LastUsed = &ts
	}
	// Update last_used asynchronously — ignore errors for this non-critical write.
	_, _ = s.db.Exec(`UPDATE api_tokens SET last_used = ? WHERE token_hash = ?`,
		time.Now().UTC().Format(time.RFC3339), tokenHash)
	return &t, nil
}

func (s *SQLiteStore) DeleteAPIToken(id int64) error {
	result, err := s.db.Exec(`DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api token: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return &ErrNotFound{Shortname: "api_token"}
	}
	return nil
}

func (s *SQLiteStore) ListAPITokens(userID int64) ([]*models.APIToken, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, token_hash, label, created_at, last_used
		FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*models.APIToken
	for rows.Next() {
		var t models.APIToken
		var createdStr string
		var lastUsedStr *string
		if err := rows.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Label, &createdStr, &lastUsedStr); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		if lastUsedStr != nil {
			ts, _ := time.Parse(time.RFC3339, *lastUsedStr)
			t.LastUsed = &ts
		}
		tokens = append(tokens, &t)
	}
	return tokens, nil
}

// ─── OAuth State ──────────────────────────────────────────────────────────────

func (s *SQLiteStore) CreateOAuthState(token string, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO oauth_state (token, expires_at) VALUES (?, ?)`,
		token, expiresAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("create oauth state: %w", err)
	}
	return nil
}

// ConsumeOAuthState validates and deletes the nonce in a single transaction.
// Returns ErrNotFound if the token doesn't exist or is expired.
func (s *SQLiteStore) ConsumeOAuthState(token string, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin oauth state tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var expiresStr string
	err = tx.QueryRow(`SELECT expires_at FROM oauth_state WHERE token = ?`, token).
		Scan(&expiresStr)
	if errors.Is(err, sql.ErrNoRows) {
		return &ErrNotFound{Shortname: "oauth_state"}
	}
	if err != nil {
		return fmt.Errorf("get oauth state: %w", err)
	}

	expiresAt, _ := time.Parse(time.RFC3339, expiresStr)
	if now.After(expiresAt) {
		return fmt.Errorf("oauth state expired")
	}

	if _, err := tx.Exec(`DELETE FROM oauth_state WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete oauth state: %w", err)
	}
	return tx.Commit()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// nullString returns nil for an empty string, used for nullable TEXT columns.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
