package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ReadFromRepoInput struct {
	Repo string `json:"repo" jsonschema:"required,Repository name (or partial match)"`
	Path string `json:"path" jsonschema:"required,File path within the repository (e.g. internal/handlers/auth.go)"`
}

func RegisterReadFromRepo(server *mcp.Server, store *catalog.Store, registry *provider.Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_from_repo",
		Description: "Read a file from any cataloged repository. Uses the local clone when available; falls back to the provider API otherwise.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ReadFromRepoInput) (*mcp.CallToolResult, any, error) {
		// Find repo in catalog
		matches := store.Search(input.Repo)
		if len(matches) == 0 {
			return nil, nil, fmt.Errorf("repository %q not found in catalog. Run sync_catalog or sync_local first", input.Repo)
		}

		// Prefer exact name match
		var target *catalog.RepoEntry
		for i := range matches {
			if strings.EqualFold(matches[i].Name, input.Repo) {
				target = &matches[i]
				break
			}
		}

		if target == nil {
			if len(matches) == 1 {
				target = &matches[0]
			} else {
				var sb strings.Builder
				fmt.Fprintf(&sb, "Multiple repositories match %q. Be more specific:\n\n", input.Repo)
				for _, m := range matches {
					cloned := ""
					if m.LocalPath != "" {
						cloned = " (cloned)"
					}
					fmt.Fprintf(&sb, "- %s [%s]%s\n", m.Name, m.Provider, cloned)
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
			}
		}

		var content string
		var source string

		if target.LocalPath != "" {
			filePath := filepath.Join(target.LocalPath, strings.TrimPrefix(input.Path, "/"))
			data, err := os.ReadFile(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, nil, fmt.Errorf("file not found: %s in %s", input.Path, target.Name)
				}
				return nil, nil, fmt.Errorf("read file: %w", err)
			}
			content = string(data)
			source = "local"
		} else {
			if registry == nil {
				return nil, nil, fmt.Errorf("repository %q is not cloned locally. Use clone_repository to clone it first", target.Name)
			}
			p := registry.GetByName(target.Provider)
			if p == nil {
				return nil, nil, fmt.Errorf("provider %q not registered, cannot fall back to API", target.Provider)
			}
			fc, err := p.GetFileContent(ctx, target.Project, target.Name, input.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("read file via %s API: %w", target.Provider, err)
			}
			if fc.IsFolder {
				return nil, nil, fmt.Errorf("path %q is a folder, not a file", input.Path)
			}
			content = fc.Content
			source = "api"
		}

		header := fmt.Sprintf("## %s:%s [%s/%s] (source: %s)\n\n", target.Name, input.Path, target.Provider, target.Project, source)
		text := header + content

		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
