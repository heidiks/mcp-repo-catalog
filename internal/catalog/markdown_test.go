package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMarkdownFile(t *testing.T) {
	tmp := t.TempDir()
	content := `---
name: go-monorepo
provider: azuredevops
project: Acme
domain: backend
languages: [Go]
integrates_with: [auth-gatekeeper, roz-storage]
url: https://dev.azure.com/acme/Acme/_git/go-monorepo
---

Monorepo principal com 15+ microservices Go.

## Key services
- auth-gatekeeper: IAM
- roz-storage: file upload
`
	path := filepath.Join(tmp, "go-monorepo.md")
	_ = os.WriteFile(path, []byte(content), 0o644)

	entry, err := parseMarkdownFile(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if entry.Name != "go-monorepo" {
		t.Errorf("expected go-monorepo, got %s", entry.Name)
	}
	if entry.Provider != "azuredevops" {
		t.Errorf("expected azuredevops, got %s", entry.Provider)
	}
	if entry.Domain != "backend" {
		t.Errorf("expected backend, got %s", entry.Domain)
	}
	if len(entry.Languages) != 1 || entry.Languages[0] != "Go" {
		t.Errorf("expected [Go], got %v", entry.Languages)
	}
	if len(entry.IntegratesWith) != 2 {
		t.Errorf("expected 2 integrations, got %d", len(entry.IntegratesWith))
	}
	if entry.Description == "" {
		t.Error("expected body as description")
	}
	if entry.URL != "https://dev.azure.com/acme/Acme/_git/go-monorepo" {
		t.Errorf("expected URL, got %s", entry.URL)
	}
}

func TestParseMarkdownFileNameFromFilename(t *testing.T) {
	tmp := t.TempDir()
	content := `---
provider: github
project: acme
domain: fiscal
---

Some repo without name in frontmatter.
`
	path := filepath.Join(tmp, "nfe-backend.md")
	_ = os.WriteFile(path, []byte(content), 0o644)

	entry, err := parseMarkdownFile(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if entry.Name != "nfe-backend" {
		t.Errorf("expected nfe-backend from filename, got %s", entry.Name)
	}
}

func TestParseMarkdownCatalogDir(t *testing.T) {
	tmp := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmp, "repo-a.md"), []byte(`---
name: repo-a
provider: github
---

Description A.
`), 0o644)

	_ = os.WriteFile(filepath.Join(tmp, "repo-b.md"), []byte(`---
name: repo-b
provider: azuredevops
project: Acme
---

Description B.
`), 0o644)

	// Non-md file should be ignored
	_ = os.WriteFile(filepath.Join(tmp, "README.txt"), []byte("ignore me"), 0o644)

	entries, err := ParseMarkdownCatalog(tmp)
	if err != nil {
		t.Fatalf("parse dir failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestSplitFrontmatterNoFrontmatter(t *testing.T) {
	_, _, err := splitFrontmatter("just plain text")
	if err == nil {
		t.Fatal("expected error for no frontmatter")
	}
}

func TestSplitFrontmatterUnclosed(t *testing.T) {
	_, _, err := splitFrontmatter("---\nname: test\nno closing")
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}
