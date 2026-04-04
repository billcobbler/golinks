// Package web serves the golinks web dashboard at /-/.
package web

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/billcobbler/golinks/internal/auth"
	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
)

// ─── Page data types ──────────────────────────────────────────────────────────

// BasePage holds common fields shared by all full-page templates.
type BasePage struct {
	Page     string // "links", "analytics", "settings" — controls nav highlighting
	Username string // logged-in username; empty when auth=none
}

// LinksPage is the data model for the links management page.
type LinksPage struct {
	BasePage
	Links    []*models.Link
	Total    int
	PageSize int
	Search   string
	Message  string // flash message (success or error)
}

// AnalyticsPage is the data model for the analytics page.
type AnalyticsPage struct {
	BasePage
	Stats *models.Stats
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// Handlers serves the web dashboard at /-/.
type Handlers struct {
	store         store.Store
	log           *slog.Logger
	tmplLinks     *template.Template // base + links.html + partials
	tmplAnalytics *template.Template // base + analytics.html + partials
}

var funcMap = template.FuncMap{
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n-3] + "…"
	},
}

// NewHandlers creates dashboard handlers and pre-parses all templates.
// Panics if any template fails to parse — catches config errors at startup.
func NewHandlers(s store.Store, log *slog.Logger) *Handlers {
	subFS, err := fs.Sub(FS, "templates")
	if err != nil {
		panic("web: sub templates FS: " + err.Error())
	}
	parse := func(pages ...string) *template.Template {
		patterns := append([]string{"base.html", "partials/*.html"}, pages...)
		t := template.New("").Funcs(funcMap)
		return template.Must(t.ParseFS(subFS, patterns...))
	}
	return &Handlers{
		store:         s,
		log:           log,
		tmplLinks:     parse("links.html"),
		tmplAnalytics: parse("analytics.html"),
	}
}

// ─── Full-page handlers ────────────────────────────────────────────────────────

// Index redirects / to /-/links.
func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/-/links", http.StatusFound)
}

// Links renders the link management page.
func (h *Handlers) Links(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("q")
	result, err := h.store.ListLinks(search, 0, 200)
	if err != nil {
		h.serverError(w, "listing links", err)
		return
	}
	h.renderPage(w, h.tmplLinks, LinksPage{
		BasePage: BasePage{Page: "links", Username: usernameFrom(r)},
		Links:    result.Links,
		Total:    result.Total,
		PageSize: len(result.Links),
		Search:   search,
		Message:  q.Get("msg"),
	})
}

// Analytics renders the analytics page.
func (h *Handlers) Analytics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetStats()
	if err != nil {
		h.serverError(w, "fetching stats", err)
		return
	}
	h.renderPage(w, h.tmplAnalytics, AnalyticsPage{
		BasePage: BasePage{Page: "analytics", Username: usernameFrom(r)},
		Stats:    stats,
	})
}

// usernameFrom extracts the logged-in username from the request context.
// Returns "" when auth is disabled or the request is unauthenticated.
func usernameFrom(r *http.Request) string {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return ""
	}
	return u.Username
}

// ─── htmx partial handlers ────────────────────────────────────────────────────

// PartialsLinks returns the links table rows fragment for htmx live search.
func (h *Handlers) PartialsLinks(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	result, err := h.store.ListLinks(search, 0, 200)
	if err != nil {
		http.Error(w, "error listing links", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, h.tmplLinks, "links_rows", LinksPage{
		Links: result.Links,
		Total: result.Total,
	})
}

// LinkEditRow returns an editable table row for a single link.
func (h *Handlers) LinkEditRow(w http.ResponseWriter, r *http.Request) {
	link, ok := h.mustGetLink(w, r)
	if !ok {
		return
	}
	h.renderPartial(w, h.tmplLinks, "link_edit_row", link)
}

// LinkRow returns the read-only table row for a single link (used by cancel edit).
func (h *Handlers) LinkRow(w http.ResponseWriter, r *http.Request) {
	link, ok := h.mustGetLink(w, r)
	if !ok {
		return
	}
	h.renderPartial(w, h.tmplLinks, "link_row", link)
}

// ─── Mutation handlers ─────────────────────────────────────────────────────────

// CreateLink handles POST /-/links (regular form submit, redirects back).
func (h *Handlers) CreateLink(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectMsg(w, r, "Invalid form data")
		return
	}
	shortname := normalizeShortname(r.FormValue("shortname"))
	targetURL := strings.TrimSpace(r.FormValue("target_url"))
	if shortname == "" || targetURL == "" {
		h.redirectMsg(w, r, "Shortname and URL are required")
		return
	}
	link := &models.Link{
		Shortname:   shortname,
		TargetURL:   targetURL,
		Description: strings.TrimSpace(r.FormValue("description")),
		IsPattern:   r.FormValue("is_pattern") == "true",
	}
	if err := h.store.CreateLink(link); err != nil {
		msg := "Failed to create link"
		if store.IsConflict(err) {
			msg = fmt.Sprintf("go/%s already exists", shortname)
		}
		h.redirectMsg(w, r, msg)
		return
	}
	h.redirectMsg(w, r, fmt.Sprintf("Created go/%s", link.Shortname))
}

