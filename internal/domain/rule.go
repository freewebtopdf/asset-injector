package domain

import (
	"regexp"
	"time"
)

// SourceType represents the origin type of a rule
type SourceType string

const (
	// SourceLocal indicates a rule created locally by the user
	SourceLocal SourceType = "local"
	// SourceCommunity indicates a rule from a community pack
	SourceCommunity SourceType = "community"
	// SourceOverride indicates a local modification of a community rule
	SourceOverride SourceType = "override"
)

// RuleSource tracks where a rule came from
type RuleSource struct {
	Type        SourceType `json:"type" yaml:"type"`                                     // local, community, override
	PackName    string     `json:"pack_name,omitempty" yaml:"pack_name,omitempty"`       // Name of the source pack
	PackVersion string     `json:"pack_version,omitempty" yaml:"pack_version,omitempty"` // Version of the source pack
	SourceURL   string     `json:"source_url,omitempty" yaml:"source_url,omitempty"`     // URL where the pack was downloaded from
}

// Rule represents a URL pattern matching rule with associated CSS/JS assets
// @Description URL pattern matching rule configuration
type Rule struct {
	ID        string    `json:"id" yaml:"id" validate:"required,uuid4" example:"123e4567-e89b-12d3-a456-426614174000"`
	Type      string    `json:"type" yaml:"type" validate:"required,oneof=exact regex wildcard" example:"exact" enums:"exact,regex,wildcard"`
	Pattern   string    `json:"pattern" yaml:"pattern" validate:"required,min=1,max=2048" example:"https://example.com/*"`
	CSS       string    `json:"css" yaml:"css" validate:"max=102400" example:".banner { display: none; }"`               // 100KB limit
	JS        string    `json:"js" yaml:"js" validate:"max=102400" example:"document.querySelector('.popup').remove();"` // 100KB limit
	Priority  *int      `json:"priority,omitempty" yaml:"priority,omitempty" validate:"omitempty,min=0,max=10000" example:"1500"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at,omitempty" example:"2023-01-01T12:00:00Z"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at,omitempty" example:"2023-01-01T12:00:00Z"`

	// Attribution fields for community sharing
	Author      string   `json:"author,omitempty" yaml:"author,omitempty" example:"contributor-name"`
	ModifiedBy  string   `json:"modified_by,omitempty" yaml:"modified_by,omitempty" example:"modifier-name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty" example:"Hides cookie banner on example.com"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty" example:"cookies,privacy"`

	// Source tracking (internal, not serialized to YAML rule files)
	Source   RuleSource `json:"source,omitempty" yaml:"-"`
	FilePath string     `json:"file_path,omitempty" yaml:"-"` // Path to the rule file on disk

	// Internal fields for performance
	compiledRegex *regexp.Regexp `json:"-" yaml:"-"` // Pre-compiled for regex rules
}

// GetCompiledRegex returns the compiled regex for the rule
func (r *Rule) GetCompiledRegex() *regexp.Regexp {
	return r.compiledRegex
}

// SetCompiledRegex sets the compiled regex for the rule
func (r *Rule) SetCompiledRegex(regex *regexp.Regexp) {
	r.compiledRegex = regex
}

// MatchResult represents the result of a URL pattern match
type MatchResult struct {
	RuleID    string    `json:"rule_id"`
	CSS       string    `json:"css"`
	JS        string    `json:"js"`
	Score     int       `json:"score,omitempty"`
	CacheHit  bool      `json:"cache_hit"`
	Timestamp time.Time `json:"timestamp"`
}

// CacheStats represents cache performance metrics
type CacheStats struct {
	Hits     int64   `json:"hits"`
	Misses   int64   `json:"misses"`
	Size     int     `json:"size"`
	MaxSize  int     `json:"max_size"`
	HitRatio float64 `json:"hit_ratio"`
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status    string         `json:"status"` // "healthy", "unhealthy", "degraded"
	Message   string         `json:"message,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusDegraded  = "degraded"
)

// SystemHealth represents overall system health
type SystemHealth struct {
	Status     string                  `json:"status"`
	Timestamp  time.Time               `json:"timestamp"`
	Components map[string]HealthStatus `json:"components"`
	Metrics    map[string]any          `json:"metrics,omitempty"`
	Uptime     time.Duration           `json:"uptime"`
}
