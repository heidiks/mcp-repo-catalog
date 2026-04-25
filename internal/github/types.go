package github

import "time"

type ghOrg struct {
	Login       string `json:"login"`
	Description string `json:"description"`
}

type ghRepository struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	DefaultBranch string    `json:"default_branch"`
	Size          int64     `json:"size"` // KB on GitHub
	HTMLURL       string    `json:"html_url"`
	Owner         ghOwner   `json:"owner"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ghOwner struct {
	Login string `json:"login"`
}

type ghCommit struct {
	SHA    string       `json:"sha"`
	Commit ghCommitData `json:"commit"`
}

type ghCommitData struct {
	Message string         `json:"message"`
	Author  ghCommitAuthor `json:"author"`
}

type ghCommitAuthor struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

type ghContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Type    string `json:"type"` // "file" or "dir"
	Content string `json:"content"`
}
