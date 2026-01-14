package api

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// OverrideCreator defines the interface for creating override files
type OverrideCreator interface {
	CreateOverride(originalRule *domain.Rule, modifiedRule *domain.Rule, modifiedBy string) error
}

// Handlers contains all HTTP handlers for the Asset Injector API
type Handlers struct {
	matcher         domain.PatternMatcher
	repository      domain.RuleRepository
	cache           domain.CacheManager
	validator       domain.Validator
	healthChecker   domain.HealthChecker
	overrideCreator OverrideCreator
}

// NewHandlers creates a new instance of API handlers
func NewHandlers(matcher domain.PatternMatcher, repository domain.RuleRepository, cache domain.CacheManager, validator domain.Validator, healthChecker domain.HealthChecker) *Handlers {
	return &Handlers{
		matcher:       matcher,
		repository:    repository,
		cache:         cache,
		validator:     validator,
		healthChecker: healthChecker,
	}
}

// SetOverrideCreator sets the override creator for handling community rule modifications
func (h *Handlers) SetOverrideCreator(oc OverrideCreator) {
	h.overrideCreator = oc
}

// ResolveRequest represents the request payload for the resolve endpoint
// @Description Request payload for URL pattern resolution
type ResolveRequest struct {
	URL string `json:"url" validate:"required,url" example:"https://example.com/page"`
}

// ResolveResponse represents the response payload for the resolve endpoint
// @Description Response payload containing matched CSS/JS assets
type ResolveResponse struct {
	RuleID   string `json:"rule_id,omitempty" example:"123e4567-e89b-12d3-a456-426614174000"`
	CSS      string `json:"css" example:".banner { display: none; }"`
	JS       string `json:"js" example:"document.querySelector('.popup').remove();"`
	CacheHit bool   `json:"cache_hit" example:"false"`
}

// ErrorResponse represents the standard error response format
// @Description Standard error response format
type ErrorResponse struct {
	Status  string `json:"status" example:"error"`
	Code    string `json:"code" example:"VALIDATION_FAILED"`
	Message string `json:"message" example:"Invalid input provided"`
	Details any    `json:"details,omitempty"`
}

// SuccessResponse represents the standard success response format
// @Description Standard success response format
type SuccessResponse struct {
	Status string `json:"status" example:"success"`
	Data   any    `json:"data"`
}

// RuleListResponse represents the response for listing rules
// @Description Response containing list of rules
type RuleListResponse struct {
	Rules []domain.Rule `json:"rules"`
	Count int           `json:"count" example:"5"`
}

// HealthResponse represents the health check response
// @Description Health check response
type HealthResponse struct {
	Status    string `json:"status" example:"ok"`
	Timestamp string `json:"timestamp" example:"2023-01-01T12:00:00Z"`
}

// MetricsResponse represents the metrics response
// @Description System metrics response
type MetricsResponse struct {
	Cache struct {
		Hits     int64   `json:"hits" example:"1500"`
		Misses   int64   `json:"misses" example:"300"`
		Size     int     `json:"size" example:"800"`
		MaxSize  int     `json:"max_size" example:"10000"`
		HitRatio float64 `json:"hit_ratio" example:"0.83"`
	} `json:"cache"`
	Rules struct {
		Count int `json:"count" example:"25"`
	} `json:"rules"`
	Uptime struct {
		Timestamp string `json:"timestamp" example:"2023-01-01T12:00:00Z"`
	} `json:"uptime"`
}

// ResolveHandler handles POST /v1/resolve requests
// @Summary      Resolve URL pattern to CSS/JS assets
// @Description  Matches a URL against configured rules and returns the most specific CSS/JS assets
// @Tags         Resolution
// @Accept       json
// @Produce      json
// @Param        request body ResolveRequest true "URL to resolve"
// @Success      200 {object} SuccessResponse{data=ResolveResponse} "Successfully resolved URL"
// @Failure      400 {object} ErrorResponse "Invalid request payload"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/resolve [post]
func (h *Handlers) ResolveHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := ""
	if rid := c.Locals("requestid"); rid != nil {
		requestID = rid.(string)
	}

	var req ResolveRequest
	if err := c.BodyParser(&req); err != nil {
		appErr := domain.NewAppError(
			domain.ErrInvalidInput,
			"Invalid JSON payload",
			400,
			map[string]string{"error": err.Error()},
		).WithContext(ctx, "resolve_request_parsing")

		return h.sendError(c, appErr)
	}

	// Validate request using the validator
	req.URL = strings.TrimSpace(req.URL)
	if err := h.validator.ValidateURL(req.URL); err != nil {
		appErr := err.(*domain.AppError).WithContext(ctx, "resolve_request_validation")
		return h.sendError(c, appErr)
	}

	// Resolve the URL pattern
	result, err := h.matcher.Resolve(ctx, req.URL)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", req.URL).
			Str("request_id", requestID).
			Msg("Failed to resolve URL")

		appErr := domain.NewAppError(
			domain.ErrInternal,
			"Failed to resolve URL pattern",
			500,
			nil,
		).WithContext(ctx, "url_pattern_resolution")

		return h.sendError(c, appErr)
	}

	// Return empty response if no match found
	if result == nil {
		return c.Status(200).JSON(SuccessResponse{
			Status: "success",
			Data: map[string]any{
				"css":       "",
				"js":        "",
				"cache_hit": false,
			},
		})
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"rule_id":   result.RuleID,
			"css":       result.CSS,
			"js":        result.JS,
			"cache_hit": result.CacheHit,
		},
	})
}

