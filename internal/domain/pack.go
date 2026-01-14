package domain

import "time"

// PackManifest represents the manifest.yaml file for a rule pack
// Contains metadata about the pack including name, version, author, and dependencies
type PackManifest struct {
	Name         string            `json:"name" yaml:"name" validate:"required"`                 // Required: unique pack identifier
	Version      string            `json:"version" yaml:"version" validate:"required"`           // Required: semantic version
	Description  string            `json:"description" yaml:"description" validate:"required"`   // Required: pack description
	Author       string            `json:"author" yaml:"author" validate:"required"`             // Required: pack author
	License      string            `json:"license,omitempty" yaml:"license,omitempty"`           // Optional: license identifier
	Homepage     string            `json:"homepage,omitempty" yaml:"homepage,omitempty"`         // Optional: project URL
	Dependencies []PackDependency  `json:"dependencies,omitempty" yaml:"dependencies,omitempty"` // Optional: dependencies on other packs
	Tags         []string          `json:"tags,omitempty" yaml:"tags,omitempty"`                 // Optional: tags for discovery
	Requires     *PackRequirements `json:"requires,omitempty" yaml:"requires,omitempty"`         // Optional: minimum version requirements
}

// PackDependency represents a dependency on another pack
type PackDependency struct {
	Name    string `json:"name" yaml:"name" validate:"required"`       // Name of the required pack
	Version string `json:"version" yaml:"version" validate:"required"` // Version constraint (e.g., ">=1.0.0")
}

// PackRequirements specifies minimum version requirements
type PackRequirements struct {
	AssetInjector string `json:"asset-injector,omitempty" yaml:"asset-injector,omitempty"` // Minimum Asset Injector version
}

// PackInfo represents metadata about an installed or available pack
type PackInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	License     string    `json:"license,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	RuleCount   int       `json:"rule_count"`
	InstalledAt time.Time `json:"installed_at,omitempty"`
	SourceURL   string    `json:"source_url,omitempty"`
}

// PackUpdate represents an available update for an installed pack
type PackUpdate struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ChangelogURL   string `json:"changelog_url,omitempty"`
}

// PackIndex represents the community repository index
type PackIndex struct {
	Version    string     `json:"version"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Packs      []PackInfo `json:"packs"`
	Categories []string   `json:"categories,omitempty"`
}

// PackSource tracks the origin of an installed pack (stored in .source.json)
type PackSource struct {
	SourceURL   string    `json:"source_url"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}
