package health

import (
	"context"
	"sync"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// SystemHealthChecker implements comprehensive system health monitoring
type SystemHealthChecker struct {
	repository domain.RuleRepository
	matcher    domain.PatternMatcher
	cache      domain.CacheManager

	// Health check configuration
	timeout   time.Duration
	startTime time.Time

	// Cached health status to avoid expensive checks on every request
	lastCheck   time.Time
	lastHealth  domain.SystemHealth
	cacheTTL    time.Duration
	healthMutex sync.RWMutex
}

// NewSystemHealthChecker creates a new system health checker
func NewSystemHealthChecker(
	repository domain.RuleRepository,
	matcher domain.PatternMatcher,
	cache domain.CacheManager,
) *SystemHealthChecker {
	return &SystemHealthChecker{
		repository: repository,
		matcher:    matcher,
		cache:      cache,
		timeout:    5 * time.Second,
		cacheTTL:   30 * time.Second,
		startTime:  time.Now(),
	}
}

// CheckHealth performs a comprehensive system health check
func (h *SystemHealthChecker) CheckHealth(ctx context.Context) domain.SystemHealth {
	h.healthMutex.Lock()
	defer h.healthMutex.Unlock()

	// Return cached result if still valid
	if time.Since(h.lastCheck) < h.cacheTTL {
		return h.lastHealth
	}

	// Create context with timeout for health checks
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	now := time.Now()
	components := make(map[string]domain.HealthStatus)
	overallStatus := "healthy"

	// Check storage component
	storageHealth := h.repository.HealthCheck(checkCtx)
	components["storage"] = storageHealth
	if storageHealth.Status != "healthy" {
		overallStatus = h.aggregateStatus(overallStatus, storageHealth.Status)
	}

	// Check matcher component
	matcherHealth := h.matcher.HealthCheck(checkCtx)
	components["matcher"] = matcherHealth
	if matcherHealth.Status != "healthy" {
		overallStatus = h.aggregateStatus(overallStatus, matcherHealth.Status)
	}

	// Check cache component
	cacheHealth := h.cache.HealthCheck(checkCtx)
	components["cache"] = cacheHealth
	if cacheHealth.Status != "healthy" {
		overallStatus = h.aggregateStatus(overallStatus, cacheHealth.Status)
	}

	// Collect system metrics
	metrics := h.collectSystemMetrics(checkCtx)

	systemHealth := domain.SystemHealth{
		Status:     overallStatus,
		Timestamp:  now,
		Components: components,
		Metrics:    metrics,
	}

	// Cache the result
	h.lastCheck = now
	h.lastHealth = systemHealth

	return systemHealth
}

// CheckComponent performs a health check on a specific component
func (h *SystemHealthChecker) CheckComponent(ctx context.Context, component string) domain.HealthStatus {
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	switch component {
	case "storage":
		return h.repository.HealthCheck(checkCtx)
	case "matcher":
		return h.matcher.HealthCheck(checkCtx)
	case "cache":
		return h.cache.HealthCheck(checkCtx)
	default:
		return domain.HealthStatus{
			Status:    "unhealthy",
			Message:   "Unknown component",
			Timestamp: time.Now(),
			Details: map[string]any{
				"component": component,
				"error":     "Component not found",
			},
		}
	}
}

// aggregateStatus determines the overall status based on component statuses
func (h *SystemHealthChecker) aggregateStatus(current, componentStatus string) string {
	// Priority: unhealthy > degraded > healthy
	statusPriority := map[string]int{
		"healthy":   0,
		"degraded":  1,
		"unhealthy": 2,
	}

	currentPriority := statusPriority[current]
	componentPriority := statusPriority[componentStatus]

	if componentPriority > currentPriority {
		return componentStatus
	}
	return current
}

// collectSystemMetrics gathers system-wide metrics
func (h *SystemHealthChecker) collectSystemMetrics(ctx context.Context) map[string]any {
	metrics := make(map[string]any)

	// Collect storage metrics
	if storageStats := h.repository.GetStats(ctx); storageStats != nil {
		metrics["storage"] = storageStats
	}

	// Collect matcher metrics
	if matcherStats := h.matcher.GetStats(ctx); matcherStats != nil {
		metrics["matcher"] = matcherStats
	}

	// Collect cache metrics
	cacheStats := h.cache.Stats()
	metrics["cache"] = map[string]any{
		"hits":      cacheStats.Hits,
		"misses":    cacheStats.Misses,
		"size":      cacheStats.Size,
		"max_size":  cacheStats.MaxSize,
		"hit_ratio": cacheStats.HitRatio,
	}

	// Add system-level metrics
	metrics["system"] = map[string]any{
		"uptime_seconds": time.Since(h.startTime).Seconds(),
		"timestamp":      time.Now(),
	}

	return metrics
}

// GetDetailedHealth returns detailed health information for debugging
func (h *SystemHealthChecker) GetDetailedHealth(ctx context.Context) map[string]any {
	systemHealth := h.CheckHealth(ctx)

	detailed := map[string]any{
		"overall_status": systemHealth.Status,
		"timestamp":      systemHealth.Timestamp,
		"components":     systemHealth.Components,
		"metrics":        systemHealth.Metrics,
	}

	// Add additional diagnostic information
	detailed["diagnostics"] = map[string]any{
		"health_check_timeout": h.timeout.String(),
		"cache_ttl":            h.cacheTTL.String(),
		"last_check_age":       time.Since(h.lastCheck).String(),
	}

	return detailed
}

// IsHealthy returns true if the system is healthy
func (h *SystemHealthChecker) IsHealthy(ctx context.Context) bool {
	health := h.CheckHealth(ctx)
	return health.Status == "healthy"
}