// ListRulesHandler handles GET /v1/rules requests
// @Summary      List all rules
// @Description  Retrieves all configured URL matching rules
// @Tags         Rules
// @Produce      json
// @Success      200 {object} SuccessResponse{data=RuleListResponse} "Successfully retrieved rules"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/rules [get]
func (h *Handlers) ListRulesHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := ""
	if rid := c.Locals("requestid"); rid != nil {
		requestID = rid.(string)
	}

	rules, err := h.repository.GetAllRules(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to retrieve rules")

		appErr := domain.NewAppError(
			domain.ErrInternal,
			"Failed to retrieve rules",
			500,
			nil,
		).WithContext(ctx, "list_rules_retrieval")

		return h.sendError(c, appErr)
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"rules": rules,
			"count": len(rules),
		},
	})
}

// CreateRuleHandler handles POST /v1/rules requests
// @Summary      Create or update a rule
// @Description  Creates a new URL matching rule or updates an existing one
// @Tags         Rules
// @Accept       json
// @Produce      json
// @Param        rule body domain.Rule true "Rule to create or update"
// @Success      201 {object} SuccessResponse{data=object{rule=domain.Rule}} "Successfully created rule"
// @Failure      400 {object} ErrorResponse "Invalid request payload"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/rules [post]
func (h *Handlers) CreateRuleHandler(c *fiber.Ctx) error {
	ctx := c.Context()

	var rule domain.Rule
	if err := c.BodyParser(&rule); err != nil {
		appErr := domain.NewAppError(
			domain.ErrInvalidInput,
			"Invalid JSON payload",
			400,
			map[string]string{"error": err.Error()},
		).WithContext(ctx, "create_rule_parsing")

		return h.sendError(c, appErr)
	}

	// Sanitize input by trimming whitespace from all string fields
	rule.Type = strings.TrimSpace(rule.Type)
	rule.Pattern = strings.TrimSpace(rule.Pattern)
	rule.CSS = strings.TrimSpace(rule.CSS)
	rule.JS = strings.TrimSpace(rule.JS)

	// Sanitize attribution fields
	rule.Author = strings.TrimSpace(rule.Author)
	rule.Description = strings.TrimSpace(rule.Description)

	// Auto-generate ID if not provided
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// Validate rule using the validator
	if err := h.validator.ValidateRule(&rule); err != nil {
		appErr := err.(*domain.AppError).WithContext(ctx, "create_rule_validation")
		return h.sendError(c, appErr)
	}

	// Set timestamps for new rule creation
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	// Set source as local for API-created rules
	rule.Source = domain.RuleSource{
		Type: domain.SourceLocal,
	}

	// Add rule to matcher first (validates regex compilation)
	if err := h.matcher.AddRule(ctx, &rule); err != nil {
		log.Error().Err(err).Str("rule_id", rule.ID).Msg("Failed to add rule to matcher")
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Invalid rule pattern",
			422,
			map[string]string{"field": "pattern", "reason": err.Error()},
		))
	}

	// Create the rule in repository
	if err := h.repository.CreateRule(ctx, &rule); err != nil {
		// Rollback: remove from matcher
		if rollbackErr := h.matcher.RemoveRule(ctx, rule.ID); rollbackErr != nil {
			log.Error().Err(rollbackErr).Str("rule_id", rule.ID).
				Msg("Failed to rollback matcher after repository failure - state may be inconsistent")
		}

		log.Error().Err(err).Interface("rule", rule).Msg("Failed to create rule")

		// Check if it's a duplicate
		if strings.Contains(err.Error(), "already exists") {
			return h.sendError(c, domain.NewAppError(
				domain.ErrConflict,
				"Rule already exists",
				409,
				map[string]string{"rule_id": rule.ID},
			))
		}

		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to create rule",
			500,
			nil,
		))
	}

	return c.Status(201).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"rule": rule,
		},
	})
}

