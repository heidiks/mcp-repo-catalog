package locator

import (
	"os"
	"path/filepath"
	"strings"
)

type RepoLocator struct {
	paths map[string]string // provider name -> base path
}

func New(providerPaths map[string]string) *RepoLocator {
	expanded := make(map[string]string, len(providerPaths))
	for k, v := range providerPaths {
		if v != "" {
			expanded[k] = expandHome(v)
		}
	}
	return &RepoLocator{paths: expanded}
}

// Resolve returns the local filesystem path for a repo if it exists.
// For a given provider, it checks:
//  1. <base_path>/<repo>/
//  2. <base_path>/<project>/<repo>/ (for GitHub owner/repo pattern)
func (l *RepoLocator) Resolve(providerName, project, repo string) string {
	base, ok := l.paths[providerName]
	if !ok || base == "" {
		return ""
	}

	// Direct: <base>/<repo>/
	direct := filepath.Join(base, repo)
	if isDir(direct) {
		return direct
	}

	// With project/owner prefix: <base>/<project>/<repo>/
	if project != "" {
		withProject := filepath.Join(base, project, repo)
		if isDir(withProject) {
			return withProject
		}
	}

	return ""
}

func (l *RepoLocator) HasPath(providerName string) bool {
	p, ok := l.paths[providerName]
	return ok && p != ""
}

func (l *RepoLocator) BasePath(providerName string) string {
	return l.paths[providerName]
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
