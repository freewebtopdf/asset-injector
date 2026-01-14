package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"gopkg.in/yaml.v3"
)

// RuleFile represents the structure of a rule file that can contain multiple rules
type RuleFile struct {
	Rules []domain.Rule `yaml:"rules" json:"rules"`
}

// LoadError represents an error that occurred while loading a specific file
type LoadError struct {
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
	Line     int    `json:"line,omitempty"`
}

// Parser handles parsing of rule files in YAML and JSON formats
type Parser struct{}

// NewParser creates a new Parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile reads and parses a rule file, returning the rules and any errors
// Supports both single-rule format and multi-rule format (with "rules" array)
func (p *Parser) ParseFile(scannedFile ScannedFile) ([]domain.Rule, *LoadError) {
	data, err := os.ReadFile(scannedFile.Path)
	if err != nil {
		return nil, &LoadError{
			FilePath: scannedFile.Path,
			Error:    fmt.Sprintf("failed to read file: %v", err),
		}
	}

	rules, loadErr := p.parseContent(data, scannedFile.Path)
	if loadErr != nil {
		return nil, loadErr
	}

	// Set source information for each rule
	for i := range rules {
		rules[i].Source = domain.RuleSource{
			Type:     scannedFile.SourceType,
			PackName: scannedFile.PackName,
		}
		rules[i].FilePath = scannedFile.Path
	}

	return rules, nil
}

// parseContent parses the file content based on file extension
func (p *Parser) parseContent(data []byte, filePath string) ([]domain.Rule, *LoadError) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Determine format based on the full extension pattern
	isYAML := strings.HasSuffix(strings.ToLower(filePath), ".rule.yaml")
	isJSON := strings.HasSuffix(strings.ToLower(filePath), ".rule.json")

	if !isYAML && !isJSON {
		// Fallback to extension-based detection
		isYAML = ext == ".yaml" || ext == ".yml"
		isJSON = ext == ".json"
	}

	if isYAML {
		return p.parseYAML(data, filePath)
	} else if isJSON {
		return p.parseJSON(data, filePath)
	}

	return nil, &LoadError{
		FilePath: filePath,
		Error:    fmt.Sprintf("unsupported file extension: %s", ext),
	}
}

// parseYAML parses YAML content, supporting both single-rule and multi-rule formats
func (p *Parser) parseYAML(data []byte, filePath string) ([]domain.Rule, *LoadError) {
	// First, try to parse as multi-rule format (with "rules" array)
	var ruleFile RuleFile
	if err := yaml.Unmarshal(data, &ruleFile); err == nil && len(ruleFile.Rules) > 0 {
		return ruleFile.Rules, nil
	}

	// Try to parse as single rule
	var singleRule domain.Rule
	if err := yaml.Unmarshal(data, &singleRule); err == nil && singleRule.ID != "" {
		return []domain.Rule{singleRule}, nil
	}

	// Try to parse as array of rules (without "rules" wrapper)
	var rulesArray []domain.Rule
	if err := yaml.Unmarshal(data, &rulesArray); err == nil && len(rulesArray) > 0 {
		return rulesArray, nil
	}

	// If all parsing attempts fail, return a detailed error
	var yamlErr error
	if err := yaml.Unmarshal(data, &ruleFile); err != nil {
		yamlErr = err
	}

	return nil, &LoadError{
		FilePath: filePath,
		Error:    fmt.Sprintf("failed to parse YAML: %v", yamlErr),
		Line:     extractYAMLErrorLine(yamlErr),
	}
}

// parseJSON parses JSON content, supporting both single-rule and multi-rule formats
func (p *Parser) parseJSON(data []byte, filePath string) ([]domain.Rule, *LoadError) {
	// First, try to parse as multi-rule format (with "rules" array)
	var ruleFile RuleFile
	if err := json.Unmarshal(data, &ruleFile); err == nil && len(ruleFile.Rules) > 0 {
		return ruleFile.Rules, nil
	}

	// Try to parse as single rule
	var singleRule domain.Rule
	if err := json.Unmarshal(data, &singleRule); err == nil && singleRule.ID != "" {
		return []domain.Rule{singleRule}, nil
	}

	// Try to parse as array of rules (without "rules" wrapper)
	var rulesArray []domain.Rule
	if err := json.Unmarshal(data, &rulesArray); err == nil && len(rulesArray) > 0 {
		return rulesArray, nil
	}

	// If all parsing attempts fail, return a detailed error
	var jsonErr error
	if err := json.Unmarshal(data, &ruleFile); err != nil {
		jsonErr = err
	}

	return nil, &LoadError{
		FilePath: filePath,
		Error:    fmt.Sprintf("failed to parse JSON: %v", jsonErr),
	}
}

// extractYAMLErrorLine attempts to extract line number from YAML error
func extractYAMLErrorLine(err error) int {
	if err == nil {
		return 0
	}
	// yaml.v3 errors often contain line information
	// This is a best-effort extraction
	return 0
}

// ParseContent parses rule content from bytes without file context
// Useful for testing or parsing rules from non-file sources
func (p *Parser) ParseContent(data []byte, format string) ([]domain.Rule, error) {
	var rules []domain.Rule
	var loadErr *LoadError

	switch strings.ToLower(format) {
	case "yaml", "yml":
		rules, loadErr = p.parseYAML(data, "content.yaml")
	case "json":
		rules, loadErr = p.parseJSON(data, "content.json")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	if loadErr != nil {
		return nil, fmt.Errorf("%s", loadErr.Error)
	}

	return rules, nil
}
