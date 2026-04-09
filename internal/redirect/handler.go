// Package redirect handles the core golink resolution and redirect logic.
package redirect

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/billcobbler/golinks/internal/store"
)

// Handler resolves incoming paths to target URLs and redirects the client.
type Handler struct {
	store store.Store
}

// NewHandler creates a new redirect Handler backed by the given store.
func NewHandler(s store.Store) *Handler {
	return &Handler{store: s}
}

// ServeHTTP resolves the request path and issues an HTTP 302 redirect.
// On a miss it returns 404; on a server error it returns 500.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		http.Redirect(w, r, "/-/", http.StatusFound)
		return
	}

	link, remaining, err := h.store.GetLinkByPath(path)
	if err != nil {
		if store.IsNotFound(err) {
			msg := url.QueryEscape("/" + path + " doesn't exist")
			http.Redirect(w, r, "/-/links?msg="+msg, http.StatusFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	target := link.TargetURL
	if link.IsPattern && remaining != "" {
		target = substitutePattern(link.TargetURL, remaining)
	}

	// Fire-and-forget click recording (non-blocking).
	go func() {
		referrer := r.Referer()
		ua := r.UserAgent()
		_ = h.store.RecordClick(link.ID, referrer, ua) //nolint:errcheck
	}()

	http.Redirect(w, r, target, http.StatusFound)
}

// substitutePattern replaces placeholders in a URL template with the
// remaining path segments from the request.
//
// Supported placeholders:
//
//	{*}   — the entire remaining path (URL-path encoded as-is)
//	{1}   — first path segment
//	{2}   — second path segment (etc.)
//
// Examples:
//
//	template "https://github.com/{*}", remaining "myorg/myrepo"
//	  → "https://github.com/myorg/myrepo"
//
//	template "https://jira.example.com/browse/{1}", remaining "PROJ-123"
//	  → "https://jira.example.com/browse/PROJ-123"
func substitutePattern(template, remaining string) string {
	result := strings.ReplaceAll(template, "{*}", remaining)

	segments := strings.Split(remaining, "/")
	for i, seg := range segments {
		placeholder := fmt.Sprintf("{%d}", i+1)
		result = strings.ReplaceAll(result, placeholder, seg)
	}

	return result
}

// CSVRecord converts a link to a CSV row for export.
// Columns: shortname, target_url, description, is_pattern
func CSVRecord(shortname, targetURL, description string, isPattern bool) []string {
	pattern := "false"
	if isPattern {
		pattern = "true"
	}
	return []string{shortname, targetURL, description, pattern}
}

// CSVHeader returns the header row for link CSV exports.
func CSVHeader() []string {
	return []string{"shortname", "target_url", "description", "is_pattern"}
}

// ParseCSV parses a CSV export back into a slice of field maps.
// Returns rows as [shortname, target_url, description, is_pattern].
func ParseCSV(r *csv.Reader) ([][]string, error) {
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	// Strip header row if present.
	if len(records) > 0 && records[0][0] == "shortname" {
		records = records[1:]
	}
	return records, nil
}
