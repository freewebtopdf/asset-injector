# Asset Injector Microservice

[![CI](https://github.com/freewebtopdf/asset-injector/workflows/CI/badge.svg)](https://github.com/freewebtopdf/asset-injector/actions)
[![CD](https://github.com/freewebtopdf/asset-injector/workflows/CD/badge.svg)](https://github.com/freewebtopdf/asset-injector/actions)
[![codecov](https://codecov.io/gh/freewebtopdf/asset-injector/branch/main/graph/badge.svg)](https://codecov.io/gh/freewebtopdf/asset-injector)
[![Go Report Card](https://goreportcard.com/badge/github.com/freewebtopdf/asset-injector)](https://goreportcard.com/report/github.com/freewebtopdf/asset-injector)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance Go microservice for URL pattern matching and CSS/JS asset injection in Web-to-PDF pipelines. Match URLs against rules and inject custom styles/scripts to modify page rendering before PDF conversion.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Architecture Overview](#architecture-overview)
- [How It Works](#how-it-works)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Project Structure](#project-structure)
- [Development](#development)
- [Testing](#testing)
- [Deployment](#deployment)
- [Contributing](#contributing)
- [License](#license)

## Features

| Feature | Description |
|---------|-------------|
| 🎯 **URL Pattern Matching** | Exact, wildcard (`*`, `?`), and regex patterns with priority scoring |
| ⚡ **Sub-millisecond Response** | LRU cache with 10K+ entry capacity for instant lookups |
| 📦 **Community Packs** | Install and share rule collections from GitHub |
| 🔄 **Singles Sync** | Auto-sync individual contributed rules from community repo |
| ⚖️ **Conflict Resolution** | Automatic priority: local > override > community |
| 💾 **Crash-Safe Persistence** | Atomic file operations with YAML-based rule storage |
| 🔒 **Security** | Rate limiting, input validation, CORS, security headers |
| 📊 **Observability** | Health checks, metrics, structured JSON logging (zerolog) |
| ☸️ **Kubernetes Ready** | Kustomize overlays, HPA, health probes, NetworkPolicy |

## Quick Start

### Prerequisites

- **Go 1.25+** (required)
- Docker & Docker Compose (optional)

### Option 1: Run Locally

```bash
# Clone the repository
git clone https://github.com/freewebtopdf/asset-injector.git
cd asset-injector

# Copy environment config
cp .env.example .env

# Run directly
go run cmd/server/main.go

# Or build and run
go build -o bin/server cmd/server/main.go
./bin/server
```

### Option 2: Run with Docker

```bash
docker-compose -f deploy/docker-compose.yml up
```

Server starts at `http://localhost:8080`

### Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# View API documentation
open http://localhost:8080/swagger/index.html
```

### Try It Out

```bash
# 1. Create a rule to hide cookie banners on example.com
curl -X POST http://localhost:8080/v1/rules \
  -H "Content-Type: application/json" \
  -d '{
    "type": "wildcard",
    "pattern": "https://example.com/*",
    "css": ".cookie-banner { display: none !important; }",
    "js": "document.querySelector('.popup')?.remove();",
    "description": "Hide cookie banner on example.com"
  }'

# 2. Resolve a URL to get matching CSS/JS
curl -X POST http://localhost:8080/v1/resolve \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/products/123"}'

# Response:
# {
#   "status": "success",
#   "data": {
#     "rule_id": "abc123...",
#     "css": ".cookie-banner { display: none !important; }",
#     "js": "document.querySelector('.popup')?.remove();",
#     "cache_hit": false
#   }
# }
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Layer (Fiber)                       │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐    │
│  │ Resolve │ │  Rules  │ │  Packs  │ │ Health  │ │ Metrics │    │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘    │
└───────┼───────────┼───────────┼───────────┼───────────┼─────────┘
        │           │           │           │           │
        ▼           ▼           ▼           ▼           ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Domain Layer                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐     │
│  │ Matcher  │  │  Store   │  │  Pack    │  │   Conflict   │     │
│  │ (Pattern │  │ (Rules   │  │ Manager  │  │   Resolver   │     │
│  │ Matching)│  │  CRUD)   │  │          │  │              │     │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────────────┘     │
│       │             │             │                             │
│       ▼             ▼             ▼                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                       │
│  │LRU Cache │  │  Loader  │  │  GitHub  │                       │
│  │ (10K)    │  │ (YAML)   │  │  Client  │                       │
│  └──────────┘  └──────────┘  └──────────┘                       │
└─────────────────────────────────────────────────────────────────┘
        │             │             │
        ▼             ▼             ▼
   [Memory]     [File System]   [GitHub API]
                rules/local/    freewebtopdf/
                rules/community/ asset-injector-community-rules
```

### Tech Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Language | Go | 1.25+ |
| Web Framework | Fiber | v2.52.10 |
| Configuration | caarlos0/env | v10.0.0 |
| Validation | go-playground/validator | v10.30.1 |
| Logging | zerolog | v1.34.0 |
| API Docs | swaggo/swag | v1.16.6 |
| Testing | testify + gopter | v1.11.1 / v0.2.11 |

## How It Works

### Pattern Matching

When you call `/v1/resolve` with a URL, the matcher evaluates all rules and returns the best match:

```
URL: https://example.com/products/123

Rules evaluated (highest score wins):
  1. exact:    "https://example.com/products/123"     → Score: 1000+
  2. regex:    "^https://example\\.com/products/\\d+$" → Score: 500+
  3. wildcard: "https://example.com/*"                → Score: 100+
```

#### Scoring Algorithm

```
score = base_score + min(pattern_length, 499)
```

| Type | Base Score | Range | Always Beats |
|------|------------|-------|--------------|
| exact | 1000 | 1000-1499 | regex, wildcard |
| regex | 500 | 500-999 | wildcard |
| wildcard | 100 | 100-599 | - |

Override with explicit `priority` field (0-10000).

#### Wildcard Syntax

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `https://example.com/*` | `/page`, `/a/b/c` | Different domain |
| `https://*.example.com/*` | `sub.example.com/x` | `example.com/x` |
| `https://example.com/user?` | `/user1`, `/userA` | `/user`, `/user12` |

### Rule Sources & Conflict Resolution

Rules are loaded from three directories with different priorities:

```
rules/
├── local/           # Priority 3 (highest) - Your custom rules
│   └── my-rule.rule.yaml
├── overrides/       # Priority 2 - Modified community rules
│   └── cookie-banners/
│       └── override.rule.yaml
└── community/       # Priority 1 (lowest) - Installed packs
    └── cookie-banners/
        ├── manifest.yaml
        └── rules/
            └── banner.rule.yaml
```

**Conflict Resolution**: When multiple rules have the same ID:

1. Highest priority source wins (local > override > community)
2. Ties broken by most recent `updated_at` timestamp

### Rule File Format

```yaml
# rules/local/example.rule.yaml
id: "550e8400-e29b-41d4-a716-446655440000"  # UUID (auto-generated if omitted)
type: "wildcard"                             # exact | regex | wildcard
pattern: "https://example.com/*"
css: |
  .cookie-banner { display: none !important; }
  .modal-overlay { opacity: 0; }
js: |
  document.querySelector('.popup')?.remove();
priority: 1500                               # Optional: override scoring
description: "Hide cookie banners"           # Optional
author: "your-name"                          # Optional
tags:                                        # Optional
  - cookies
  - privacy
```

### Caching

- **LRU Cache**: 10,000 entries by default (configurable)
- **Invalidation**: Cache cleared automatically on any rule change
- **Response**: `cache_hit: true` indicates cached result

## API Reference

### Core Endpoints

#### POST /v1/resolve

Match a URL and return CSS/JS assets.

```bash
curl -X POST http://localhost:8080/v1/resolve \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/page"}'
```

**Response:**

```json
{
  "status": "success",
  "data": {
    "rule_id": "550e8400-e29b-41d4-a716-446655440000",
    "css": ".banner { display: none; }",
    "js": "console.log('injected');",
    "cache_hit": false
  }
}
```

### Rules Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/rules` | List all rules |
| `POST` | `/v1/rules` | Create rule (ID auto-generated) |
| `PUT` | `/v1/rules/:id` | Update rule |
| `DELETE` | `/v1/rules/:id` | Delete rule |
| `GET` | `/v1/rules/:id/source` | Get rule origin info |
| `POST` | `/v1/rules/export` | Export rules as pack |

### Pack Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/packs` | List installed packs |
| `POST` | `/v1/packs/install` | Install pack (`{"source": "pack-name@1.0.0"}`) |
| `DELETE` | `/v1/packs/:name` | Uninstall pack |
| `GET` | `/v1/packs/available` | Browse community packs |
| `POST` | `/v1/packs/update` | Update packs (`{"all": true}` or `{"names": [...]}`) |

### System Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check (liveness + readiness) |
| `GET` | `/metrics` | Cache stats, rule counts |
| `GET` | `/swagger/*` | Interactive API documentation |

📖 **Full API docs**: `http://localhost:8080/swagger/index.html`

## Configuration

All configuration via environment variables. Copy `.env.example` to `.env`:

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `READ_TIMEOUT` | `5s` | Request read timeout |
| `WRITE_TIMEOUT` | `5s` | Response write timeout |
| `BODY_LIMIT` | `1048576` | Max request body (1MB) |
| `DOMAIN` | `` | Domain for Swagger docs |

### Cache

| Variable | Default | Description |
|----------|---------|-------------|
| `CACHE_MAX_SIZE` | `10000` | Max cached URL resolutions |
| `CACHE_TTL` | `1h` | Cache entry TTL |

### Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `DATA_DIR` | `./data` | Data directory |
| `RULES_DIR` | `./rules` | Rules root directory |
| `LOCAL_RULES_DIR` | `./rules/local` | Local rules (priority 3) |
| `COMMUNITY_RULES_DIR` | `./rules/community` | Community packs (priority 1) |
| `OVERRIDE_RULES_DIR` | `./rules/overrides` | Overrides (priority 2) |

### Community

| Variable | Default | Description |
|----------|---------|-------------|
| `COMMUNITY_REPO_URL` | `https://api.github.com/repos/freewebtopdf/asset-injector-community-rules` | Community pack repository |
| `COMMUNITY_REPO_TIMEOUT` | `30s` | GitHub API timeout |
| `AUTO_UPDATE_PACKS` | `false` | Auto-update packs on startup |
| `SINGLES_SYNC_ENABLED` | `false` | Enable auto-sync of individual contributed rules |
| `SINGLES_SYNC_INTERVAL` | `5m` | Polling interval for singles sync |

### Security

| Variable | Default | Description |
|----------|---------|-------------|
| `CORS_ORIGINS` | `*` | Allowed CORS origins (comma-separated) |
| `ENABLE_HTTPS` | `false` | Enable HTTPS |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `json` | `json` / `text` |

## Project Structure

```
github.com/freewebtopdf/asset-injector/
├── cmd/
│   └── server/
│       └── main.go              # Application entrypoint & DI wiring
├── internal/                    # Private application code
│   ├── api/                     # HTTP layer
│   │   ├── router.go            # Fiber app setup & middleware
│   │   ├── handlers.go          # Core handlers (resolve, rules)
│   │   └── pack_handlers.go     # Pack management handlers
│   ├── cache/
│   │   └── lru.go               # LRU cache implementation
│   ├── community/
│   │   ├── client.go            # GitHub API client
│   │   ├── cache.go             # Index caching
│   │   └── version.go           # SemVer utilities
│   ├── config/
│   │   └── config.go            # Environment config (caarlos0/env)
│   ├── conflict/
│   │   ├── detector.go          # Conflict detection
│   │   └── resolver.go          # Priority-based resolution
│   ├── domain/                  # Core domain models
│   │   ├── rule.go              # Rule model
│   │   ├── pack.go              # Pack model
│   │   ├── interfaces.go        # Repository/Service interfaces
│   │   ├── errors.go            # Domain errors
│   │   └── validator.go         # Input validation
│   ├── health/
│   │   └── checker.go           # System health monitoring
│   ├── loader/
│   │   ├── loader.go            # Rule file loading
│   │   ├── scanner.go           # Directory scanning
│   │   ├── parser.go            # YAML parsing
│   │   └── writer.go            # Atomic file writes
│   ├── matcher/
│   │   └── matcher.go           # Pattern matching engine
│   ├── middleware/
│   │   └── ratelimit.go         # Token bucket rate limiter
│   ├── pack/
│   │   ├── manager.go           # Pack install/update/remove
│   │   ├── manifest.go          # Manifest parsing
│   │   └── dependency.go        # Dependency resolution
│   └── storage/
│       └── store.go             # Rule repository (in-memory + file)
├── deploy/
│   ├── base/                    # Kubernetes base manifests
│   ├── overlays/
│   │   ├── staging/             # Staging overrides
│   │   └── production/          # Production overrides
│   ├── monitoring/              # Prometheus rules
│   └── docker-compose.yml
├── docs/
│   ├── swagger.yaml             # OpenAPI spec
│   ├── architecture.md          # System design
│   ├── deployment.md            # Deployment guide
│   └── runbook.md               # Operations runbook
├── scripts/
│   ├── health-check.sh          # Health check script
│   └── rollback.sh              # Rollback script
├── .env.example                 # Environment template
├── .golangci.yml                # Linter config
├── Dockerfile
├── Makefile
└── go.mod
```

## Development

### Prerequisites

```bash
# Required: Go 1.25+
go version

# Install development tools
go install github.com/swaggo/swag/cmd/swag@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Common Commands

```bash
# Run locally with hot reload (using air or similar)
go run cmd/server/main.go

# Build binary
go build -o bin/server cmd/server/main.go

# Run linter
golangci-lint run

# Generate Swagger docs
swag init -g cmd/server/main.go -o ./docs

# Format code
go fmt ./...
goimports -w .
```

### Using Make

```bash
make build       # Build binary
make test        # Run tests
make test-race   # Run tests with race detector
make coverage    # Generate coverage report
make lint        # Run linter
make docs        # Generate Swagger docs
make docker-build # Build Docker image
make run         # Run locally
```

### Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` / `goimports` for formatting
- Group imports: stdlib → external → internal
- Write table-driven tests
- Document all exported types and functions
- Keep functions focused and small (<50 lines preferred)

## Testing

### Run Tests

```bash
# All tests
go test ./...

# With race detector (CI requirement)
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Specific package
go test ./internal/matcher/...

# Verbose output
go test -v ./internal/api/...
```

### Test Types

| Type | Location | Description |
|------|----------|-------------|
| Unit | `*_test.go` | Isolated component tests |
| Integration | `integration_test.go` | Full HTTP request/response |
| Property-based | Uses `gopter` | Randomized input testing |

### Example Test

```go
func TestMatcher_Resolve(t *testing.T) {
    tests := []struct {
        name    string
        rules   []domain.Rule
        url     string
        wantID  string
        wantErr bool
    }{
        {
            name: "exact match wins over wildcard",
            rules: []domain.Rule{
                {ID: "1", Type: "exact", Pattern: "https://example.com/page"},
                {ID: "2", Type: "wildcard", Pattern: "https://example.com/*"},
            },
            url:    "https://example.com/page",
            wantID: "1",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ... test implementation
        })
    }
}
```

## Deployment

### Docker

```bash
# Build image
docker build -t freewebtopdf/asset-injector:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/rules:/app/rules \
  -v $(pwd)/data:/app/data \
  -e LOG_LEVEL=info \
  freewebtopdf/asset-injector:latest
```

### Kubernetes

```bash
# Deploy to staging
kubectl apply -k deploy/overlays/staging

# Deploy to production
kubectl apply -k deploy/overlays/production

# Check rollout status
kubectl rollout status deployment/asset-injector -n asset-injector

# View logs
kubectl logs -f deployment/asset-injector -n asset-injector
```

### Health Checks

```bash
# Liveness probe (is the process alive?)
curl http://localhost:8080/health

# Response when healthy:
{
  "status": "healthy",
  "timestamp": "2026-01-12T19:00:00Z",
  "components": {
    "storage": {"status": "healthy"},
    "matcher": {"status": "healthy"},
    "cache": {"status": "healthy"}
  }
}
```

### Monitoring

- **Metrics endpoint**: `/metrics` returns cache stats, rule counts
- **Prometheus**: ServiceMonitor in `deploy/monitoring/`
- **Alerting**: Rules in `deploy/monitoring/alerting-rules.yaml`

See [docs/deployment.md](docs/deployment.md) and [docs/runbook.md](docs/runbook.md) for detailed guides.

## Contributing

We welcome contributions!

### Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/asset-injector.git`
3. Create a feature branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Run tests: `go test -race ./...`
6. Run linter: `golangci-lint run`
7. Commit: `git commit -m 'feat: add my feature'`
8. Push: `git push origin feature/my-feature`
9. Open a Pull Request

### Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation
- `refactor:` Code refactoring
- `test:` Adding tests
- `chore:` Maintenance

### Reporting Issues

- Use [GitHub Issues](https://github.com/freewebtopdf/asset-injector/issues)
- Include: Go version, OS, steps to reproduce
- For security issues: email <security@freewebtopdf.com>

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Quick Links

| Resource | Link |
|----------|------|
| 📖 API Documentation | [docs/README.md](docs/README.md) |
| 🏗️ Architecture | [docs/architecture.md](docs/architecture.md) |
| 🚀 Deployment Guide | [docs/deployment.md](docs/deployment.md) |
| 📋 Operations Runbook | [docs/runbook.md](docs/runbook.md) |
| 🐛 Issue Tracker | [GitHub Issues](https://github.com/freewebtopdf/asset-injector/issues) |
| 📦 Community Packs | [asset-injector-community-rules](https://github.com/freewebtopdf/asset-injector-community-rules) |
