package api

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
	"github.com/go-chi/chi/v5"
)

// Handlers holds dependencies for the API handler methods.
type Handlers struct {
	store store.Store
}

// NewHandlers returns a new Handlers bound to the given store.
func NewHandlers(s store.Store) *Handlers {
	return &Handlers{store: s}
}

// ─── Links ────────────────────────────────────────────────────────────────────

// ListLinks handles GET /api/links
func (h *Handlers) ListLinks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("q")
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	result, err := h.store.ListLinks(search, offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetLink handles GET /api/links/{shortname}
func (h *Handlers) GetLink(w http.ResponseWriter, r *http.Request) {
	shortname := chi.URLParam(r, "*")

	link, err := h.store.GetLink(shortname)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get link")
		return
	}
	writeJSON(w, http.StatusOK, link)
}

// createLinkRequest is the request body for creating a link.
type createLinkRequest struct {
	Shortname   string `json:"shortname"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
	IsPattern   bool   `json:"is_pattern"`
}

// CreateLink handles POST /api/links
func (h *Handlers) CreateLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := validateLinkRequest(req.Shortname, req.TargetURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	link := &models.Link{
		Shortname:   normalizeShortname(req.Shortname),
		TargetURL:   strings.TrimSpace(req.TargetURL),
		Description: strings.TrimSpace(req.Description),
		IsPattern:   req.IsPattern,
	}

	if err := h.store.CreateLink(link); err != nil {
		if store.IsConflict(err) {
			writeError(w, http.StatusConflict, "shortname already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create link")
		return
	}

	writeJSON(w, http.StatusCreated, link)
}

// updateLinkRequest is the request body for updating a link.
type updateLinkRequest struct {
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
	IsPattern   bool   `json:"is_pattern"`
}

// UpdateLink handles PUT /api/links/{shortname}
func (h *Handlers) UpdateLink(w http.ResponseWriter, r *http.Request) {
	shortname := chi.URLParam(r, "*")

	var req updateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.TargetURL) == "" {
		writeError(w, http.StatusBadRequest, "target_url is required")
		return
	}

	link := &models.Link{
		Shortname:   shortname,
		TargetURL:   strings.TrimSpace(req.TargetURL),
		Description: strings.TrimSpace(req.Description),
		IsPattern:   req.IsPattern,
	}

	if err := h.store.UpdateLink(link); err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update link")
		return
	}

	// Return updated link.
	updated, err := h.store.GetLink(shortname)
	if err != nil {
		writeJSON(w, http.StatusOK, link)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLink handles DELETE /api/links/{shortname}
func (h *Handlers) DeleteLink(w http.ResponseWriter, r *http.Request) {
	shortname := chi.URLParam(r, "*")

	if err := h.store.DeleteLink(shortname); err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete link")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Stats ────────────────────────────────────────────────────────────────────

// GetStats handles GET /api/stats
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ─── Export / Import ──────────────────────────────────────────────────────────

// ExportLinks handles GET /api/export?format=json|csv
func (h *Handlers) ExportLinks(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	links, err := h.store.ExportLinks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export failed")
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=golinks-export.csv")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"shortname", "target_url", "description", "is_pattern"})
		for _, link := range links {
			pattern := "false"
			if link.IsPattern {
				pattern = "true"
			}
			_ = cw.Write([]string{link.Shortname, link.TargetURL, link.Description, pattern})
		}
		cw.Flush()

	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=golinks-export.json")
		_ = json.NewEncoder(w).Encode(links)
	}
}

// importResult is the response envelope for POST /api/import.
type importResult struct {
	Imported int    `json:"imported"`
	Message  string `json:"message"`
}

// ImportLinks handles POST /api/import
// Accepts JSON array or CSV (detected by Content-Type).
func (h *Handlers) ImportLinks(w http.ResponseWriter, r *http.Request) {
	overwrite := r.URL.Query().Get("overwrite") == "true"
	contentType := r.Header.Get("Content-Type")

	var links []*models.Link

	if strings.Contains(contentType, "text/csv") {
		cr := csv.NewReader(r.Body)
		records, err := cr.ReadAll()
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
			return
		}
		// Strip header if present.
		if len(records) > 0 && records[0][0] == "shortname" {
			records = records[1:]
		}
		for _, row := range records {
			if len(row) < 2 {
				continue
			}
			link := &models.Link{
				Shortname: normalizeShortname(row[0]),
				TargetURL: strings.TrimSpace(row[1]),
			}
			if len(row) >= 3 {
				link.Description = strings.TrimSpace(row[2])
			}
			if len(row) >= 4 {
				link.IsPattern = strings.EqualFold(row[3], "true")
			}
			links = append(links, link)
		}
	} else {
		// Default: JSON array.
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB limit
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}
		if err := json.Unmarshal(body, &links); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	if len(links) == 0 {
		writeError(w, http.StatusBadRequest, "no links provided")
		return
	}

	imported, err := h.store.ImportLinks(links, overwrite)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, importResult{
		Imported: imported,
		Message:  formatImportMessage(imported, len(links), overwrite),
	})
}

// ─── Health ───────────────────────────────────────────────────────────────────

// healthResponse is the response body for GET /api/health.
type healthResponse struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

// Health handles GET /api/health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Time:   time.Now().UTC(),
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func normalizeShortname(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.Trim(s, "/")
	return s
}

func validateLinkRequest(shortname, targetURL string) error {
	normalized := normalizeShortname(shortname)
	if normalized == "" {
		return errorf("shortname is required")
	}
	if strings.ContainsAny(shortname, " \t\n") {
		return errorf("shortname must not contain whitespace")
	}
	if strings.HasPrefix(normalized, "-") {
		return errorf("shortname may not start with '-' (reserved namespace)")
	}
	if strings.TrimSpace(targetURL) == "" {
		return errorf("target_url is required")
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return errorf("target_url must start with http:// or https://")
	}
	return nil
}

// errorf is a simple string-based error constructor used for validation messages.
type validationError string

func (e validationError) Error() string { return string(e) }

func errorf(msg string) error { return validationError(msg) }

func formatImportMessage(imported, total int, overwrite bool) string {
	skipped := total - imported
	action := "created"
	if overwrite {
		action = "upserted"
	}
	if skipped == 0 {
		return strconv.Itoa(imported) + " links " + action
	}
	return strconv.Itoa(imported) + " links " + action + ", " + strconv.Itoa(skipped) + " skipped (already exist)"
}
