# Architecture

## Overview

The Asset Injector Microservice uses a layered architecture with clear separation of concerns.

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Layer (Fiber)                   │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────────┐ │
│  │ Resolve │ │  Rules  │ │  Packs  │ │ Health/Metrics│ │
│  └────┬────┘ └────┬────┘ └────┬────┘ └───────┬───────┘ │
└───────┼───────────┼───────────┼───────────────┼─────────┘
        │           │           │               │
┌───────▼───────────▼───────────▼───────────────▼─────────┐
│                    Domain Layer                          │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────┐  │
│  │ Matcher │ │ Validator│ │ Conflict │ │HealthChecker│  │
│  └────┬────┘ └──────────┘ │ Resolver │ └─────────────┘  │
│       │                   └────┬─────┘                   │
└───────┼────────────────────────┼────────────────────────┘
        │                        │
┌───────▼────────────────────────▼────────────────────────┐
│                  Infrastructure Layer                    │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────────┐  │
│  │LRU Cache│ │  Store  │ │ Loader  │ │Community Client│  │
│  └─────────┘ └─────────┘ └─────────┘ └───────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## Pattern Matching Algorithm

### Scoring System

Each rule type has a base score ensuring type hierarchy:

| Type | Base Score | Final Score |
|------|------------|-------------|
| exact | 1000 | 1000 + min(len(pattern), 499) |
| regex | 500 | 500 + min(len(pattern), 499) |
| wildcard | 100 | 100 + min(len(pattern), 499) |

This guarantees: **exact > regex > wildcard** regardless of pattern length.

### Priority Override

Setting `priority` on a rule overrides the calculated score entirely.

### Wildcard Matching

Uses an iterative algorithm (not recursive) for O(n*m) worst-case performance:
- `*` matches zero or more characters
- `?` matches exactly one character

## Conflict Resolution

When multiple rules share the same ID (from different sources):

```
Priority: local (3) > override (2) > community (1)
```

Ties broken by `UpdatedAt` timestamp (most recent wins).

## Caching Strategy

- **LRU Cache**: O(1) get/set with doubly-linked list + hashmap
- **Cache Invalidation**: Full clear on any rule change
- **No negative caching**: Empty results not cached to avoid pollution

## File-Based Storage

```
rules/
├── local/           # User rules (highest priority)
│   └── *.rule.yaml
├── community/       # Installed packs
│   └── {pack-name}/
│       └── *.rule.yaml
└── overrides/       # Local modifications to community rules
    └── *.rule.yaml
```

### Atomic Writes

All file operations use: `temp file → fsync → rename` pattern for crash safety.

## Thread Safety

| Component | Strategy |
|-----------|----------|
| Matcher | RWMutex (read-heavy) |
| Cache | Mutex per operation |
| Store | RWMutex with copy-on-read |
| Rate Limiter | Per-bucket mutex |
