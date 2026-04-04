package store_test

import (
	"os"
	"testing"
	"time"

	"github.com/billcobbler/golinks/internal/models"
	"github.com/billcobbler/golinks/internal/store"
)

// newTestStore creates an in-memory SQLite store for tests.
func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetLink(t *testing.T) {
	s := newTestStore(t)

	link := &models.Link{
		Shortname:   "docs",
		TargetURL:   "https://docs.example.com",
		Description: "Internal docs",
	}

	if err := s.CreateLink(link); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if link.ID == 0 {
		t.Error("expected non-zero ID after create")
	}

	got, err := s.GetLink("docs")
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if got.TargetURL != link.TargetURL {
		t.Errorf("TargetURL: got %q want %q", got.TargetURL, link.TargetURL)
	}
}

func TestCreateLink_Conflict(t *testing.T) {
	s := newTestStore(t)
	link := &models.Link{Shortname: "dup", TargetURL: "https://a.example.com"}
	if err := s.CreateLink(link); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := s.CreateLink(&models.Link{Shortname: "dup", TargetURL: "https://b.example.com"})
	if !store.IsConflict(err) {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestGetLink_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetLink("nope")
	if !store.IsNotFound(err) {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

func TestUpdateLink(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateLink(&models.Link{Shortname: "gh", TargetURL: "https://github.com"}); err != nil {
		t.Fatal(err)
	}

	updated := &models.Link{Shortname: "gh", TargetURL: "https://github.com/myorg", Description: "updated"}
	if err := s.UpdateLink(updated); err != nil {
		t.Fatalf("UpdateLink: %v", err)
	}

	got, _ := s.GetLink("gh")
	if got.TargetURL != "https://github.com/myorg" {
		t.Errorf("TargetURL not updated: got %q", got.TargetURL)
	}
	if got.Description != "updated" {
		t.Errorf("Description not updated: got %q", got.Description)
	}
}

func TestUpdateLink_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateLink(&models.Link{Shortname: "ghost", TargetURL: "https://example.com"})
	if !store.IsNotFound(err) {
		t.Errorf("expected not-found, got: %v", err)
	}
}

func TestDeleteLink(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateLink(&models.Link{Shortname: "bye", TargetURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLink("bye"); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	_, err := s.GetLink("bye")
	if !store.IsNotFound(err) {
		t.Error("expected not-found after delete")
	}
}

func TestListLinks(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		_ = s.CreateLink(&models.Link{Shortname: name, TargetURL: "https://example.com/" + name})
	}

	result, err := s.ListLinks("", 0, 10)
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("Total: got %d want 3", result.Total)
	}
	if len(result.Links) != 3 {
		t.Errorf("len(Links): got %d want 3", len(result.Links))
	}
}

func TestListLinks_Search(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateLink(&models.Link{Shortname: "go-docs", TargetURL: "https://go.dev/docs", Description: "Go docs"})
	_ = s.CreateLink(&models.Link{Shortname: "jira", TargetURL: "https://jira.example.com"})

	result, err := s.ListLinks("go", 0, 10)
	if err != nil {
		t.Fatalf("ListLinks search: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 result for 'go', got %d", result.Total)
	}
}

func TestGetLinkByPath_ExactMatch(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateLink(&models.Link{Shortname: "team/standup", TargetURL: "https://meet.example.com/standup"})

	link, remaining, err := s.GetLinkByPath("team/standup")
	if err != nil {
		t.Fatalf("GetLinkByPath: %v", err)
	}
	if link.Shortname != "team/standup" {
		t.Errorf("got shortname %q", link.Shortname)
	}
	if remaining != "" {
		t.Errorf("expected empty remaining, got %q", remaining)
	}
}

func TestGetLinkByPath_PatternMatch(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateLink(&models.Link{
		Shortname: "gh",
		TargetURL: "https://github.com/{*}",
		IsPattern: true,
	})

	link, remaining, err := s.GetLinkByPath("gh/myorg/myrepo")
	if err != nil {
		t.Fatalf("GetLinkByPath pattern: %v", err)
	}
	if link.Shortname != "gh" {
		t.Errorf("got shortname %q want gh", link.Shortname)
	}
	if remaining != "myorg/myrepo" {
		t.Errorf("remaining: got %q want %q", remaining, "myorg/myrepo")
	}
}

func TestGetLinkByPath_LongestPrefixWins(t *testing.T) {
	s := newTestStore(t)
	// Two pattern links with overlapping prefixes.
	_ = s.CreateLink(&models.Link{Shortname: "eng", TargetURL: "https://eng.example.com/{*}", IsPattern: true})
	_ = s.CreateLink(&models.Link{Shortname: "eng/docs", TargetURL: "https://docs.eng.example.com/{*}", IsPattern: true})

	link, remaining, err := s.GetLinkByPath("eng/docs/api")
	if err != nil {
		t.Fatalf("GetLinkByPath longest prefix: %v", err)
	}
	if link.Shortname != "eng/docs" {
		t.Errorf("expected eng/docs to win, got %q", link.Shortname)
	}
	if remaining != "api" {
		t.Errorf("remaining: got %q want api", remaining)
	}
}

func TestRecordClick(t *testing.T) {
	s := newTestStore(t)
	link := &models.Link{Shortname: "clickme", TargetURL: "https://example.com"}
	_ = s.CreateLink(link)

	if err := s.RecordClick(link.ID, "https://referrer.example.com", "TestAgent/1.0"); err != nil {
		t.Fatalf("RecordClick: %v", err)
	}

	got, _ := s.GetLink("clickme")
	if got.ClickCount != 1 {
		t.Errorf("ClickCount: got %d want 1", got.ClickCount)
	}
	if got.LastClicked == nil {
		t.Error("expected LastClicked to be set")
	}
}

func TestGetStats(t *testing.T) {
	s := newTestStore(t)
	link := &models.Link{Shortname: "popular", TargetURL: "https://example.com"}
	_ = s.CreateLink(link)
	_ = s.RecordClick(link.ID, "", "")
	_ = s.RecordClick(link.ID, "", "")

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalLinks != 1 {
		t.Errorf("TotalLinks: got %d want 1", stats.TotalLinks)
	}
	if stats.TotalClicks != 2 {
		t.Errorf("TotalClicks: got %d want 2", stats.TotalClicks)
	}
}

func TestPruneAnalytics(t *testing.T) {
	s := newTestStore(t)
	link := &models.Link{Shortname: "prune-me", TargetURL: "https://example.com"}
	_ = s.CreateLink(link)
	_ = s.RecordClick(link.ID, "", "")

	// Prune everything older than the future — should delete the click.
	n, err := s.PruneAnalytics(time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("PruneAnalytics: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row pruned, got %d", n)
	}
}

func TestExportImport(t *testing.T) {
	s := newTestStore(t)
	originals := []*models.Link{
		{Shortname: "a", TargetURL: "https://a.example.com"},
		{Shortname: "b", TargetURL: "https://b.example.com", IsPattern: true},
	}
	for _, l := range originals {
		_ = s.CreateLink(l)
	}

	exported, err := s.ExportLinks()
	if err != nil {
		t.Fatalf("ExportLinks: %v", err)
	}
	if len(exported) != 2 {
		t.Fatalf("exported %d links, want 2", len(exported))
	}

	// Import into a fresh store.
	s2 := newTestStore(t)
	imported, err := s2.ImportLinks(exported, false)
	if err != nil {
		t.Fatalf("ImportLinks: %v", err)
	}
	if imported != 2 {
		t.Errorf("imported %d, want 2", imported)
	}

	got, _ := s2.GetLink("b")
	if !got.IsPattern {
		t.Error("expected IsPattern=true after import")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
