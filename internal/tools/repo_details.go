package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/heidiks/mcp-repo-catalog/internal/cache"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const repoDetailsCacheTTL = 5 * time.Minute

type RepoDetailsInput struct {
	Project string `json:"project" jsonschema:"required,Project name (ADO) or owner/org (GitHub)"`
	Repo    string `json:"repo" jsonschema:"required,Repository name"`
}

func RegisterRepoDetails(server *mcp.Server, registry *provider.Registry, c *cache.Cache, loc *locator.RepoLocator) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_repo_details",
		Description: "Get detailed metadata for a specific repository including README and last commit. Searches across all providers.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input RepoDetailsInput) (*mcp.CallToolResult, any, error) {
		cacheKey := fmt.Sprintf("repo_details:%s:%s", input.Project, input.Repo)
		if cached, ok := c.Get(cacheKey); ok {
			text := cached.(string)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		}

		// Try each provider until one succeeds
		for _, p := range registry.AllProviders() {
			text, err := fetchRepoDetails(ctx, p, loc, input.Project, input.Repo)
			if err != nil {
				continue
			}

			c.Set(cacheKey, text, repoDetailsCacheTTL)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		}

		return nil, nil, fmt.Errorf("repository %s/%s not found in any provider", input.Project, input.Repo)
	})
}

func fetchRepoDetails(ctx context.Context, p provider.Provider, loc *locator.RepoLocator, project, repoName string) (string, error) {
	var (
		repo    *provider.Repository
		readme  string
		commit  *provider.Commit
		repoErr error
		readErr error
		comErr  error
		wg      sync.WaitGroup
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		repo, repoErr = p.GetRepository(ctx, project, repoName)
	}()

	go func() {
		defer wg.Done()
		fc, err := p.GetFileContent(ctx, project, repoName, "/README.md")
		if err != nil {
			readErr = err
			return
		}
		readme = fc.Content
	}()

	go func() {
		defer wg.Done()
		commit, comErr = p.GetLastCommit(ctx, project, repoName)
	}()

	wg.Wait()

	if repoErr != nil {
		return "", repoErr
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s [%s]\n\n", repo.Name, repo.Provider)
	fmt.Fprintf(&sb, "- **Project/Owner:** %s\n", repo.Project)
	fmt.Fprintf(&sb, "- **Default branch:** %s\n", repo.DefaultBranch)
	fmt.Fprintf(&sb, "- **Size:** %s\n", formatSize(repo.Size))
	fmt.Fprintf(&sb, "- **URL:** %s\n", repo.WebURL)

	if loc != nil {
		localPath := loc.Resolve(repo.Provider, repo.Project, repo.Name)
		if localPath != "" {
			fmt.Fprintf(&sb, "- **Local path:** %s\n", localPath)
		} else if loc.HasPath(repo.Provider) {
			sb.WriteString("- **Local path:** not cloned locally\n")
		}
	}

	if commit != nil && comErr == nil {
		sb.WriteString("\n### Last commit\n")
		fmt.Fprintf(&sb, "- **Author:** %s (%s)\n", commit.Author, commit.Email)
		fmt.Fprintf(&sb, "- **Date:** %s\n", commit.Date.Format("2006-01-02 15:04"))
		fmt.Fprintf(&sb, "- **Message:** %s\n", commit.Message)
	}

	if readme != "" && readErr == nil {
		sb.WriteString("\n### README (preview)\n\n")
		if len(readme) > 500 {
			readme = readme[:500] + "..."
		}
		sb.WriteString(readme)
		sb.WriteString("\n")
	} else {
		var notFound *provider.NotFoundError
		if errors.As(readErr, &notFound) || readErr == nil {
			sb.WriteString("\n*No README.md found.*\n")
		}
	}

	return sb.String(), nil
}
