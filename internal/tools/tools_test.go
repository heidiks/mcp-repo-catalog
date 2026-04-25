package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heidiks/mcp-repo-catalog/internal/azuredevops"
	"github.com/heidiks/mcp-repo-catalog/internal/cache"
	gh "github.com/heidiks/mcp-repo-catalog/internal/github"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Test Helpers ---

type rewriteTransport struct {
	base      http.RoundTripper
	targetURL string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(rt.targetURL, "http://")
	return rt.base.RoundTrip(req)
}

func adoClientFromTestServer(t *testing.T, ts *httptest.Server) *azuredevops.Client {
	t.Helper()
	transport := &rewriteTransport{base: http.DefaultTransport, targetURL: ts.URL}
	return azuredevops.NewClientWithHTTPClient("testorg", "testpat", &http.Client{Transport: transport})
}

func ghClientFromTestServer(t *testing.T, ts *httptest.Server) *gh.Client {
	t.Helper()
	return gh.NewClientWithHTTPClient("test-token", nil, "", ts.Client(), ts.URL)
}

func callToolViaSession(t *testing.T, server *mcp.Server, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) failed: %v", toolName, err)
	}
	if result.IsError {
		t.Fatalf("CallTool(%s) returned error result", toolName)
	}

	return result
}

func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// --- List Projects Tests (ADO) ---

func TestListProjectsADO(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"count": 2, "value": []map[string]any{
			{"name": "ProjectA", "description": "First", "lastUpdateTime": "2026-04-10T00:00:00Z"},
			{"name": "ProjectB", "description": "", "lastUpdateTime": "2026-04-08T00:00:00Z"},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterListProjects(server, registry, c)

	result := callToolViaSession(t, server, "list_projects", map[string]any{})
	text := extractText(t, result)

	if !strings.Contains(text, "ProjectA") {
		t.Errorf("expected ProjectA, got: %s", text)
	}
	if !strings.Contains(text, "azuredevops") {
		t.Errorf("expected azuredevops provider label, got: %s", text)
	}
}

// --- List Projects Tests (Multi-provider) ---

func TestListProjectsMultiProvider(t *testing.T) {
	adoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"count": 1, "value": []map[string]any{
			{"name": "ado-project", "description": "ADO", "lastUpdateTime": "2026-04-10T00:00:00Z"},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer adoServer.Close()

	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"login": "gh-org", "description": "GitHub org"},
		})
	}))
	defer ghServer.Close()

	registry := provider.NewRegistry(
		adoClientFromTestServer(t, adoServer),
		ghClientFromTestServer(t, ghServer),
	)
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterListProjects(server, registry, c)

	result := callToolViaSession(t, server, "list_projects", map[string]any{})
	text := extractText(t, result)

	if !strings.Contains(text, "ado-project") {
		t.Errorf("expected ado-project, got: %s", text)
	}
	if !strings.Contains(text, "gh-org") {
		t.Errorf("expected gh-org, got: %s", text)
	}
}

func TestListProjectsCaching(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := map[string]any{"count": 1, "value": []map[string]any{{"name": "P1"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterListProjects(server, registry, c)

	callToolViaSession(t, server, "list_projects", map[string]any{})
	callToolViaSession(t, server, "list_projects", map[string]any{})

	if callCount != 1 {
		t.Errorf("expected 1 API call (cached), got %d", callCount)
	}
}

// --- Search Repos Tests ---

func TestSearchReposMatchesName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"count": 3, "value": []map[string]any{
			{"name": "auth-gatekeeper", "defaultBranch": "refs/heads/master", "project": map[string]any{"name": "backend"}},
			{"name": "captcha-gateway", "defaultBranch": "refs/heads/main", "project": map[string]any{"name": "backend"}},
			{"name": "frontend-app", "defaultBranch": "refs/heads/main", "project": map[string]any{"name": "backend"}},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterSearchRepos(server, registry, c, locator.New(nil))

	result := callToolViaSession(t, server, "search_repositories", map[string]any{
		"query": "auth", "project": "backend",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "auth-gatekeeper") {
		t.Errorf("expected auth-gatekeeper, got: %s", text)
	}
	if strings.Contains(text, "frontend-app") {
		t.Errorf("frontend-app should not match, got: %s", text)
	}
}

func TestSearchReposNoResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"count": 0, "value": []any{}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterSearchRepos(server, registry, c, locator.New(nil))

	result := callToolViaSession(t, server, "search_repositories", map[string]any{
		"query": "nonexistent", "project": "p1",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "No repositories found") {
		t.Errorf("expected 'No repositories found', got: %s", text)
	}
}

// --- Repo Details Tests ---

func TestRepoDetailsWithReadme(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/commits"):
			resp := map[string]any{"count": 1, "value": []map[string]any{
				{"commitId": "abc123", "comment": "fix bug", "author": map[string]any{
					"name": "Dev", "email": "dev@test.com", "date": "2026-04-14T10:00:00Z",
				}},
			}}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/items"):
			resp := map[string]any{"path": "/README.md", "content": "# My Repo\nGreat stuff"}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			resp := map[string]any{
				"name": "my-repo", "defaultBranch": "refs/heads/main",
				"size": 50000, "webUrl": "https://dev.azure.com/org/proj/_git/my-repo",
				"project": map[string]any{"name": "proj"},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	registry := provider.NewRegistry(adoClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterRepoDetails(server, registry, c, locator.New(nil))

	result := callToolViaSession(t, server, "get_repo_details", map[string]any{
		"project": "proj", "repo": "my-repo",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "my-repo") {
		t.Errorf("expected repo name, got: %s", text)
	}
	if !strings.Contains(text, "fix bug") {
		t.Errorf("expected commit, got: %s", text)
	}
}

func TestRepoDetailsGitHub(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("# GH Repo"))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/commits"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"sha": "def456", "commit": map[string]any{
					"message": "init", "author": map[string]any{
						"name": "Dev", "email": "dev@gh.com", "date": "2026-04-14T10:00:00Z",
					},
				}},
			})
		case strings.Contains(r.URL.Path, "/contents"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "README.md", "path": "README.md", "type": "file", "content": encoded,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "gh-repo", "default_branch": "main", "size": 100,
				"html_url": "https://github.com/org/gh-repo",
				"owner":    map[string]any{"login": "org"},
			})
		}
	}))
	defer ts.Close()

	registry := provider.NewRegistry(ghClientFromTestServer(t, ts))
	c := cache.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	RegisterRepoDetails(server, registry, c, locator.New(nil))

	result := callToolViaSession(t, server, "get_repo_details", map[string]any{
		"project": "org", "repo": "gh-repo",
	})
	text := extractText(t, result)

	if !strings.Contains(text, "gh-repo") {
		t.Errorf("expected gh-repo, got: %s", text)
	}
	if !strings.Contains(text, "github") {
		t.Errorf("expected github provider, got: %s", text)
	}
}

// --- Format Size Tests ---

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{2621440, "2.5 MB"},
	}

	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.expected {
			t.Errorf("formatSize(%d) = %s, want %s", tt.bytes, got, tt.expected)
		}
	}
}
