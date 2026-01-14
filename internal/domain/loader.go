package domain

// ChangeType represents the type of file system change
type ChangeType string

const (
	// ChangeCreated indicates a new file was created
	ChangeCreated ChangeType = "created"
	// ChangeModified indicates an existing file was modified
	ChangeModified ChangeType = "modified"
	// ChangeDeleted indicates a file was deleted
	ChangeDeleted ChangeType = "deleted"
)

// LoadError represents an error loading a specific rule file
type LoadError struct {
	FilePath string `json:"file_path"`      // Path to the file that failed to load
	Error    string `json:"error"`          // Error message describing the failure
	Line     int    `json:"line,omitempty"` // Line number where the error occurred (if applicable)
}

// RuleChangeEvent represents a file system change affecting rules
type RuleChangeEvent struct {
	Type     ChangeType `json:"type"`               // Type of change: created, modified, deleted
	FilePath string     `json:"file_path"`          // Path to the changed file
	RuleIDs  []string   `json:"rule_ids,omitempty"` // IDs of rules affected by this change
}

// ExportOptions configures pack export operations
type ExportOptions struct {
	Name        string   `json:"name"`               // Pack name for the export
	Version     string   `json:"version"`            // Pack version
	Description string   `json:"description"`        // Pack description
	Author      string   `json:"author"`             // Pack author
	RuleIDs     []string `json:"rule_ids,omitempty"` // Specific rule IDs to export (empty = all local rules)
	Format      string   `json:"format"`             // Output format: "yaml" or "json"
}

// ConflictInfo describes a rule ID conflict between different sources
type ConflictInfo struct {
	RuleID       string       `json:"rule_id"`       // The conflicting rule ID
	Sources      []RuleSource `json:"sources"`       // All sources where this rule ID exists
	ActiveSource RuleSource   `json:"active_source"` // The source that takes precedence
}

// RuleFile represents a rule file that can contain one or more rules
type RuleFile struct {
	Rules []Rule `json:"rules" yaml:"rules"` // List of rules in the file
}
