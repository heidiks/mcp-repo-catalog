package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStoreAndSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "catalog.json")

	store := NewStore(path)

	repos := []RepoEntry{
		{Provider: "azuredevops", Project: "Acme", Name: "go-monorepo", URL: "https://example.com", Domain: "backend"},
		{Provider: "github", Project: "acme", Name: "frontend", URL: "https://github.com/acme/frontend"},
	}
	store.SetRepositories(repos)

	if err := store.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	store2 := NewStore(path)
	if err := store2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	got := store2.GetAll()
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}

	if got[0].Name != "go-monorepo" {
		t.Fatalf("expected go-monorepo, got %s", got[0].Name)
	}
}

func TestLoadNonExistent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing.json"))

	if err := store.Load(); err != nil {
		t.Fatalf("load of missing file should not error: %v", err)
	}

	if len(store.GetAll()) != 0 {
		t.Fatal("expected empty catalog")
	}
}

func TestSearch(t *testing.T) {
	store := NewStore("")
	store.SetRepositories([]RepoEntry{
		{Name: "auth-gatekeeper", Domain: "identity", Description: "IAM service"},
		{Name: "go-monorepo", Domain: "backend", Description: "Monorepo"},
		{Name: "pagamentos", Domain: "payments", Description: "Payment processing"},
	})

	tests := []struct {
		query    string
		expected int
	}{
		{"auth", 1},
		{"payment", 1}, // matches desc "Payment processing" on pagamentos
		{"identity", 1},
		{"nonexistent", 0},
		{"mono", 1},
	}

	for _, tt := range tests {
		got := store.Search(tt.query)
		if len(got) != tt.expected {
			names := make([]string, len(got))
			for i, r := range got {
				names[i] = r.Name
			}
			t.Errorf("Search(%q) = %d results %v, want %d", tt.query, len(got), names, tt.expected)
		}
	}
}

func TestUpdateLocalPath(t *testing.T) {
	store := NewStore("")
	store.SetRepositories([]RepoEntry{
		{Provider: "azuredevops", Project: "P1", Name: "repo1"},
	})

	store.UpdateLocalPath("azuredevops", "P1", "repo1", "/some/path")

	entry := store.FindByName("azuredevops", "P1", "repo1")
	if entry == nil {
		t.Fatal("expected to find repo1")
	}
	if entry.LocalPath != "/some/path" {
		t.Fatalf("expected /some/path, got %s", entry.LocalPath)
	}
}

func TestFindByNameNotFound(t *testing.T) {
	store := NewStore("")
	store.SetRepositories([]RepoEntry{})

	entry := store.FindByName("github", "org", "missing")
	if entry != nil {
		t.Fatal("expected nil for missing repo")
	}
}

func TestIsStale(t *testing.T) {
	store := NewStore("")

	// Empty catalog is stale
	if !store.IsStale(24 * time.Hour) {
		t.Fatal("empty catalog should be stale")
	}

	store.SetRepositories([]RepoEntry{{Name: "test"}})

	// Just synced, not stale
	if store.IsStale(24 * time.Hour) {
		t.Fatal("fresh catalog should not be stale")
	}

	// Force old timestamp
	store.data.LastSynced = time.Now().Add(-25 * time.Hour)
	if !store.IsStale(24 * time.Hour) {
		t.Fatal("old catalog should be stale")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", "c", "catalog.json")

	store := NewStore(nested)
	store.SetRepositories([]RepoEntry{{Name: "test"}})

	if err := store.Save(); err != nil {
		t.Fatalf("save should create dirs: %v", err)
	}

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}
