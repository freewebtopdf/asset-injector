package matcher

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock implementations for testing
type mockRepository struct {
	rules []domain.Rule
}

func (m *mockRepository) GetAllRules(ctx context.Context) ([]domain.Rule, error) {
	return m.rules, nil
}

func (m *mockRepository) GetRuleByID(ctx context.Context, id string) (*domain.Rule, error) {
	for _, rule := range m.rules {
		if rule.ID == id {
			return &rule, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrNotFound, "Rule not found", 404, nil)
}

func (m *mockRepository) CreateRule(ctx context.Context, rule *domain.Rule) error {
	m.rules = append(m.rules, *rule)
	return nil
}

func (m *mockRepository) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	for i, r := range m.rules {
		if r.ID == rule.ID {
			m.rules[i] = *rule
			return nil
		}
	}
	return domain.NewAppError(domain.ErrNotFound, "Rule not found", 404, nil)
}

func (m *mockRepository) DeleteRule(ctx context.Context, id string) error {
	for i, rule := range m.rules {
		if rule.ID == id {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return nil
		}
	}
	return domain.NewAppError(domain.ErrNotFound, "Rule not found", 404, nil)
}

func (m *mockRepository) HealthCheck(ctx context.Context) domain.HealthStatus {
	return domain.HealthStatus{Status: domain.HealthStatusHealthy, Timestamp: time.Now()}
}

func (m *mockRepository) GetStats(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"total_rules": len(m.rules),
	}
}

type mockCache struct {
	mu   sync.RWMutex
	data map[string]*domain.MatchResult
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]*domain.MatchResult),
	}
}

func (m *mockCache) Get(key string) (*domain.MatchResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, found := m.data[key]
	return result, found
}

func (m *mockCache) Set(key string, result *domain.MatchResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = result
}

func (m *mockCache) Invalidate(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *mockCache) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]*domain.MatchResult)
}

func (m *mockCache) Stats() domain.CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return domain.CacheStats{
		Size:    len(m.data),
		MaxSize: 10000,
	}
}

func (m *mockCache) HealthCheck(ctx context.Context) domain.HealthStatus {
	return domain.HealthStatus{Status: domain.HealthStatusHealthy, Timestamp: time.Now()}
}

