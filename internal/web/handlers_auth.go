package web

// Auth HTTP handlers: login, logout, first-run setup, OAuth flow, API token management.
// These live in the web package so they share templates and the common render helpers.

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/billcobbler/golinks/internal/auth"
	"github.com/billcobbler/golinks/internal/config"
	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
)

// ─── AuthHandlers ─────────────────────────────────────────────────────────────

// AuthHandlers serves the authentication UI at /-/auth/*.
type AuthHandlers struct {
	store        store.Store
	cfg          *config.Config
	manager      *auth.Manager
	oauth        *auth.OAuthProvider // nil when OAuth is not configured
	log          *slog.Logger
	tmplLogin    *template.Template
	tmplSetup    *template.Template
	tmplSettings *template.Template
}

// LoginPage is the data model for the login template.
type LoginPage struct {
	BasePage
	Error       string
	HasLocal    bool // show username/password form
	HasOAuth    bool // show OAuth button
	OAuthLabel  string
	SetupNeeded bool
}

// SetupPage is the data model for the first-run setup template.
type SetupPage struct {
	BasePage
	Error string
}

// SettingsPage is the data model for the settings/tokens template.
type SettingsPage struct {
	BasePage
	Tokens   []*models.APIToken
	NewToken string // raw token shown once after creation
	Message  string
}

// NewAuthHandlers creates AuthHandlers and pre-parses all auth templates.
func NewAuthHandlers(s store.Store, cfg *config.Config, manager *auth.Manager, log *slog.Logger) *AuthHandlers {
	subFS, err := fs.Sub(FS, "templates")
	if err != nil {
		panic("web: sub templates FS: " + err.Error())
	}
	parse := func(pages ...string) *template.Template {
		patterns := append([]string{"base.html"}, pages...)
		t := template.New("").Funcs(funcMap)
		return template.Must(t.ParseFS(subFS, patterns...))
	}

	ah := &AuthHandlers{
		store:        s,
		cfg:          cfg,
		manager:      manager,
		log:          log,
		tmplLogin:    parse("login.html"),
		tmplSetup:    parse("setup.html"),
		tmplSettings: parse("settings.html"),
	}

	// Initialise OAuth provider if configured.
	if cfg.AuthMode == "oauth" || cfg.AuthMode == "local+oauth" {
		p, err := auth.NewOAuthProvider(cfg.OAuthProvider, cfg.OAuthClientID, cfg.OAuthClientSecret, cfg.BaseURL)
		if err != nil {
			panic("auth: " + err.Error())
		}
		ah.oauth = p
	}

	return ah
}

// ─── Login / Logout ───────────────────────────────────────────────────────────

func (ah *AuthHandlers) ShowLogin(w http.ResponseWriter, r *http.Request) {
	if ah.cfg.AuthMode == "none" {
		http.Redirect(w, r, "/-/", http.StatusFound)
		return
	}
	count, _ := ah.store.CountUsers()
	ah.renderAuth(w, ah.tmplLogin, LoginPage{
		BasePage:    BasePage{Page: ""},
		Error:       r.URL.Query().Get("error"),
		HasLocal:    ah.cfg.AuthMode == "local" || ah.cfg.AuthMode == "local+oauth",
		HasOAuth:    ah.oauth != nil,
		OAuthLabel:  oauthLabel(ah.cfg.OAuthProvider),
		SetupNeeded: count == 0 && (ah.cfg.AuthMode == "local" || ah.cfg.AuthMode == "local+oauth"),
	})
}

func (ah *AuthHandlers) HandleLocalLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	u, err := ah.store.GetUserByUsername(username)
	if err != nil || u.Provider != "local" {
		http.Redirect(w, r, "/-/auth/login?error=invalid+credentials", http.StatusSeeOther)
		return
	}

	if !auth.CheckPassword(password, u.Password) {
		http.Redirect(w, r, "/-/auth/login?error=invalid+credentials", http.StatusSeeOther)
		return
	}

	if err := ah.createSession(w, u); err != nil {
		ah.log.Error("create session", "err", err)
		http.Redirect(w, r, "/-/auth/login?error=internal+error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/-/links", http.StatusSeeOther)
}

func (ah *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		hash := auth.HashToken(cookie.Value)
		_ = ah.store.DeleteSession(hash)
	}
	ah.manager.ClearSessionCookie(w)
	http.Redirect(w, r, "/-/auth/login", http.StatusSeeOther)
}

// ─── First-run setup ──────────────────────────────────────────────────────────

func (ah *AuthHandlers) ShowSetup(w http.ResponseWriter, r *http.Request) {
	if ah.cfg.AuthMode == "none" {
		http.NotFound(w, r)
		return
	}
	count, err := ah.store.CountUsers()
	if err != nil || count > 0 {
		http.NotFound(w, r)
		return
	}
	ah.renderAuth(w, ah.tmplSetup, SetupPage{Error: r.URL.Query().Get("error")})
}

