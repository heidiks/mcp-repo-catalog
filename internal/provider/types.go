package provider

import "time"

type Project struct {
	Name        string
	Description string
	Provider    string
	UpdatedAt   time.Time
}

type Repository struct {
	Name          string
	Description   string
	DefaultBranch string
	Size          int64
	WebURL        string
	Project       string
	Provider      string
}

type Commit struct {
	ID      string
	Message string
	Author  string
	Email   string
	Date    time.Time
}

type FileContent struct {
	Path     string
	Content  string
	IsFolder bool
}
