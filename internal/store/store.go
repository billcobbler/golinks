package store

import (
	"errors"
	"fmt"
	"time"

	"github.com/billcobbler/golinks/internal/models"
)

// Store defines the persistence interface for golinks.
type Store interface {
	// Link operations
	GetLink(shortname string) (*models.Link, error)
	GetLinkByPath(path string) (*models.Link, string, error) // returns link + remaining path suffix
	ListLinks(search string, offset, limit int) (*models.ListResult, error)
	CreateLink(link *models.Link) error
	UpdateLink(link *models.Link) error
	DeleteLink(shortname string) error

	// Analytics
	RecordClick(linkID int64, referrer, userAgent string) error
	GetStats() (*models.Stats, error)
	PruneAnalytics(olderThan time.Time) (int64, error)

	// Export / Import
	ExportLinks() ([]*models.Link, error)
	ImportLinks(links []*models.Link, overwrite bool) (int, error)

	// ── Auth ──────────────────────────────────────────────────────────────────

	// User management
	CreateUser(u *models.User) error
	GetUserByUsername(username string) (*models.User, error)
	GetUserByProvider(provider, providerID string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	CountUsers() (int, error)

	// Session management
	CreateSession(s *models.Session) error
	GetSession(tokenHash string) (*models.Session, error)
	TouchSession(tokenHash string, lastSeen time.Time) error
	DeleteSession(tokenHash string) error
	PruneSessions(now time.Time) (int64, error)

	// API token management
	CreateAPIToken(t *models.APIToken) error
	GetAPIToken(tokenHash string) (*models.APIToken, error)
	DeleteAPIToken(id int64) error
	ListAPITokens(userID int64) ([]*models.APIToken, error)

	// OAuth state (CSRF nonce)
	CreateOAuthState(token string, expiresAt time.Time) error
	ConsumeOAuthState(token string, now time.Time) error

	Close() error
}

// ErrNotFound is returned when a link cannot be found.
type ErrNotFound struct {
	Shortname string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("link %q not found", e.Shortname)
}

// IsNotFound returns true if the error is an ErrNotFound.
func IsNotFound(err error) bool {
	var e *ErrNotFound
	return errors.As(err, &e)
}

// ErrConflict is returned when a link already exists with the same shortname.
type ErrConflict struct {
	Shortname string
}

func (e *ErrConflict) Error() string {
	return fmt.Sprintf("link %q already exists", e.Shortname)
}

// IsConflict returns true if the error is an ErrConflict.
func IsConflict(err error) bool {
	var e *ErrConflict
	return errors.As(err, &e)
}
