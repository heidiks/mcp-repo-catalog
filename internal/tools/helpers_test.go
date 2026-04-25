package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heidiks/mcp-repo-catalog/internal/locator"
)

// --- injectAuth ---

func TestInjectAuthADO(t *testing.T) {
	got := injectAuth("https://dev.azure.com/org/proj/_git/repo", "ADO_PAT", "GH_TOKEN")
	want := "https://ADO_PAT@dev.azure.com/org/proj/_git/repo"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestInjectAuthGitHub(t *testing.T) {
	got := injectAuth("https://github.com/org/repo.git", "ADO_PAT", "GH_TOKEN")
	want := "https://GH_TOKEN@github.com/org/repo.git"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestInjectAuthUnknownProvider(t *testing.T) {
	url := "https://gitlab.com/org/repo"
	got := injectAuth(url, "ADO_PAT", "GH_TOKEN")
	if got != url {
		t.Errorf("expected url unchanged, got %s", got)
	}
}

func TestInjectAuthNoToken(t *testing.T) {
	url := "https://github.com/org/repo"
	got := injectAuth(url, "", "")
	if got != url {
		t.Errorf("expected url unchanged when no token, got %s", got)
	}
}

// --- isGitRepoDir / isGitRepo ---

func TestIsGitRepoDir(t *testing.T) {
	tmp := t.TempDir()
	if isGitRepoDir(tmp) {
		t.Error("expected false for non-git dir")
	}

	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isGitRepoDir(tmp) {
		t.Error("expected true for git dir")
	}
}

func TestIsGitRepo(t *testing.T) {
	tmp := t.TempDir()
	if isGitRepo(tmp) {
		t.Error("expected false for non-git dir")
	}

	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isGitRepo(tmp) {
		t.Error("expected true for git dir")
	}
}

// --- scanForRepos ---

func TestScanForReposLevel1(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"repo-a", "repo-b"} {
		if err := os.MkdirAll(filepath.Join(tmp, name, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Non-git dir should be ignored
	if err := os.MkdirAll(filepath.Join(tmp, "not-a-repo"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries := scanForRepos(tmp, "azuredevops")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Provider != "azuredevops" {
			t.Errorf("expected provider azuredevops, got %s", e.Provider)
		}
		if !strings.HasPrefix(e.Name, "repo-") {
			t.Errorf("unexpected name %s", e.Name)
		}
	}
}

func TestScanForReposLevel2(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "owner", "repo-x", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries := scanForRepos(tmp, "github")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Project != "owner" || entries[0].Name != "repo-x" {
		t.Errorf("expected owner/repo-x, got %s/%s", entries[0].Project, entries[0].Name)
	}
}

// --- calculateDestPath / reposPathEnvVar ---

func TestCalculateDestPathADO(t *testing.T) {
	loc := locator.New(map[string]string{"azuredevops": "/repos/ado"})
	got := calculateDestPath(loc, "azuredevops", "proj", "repo-x")
	want := "/repos/ado/repo-x"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCalculateDestPathGitHubWithProject(t *testing.T) {
	loc := locator.New(map[string]string{"github": "/repos/gh"})
	got := calculateDestPath(loc, "github", "owner", "repo-x")
	want := "/repos/gh/owner/repo-x"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCalculateDestPathNoConfig(t *testing.T) {
	loc := locator.New(map[string]string{})
	got := calculateDestPath(loc, "github", "owner", "repo-x")
	if got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestReposPathEnvVar(t *testing.T) {
	tests := map[string]string{
		"azuredevops": "AZURE_DEVOPS_REPOS_PATH",
		"github":      "GITHUB_REPOS_PATH",
		"gitlab":      "gitlab_REPOS_PATH",
	}
	for in, want := range tests {
		if got := reposPathEnvVar(in); got != want {
			t.Errorf("reposPathEnvVar(%q) = %s, want %s", in, got, want)
		}
	}
}
