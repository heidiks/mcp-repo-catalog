# mcp-repo-catalog

MCP server for discovering and exploring repositories across multiple git platforms. Supports **Azure DevOps** and **GitHub** simultaneously, with results aggregated from all configured providers.

Complements the [official Microsoft Azure DevOps MCP](https://github.com/microsoft/azure-devops-mcp) (which handles operations like PRs, work items, pipelines) by focusing on **discovery, local repo mapping, and architectural context**.

## Tools

Always available:

| Tool | Description |
|------|-------------|
| `list_projects` | List all projects/organizations across configured providers |
| `search_repositories` | Search repositories by name, with local clone path when available |
| `get_repo_details` | Detailed metadata for a repo (README, last commit, size) |
| `view_catalog` | Show the local catalog grouped by provider |
| `read_from_repo` | Read a file from any cloned repo (cross-repo lookup) |
| `clone_repository` | Clone a repo into the configured local directory |

Mode-dependent:

| Tool | Mode | Description |
|------|------|-------------|
| `sync_remote` | Remote | Pull a central catalog repo and map local paths |
| `sync_catalog` | Local | Sync the catalog by querying provider APIs |
| `update_catalog_entry` | Local | Edit a catalog entry locally |

The mode is determined by the `CATALOG_REMOTE_REPO` env var: when set, the server runs in **remote** mode (read-only catalog from a shared repo); otherwise it runs in **local** mode.

## Installation

```bash
go install github.com/heidiks/mcp-repo-catalog/cmd@latest
```

Or build from source:

```bash
git clone https://github.com/heidiks/mcp-repo-catalog
cd mcp-repo-catalog
make build   # binary at bin/mcp-repo-catalog
```

## Configuration

### Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `AZURE_DEVOPS_ORG` | No* | Azure DevOps organization name |
| `AZURE_DEVOPS_TOKEN` | No* | Azure DevOps Personal Access Token |
| `AZURE_DEVOPS_REPOS_PATH` | No | Local path where ADO repos are cloned |
| `GITHUB_TOKEN` | No* | GitHub Personal Access Token |
| `GITHUB_ORGS` | No | Comma-separated list of GitHub orgs to list |
| `GITHUB_USER` | No | GitHub username for personal repos |
| `GITHUB_REPOS_PATH` | No | Local path where GitHub repos are cloned |
| `DISABLED_PROVIDERS` | No | Comma-separated providers to disable (`github`, `azuredevops`) |
| `CATALOG_REMOTE_REPO` | No | URL of a central catalog repo (enables remote mode) |
| `CATALOG_REMOTE_PATH` | No | Path within the remote repo where `.md` files live (default: `catalog/`) |

*At least one provider must be configured (Azure DevOps and/or GitHub).

### PAT permissions

- **Azure DevOps**: Code (Read), Project and Team (Read)
- **GitHub**: `repo` scope (or `public_repo` for public repos only)

### Claude Code

```bash
claude mcp add repo-catalog \
  --transport stdio \
  --scope user \
  --env AZURE_DEVOPS_ORG=your-org \
  --env 'AZURE_DEVOPS_TOKEN=${AZURE_DEVOPS_TOKEN}' \
  --env 'AZURE_DEVOPS_REPOS_PATH=~/path/to/azure/repos' \
  --env 'GITHUB_TOKEN=${GITHUB_TOKEN}' \
  --env 'GITHUB_REPOS_PATH=~/path/to/github/repos' \
  -- \
  /path/to/mcp-repo-catalog
```

Tokens (`AZURE_DEVOPS_TOKEN`, `GITHUB_TOKEN`) must be exported in your shell (e.g. `~/.zshrc`).

## Usage examples

- "List all projects in the organization"
- "Search for repositories with 'auth' in the name"
- "Get details of the go-monorepo in project backend"
- "Read the CLAUDE.md from the payments-api repo"
- "Clone the user-service repo locally"

## Supported providers

| Provider | Status | Config |
|----------|--------|--------|
| Azure DevOps | Supported | `AZURE_DEVOPS_ORG` + `AZURE_DEVOPS_TOKEN` |
| GitHub | Supported | `GITHUB_TOKEN` |
| GitLab | Planned | - |

## Development

```bash
make setup    # Create .env from .env.example
make build    # Build binary
make test     # Run tests with coverage
make lint     # Run linter
make fmt      # Format code
```

## License

MIT License. See [LICENSE](LICENSE).
