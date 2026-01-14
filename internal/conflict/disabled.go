package conflict

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DisabledRuleEntry represents a disabled rule with metadata
type DisabledRuleEntry struct {
	RuleID     string    `json:"rule_id"`
	DisabledAt time.Time `json:"disabled_at"`
	Reason     string    `json:"reason,omitempty"`
}

// DisabledRulesFile represents the structure of the .disabled.json file
type DisabledRulesFile struct {
	Version       string              `json:"version"`
	UpdatedAt     time.Time           `json:"updated_at"`
	DisabledRules []DisabledRuleEntry `json:"disabled_rules"`
}

// DisabledRulesManager manages the persistence of disabled rule preferences
type DisabledRulesManager struct {
	mu       sync.RWMutex
	filePath string
	disabled map[string]DisabledRuleEntry
}

// NewDisabledRulesManager creates a new manager for disabled rules tracking
func NewDisabledRulesManager(rulesDir string) *DisabledRulesManager {
	return &DisabledRulesManager{
		filePath: filepath.Join(rulesDir, ".disabled.json"),
		disabled: make(map[string]DisabledRuleEntry),
	}
}

// Load reads the disabled rules from the .disabled.json file
func (m *DisabledRulesManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(m.filePath); os.IsNotExist(err) {
		// File doesn't exist, start with empty set
		m.disabled = make(map[string]DisabledRuleEntry)
		return nil
	}

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var file DisabledRulesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	// Convert to map for efficient lookups
	m.disabled = make(map[string]DisabledRuleEntry, len(file.DisabledRules))
	for _, entry := range file.DisabledRules {
		m.disabled[entry.RuleID] = entry
	}

	return nil
}

// Save writes the disabled rules to the .disabled.json file
func (m *DisabledRulesManager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.saveUnsafe()
}

// saveUnsafe performs the save without acquiring locks (caller must hold lock)
func (m *DisabledRulesManager) saveUnsafe() error {
	// Convert map to slice
	entries := make([]DisabledRuleEntry, 0, len(m.disabled))
	for _, entry := range m.disabled {
		entries = append(entries, entry)
	}

	file := DisabledRulesFile{
		Version:       "1.0",
		UpdatedAt:     time.Now(),
		DisabledRules: entries,
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Atomic write using temp file
	tempPath := m.filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempPath, m.filePath)
}

// DisableRule marks a rule as disabled
func (m *DisabledRulesManager) DisableRule(ruleID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabled[ruleID] = DisabledRuleEntry{
		RuleID:     ruleID,
		DisabledAt: time.Now(),
		Reason:     reason,
	}

	return m.saveUnsafe()
}

// EnableRule removes a rule from the disabled list
func (m *DisabledRulesManager) EnableRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.disabled[ruleID]; !exists {
		return nil // Already enabled
	}

	delete(m.disabled, ruleID)
	return m.saveUnsafe()
}

// IsDisabled checks if a rule is disabled
func (m *DisabledRulesManager) IsDisabled(ruleID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.disabled[ruleID]
	return exists
}

// GetDisabledRules returns all disabled rule IDs
func (m *DisabledRulesManager) GetDisabledRules() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.disabled))
	for id := range m.disabled {
		ids = append(ids, id)
	}
	return ids
}

// GetDisabledRuleEntries returns all disabled rule entries with metadata
func (m *DisabledRulesManager) GetDisabledRuleEntries() []DisabledRuleEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]DisabledRuleEntry, 0, len(m.disabled))
	for _, entry := range m.disabled {
		entries = append(entries, entry)
	}
	return entries
}

// GetDisabledEntry returns the disabled entry for a specific rule, or nil if not disabled
func (m *DisabledRulesManager) GetDisabledEntry(ruleID string) *DisabledRuleEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, exists := m.disabled[ruleID]; exists {
		return &entry
	}
	return nil
}

// Count returns the number of disabled rules
func (m *DisabledRulesManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.disabled)
}

// Clear removes all disabled rules
func (m *DisabledRulesManager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabled = make(map[string]DisabledRuleEntry)
	return m.saveUnsafe()
}

// SetDisabledRules replaces all disabled rules with the given list
func (m *DisabledRulesManager) SetDisabledRules(ruleIDs []string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.disabled = make(map[string]DisabledRuleEntry, len(ruleIDs))
	for _, id := range ruleIDs {
		m.disabled[id] = DisabledRuleEntry{
			RuleID:     id,
			DisabledAt: now,
			Reason:     reason,
		}
	}

	return m.saveUnsafe()
}
