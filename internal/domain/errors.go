package domain

import (
	"context"
	"fmt"
	"time"
)

// AppError represents a domain-specific error with structured information and enhanced context
type AppError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"-"`
	Details    any       `json:"details,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"request_id,omitempty"`
	Operation  string    `json:"operation,omitempty"`
	Cause      error     `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for error wrapping
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *AppError) WithContext(ctx context.Context, operation string) *AppError {
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			e.RequestID = id
		}
	}
	e.Operation = operation
	return e
}

// Error codes for different error categories
const (
	ErrInvalidInput     = "INVALID_INPUT"     // 400 Bad Request
	ErrValidationFailed = "VALIDATION_FAILED" // 422 Unprocessable Entity
	ErrNotFound         = "NOT_FOUND"         // 404 Not Found
	ErrConflict         = "CONFLICT"          // 409 Conflict
	ErrInternal         = "INTERNAL_ERROR"    // 500 Internal Server Error
	ErrTimeout          = "TIMEOUT"           // 408 Request Timeout
	ErrTooLarge         = "PAYLOAD_TOO_LARGE" // 413 Payload Too Large
	ErrRateLimit        = "RATE_LIMIT"        // 429 Too Many Requests
	ErrUnauthorized     = "UNAUTHORIZED"      // 401 Unauthorized

	// Community-specific error codes
	ErrPackNotFound      = "PACK_NOT_FOUND"     // 404 Pack not found
	ErrPackInvalid       = "PACK_INVALID"       // 422 Invalid pack
	ErrManifestInvalid   = "MANIFEST_INVALID"   // 422 Invalid manifest
	ErrVersionConflict   = "VERSION_CONFLICT"   // 409 Version conflict
	ErrDependencyMissing = "DEPENDENCY_MISSING" // 422 Missing dependency
	ErrImportFailed      = "IMPORT_FAILED"      // 500 Import failed
	ErrExportFailed      = "EXPORT_FAILED"      // 500 Export failed
	ErrRepoUnavailable   = "REPO_UNAVAILABLE"   // 503 Repository unavailable
	ErrRuleConflict      = "RULE_CONFLICT"      // 409 Rule conflict
)

// NewAppError creates a new AppError with the specified parameters
func NewAppError(code, message string, statusCode int, details any) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Details:    details,
		Timestamp:  time.Now(),
	}
}

// NewAppErrorWithCause creates a new AppError with underlying cause
func NewAppErrorWithCause(code, message string, statusCode int, cause error, details any) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Details:    details,
		Timestamp:  time.Now(),
		Cause:      cause,
	}
}

// IsTimeout checks if the error is a timeout error
func IsTimeout(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrTimeout
	}
	return false
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrNotFound
	}
	return false
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrValidationFailed
	}
	return false
}
