package tools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
)

// scanForRepos scans a base directory for git repos (1-2 levels deep).
func scanForRepos(basePath, providerName string) []catalog.RepoEntry {
	var entries []catalog.RepoEntry

	// Level 1: <basePath>/<repo>/.git
	level1, _ := os.ReadDir(basePath)
	for _, d := range level1 {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}

		repoPath := filepath.Join(basePath, d.Name())
		if isGitRepo(repoPath) {
			entry := catalog.RepoEntry{
				Provider:  providerName,
				Name:      d.Name(),
				LocalPath: repoPath,
				Domain:    catalog.InferDomain(d.Name(), "", ""),
			}
			entries = append(entries, entry)
			continue
		}

		// Level 2: <basePath>/<owner>/<repo>/.git (GitHub pattern)
		level2, _ := os.ReadDir(repoPath)
		for _, d2 := range level2 {
			if !d2.IsDir() || strings.HasPrefix(d2.Name(), ".") {
				continue
			}

			subRepoPath := filepath.Join(repoPath, d2.Name())
			if isGitRepo(subRepoPath) {
				entry := catalog.RepoEntry{
					Provider:  providerName,
					Project:   d.Name(),
					Name:      d2.Name(),
					LocalPath: subRepoPath,
					Domain:    catalog.InferDomain(d2.Name(), "", ""),
				}
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}