func (ah *AuthHandlers) HandleSetup(w http.ResponseWriter, r *http.Request) {
	if ah.cfg.AuthMode == "none" {
		http.NotFound(w, r)
		return
	}
	count, _ := ah.store.CountUsers()
	if count > 0 {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	if username == "" || password == "" {
		http.Redirect(w, r, "/-/auth/setup?error=username+and+password+required", http.StatusSeeOther)
		return
	}
	if password != confirm {
		http.Redirect(w, r, "/-/auth/setup?error=passwords+do+not+match", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		ah.log.Error("hash password", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	u := &models.User{
		Username: username,
		Password: hash,
		Provider: "local",
	}
	if err := ah.store.CreateUser(u); err != nil {
		http.Redirect(w, r, "/-/auth/setup?error="+err.Error(), http.StatusSeeOther)
		return
	}

	if err := ah.createSession(w, u); err != nil {
		ah.log.Error("create session after setup", "err", err)
	}
	http.Redirect(w, r, "/-/links", http.StatusSeeOther)
}

// ─── OAuth ────────────────────────────────────────────────────────────────────

func (ah *AuthHandlers) HandleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if ah.oauth == nil {
		http.NotFound(w, r)
		return
	}
	_, stateHash, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Use the hash as the state nonce — it's random and unexploitable.
	if err := ah.store.CreateOAuthState(stateHash, time.Now().Add(10*time.Minute)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, ah.oauth.AuthCodeURL(stateHash), http.StatusFound)
}

func (ah *AuthHandlers) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if ah.oauth == nil {
		http.NotFound(w, r)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if err := ah.store.ConsumeOAuthState(state, time.Now()); err != nil {
		http.Redirect(w, r, "/-/auth/login?error=invalid+oauth+state", http.StatusSeeOther)
		return
	}

	accessToken, err := ah.oauth.Exchange(r.Context(), code)
	if err != nil {
		ah.log.Error("oauth exchange", "err", err)
		http.Redirect(w, r, "/-/auth/login?error=oauth+exchange+failed", http.StatusSeeOther)
		return
	}

	info, err := ah.oauth.FetchUserInfo(r.Context(), accessToken)
	if err != nil {
		ah.log.Error("fetch userinfo", "err", err)
		http.Redirect(w, r, "/-/auth/login?error=could+not+fetch+user+info", http.StatusSeeOther)
		return
	}

	// Upsert: find existing user by (provider, providerID) or email, else create.
	u, err := ah.store.GetUserByProvider(ah.cfg.OAuthProvider, info.ProviderID)
	if store.IsNotFound(err) {
		u = &models.User{
			Username:   info.Login,
			Email:      info.Email,
			Provider:   ah.cfg.OAuthProvider,
			ProviderID: info.ProviderID,
		}
		if createErr := ah.store.CreateUser(u); createErr != nil {
			ah.log.Error("create oauth user", "err", createErr)
			http.Redirect(w, r, "/-/auth/login?error=could+not+create+user", http.StatusSeeOther)
			return
		}
	} else if err != nil {
		ah.log.Error("lookup oauth user", "err", err)
		http.Redirect(w, r, "/-/auth/login?error=internal+error", http.StatusSeeOther)
		return
	}

	if err := ah.createSession(w, u); err != nil {
		ah.log.Error("create oauth session", "err", err)
		http.Redirect(w, r, "/-/auth/login?error=internal+error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/-/links", http.StatusSeeOther)
}

// ─── API Token Management ─────────────────────────────────────────────────────

func (ah *AuthHandlers) ShowSettings(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	tokens, err := ah.store.ListAPITokens(u.ID)
	if err != nil {
		ah.log.Error("list tokens", "err", err)
		tokens = nil
	}
	ah.renderAuth(w, ah.tmplSettings, SettingsPage{
		BasePage: BasePage{Page: "settings", Username: u.Username},
		Tokens:   tokens,
		NewToken: r.URL.Query().Get("new_token"),
		Message:  r.URL.Query().Get("msg"),
	})
}

func (ah *AuthHandlers) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, _ := auth.UserFromContext(r.Context())
	raw, hash, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	t := &models.APIToken{
		UserID:    u.ID,
		TokenHash: hash,
		Label:     r.FormValue("label"),
	}
	if err := ah.store.CreateAPIToken(t); err != nil {
		ah.log.Error("create api token", "err", err)
		http.Redirect(w, r, "/-/settings?msg=failed+to+create+token", http.StatusSeeOther)
		return
	}
	// Raw token is shown exactly once; encode it in the redirect URL.
	http.Redirect(w, r, "/-/settings?new_token="+raw, http.StatusSeeOther)
}

func (ah *AuthHandlers) HandleDeleteToken(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := ah.store.DeleteAPIToken(id); err != nil {
		ah.log.Error("delete api token", "err", err, "id", id)
	}
	http.Redirect(w, r, "/-/settings?msg=token+deleted", http.StatusSeeOther)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (ah *AuthHandlers) createSession(w http.ResponseWriter, u *models.User) error {
	raw, hash, err := auth.GenerateToken()
	if err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}
	now := time.Now().UTC()
	sess := &models.Session{
		ID:        hash,
		UserID:    u.ID,
		Username:  u.Username,
		CreatedAt: now,
		ExpiresAt: now.Add(auth.SessionDuration),
		LastSeen:  now,
	}
	if err := ah.store.CreateSession(sess); err != nil {
		return fmt.Errorf("store session: %w", err)
	}
	ah.manager.SetSessionCookie(w, raw, sess.ExpiresAt)
	return nil
}

func (ah *AuthHandlers) renderAuth(w http.ResponseWriter, tmpl *template.Template, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		ah.log.Error("render auth template", "err", err)
	}
}

func oauthLabel(provider string) string {
	switch provider {
	case "google":
		return "Sign in with Google"
	case "github":
		return "Sign in with GitHub"
	default:
		return "Sign in with OAuth"
	}
}
