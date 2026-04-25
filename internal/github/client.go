package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/heidiks/mcp-repo-catalog/internal/provider"
)

const baseURL = "https://api.github.com"

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	orgs       []string // explicit orgs to list
	user       string   // explicit user for personal repos
}

func NewClient(token string, orgs []string, user string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    baseURL,
		token:      token,
		orgs:       orgs,
		user:       user,
	}
}

func NewClientWithHTTPClient(token string, orgs []string, user string, httpClient *http.Client, overrideBaseURL string) *Client {
	base := baseURL
	if overrideBaseURL != "" {
		base = overrideBaseURL
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    base,
		token:      token,
		orgs:       orgs,
		user:       user,
	}
}

func (c *Client) Name() string {
	return "github"
}

func (c *Client) ListProjects(ctx context.Context, filter string) ([]provider.Project, error) {
	var projects []provider.Project

	// If explicit orgs configured, list those
	if len(c.orgs) > 0 {
		for _, org := range c.orgs {
			projects = append(projects, provider.Project{
				Name:        org,
				Description: fmt.Sprintf("GitHub organization: %s", org),
				Provider:    "github",
				UpdatedAt:   time.Now(),
			})
		}
	} else {
		// Auto-discover orgs from authenticated user
		var orgs []ghOrg
		if err := c.doRequest(ctx, fmt.Sprintf("%s/user/orgs", c.baseURL), &orgs); err != nil {
			return nil, fmt.Errorf("list orgs: %w", err)
		}

		for _, o := range orgs {
			projects = append(projects, provider.Project{
				Name:        o.Login,
				Description: o.Description,
				Provider:    "github",
			})
		}
	}

	// Add personal user if configured
	if c.user != "" {
		projects = append(projects, provider.Project{
			Name:        c.user,
			Description: "Personal GitHub repositories",
			Provider:    "github",
		})
	}

	return projects, nil
}

func (c *Client) ListRepositories(ctx context.Context, owner string) ([]provider.Repository, error) {
	// Try as org first, fall back to user
	var repos []ghRepository
	endpoint := fmt.Sprintf("%s/orgs/%s/repos?per_page=100", c.baseURL, owner)

	err := c.doRequest(ctx, endpoint, &repos)
	if err != nil {
		// Try as user
		endpoint = fmt.Sprintf("%s/users/%s/repos?per_page=100", c.baseURL, owner)
		if err := c.doRequest(ctx, endpoint, &repos); err != nil {
			return nil, fmt.Errorf("list repositories for %s: %w", owner, err)
		}
	}

	result := make([]provider.Repository, len(repos))
	for i, r := range repos {
		result[i] = convertRepo(r)
	}

	return result, nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	var r ghRepository
	if err := c.doRequest(ctx, endpoint, &r); err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	result := convertRepo(r)
	return &result, nil
}

func (c *Client) GetFileContent(ctx context.Context, owner, repo, path string) (*provider.FileContent, error) {
	path = strings.TrimPrefix(path, "/")
	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)

	var content ghContent
	if err := c.doRequest(ctx, endpoint, &content); err != nil {
		return nil, fmt.Errorf("get file %s: %w", path, err)
	}

	if content.Type == "dir" {
		return &provider.FileContent{
			Path:     path,
			IsFolder: true,
		}, nil
	}

	// GitHub returns content as base64
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode base64 content: %w", err)
	}

	return &provider.FileContent{
		Path:    path,
		Content: string(decoded),
	}, nil
}

func (c *Client) GetLastCommit(ctx context.Context, owner, repo string) (*provider.Commit, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=1", c.baseURL, owner, repo)

	var commits []ghCommit
	if err := c.doRequest(ctx, endpoint, &commits); err != nil {
		return nil, fmt.Errorf("get commits: %w", err)
	}

	if len(commits) == 0 {
		return nil, nil
	}

	gc := commits[0]
	return &provider.Commit{
		ID:      gc.SHA,
		Message: gc.Commit.Message,
		Author:  gc.Commit.Author.Name,
		Email:   gc.Commit.Author.Email,
		Date:    gc.Commit.Author.Date,
	}, nil
}

func convertRepo(r ghRepository) provider.Repository {
	return provider.Repository{
		Name:          r.Name,
		Description:   r.Description,
		DefaultBranch: r.DefaultBranch,
		Size:          r.Size * 1024, // GitHub reports in KB, normalize to bytes
		WebURL:        r.HTMLURL,
		Project:       r.Owner.Login,
		Provider:      "github",
	}
}

func (c *Client) doRequest(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return &provider.NotFoundError{Resource: endpoint}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
