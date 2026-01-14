package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"gopkg.in/yaml.v3"
)

// Writer handles writing rules to disk in YAML format
type Writer struct {
	baseDir string // Base directory for writing rule files
}

// NewWriter creates a new Writer with the specified base directory
func NewWriter(baseDir string) *Writer {
	return &Writer{baseDir: baseDir}
}

// WriteRule writes a single rule to a YAML file
// Uses atomic write pattern: temp file → sync → rename
func (w *Writer) WriteRule(rule *domain.Rule) error {
	// Generate filename from rule ID
	filename := fmt.Sprintf("%s.rule.yaml", rule.ID)
	filePath := filepath.Join(w.baseDir, filename)

	return w.WriteRuleToPath(rule, filePath)
}

// WriteRuleToPath writes a single rule to a specific file path
// Uses atomic write pattern: temp file → sync → rename
func (w *Writer) WriteRuleToPath(rule *domain.Rule, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Prepare rule for serialization (create a copy to avoid modifying original)
	ruleToWrite := prepareRuleForWrite(rule)

	// Marshal to YAML
	data, err := yaml.Marshal(ruleToWrite)
	if err != nil {
		return fmt.Errorf("failed to marshal rule to YAML: %w", err)
	}

	// Atomic write: temp file → sync → rename
	return atomicWrite(filePath, data)
}

// WriteRules writes multiple rules to a single YAML file
// Uses the multi-rule format with "rules" array
func (w *Writer) WriteRules(rules []domain.Rule, filename string) error {
	filePath := filepath.Join(w.baseDir, filename)
	return w.WriteRulesToPath(rules, filePath)
}

// WriteRulesToPath writes multiple rules to a specific file path
func (w *Writer) WriteRulesToPath(rules []domain.Rule, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Prepare rules for serialization
	rulesToWrite := make([]ruleYAML, len(rules))
	for i, rule := range rules {
		rulesToWrite[i] = prepareRuleForWrite(&rule)
	}

	// Create rule file structure
	ruleFile := struct {
		Rules []ruleYAML `yaml:"rules"`
	}{
		Rules: rulesToWrite,
	}

	// Marshal to YAML
	data, err := yaml.Marshal(ruleFile)
	if err != nil {
		return fmt.Errorf("failed to marshal rules to YAML: %w", err)
	}

	// Atomic write
	return atomicWrite(filePath, data)
}

// ruleYAML is the YAML-serializable representation of a rule
// Excludes internal fields like Source and FilePath
type ruleYAML struct {
	ID          string    `yaml:"id"`
	Type        string    `yaml:"type"`
	Pattern     string    `yaml:"pattern"`
	CSS         string    `yaml:"css,omitempty"`
	JS          string    `yaml:"js,omitempty"`
	Priority    *int      `yaml:"priority,omitempty"`
	Author      string    `yaml:"author,omitempty"`
	ModifiedBy  string    `yaml:"modified_by,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Tags        []string  `yaml:"tags,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty"`
}

// prepareRuleForWrite converts a domain.Rule to the YAML-serializable format
func prepareRuleForWrite(rule *domain.Rule) ruleYAML {
	return ruleYAML{
		ID:          rule.ID,
		Type:        rule.Type,
		Pattern:     rule.Pattern,
		CSS:         rule.CSS,
		JS:          rule.JS,
		Priority:    rule.Priority,
		Author:      rule.Author,
		ModifiedBy:  rule.ModifiedBy,
		Description: rule.Description,
		Tags:        rule.Tags,
		CreatedAt:   rule.CreatedAt,
		UpdatedAt:   rule.UpdatedAt,
	}
}

// atomicWrite performs an atomic file write using temp file → sync → rename pattern
func atomicWrite(targetPath string, data []byte) error {
	// Create temp file in the same directory to ensure same filesystem
	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".rule-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup on error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close the file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	success = true
	return nil
}

// DeleteRule removes a rule file from disk
func (w *Writer) DeleteRule(ruleID string) error {
	filename := fmt.Sprintf("%s.rule.yaml", ruleID)
	filePath := filepath.Join(w.baseDir, filename)
	return w.DeleteRuleFile(filePath)
}

// DeleteRuleFile removes a rule file at the specified path
func (w *Writer) DeleteRuleFile(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // File already doesn't exist
		}
		return fmt.Errorf("failed to delete rule file %s: %w", filePath, err)
	}
	return nil
}

// UpdateRule updates an existing rule file or creates a new one
func (w *Writer) UpdateRule(rule *domain.Rule) error {
	// If the rule has a FilePath, update that file
	if rule.FilePath != "" {
		return w.WriteRuleToPath(rule, rule.FilePath)
	}
	// Otherwise, write to the default location
	return w.WriteRule(rule)
}
