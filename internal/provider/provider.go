package provider

import "context"

type Provider interface {
	Name() string
	ListProjects(ctx context.Context, filter string) ([]Project, error)
	ListRepositories(ctx context.Context, project string) ([]Repository, error)
	GetRepository(ctx context.Context, project, repo string) (*Repository, error)
	GetFileContent(ctx context.Context, project, repo, path string) (*FileContent, error)
	GetLastCommit(ctx context.Context, project, repo string) (*Commit, error)
}
