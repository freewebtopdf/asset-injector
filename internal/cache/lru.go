package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// node represents a node in the doubly-linked list
type node struct {
	key   string
	value *domain.MatchResult
	prev  *node
	next  *node
}

// LRUCache implements the CacheManager interface using LRU eviction policy
type LRUCache struct {
	maxSize int
	size    int

	// Doubly-linked list for LRU ordering
	head *node
	tail *node

	// HashMap for O(1) lookups
	cache map[string]*node

	// Thread safety
	mutex sync.RWMutex

	// Atomic counters for metrics
	hits   int64
	misses int64

	// Health monitoring
	lastHealthCheck time.Time
	healthMutex     sync.RWMutex
}

// NewLRUCache creates a new LRU cache with the specified maximum size
func NewLRUCache(maxSize int) *LRUCache {
	if maxSize <= 0 {
		maxSize = 10000 // Default size from requirements
	}

	// Create dummy head and tail nodes for easier list manipulation
	head := &node{}
	tail := &node{}
	head.next = tail
	tail.prev = head

	return &LRUCache{
		maxSize:         maxSize,
		size:            0,
		head:            head,
		tail:            tail,
		cache:           make(map[string]*node),
		lastHealthCheck: time.Now(),
	}
}

// Get retrieves a value from the cache and marks it as recently used
func (c *LRUCache) Get(key string) (*domain.MatchResult, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	foundNode, exists := c.cache[key]
	if !exists {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// Move to front (most recently used)
	c.moveToFront(foundNode)
	atomic.AddInt64(&c.hits, 1)

	// Create a copy to avoid race conditions on shared cached objects
	result := &domain.MatchResult{
		RuleID:    foundNode.value.RuleID,
		CSS:       foundNode.value.CSS,
		JS:        foundNode.value.JS,
		Score:     foundNode.value.Score,
		CacheHit:  true,
		Timestamp: time.Now(),
	}
	return result, true
}

// Set adds or updates a value in the cache
func (c *LRUCache) Set(key string, result *domain.MatchResult) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if node, exists := c.cache[key]; exists {
		// Update existing node with a copy
		node.value = &domain.MatchResult{
			RuleID:    result.RuleID,
			CSS:       result.CSS,
			JS:        result.JS,
			Score:     result.Score,
			CacheHit:  result.CacheHit,
			Timestamp: result.Timestamp,
		}
		c.moveToFront(node)
		return
	}

	// Create new node with a copy of the result
	newNode := &node{
		key: key,
		value: &domain.MatchResult{
			RuleID:    result.RuleID,
			CSS:       result.CSS,
			JS:        result.JS,
			Score:     result.Score,
			CacheHit:  result.CacheHit,
			Timestamp: result.Timestamp,
		},
	}

	// Add to front of list
	c.addToFront(newNode)
	c.cache[key] = newNode
	c.size++

	// Check if we need to evict
	if c.size > c.maxSize {
		c.evictLRU()
	}
}

// Invalidate removes a specific key from the cache
func (c *LRUCache) Invalidate(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if node, exists := c.cache[key]; exists {
		c.removeNode(node)
		delete(c.cache, key)
		c.size--
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Reset the doubly-linked list
	c.head.next = c.tail
	c.tail.prev = c.head

	// Clear the hashmap
	c.cache = make(map[string]*node)
	c.size = 0

	// Reset counters
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

// Stats returns current cache statistics
func (c *LRUCache) Stats() domain.CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	return domain.CacheStats{
		Hits:     hits,
		Misses:   misses,
		Size:     c.size,
		MaxSize:  c.maxSize,
		HitRatio: hitRatio,
	}
}

// HealthCheck performs a health check on the cache
func (c *LRUCache) HealthCheck(ctx context.Context) domain.HealthStatus {
	c.healthMutex.Lock()
	defer c.healthMutex.Unlock()

	now := time.Now()
	c.lastHealthCheck = now

	stats := c.Stats()

	status := "healthy"
	message := "Cache is operating normally"
	details := map[string]any{
		"size":      stats.Size,
		"max_size":  stats.MaxSize,
		"hit_ratio": stats.HitRatio,
		"hits":      stats.Hits,
		"misses":    stats.Misses,
	}

	// Check for potential issues
	if stats.Size >= int(float64(stats.MaxSize)*0.9) {
		status = "degraded"
		message = "Cache is near capacity"
		details["warning"] = "Cache utilization above 90%"
	}

	if stats.HitRatio < 0.5 && stats.Hits+stats.Misses > 100 {
		if status == "healthy" {
			status = "degraded"
			message = "Low cache hit ratio"
		}
		details["hit_ratio_warning"] = "Hit ratio below 50%"
	}

	return domain.HealthStatus{
		Status:    status,
		Message:   message,
		Details:   details,
		Timestamp: now,
	}
}

// moveToFront moves a node to the front of the list (most recently used)
func (c *LRUCache) moveToFront(node *node) {
	c.removeNode(node)
	c.addToFront(node)
}

// addToFront adds a node to the front of the list
func (c *LRUCache) addToFront(node *node) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode removes a node from the list
func (c *LRUCache) removeNode(node *node) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// evictLRU removes the least recently used item from the cache
func (c *LRUCache) evictLRU() {
	if c.tail.prev == c.head {
		return // Empty cache
	}

	lru := c.tail.prev
	c.removeNode(lru)
	delete(c.cache, lru.key)
	c.size--
}
