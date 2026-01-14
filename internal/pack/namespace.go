package pack

import (
	"fmt"
	"strings"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// NamespaceSeparator is the character used to separate pack namespace from rule ID
const NamespaceSeparator = ":"

// Namespacer handles rule ID prefixing for community packs
type Namespacer struct{}

// NewNamespacer creates a new Namespacer instance
func NewNamespacer() *Namespacer {
	return &Namespacer{}
}

// ApplyNamespace prefixes a rule ID with the pack namespace
// Format: "packname:ruleid"
func (n *Namespacer) ApplyNamespace(packName, ruleID string) string {
	if packName == "" {
		return ruleID
	}
	return fmt.Sprintf("%s%s%s", packName, NamespaceSeparator, ruleID)
}

// StripNamespace removes the pack namespace prefix from a rule ID
// Returns the original rule ID and the pack name (if any)
func (n *Namespacer) StripNamespace(namespacedID string) (packName, ruleID string) {
	parts := strings.SplitN(namespacedID, NamespaceSeparator, 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", namespacedID
}

// HasNamespace checks if a rule ID has a namespace prefix
func (n *Namespacer) HasNamespace(ruleID string) bool {
	return strings.Contains(ruleID, NamespaceSeparator)
}

// GetNamespace extracts the namespace from a rule ID
// Returns empty string if no namespace is present
func (n *Namespacer) GetNamespace(ruleID string) string {
	packName, _ := n.StripNamespace(ruleID)
	return packName
}

// GetBaseID extracts the base rule ID without namespace
func (n *Namespacer) GetBaseID(ruleID string) string {
	_, baseID := n.StripNamespace(ruleID)
	return baseID
}

// ApplyNamespaceToRule applies namespace prefixing to a rule from a community pack
// Only applies to rules from community sources
func (n *Namespacer) ApplyNamespaceToRule(rule *domain.Rule) {
	if rule.Source.Type != domain.SourceCommunity {
		return
	}
	if rule.Source.PackName == "" {
		return
	}
	// Don't double-prefix
	if n.HasNamespace(rule.ID) {
		return
	}
	rule.ID = n.ApplyNamespace(rule.Source.PackName, rule.ID)
}

// ApplyNamespaceToRules applies namespace prefixing to multiple rules
func (n *Namespacer) ApplyNamespaceToRules(rules []domain.Rule) {
	for i := range rules {
		n.ApplyNamespaceToRule(&rules[i])
	}
}

// FormatDisplayID returns a display-friendly version of the rule ID
// For namespaced IDs, returns "packname/ruleid" format
func (n *Namespacer) FormatDisplayID(ruleID string) string {
	packName, baseID := n.StripNamespace(ruleID)
	if packName == "" {
		return ruleID
	}
	return fmt.Sprintf("%s/%s", packName, baseID)
}

// ParseDisplayID parses a display-formatted ID back to internal format
// Converts "packname/ruleid" to "packname:ruleid"
func (n *Namespacer) ParseDisplayID(displayID string) string {
	parts := strings.SplitN(displayID, "/", 2)
	if len(parts) == 2 {
		return n.ApplyNamespace(parts[0], parts[1])
	}
	return displayID
}

// MatchesNamespace checks if a rule ID belongs to a specific namespace
func (n *Namespacer) MatchesNamespace(ruleID, packName string) bool {
	if packName == "" {
		return !n.HasNamespace(ruleID)
	}
	namespace := n.GetNamespace(ruleID)
	return namespace == packName
}

// FilterByNamespace filters rules by their namespace
func (n *Namespacer) FilterByNamespace(rules []domain.Rule, packName string) []domain.Rule {
	var filtered []domain.Rule
	for _, rule := range rules {
		if n.MatchesNamespace(rule.ID, packName) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}
