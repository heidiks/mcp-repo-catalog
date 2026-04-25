package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ViewCatalogInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"Filter by name, domain, or provider. Empty shows all."`
}

func RegisterViewCatalog(server *mcp.Server, store *catalog.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "view_catalog",
		Description: "View the local repository catalog. Shows all indexed repos grouped by provider with domain, languages, and local path. Optionally filter by name, domain, or provider.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ViewCatalogInput) (*mcp.CallToolResult, any, error) {
		var entries []catalog.RepoEntry

		if input.Filter != "" {
			entries = store.Search(input.Filter)
		} else {
			entries = store.GetAll()
		}

		if len(entries) == 0 {
			msg := "Catalog is empty. Run `sync_catalog` to populate from APIs or `sync_local` to scan local repos."
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: msg}}}, nil, nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Catalog: %d repositories", len(entries))
		if input.Filter != "" {
			fmt.Fprintf(&sb, " (filter: %q)", input.Filter)
		}
		sb.WriteString("\n\n")

		// Group by provider
		grouped := make(map[string][]catalog.RepoEntry)
		var providerOrder []string
		for _, e := range entries {
			if _, exists := grouped[e.Provider]; !exists {
				providerOrder = append(providerOrder, e.Provider)
			}
			grouped[e.Provider] = append(grouped[e.Provider], e)
		}

		for _, prov := range providerOrder {
			repos := grouped[prov]
			fmt.Fprintf(&sb, "### %s (%d repos)\n\n", prov, len(repos))

			for _, e := range repos {
				// Line 1: name + project
				if e.Project != "" {
					fmt.Fprintf(&sb, "- **%s** (%s)", e.Name, e.Project)
				} else {
					fmt.Fprintf(&sb, "- **%s**", e.Name)
				}

				// Inline tags
				var tags []string
				if e.Domain != "" {
					tags = append(tags, e.Domain)
				}
				if len(e.Languages) > 0 {
					tags = append(tags, strings.Join(e.Languages, ", "))
				}
				if len(e.Frameworks) > 0 {
					tags = append(tags, strings.Join(e.Frameworks, ", "))
				}
				if e.IsMonorepo {
					tags = append(tags, "monorepo")
				}
				if len(tags) > 0 {
					fmt.Fprintf(&sb, " [%s]", strings.Join(tags, " | "))
				}

				// Local indicator
				if e.LocalPath != "" {
					sb.WriteString(" *")
				}

				sb.WriteString("\n")

				// Line 2: description if present
				if e.Description != "" {
					fmt.Fprintf(&sb, "  %s\n", e.Description)
				}
			}

			sb.WriteString("\n")
		}

		sb.WriteString("---\n* = cloned locally\n")

		text := sb.String()
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
