package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// TestAPIEndpointsIntegration tests all API endpoints work correctly
func TestAPIEndpointsIntegration(t *testing.T) {
	// Setup mocks
	mockMatcher := new(MockPatternMatcher)
	mockRepo := new(MockRuleRepository)
	mockCache := new(MockCacheManager)
	mockValidator := new(MockValidator)
	mockHealthChecker := new(MockHealthChecker)

	// Configure mocks for successful operations
	mockMatcher.On("Resolve", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	mockValidator.On("ValidateURL", mock.AnythingOfType("string")).Return(nil)
	mockRepo.On("GetAllRules", mock.Anything).Return([]domain.Rule{}, nil)
	mockHealthChecker.On("CheckHealth", mock.Anything).Return(domain.SystemHealth{
		Status: domain.HealthStatusHealthy,
		Components: map[string]domain.HealthStatus{
			"matcher": {Status: domain.HealthStatusHealthy, Timestamp: time.Now()},
			"storage": {Status: domain.HealthStatusHealthy, Timestamp: time.Now()},
			"cache":   {Status: domain.HealthStatusHealthy, Timestamp: time.Now()},
		},
		Timestamp: time.Now(),
	})
	mockCache.On("Stats").Return(domain.CacheStats{
		Hits:     100,
		Misses:   50,
		Size:     10,
		MaxSize:  1000,
		HitRatio: 0.67,
	})

	// Create app with longer timeout to avoid timeout issues in tests
	config := RouterConfig{
		CORSOrigins: []string{},
		BodyLimit:   1048576,
	}
	app := SetupRouter(mockMatcher, mockRepo, mockCache, mockValidator, mockHealthChecker, config)

	t.Run("Health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req, 5000) // 5 second timeout

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
		assert.Contains(t, response, "timestamp")
	})

	t.Run("Metrics endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var response SuccessResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "success", response.Status)

		data, ok := response.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, data, "cache")
		assert.Contains(t, data, "rules")
		assert.Contains(t, data, "uptime")
	})

	t.Run("Resolve endpoint", func(t *testing.T) {
		reqBody := ResolveRequest{URL: "https://example.com"}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/v1/resolve", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var response SuccessResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "success", response.Status)

		data, ok := response.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, data, "css")
		assert.Contains(t, data, "js")
		assert.Contains(t, data, "cache_hit")
	})

	t.Run("Rules listing endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/rules", nil)
		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var response SuccessResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "success", response.Status)

		data, ok := response.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, data, "rules")
		assert.Contains(t, data, "count")
	})

	t.Run("Security headers are present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		// Check for required security headers
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
		assert.Contains(t, resp.Header.Get("Strict-Transport-Security"), "max-age=")
		assert.NotEmpty(t, resp.Header.Get("Referrer-Policy"))
		assert.NotEmpty(t, resp.Header.Get("Permissions-Policy"))
	})

	t.Run("Request ID is generated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		requestID := resp.Header.Get("X-Request-ID")
		assert.NotEmpty(t, requestID)
		assert.Len(t, requestID, 36)       // UUID format length
		assert.Contains(t, requestID, "-") // UUID contains hyphens
	})

	// Verify mocks were called
	mockMatcher.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

// TestConcurrentRequests tests that the API can handle concurrent requests
func TestConcurrentRequests(t *testing.T) {
	// Setup mocks
	mockMatcher := new(MockPatternMatcher)
	mockRepo := new(MockRuleRepository)
	mockCache := new(MockCacheManager)
	mockValidator := new(MockValidator)
	mockHealthChecker := new(MockHealthChecker)

	// Configure mocks for concurrent access
	mockMatcher.On("Resolve", mock.Anything, mock.AnythingOfType("string")).Return(nil, nil)
	mockValidator.On("ValidateURL", mock.AnythingOfType("string")).Return(nil)

	// Create app
	config := RouterConfig{
		CORSOrigins: []string{},
		BodyLimit:   1048576,
	}
	app := SetupRouter(mockMatcher, mockRepo, mockCache, mockValidator, mockHealthChecker, config)

	// Test concurrent requests
	const numRequests = 10
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			reqBody := ResolveRequest{URL: "https://example.com"}
			jsonBody, _ := json.Marshal(reqBody)
			req := httptest.NewRequest("POST", "/v1/resolve", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, 5000)
			if err != nil {
				results <- 0
				return
			}
			results <- resp.StatusCode
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		statusCode := <-results
		if statusCode == 200 {
			successCount++
		}
	}

	// All requests should succeed
	assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
}

// TestErrorHandling tests error responses
func TestErrorHandling(t *testing.T) {
	// Setup mocks
	mockMatcher := new(MockPatternMatcher)
	mockRepo := new(MockRuleRepository)
	mockCache := new(MockCacheManager)
	mockValidator := new(MockValidator)
	mockHealthChecker := new(MockHealthChecker)

	// Create app
	config := RouterConfig{
		CORSOrigins: []string{},
		BodyLimit:   1048576,
	}
	app := SetupRouter(mockMatcher, mockRepo, mockCache, mockValidator, mockHealthChecker, config)

	t.Run("Invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/resolve", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)

		var response ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "error", response.Status)
		assert.NotEmpty(t, response.Code)
		assert.NotEmpty(t, response.Message)
	})

	t.Run("Missing URL returns 422", func(t *testing.T) {
		// Configure validator to reject empty URL
		mockValidator.On("ValidateURL", "").Return(domain.NewAppError(domain.ErrValidationFailed, "URL is required", 422, nil))

		reqBody := ResolveRequest{URL: ""}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/v1/resolve", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 5000)

		assert.NoError(t, err)
		assert.Equal(t, 422, resp.StatusCode)

		var response ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "error", response.Status)
		assert.Equal(t, domain.ErrValidationFailed, response.Code)
	})
}
