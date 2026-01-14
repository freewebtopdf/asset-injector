package matcher

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// Matcher implements the PatternMatcher interface with thread-safe operations
type Matcher struct {
	mu         sync.RWMutex
	rules      []domain.Rule
	repository domain.RuleRepository
	cache      domain.CacheManager
}

// NewMatcher creates a new Matcher instance
func NewMatcher(repository domain.RuleRepository, cache domain.CacheManager) *Matcher {
	return &Matcher{
		repository: repository,
		cache:      cache,
		rules:      make([]domain.Rule, 0),
	}
}

// Resolve finds the best matching rule for the given URL
func (m *Matcher) Resolve(ctx context.Context, url string) (*domain.MatchResult, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, domain.NewAppErrorWithCause(
			domain.ErrTimeout,
			"Resolve operation cancelled",
			408,
			ctx.Err(),
			map[string]any{"url": url},
		).WithContext(ctx, "resolve")
	default:
	}

	// Check cache first
	if cachedResult, found := m.cache.Get(url); found {
		// Create a new result to avoid race conditions on shared cached objects
		result := &domain.MatchResult{
			RuleID:    cachedResult.RuleID,
			CSS:       cachedResult.CSS,
			JS:        cachedResult.JS,
			Score:     cachedResult.Score,
			CacheHit:  true,
			Timestamp: time.Now(),
		}
		return result, nil
	}

	// Use read lock for concurrent access and release it as soon as possible
	m.mu.RLock()
	rulesCopy := make([]domain.Rule, len(m.rules))
	copy(rulesCopy, m.rules)
	m.mu.RUnlock()

	var bestMatch *domain.Rule
	var bestScore int

	// Iterate through all rules to find the best match
	for i := range rulesCopy {
		// Check context periodically during long operations
		select {
		case <-ctx.Done():
			return nil, domain.NewAppErrorWithCause(
				domain.ErrTimeout,
				"Resolve operation cancelled during matching",
				408,
				ctx.Err(),
				map[string]any{"url": url, "processed_rules": i},
			).WithContext(ctx, "resolve")
		default:
		}

		rule := &rulesCopy[i]
		if matches, score := m.matchRule(rule, url); matches {
			// Higher score wins; on tie, prefer rule added earlier (stable)
			if bestMatch == nil || score > bestScore {
				bestMatch = rule
				bestScore = score
			}
		}
	}

	// Create result
	var result *domain.MatchResult
	if bestMatch != nil {
		result = &domain.MatchResult{
			RuleID:    bestMatch.ID,
			CSS:       bestMatch.CSS,
			JS:        bestMatch.JS,
			Score:     bestScore,
			CacheHit:  false,
			Timestamp: time.Now(),
		}
		// Only cache positive matches to avoid cache pollution
		m.cache.Set(url, result)
	} else {
		result = &domain.MatchResult{
			RuleID:    "",
			CSS:       "",
			JS:        "",
			Score:     0,
			CacheHit:  false,
			Timestamp: time.Now(),
		}
		// Don't cache empty results - they would pollute the cache
	}

	return result, nil
}

// matchRule checks if a rule matches the URL and returns the score
func (m *Matcher) matchRule(rule *domain.Rule, url string) (bool, int) {
	var matches bool
	var baseScore int

	switch rule.Type {
	case "exact":
		matches = rule.Pattern == url
		baseScore = 1000
	case "regex":
		if rule.GetCompiledRegex() == nil {
			log.Warn().Str("rule_id", rule.ID).Str("pattern", rule.Pattern).Msg("Regex rule has nil compiled pattern")
			return false, 0
		}
		matches = rule.GetCompiledRegex().MatchString(url)
		baseScore = 500
	case "wildcard":
		matches = m.matchWildcard(rule.Pattern, url)
		baseScore = 100
	default:
		return false, 0
	}

	if !matches {
		return false, 0
	}

	// Priority override takes precedence
	if rule.Priority != nil {
		return true, *rule.Priority
	}

	// Base scores ensure type hierarchy: exact (1000-1499) > regex (500-999) > wildcard (100-499)
	return true, baseScore + min(len(rule.Pattern), 499)
}

// matchWildcard performs wildcard pattern matching for URLs
// Supports * (matches any characters except nothing) and ? (matches single character)
func (m *Matcher) matchWildcard(pattern, url string) bool {
	return wildcardMatch(pattern, url)
}

