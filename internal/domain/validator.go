package domain

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// InputValidator implements comprehensive input validation
type InputValidator struct {
	maxContentSize    int
	allowedSchemes    []string
	dangerousPatterns []*regexp.Regexp
	// Pre-compiled sanitization patterns
	sanitizeScriptTag     *regexp.Regexp
	sanitizeJSProtocol    *regexp.Regexp
	sanitizeVBProtocol    *regexp.Regexp
	sanitizeEventHandlers *regexp.Regexp
}

// NewInputValidator creates a new input validator with default settings
func NewInputValidator() *InputValidator {
	return &InputValidator{
		maxContentSize: 102400, // 100KB
		allowedSchemes: []string{"http", "https"},
		// Compile dangerous patterns once for performance
		dangerousPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)<script[^>]*>`),
			regexp.MustCompile(`(?i)javascript:`),
			regexp.MustCompile(`(?i)vbscript:`),
		},
		// Pre-compile sanitization patterns
		sanitizeScriptTag:     regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`),
		sanitizeJSProtocol:    regexp.MustCompile(`(?i)javascript:`),
		sanitizeVBProtocol:    regexp.MustCompile(`(?i)vbscript:`),
		sanitizeEventHandlers: regexp.MustCompile(`(?i)on\w+\s*=\s*["'][^"']*["']`),
	}
}

// ValidateRule validates a complete rule structure
func (v *InputValidator) ValidateRule(rule *Rule) error {
	if rule == nil {
		return NewAppError(ErrValidationFailed, "Rule cannot be nil", 422, nil)
	}

	// Validate ID format (should be UUID)
	if rule.ID == "" {
		return NewAppError(ErrValidationFailed, "Rule ID is required", 422, map[string]any{"field": "id"})
	}

	// Validate rule type
	if err := v.validateRuleType(rule.Type); err != nil {
		return err
	}

	// Validate pattern
	if err := v.validatePattern(rule.Type, rule.Pattern); err != nil {
		return err
	}

	// Validate CSS content
	if err := v.ValidateContent(rule.CSS, v.maxContentSize); err != nil {
		return NewAppErrorWithCause(ErrValidationFailed, "Invalid CSS content", 422, err, map[string]any{"field": "css"})
	}

	// Validate JS content
	if err := v.ValidateContent(rule.JS, v.maxContentSize); err != nil {
		return NewAppErrorWithCause(ErrValidationFailed, "Invalid JS content", 422, err, map[string]any{"field": "js"})
	}

	// Ensure at least one of CSS or JS is provided
	if rule.CSS == "" && rule.JS == "" {
		return NewAppError(ErrValidationFailed, "Rule must have at least one of css or js", 422, map[string]any{"fields": []string{"css", "js"}})
	}

	// Validate priority if set
	if rule.Priority != nil {
		if *rule.Priority < 0 || *rule.Priority > 10000 {
			return NewAppError(ErrValidationFailed, "Priority must be between 0 and 10000", 422, map[string]any{"field": "priority", "value": *rule.Priority})
		}
	}

	return nil
}

// ValidateURL validates a URL for resolve requests
func (v *InputValidator) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return NewAppError(ErrValidationFailed, "URL is required", 422, map[string]any{"field": "url"})
	}

	// Check length
	if len(urlStr) > 2048 {
		return NewAppError(ErrValidationFailed, "URL too long (max 2048 characters)", 422, map[string]any{
			"field":      "url",
			"length":     len(urlStr),
			"max_length": 2048,
		})
	}

	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return NewAppErrorWithCause(ErrValidationFailed, "Invalid URL format", 422, err, map[string]any{"field": "url"})
	}

	// Validate scheme
	if !v.isAllowedScheme(parsedURL.Scheme) {
		return NewAppError(ErrValidationFailed, "Only HTTP and HTTPS URLs are allowed", 422, map[string]any{
			"field":           "url",
			"scheme":          parsedURL.Scheme,
			"allowed_schemes": v.allowedSchemes,
		})
	}

	// Validate host is present
	if parsedURL.Host == "" {
		return NewAppError(ErrValidationFailed, "URL must have a valid host", 422, map[string]any{"field": "url"})
	}

	// Check for suspicious patterns
	if v.containsDangerousPatterns(urlStr) {
		return NewAppError(ErrValidationFailed, "URL contains potentially dangerous content", 422, map[string]any{"field": "url"})
	}

	return nil
}

