package conflict

import (
	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// RuleWithConflictInfo extends a rule with conflict and status information
type RuleWithConflictInfo struct {
	domain.Rule
	IsOverridden bool          `json:"is_overridden"`
	IsDisabled   bool          `json:"is_disabled"`
	HasConflict  bool          `json:"has_conflict"`
	Conflict     *ConflictInfo `json:"conflict,omitempty"`
}

// RuleListWithConflicts represents a list of rules with conflict information
type RuleListWithConflicts struct {
	Rules           []RuleWithConflictInfo `json:"rules"`
	Count           int                    `json:"count"`
	ConflictCount   int                    `json:"conflict_count"`
	DisabledCount   int                    `json:"disabled_count"`
	OverriddenCount int                    `json:"overridden_count"`
}

// ConflictManager combines detection, resolution, and disabled rules management
type ConflictManager struct {
	detector        *Detector
	resolver        *Resolver
	disabledManager *DisabledRulesManager
}

// NewConflictManager creates a new conflict manager
func NewConflictManager(rulesDir string) *ConflictManager {
	return &ConflictManager{
		detector:        NewDetector(),
		resolver:        NewResolver(),
		disabledManager: NewDisabledRulesManager(rulesDir),
	}
}

// Load initializes the conflict manager by loading disabled rules
func (m *ConflictManager) Load() error {
	return m.disabledManager.Load()
}

// GetDetector returns the conflict detector
func (m *ConflictManager) GetDetector() *Detector {
	return m.detector
}

// GetResolver returns the conflict resolver
func (m *ConflictManager) GetResolver() *Resolver {
	return m.resolver
}

// GetDisabledManager returns the disabled rules manager
func (m *ConflictManager) GetDisabledManager() *DisabledRulesManager {
	return m.disabledManager
}

// EnrichRulesWithConflictInfo adds conflict and status information to rules
func (m *ConflictManager) EnrichRulesWithConflictInfo(rules []domain.Rule) RuleListWithConflicts {
	// Detect all conflicts
	conflicts := m.detector.DetectConflicts(rules)

	// Get overridden rules
	overriddenRules := m.resolver.GetOverriddenRules(rules)
	overriddenSet := make(map[string]bool)
	for _, rule := range overriddenRules {
		// Create a unique key combining ID and source
		key := rule.ID + ":" + string(rule.Source.Type) + ":" + rule.Source.PackName
		overriddenSet[key] = true
	}

	// Build enriched rule list
	enrichedRules := make([]RuleWithConflictInfo, 0, len(rules))
	conflictCount := 0
	disabledCount := 0
	overriddenCount := 0

	// Track which rule IDs we've already added (to avoid duplicates after resolution)
	addedIDs := make(map[string]bool)

	for _, rule := range rules {
		// Skip if we've already added a rule with this ID (keep the first/highest priority one)
		if addedIDs[rule.ID] {
			continue
		}

		isDisabled := m.disabledManager.IsDisabled(rule.ID)
		key := rule.ID + ":" + string(rule.Source.Type) + ":" + rule.Source.PackName
		isOverridden := overriddenSet[key]

		var conflictInfo *ConflictInfo
		hasConflict := false
		if conflict, exists := conflicts[rule.ID]; exists {
			hasConflict = true
			conflictInfo = &conflict
			conflictCount++
		}

		if isDisabled {
			disabledCount++
		}
		if isOverridden {
			overriddenCount++
		}

		enrichedRules = append(enrichedRules, RuleWithConflictInfo{
			Rule:         rule,
			IsOverridden: isOverridden,
			IsDisabled:   isDisabled,
			HasConflict:  hasConflict,
			Conflict:     conflictInfo,
		})

		addedIDs[rule.ID] = true
	}

	return RuleListWithConflicts{
		Rules:           enrichedRules,
		Count:           len(enrichedRules),
		ConflictCount:   conflictCount,
		DisabledCount:   disabledCount,
		OverriddenCount: overriddenCount,
	}
}

// GetActiveRules returns only the active (non-disabled, resolved) rules
func (m *ConflictManager) GetActiveRules(rules []domain.Rule) []domain.Rule {
	// First resolve conflicts
	resolved := m.resolver.ResolveConflicts(rules)

	// Then filter out disabled rules
	active := make([]domain.Rule, 0, len(resolved))
	for _, rule := range resolved {
		if !m.disabledManager.IsDisabled(rule.ID) {
			active = append(active, rule)
		}
	}

	return active
}

// DisableRule disables a rule
func (m *ConflictManager) DisableRule(ruleID string, reason string) error {
	return m.disabledManager.DisableRule(ruleID, reason)
}

// EnableRule enables a previously disabled rule
func (m *ConflictManager) EnableRule(ruleID string) error {
	return m.disabledManager.EnableRule(ruleID)
}

// IsDisabled checks if a rule is disabled
func (m *ConflictManager) IsDisabled(ruleID string) bool {
	return m.disabledManager.IsDisabled(ruleID)
}

// GetRuleConflictInfo returns conflict information for a specific rule
func (m *ConflictManager) GetRuleConflictInfo(ruleID string, rules []domain.Rule) *ConflictInfo {
	return m.detector.DetectConflictsForRule(ruleID, rules)
}