// wildcardMatch implements glob-style matching using iterative algorithm
// * matches zero or more characters, ? matches exactly one character
// Uses O(n) space and O(n*m) worst-case time with early termination
func wildcardMatch(pattern, str string) bool {
	pi, si := 0, 0
	starIdx, matchIdx := -1, 0

	for si < len(str) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == str[si]) {
			pi++
			si++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			matchIdx = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
		} else {
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// AddRule adds a new rule to the matcher
func (m *Matcher) AddRule(ctx context.Context, rule *domain.Rule) error {
	// Pre-compile regex if needed
	if rule.Type == "regex" {
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return domain.NewAppError(
				domain.ErrValidationFailed,
				"Invalid regex pattern",
				422,
				map[string]any{
					"field":  "pattern",
					"value":  rule.Pattern,
					"reason": err.Error(),
				},
			)
		}
		rule.SetCompiledRegex(compiled)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Add rule to internal slice
	m.rules = append(m.rules, *rule)

	// Invalidate cache since rules changed
	m.cache.Clear()

	return nil
}

// RemoveRule removes a rule from the matcher
func (m *Matcher) RemoveRule(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove rule
	for i, rule := range m.rules {
		if rule.ID == id {
			// Remove rule from slice
			m.rules = append(m.rules[:i], m.rules[i+1:]...)

			// Invalidate cache since rules changed
			m.cache.Clear()

			return nil
		}
	}

	return domain.NewAppError(
		domain.ErrNotFound,
		"Rule not found",
		404,
		map[string]any{
			"rule_id": id,
		},
	)
}

// UpdateRule updates an existing rule in the matcher
func (m *Matcher) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	// Pre-compile regex if needed
	if rule.Type == "regex" {
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return domain.NewAppError(
				domain.ErrValidationFailed,
				"Invalid regex pattern",
				422,
				map[string]any{
					"field":  "pattern",
					"value":  rule.Pattern,
					"reason": err.Error(),
				},
			)
		}
		rule.SetCompiledRegex(compiled)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and update rule
	for i, existingRule := range m.rules {
		if existingRule.ID == rule.ID {
			// Update rule in slice
			m.rules[i] = *rule

			// Invalidate cache since rules changed
			m.cache.Clear()

			return nil
		}
	}

	return domain.NewAppError(
		domain.ErrNotFound,
		"Rule not found",
		404,
		map[string]any{
			"rule_id": rule.ID,
		},
	)
}

// InvalidateCache clears the cache
func (m *Matcher) InvalidateCache(ctx context.Context) error {
	m.cache.Clear()
	return nil
}

// LoadRules loads all rules from the repository
func (m *Matcher) LoadRules(ctx context.Context) error {
	rules, err := m.repository.GetAllRules(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Pre-compile regex patterns
	for i := range rules {
		if rules[i].Type == "regex" {
			compiled, err := regexp.Compile(rules[i].Pattern)
			if err != nil {
				// Log error but continue with other rules
				continue
			}
			rules[i].SetCompiledRegex(compiled)
		}
	}

	m.rules = rules
	m.cache.Clear()

	return nil
}

// HealthCheck performs a health check on the matcher
func (m *Matcher) HealthCheck(ctx context.Context) domain.HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	status := "healthy"
	message := "Matcher is operating normally"

	ruleCount := len(m.rules)
	cacheStats := m.cache.Stats()

	details := map[string]any{
		"rule_count":      ruleCount,
		"cache_size":      cacheStats.Size,
		"cache_hits":      cacheStats.Hits,
		"cache_misses":    cacheStats.Misses,
		"cache_hit_ratio": cacheStats.HitRatio,
	}

	// Check for potential issues
	if ruleCount == 0 {
		status = "degraded"
		message = "No rules loaded"
		details["warning"] = "Matcher has no rules to match against"
	}

	// Check cache health
	cacheHealth := m.cache.HealthCheck(ctx)
	if cacheHealth.Status != "healthy" {
		if status == "healthy" {
			status = "degraded"
		}
		message = "Cache issues detected"
		details["cache_status"] = cacheHealth.Status
		details["cache_message"] = cacheHealth.Message
	}

	// Validate rule integrity
	invalidRules := 0
	for _, rule := range m.rules {
		if rule.Type == "regex" && rule.GetCompiledRegex() == nil {
			invalidRules++
		}
	}

	if invalidRules > 0 {
		status = "degraded"
		message = "Some rules have compilation issues"
		details["invalid_rules"] = invalidRules
	}

	return domain.HealthStatus{
		Status:    status,
		Message:   message,
		Details:   details,
		Timestamp: now,
	}
}

// GetStats returns matcher statistics
func (m *Matcher) GetStats(ctx context.Context) map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cacheStats := m.cache.Stats()

	stats := map[string]any{
		"rule_count":      len(m.rules),
		"cache_hits":      cacheStats.Hits,
		"cache_misses":    cacheStats.Misses,
		"cache_size":      cacheStats.Size,
		"cache_max_size":  cacheStats.MaxSize,
		"cache_hit_ratio": cacheStats.HitRatio,
	}

	// Add rule type distribution
	typeCount := make(map[string]int)
	compiledRegexCount := 0

	for _, rule := range m.rules {
		typeCount[rule.Type]++
		if rule.Type == "regex" && rule.GetCompiledRegex() != nil {
			compiledRegexCount++
		}
	}

	stats["rule_types"] = typeCount
	stats["compiled_regex_rules"] = compiledRegexCount

	return stats
}