// ValidateContent validates CSS/JS content for safety and size
func (v *InputValidator) ValidateContent(content string, maxSize int) error {
	if content == "" {
		return nil // Empty content is allowed
	}

	// Check UTF-8 validity
	if !utf8.ValidString(content) {
		return NewAppError(ErrValidationFailed, "Content must be valid UTF-8", 422, nil)
	}

	// Check size
	if len(content) > maxSize {
		return NewAppError(ErrValidationFailed, fmt.Sprintf("Content too large (max %d bytes)", maxSize), 422, map[string]any{
			"size":     len(content),
			"max_size": maxSize,
		})
	}

	// Check for dangerous patterns
	if v.containsDangerousPatterns(content) {
		return NewAppError(ErrValidationFailed, "Content contains potentially dangerous patterns", 422, nil)
	}

	return nil
}

// validateRuleType validates the rule type
func (v *InputValidator) validateRuleType(ruleType string) error {
	allowedTypes := []string{"exact", "regex", "wildcard"}
	if slices.Contains(allowedTypes, ruleType) {
		return nil
	}
	return NewAppError(ErrValidationFailed, "Invalid rule type", 422, map[string]any{
		"field":          "type",
		"value":          ruleType,
		"allowed_values": allowedTypes,
	})
}

// validatePattern validates the pattern based on rule type
func (v *InputValidator) validatePattern(ruleType, pattern string) error {
	if pattern == "" {
		return NewAppError(ErrValidationFailed, "Pattern is required", 422, map[string]any{"field": "pattern"})
	}

	if len(pattern) > 2048 {
		return NewAppError(ErrValidationFailed, "Pattern too long (max 2048 characters)", 422, map[string]any{
			"field":      "pattern",
			"length":     len(pattern),
			"max_length": 2048,
		})
	}

	switch ruleType {
	case "regex":
		// Validate regex compilation
		if _, err := regexp.Compile(pattern); err != nil {
			return NewAppErrorWithCause(ErrValidationFailed, "Invalid regex pattern", 422, err, map[string]any{
				"field":   "pattern",
				"pattern": pattern,
			})
		}
	case "exact":
		// For exact matches, validate as URL
		if err := v.ValidateURL(pattern); err != nil {
			return NewAppErrorWithCause(ErrValidationFailed, "Invalid exact pattern URL", 422, err, map[string]any{
				"field":   "pattern",
				"pattern": pattern,
			})
		}
	case "wildcard":
		// Validate that pattern starts with a valid scheme
		if !strings.HasPrefix(pattern, "http://") && !strings.HasPrefix(pattern, "https://") {
			return NewAppError(ErrValidationFailed, "Wildcard pattern must start with http:// or https://", 422, map[string]any{
				"field":   "pattern",
				"pattern": pattern,
			})
		}
		// Ensure there's something after the scheme (not just http://*)
		schemeEnd := strings.Index(pattern, "://") + 3
		afterScheme := pattern[schemeEnd:]
		if afterScheme == "" || afterScheme == "*" {
			return NewAppError(ErrValidationFailed, "Wildcard pattern must include a host pattern", 422, map[string]any{
				"field":   "pattern",
				"pattern": pattern,
			})
		}
	}

	return nil
}

// isAllowedScheme checks if the URL scheme is allowed
func (v *InputValidator) isAllowedScheme(scheme string) bool {
	return slices.Contains(v.allowedSchemes, strings.ToLower(scheme))
}

// containsDangerousPatterns checks for potentially dangerous content
func (v *InputValidator) containsDangerousPatterns(content string) bool {
	for _, pattern := range v.dangerousPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// NewValidator creates a new input validator instance
func NewValidator() Validator {
	return NewInputValidator()
}

// SanitizeContent removes or escapes potentially dangerous content
func (v *InputValidator) SanitizeContent(content string) string {
	content = v.sanitizeScriptTag.ReplaceAllString(content, "")
	content = v.sanitizeJSProtocol.ReplaceAllString(content, "")
	content = v.sanitizeVBProtocol.ReplaceAllString(content, "")
	content = v.sanitizeEventHandlers.ReplaceAllString(content, "")
	return content
}
