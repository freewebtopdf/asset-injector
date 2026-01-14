# Asset Injector Microservice Makefile
.PHONY: help build run test test-race lint clean docker-build docker-run docs deps fmt vet

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=asset-injector
DOCKER_IMAGE=asset-injector:latest
GO_VERSION=1.25
MAIN_PATH=./cmd/server
BUILD_DIR=./build

# Build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME) -w -s"

help: ## Display this help message
	@echo "Asset Injector Microservice - Development Commands"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

deps: ## Install dependencies
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: build ## Build and run the application
	@echo "Starting $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	go test -race -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run golangci-lint
	@echo "Running linter..."
	golangci-lint run --config .golangci.yml

fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean -cache
	go clean -testcache

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) GO_DOCKER_TAG=${GO_DOCKER_TAG}..."
	@docker build --build-arg HTTP_PROXY=${http_proxy} --build-arg GO_DOCKER_TAG="${GO_DOCKER_TAG}" -t $(DOCKER_IMAGE) .
	@echo "Docker image built."

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 \
		-e LOG_LEVEL=debug \
		-v $(PWD)/data:/data \
		$(DOCKER_IMAGE)

docker-clean: ## Clean Docker images and containers
	@echo "Cleaning Docker artifacts..."
	docker rmi $(DOCKER_IMAGE) 2>/dev/null || true
	docker system prune -f

docs: ## Generate API documentation
	@echo "Generating API documentation..."
	swag init -g $(MAIN_PATH)/main.go -o ./docs
	@echo "API documentation generated in ./docs"

docs-serve: docs ## Generate and serve API documentation
	@echo "Serving API documentation on http://localhost:8081"
	@cd docs && python3 -m http.server 8081 2>/dev/null || python -m SimpleHTTPServer 8081

dev: ## Start development environment
	@echo "Starting development environment..."
	@make deps
	@make lint
	@make test-race
	@make build
	@make run

ci: ## Run CI pipeline locally
	@echo "Running CI pipeline..."
	@make deps
	@make fmt
	@make vet
	@make lint
	@make test-race
	@make build

install: build ## Install binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin"
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

profile: ## Run with CPU profiling
	@echo "Running with CPU profiling..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME)-profile $(MAIN_PATH)
	./$(BUILD_DIR)/$(BINARY_NAME)-profile -cpuprofile=cpu.prof
	go tool pprof cpu.prof

# Development helpers
watch: ## Watch for changes and rebuild
	@echo "Watching for changes..."
	@which fswatch > /dev/null || (echo "fswatch not installed. Install with: brew install fswatch" && exit 1)
	fswatch -o . -e ".*" -i "\\.go$$" | xargs -n1 -I{} make build

.PHONY: deps build run test test-race test-coverage lint fmt vet clean docker-build docker-run docker-clean docs docs-serve dev ci install benchmark profile watch
