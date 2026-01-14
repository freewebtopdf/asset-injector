package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"github.com/gofiber/fiber/v2"
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	capacity   int
	tokens     float64 // Use float for precise refill
	refillRate int     // tokens per second
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     float64(capacity),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed
func (tb *TokenBucket) Allow() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// Refill tokens based on elapsed time (fractional)
	tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*float64(tb.refillRate))
	tb.lastRefill = now

	// Check if we have tokens available
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// RateLimiter manages rate limiting for different endpoints
type RateLimiter struct {
	buckets map[string]*TokenBucket
	mutex   sync.RWMutex

	// Default limits
	defaultCapacity   int
	defaultRefillRate int

	// Per-endpoint limits
	endpointLimits map[string]struct {
		capacity   int
		refillRate int
	}
}

// NewRateLimiter creates a new rate limiter with configurable parameters
func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:           make(map[string]*TokenBucket),
		defaultCapacity:   burst,
		defaultRefillRate: rps,
		endpointLimits:    make(map[string]struct{ capacity, refillRate int }),
	}

	// Configure per-endpoint limits based on provided parameters
	rl.endpointLimits["/v1/resolve"] = struct{ capacity, refillRate int }{burst * 2, rps * 2} // Higher for resolve
	rl.endpointLimits["/v1/rules"] = struct{ capacity, refillRate int }{burst / 2, rps / 2}   // Lower for rules
	rl.endpointLimits["/health"] = struct{ capacity, refillRate int }{20, 2}                  // Very low for health
	rl.endpointLimits["/metrics"] = struct{ capacity, refillRate int }{20, 2}                 // Very low for metrics

	return rl
}

// getBucket gets or creates a token bucket for a client+endpoint combination
func (rl *RateLimiter) getBucket(clientID, endpoint string) *TokenBucket {
	key := clientID + ":" + endpoint

	rl.mutex.RLock()
	bucket, exists := rl.buckets[key]
	rl.mutex.RUnlock()

	if exists {
		return bucket
	}

	// Create new bucket
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists := rl.buckets[key]; exists {
		return bucket
	}

	// Get limits for this endpoint
	limits, exists := rl.endpointLimits[endpoint]
	if !exists {
		limits = struct{ capacity, refillRate int }{rl.defaultCapacity, rl.defaultRefillRate}
	}

	bucket = NewTokenBucket(limits.capacity, limits.refillRate)
	rl.buckets[key] = bucket

	return bucket
}

// getClientID extracts client identifier from request
func (rl *RateLimiter) getClientID(c *fiber.Ctx) string {
	// Try to get client ID from various sources

	// 1. API Key header
	if apiKey := c.Get("X-API-Key"); apiKey != "" {
		return "api:" + apiKey
	}

	// 2. Authorization header
	if auth := c.Get("Authorization"); auth != "" {
		return "auth:" + auth
	}

	// 3. Fall back to IP address
	return "ip:" + c.IP()
}

// Middleware returns a Fiber middleware for rate limiting
func (rl *RateLimiter) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		clientID := rl.getClientID(c)
		endpoint := c.Path()

		bucket := rl.getBucket(clientID, endpoint)

		if !bucket.Allow() {
			// Rate limit exceeded
			appErr := domain.NewAppError(
				domain.ErrRateLimit,
				"Rate limit exceeded",
				429,
				map[string]any{
					"client_id":   clientID,
					"endpoint":    endpoint,
					"retry_after": "60", // Suggest retry after 60 seconds
				},
			).WithContext(c.Context(), "rate_limit")

			c.Set("Retry-After", "60")
			c.Set("X-RateLimit-Limit", "100")
			c.Set("X-RateLimit-Remaining", "0")
			c.Set("X-RateLimit-Reset", time.Now().Add(time.Minute).Format(time.RFC3339))

			return c.Status(appErr.StatusCode).JSON(map[string]any{
				"status":  "error",
				"code":    appErr.Code,
				"message": appErr.Message,
				"details": appErr.Details,
			})
		}

		// Add rate limit headers for successful requests
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.defaultCapacity))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rl.defaultCapacity-1)) // Approximate

		return c.Next()
	}
}

// CleanupOldBuckets removes unused buckets to prevent memory leaks
func (rl *RateLimiter) CleanupOldBuckets() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	for key, bucket := range rl.buckets {
		bucket.mutex.Lock()
		idle := now.Sub(bucket.lastRefill)
		bucket.mutex.Unlock()
		// Remove buckets that haven't been used in the last hour
		if idle > time.Hour {
			delete(rl.buckets, key)
		}
	}
}

// StartCleanupRoutine starts a background routine to clean up old buckets
// Returns a stop function to cancel the routine
func (rl *RateLimiter) StartCleanupRoutine() (stop func()) {
	ticker := time.NewTicker(10 * time.Minute)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				rl.CleanupOldBuckets()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() { close(done) }
}

// GetStats returns rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]any {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	stats := map[string]any{
		"active_buckets":      len(rl.buckets),
		"default_capacity":    rl.defaultCapacity,
		"default_refill_rate": rl.defaultRefillRate,
		"endpoint_limits":     rl.endpointLimits,
	}

	return stats
}
