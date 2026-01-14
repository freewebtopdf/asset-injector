// Package conflict provides conflict detection and resolution for rules from multiple sources.
package conflict

import (
	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// ConflictInfo describes a rule ID conflict between multiple sources
type ConflictInfo struct {
	RuleID       string              `json:"rule_id"`
	Sources      []domain.RuleSource `json:"sources"`
	ActiveSource domain.RuleSource   `json:"active_source"`
}

// Detector identifies rule ID conflicts across different sources
type Detector struct{}

// NewDetector creates a new conflict detector
func NewDetector() *Detector {
	return &Detector{}
}

// DetectConflicts identifies all rule ID conflicts in the given set of rules.
// A conflict occurs when multiple rules have the same ID but come from different sources.
// Returns a map of rule ID to ConflictInfo for all conflicting rules.
func (d *Detector) DetectConflicts(rules []domain.Rule) map[string]ConflictInfo {
	// Group rules by ID
	rulesByID := make(map[string][]domain.Rule)
	for _, rule := range rules {
		rulesByID[rule.ID] = append(rulesByID[rule.ID], rule)
	}

	// Find conflicts (rules with same ID from different sources)
	conflicts := make(map[string]ConflictInfo)
	for id, rulesWithID := range rulesByID {
		if len(rulesWithID) > 1 {
			// Collect all unique sources
			sources := make([]domain.RuleSource, 0, len(rulesWithID))
			for _, rule := range rulesWithID {
				sources = append(sources, rule.Source)
			}

			// Determine the active source based on priority
			activeRule := d.resolveByPriority(rulesWithID)

			conflicts[id] = ConflictInfo{
				RuleID:       id,
				Sources:      sources,
				ActiveSource: activeRule.Source,
			}
		}
	}

	return conflicts
}

// DetectConflictsForRule checks if a specific rule ID has conflicts in the given rule set.
// Returns the ConflictInfo if a conflict exists, or nil if no conflict.
func (d *Detector) DetectConflictsForRule(ruleID string, rules []domain.Rule) *ConflictInfo {
	// Find all rules with the given ID
	var rulesWithID []domain.Rule
	for _, rule := range rules {
		if rule.ID == ruleID {
			rulesWithID = append(rulesWithID, rule)
		}
	}

	// No conflict if only one or zero rules with this ID
	if len(rulesWithID) <= 1 {
		return nil
	}

	// Collect all sources
	sources := make([]domain.RuleSource, 0, len(rulesWithID))
	for _, rule := range rulesWithID {
		sources = append(sources, rule.Source)
	}

	// Determine the active source based on priority
	activeRule := d.resolveByPriority(rulesWithID)

	return &ConflictInfo{
		RuleID:       ruleID,
		Sources:      sources,
		ActiveSource: activeRule.Source,
	}
}

// HasConflict checks if a rule ID has any conflicts in the given rule set
func (d *Detector) HasConflict(ruleID string, rules []domain.Rule) bool {
	count := 0
	for _, rule := range rules {
		if rule.ID == ruleID {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}

// GetConflictingRuleIDs returns a list of all rule IDs that have conflicts
func (d *Detector) GetConflictingRuleIDs(rules []domain.Rule) []string {
	conflicts := d.DetectConflicts(rules)
	ids := make([]string, 0, len(conflicts))
	for id := range conflicts {
		ids = append(ids, id)
	}
	return ids
}

// resolveByPriority returns the rule that should be active based on source priority.
// Priority order: local > override > community
func (d *Detector) resolveByPriority(rules []domain.Rule) domain.Rule {
	if len(rules) == 0 {
		return domain.Rule{}
	}

	// Find the highest priority rule
	var best domain.Rule
	bestPriority := -1

	for _, rule := range rules {
		priority := sourcePriority(rule.Source.Type)
		if priority > bestPriority {
			bestPriority = priority
			best = rule
		}
	}

	return best
}

// sourcePriority returns the priority value for a source type.
// Higher values indicate higher priority.
// This is the canonical implementation - used by both Detector and Resolver.
func sourcePriority(sourceType domain.SourceType) int {
	switch sourceType {
	case domain.SourceLocal:
		return 3 // Highest priority
	case domain.SourceOverride:
		return 2 // Medium priority
	case domain.SourceCommunity:
		return 1 // Lowest priority
	default:
		return 0 // Unknown source type
	}
}
