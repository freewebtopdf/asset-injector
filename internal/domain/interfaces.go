package domain

import "context"

// RuleRepository defines the contract for rule storage operations
type RuleRepository interface {
	GetAllRules(ctx context.Context) ([]Rule, error)
	GetRuleByID(ctx context.Context, id string) (*Rule, error)
	CreateRule(ctx context.Context, rule *Rule) error
	UpdateRule(ctx context.Context, rule *Rule) error
	DeleteRule(ctx context.Context, id string) error

	// Health and monitoring
	HealthCheck(ctx context.Context) HealthStatus
	GetStats(ctx context.Context) map[string]any
}

// BatchRuleRepository extends RuleRepository with batch operations for better performance
type BatchRuleRepository interface {
	RuleRepository
	CreateRules(ctx context.Context, rules []*Rule) error
	UpdateRules(ctx context.Context, rules []*Rule) error
	DeleteRules(ctx context.Context, ids []string) error
}

// PatternMatcher defines the contract for URL matching operations
type PatternMatcher interface {
	Resolve(ctx context.Context, url string) (*MatchResult, error)
	AddRule(ctx context.Context, rule *Rule) error
	UpdateRule(ctx context.Context, rule *Rule) error
	RemoveRule(ctx context.Context, id string) error
	InvalidateCache(ctx context.Context) error
	LoadRules(ctx context.Context) error

	// Health and monitoring
	HealthCheck(ctx context.Context) HealthStatus
	GetStats(ctx context.Context) map[string]any
}

// CacheManager defines the contract for caching operations
type CacheManager interface {
	Get(key string) (*MatchResult, bool)
	Set(key string, result *MatchResult)
	Invalidate(key string)
	Clear()
	Stats() CacheStats

	// Health and monitoring
	HealthCheck(ctx context.Context) HealthStatus
}

// HealthChecker defines the interface for system health monitoring
type HealthChecker interface {
	CheckHealth(ctx context.Context) SystemHealth
	CheckComponent(ctx context.Context, component string) HealthStatus
}

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	IncrementCounter(name string, labels map[string]string)
	RecordHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
	GetMetrics(ctx context.Context) map[string]any
}

// Validator defines the interface for input validation
type Validator interface {
	ValidateRule(rule *Rule) error
	ValidateURL(url string) error
	ValidateContent(content string, maxSize int) error
}
