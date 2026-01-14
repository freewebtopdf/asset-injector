package api

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/freewebtopdf/asset-injector/internal/domain"
	"github.com/freewebtopdf/asset-injector/internal/middleware"
)

// RouterConfig contains configuration for the HTTP router
type RouterConfig struct {
	CORSOrigins    []string
	BodyLimit      int
	RateLimitRPS   int
	RateLimitBurst int
}

// RouterDependencies contains all dependencies needed by the router
type RouterDependencies struct {
	Matcher       domain.PatternMatcher
	Repository    domain.RuleRepository
	Cache         domain.CacheManager
	Validator     domain.Validator
	HealthChecker domain.HealthChecker
	PackManager   PackManager
	RuleExporter  RuleExporter
}

// RouterResult contains the configured app and cleanup function
type RouterResult struct {
	App     *fiber.App
	Cleanup func()
}

// SetupRouter creates and configures the Fiber app with all routes and middleware
func SetupRouter(matcher domain.PatternMatcher, repository domain.RuleRepository, cache domain.CacheManager, validator domain.Validator, healthChecker domain.HealthChecker, config RouterConfig) *fiber.App {
	result := SetupRouterWithDeps(RouterDependencies{
		Matcher:       matcher,
		Repository:    repository,
		Cache:         cache,
		Validator:     validator,
		HealthChecker: healthChecker,
	}, config)
	return result.App
}

// SetupRouterWithDeps creates and configures the Fiber app with all dependencies
func SetupRouterWithDeps(deps RouterDependencies, config RouterConfig) *RouterResult {
	// Create Fiber app with custom config
	app := fiber.New(fiber.Config{
		BodyLimit:    config.BodyLimit,
		ErrorHandler: customErrorHandler,
	})

	// Create handlers
	handlers := NewHandlers(deps.Matcher, deps.Repository, deps.Cache, deps.Validator, deps.HealthChecker)
	packHandlers := NewPackHandlers(deps.PackManager, deps.Repository, deps.RuleExporter)

	// Middleware pipeline (order is critical)

	// 1. RequestID middleware for UUID generation
	app.Use(requestid.New(requestid.Config{
		Header: "X-Request-ID",
		Generator: func() string {
			return generateUUID()
		},
	}))

	// 2. Structured logging middleware with zerolog
	app.Use(structuredLoggingMiddleware())

	// 3. Panic recovery middleware with stack trace logging
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			requestID := ""
			if rid, ok := c.Locals("requestid").(string); ok {
				requestID = rid
			}
			log.Error().
				Str("request_id", requestID).
				Interface("panic", e).
				Str("method", c.Method()).
				Str("path", c.Path()).
				Str("ip", c.IP()).
				Msg("Panic recovered")
		},
	}))

	// 4. Security headers middleware (HSTS, XSS protection)
	app.Use(securityHeadersMiddleware())

	// 5. Rate limiting middleware (before CORS to limit all requests)
	var stopRateLimiter func()
	if config.RateLimitRPS > 0 {
		rateLimiter := middleware.NewRateLimiter(config.RateLimitRPS, config.RateLimitBurst)
		stopRateLimiter = rateLimiter.StartCleanupRoutine()
		app.Use(rateLimiter.Middleware())
	}

	// 6. CORS middleware with origin restrictions
	if len(config.CORSOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(config.CORSOrigins, ","),
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID",
			AllowCredentials: false,
			MaxAge:           86400, // 24 hours
		}))
	}

	// 7. Body limit middleware (1MB maximum) - handled by Fiber config above

	// API routes
	v1 := app.Group("/v1")

	// Resolve endpoint
	v1.Post("/resolve", handlers.ResolveHandler)

	// Rules endpoints
	v1.Get("/rules", handlers.ListRulesHandler)
	v1.Post("/rules", handlers.CreateRuleHandler)
	v1.Put("/rules/:id", handlers.UpdateRuleHandler)
	v1.Delete("/rules/:id", handlers.DeleteRuleHandler)
	v1.Get("/rules/:id/source", packHandlers.GetRuleSourceHandler)

	// Pack management endpoints
	v1.Get("/packs", packHandlers.ListInstalledPacksHandler)
	v1.Post("/packs/install", packHandlers.InstallPackHandler)
	v1.Delete("/packs/:name", packHandlers.UninstallPackHandler)

	// Community discovery endpoints
	v1.Get("/packs/available", packHandlers.ListAvailablePacksHandler)
	v1.Post("/packs/update", packHandlers.UpdatePacksHandler)

	// Export endpoint
	v1.Post("/rules/export", packHandlers.ExportRulesHandler)

	// Health and metrics endpoints
	app.Get("/health", handlers.HealthHandler)
	app.Get("/metrics", handlers.MetricsHandler)

	// Swagger documentation endpoint
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Build cleanup function
	cleanup := func() {
		if stopRateLimiter != nil {
			stopRateLimiter()
		}
	}

	return &RouterResult{App: app, Cleanup: cleanup}
}

// customErrorHandler handles Fiber framework errors
func customErrorHandler(c *fiber.Ctx, err error) error {
	// Default to 500 server error
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	// Check if it's a Fiber error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	// Map common Fiber errors to domain errors
	switch code {
	case fiber.StatusRequestEntityTooLarge:
		return c.Status(413).JSON(ErrorResponse{
			Status:  "error",
			Code:    domain.ErrTooLarge,
			Message: "Request payload too large",
		})
	case fiber.StatusBadRequest:
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Code:    domain.ErrInvalidInput,
			Message: message,
		})
	default:
		return c.Status(code).JSON(ErrorResponse{
			Status:  "error",
			Code:    domain.ErrInternal,
			Message: message,
		})
	}
}

// generateUUID generates a UUID v4 for request tracking
func generateUUID() string {
	return uuid.New().String()
}

// structuredLoggingMiddleware creates structured JSON logging middleware with zerolog
func structuredLoggingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Log request details
		requestID := "unknown"
		if rid, ok := c.Locals("requestid").(string); ok {
			requestID = rid
		}

		latency := time.Since(start)
		status := c.Response().StatusCode()

		logEvent := log.Info()
		if status >= 400 {
			logEvent = log.Error()
		}

		logEvent.
			Str("request_id", requestID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Dur("latency", latency).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent")).
			Int("body_size", len(c.Body())).
			Int("response_size", len(c.Response().Body())).
			Msg("HTTP request processed")

		return err
	}
}

// securityHeadersMiddleware adds security headers (HSTS, XSS protection)
func securityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Security headers
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		return c.Next()
	}
}
