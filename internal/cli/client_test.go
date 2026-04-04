package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/billcobbler/golinks/internal/models"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(&Config{Server: srv.URL})
}

func TestClient_ListLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/api/links", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result := &models.ListResult{
			Links: []*models.Link{
				{Shortname: "docs", TargetURL: "https://docs.example.com", ClickCount: 5},
			},
			Total: 1, Offset: 0, Limit: 50,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	client := newTestClient(t, mux)
	result, err := client.ListLinks("", 0, 50)
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if result.Links[0].Shortname != "docs" {
		t.Errorf("Shortname = %q, want %q", result.Links[0].Shortname, "docs")
	}
}

func TestClient_GetLink(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/api/links/docs", func(w http.ResponseWriter, r *http.Request) {
		link := &models.Link{
			Shortname: "docs", TargetURL: "https://docs.example.com",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(link)
	})

	client := newTestClient(t, mux)
	link, err := client.GetLink("docs")
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if link.Shortname != "docs" {
		t.Errorf("Shortname = %q, want %q", link.Shortname, "docs")
	}
}

func TestClient_GetLink_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/api/links/missing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "link not found"})
	})

	client := newTestClient(t, mux)
	_, err := client.GetLink("missing")
	if err == nil {
		t.Fatal("expected error for missing link, got nil")
	}
}

func TestClient_CreateLink(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/api/links", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req CreateLinkInput
		_ = json.NewDecoder(r.Body).Decode(&req)
		link := &models.Link{
			Shortname: req.Shortname, TargetURL: req.TargetURL,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(link)
	})

	client := newTestClient(t, mux)
	link, err := client.CreateLink(CreateLinkInput{
		Shortname: "test", TargetURL: "https://test.example.com",
	})
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if link.Shortname != "test" {
		t.Errorf("Shortname = %q, want %q", link.Shortname, "test")
	}
}

func TestClient_DeleteLink(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/api/links/docs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client := newTestClient(t, mux)
	if err := client.DeleteLink("docs"); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
}

func TestClient_GetStats(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := &models.Stats{TotalLinks: 10, TotalClicks: 250}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
	})

	client := newTestClient(t, mux)
	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalLinks != 10 {
		t.Errorf("TotalLinks = %d, want 10", stats.TotalLinks)
	}
	if stats.TotalClicks != 250 {
		t.Errorf("TotalClicks = %d, want 250", stats.TotalClicks)
	}
}
