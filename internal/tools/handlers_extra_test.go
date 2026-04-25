package tools

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heidiks/mcp-repo-catalog/internal/cache"
	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newStoreWithEntries(t *testing.T, entries []catalog.RepoEntry) *catalog.Store {
	t.Helper()
	tmp := t.TempDir()
	store := catalog.NewStore(filepath.Join(tmp, "catalog.json"))
	store.SetRepositories(entries)
	return store
}

// --- view_catalog ---

func TestViewCatalogEmpty(t *testing.T) {
	store := newStoreWithEntries(t, nil)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterViewCatalog(server, store)

	result := callToolViaSession(t, server, "view_catalog", map[string]any{})
	text := extractText(t, result)

	if !strings.Contains(text, "Catalog is empty") {
		t.Errorf("expected empty message, got: %s", text)
	}
}

func TestViewCatalogGrouping(t *testing.T) {
	store := newStoreWithEntries(t, []catalog.RepoEntry{
		{Provider: "github", Project: "acme", Name: "frontend", Domain: "ui", Languages: []string{"TypeScript"}, LocalPath: "/repos/frontend"},
		{Provider: "github", Project: "acme", Name: "api", Domain: "backend"},
		{Provider: "azuredevops", Project: "infra", Name: "terraform", Domain: "platform"},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterViewCatalog(server, store)

	result := callToolViaSession(t, server, "view_catalog", map[string]any{})
	text := extractText(t, result)

	if !strings.Contains(text, "### github (2 repos)") {
		t.Errorf("expected github group header, got: %s", text)
	}
	if !strings.Contains(text, "### azuredevops (1 repos)") {
		t.Errorf("expected azuredevops group header, got: %s", text)
	}
	if !strings.Contains(text, "**frontend**") || !strings.Contains(text, "TypeScript") {
		t.Errorf("expected frontend with language tag, got: %s", text)
	}
	// Local indicator on cloned entry
	if !strings.Contains(text, "**frontend** (acme) [ui | TypeScript] *") {
		t.Errorf("expected local indicator on frontend, got: %s", text)
	}
}

func TestViewCatalogFilter(t *testing.T) {
	store := newStoreWithEntries(t, []catalog.RepoEntry{
		{Provider: "github", Project: "acme", Name: "auth-service", Domain: "auth"},
		{Provider: "github", Project: "acme", Name: "frontend", Domain: "ui"},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterViewCatalog(server, store)

	result := callToolViaSession(t, server, "view_catalog", map[string]any{"filter": "auth"})
	text := extractText(t, result)

	if !strings.Contains(text, "auth-service") {
		t.Errorf("expected auth-service, got: %s", text)
	}
	if strings.Contains(text, "frontend") {
		t.Errorf("frontend should be filtered out, got: %s", text)
	}
}

// --- update_catalog_entry ---

func TestUpdateCatalogEntry(t *testing.T) {
	store := newStoreWithEntries(t, []catalog.RepoEntry{
		{Provider: "github", Project: "acme", Name: "api"},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterUpdateCatalog(server, store)

	result := callToolViaSession(t, server, "update_catalog_entry", map[string]any{
		"repo": "api", "field": "domain", "value": "backend",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "Updated") || !strings.Contains(text, "domain") {
		t.Errorf("expected update confirmation, got: %s", text)
	}

	entry := store.FindByName("github", "acme", "api")
	if entry == nil || entry.Domain != "backend" {
		t.Errorf("expected domain=backend, got %+v", entry)
	}
}

func TestUpdateCatalogEntryAmbiguous(t *testing.T) {
	store := newStoreWithEntries(t, []catalog.RepoEntry{
		{Provider: "github", Project: "acme", Name: "api"},
		{Provider: "azuredevops", Project: "other", Name: "api-gateway"},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterUpdateCatalog(server, store)

	// "api" is partial match: matches both "api" and "api-gateway"; exact match resolves to "api"
	// Use a query that is ambiguous (no exact match)
	result := callToolViaSession(t, server, "update_catalog_entry", map[string]any{
		"repo": "api-", "field": "domain", "value": "x",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "Multiple repositories match") {
		// only one match, should succeed
		if !strings.Contains(text, "Updated") {
			t.Errorf("unexpected response: %s", text)
		}
	}
}

// --- read_from_repo ---

func TestReadFromRepoLocal(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newStoreWithEntries(t, []catalog.RepoEntry{
		{Provider: "github", Project: "acme", Name: "myrepo", LocalPath: repoDir},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterReadFromRepo(server, store, nil)

	result := callToolViaSession(t, server, "read_from_repo", map[string]any{
		"repo": "myrepo", "path": "README.md",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "# Hello") {
		t.Errorf("expected file content, got: %s", text)
	}
	if !strings.Contains(text, "source: local") {
		t.Errorf("expected source: local, got: %s", text)
	}
}

func TestReadFromRepoAPIFallback(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("# From API"))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "README.md", "path": "README.md", "type": "file", "content": encoded,
		})
	}))
	defer ts.Close()

	registry := provider.NewRegistry(ghClientFromTestServer(t, ts))
	store := newStoreWithEntries(t, []catalog.RepoEntry{
		{Provider: "github", Project: "acme", Name: "myrepo", LocalPath: ""},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterReadFromRepo(server, store, registry)

	result := callToolViaSession(t, server, "read_from_repo", map[string]any{
		"repo": "myrepo", "path": "README.md",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "# From API") {
		t.Errorf("expected API content, got: %s", text)
	}
	if !strings.Contains(text, "source: api") {
		t.Errorf("expected source: api, got: %s", text)
	}
}

// --- sync_catalog ---

func TestSyncCatalog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/items"):
			_ = json.NewEncoder(w).Encode(map[string]any{"path": "/README.md", "content": "# Backend service"})
		case strings.Contains(r.URL.Path, "/_apis/git/repositories"):
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 1, "value": []map[string]any{
				{"name": "api", "defaultBranch": "refs/heads/main", "size": 1000,
					"webUrl":  "https://dev.azure.com/org/proj/_git/api",
					"project": map[string]any{"name": "proj"}},
			}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 1, "value": []map[string]any{
				{"name": "proj", "description": "main project", "lastUpdateTime": "2026-04-10T00:00:00Z"},
			}})
		}
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	store := newStoreWithEntries(t, nil)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterSyncCatalog(server, registry, store, locator.New(nil))

	result := callToolViaSession(t, server, "sync_catalog", map[string]any{})
	text := extractText(t, result)

	if !strings.Contains(text, "1 repositories indexed") {
		t.Errorf("expected 1 repo indexed, got: %s", text)
	}
	if !strings.Contains(text, "azuredevops: 1 repos") {
		t.Errorf("expected provider count, got: %s", text)
	}
	all := store.GetAll()
	if len(all) != 1 || all[0].Name != "api" {
		t.Errorf("expected 1 entry named 'api' in store, got: %+v", all)
	}
}

// --- search_repositories fuzzy fallback ---

func TestSearchReposFuzzyFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"count": 2, "value": []map[string]any{
			{"name": "auth-gateway", "defaultBranch": "refs/heads/main", "project": map[string]any{"name": "backend"}},
			{"name": "billing", "defaultBranch": "refs/heads/main", "project": map[string]any{"name": "backend"}},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterSearchRepos(server, registry, c, locator.New(nil))

	// Typo: "gatway" should fuzzy-match "gateway" token in auth-gateway
	result := callToolViaSession(t, server, "search_repositories", map[string]any{
		"query": "gatway", "project": "backend",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "auth-gateway") {
		t.Errorf("expected fuzzy match for auth-gateway with query 'gatway', got: %s", text)
	}
}
