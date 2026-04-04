package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
	"os"

	"github.com/billcobbler/golinks/internal/api"
	"github.com/billcobbler/golinks/internal/config"
	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("newTestServer store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	cfg := &config.Config{AuthMode: "none"}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	router := api.NewRouter(s, cfg, log)
	return httptest.NewServer(router)
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/-/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d want 200", resp.StatusCode)
	}
}

func TestCreateAndListLinks(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// Create
	body := `{"shortname":"docs","target_url":"https://docs.example.com","description":"Docs"}`
	resp, err := http.Post(srv.URL+"/-/api/links", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("create status: got %d want 201", resp.StatusCode)
	}

	var created models.Link
	_ = json.NewDecoder(resp.Body).Decode(&created)
	if created.Shortname != "docs" {
		t.Errorf("shortname: got %q want docs", created.Shortname)
	}

	// List
	resp2, err := http.Get(srv.URL + "/-/api/links")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var result models.ListResult
	_ = json.NewDecoder(resp2.Body).Decode(&result)
	if result.Total != 1 {
		t.Errorf("total: got %d want 1", result.Total)
	}
}

func TestCreateLink_Validation(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing shortname", `{"target_url":"https://example.com"}`, http.StatusBadRequest},
		{"missing url", `{"shortname":"foo"}`, http.StatusBadRequest},
		{"bad url scheme", `{"shortname":"foo","target_url":"ftp://example.com"}`, http.StatusBadRequest},
		{"reserved namespace", `{"shortname":"-internal","target_url":"https://example.com"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(srv.URL+"/-/api/links", "application/json", bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status: got %d want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestCreateLink_Conflict(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	body := `{"shortname":"dup","target_url":"https://example.com"}`
	http.Post(srv.URL+"/-/api/links", "application/json", bytes.NewBufferString(body)) //nolint
	resp, err := http.Post(srv.URL+"/-/api/links", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

func TestUpdateLink(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// Seed
	http.Post(srv.URL+"/-/api/links", "application/json", //nolint
		bytes.NewBufferString(`{"shortname":"upd","target_url":"https://old.example.com"}`))

	// Update
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/-/api/links/upd",
		bytes.NewBufferString(`{"target_url":"https://new.example.com","description":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("update status: got %d want 200", resp.StatusCode)
	}

	var updated models.Link
	_ = json.NewDecoder(resp.Body).Decode(&updated)
	if updated.TargetURL != "https://new.example.com" {
		t.Errorf("TargetURL: got %q", updated.TargetURL)
	}
}

func TestDeleteLink(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	http.Post(srv.URL+"/-/api/links", "application/json", //nolint
		bytes.NewBufferString(`{"shortname":"del","target_url":"https://example.com"}`))

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/-/api/links/del", nil)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status: got %d want 204", resp.StatusCode)
	}

	// Confirm it's gone.
	resp2, _ := http.Get(srv.URL + "/-/api/links/del")
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestGetStats(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/-/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("stats status: got %d want 200", resp.StatusCode)
	}

	var stats models.Stats
	_ = json.NewDecoder(resp.Body).Decode(&stats)
	if stats.TotalLinks != 0 {
		t.Errorf("TotalLinks: got %d want 0", stats.TotalLinks)
	}
}

func TestExportJSON(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	http.Post(srv.URL+"/-/api/links", "application/json", //nolint
		bytes.NewBufferString(`{"shortname":"export-me","target_url":"https://example.com"}`))

	resp, err := http.Get(srv.URL + "/-/api/export?format=json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("export status: got %d want 200", resp.StatusCode)
	}

	var links []models.Link
	_ = json.NewDecoder(resp.Body).Decode(&links)
	if len(links) != 1 {
		t.Errorf("exported %d links, want 1", len(links))
	}
}

func TestRedirect(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	http.Post(srv.URL+"/-/api/links", "application/json", //nolint
		bytes.NewBufferString(`{"shortname":"go-home","target_url":"https://example.com/home"}`))

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // don't follow redirects
	}}

	resp, err := client.Get(srv.URL + "/go-home")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("redirect status: got %d want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://example.com/home" {
		t.Errorf("Location: got %q want https://example.com/home", loc)
	}
}

func TestRedirectPattern(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	http.Post(srv.URL+"/-/api/links", "application/json", //nolint
		bytes.NewBufferString(`{"shortname":"gh","target_url":"https://github.com/{*}","is_pattern":true}`))

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(srv.URL + "/gh/myorg/myrepo")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("redirect status: got %d want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://github.com/myorg/myrepo" {
		t.Errorf("Location: got %q want https://github.com/myorg/myrepo", loc)
	}
}

func TestRedirect_NotFound(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
