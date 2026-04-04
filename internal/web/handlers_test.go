package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
)

// newTestHandlers creates Handlers backed by a real in-memory SQLite store.
func newTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return NewHandlers(s, nil)
}

// seedLink inserts a link directly into the store.
func seedLink(t *testing.T, s store.Store, shortname, targetURL string) *models.Link {
	t.Helper()
	link := &models.Link{Shortname: shortname, TargetURL: targetURL, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.CreateLink(link); err != nil {
		t.Fatalf("seedLink %q: %v", shortname, err)
	}
	got, _ := s.GetLink(shortname)
	return got
}

func TestHandlers_Index_Redirects(t *testing.T) {
	h := newTestHandlers(t)
	r := httptest.NewRequest(http.MethodGet, "/-/", nil)
	w := httptest.NewRecorder()
	h.Index(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if loc := w.Header().Get("Location"); loc != "/-/links" {
		t.Errorf("Location = %q, want /-/links", loc)
	}
}

func TestHandlers_Links_RendersPage(t *testing.T) {
	h := newTestHandlers(t)
	r := httptest.NewRequest(http.MethodGet, "/-/links", nil)
	w := httptest.NewRecorder()
	h.Links(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "go/") {
		t.Errorf("response does not contain nav brand 'go/'")
	}
	if !strings.Contains(body, "Add link") {
		t.Errorf("response does not contain 'Add link' button")
	}
}

func TestHandlers_Links_ShowsFlashMessage(t *testing.T) {
	h := newTestHandlers(t)
	r := httptest.NewRequest(http.MethodGet, "/-/links?msg=Created+go%2Fdocs", nil)
	w := httptest.NewRecorder()
	h.Links(w, r)
	if !strings.Contains(w.Body.String(), "Created go/docs") {
		t.Error("flash message not rendered")
	}
}

func TestHandlers_Analytics_RendersPage(t *testing.T) {
	h := newTestHandlers(t)
	r := httptest.NewRequest(http.MethodGet, "/-/analytics", nil)
	w := httptest.NewRecorder()
	h.Analytics(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Analytics") {
		t.Errorf("response does not contain 'Analytics' heading")
	}
}

func TestHandlers_PartialsLinks_ReturnsRows(t *testing.T) {
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	seedLink(t, s, "docs", "https://docs.example.com")
	h := NewHandlers(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/-/partials/links", nil)
	w := httptest.NewRecorder()
	h.PartialsLinks(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "docs") {
		t.Errorf("partial does not contain shortname 'docs'")
	}
}

func TestHandlers_CreateLink_RedirectsOnSuccess(t *testing.T) {
	h := newTestHandlers(t)
	form := url.Values{"shortname": {"docs"}, "target_url": {"https://docs.example.com"}}
	r := httptest.NewRequest(http.MethodPost, "/-/links", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.CreateLink(w, r)
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/-/links") {
		t.Errorf("Location = %q, want /-/links prefix", loc)
	}
	if !strings.Contains(loc, "Created+go") && !strings.Contains(loc, "Created%20go") {
		t.Errorf("Location %q does not contain success message", loc)
	}
}

func TestHandlers_CreateLink_ConflictRedirectsWithError(t *testing.T) {
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	seedLink(t, s, "docs", "https://docs.example.com")
	h := NewHandlers(s, nil)

	form := url.Values{"shortname": {"docs"}, "target_url": {"https://other.example.com"}}
	r := httptest.NewRequest(http.MethodPost, "/-/links", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.CreateLink(w, r)
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "already+exists") &&
		!strings.Contains(w.Header().Get("Location"), "already%20exists") {
		t.Errorf("error message not in redirect: %s", w.Header().Get("Location"))
	}
}

func TestHandlers_UpdateLink_ReturnsRow(t *testing.T) {
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	seedLink(t, s, "docs", "https://docs.example.com")
	h := NewHandlers(s, nil)

	form := url.Values{"target_url": {"https://newdocs.example.com"}, "description": {"New docs"}, "is_pattern": {"false"}}
	r := httptest.NewRequest(http.MethodPut, "/-/links/update?name=docs", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.UpdateLink(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "newdocs.example.com") {
		t.Errorf("updated URL not in response: %s", body)
	}
}

func TestHandlers_DeleteLink_ReturnsEmpty(t *testing.T) {
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	seedLink(t, s, "docs", "https://docs.example.com")
	h := NewHandlers(s, nil)

	r := httptest.NewRequest(http.MethodDelete, "/-/links/delete?name=docs", nil)
	w := httptest.NewRecorder()
	h.DeleteLink(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for delete, got %q", w.Body.String())
	}
}

func TestHandlers_LinkEditRow_ReturnsForm(t *testing.T) {
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	seedLink(t, s, "docs", "https://docs.example.com")
	h := NewHandlers(s, nil)

	r := httptest.NewRequest(http.MethodGet, "/-/links/edit?name=docs", nil)
	w := httptest.NewRecorder()
	h.LinkEditRow(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `hx-put`) {
		t.Errorf("edit row missing hx-put attribute")
	}
	if !strings.Contains(body, "docs.example.com") {
		t.Errorf("edit row missing current URL value")
	}
}

func TestHandlers_NewHandlers_ParsesTemplates(t *testing.T) {
	// NewHandlers panics on template parse errors — this test confirms it doesn't.
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewHandlers panicked: %v", r)
		}
	}()
	NewHandlers(s, nil)
}