// DeleteRuleHandler handles DELETE /v1/rules/:id requests
// @Summary      Delete a rule
// @Description  Deletes a URL matching rule by its ID
// @Tags         Rules
// @Produce      json
// @Param        id path string true "Rule ID" format(uuid)
// @Success      200 {object} SuccessResponse{data=object{message=string,rule_id=string}} "Successfully deleted rule"
// @Failure      404 {object} ErrorResponse "Rule not found"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/rules/{id} [delete]
func (h *Handlers) DeleteRuleHandler(c *fiber.Ctx) error {
	ctx := c.Context()

	ruleID := strings.TrimSpace(c.Params("id"))
	if ruleID == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Rule ID is required",
			422,
			map[string]string{"field": "id", "reason": "required"},
		))
	}

	// Check if rule exists
	_, err := h.repository.GetRuleByID(ctx, ruleID)
	if err != nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrNotFound,
			"Rule not found",
			404,
			map[string]string{"rule_id": ruleID},
		))
	}

	// Delete the rule
	if err := h.repository.DeleteRule(ctx, ruleID); err != nil {
		log.Error().Err(err).Str("rule_id", ruleID).Msg("Failed to delete rule")
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to delete rule",
			500,
			nil,
		))
	}

	// Remove rule from matcher
	if err := h.matcher.RemoveRule(ctx, ruleID); err != nil {
		log.Error().Err(err).Str("rule_id", ruleID).Msg("Failed to remove rule from matcher")
		// Continue - rule is deleted, matcher will be updated on next restart
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"message": "Rule deleted successfully",
			"rule_id": ruleID,
		},
	})
}

// UpdateRuleRequest represents the request payload for updating a rule
// @Description Request payload for updating a rule
type UpdateRuleRequest struct {
	Type        string   `json:"type,omitempty" validate:"omitempty,oneof=exact regex wildcard" example:"exact"`
	Pattern     string   `json:"pattern,omitempty" validate:"omitempty,min=1,max=2048" example:"https://example.com/*"`
	CSS         string   `json:"css,omitempty" validate:"omitempty,max=102400" example:".banner { display: none; }"`
	JS          string   `json:"js,omitempty" validate:"omitempty,max=102400" example:"document.querySelector('.popup').remove();"`
	Priority    *int     `json:"priority,omitempty" validate:"omitempty,min=0,max=10000" example:"1500"`
	ModifiedBy  string   `json:"modified_by,omitempty" example:"modifier-name"`
	Description string   `json:"description,omitempty" example:"Updated description"`
	Tags        []string `json:"tags,omitempty" example:"cookies,privacy"`
}

