// Package auth implements authentication middleware and session management.
//
// Supported modes (controlled by config.AuthMode):
//   - "none"       — all routes are public (default)
//   - "local"      — username/password login with bcrypt + secure cookies
//   - "oauth"      — Google or GitHub OAuth2
//   - "local+oauth"— both local and OAuth
package auth

import (
	"net/http"
	"time"

	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
)

const (
	SessionCookieName = "golinks_session"
	SessionDuration   = 30 * 24 * time.Hour // 30 days
	// Sessions are "touched" (last_seen updated) at most this often to
	// avoid a DB write on every single request.
	TouchHysteresis = time.Hour
)

// Manager holds auth state needed by middleware and handlers.
type Manager struct {
	store           store.Store
	insecureCookies bool // set true for non-TLS environments
}

// NewManager creates an auth Manager.
// insecureCookies should be true when the server is not behind TLS,
// otherwise the Secure cookie flag will prevent the session cookie from
// being sent over plain HTTP.
func NewManager(s store.Store, insecureCookies bool) *Manager {
	return &Manager{store: s, insecureCookies: insecureCookies}
}

// ─── Middleware ───────────────────────────────────────────────────────────────

// RequireSession is HTTP middleware that enforces a valid session cookie.
// Unauthenticated requests are redirected to /-/auth/login.
//
// htmx-aware: when the request carries "HX-Request: true", the response
// uses the "HX-Redirect" header instead of a 302 redirect, so htmx
// performs a full-page navigation instead of swapping partial HTML.
func (m *Manager) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := m.sessionFromRequest(r)
		if !ok {
			m.redirectToLogin(w, r)
			return
		}

		// Touch last_seen at most once per hour to limit write traffic.
		if time.Since(sess.LastSeen) > TouchHysteresis {
			_ = m.store.TouchSession(sess.ID, time.Now().UTC())
		}

		ctx := SetUser(r.Context(), UserContext{ID: sess.UserID, Username: sess.Username})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAPIToken is HTTP middleware that enforces a valid Bearer token.
// Unauthenticated requests receive 401 JSON.
func (m *Manager) RequireAPIToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			writeAuthError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}
		hash := HashToken(raw)
		tok, err := m.store.GetAPIToken(hash)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid or revoked token")
			return
		}
		ctx := SetUser(r.Context(), UserContext{ID: tok.UserID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (m *Manager) sessionFromRequest(r *http.Request) (*models.Session, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, false
	}
	hash := HashToken(cookie.Value)
	sess, err := m.store.GetSession(hash)
	if err != nil {
		return nil, false
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = m.store.DeleteSession(hash)
		return nil, false
	}
	return sess, true
}

func (m *Manager) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/-/auth/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/-/auth/login", http.StatusSeeOther)
}

// SetSessionCookie writes the session cookie to the response.
func (m *Manager) SetSessionCookie(w http.ResponseWriter, rawToken string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !m.insecureCookies,
	})
}

// ClearSessionCookie removes the session cookie.
func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !m.insecureCookies,
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
