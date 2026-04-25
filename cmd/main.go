package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/azuredevops"
	"github.com/heidiks/mcp-repo-catalog/internal/cache"
	"github.com/heidiks/mcp-repo-catalog/internal/catalog"
	gh "github.com/heidiks/mcp-repo-catalog/internal/github"
	"github.com/heidiks/mcp-repo-catalog/internal/locator"
	"github.com/heidiks/mcp-repo-catalog/internal/provider"
	"github.com/heidiks/mcp-repo-catalog/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(version)
			return
		}
	}

	disabled := parseList(os.Getenv("DISABLED_PROVIDERS"))
	var providers []provider.Provider
	repoPaths := make(map[string]string)

	// Azure DevOps provider
	if !isDisabled(disabled, "azuredevops") {
		org := os.Getenv("AZURE_DEVOPS_ORG")
		pat := os.Getenv("AZURE_DEVOPS_TOKEN")
		if org != "" && pat != "" {
			providers = append(providers, azuredevops.NewClient(org, pat))
			fmt.Fprintf(os.Stderr, "Provider registered: azuredevops (org: %s)\n", org)
		}
		if p := os.Getenv("AZURE_DEVOPS_REPOS_PATH"); p != "" {
			repoPaths["azuredevops"] = p
		}
	}

	// GitHub provider
	if !isDisabled(disabled, "github") {
		token := os.Getenv("GITHUB_TOKEN")
		if token != "" {
			orgs := parseList(os.Getenv("GITHUB_ORGS"))
			user := os.Getenv("GITHUB_USER")
			providers = append(providers, gh.NewClient(token, orgs, user))
			fmt.Fprintf(os.Stderr, "Provider registered: github\n")
		}
		if p := os.Getenv("GITHUB_REPOS_PATH"); p != "" {
			repoPaths["github"] = p
		}
	}

	if len(providers) == 0 {
		fmt.Fprintln(os.Stderr, "No providers configured. Set AZURE_DEVOPS_TOKEN and/or GITHUB_TOKEN.")
		os.Exit(1)
	}

	remoteRepo := os.Getenv("CATALOG_REMOTE_REPO")
	remotePath := os.Getenv("CATALOG_REMOTE_PATH")
	isRemoteMode := remoteRepo != ""

	registry := provider.NewRegistry(providers...)
	c := cache.New()
	loc := locator.New(repoPaths)
	store := catalog.NewStore("")

	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load catalog: %v\n", err)
	}

	if isRemoteMode {
		fmt.Fprintf(os.Stderr, "Remote mode: catalog managed by %s\n", remoteRepo)
	}

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "mcp-repo-catalog",
			Version: version,
		},
		nil,
	)

	// Always available
	tools.RegisterListProjects(server, registry, c)
	tools.RegisterSearchRepos(server, registry, c, loc)
	tools.RegisterRepoDetails(server, registry, c, loc)
	tools.RegisterViewCatalog(server, store)
	tools.RegisterReadFromRepo(server, store, registry)
	tools.RegisterCloneRepo(server, registry, store, loc)

	if isRemoteMode {
		// Remote mode: sync from central repo, no local edits
		adoToken := os.Getenv("AZURE_DEVOPS_TOKEN")
		ghToken := os.Getenv("GITHUB_TOKEN")
		tools.RegisterSyncRemote(server, store, loc, remoteRepo, remotePath, adoToken, ghToken)
	} else {
		// Local mode: sync from APIs, allow edits
		tools.RegisterSyncCatalog(server, registry, store, loc)
		tools.RegisterUpdateCatalog(server, store)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func isDisabled(disabled []string, name string) bool {
	for _, d := range disabled {
		if strings.EqualFold(d, name) {
			return true
		}
	}
	return false
}
