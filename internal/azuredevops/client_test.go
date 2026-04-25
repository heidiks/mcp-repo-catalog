package azuredevops

import (
	"context"
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

	client := &Client{
		httpClient: ts.Client(),
		baseURL:    ts.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
	}

	return client, ts
}

func TestName(t *testing.T) {
	client := NewClient("org", "pat")
	if client.Name() != "azuredevops" {
		t.Fatalf("expected azuredevops, got %s", client.Name())
	}
}

func TestListProjects(t *testing.T) {
	projects := []adoProject{
		{ID: "1", Name: "ProjectA", Description: "First project", State: "wellFormed"},
		{ID: "2", Name: "ProjectB", Description: "Second project", State: "wellFormed"},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		resp := listResponse[adoProject]{Count: len(projects), Value: projects}
		_ = json.NewEncoder(w).Encode(resp)
	})

	got, err := client.ListProjects(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}

	if got[0].Name != "ProjectA" {
		t.Fatalf("expected ProjectA, got %s", got[0].Name)
	}

	if got[0].Provider != "azuredevops" {
		t.Fatalf("expected provider azuredevops, got %s", got[0].Provider)
	}
}

func TestListProjectsWithPagination(t *testing.T) {
	callCount := 0

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("x-ms-continuationtoken", "page2")
			resp := listResponse[adoProject]{Count: 1, Value: []adoProject{{ID: "1", Name: "Page1"}}}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp := listResponse[adoProject]{Count: 1, Value: []adoProject{{ID: "2", Name: "Page2"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	got, err := client.ListProjects(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}

	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
}

func TestListRepositories(t *testing.T) {
	repos := []adoRepository{
		{ID: "1", Name: "repo-a", DefaultBranch: "refs/heads/main", Size: 1024},
		{ID: "2", Name: "repo-b", DefaultBranch: "refs/heads/master", Size: 2048},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := listResponse[adoRepository]{Count: len(repos), Value: repos}
		_ = json.NewEncoder(w).Encode(resp)
	})

	got, err := client.ListRepositories(context.Background(), "MyProject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}

	if got[0].DefaultBranch != "main" {
		t.Fatalf("expected branch 'main' (stripped), got %s", got[0].DefaultBranch)
	}
}

func TestGetRepository(t *testing.T) {
	repo := adoRepository{
		ID: "abc-123", Name: "go-monorepo", DefaultBranch: "refs/heads/master",
		Size: 50000, WebURL: "https://dev.azure.com/org/proj/_git/go-monorepo",
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(repo)
	})

	got, err := client.GetRepository(context.Background(), "MyProject", "go-monorepo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "go-monorepo" {
		t.Fatalf("expected go-monorepo, got %s", got.Name)
	}

	if got.Provider != "azuredevops" {
		t.Fatalf("expected provider azuredevops, got %s", got.Provider)
	}
}

func TestGetFileContent(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path != "/README.md" {
			t.Errorf("expected path=/README.md, got %s", path)
		}
		item := adoItem{Path: "/README.md", Content: "# Hello World"}
		_ = json.NewEncoder(w).Encode(item)
	})

	got, err := client.GetFileContent(context.Background(), "MyProject", "repo", "/README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Content != "# Hello World" {
		t.Fatalf("expected content, got %q", got.Content)
	}
}

func TestGetFileContentNotFound(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetFileContent(context.Background(), "MyProject", "repo", "/MISSING.md")
	if err == nil {
		t.Fatal("expected error for 404")
	}

	var notFound *provider.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestGetLastCommit(t *testing.T) {
	commits := []adoCommit{
		{CommitID: "abc123", Comment: "initial commit", Author: adoCommitAuthor{Name: "Dev", Email: "dev@test.com"}},
	}

	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := listResponse[adoCommit]{Count: 1, Value: commits}
		_ = json.NewEncoder(w).Encode(resp)
	})

	got, err := client.GetLastCommit(context.Background(), "MyProject", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("expected commit, got nil")
	}

	if got.Message != "initial commit" {
		t.Fatalf("expected 'initial commit', got %s", got.Message)
	}
}

func TestGetLastCommitEmpty(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := listResponse[adoCommit]{Count: 0, Value: []adoCommit{}}
		_ = json.NewEncoder(w).Encode(resp)
	})

	got, err := client.GetLastCommit(context.Background(), "MyProject", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil for empty commits, got %v", got)
	}
}

func TestServerError(t *testing.T) {
	client, _ := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal error"}`))
	})

	_, err := client.ListProjects(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestAuthHeader(t *testing.T) {
	client := NewClient("testorg", "mytoken123")
	if client.baseURL != "https://dev.azure.com/testorg" {
		t.Fatalf("unexpected baseURL: %s", client.baseURL)
	}
	if client.authHeader == "" {
		t.Fatal("expected auth header to be set")
	}
}
