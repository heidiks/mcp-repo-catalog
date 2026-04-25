package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ReadFromRepoInput struct {
	Repo string `json:"repo" jsonschema:"required,Repository name (or partial match)"`
	Path string `json:"path" jsonschema:"required,File path within the repository (e.g. internal/handlers/auth.go)"`
}

func RegisterReadFromRepo(server *mcp.Server, store *catalog.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_from_repo",
		Description: "Read a file from any cataloged repository using its local clone. Useful for cross-repo context when working on integrations.",
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

		if target.LocalPath == "" {
			return nil, nil, fmt.Errorf("repository %q is not cloned locally. Use clone_repository to clone it first", target.Name)
		}

		filePath := filepath.Join(target.LocalPath, strings.TrimPrefix(input.Path, "/"))

		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("file not found: %s in %s", input.Path, target.Name)
			}
			return nil, nil, fmt.Errorf("read file: %w", err)
		}

		header := fmt.Sprintf("## %s:%s [%s/%s]\n\n", target.Name, input.Path, target.Provider, target.Project)
		text := header + string(content)

		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
