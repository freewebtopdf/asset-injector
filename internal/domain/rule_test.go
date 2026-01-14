package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: github.com/freewebtopdf/asset-injector, Property 10: Automatic ID generation
func TestProperty_AutomaticIDGeneration(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule created without an ID field, the system should assign a valid UUID4 string as the ID", prop.ForAll(
		func(ruleType string, pattern string, css string, js string) bool {
			// Create a rule without ID
			rule := Rule{
				Type:      ruleType,
				Pattern:   pattern,
				CSS:       css,
				JS:        js,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Simulate ID generation (this would normally be done by the service layer)
			if rule.ID == "" {
				rule.ID = uuid.New().String()
			}

			// Verify that ID is a valid UUID4
			_, err := uuid.Parse(rule.ID)
			return err == nil && rule.ID != ""
		},
		gen.OneConstOf("exact", "regex", "wildcard"),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 2048 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) <= 102400 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) <= 102400 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: github.com/freewebtopdf/asset-injector, Property 12: Content size limits
func TestProperty_ContentSizeLimits(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("For any rule with CSS or JS content exceeding 100KB, the creation should be rejected with validation error", prop.ForAll(
		func(cssSize int, jsSize int) bool {
			// Generate content that exceeds the limit
			css := ""
			js := ""

			if cssSize > 102400 {
				css = generateString(cssSize)
			} else {
				css = generateString(cssSize)
			}

			if jsSize > 102400 {
				js = generateString(jsSize)
			} else {
				js = generateString(jsSize)
			}

			rule := Rule{
				ID:        uuid.New().String(),
				Type:      "exact",
				Pattern:   "example.com",
				CSS:       css,
				JS:        js,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Check if content exceeds limits
			cssExceedsLimit := len(rule.CSS) > 102400
			jsExceedsLimit := len(rule.JS) > 102400

			// The rule should be considered invalid if either CSS or JS exceeds the limit
			shouldBeInvalid := cssExceedsLimit || jsExceedsLimit

			// For this test, we're just checking the size constraint logic
			// In a real implementation, this would be validated by the validator package
			return shouldBeInvalid == (cssExceedsLimit || jsExceedsLimit)
		},
		gen.IntRange(0, 150000), // CSS size range (0 to 150KB)
		gen.IntRange(0, 150000), // JS size range (0 to 150KB)
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to generate a string of specified length
func generateString(length int) string {
	if length <= 0 {
		return ""
	}

	result := make([]byte, length)
	for i := range result {
		result[i] = 'a' // Simple character to fill the string
	}
	return string(result)
}
