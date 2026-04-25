package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SyncRemoteInput struct{}

func RegisterSyncRemote(server *mcp.Server, store *catalog.Store, loc *locator.RepoLocator, remoteRepo, remotePath, adoPAT, ghToken string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sync_remote",
		Description: "Pull the central catalog from the configured remote git repository and merge with local paths.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SyncRemoteInput) (*mcp.CallToolResult, any, error) {
		if remoteRepo == "" {
			return nil, nil, fmt.Errorf("CATALOG_REMOTE_REPO is not configured")
		}

		cacheDir, err := catalogCacheDir()
		if err != nil {
			return nil, nil, err
		}

		repoDir := filepath.Join(cacheDir, "remote-catalog")
		authURL := injectAuth(remoteRepo, adoPAT, ghToken)

		// Clone or pull
		if isGitRepoDir(repoDir) {
			cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "pull", "--ff-only")
			output, err := cmd.CombinedOutput()
			if err != nil {
				return nil, nil, fmt.Errorf("git pull failed: %w\n%s", err, string(output))
			}
		} else {
			if err := os.RemoveAll(repoDir); err != nil {
				return nil, nil, fmt.Errorf("remove existing repo dir: %w", err)
			}
			cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", authURL, repoDir)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return nil, nil, fmt.Errorf("git clone failed: %w\n%s", err, string(output))
			}
		}

		// Find catalog directory
		var catalogDir string
		if remotePath != "" {
			catalogDir = filepath.Join(repoDir, remotePath)
		} else {
			// Default: try catalog/, then root
			catalogDir = filepath.Join(repoDir, "catalog")
			if _, err := os.Stat(catalogDir); os.IsNotExist(err) {
				catalogDir = repoDir
			}
		}

		// Parse markdown files
		remoteEntries, err := catalog.ParseMarkdownCatalog(catalogDir)
		if err != nil {
			return nil, nil, fmt.Errorf("parse remote catalog: %w", err)
		}

		// Merge: remote metadata wins, local paths preserved
		localEntries := store.GetAll()
		localPathMap := make(map[string]string)
		for _, e := range localEntries {
			key := e.Provider + "/" + e.Name
			if e.LocalPath != "" {
				localPathMap[key] = e.LocalPath
			}
		}

		for i := range remoteEntries {
			key := remoteEntries[i].Provider + "/" + remoteEntries[i].Name
			if lp, ok := localPathMap[key]; ok {
				remoteEntries[i].LocalPath = lp
			}
		}

		// Auto-scan local directories to fill local_path
		if loc != nil {
			providerDirs := map[string]string{
				"azuredevops": loc.BasePath("azuredevops"),
				"github":      loc.BasePath("github"),
			}

			entryMap := make(map[string]int) // key -> index
			for i, e := range remoteEntries {
				entryMap[e.Provider+"/"+e.Name] = i
			}

			for providerName, basePath := range providerDirs {
				if basePath == "" {
					continue
				}
				localRepos := scanForRepos(basePath, providerName)
				for _, found := range localRepos {
					// Match by provider+name
					if idx, ok := entryMap[found.Provider+"/"+found.Name]; ok {
						if remoteEntries[idx].LocalPath == "" {
							remoteEntries[idx].LocalPath = found.LocalPath
						}
					}
				}
			}
		}

		store.SetRepositories(remoteEntries)
		if err := store.Save(); err != nil {
			return nil, nil, fmt.Errorf("save catalog: %w", err)
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Remote catalog synced: %d repositories\n", len(remoteEntries))

		localCount := 0
		for _, e := range remoteEntries {
			if e.LocalPath != "" {
				localCount++
			}
		}
		fmt.Fprintf(&sb, "Cloned locally: %d/%d\n", localCount, len(remoteEntries))

		text := sb.String()
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

func catalogCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "mcp-repo-catalog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

func isGitRepoDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

// injectAuth adds authentication to a git clone URL based on the provider detected from the URL.
// dev.azure.com -> uses ADO PAT
// github.com -> uses GitHub token
func injectAuth(repoURL, adoPAT, ghToken string) string {
	if strings.Contains(repoURL, "dev.azure.com") && adoPAT != "" {
		// ADO: https://PAT@dev.azure.com/org/project/_git/repo
		return strings.Replace(repoURL, "https://", "https://"+adoPAT+"@", 1)
	}
	if strings.Contains(repoURL, "github.com") && ghToken != "" {
		// GitHub: https://TOKEN@github.com/owner/repo
		return strings.Replace(repoURL, "https://", "https://"+ghToken+"@", 1)
	}
	return repoURL
}
