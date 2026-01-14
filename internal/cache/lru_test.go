package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache(100)
	assert.Equal(t, 100, cache.maxSize)
	assert.Equal(t, 0, cache.size)
	assert.NotNil(t, cache.cache)
	assert.NotNil(t, cache.head)
	assert.NotNil(t, cache.tail)
	assert.Equal(t, cache.tail, cache.head.next)
	assert.Equal(t, cache.head, cache.tail.prev)
}

func TestNewLRUCache_DefaultSize(t *testing.T) {
	cache := NewLRUCache(0)
	assert.Equal(t, 10000, cache.maxSize)
}

func TestLRUCache_SetAndGet(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{
		RuleID:    "rule1",
		CSS:       "body { color: red; }",
		JS:        "console.log('test');",
		Timestamp: time.Now(),
	}

	// Test cache miss
	value, found := cache.Get("key1")
	assert.False(t, found)
	assert.Nil(t, value)

	// Test set and get
	cache.Set("key1", result1)
	value, found = cache.Get("key1")
	assert.True(t, found)
	// When retrieved from cache, CacheHit should be true
	assert.Equal(t, result1.RuleID, value.RuleID)
	assert.Equal(t, result1.CSS, value.CSS)
	assert.Equal(t, result1.JS, value.JS)
	assert.True(t, value.CacheHit) // Cache hit should be true when retrieved from cache
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{RuleID: "rule1", CSS: "css1"}
	result2 := &domain.MatchResult{RuleID: "rule2", CSS: "css2"}
	result3 := &domain.MatchResult{RuleID: "rule3", CSS: "css3"}

	// Fill cache to capacity
	cache.Set("key1", result1)
	cache.Set("key2", result2)

	// Verify both items are in cache
	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	assert.True(t, found1)
	assert.True(t, found2)

	// Add third item, should evict least recently used (key1)
	cache.Set("key3", result3)

	// key1 should be evicted, key2 and key3 should remain
	_, found1 = cache.Get("key1")
	_, found2 = cache.Get("key2")
	_, found3 := cache.Get("key3")

	assert.False(t, found1) // Evicted
	assert.True(t, found2)  // Still there
	assert.True(t, found3)  // Newly added
}

func TestLRUCache_LRUOrdering(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{RuleID: "rule1", CSS: "css1"}
	result2 := &domain.MatchResult{RuleID: "rule2", CSS: "css2"}
	result3 := &domain.MatchResult{RuleID: "rule3", CSS: "css3"}

	// Add two items
	cache.Set("key1", result1)
	cache.Set("key2", result2)

	// Access key1 to make it most recently used
	cache.Get("key1")

	// Add third item, should evict key2 (least recently used)
	cache.Set("key3", result3)

	// key2 should be evicted, key1 and key3 should remain
	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	_, found3 := cache.Get("key3")

	assert.True(t, found1)  // Most recently used, should remain
	assert.False(t, found2) // Least recently used, should be evicted
	assert.True(t, found3)  // Newly added
}

func TestLRUCache_Update(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{RuleID: "rule1", CSS: "css1"}
	result2 := &domain.MatchResult{RuleID: "rule1", CSS: "css2"}

	// Set initial value
	cache.Set("key1", result1)
	value, found := cache.Get("key1")
	require.True(t, found)
	assert.Equal(t, "css1", value.CSS)

	// Update value
	cache.Set("key1", result2)
	value, found = cache.Get("key1")
	require.True(t, found)
	assert.Equal(t, "css2", value.CSS)

	// Size should remain 1
	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
}

func TestLRUCache_Invalidate(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{RuleID: "rule1", CSS: "css1"}

	cache.Set("key1", result1)
	_, found := cache.Get("key1")
	assert.True(t, found)

	cache.Invalidate("key1")
	_, found = cache.Get("key1")
	assert.False(t, found)

	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{RuleID: "rule1", CSS: "css1"}
	result2 := &domain.MatchResult{RuleID: "rule2", CSS: "css2"}

	cache.Set("key1", result1)
	cache.Set("key2", result2)

	stats := cache.Stats()
	assert.Equal(t, 2, stats.Size)

	cache.Clear()

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)

	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	assert.False(t, found1)
	assert.False(t, found2)
}

