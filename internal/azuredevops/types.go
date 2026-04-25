package azuredevops

import "time"

type listResponse[T any] struct {
	Count             int    `json:"count"`
	Value             []T    `json:"value"`
	ContinuationToken string `json:"continuationToken,omitempty"`
}

type adoProject struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	State          string    `json:"state"`
	LastUpdateTime time.Time `json:"lastUpdateTime"`
}

type adoRepository struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	DefaultBranch string     `json:"defaultBranch"`
	Size          int64      `json:"size"`
	RemoteURL     string     `json:"remoteUrl"`
	WebURL        string     `json:"webUrl"`
	Project       adoProject `json:"project"`
}

type adoCommit struct {
	CommitID string          `json:"commitId"`
	Comment  string          `json:"comment"`
	Author   adoCommitAuthor `json:"author"`
}

type adoCommitAuthor struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

type adoItem struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	IsFolder bool   `json:"isFolder"`
}
