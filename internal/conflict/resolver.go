package conflict

import (
	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// Resolver applies priority-based conflict resolution to rules from multiple sources.
// Priority order: local > override > community
type Resolver struct {
	detector *Detector
}

// NewResolver creates a new conflict resolver
func NewResolver() *Resolver {
	return &Resolver{
		detector: NewDetector(),
	}
}

// ResolveConflicts takes a slice of rules (potentially with duplicates) and returns
// a deduplicated slice where conflicts are resolved based on source priority.
// Priority order: local > override > community
func (r *Resolver) ResolveConflicts(rules []domain.Rule) []domain.Rule {
	if len(rules) == 0 {
		return rules
	}

	// Group rules by ID
	rulesByID := make(map[string][]domain.Rule)
	for _, rule := range rules {
		rulesByID[rule.ID] = append(rulesByID[rule.ID], rule)
	}

	// Resolve each group and collect results
	resolved := make([]domain.Rule, 0, len(rulesByID))
	for _, rulesWithID := range rulesByID {
		winner := r.resolveGroup(rulesWithID)
		resolved = append(resolved, winner)
	}

	return resolved
}

// ResolveConflictsWithInfo resolves conflicts and returns both the resolved rules
// and information about any conflicts that were resolved.
func (r *Resolver) ResolveConflictsWithInfo(rules []domain.Rule) ([]domain.Rule, []ConflictInfo) {
	if len(rules) == 0 {
		return rules, nil
	}

	// Group rules by ID
	rulesByID := make(map[string][]domain.Rule)
	for _, rule := range rules {
		rulesByID[rule.ID] = append(rulesByID[rule.ID], rule)
	}

	// Resolve each group and collect results
	resolved := make([]domain.Rule, 0, len(rulesByID))
	var conflicts []ConflictInfo

	for id, rulesWithID := range rulesByID {
		winner := r.resolveGroup(rulesWithID)
		resolved = append(resolved, winner)

		// Record conflict info if there were multiple rules
		if len(rulesWithID) > 1 {
			sources := make([]domain.RuleSource, 0, len(rulesWithID))
			for _, rule := range rulesWithID {
				sources = append(sources, rule.Source)
			}
			conflicts = append(conflicts, ConflictInfo{
				RuleID:       id,
				Sources:      sources,
				ActiveSource: winner.Source,
			})
		}
	}

	return resolved, conflicts
}

// resolveGroup selects the winning rule from a group of rules with the same ID.
// Uses source priority: local > override > community
// Ties are broken by UpdatedAt (most recent wins)
func (r *Resolver) resolveGroup(rules []domain.Rule) domain.Rule {
	if len(rules) == 0 {
		return domain.Rule{}
	}
	if len(rules) == 1 {
		return rules[0]
	}

	best := rules[0]
	bestPriority := sourcePriority(best.Source.Type)

	for i := 1; i < len(rules); i++ {
		p := sourcePriority(rules[i].Source.Type)
		// Higher priority wins, or same priority but more recently updated
		if p > bestPriority || (p == bestPriority && rules[i].UpdatedAt.After(best.UpdatedAt)) {
			best = rules[i]
			bestPriority = p
		}
	}
	return best
}

// GetActiveRule returns the rule that should be active for a given ID,
// considering all rules and applying priority-based resolution.
func (r *Resolver) GetActiveRule(ruleID string, rules []domain.Rule) *domain.Rule {
	var candidates []domain.Rule
	for _, rule := range rules {
		if rule.ID == ruleID {
			candidates = append(candidates, rule)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	winner := r.resolveGroup(candidates)
	return &winner
}

// IsOverridden checks if a rule from a specific source is overridden by a higher-priority source.
func (r *Resolver) IsOverridden(ruleID string, sourceType domain.SourceType, rules []domain.Rule) bool {
	var candidates []domain.Rule
	for _, rule := range rules {
		if rule.ID == ruleID {
			candidates = append(candidates, rule)
		}
	}

	if len(candidates) <= 1 {
		return false
	}

	// Check if any rule has higher priority than the given source type
	givenPriority := sourcePriority(sourceType)
	for _, rule := range candidates {
		if sourcePriority(rule.Source.Type) > givenPriority {
			return true
		}
	}

	return false
}

// GetOverriddenRules returns all rules that are overridden by higher-priority sources.
func (r *Resolver) GetOverriddenRules(rules []domain.Rule) []domain.Rule {
	// Group rules by ID
	rulesByID := make(map[string][]domain.Rule)
	for _, rule := range rules {
		rulesByID[rule.ID] = append(rulesByID[rule.ID], rule)
	}

	var overridden []domain.Rule
	for _, rulesWithID := range rulesByID {
		if len(rulesWithID) <= 1 {
			continue
		}

		// Find the winner
		winner := r.resolveGroup(rulesWithID)

		// All non-winners are overridden
		for _, rule := range rulesWithID {
			if rule.Source.Type != winner.Source.Type ||
				rule.Source.PackName != winner.Source.PackName {
				overridden = append(overridden, rule)
			}
		}
	}

	return overridden
}

// MergeRuleSets merges multiple rule sets and resolves conflicts.
// Rules from earlier sets have lower priority than rules from later sets
// unless source type priority overrides this.
func (r *Resolver) MergeRuleSets(ruleSets ...[]domain.Rule) []domain.Rule {
	var allRules []domain.Rule
	for _, set := range ruleSets {
		allRules = append(allRules, set...)
	}
	return r.ResolveConflicts(allRules)
}
