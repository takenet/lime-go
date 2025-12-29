.PHONY: help build run test fmt docs clean vet lint coverage install-tools

# Variables
BINARY_NAME=lime-go
GO=go
GOFLAGS=-v
GOTEST=$(GO) test
GOVET=$(GO) vet
GOFMT=gofmt
GODOC=$(GO) doc
MODULE=github.com/takenet/lime-go

# Default target
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the project
	@echo "Building..."
	$(GO) build $(GOFLAGS) ./...

run: ## Run the example client
	@echo "Running example client..."
	$(GO) run examples/client/main.go

run-server: ## Run the example server
	@echo "Running example server..."
	$(GO) run examples/server/main.go examples/server/certificate.go examples/server/credentials.go

run-ws-chat: ## Run the websocket chat server
	@echo "Running websocket chat server..."
	$(GO) run examples/ws-chat/server/main.go

test: ## Run all tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) $(GOFLAGS) -race -coverprofile=coverage.txt -covermode=atomic ./...

test-no-race: ## Run all tests without race detector
	@echo "Running tests without race detector..."
	$(GOTEST) $(GOFLAGS) -coverprofile=coverage.txt -covermode=atomic ./...

test-short: ## Run tests in short mode
	@echo "Running short tests..."
	$(GOTEST) $(GOFLAGS) -short ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests with verbose output..."
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage: test ## Run tests and show coverage
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Code formatted successfully"

fmt-check: ## Check if code is formatted
	@echo "Checking code format..."
	@test -z "$$($(GOFMT) -l .)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run 'make install-tools'" && exit 1)
	golangci-lint run ./...

docs: ## Generate and display documentation
	@echo "Generating documentation..."
	@echo "Main package:"
	$(GODOC) $(MODULE)
	@echo "\nFor full documentation, run: godoc -http=:6060"
	@echo "Then visit: http://localhost:6060/pkg/$(MODULE)/"

docs-serve: ## Serve documentation on http://localhost:6060
	@echo "Starting documentation server..."
	@which godoc > /dev/null || (echo "godoc not installed. Run 'make install-tools'" && exit 1)
	godoc -http=:6060

clean: ## Clean build artifacts and test cache
	@echo "Cleaning..."
	$(GO) clean
	rm -f coverage.txt coverage.html
	@echo "Clean complete"

clean-cache: ## Clean Go build and module cache (fixes version mismatch issues)
	@echo "Cleaning Go cache..."
	$(GO) clean -cache -modcache -i -r
	@echo "Cache cleaned. Run 'make deps' to re-download dependencies."

tidy: ## Tidy and verify module dependencies
	@echo "Tidying module dependencies..."
	$(GO) mod tidy
	$(GO) mod verify

install-tools: ## Install development tools
	@echo "Installing development tools..."
	$(GO) install golang.org/x/tools/cmd/godoc@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed successfully"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download

check: fmt-check vet test ## Run all checks (format, vet, test)

ci: check lint ## Run all CI checks

all: clean deps build test ## Clean, download deps, build and test

.DEFAULT_GOAL := help
