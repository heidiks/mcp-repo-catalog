package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var allowedFields = map[string]bool{
	"domain":      true,
	"description": true,
	"local_path":  true,
	"languages":   true,
}

type UpdateCatalogInput struct {
	Repo  string `json:"repo" jsonschema:"required,Repository name (or partial match)"`
	Field string `json:"field" jsonschema:"required,Field to update: domain, description, local_path, or languages"`
	Value string `json:"value" jsonschema:"required,New value for the field. For languages use comma-separated values"`
}

func RegisterUpdateCatalog(server *mcp.Server, store *catalog.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_catalog_entry",
		Description: "Update a field in the local catalog for a repository. Editable fields: domain, description, local_path, languages.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input UpdateCatalogInput) (*mcp.CallToolResult, any, error) {
		field := strings.ToLower(input.Field)
		if !allowedFields[field] {
			keys := make([]string, 0, len(allowedFields))
			for k := range allowedFields {
				keys = append(keys, k)
			}
			return nil, nil, fmt.Errorf("invalid field %q. Allowed: %s", input.Field, strings.Join(keys, ", "))
		}

		// Find matching repos
		matches := store.Search(input.Repo)
		if len(matches) == 0 {
			return nil, nil, fmt.Errorf("no repository matching %q found in catalog. Run sync_catalog first", input.Repo)
		}

		// If multiple matches, try exact name match
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
					fmt.Fprintf(&sb, "- %s [%s] (%s)\n", m.Name, m.Provider, m.Project)
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
			}
		}

		// Apply update
		switch field {
		case "domain":
			store.UpdateField(target.Provider, target.Project, target.Name, "domain", input.Value)
		case "description":
			store.UpdateField(target.Provider, target.Project, target.Name, "description", input.Value)
		case "local_path":
			store.UpdateField(target.Provider, target.Project, target.Name, "local_path", input.Value)
		case "languages":
			store.UpdateField(target.Provider, target.Project, target.Name, "languages", input.Value)
		}

		if err := store.Save(); err != nil {
			return nil, nil, fmt.Errorf("save catalog: %w", err)
		}

		text := fmt.Sprintf("Updated **%s** [%s/%s]: %s = %q", target.Name, target.Provider, target.Project, field, input.Value)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