// UpdateLink handles PUT /-/links/update?name=<shortname> (htmx, returns updated row).
func (h *Handlers) UpdateLink(w http.ResponseWriter, r *http.Request) {
	shortname := r.URL.Query().Get("name")
	if shortname == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	link := &models.Link{
		Shortname:   shortname,
		TargetURL:   strings.TrimSpace(r.FormValue("target_url")),
		Description: strings.TrimSpace(r.FormValue("description")),
		IsPattern:   r.FormValue("is_pattern") == "true",
	}
	if err := h.store.UpdateLink(link); err != nil {
		if store.IsNotFound(err) {
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	updated, err := h.store.GetLink(shortname)
	if err != nil {
		updated = link
	}
	h.renderPartial(w, h.tmplLinks, "link_row", updated)
}

// DeleteLink handles DELETE /-/links/delete?name=<shortname> (htmx, returns empty to remove row).
func (h *Handlers) DeleteLink(w http.ResponseWriter, r *http.Request) {
	shortname := r.URL.Query().Get("name")
	if shortname == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteLink(shortname); err != nil {
		if store.IsNotFound(err) {
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	// Empty body — htmx outerHTML swap removes the row.
	w.WriteHeader(http.StatusOK)
}

// Import handles POST /-/import (multipart file upload, redirects back with message).
func (h *Handlers) Import(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.redirectMsg(w, r, "Failed to parse upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		h.redirectMsg(w, r, "No file provided")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		h.redirectMsg(w, r, "Failed to read file")
		return
	}

	overwrite := r.FormValue("overwrite") == "true"
	var links []*models.Link

	name := strings.ToLower(header.Filename)
	if strings.HasSuffix(name, ".csv") {
		cr := csv.NewReader(strings.NewReader(string(data)))
		records, err := cr.ReadAll()
		if err != nil {
			h.redirectMsg(w, r, "Invalid CSV: "+err.Error())
			return
		}
		if len(records) > 0 && records[0][0] == "shortname" {
			records = records[1:]
		}
		for _, row := range records {
			if len(row) < 2 {
				continue
			}
			lnk := &models.Link{
				Shortname: normalizeShortname(row[0]),
				TargetURL: strings.TrimSpace(row[1]),
			}
			if len(row) >= 3 {
				lnk.Description = strings.TrimSpace(row[2])
			}
			if len(row) >= 4 {
				lnk.IsPattern = strings.EqualFold(row[3], "true")
			}
			links = append(links, lnk)
		}
	} else {
		if err := json.Unmarshal(data, &links); err != nil {
			h.redirectMsg(w, r, "Invalid JSON: "+err.Error())
			return
		}
	}

	if len(links) == 0 {
		h.redirectMsg(w, r, "No links found in file")
		return
	}

	imported, err := h.store.ImportLinks(links, overwrite)
	if err != nil {
		h.redirectMsg(w, r, "Import failed: "+err.Error())
		return
	}
	action := "created"
	if overwrite {
		action = "upserted"
	}
	skipped := len(links) - imported
	msg := fmt.Sprintf("%d links %s", imported, action)
	if skipped > 0 {
		msg += fmt.Sprintf(", %d skipped", skipped)
	}
	h.redirectMsg(w, r, msg)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (h *Handlers) renderPage(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		h.log.Error("template render error", "err", err)
	}
}

func (h *Handlers) renderPartial(w http.ResponseWriter, t *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		h.log.Error("partial render error", "name", name, "err", err)
	}
}

func (h *Handlers) serverError(w http.ResponseWriter, op string, err error) {
	h.log.Error("dashboard error", "op", op, "err", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func (h *Handlers) redirectMsg(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/-/links?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (h *Handlers) mustGetLink(w http.ResponseWriter, r *http.Request) (*models.Link, bool) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return nil, false
	}
	link, err := h.store.GetLink(name)
	if err != nil {
		if store.IsNotFound(err) {
			http.Error(w, "link not found", http.StatusNotFound)
		} else {
			http.Error(w, "server error", http.StatusInternalServerError)
		}
		return nil, false
	}
	return link, true
}

func normalizeShortname(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.Trim(s, "/")
	return s
}
