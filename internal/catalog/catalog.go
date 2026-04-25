package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultDir = ".config/mcp-repo-catalog"
const fileName = "catalog.json"

type RepoEntry struct {
	Provider       string    `json:"provider"`
	Project        string    `json:"project"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	CloneURL       string    `json:"clone_url"`
	Description    string    `json:"description"`
	Languages      []string  `json:"languages"`
	Frameworks     []string  `json:"frameworks,omitempty"`
	Domain         string    `json:"domain"`
	IntegratesWith []string  `json:"integrates_with,omitempty"`
	IsMonorepo     bool      `json:"is_monorepo,omitempty"`
	LocalPath      string    `json:"local_path"`
	LastSynced     time.Time `json:"last_synced"`
}

type Catalog struct {
	LastSynced   time.Time   `json:"last_synced"`
	Repositories []RepoEntry `json:"repositories"`
}

type Store struct {
	mu       sync.RWMutex
	filePath string
	data     *Catalog
}

func NewStore(filePath string) *Store {
	if filePath == "" {
		home, _ := os.UserHomeDir()
		filePath = filepath.Join(home, defaultDir, fileName)
	}
	return &Store{
		filePath: filePath,
		data:     &Catalog{},
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = &Catalog{}
			return nil
		}
		return fmt.Errorf("read catalog: %w", err)
	}

	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("parse catalog: %w", err)
	}

	s.data = &c
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create catalog dir: %w", err)
	}

	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal catalog: %w", err)
	}

	return os.WriteFile(s.filePath, raw, 0o644)
}

func (s *Store) SetRepositories(repos []RepoEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Repositories = repos
	s.data.LastSynced = time.Now()
}

func (s *Store) GetAll() []RepoEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.Repositories
}

func (s *Store) Search(query string) []RepoEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []RepoEntry
	for _, r := range s.data.Repositories {
		if containsIgnoreCase(r.Name, query) ||
			containsIgnoreCase(r.Description, query) ||
			containsIgnoreCase(r.Domain, query) {
			matches = append(matches, r)
		}
	}
	return matches
}

func (s *Store) UpdateField(provider, project, repo, field, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Repositories {
		r := &s.data.Repositories[i]
		if r.Provider == provider && r.Project == project && r.Name == repo {
			switch field {
			case "domain":
				r.Domain = value
			case "description":
				r.Description = value
			case "local_path":
				r.LocalPath = value
			case "languages":
				r.Languages = splitAndTrim(value)
			case "frameworks":
				r.Frameworks = splitAndTrim(value)
			case "is_monorepo":
				r.IsMonorepo = parseBoolValue(value)
			}
			return
		}
	}
}

func (s *Store) UpdateLocalPath(provider, project, repo, localPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Repositories {
		r := &s.data.Repositories[i]
		if r.Provider == provider && r.Project == project && r.Name == repo {
			r.LocalPath = localPath
			return
		}
	}
}

func (s *Store) FindByName(provider, project, repo string) *RepoEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.data.Repositories {
		r := &s.data.Repositories[i]
		if r.Provider == provider && r.Project == project && r.Name == repo {
			return r
		}
	}
	return nil
}

func (s *Store) IsStale(maxAge time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.data.Repositories) == 0 {
		return true
	}
	return time.Since(s.data.LastSynced) > maxAge
}

func (s *Store) FilePath() string {
	return s.filePath
}