func TestLRUCache_Stats(t *testing.T) {
	cache := NewLRUCache(2)

	result1 := &domain.MatchResult{RuleID: "rule1", CSS: "css1"}

	// Initial stats
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 2, stats.MaxSize)
	assert.Equal(t, float64(0), stats.HitRatio)

	// Cache miss
	cache.Get("key1")
	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, float64(0), stats.HitRatio)

	// Cache set and hit
	cache.Set("key1", result1)
	cache.Get("key1")
	stats = cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, float64(0.5), stats.HitRatio)
}

// Property 38: LRU cache size limits
// Feature: github.com/freewebtopdf/asset-injector, Property 38: LRU cache size limits
// Validates: Requirements 9.1, 9.2
func TestProperty_LRUCacheSizeLimits(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("cache never exceeds maximum size", prop.ForAll(
		func(maxSize int, numOperations int) bool {
			if maxSize <= 0 {
				maxSize = 1 // Minimum valid size
			}
			if numOperations < 0 {
				numOperations = 0
			}

			cache := NewLRUCache(maxSize)

			// Perform random set operations
			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("key%d", i)
				result := &domain.MatchResult{
					RuleID: fmt.Sprintf("rule%d", i),
					CSS:    fmt.Sprintf("css%d", i),
				}
				cache.Set(key, result)

				// Check that cache size never exceeds maxSize
				stats := cache.Stats()
				if stats.Size > maxSize {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 100), // maxSize
		gen.IntRange(0, 200), // numOperations
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 9: Cache hit consistency
// Feature: github.com/freewebtopdf/asset-injector, Property 9: Cache hit consistency
// Validates: Requirements 2.6, 9.3
func TestProperty_CacheHitConsistency(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("cached results are consistent until invalidation", prop.ForAll(
		func(cacheSize int, keys []string) bool {
			if cacheSize <= 0 {
				cacheSize = 10
			}
			if len(keys) == 0 {
				return true // Empty test case
			}

			cache := NewLRUCache(cacheSize)

			// Store initial results for each key
			initialResults := make(map[string]*domain.MatchResult)
			for i, key := range keys {
				result := &domain.MatchResult{
					RuleID: fmt.Sprintf("rule%d", i),
					CSS:    fmt.Sprintf("css%d", i),
					JS:     fmt.Sprintf("js%d", i),
				}
				cache.Set(key, result)
				initialResults[key] = result
			}

			// Verify that subsequent gets return the same results
			for _, key := range keys {
				cachedResult, found := cache.Get(key)
				if !found {
					// Key might have been evicted due to cache size limits
					continue
				}

				expectedResult := initialResults[key]
				if cachedResult.RuleID != expectedResult.RuleID ||
					cachedResult.CSS != expectedResult.CSS ||
					cachedResult.JS != expectedResult.JS {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),                 // cacheSize
		gen.SliceOfN(10, gen.AlphaString()), // keys
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 39: Cache miss handling
// Feature: github.com/freewebtopdf/asset-injector, Property 39: Cache miss handling
// Validates: Requirements 9.4
func TestProperty_CacheMissHandling(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("cache misses return false and increment miss counter", prop.ForAll(
		func(cacheSize int, missKeys []string, hitKeys []string) bool {
			if cacheSize <= 0 {
				cacheSize = 10
			}

			cache := NewLRUCache(cacheSize)

			// Store some results for hit keys
			for i, key := range hitKeys {
				result := &domain.MatchResult{
					RuleID: fmt.Sprintf("rule%d", i),
					CSS:    fmt.Sprintf("css%d", i),
				}
				cache.Set(key, result)
			}

			initialStats := cache.Stats()

			// Test cache misses
			missCount := 0
			for _, key := range missKeys {
				// Only test keys that weren't stored
				found := false
				for _, hitKey := range hitKeys {
					if key == hitKey {
						found = true
						break
					}
				}
				if found {
					continue // Skip keys that should hit
				}

				result, hit := cache.Get(key)
				if hit || result != nil {
					return false // Should be a miss
				}
				missCount++
			}

			finalStats := cache.Stats()
			expectedMisses := initialStats.Misses + int64(missCount)

			return finalStats.Misses == expectedMisses
		},
		gen.IntRange(1, 20),                // cacheSize
		gen.SliceOfN(5, gen.AlphaString()), // missKeys
		gen.SliceOfN(3, gen.AlphaString()), // hitKeys
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 40: Cache invalidation on rule changes
// Feature: github.com/freewebtopdf/asset-injector, Property 40: Cache invalidation on rule changes
// Validates: Requirements 9.5
func TestProperty_CacheInvalidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("invalidated keys are no longer found in cache", prop.ForAll(
		func(cacheSize int, keys []string, invalidateKeys []string) bool {
			if cacheSize <= 0 {
				cacheSize = 10
			}
			if len(keys) == 0 {
				return true // Empty test case
			}

			cache := NewLRUCache(cacheSize)

			// Store results for all keys
			for i, key := range keys {
				result := &domain.MatchResult{
					RuleID: fmt.Sprintf("rule%d", i),
					CSS:    fmt.Sprintf("css%d", i),
				}
				cache.Set(key, result)
			}

			// Invalidate specific keys
			invalidatedSet := make(map[string]bool)
			for _, key := range invalidateKeys {
				cache.Invalidate(key)
				invalidatedSet[key] = true
			}

			// Verify invalidated keys are no longer found
			for _, key := range keys {
				_, found := cache.Get(key)

				if invalidatedSet[key] {
					// This key should have been invalidated
					if found {
						return false
					}
				} else {
					// This key should still be there (unless evicted by LRU)
					// We can't guarantee it's still there due to LRU eviction,
					// but if it is there, it should be valid
					// This is acceptable behavior
				}
			}

			return true
		},
		gen.IntRange(1, 20),                 // cacheSize
		gen.SliceOfN(10, gen.AlphaString()), // keys
		gen.SliceOfN(5, gen.AlphaString()),  // invalidateKeys
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 33: Cache metrics tracking
func TestProperty_CacheMetricsTracking(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any cache operation (hit or miss), the corresponding atomic counters should be incremented accurately", prop.ForAll(
		func(cacheSize int, operations []string) bool {
			if cacheSize <= 0 || len(operations) == 0 {
				return true // Skip invalid inputs
			}

			cache := NewLRUCache(cacheSize)

			// Track expected hits and misses
			expectedHits := int64(0)
			expectedMisses := int64(0)

			// Perform operations and track expected metrics
			for i, op := range operations {
				key := fmt.Sprintf("key-%d", i%10) // Use limited key set to create hits

				switch op {
				case "get":
					// Get operation - will be hit or miss
					_, found := cache.Get(key)
					if found {
						expectedHits++
					} else {
						expectedMisses++
					}

				case "set":
					// Set operation followed by get to create hit
					result := &domain.MatchResult{
						RuleID:    fmt.Sprintf("rule-%d", i),
						CSS:       "test-css",
						JS:        "test-js",
						Timestamp: time.Now(),
					}
					cache.Set(key, result)

					// Now get it to create a hit
					_, found := cache.Get(key)
					if found {
						expectedHits++
					} else {
						expectedMisses++
					}
				}
			}

			// Check that metrics match expected values
			stats := cache.Stats()
			return stats.Hits == expectedHits && stats.Misses == expectedMisses
		},
		gen.IntRange(1, 100),                           // Cache size
		gen.SliceOfN(20, gen.OneConstOf("get", "set")), // Operations
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