// UpdateRuleHandler handles PUT /v1/rules/:id requests
// @Summary      Update a rule
// @Description  Updates an existing URL matching rule
// @Tags         Rules
// @Accept       json
// @Produce      json
// @Param        id path string true "Rule ID" format(uuid)
// @Param        rule body UpdateRuleRequest true "Rule fields to update"
// @Success      200 {object} SuccessResponse{data=object{rule=domain.Rule}} "Successfully updated rule"
// @Failure      400 {object} ErrorResponse "Invalid request payload"
// @Failure      404 {object} ErrorResponse "Rule not found"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/rules/{id} [put]
func (h *Handlers) UpdateRuleHandler(c *fiber.Ctx) error {
	ctx := c.Context()

	ruleID := strings.TrimSpace(c.Params("id"))
	if ruleID == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Rule ID is required",
			422,
			map[string]string{"field": "id", "reason": "required"},
		))
	}

	// Get existing rule
	existingRule, err := h.repository.GetRuleByID(ctx, ruleID)
	if err != nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrNotFound,
			"Rule not found",
			404,
			map[string]string{"rule_id": ruleID},
		))
	}

	// Store original rule for override creation (if it's a community rule)
	var originalRule *domain.Rule
	isCommunityRule := existingRule.Source.Type == domain.SourceCommunity
	if isCommunityRule {
		// Make a copy of the original rule before modifications
		originalCopy := *existingRule
		originalRule = &originalCopy
	}

	var req UpdateRuleRequest
	if err := c.BodyParser(&req); err != nil {
		appErr := domain.NewAppError(
			domain.ErrInvalidInput,
			"Invalid JSON payload",
			400,
			map[string]string{"error": err.Error()},
		).WithContext(ctx, "update_rule_parsing")

		return h.sendError(c, appErr)
	}

	// Apply updates to existing rule (only update fields that are provided)
	if req.Type != "" {
		existingRule.Type = strings.TrimSpace(req.Type)
	}
	if req.Pattern != "" {
		existingRule.Pattern = strings.TrimSpace(req.Pattern)
	}
	if req.CSS != "" {
		existingRule.CSS = strings.TrimSpace(req.CSS)
	}
	if req.JS != "" {
		existingRule.JS = strings.TrimSpace(req.JS)
	}
	if req.Priority != nil {
		existingRule.Priority = req.Priority
	}
	if req.Description != "" {
		existingRule.Description = strings.TrimSpace(req.Description)
	}
	if len(req.Tags) > 0 {
		existingRule.Tags = req.Tags
	}

	// Track modification - set ModifiedBy if provided
	modifiedBy := ""
	if req.ModifiedBy != "" {
		modifiedBy = strings.TrimSpace(req.ModifiedBy)
		existingRule.ModifiedBy = modifiedBy
	}

	// Update modification timestamp
	existingRule.UpdatedAt = time.Now()

	// Validate the updated rule
	if err := h.validator.ValidateRule(existingRule); err != nil {
		appErr := err.(*domain.AppError).WithContext(ctx, "update_rule_validation")
		return h.sendError(c, appErr)
	}

	// If this is a community rule and we have an override creator, create an override file
	if isCommunityRule && h.overrideCreator != nil && originalRule != nil {
		if err := h.overrideCreator.CreateOverride(originalRule, existingRule, modifiedBy); err != nil {
			log.Error().Err(err).Str("rule_id", existingRule.ID).Msg("Failed to create override file for community rule")
			// Continue - we'll still update the rule in the repository
		} else {
			log.Info().Str("rule_id", existingRule.ID).Msg("Created override file for community rule")
		}
	}

	// Update the rule in repository
	if err := h.repository.UpdateRule(ctx, existingRule); err != nil {
		log.Error().Err(err).Interface("rule", existingRule).Msg("Failed to update rule")

		// Check if it's a validation error (e.g., invalid regex)
		if strings.Contains(err.Error(), "regex") || strings.Contains(err.Error(), "compile") {
			return h.sendError(c, domain.NewAppError(
				domain.ErrValidationFailed,
				"Invalid regex pattern",
				422,
				map[string]string{"field": "pattern", "reason": "invalid_regex"},
			))
		}

		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to update rule",
			500,
			nil,
		))
	}

	// Update rule in matcher
	if err := h.matcher.UpdateRule(ctx, existingRule); err != nil {
		log.Error().Err(err).Str("rule_id", existingRule.ID).Msg("Failed to update rule in matcher")
		// Continue - rule is saved, matcher will be updated on next restart
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"rule": existingRule,
		},
	})
}

// HealthHandler handles GET /health requests
// @Summary      Health check
// @Description  Returns the health status of the service
// @Tags         System
// @Produce      json
// @Success      200 {object} HealthResponse "Service is healthy"
// @Router       /health [get]
func (h *Handlers) HealthHandler(c *fiber.Ctx) error {
	ctx := c.Context()

	// Perform comprehensive health check
	health := h.healthChecker.CheckHealth(ctx)

	// Determine HTTP status based on health status
	status := 200
	if health.Status != domain.HealthStatusHealthy {
		status = 503 // Service Unavailable
	}

	return c.Status(status).JSON(map[string]any{
		"status":     string(health.Status),
		"timestamp":  health.Timestamp.Format(time.RFC3339),
		"components": health.Components,
		"uptime":     health.Uptime,
	})
}

// MetricsHandler handles GET /metrics requests
// @Summary      System metrics
// @Description  Returns system metrics including cache statistics and rule counts
// @Tags         System
// @Produce      json
// @Success      200 {object} SuccessResponse{data=MetricsResponse} "Successfully retrieved metrics"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /metrics [get]
func (h *Handlers) MetricsHandler(c *fiber.Ctx) error {
	ctx := c.Context()

	// Get cache statistics
	cacheStats := h.cache.Stats()

	// Get rule count
	rules, err := h.repository.GetAllRules(ctx)
	ruleCount := 0
	if err == nil {
		ruleCount = len(rules)
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"cache": map[string]any{
				"hits":      cacheStats.Hits,
				"misses":    cacheStats.Misses,
				"size":      cacheStats.Size,
				"max_size":  cacheStats.MaxSize,
				"hit_ratio": cacheStats.HitRatio,
			},
			"rules": map[string]any{
				"count": ruleCount,
			},
			"uptime": map[string]any{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
}

// sendError sends a standardized error response
func (h *Handlers) sendError(c *fiber.Ctx, appErr *domain.AppError) error {
	return c.Status(appErr.StatusCode).JSON(ErrorResponse{
		Status:  "error",
		Code:    appErr.Code,
		Message: appErr.Message,
		Details: appErr.Details,
	})
}
