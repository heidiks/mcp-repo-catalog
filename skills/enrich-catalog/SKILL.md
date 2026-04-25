---
name: enrich-catalog
description: Use to turn dry repository catalog entries (`view_catalog` output) into rich documentation by reading manifest files, README/CLAUDE.md, detecting frameworks and monorepo signals, and updating the catalog metadata. Trigger when the user asks to enrich the catalog, fill in missing domains/frameworks/languages, or "document the repos".
---

# Enrich Catalog

You enrich the repository catalog exposed by the `repo-catalog` MCP server. Your job is to turn dry `.md` entries into rich documentation that serves as context for future LLM work.

## Setup

1. Call the `view_catalog` tool to see all indexed repos.
2. Identify repos with generic descriptions (e.g. "xxx repository.") or empty fields (`domain`, `languages`, `frameworks`, `is_monorepo`, `integrates_with`).
3. Prioritize repos that are cloned locally (marked with `*` in `view_catalog` output).

## Enrichment loop

For each repo that needs enrichment:

### 1. Collect context

Use `read_from_repo` to read these files (skip if the file doesn't exist). The tool falls back to the provider API when the repo isn't cloned locally.

**Documentation (try these first)**

- `README.md` — overview, setup, architecture
- `CLAUDE.md` — AI instructions, conventions, stack hints

**Dependency manifests (language map)**

| File | Language | What to extract |
|---|---|---|
| `go.mod` | Go | Go version, module name, framework (gin, echo, fiber, chi, grpc) |
| `go.work` | Go | monorepo signal (workspaces) |
| `package.json` | JS/TS | `dependencies` (next, react, vue, nest, express, fastify), `workspaces` (monorepo) |
| `pnpm-workspace.yaml` / `lerna.json` / `nx.json` / `turbo.json` | JS/TS | monorepo signal |
| `pom.xml` | Java | Spring Boot, Quarkus, `<modules>` (Maven multi-module = monorepo) |
| `build.gradle` / `settings.gradle` | Java/Kotlin | Spring Boot, Gradle modules (monorepo) |
| `requirements.txt` / `pyproject.toml` / `Pipfile` | Python | FastAPI, Django, Flask, Celery |
| `Cargo.toml` | Rust | `[workspace]` = monorepo |
| `*.csproj` / `*.sln` | C#/.NET | ASP.NET Core, multiple projects in `.sln` |
| `Gemfile` | Ruby | Rails, Sinatra |
| `composer.json` | PHP | Laravel, Symfony |
| `mix.exs` | Elixir | Phoenix |
| `pubspec.yaml` | Dart/Flutter | Flutter |

**Config and build (optional, skim if available)**

- `internal/config/config.go` or `application.yaml` or `.env.example` — env vars
- `Makefile` or `Dockerfile` or `docker-compose.yml` — build, deploy

### 2. Detect monorepo

Mark `is_monorepo: true` when ANY of the following is true:

- `go.work` exists at the repo root
- `package.json` has a `workspaces` field
- `pnpm-workspace.yaml`, `lerna.json`, `nx.json`, or `turbo.json` exists
- `pom.xml` declares `<modules>` with more than one module
- `settings.gradle` includes multiple `include` directives
- `Cargo.toml` has a `[workspace]` section
- `.sln` references multiple `.csproj` files
- The README explicitly describes the repo as a monorepo or lists subprojects as independently deployable

When marking as monorepo, list each subproject in the body of the catalog entry under a `## Modules` section (one line per module: language + responsibility).

### 3. Extract frameworks (not transitive deps)

Capture ONLY frameworks/runtimes that define the repo's stack. Do NOT list auxiliary libraries.

- Yes: `Spring Boot`, `Quarkus`, `Gin`, `Echo`, `NestJS`, `Next.js`, `FastAPI`, `Django`, `Express`, `React`, `Vue`, `Flutter`, `Rails`, `Phoenix`, `ASP.NET Core`
- No: logging libraries, ORMs, validators, generic util libs

Set `frameworks: [...]` in the frontmatter with the detected names.

### 4. Update the catalog entry

Use `update_catalog_entry` to set the fields you collected. One call per field:

- `update_catalog_entry repo=<name> field=domain value=<domain>`
- `update_catalog_entry repo=<name> field=languages value="Go, TypeScript"`
- `update_catalog_entry repo=<name> field=frameworks value="Gin, gRPC"`
- `update_catalog_entry repo=<name> field=is_monorepo value=true`
- `update_catalog_entry repo=<name> field=description value="<one-or-two-sentence summary>"`

If the catalog is in **remote mode** (synced from a markdown repo), update the upstream `.md` files directly instead — the next `sync_remote` call will pull your changes back. The fields above map directly to YAML frontmatter keys.

### 5. Rules

- Never invent information — only use what you found in the files.
- If the repo is unreachable (not cloned and API call fails), only fill what can be inferred from the existing frontmatter. Skip and continue.
- Preserve existing frontmatter fields (`url`, `clone_url`, `project`) — only add or update.
- Sections without enough info: omit them rather than leaving template placeholders.
- Match the project's existing language and conventions (check the catalog's existing entries for tone and style before writing new ones).
- No emoji unless the project's existing entries use them.

### 6. Cadence

- Enrich 5 repos per round.
- After each repo, report which one you enriched, frameworks detected, and whether it's a monorepo.
- At the end of each round, ask whether to continue.

## Starting

Begin by saying: "Let me scan the catalog for repos that need enrichment."

Then list the first 5 candidates and start with the one that has the most context available (cloned locally + has README).

## Customizing for your organization

This skill is generic. To tune it for your org:

- Add an org-specific domain table to a `CLAUDE.md` at the catalog root, then have the skill reference it (replace step 4's domain value with a lookup against that table).
- Adjust the frameworks list in step 3 to include your in-house frameworks.
- Override the language used in descriptions if your team writes docs in something other than English.