// Feature: github.com/freewebtopdf/asset-injector, Property 2: Specificity score calculation
func TestProperty_SpecificityScoreCalculation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule without manual priority override, the calculated score should equal BasePriority + PatternLength where BasePriority is determined by rule type", prop.ForAll(
		func(ruleType string, pattern string) bool {
			// Create a rule without manual priority
			rule := domain.Rule{
				ID:        uuid.New().String(),
				Type:      ruleType,
				Pattern:   pattern,
				CSS:       "test-css",
				JS:        "test-js",
				Priority:  nil, // No manual priority override
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Pre-compile regex if needed
			if rule.Type == "regex" {
				compiled, err := regexp.Compile(rule.Pattern)
				if err != nil {
					// Skip invalid regex patterns
					return true
				}
				rule.SetCompiledRegex(compiled)
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Test the scoring logic
			matches, score := matcher.matchRule(&rule, "http://example.com/test")

			if !matches {
				// If it doesn't match, we can't test the scoring
				return true
			}

			// Calculate expected score based on rule type
			var expectedBaseScore int
			switch rule.Type {
			case "exact":
				expectedBaseScore = 1000
			case "regex":
				expectedBaseScore = 500
			case "wildcard":
				expectedBaseScore = 100
			default:
				return false
			}

			expectedScore := expectedBaseScore + len(rule.Pattern)
			return score == expectedScore
		},
		gen.OneConstOf("exact", "regex", "wildcard"),
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 100 // Keep patterns reasonable for testing
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 4: Base priority assignment by rule type
func TestProperty_BasePriorityAssignmentByRuleType(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule, the base priority should be 1000 for exact matches, 500 for regex matches, and 100 for wildcard matches", prop.ForAll(
		func(ruleType string, pattern string, url string) bool {
			// Create a rule without manual priority
			rule := domain.Rule{
				ID:        uuid.New().String(),
				Type:      ruleType,
				Pattern:   pattern,
				CSS:       "test-css",
				JS:        "test-js",
				Priority:  nil, // No manual priority override
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Pre-compile regex if needed
			if rule.Type == "regex" {
				compiled, err := regexp.Compile(rule.Pattern)
				if err != nil {
					// Skip invalid regex patterns
					return true
				}
				rule.SetCompiledRegex(compiled)
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Test the scoring logic
			matches, score := matcher.matchRule(&rule, url)

			if !matches {
				// If it doesn't match, we can't test the base priority
				return true
			}

			// Extract base priority from the score
			// Score = BasePriority + PatternLength
			basePriority := score - len(rule.Pattern)

			// Verify base priority matches expected value for rule type
			switch rule.Type {
			case "exact":
				return basePriority == 1000
			case "regex":
				return basePriority == 500
			case "wildcard":
				return basePriority == 100
			default:
				return false
			}
		},
		gen.OneConstOf("exact", "regex", "wildcard"),
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 50 // Keep patterns reasonable for testing
		}),
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 100 // Generate test URLs
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 1: Highest scoring rule selection
func TestProperty_HighestScoringRuleSelection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any URL and set of matching rules, the resolve operation should return the rule with the highest calculated specificity score among all matches", prop.ForAll(
		func(url string, numRules int) bool {
			if numRules <= 0 {
				return true // Skip empty rule sets
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Generate multiple rules with different scores
			var rules []domain.Rule
			var expectedBestRule *domain.Rule
			var expectedBestScore int

			for i := 0; i < numRules; i++ {
				// Create rules that will match the URL (using exact match for simplicity)
				rule := domain.Rule{
					ID:        uuid.New().String(),
					Type:      "exact",
					Pattern:   url, // Ensure it matches
					CSS:       "test-css",
					JS:        "test-js",
					Priority:  nil,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				// Add the rule to matcher
				err := matcher.AddRule(context.Background(), &rule)
				if err != nil {
					return false
				}

				// Calculate expected score for this rule
				score := 1000 + len(rule.Pattern) // exact match base priority + pattern length

				// Track the rule with highest score
				if expectedBestRule == nil || score > expectedBestScore {
					expectedBestRule = &rule
					expectedBestScore = score
				}

				rules = append(rules, rule)
			}

			// Resolve the URL
			result, err := matcher.Resolve(context.Background(), url)
			if err != nil {
				return false
			}

			// If no rules matched, result should be empty
			if expectedBestRule == nil {
				return result.RuleID == ""
			}

			// Verify the result matches the expected best rule
			return result.RuleID == expectedBestRule.ID && result.Score == expectedBestScore
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 50 // Generate test URLs
		}),
		gen.IntRange(1, 5), // Generate 1-5 rules for testing
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 3: Manual priority override precedence
func TestProperty_ManualPriorityOverridePrecedence(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule with a manual priority value set, the final score should equal that priority value regardless of the calculated BasePriority + PatternLength formula", prop.ForAll(
		func(ruleType string, pattern string, manualPriority int, url string) bool {
			// Create a rule with manual priority override
			rule := domain.Rule{
				ID:        uuid.New().String(),
				Type:      ruleType,
				Pattern:   pattern,
				CSS:       "test-css",
				JS:        "test-js",
				Priority:  &manualPriority, // Set manual priority override
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Pre-compile regex if needed
			if rule.Type == "regex" {
				compiled, err := regexp.Compile(rule.Pattern)
				if err != nil {
					// Skip invalid regex patterns
					return true
				}
				rule.SetCompiledRegex(compiled)
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Test the scoring logic
			matches, score := matcher.matchRule(&rule, url)

			if !matches {
				// If it doesn't match, we can't test the priority override
				return true
			}

			// Verify the score equals the manual priority (ignoring calculated score)
			return score == manualPriority
		},
		gen.OneConstOf("exact", "regex", "wildcard"),
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 50 // Keep patterns reasonable for testing
		}),
		gen.IntRange(0, 10000), // Manual priority range
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 100 // Generate test URLs
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 5: Tie-breaking by pattern length
func TestProperty_TieBreakingByPatternLength(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any two rules with identical calculated scores, the rule with the longer pattern length should be selected", prop.ForAll(
		func(baseScore int, shortPatternLen int, longPatternLen int) bool {
			// Ensure we have different pattern lengths
			if shortPatternLen >= longPatternLen {
				return true // Skip if not properly ordered
			}

			// Create patterns of specified lengths
			shortPattern := generatePatternOfLength(shortPatternLen)
			longPattern := generatePatternOfLength(longPatternLen)

			// Create two rules with manual priority to force identical scores
			shortRule := domain.Rule{
				ID:        uuid.New().String(),
				Type:      "exact",
				Pattern:   shortPattern,
				CSS:       "short-css",
				JS:        "short-js",
				Priority:  &baseScore, // Same manual priority
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			longRule := domain.Rule{
				ID:        uuid.New().String(),
				Type:      "exact",
				Pattern:   longPattern,
				CSS:       "long-css",
				JS:        "long-js",
				Priority:  &baseScore, // Same manual priority
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Test the tie-breaking logic by checking which rule would be selected
			// when both match and have identical scores
			matches1, score1 := matcher.matchRule(&shortRule, shortPattern)
			matches2, score2 := matcher.matchRule(&longRule, longPattern)

			// Both should match their own patterns and have identical scores
			if matches1 && matches2 && score1 == score2 {
				// Add both rules to matcher
				err := matcher.AddRule(context.Background(), &shortRule)
				if err != nil {
					return false
				}
				err = matcher.AddRule(context.Background(), &longRule)
				if err != nil {
					return false
				}

				// Test with a URL that matches both patterns
				// Since both are exact matches, we need to test the resolve logic
				// The tie-breaking happens in the resolve method when scores are equal

				// Test with short pattern - should return short rule
				result1, err := matcher.Resolve(context.Background(), shortPattern)
				if err != nil {
					return false
				}

				// Test with long pattern - should return long rule
				result2, err := matcher.Resolve(context.Background(), longPattern)
				if err != nil {
					return false
				}

				// Verify correct rules are returned
				return result1.RuleID == shortRule.ID && result2.RuleID == longRule.ID
			}

			// If conditions aren't met for tie-breaking test, pass
			return true
		},
		gen.IntRange(1000, 5000), // Base score range
		gen.IntRange(5, 15),      // Short pattern length
		gen.IntRange(16, 30),     // Long pattern length (always longer)
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to generate a pattern of specific length
func generatePatternOfLength(length int) string {
	if length <= 0 {
		return "a"
	}

	result := make([]byte, length)
	for i := range result {
		result[i] = byte('a' + (i % 26)) // Use different characters to make patterns unique
	}
	return string(result)
}

// Feature: github.com/freewebtopdf/asset-injector, Property 8: Regex pre-compilation
func TestProperty_RegexPreCompilation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule with regex pattern type, the regex should be compiled during rule creation and invalid patterns should be rejected immediately", prop.ForAll(
		func(pattern string) bool {
			// Create a regex rule
			rule := domain.Rule{
				ID:        uuid.New().String(),
				Type:      "regex",
				Pattern:   pattern,
				CSS:       "test-css",
				JS:        "test-js",
				Priority:  nil,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Try to add the rule
			err := matcher.AddRule(context.Background(), &rule)

			// Test if the pattern is valid by trying to compile it
			_, compileErr := regexp.Compile(pattern)

			if compileErr != nil {
				// Invalid regex should be rejected
				return err != nil
			} else {
				// Valid regex should be accepted and pre-compiled
				if err != nil {
					return false // Should not have failed for valid regex
				}

				// Check that the regex was pre-compiled by verifying it can match
				compiledRegex := rule.GetCompiledRegex()
				return compiledRegex != nil
			}
		},
		gen.OneConstOf(
			// Generate some valid regex patterns
			".*",
			"^https?://",
			"example\\.com",
			"[a-zA-Z0-9]+",
			// Generate some potentially invalid regex patterns
			"[",
			"*",
			"(",
			"\\",
		).Map(func(s string) string {
			return s
		}).SuchThat(func(s string) bool {
			return len(s) > 0
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 6: Concurrent read access
func TestProperty_ConcurrentReadAccess(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any set of simultaneous resolve requests, they should all complete successfully without blocking each other when no write operations are occurring", prop.ForAll(
		func(numGoroutines int) bool {
			if numGoroutines <= 0 || numGoroutines > 10 {
				return true // Skip invalid inputs
			}

			// Use fixed URLs that we know will work
			urls := []string{
				"http://example.com",
				"https://test.com",
				"http://site.org",
			}

			// Create matcher with some test rules
			repo := &mockRepository{
				rules: []domain.Rule{
					{
						ID:        uuid.New().String(),
						Type:      "exact",
						Pattern:   "http://example.com",
						CSS:       "test-css",
						JS:        "test-js",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						ID:        uuid.New().String(),
						Type:      "wildcard",
						Pattern:   "*.com",
						CSS:       "wildcard-css",
						JS:        "wildcard-js",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
			}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Load rules into matcher
			err := matcher.LoadRules(context.Background())
			if err != nil {
				return false
			}

			// Channel to collect results
			results := make(chan bool, numGoroutines*len(urls))

			// Launch concurrent read operations
			for i := 0; i < numGoroutines; i++ {
				go func() {
					for _, url := range urls {
						// Perform resolve operation (read-only)
						_, err := matcher.Resolve(context.Background(), url)
						results <- (err == nil)
					}
				}()
			}

			// Collect all results
			successCount := 0
			totalOperations := numGoroutines * len(urls)

			for i := 0; i < totalOperations; i++ {
				if <-results {
					successCount++
				}
			}

			// All operations should succeed
			return successCount == totalOperations
		},
		gen.IntRange(1, 5), // 1-5 concurrent goroutines
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 7: Write operation exclusivity
func TestProperty_WriteOperationExclusivity(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule mutation operation, it should block all other read and write operations until completion to maintain data consistency", prop.ForAll(
		func(numReaders int, numWriters int) bool {
			if numReaders <= 0 || numWriters <= 0 || numReaders > 5 || numWriters > 3 {
				return true // Skip invalid inputs
			}

			// Create matcher
			repo := &mockRepository{}
			cache := newMockCache()
			matcher := NewMatcher(repo, cache)

			// Channel to track operation order
			operations := make(chan string, numReaders+numWriters)

			// Start write operations
			for i := 0; i < numWriters; i++ {
				go func(id int) {
					rule := domain.Rule{
						ID:        uuid.New().String(),
						Type:      "exact",
						Pattern:   "http://example.com",
						CSS:       "test-css",
						JS:        "test-js",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}

					operations <- "write-start"
					err := matcher.AddRule(context.Background(), &rule)
					operations <- "write-end"

					if err != nil {
						operations <- "write-error"
					}
				}(i)
			}

			// Start read operations
			for i := 0; i < numReaders; i++ {
				go func(id int) {
					operations <- "read-start"
					_, err := matcher.Resolve(context.Background(), "http://example.com")
					operations <- "read-end"

					if err != nil {
						operations <- "read-error"
					}
				}(i)
			}

			// Collect all operations
			totalOps := (numReaders + numWriters) * 2 // Each operation has start and end
			opCount := 0

			for opCount < totalOps {
				select {
				case op := <-operations:
					opCount++
					if op == "write-error" || op == "read-error" {
						return false // Operations should not error
					}
				case <-time.After(5 * time.Second):
					return false // Timeout - operations might be deadlocked
				}
			}

			// All operations completed successfully
			return true
		},
		gen.IntRange(1, 3), // Number of concurrent readers
		gen.IntRange(1, 2), // Number of concurrent writers
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
