package locator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDirectMatch(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "my-repo"), 0o755)

	loc := New(map[string]string{"azuredevops": tmp})

	got := loc.Resolve("azuredevops", "some-project", "my-repo")
	expected := filepath.Join(tmp, "my-repo")

	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestResolveWithOwnerPrefix(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "acme", "go-monorepo"), 0o755)

	loc := New(map[string]string{"github": tmp})

	got := loc.Resolve("github", "acme", "go-monorepo")
	expected := filepath.Join(tmp, "acme", "go-monorepo")

	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestResolveNotFound(t *testing.T) {
	tmp := t.TempDir()

	loc := New(map[string]string{"github": tmp})

	got := loc.Resolve("github", "org", "nonexistent")
	if got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}

func TestResolveNoPathConfigured(t *testing.T) {
	loc := New(map[string]string{})

	got := loc.Resolve("github", "org", "repo")
	if got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}

func TestResolveUnknownProvider(t *testing.T) {
	loc := New(map[string]string{"github": "/tmp"})

	got := loc.Resolve("gitlab", "org", "repo")
	if got != "" {
		t.Fatalf("expected empty for unknown provider, got %s", got)
	}
}

func TestHasPath(t *testing.T) {
	loc := New(map[string]string{
		"github":      "/some/path",
		"azuredevops": "",
	})

	if !loc.HasPath("github") {
		t.Fatal("expected HasPath=true for github")
	}

	if loc.HasPath("azuredevops") {
		t.Fatal("expected HasPath=false for empty azuredevops path")
	}

	if loc.HasPath("gitlab") {
		t.Fatal("expected HasPath=false for unconfigured provider")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	got := expandHome("~/repos")
	expected := filepath.Join(home, "repos")

	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}

	got2 := expandHome("/absolute/path")
	if got2 != "/absolute/path" {
		t.Fatalf("expected /absolute/path, got %s", got2)
	}
}

func TestDirectMatchTakesPriority(t *testing.T) {
	tmp := t.TempDir()
	// Both direct and owner-prefixed exist
	_ = os.MkdirAll(filepath.Join(tmp, "repo"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "org", "repo"), 0o755)

	loc := New(map[string]string{"github": tmp})

	got := loc.Resolve("github", "org", "repo")
	expected := filepath.Join(tmp, "repo")

	if got != expected {
		t.Fatalf("direct match should take priority, expected %s, got %s", expected, got)
	}
}
