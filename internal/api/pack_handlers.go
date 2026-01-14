package api

import (
	"context"
	"strings"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// PackManager defines the interface for pack management operations
type PackManager interface {
	ListInstalled(ctx context.Context) ([]domain.PackInfo, error)
	ListAvailable(ctx context.Context) ([]domain.PackInfo, error)
	Install(ctx context.Context, source string) error
	Uninstall(ctx context.Context, name string) error
	Update(ctx context.Context, name string) error
	CheckUpdates(ctx context.Context) ([]domain.PackUpdate, error)
}

// RuleExporter defines the interface for rule export operations
type RuleExporter interface {
	ExportRule(ctx context.Context, id string) ([]byte, error)
	ExportPack(ctx context.Context, opts domain.ExportOptions) ([]byte, error)
}

// PackHandlers contains HTTP handlers for pack and community features
type PackHandlers struct {
	packManager  PackManager
	repository   domain.RuleRepository
	ruleExporter RuleExporter
}

// NewPackHandlers creates a new instance of pack handlers
func NewPackHandlers(packManager PackManager, repository domain.RuleRepository, ruleExporter RuleExporter) *PackHandlers {
	return &PackHandlers{
		packManager:  packManager,
		repository:   repository,
		ruleExporter: ruleExporter,
	}
}

// PackListResponse represents the response for listing packs
// @Description Response containing list of installed packs
type PackListResponse struct {
	Packs []domain.PackInfo `json:"packs"`
	Count int               `json:"count"`
}

// InstallPackRequest represents the request payload for installing a pack
// @Description Request payload for pack installation
type InstallPackRequest struct {
	Source string `json:"source" validate:"required" example:"cookie-banners@1.0.0"`
}

// UpdatePacksRequest represents the request payload for updating packs
// @Description Request payload for updating packs
type UpdatePacksRequest struct {
	Names []string `json:"names,omitempty" example:"cookie-banners,ad-blockers"`
	All   bool     `json:"all,omitempty" example:"false"`
}

// ExportRulesRequest represents the request payload for exporting rules
// @Description Request payload for exporting rules as a pack
type ExportRulesRequest struct {
	Name        string   `json:"name" validate:"required" example:"my-custom-pack"`
	Version     string   `json:"version" validate:"required" example:"1.0.0"`
	Description string   `json:"description" validate:"required" example:"My custom rule pack"`
	Author      string   `json:"author" validate:"required" example:"user@example.com"`
	RuleIDs     []string `json:"rule_ids,omitempty" example:"rule-1,rule-2"`
	Format      string   `json:"format,omitempty" example:"yaml"`
}

// RuleSourceResponse represents the response for rule source information
// @Description Response containing rule origin and attribution
type RuleSourceResponse struct {
	RuleID      string            `json:"rule_id"`
	Source      domain.RuleSource `json:"source"`
	Author      string            `json:"author,omitempty"`
	ModifiedBy  string            `json:"modified_by,omitempty"`
	Description string            `json:"description,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
}

// ListInstalledPacksHandler handles GET /v1/packs requests
// @Summary      List installed packs
// @Description  Returns all installed rule packs with metadata
// @Tags         Packs
// @Produce      json
// @Success      200 {object} SuccessResponse{data=PackListResponse} "Successfully retrieved packs"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/packs [get]
func (h *PackHandlers) ListInstalledPacksHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	if h.packManager == nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Pack manager not configured",
			500,
			nil,
		))
	}

	packs, err := h.packManager.ListInstalled(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to list installed packs")

		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to list installed packs",
			500,
			nil,
		))
	}

	if packs == nil {
		packs = []domain.PackInfo{}
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"packs": packs,
			"count": len(packs),
		},
	})
}

// InstallPackHandler handles POST /v1/packs/install requests
// @Summary      Install a pack
// @Description  Installs a pack from the specified source
// @Tags         Packs
// @Accept       json
// @Produce      json
// @Param        request body InstallPackRequest true "Pack source to install"
// @Success      201 {object} SuccessResponse{data=object{message=string,source=string}} "Successfully installed pack"
// @Failure      400 {object} ErrorResponse "Invalid request payload"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/packs/install [post]
func (h *PackHandlers) InstallPackHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	if h.packManager == nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Pack manager not configured",
			500,
			nil,
		))
	}

	var req InstallPackRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInvalidInput,
			"Invalid JSON payload",
			400,
			map[string]string{"error": err.Error()},
		))
	}

	// Validate source
	req.Source = strings.TrimSpace(req.Source)
	if req.Source == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Pack source is required",
			422,
			map[string]string{"field": "source", "reason": "required"},
		))
	}

	// Install the pack
	if err := h.packManager.Install(ctx, req.Source); err != nil {
		log.Error().
			Err(err).
			Str("source", req.Source).
			Str("request_id", requestID).
			Msg("Failed to install pack")

		// Check for specific error types
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return h.sendError(c, domain.NewAppError(
				domain.ErrPackNotFound,
				"Pack not found",
				404,
				map[string]string{"source": req.Source},
			))
		}
		if strings.Contains(errMsg, "manifest") || strings.Contains(errMsg, "invalid") {
			return h.sendError(c, domain.NewAppError(
				domain.ErrPackInvalid,
				"Invalid pack",
				422,
				map[string]string{"source": req.Source, "error": errMsg},
			))
		}

		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to install pack",
			500,
			map[string]string{"error": errMsg},
		))
	}

	return c.Status(201).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"message": "Pack installed successfully",
			"source":  req.Source,
		},
	})
}

// UninstallPackHandler handles DELETE /v1/packs/:name requests
// @Summary      Uninstall a pack
// @Description  Uninstalls the specified pack
// @Tags         Packs
// @Produce      json
// @Param        name path string true "Pack name"
// @Success      200 {object} SuccessResponse{data=object{message=string,name=string}} "Successfully uninstalled pack"
// @Failure      404 {object} ErrorResponse "Pack not found"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/packs/{name} [delete]
func (h *PackHandlers) UninstallPackHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	if h.packManager == nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Pack manager not configured",
			500,
			nil,
		))
	}

	packName := strings.TrimSpace(c.Params("name"))
	if packName == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Pack name is required",
			422,
			map[string]string{"field": "name", "reason": "required"},
		))
	}

	// Uninstall the pack
	if err := h.packManager.Uninstall(ctx, packName); err != nil {
		log.Error().
			Err(err).
			Str("pack_name", packName).
			Str("request_id", requestID).
			Msg("Failed to uninstall pack")

		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return h.sendError(c, domain.NewAppError(
				domain.ErrPackNotFound,
				"Pack not found",
				404,
				map[string]string{"name": packName},
			))
		}

		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to uninstall pack",
			500,
			map[string]string{"error": errMsg},
		))
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"message": "Pack uninstalled successfully",
			"name":    packName,
		},
	})
}

// ListAvailablePacksHandler handles GET /v1/packs/available requests
// @Summary      List available community packs
// @Description  Returns available packs from the community repository
// @Tags         Packs
// @Produce      json
// @Success      200 {object} SuccessResponse{data=PackListResponse} "Successfully retrieved available packs"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Failure      503 {object} ErrorResponse "Community repository unavailable"
// @Router       /v1/packs/available [get]
func (h *PackHandlers) ListAvailablePacksHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	if h.packManager == nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Pack manager not configured",
			500,
			nil,
		))
	}

	packs, err := h.packManager.ListAvailable(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Msg("Failed to list available packs")

		errMsg := err.Error()
		if strings.Contains(errMsg, "fetch") || strings.Contains(errMsg, "unavailable") {
			return h.sendError(c, domain.NewAppError(
				domain.ErrRepoUnavailable,
				"Community repository unavailable",
				503,
				map[string]string{"error": errMsg},
			))
		}

		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Failed to list available packs",
			500,
			nil,
		))
	}

	if packs == nil {
		packs = []domain.PackInfo{}
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data: map[string]any{
			"packs": packs,
			"count": len(packs),
		},
	})
}

// UpdatePacksHandler handles POST /v1/packs/update requests
// @Summary      Update packs
// @Description  Updates specified packs to their latest versions
// @Tags         Packs
// @Accept       json
// @Produce      json
// @Param        request body UpdatePacksRequest true "Packs to update"
// @Success      200 {object} SuccessResponse{data=object{message=string,updated=[]string}} "Successfully updated packs"
// @Failure      400 {object} ErrorResponse "Invalid request payload"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/packs/update [post]
func (h *PackHandlers) UpdatePacksHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	if h.packManager == nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Pack manager not configured",
			500,
			nil,
		))
	}

	var req UpdatePacksRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInvalidInput,
			"Invalid JSON payload",
			400,
			map[string]string{"error": err.Error()},
		))
	}

	var packsToUpdate []string
	var errors []string
	var updated []string

	if req.All {
		// Get all installed packs with available updates
		updates, err := h.packManager.CheckUpdates(ctx)
		if err != nil {
			log.Error().
				Err(err).
				Str("request_id", requestID).
				Msg("Failed to check for updates")

			return h.sendError(c, domain.NewAppError(
				domain.ErrInternal,
				"Failed to check for updates",
				500,
				nil,
			))
		}
		for _, update := range updates {
			packsToUpdate = append(packsToUpdate, update.Name)
		}
	} else {
		packsToUpdate = req.Names
	}

	// Update each pack
	for _, name := range packsToUpdate {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if err := h.packManager.Update(ctx, name); err != nil {
			log.Warn().
				Err(err).
				Str("pack_name", name).
				Str("request_id", requestID).
				Msg("Failed to update pack")
			errors = append(errors, name+": "+err.Error())
		} else {
			updated = append(updated, name)
		}
	}

	response := map[string]any{
		"message": "Pack update completed",
		"updated": updated,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data:   response,
	})
}

// GetRuleSourceHandler handles GET /v1/rules/:id/source requests
// @Summary      Get rule source information
// @Description  Returns the origin and attribution information for a rule
// @Tags         Rules
// @Produce      json
// @Param        id path string true "Rule ID"
// @Success      200 {object} SuccessResponse{data=RuleSourceResponse} "Successfully retrieved rule source"
// @Failure      404 {object} ErrorResponse "Rule not found"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/rules/{id}/source [get]
func (h *PackHandlers) GetRuleSourceHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	ruleID := strings.TrimSpace(c.Params("id"))
	if ruleID == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Rule ID is required",
			422,
			map[string]string{"field": "id", "reason": "required"},
		))
	}

	// Get the rule from repository
	rule, err := h.repository.GetRuleByID(ctx, ruleID)
	if err != nil {
		log.Warn().
			Err(err).
			Str("rule_id", ruleID).
			Str("request_id", requestID).
			Msg("Rule not found")

		return h.sendError(c, domain.NewAppError(
			domain.ErrNotFound,
			"Rule not found",
			404,
			map[string]string{"rule_id": ruleID},
		))
	}

	// Build source response
	response := RuleSourceResponse{
		RuleID:      rule.ID,
		Source:      rule.Source,
		Author:      rule.Author,
		ModifiedBy:  rule.ModifiedBy,
		Description: rule.Description,
	}

	if !rule.CreatedAt.IsZero() {
		response.CreatedAt = rule.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !rule.UpdatedAt.IsZero() {
		response.UpdatedAt = rule.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return c.Status(200).JSON(SuccessResponse{
		Status: "success",
		Data:   response,
	})
}

// ExportRulesHandler handles POST /v1/rules/export requests
// @Summary      Export rules as a pack
// @Description  Generates downloadable rule pack files
// @Tags         Rules
// @Accept       json
// @Produce      application/x-yaml
// @Produce      json
// @Param        request body ExportRulesRequest true "Export options"
// @Success      200 {string} string "Pack file content"
// @Failure      400 {object} ErrorResponse "Invalid request payload"
// @Failure      422 {object} ErrorResponse "Validation failed"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /v1/rules/export [post]
func (h *PackHandlers) ExportRulesHandler(c *fiber.Ctx) error {
	ctx := c.Context()
	requestID := getRequestID(c)

	if h.ruleExporter == nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInternal,
			"Rule exporter not configured",
			500,
			nil,
		))
	}

	var req ExportRulesRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, domain.NewAppError(
			domain.ErrInvalidInput,
			"Invalid JSON payload",
			400,
			map[string]string{"error": err.Error()},
		))
	}

	// Validate required fields
	req.Name = strings.TrimSpace(req.Name)
	req.Version = strings.TrimSpace(req.Version)
	req.Description = strings.TrimSpace(req.Description)
	req.Author = strings.TrimSpace(req.Author)

	if req.Name == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Pack name is required",
			422,
			map[string]string{"field": "name", "reason": "required"},
		))
	}
	if req.Version == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Pack version is required",
			422,
			map[string]string{"field": "version", "reason": "required"},
		))
	}
	if req.Description == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Pack description is required",
			422,
			map[string]string{"field": "description", "reason": "required"},
		))
	}
	if req.Author == "" {
		return h.sendError(c, domain.NewAppError(
			domain.ErrValidationFailed,
			"Pack author is required",
			422,
			map[string]string{"field": "author", "reason": "required"},
		))
	}

	// Set default format
	if req.Format == "" {
		req.Format = "yaml"
	}

	// Build export options
	opts := domain.ExportOptions{
		Name:        req.Name,
		Version:     req.Version,
		Description: req.Description,
		Author:      req.Author,
		RuleIDs:     req.RuleIDs,
		Format:      req.Format,
	}

	// Export the pack
	data, err := h.ruleExporter.ExportPack(ctx, opts)
	if err != nil {
		log.Error().
			Err(err).
			Str("pack_name", req.Name).
			Str("request_id", requestID).
			Msg("Failed to export pack")

		return h.sendError(c, domain.NewAppError(
			domain.ErrExportFailed,
			"Failed to export pack",
			500,
			map[string]string{"error": err.Error()},
		))
	}

	// Set appropriate content type
	contentType := "application/x-yaml"
	if req.Format == "json" {
		contentType = "application/json"
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename="+req.Name+"-"+req.Version+"."+req.Format)

	return c.Send(data)
}

// sendError sends a standardized error response
func (h *PackHandlers) sendError(c *fiber.Ctx, appErr *domain.AppError) error {
	return c.Status(appErr.StatusCode).JSON(ErrorResponse{
		Status:  "error",
		Code:    appErr.Code,
		Message: appErr.Message,
		Details: appErr.Details,
	})
}

// getRequestID extracts the request ID from context
func getRequestID(c *fiber.Ctx) string {
	if rid := c.Locals("requestid"); rid != nil {
		return rid.(string)
	}
	return ""
}
