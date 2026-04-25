package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type markdownFrontmatter struct {
	Name           string   `yaml:"name"`
	Provider       string   `yaml:"provider"`
	Project        string   `yaml:"project"`
	URL            string   `yaml:"url"`
	CloneURL       string   `yaml:"clone_url"`
	Domain         string   `yaml:"domain"`
	Languages      []string `yaml:"languages"`
	IntegratesWith []string `yaml:"integrates_with"`
}

// ParseMarkdownCatalog reads a directory of .md files with YAML frontmatter
// and returns catalog entries. The markdown body is stored in Description.
func ParseMarkdownCatalog(dir string) ([]RepoEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir: %w", err)
	}

	var entries []RepoEntry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		entry, err := parseMarkdownFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}

		entries = append(entries, *entry)
	}

	return entries, nil
}

func parseMarkdownFile(path string) (*RepoEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(raw)

	// Split frontmatter from body
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var meta markdownFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}

	if meta.Name == "" {
		// Use filename without extension as name
		meta.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	return &RepoEntry{
		Provider:       meta.Provider,
		Project:        meta.Project,
		Name:           meta.Name,
		URL:            meta.URL,
		CloneURL:       meta.CloneURL,
		Description:    strings.TrimSpace(body),
		Languages:      meta.Languages,
		Domain:         meta.Domain,
		IntegratesWith: meta.IntegratesWith,
	}, nil
}

func splitFrontmatter(content string) (frontmatter, body string, err error) {
	if !strings.HasPrefix(content, "---") {
		return "", content, fmt.Errorf("no frontmatter found")
	}

	// Find closing ---
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content, fmt.Errorf("unclosed frontmatter")
	}

	frontmatter = strings.TrimSpace(rest[:idx])
	body = strings.TrimSpace(rest[idx+4:])
	return frontmatter, body, nil
}
