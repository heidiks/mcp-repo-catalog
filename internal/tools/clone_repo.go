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
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CloneRepoInput struct {
	Project string `json:"project" jsonschema:"required,Project name (ADO) or owner (GitHub)"`
	Repo    string `json:"repo" jsonschema:"required,Repository name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"Set to true to execute the clone. First call without confirm to see details."`
}

func RegisterCloneRepo(server *mcp.Server, registry *provider.Registry, store *catalog.Store, loc *locator.RepoLocator) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "clone_repository",
		Description: "Clone a repository to the configured local directory. Call first without confirm=true to see details, then with confirm=true to execute.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input CloneRepoInput) (*mcp.CallToolResult, any, error) {
		// Find repo info
		var repo *provider.Repository
		var providerName string

		for _, p := range registry.AllProviders() {
			r, err := p.GetRepository(ctx, input.Project, input.Repo)
			if err != nil {
				continue
			}
			repo = r
			providerName = p.Name()
			break
		}

		if repo == nil {
			// Try from catalog
			entry := store.FindByName("", input.Project, input.Repo)
			if entry == nil {
				return nil, nil, fmt.Errorf("repository %s/%s not found in any provider", input.Project, input.Repo)
			}
			repo = &provider.Repository{
				Name:    entry.Name,
				WebURL:  entry.URL,
				Project: entry.Project,
			}
			providerName = entry.Provider
		}

		// Check if already cloned
		if loc != nil {
			existing := loc.Resolve(providerName, repo.Project, repo.Name)
			if existing != "" {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Repository already cloned at: %s", existing),
				}}}, nil, nil
			}
		}

		// Calculate destination
		destPath := calculateDestPath(loc, providerName, repo.Project, repo.Name)
		if destPath == "" {
			return nil, nil, fmt.Errorf("no repos path configured for provider %s. Set %s env var",
				providerName, reposPathEnvVar(providerName))
		}

		cloneURL := repo.WebURL

		if !input.Confirm {
			var sb strings.Builder
			sb.WriteString("Clone details:\n\n")
			fmt.Fprintf(&sb, "- **Repository:** %s/%s\n", repo.Project, repo.Name)
			fmt.Fprintf(&sb, "- **Provider:** %s\n", providerName)
			fmt.Fprintf(&sb, "- **Clone URL:** %s\n", cloneURL)
			fmt.Fprintf(&sb, "- **Destination:** %s\n", destPath)
			sb.WriteString("\nCall again with `confirm: true` to execute the clone.")

			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
		}

		// Execute clone
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return nil, nil, fmt.Errorf("create parent directory: %w", err)
		}

		cmd := exec.CommandContext(ctx, "git", "clone", cloneURL, destPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, nil, fmt.Errorf("git clone failed: %w\n%s", err, string(output))
		}

		// Update catalog
		store.UpdateLocalPath(providerName, repo.Project, repo.Name, destPath)
		if err := store.Save(); err != nil {
			return nil, nil, fmt.Errorf("save catalog: %w", err)
		}

		text := fmt.Sprintf("Cloned successfully to: %s\n\n%s", destPath, string(output))
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

func calculateDestPath(loc *locator.RepoLocator, providerName, project, repo string) string {
	if loc == nil || !loc.HasPath(providerName) {
		return ""
	}

	basePath := loc.BasePath(providerName)
	if basePath == "" {
		return ""
	}

	if providerName == "github" && project != "" {
		return filepath.Join(basePath, project, repo)
	}

	return filepath.Join(basePath, repo)
}

func reposPathEnvVar(providerName string) string {
	switch providerName {
	case "azuredevops":
		return "AZURE_DEVOPS_REPOS_PATH"
	case "github":
		return "GITHUB_REPOS_PATH"
	default:
		return providerName + "_REPOS_PATH"
	}
}
