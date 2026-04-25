package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heidiks/mcp-repo-catalog/internal/cache"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const reposCacheTTL = 5 * time.Minute

type SearchReposInput struct {
	Query   string `json:"query" jsonschema:"required,Search term to match against repository name"`
	Project string `json:"project,omitempty" jsonschema:"Project/org name. If empty, searches all projects across all providers"`
}

func RegisterSearchRepos(server *mcp.Server, registry *provider.Registry, c *cache.Cache, loc *locator.RepoLocator) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_repositories",
		Description: "Search repositories by name across all configured providers (Azure DevOps, GitHub)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchReposInput) (*mcp.CallToolResult, any, error) {
		query := strings.ToLower(input.Query)

		var allRepos []provider.Repository

		for _, p := range registry.AllProviders() {
			if input.Project != "" {
				repos, err := getCachedRepos(ctx, p, c, input.Project)
				if err != nil {
					continue
				}
				allRepos = append(allRepos, repos...)
			} else {
				projects, err := p.ListProjects(ctx, "")
				if err != nil {
					continue
				}
				for _, proj := range projects {
					repos, err := getCachedRepos(ctx, p, c, proj.Name)
					if err != nil {
						continue
					}
					allRepos = append(allRepos, repos...)
				}
			}
		}

		var matches []provider.Repository
		for _, r := range allRepos {
			if strings.Contains(strings.ToLower(r.Name), query) {
				matches = append(matches, r)
			}
		}

		// Fuzzy fallback when no substring match
		if len(matches) == 0 && len(query) >= 3 {
			threshold := fuzzyThreshold(len(query))
			for _, r := range allRepos {
				name := strings.ToLower(r.Name)
				if levenshtein(name, query) <= threshold {
					matches = append(matches, r)
					continue
				}
				// also tolerate fuzzy match against any token of the name (split by - or _)
				for _, token := range splitNameTokens(name) {
					if levenshtein(token, query) <= threshold {
						matches = append(matches, r)
						break
					}
				}
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d repositories matching '%s':\n\n", len(matches), input.Query)

		for i, r := range matches {
			fmt.Fprintf(&sb, "%d. **%s** [%s] (project: %s, branch: %s, size: %s)\n   %s\n",
				i+1, r.Name, r.Provider, r.Project, r.DefaultBranch, formatSize(r.Size), r.WebURL)
			if loc != nil {
				localPath := loc.Resolve(r.Provider, r.Project, r.Name)
				if localPath != "" {
					fmt.Fprintf(&sb, "   Local: %s\n", localPath)
				}
			}
		}

		if len(matches) == 0 {
			sb.WriteString("No repositories found matching the query.")
		}

		text := sb.String()
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

func getCachedRepos(ctx context.Context, p provider.Provider, c *cache.Cache, project string) ([]provider.Repository, error) {
	cacheKey := fmt.Sprintf("repos:%s:%s", p.Name(), project)
	if cached, ok := c.Get(cacheKey); ok {
		return cached.([]provider.Repository), nil
	}

	repos, err := p.ListRepositories(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("list repos for %s/%s: %w", p.Name(), project, err)
	}

	c.Set(cacheKey, repos, reposCacheTTL)
	return repos, nil
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
	)

	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
