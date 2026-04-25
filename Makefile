APP_NAME := mcp-repo-catalog
BIN_DIR := bin

.PHONY: all build test lint fmt clean setup

all: fmt build lint test

build:
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/

test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

clean:
	rm -rf $(BIN_DIR) coverage.out

setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example"; \
	else \
		echo ".env already exists (not overwritten)"; \
	fi
	@echo ""
	@echo "Edit .env with your credentials:"
	@echo "  - AZURE_DEVOPS_ORG and AZURE_DEVOPS_TOKEN for Azure DevOps"
	@echo "  - GITHUB_TOKEN for GitHub"
	@echo "  - AZURE_DEVOPS_REPOS_PATH / GITHUB_REPOS_PATH for local repo mapping"
	@echo ""
	@if [ -n "$$EDITOR" ]; then \
		$$EDITOR .env; \
	elif command -v code >/dev/null 2>&1; then \
		code .env; \
	else \
		echo "Open .env in your editor to configure."; \
	fi
