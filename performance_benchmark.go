package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/api"
	"github.com/freewebtopdf/asset-injector/internal/cache"
	"github.com/freewebtopdf/asset-injector/internal/config"
	"github.com/freewebtopdf/asset-injector/internal/domain"
	"github.com/freewebtopdf/asset-injector/internal/health"
	"github.com/freewebtopdf/asset-injector/internal/matcher"
	"github.com/freewebtopdf/asset-injector/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	// Initialize components
	store := storage.NewStore(cfg.Storage.DataDir)
	lruCache := cache.NewLRUCache(cfg.Cache.MaxSize)
	patternMatcher := matcher.NewMatcher(store, lruCache)
	validator := domain.NewValidator()
	healthChecker := health.NewSystemHealthChecker(store, patternMatcher, lruCache)

	// Create test rules for performance testing
	testRules := []*domain.Rule{
		{
			ID:      "test-1",
			Type:    "exact",
			Pattern: "https://example.com/test1",
			CSS:     "body { background: red; }",
			JS:      "console.log('test1');",
		},
		{
			ID:      "test-2",
			Type:    "regex",
			Pattern: "https://.*\\.example\\.com/.*",
			CSS:     "body { background: blue; }",
			JS:      "console.log('test2');",
		},
		{
			ID:      "test-3",
			Type:    "wildcard",
			Pattern: "https://test.com/*",
			CSS:     "body { background: green; }",
			JS:      "console.log('test3');",
		},
	}

	// Add test rules
	ctx := context.Background()
	for _, rule := range testRules {
		if err := store.CreateRule(ctx, rule); err != nil {
			fmt.Printf("Failed to create rule: %v\n", err)
			return
		}
		if err := patternMatcher.AddRule(ctx, rule); err != nil {
			fmt.Printf("Failed to add rule to matcher: %v\n", err)
			return
		}
	}

	// Start HTTP server
	routerConfig := api.RouterConfig{
		CORSOrigins: cfg.Security.CORSOrigins,
		BodyLimit:   cfg.Server.BodyLimit,
	}
	app := api.SetupRouter(patternMatcher, store, lruCache, validator, healthChecker, routerConfig)

	go func() {
		if err := app.Listen(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
			fmt.Printf("Server failed: %v\n", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Pre-warm the cache with a few requests to get more realistic performance
	fmt.Printf("Pre-warming cache...\n")
	client := &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	testURLs := []string{
		"https://example.com/test1",
		"https://sub.example.com/page",
		"https://test.com/page",
		"https://other.com/page",
	}

	for _, url := range testURLs {
		payload := map[string]string{"url": url}
		jsonPayload, _ := json.Marshal(payload)
		resp, err := client.Post(
			fmt.Sprintf("http://localhost:%d/v1/resolve", cfg.Server.Port),
			"application/json",
			bytes.NewBuffer(jsonPayload),
		)
		if err == nil {
			_ = resp.Body.Close()
		}
	}

	// Performance test parameters
	const (
		numConcurrentRequests = 50 // Reduced to avoid overwhelming the server
		numRequestsPerWorker  = 20 // Increased per worker
		totalRequests         = numConcurrentRequests * numRequestsPerWorker
	)

	fmt.Printf("Starting performance test with %d concurrent workers, %d requests each (%d total)\n",
		numConcurrentRequests, numRequestsPerWorker, totalRequests)

	// Test URLs (reuse from pre-warming)
	// testURLs already defined above

	// Performance metrics
	var (
		successCount int64
		errorCount   int64
		totalLatency time.Duration
		maxLatency   time.Duration
		minLatency   = time.Hour // Initialize to a large value
		mu           sync.Mutex
	)

	startTime := time.Now()
	var wg sync.WaitGroup

	// Launch concurrent workers
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{
				Timeout: 1 * time.Second, // Reduced timeout
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					IdleConnTimeout:     30 * time.Second,
				},
			}

			for j := 0; j < numRequestsPerWorker; j++ {
				// Select test URL
				url := testURLs[j%len(testURLs)]

				// Create request payload
				payload := map[string]string{"url": url}
				jsonPayload, _ := json.Marshal(payload)

				// Measure request latency
				reqStart := time.Now()

				resp, err := client.Post(
					fmt.Sprintf("http://localhost:%d/v1/resolve", cfg.Server.Port),
					"application/json",
					bytes.NewBuffer(jsonPayload),
				)

				latency := time.Since(reqStart)

				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					successCount++
					_ = resp.Body.Close()

					totalLatency += latency
					if latency > maxLatency {
						maxLatency = latency
					}
					if latency < minLatency {
						minLatency = latency
					}
				}
				mu.Unlock()
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	totalTime := time.Since(startTime)

	// Calculate metrics
	avgLatency := time.Duration(0)
	if successCount > 0 {
		avgLatency = totalLatency / time.Duration(successCount)
	}

	requestsPerSecond := float64(totalRequests) / totalTime.Seconds()

	// Print results
	fmt.Printf("\n=== Performance Test Results ===\n")
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Total requests: %d\n", totalRequests)
	fmt.Printf("Successful requests: %d\n", successCount)
	fmt.Printf("Failed requests: %d\n", errorCount)
	fmt.Printf("Success rate: %.2f%%\n", float64(successCount)/float64(totalRequests)*100)
	fmt.Printf("Requests per second: %.2f\n", requestsPerSecond)
	fmt.Printf("Average latency: %v\n", avgLatency)
	fmt.Printf("Min latency: %v\n", minLatency)
	fmt.Printf("Max latency: %v\n", maxLatency)

	// Test cache performance
	fmt.Printf("\n=== Cache Performance Test ===\n")
	cacheTestStart := time.Now()

	// Make the same request multiple times to test cache hits
	cacheClient := &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	payload := map[string]string{"url": "https://example.com/test1"}
	jsonPayload, _ := json.Marshal(payload)

	var cacheHitLatencies []time.Duration
	for i := 0; i < 10; i++ {
		reqStart := time.Now()
		resp, err := cacheClient.Post(
			fmt.Sprintf("http://localhost:%d/v1/resolve", cfg.Server.Port),
			"application/json",
			bytes.NewBuffer(jsonPayload),
		)
		latency := time.Since(reqStart)

		if err == nil {
			cacheHitLatencies = append(cacheHitLatencies, latency)
			_ = resp.Body.Close()
		}
	}

	if len(cacheHitLatencies) > 0 {
		var totalCacheLatency time.Duration
		for _, lat := range cacheHitLatencies {
			totalCacheLatency += lat
		}
		avgCacheLatency := totalCacheLatency / time.Duration(len(cacheHitLatencies))
		fmt.Printf("Average cached request latency: %v\n", avgCacheLatency)

		// Check if cached requests are sub-millisecond (requirement 2.5)
		if avgCacheLatency < time.Millisecond {
			fmt.Printf("✓ Sub-millisecond cache performance achieved\n")
		} else {
			fmt.Printf("✗ Cache performance above 1ms threshold\n")
		}
	}

	cacheTestTime := time.Since(cacheTestStart)
	fmt.Printf("Cache test completed in: %v\n", cacheTestTime)

	// Get cache statistics
	stats := lruCache.Stats()
	fmt.Printf("Cache hits: %d\n", stats.Hits)
	fmt.Printf("Cache misses: %d\n", stats.Misses)
	if stats.Hits+stats.Misses > 0 {
		hitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
		fmt.Printf("Cache hit rate: %.2f%%\n", hitRate)
	}

	// Shutdown server
	if err := app.Shutdown(); err != nil {
		fmt.Printf("Server shutdown error: %v\n", err)
	}

	fmt.Printf("\n=== Performance Requirements Check ===\n")
	if requestsPerSecond >= 100 {
		fmt.Printf("✓ Concurrent request handling: %.2f RPS (target: >100)\n", requestsPerSecond)
	} else {
		fmt.Printf("✗ Concurrent request handling: %.2f RPS (target: >100)\n", requestsPerSecond)
	}

	if avgLatency < 10*time.Millisecond {
		fmt.Printf("✓ Average response time: %v (target: <10ms)\n", avgLatency)
	} else {
		fmt.Printf("✗ Average response time: %v (target: <10ms)\n", avgLatency)
	}

	fmt.Printf("Performance test completed successfully\n")
}
