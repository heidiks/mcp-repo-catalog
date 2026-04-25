package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heidiks/mcp-repo-catalog/internal/cache"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const listProjectsCacheTTL = 10 * time.Minute

type ListProjectsInput struct {
	State string `json:"state,omitempty" jsonschema:"Filter by project state. Applies to providers that support it (e.g. Azure DevOps)"`
}

func RegisterListProjects(server *mcp.Server, registry *provider.Registry, c *cache.Cache) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_projects",
		Description: "List all projects/organizations across all configured providers (Azure DevOps, GitHub)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, any, error) {
		cacheKey := fmt.Sprintf("list_projects:%s", input.State)
		if cached, ok := c.Get(cacheKey); ok {
			text := cached.(string)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		}

		var allProjects []provider.Project
		var errs []string

		for _, p := range registry.AllProviders() {
			projects, err := p.ListProjects(ctx, input.State)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
				continue
			}
			allProjects = append(allProjects, projects...)
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d projects:\n\n", len(allProjects))

		currentProvider := ""
		idx := 1
		for _, p := range allProjects {
			if p.Provider != currentProvider {
				currentProvider = p.Provider
				fmt.Fprintf(&sb, "### %s\n", currentProvider)
			}
			desc := p.Description
			if desc == "" {
				desc = "(no description)"
			}
			updated := ""
			if !p.UpdatedAt.IsZero() {
				updated = fmt.Sprintf(" (updated: %s)", p.UpdatedAt.Format("2006-01-02"))
			}
			fmt.Fprintf(&sb, "%d. **%s** - %s%s\n", idx, p.Name, desc, updated)
			idx++
		}

		if len(errs) > 0 {
			fmt.Fprintf(&sb, "\nWarnings: %s\n", strings.Join(errs, "; "))
		}

		text := sb.String()
		c.Set(cacheKey, text, listProjectsCacheTTL)

		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
