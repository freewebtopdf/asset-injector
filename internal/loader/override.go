package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// OverrideManager handles creating and managing override files for community rules
// When a community rule is modified locally, an override file is created that
// preserves the original author attribution while adding modifier information
type OverrideManager struct {
	overrideDir string
	writer      *Writer
}

// NewOverrideManager creates a new OverrideManager with the specified override directory
func NewOverrideManager(overrideDir string) *OverrideManager {
	return &OverrideManager{
		overrideDir: overrideDir,
		writer:      NewWriter(overrideDir),
	}
}

// CreateOverride creates an override file for a community rule that has been modified locally
// It preserves the original author attribution and adds the modifier's attribution
func (m *OverrideManager) CreateOverride(originalRule *domain.Rule, modifiedRule *domain.Rule, modifiedBy string) error {
	// Validate that the original rule is from a community source
	if originalRule.Source.Type != domain.SourceCommunity {
		return fmt.Errorf("can only create overrides for community rules, got source type: %s", originalRule.Source.Type)
	}

	// Create the override rule by copying the modified rule
	overrideRule := *modifiedRule

	// Preserve original author attribution
	overrideRule.Author = originalRule.Author

	// Set modifier attribution
	if modifiedBy != "" {
		overrideRule.ModifiedBy = modifiedBy
	}

	// Update source to indicate this is an override
	overrideRule.Source = domain.RuleSource{
		Type:        domain.SourceOverride,
		PackName:    originalRule.Source.PackName,
		PackVersion: originalRule.Source.PackVersion,
		SourceURL:   originalRule.Source.SourceURL,
	}

	// Set timestamps
	overrideRule.UpdatedAt = time.Now()
	// Preserve original creation time
	overrideRule.CreatedAt = originalRule.CreatedAt

	// Determine the override file path
	// If the original rule came from a pack, create a subdirectory for that pack
	var overridePath string
	if originalRule.Source.PackName != "" {
		packDir := filepath.Join(m.overrideDir, originalRule.Source.PackName)
		overridePath = filepath.Join(packDir, fmt.Sprintf("%s.rule.yaml", overrideRule.ID))
	} else {
		overridePath = filepath.Join(m.overrideDir, fmt.Sprintf("%s.rule.yaml", overrideRule.ID))
	}

	// Update the file path in the rule
	overrideRule.FilePath = overridePath

	// Write the override file
	if err := m.writer.WriteRuleToPath(&overrideRule, overridePath); err != nil {
		return fmt.Errorf("failed to write override file: %w", err)
	}

	return nil
}

// GetOverridePath returns the path where an override file would be stored for a given rule
func (m *OverrideManager) GetOverridePath(rule *domain.Rule) string {
	if rule.Source.PackName != "" {
		return filepath.Join(m.overrideDir, rule.Source.PackName, fmt.Sprintf("%s.rule.yaml", rule.ID))
	}
	return filepath.Join(m.overrideDir, fmt.Sprintf("%s.rule.yaml", rule.ID))
}

// OverrideExists checks if an override file exists for a given rule ID
func (m *OverrideManager) OverrideExists(ruleID string, packName string) bool {
	var overridePath string
	if packName != "" {
		overridePath = filepath.Join(m.overrideDir, packName, fmt.Sprintf("%s.rule.yaml", ruleID))
	} else {
		overridePath = filepath.Join(m.overrideDir, fmt.Sprintf("%s.rule.yaml", ruleID))
	}

	_, err := os.Stat(overridePath)
	return err == nil
}

// DeleteOverride removes an override file for a given rule
func (m *OverrideManager) DeleteOverride(ruleID string, packName string) error {
	var overridePath string
	if packName != "" {
		overridePath = filepath.Join(m.overrideDir, packName, fmt.Sprintf("%s.rule.yaml", ruleID))
	} else {
		overridePath = filepath.Join(m.overrideDir, fmt.Sprintf("%s.rule.yaml", ruleID))
	}

	if err := os.Remove(overridePath); err != nil {
		if os.IsNotExist(err) {
			return nil // File already doesn't exist
		}
		return fmt.Errorf("failed to delete override file %s: %w", overridePath, err)
	}
	return nil
}

// IsCommunityRule checks if a rule is from a community source
func IsCommunityRule(rule *domain.Rule) bool {
	return rule.Source.Type == domain.SourceCommunity
}

// IsOverrideRule checks if a rule is an override of a community rule
func IsOverrideRule(rule *domain.Rule) bool {
	return rule.Source.Type == domain.SourceOverride
}

// IsLocalRule checks if a rule is a local rule
func IsLocalRule(rule *domain.Rule) bool {
	return rule.Source.Type == domain.SourceLocal
}
