package models

import "time"

// User represents an authenticated user.
type User struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`    // empty for OAuth-only users
	Email      string    `json:"email"`       // empty for local-only users
	Password   string    `json:"-"`           // bcrypt hash; never serialised
	Provider   string    `json:"provider"`    // "local" | "google" | "github"
	ProviderID string    `json:"provider_id"` // OAuth subject; empty for local
	CreatedAt  time.Time `json:"created_at"`
}

// Session represents an authenticated browser session.
type Session struct {
	ID        string    `json:"id"` // SHA-256(raw_token), hex
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"` // denormalised for fast middleware lookup
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// APIToken represents a long-lived token for CLI / extension access.
type APIToken struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	TokenHash string     `json:"-"` // SHA-256(raw); never serialised
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}
