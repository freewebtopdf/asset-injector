# Asset Injector Community Rules

This document explains how the Asset Injector integrates with the community rules repository ([freewebtopdf/asset-injector-community-rules](https://github.com/freewebtopdf/asset-injector-community-rules)) for sharing and distributing rules.

## Overview

The community system supports two contribution models:

| Model | Use Case | Barrier to Entry |
|-------|----------|------------------|
| **Singles** | Share one rule quickly | Low - just PR a YAML file |
| **Packs** | Distribute curated collections | Medium - requires manifest + versioning |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Asset Injector Worker                            │
│                                                                         │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐   │
│  │  PackManager    │     │  SinglesSyncer  │     │    Store        │   │
│  │  - Install      │     │  - Poll index   │     │  - Load rules   │   │
│  │  - Update       │     │  - Download new │     │  - Reload       │   │
│  │  - Uninstall    │     │  - ETag caching │     │                 │   │
│  └────────┬────────┘     └────────┬────────┘     └────────┬────────┘   │
│           │                       │                       │            │
└───────────┼───────────────────────┼───────────────────────┼────────────┘
            │                       │                       │
            ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    GitHub: freewebtopdf/asset-injector-community-rules  │
│                                                                         │
│  ┌──────────────────────────────┐  ┌──────────────────────────────────┐ │
│  │ index.json (curated packs)   │  │ singles/index.json (auto-gen)    │ │
│  │ packs/{name}/{name}-v.zip    │  │ singles/rules/*.rule.yaml        │ │
│  └──────────────────────────────┘  └──────────────────────────────────┘ │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ GitHub Actions                                                    │   │
│  │ - validate-rules.yml: Check PR syntax                            │   │
│  │ - update-singles-index.yml: Regenerate index on merge            │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## Contributing Rules

### Option 1: Singles (Recommended for Individual Rules)

The simplest way to share a rule:

1. Fork `freewebtopdf/asset-injector-community-rules`
2. Create `singles/rules/{descriptive-name}.rule.yaml`
3. Submit a PR

**Rule file format:**

```yaml
# singles/rules/medium-paywall.rule.yaml
id: "medium-paywall"
type: wildcard
pattern: "https://medium.com/*"
description: "Remove Medium paywall overlay"
author: "your-github-username"
tags:
  - paywall
  - reading
css: |
  .meteredContent { display: none !important; }
js: |
  document.querySelector('.paywall-overlay')?.remove();
```

**Naming conventions:**
- Lowercase with hyphens: `site-name-what-it-does.rule.yaml`
- Examples: `medium-paywall.rule.yaml`, `youtube-cookie-consent.rule.yaml`

**What happens after merge:**
1. GitHub Action validates the rule syntax
2. On merge, `singles/index.json` is auto-regenerated
3. Workers with `SINGLES_SYNC_ENABLED=true` pick up the new rule within their sync interval

### Option 2: Packs (For Curated Collections)

For distributing versioned collections of related rules:

**Pack structure:**

```
packs/cookie-banners/
└── cookie-banners-1.0.0.zip
    ├── manifest.yaml
    └── rules/
        ├── generic.rule.yaml
        ├── onetrust.rule.yaml
        └── cookiebot.rule.yaml
```

**manifest.yaml:**

```yaml
name: cookie-banners
version: 1.0.0
description: Hide cookie consent banners for cleaner PDFs
author: community
license: MIT
homepage: https://github.com/freewebtopdf/asset-injector-community-rules
tags:
  - privacy
  - cookies
  - gdpr
dependencies: []
```

## Repository Structure

```
freewebtopdf/asset-injector-community-rules/
├── index.json                    # Pack index (curated packs)
├── packs/                        # Versioned pack archives
│   └── {pack-name}/
│       └── {pack-name}-{version}.zip
├── singles/
│   ├── index.json                # Auto-generated from rules/
│   └── rules/                    # Individual contributed rules
│       ├── medium-paywall.rule.yaml
│       └── youtube-cookie-consent.rule.yaml
├── scripts/
│   └── generate-singles-index.js # Index generator
└── .github/
    └── workflows/
        ├── validate-rules.yml    # PR validation
        └── update-singles-index.yml  # Auto-regenerate index
```

### index.json (Packs)

```json
{
  "version": "1.0.0",
  "updated_at": "2026-01-13T00:00:00Z",
  "categories": ["privacy", "accessibility", "print-optimization"],
  "packs": [
    {
      "name": "cookie-banners",
      "version": "2.1.0",
      "description": "Hide cookie consent banners",
      "author": "community",
      "rule_count": 45
    }
  ]
}
```

### singles/index.json (Auto-generated)

```json
{
  "version": "1.0.0",
  "updated_at": "2026-01-13T04:30:00Z",
  "count": 12,
  "rules": [
    {
      "id": "medium-paywall",
      "pattern": "https://medium.com/*",
      "type": "wildcard",
      "description": "Remove Medium paywall overlay",
      "author": "contributor",
      "tags": ["paywall", "reading"],
      "file": "medium-paywall.rule.yaml"
    }
  ]
}
```

## Worker Configuration

### Singles Sync (Individual Rules)

Enable automatic syncing of individual contributed rules:

```bash
# Enable singles sync
SINGLES_SYNC_ENABLED=true

# Sync interval (default: 5 minutes)
SINGLES_SYNC_INTERVAL=5m
```

**How it works:**

1. Worker polls `singles/index.json` every sync interval
2. Uses HTTP ETag header - skips download if unchanged
3. Downloads new/updated rule files to `rules/community/singles/`
4. Triggers store reload and cache clear
5. New rules are immediately available for matching

### Pack Management (Curated Collections)

```bash
# Auto-update installed packs on startup
AUTO_UPDATE_PACKS=true

# Repository settings
COMMUNITY_REPO_URL=https://api.github.com/repos/freewebtopdf/asset-injector-community-rules
COMMUNITY_REPO_TIMEOUT=30s
```

## API Endpoints

### Singles (via sync)

Singles are synced automatically - no API needed. They appear as regular rules after sync.

### Packs

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/packs` | List installed packs |
| `GET` | `/v1/packs/available` | Browse available packs |
| `POST` | `/v1/packs/install` | Install pack |
| `DELETE` | `/v1/packs/:name` | Uninstall pack |
| `POST` | `/v1/packs/update` | Update packs |

**Install a pack:**

```bash
curl -X POST http://localhost:8080/v1/packs/install \
  -H "Content-Type: application/json" \
  -d '{"source": "cookie-banners@1.0.0"}'

# Or latest version
curl -X POST http://localhost:8080/v1/packs/install \
  -H "Content-Type: application/json" \
  -d '{"source": "cookie-banners"}'
```

**Update all packs:**

```bash
curl -X POST http://localhost:8080/v1/packs/update \
  -H "Content-Type: application/json" \
  -d '{"all": true}'
```

## Local Storage

```
rules/
├── local/                      # Priority 3 (highest) - Your rules
│   └── my-rule.rule.yaml
├── overrides/                  # Priority 2 - Modified community rules
│   └── cookie-banners/
│       └── custom.rule.yaml
└── community/                  # Priority 1 (lowest) - Community rules
    ├── singles/                # Auto-synced individual rules
    │   ├── medium-paywall.rule.yaml
    │   └── youtube-cookie-consent.rule.yaml
    └── cookie-banners/         # Installed packs
        ├── manifest.yaml
        ├── .source.json
        └── rules/
            └── *.rule.yaml
```

## Conflict Resolution

When multiple rules match the same URL pattern:

| Priority | Source | Location |
|----------|--------|----------|
| 3 (highest) | Local | `rules/local/` |
| 2 | Override | `rules/overrides/` |
| 1 (lowest) | Community | `rules/community/` |

Equal priority ties are broken by most recent `updated_at` timestamp.

### Creating Overrides

To customize a community rule without losing updates:

1. Create `rules/overrides/{pack-name}/` directory
2. Add your modified rule with the same ID
3. Your override takes precedence

## GitHub Actions Workflow

### PR Validation (`validate-rules.yml`)

Runs on PRs to `singles/rules/`:

- Validates YAML syntax
- Checks required fields: `id`, `pattern`, `type`
- Ensures `css` or `js` is present
- Validates `type` is one of: `exact`, `wildcard`, `regex`

### Index Generation (`update-singles-index.yml`)

Runs on merge to `main`:

1. Scans all `*.rule.yaml` files in `singles/rules/`
2. Parses metadata from each file
3. Generates `singles/index.json`
4. Commits and pushes the updated index

## Rule File Reference

```yaml
# Required fields
id: "unique-rule-id"              # Unique identifier
type: "wildcard"                  # exact | wildcard | regex
pattern: "https://example.com/*"  # URL pattern to match

# At least one required
css: |
  .element { display: none; }
js: |
  document.querySelector('.popup')?.remove();

# Optional metadata
description: "What this rule does"
author: "github-username"
tags:
  - category1
  - category2
priority: 1500                    # Override default scoring (0-10000)
```

### Pattern Types

| Type | Example | Matches |
|------|---------|---------|
| `exact` | `https://example.com/page` | Only that exact URL |
| `wildcard` | `https://example.com/*` | Any path on example.com |
| `wildcard` | `https://*.example.com/*` | Any subdomain |
| `regex` | `^https://example\\.com/user/\\d+$` | Regex pattern |

## Caching

### Singles Index Cache

- Uses HTTP ETag for change detection
- Only downloads when index changes
- Falls back to last known state if GitHub unavailable

### Pack Index Cache

| Setting | Default | Description |
|---------|---------|-------------|
| Cache TTL | 1 hour | Time before re-fetching |
| Location | `{DATA_DIR}/pack-index.cache.json` | Cache file |
| Fallback | Expired cache | Used when GitHub unavailable |

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `COMMUNITY_REPO_URL` | `https://api.github.com/repos/freewebtopdf/asset-injector-community-rules` | Repository API URL |
| `COMMUNITY_REPO_TIMEOUT` | `30s` | API request timeout |
| `COMMUNITY_RULES_DIR` | `./rules/community` | Install directory |
| `OVERRIDE_RULES_DIR` | `./rules/overrides` | Override directory |
| `AUTO_UPDATE_PACKS` | `false` | Auto-update packs on startup |
| `SINGLES_SYNC_ENABLED` | `false` | Enable singles auto-sync |
| `SINGLES_SYNC_INTERVAL` | `5m` | Sync polling interval |

## Security

- Rule files are validated before use
- Zip archives checked for path traversal (zip slip)
- HTTPS required for repository communication
- Rules execute in browser context (same security model as browser extensions)

## Troubleshooting

### Singles not syncing

1. Check `SINGLES_SYNC_ENABLED=true`
2. Verify network access to GitHub API
3. Check logs for sync errors

### Pack install fails

1. Verify pack exists: `GET /v1/packs/available`
2. Check version format: `pack-name@1.0.0`
3. Review logs for download/validation errors

### Rule not matching

1. Verify rule is loaded: `GET /v1/rules`
2. Check pattern syntax matches URL
3. Test with `/v1/resolve` endpoint
