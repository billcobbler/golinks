// Package api wires up the HTTP router for the golinks REST API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/billcobbler/golinks/internal/auth"
	"github.com/billcobbler/golinks/internal/config"
	"github.com/billcobbler/golinks/internal/redirect"
	"github.com/billcobbler/golinks/internal/store"
	"github.com/billcobbler/golinks/internal/web"
)

// NewRouter builds and returns the root chi router.
// All server-owned routes live under /-/ to keep the entire URL space free for
// user-defined shortnames. A shortname starting with '-' is rejected by
// validation, so /-/ can never be shadowed.
func NewRouter(s store.Store, cfg *config.Config, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(Recoverer(log))
	r.Use(Logger(log))
	r.Use(CORS)
	r.Use(middleware.StripSlashes)

	h := NewHandlers(s)
	rh := redirect.NewHandler(s)
	dh := web.NewHandlers(s, log)

	// Auth manager — used for middleware and by auth handlers.
	am := auth.NewManager(s, cfg.InsecureCookies)
	ah := web.NewAuthHandlers(s, cfg, am, log)

	requireAuth := cfg.AuthMode != "none"

	r.Route("/-", func(r chi.Router) {

		// ── Auth routes ─────────────────────────────────────────────────────
		// Always registered regardless of auth mode; handlers return 404 or
		// redirect when auth is disabled.
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", ah.ShowLogin)
			r.Post("/login", ah.HandleLocalLogin)
			r.Get("/login/oauth", ah.HandleOAuthStart)
			r.Get("/callback", ah.HandleOAuthCallback)
			r.Post("/logout", ah.HandleLogout)
			r.Get("/setup", ah.ShowSetup)
			r.Post("/setup", ah.HandleSetup)
		})

		// ── REST API ─────────────────────────────────────────────────────────
		// Health check is always public (Docker/k8s probes must not need a token).
		r.Get("/api/health", h.Health)

		r.Route("/api", func(r chi.Router) {
			if requireAuth {
				r.Use(am.RequireAPIToken)
			}
			r.Get("/stats", h.GetStats)
			r.Get("/export", h.ExportLinks)
			r.Post("/import", h.ImportLinks)
			r.Route("/links", func(r chi.Router) {
				r.Get("/", h.ListLinks)
				r.Post("/", h.CreateLink)
				r.Get("/*", h.GetLink)
				r.Put("/*", h.UpdateLink)
				r.Delete("/*", h.DeleteLink)
			})
		})

		// ── Web dashboard ────────────────────────────────────────────────────
		r.Group(func(r chi.Router) {
			if requireAuth {
				r.Use(am.RequireSession)
			}

			r.Get("/", dh.Index)
			r.Get("/links", dh.Links)
			r.Get("/analytics", dh.Analytics)

			// htmx partials
			r.Get("/partials/links", dh.PartialsLinks)
			r.Get("/links/row", dh.LinkRow)
			r.Get("/links/edit", dh.LinkEditRow)
			r.Put("/links/update", dh.UpdateLink)
			r.Delete("/links/delete", dh.DeleteLink)

			// Mutations (regular form POST → redirect)
			r.Post("/links", dh.CreateLink)
			r.Post("/import", dh.Import)

			// Settings / API token management
			r.Get("/settings", ah.ShowSettings)
			r.Post("/settings/tokens", ah.HandleCreateToken)
			r.Post("/settings/tokens/{id}/delete", ah.HandleDeleteToken)
		})

		// Static assets (pico.min.css, htmx.min.js, app.css)
		staticSrv := http.FileServerFS(web.StaticFS)
		r.Handle("/static/*", http.StripPrefix("/-/static/", staticSrv))
	})

	// ── Redirect handler ──────────────────────────────────────────────────────
	// Must come last — catches everything not matched above.
	r.Handle("/*", rh)

	return r
}
