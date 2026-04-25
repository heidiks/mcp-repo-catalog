package azuredevops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/heidiks/mcp-repo-catalog/internal/provider"
)

const apiVersion = "7.1"

type Client struct {
	httpClient *http.Client
	baseURL    string
	authHeader string
}

func NewClient(org, pat string) *Client {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + pat))
	return &Client{
		httpClient: &http.Client{},
		baseURL:    fmt.Sprintf("https://dev.azure.com/%s", org),
		authHeader: "Basic " + encoded,
	}
}

func NewClientWithHTTPClient(org, pat string, httpClient *http.Client) *Client {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + pat))
	return &Client{
		httpClient: httpClient,
		baseURL:    fmt.Sprintf("https://dev.azure.com/%s", org),
		authHeader: "Basic " + encoded,
	}
}

func (c *Client) Name() string {
	return "azuredevops"
}

func (c *Client) ListProjects(ctx context.Context, filter string) ([]provider.Project, error) {
	params := url.Values{"api-version": {apiVersion}}
	if filter != "" {
		params.Set("stateFilter", filter)
	}

	var all []adoProject
	continuationToken := ""

	for {
		if continuationToken != "" {
			params.Set("continuationToken", continuationToken)
		}

		endpoint := fmt.Sprintf("%s/_apis/projects?%s", c.baseURL, params.Encode())

		var resp listResponse[adoProject]
		token, err := c.doRequest(ctx, endpoint, &resp)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}

		all = append(all, resp.Value...)

		if token == "" {
			break
		}
		continuationToken = token
	}

	result := make([]provider.Project, len(all))
	for i, p := range all {
		result[i] = provider.Project{
			Name:        p.Name,
			Description: p.Description,
			Provider:    "azuredevops",
			UpdatedAt:   p.LastUpdateTime,
		}
	}

	return result, nil
}

func (c *Client) ListRepositories(ctx context.Context, project string) ([]provider.Repository, error) {
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=%s",
		c.baseURL, url.PathEscape(project), apiVersion)

	var resp listResponse[adoRepository]
	if _, err := c.doRequest(ctx, endpoint, &resp); err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	result := make([]provider.Repository, len(resp.Value))
	for i, r := range resp.Value {
		result[i] = convertRepo(r, project)
	}

	return result, nil
}

func (c *Client) GetRepository(ctx context.Context, project, repo string) (*provider.Repository, error) {
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s?api-version=%s",
		c.baseURL, url.PathEscape(project), url.PathEscape(repo), apiVersion)

	var r adoRepository
	if _, err := c.doRequest(ctx, endpoint, &r); err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	result := convertRepo(r, project)
	return &result, nil
}

func (c *Client) GetFileContent(ctx context.Context, project, repo, path string) (*provider.FileContent, error) {
	params := url.Values{
		"api-version":    {apiVersion},
		"path":           {path},
		"includeContent": {"true"},
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items?%s",
		c.baseURL, url.PathEscape(project), url.PathEscape(repo), params.Encode())

	var item adoItem
	if _, err := c.doRequest(ctx, endpoint, &item); err != nil {
		return nil, fmt.Errorf("get item %s: %w", path, err)
	}

	return &provider.FileContent{
		Path:     item.Path,
		Content:  item.Content,
		IsFolder: item.IsFolder,
	}, nil
}

func (c *Client) GetLastCommit(ctx context.Context, project, repo string) (*provider.Commit, error) {
	params := url.Values{
		"api-version": {apiVersion},
		"$top":        {"1"},
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/commits?%s",
		c.baseURL, url.PathEscape(project), url.PathEscape(repo), params.Encode())

	var resp listResponse[adoCommit]
	if _, err := c.doRequest(ctx, endpoint, &resp); err != nil {
		return nil, fmt.Errorf("get commits: %w", err)
	}

	if len(resp.Value) == 0 {
		return nil, nil
	}

	ac := resp.Value[0]
	return &provider.Commit{
		ID:      ac.CommitID,
		Message: ac.Comment,
		Author:  ac.Author.Name,
		Email:   ac.Author.Email,
		Date:    ac.Author.Date,
	}, nil
}

func convertRepo(r adoRepository, project string) provider.Repository {
	branch := strings.TrimPrefix(r.DefaultBranch, "refs/heads/")
	return provider.Repository{
		Name:          r.Name,
		DefaultBranch: branch,
		Size:          r.Size,
		WebURL:        r.WebURL,
		Project:       project,
		Provider:      "azuredevops",
	}
}

func (c *Client) doRequest(ctx context.Context, endpoint string, target any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", &provider.NotFoundError{Resource: endpoint}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return resp.Header.Get("x-ms-continuationtoken"), nil
}
