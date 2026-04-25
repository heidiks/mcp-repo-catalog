package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SyncCatalogInput struct{}

func RegisterSyncCatalog(server *mcp.Server, registry *provider.Registry, store *catalog.Store, loc *locator.RepoLocator) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sync_catalog",
		Description: "Sync the local repository catalog from all configured providers. Updates metadata, languages, domains, and local paths.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SyncCatalogInput) (*mcp.CallToolResult, any, error) {
		var entries []catalog.RepoEntry
		var errs []string

		for _, p := range registry.AllProviders() {
			projects, err := p.ListProjects(ctx, "")
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: list projects: %v", p.Name(), err))
				continue
			}

			for _, proj := range projects {
				repos, err := p.ListRepositories(ctx, proj.Name)
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s/%s: list repos: %v", p.Name(), proj.Name, err))
					continue
				}

				for _, r := range repos {
					// Try to read README for domain inference
					readme := ""
					fc, err := p.GetFileContent(ctx, r.Project, r.Name, "/README.md")
					if err == nil {
						readme = fc.Content
						if len(readme) > 1000 {
							readme = readme[:1000]
						}
					}

					domain := catalog.InferDomain(r.Name, r.Description, readme)
					localPath := ""
					if loc != nil {
						localPath = loc.Resolve(r.Provider, r.Project, r.Name)
					}

					entries = append(entries, catalog.RepoEntry{
						Provider:    r.Provider,
						Project:     r.Project,
						Name:        r.Name,
						URL:         r.WebURL,
						CloneURL:    r.WebURL, // TODO: proper clone URL
						Description: r.Description,
						Domain:      domain,
						LocalPath:   localPath,
					})
				}
			}
		}

		store.SetRepositories(entries)
		if err := store.Save(); err != nil {
			return nil, nil, fmt.Errorf("save catalog: %w", err)
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Catalog synced: %d repositories indexed.\n", len(entries))
		fmt.Fprintf(&sb, "Saved to: %s\n", store.FilePath())

		// Stats
		domains := make(map[string]int)
		providers := make(map[string]int)
		localCount := 0
		for _, e := range entries {
			providers[e.Provider]++
			if e.Domain != "" {
				domains[e.Domain]++
			}
			if e.LocalPath != "" {
				localCount++
			}
		}

		sb.WriteString("\nBy provider:\n")
		for p, count := range providers {
			fmt.Fprintf(&sb, "  - %s: %d repos\n", p, count)
		}

		fmt.Fprintf(&sb, "\nCloned locally: %d/%d\n", localCount, len(entries))

		if len(domains) > 0 {
			sb.WriteString("\nBy domain:\n")
			for d, count := range domains {
				fmt.Fprintf(&sb, "  - %s: %d repos\n", d, count)
			}
		}

		if len(errs) > 0 {
			fmt.Fprintf(&sb, "\nWarnings: %s\n", strings.Join(errs, "; "))
		}

		text := sb.String()
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
