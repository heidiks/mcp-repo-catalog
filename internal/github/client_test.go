package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heidiks/mcp-repo-catalog/internal/provider"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := NewClientWithHTTPClient("test-token", nil, "", ts.Client(), ts.URL)
	return client, ts
}

func TestName(t *testing.T) {
	client := NewClient("token", nil, "")
	if client.Name() != "github" {
		t.Fatalf("expected github, got %s", client.Name())
	}
}

func TestListProjectsAutoDiscover(t *testing.T) {
	orgs := []ghOrg{
		{Login: "acme", Description: "Acme org"},
		{Login: "another-org", Description: "Another"},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(orgs)
	})

	got, err := client.ListProjects(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}

	if got[0].Name != "acme" {
		t.Fatalf("expected acme, got %s", got[0].Name)
	}

	if got[0].Provider != "github" {
		t.Fatalf("expected provider github, got %s", got[0].Provider)
	}
}

func TestListProjectsExplicitOrgs(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make API call when orgs are explicit")
	})
	client.orgs = []string{"org1", "org2"}

	got, err := client.ListProjects(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}
}

func TestListProjectsWithUser(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]ghOrg{})
	})
	client.user = "heidiks"

	got, err := client.ListProjects(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 project (user), got %d", len(got))
	}

	if got[0].Name != "heidiks" {
		t.Fatalf("expected heidiks, got %s", got[0].Name)
	}
}

func TestListRepositories(t *testing.T) {
	repos := []ghRepository{
		{Name: "repo-a", DefaultBranch: "main", Size: 100, HTMLURL: "https://github.com/org/repo-a", Owner: ghOwner{Login: "org"}},
		{Name: "repo-b", DefaultBranch: "master", Size: 200, HTMLURL: "https://github.com/org/repo-b", Owner: ghOwner{Login: "org"}},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(repos)
	})

	got, err := client.ListRepositories(context.Background(), "org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}

	if got[0].Size != 100*1024 {
		t.Fatalf("expected size in bytes (100*1024=%d), got %d", 100*1024, got[0].Size)
	}
}

func TestGetRepository(t *testing.T) {
	repo := ghRepository{
		Name: "mcp-repo-catalog", DefaultBranch: "main", Size: 50,
		HTMLURL: "https://github.com/heidi/mcp-repo-catalog",
		Owner:   ghOwner{Login: "heidi"},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(repo)
	})

	got, err := client.GetRepository(context.Background(), "heidi", "mcp-repo-catalog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "mcp-repo-catalog" {
		t.Fatalf("expected mcp-repo-catalog, got %s", got.Name)
	}

	if got.Provider != "github" {
		t.Fatalf("expected github, got %s", got.Provider)
	}
}

func TestGetFileContent(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("# Hello World"))

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		content := ghContent{Name: "README.md", Path: "README.md", Type: "file", Content: encoded}
		_ = json.NewEncoder(w).Encode(content)
	})

	got, err := client.GetFileContent(context.Background(), "org", "repo", "/README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Content != "# Hello World" {
		t.Fatalf("expected decoded content, got %q", got.Content)
	}
}

func TestGetFileContentNotFound(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetFileContent(context.Background(), "org", "repo", "/MISSING.md")
	if err == nil {
		t.Fatal("expected error for 404")
	}

	var notFound *provider.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestGetFileContentDir(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		content := ghContent{Name: "docs", Path: "docs", Type: "dir"}
		_ = json.NewEncoder(w).Encode(content)
	})

	got, err := client.GetFileContent(context.Background(), "org", "repo", "/docs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !got.IsFolder {
		t.Fatal("expected IsFolder=true for directory")
	}
}

func TestGetLastCommit(t *testing.T) {
	commits := []ghCommit{
		{SHA: "abc123", Commit: ghCommitData{
			Message: "fix: something", Author: ghCommitAuthor{Name: "Dev", Email: "dev@test.com"},
		}},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(commits)
	})

	got, err := client.GetLastCommit(context.Background(), "org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Message != "fix: something" {
		t.Fatalf("expected commit message, got %s", got.Message)
	}
}

func TestGetLastCommitEmpty(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]ghCommit{})
	})

	got, err := client.GetLastCommit(context.Background(), "org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Fatal("expected nil for empty commits")
	}
}
