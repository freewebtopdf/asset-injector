package loader

import (
	"context"
	"sync"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// ChangeType represents the type of file system change
type ChangeType string

const (
	ChangeCreated  ChangeType = "created"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// RuleChangeEvent represents a file system change affecting rules
type RuleChangeEvent struct {
	Type     ChangeType `json:"type"`
	FilePath string     `json:"file_path"`
	RuleIDs  []string   `json:"rule_ids,omitempty"`
}

// RuleLoader handles loading rules from the file system
// Implements the RuleLoader interface from the design document
type RuleLoader interface {
	// LoadAll scans directories and returns all valid rules
	LoadAll(ctx context.Context) ([]domain.Rule, []LoadError, error)
	// Reload triggers a full reload of all rules
	Reload(ctx context.Context) error
	// GetRules returns the currently loaded rules
	GetRules() []domain.Rule
	// GetLoadErrors returns errors from the last load operation
	GetLoadErrors() []LoadError
}

// FileRuleLoader implements RuleLoader using file-based storage
type FileRuleLoader struct {
	scanner    *Scanner
	parser     *Parser
	mu         sync.RWMutex
	rules      []domain.Rule
	loadErrors []LoadError
}

// NewFileRuleLoader creates a new FileRuleLoader with the given configuration
func NewFileRuleLoader(config ScanConfig) *FileRuleLoader {
	return &FileRuleLoader{
		scanner:    NewScanner(config),
		parser:     NewParser(),
		rules:      make([]domain.Rule, 0),
		loadErrors: make([]LoadError, 0),
	}
}

// LoadAll scans all configured directories and loads rules from discovered files
// Returns the loaded rules, any load errors, and a fatal error if scanning fails
func (l *FileRuleLoader) LoadAll(ctx context.Context) ([]domain.Rule, []LoadError, error) {
	// Scan for rule files
	scannedFiles, err := l.scanner.Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	var rules []domain.Rule
	var loadErrors []LoadError

	// Parse each discovered file
	for _, scannedFile := range scannedFiles {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		fileRules, loadErr := l.parser.ParseFile(scannedFile)
		if loadErr != nil {
			// Record error but continue loading other files
			loadErrors = append(loadErrors, *loadErr)
			continue
		}

		rules = append(rules, fileRules...)
	}

	// Store results
	l.mu.Lock()
	l.rules = rules
	l.loadErrors = loadErrors
	l.mu.Unlock()

	return rules, loadErrors, nil
}

// Reload triggers a full reload of all rules from disk
func (l *FileRuleLoader) Reload(ctx context.Context) error {
	_, _, err := l.LoadAll(ctx)
	return err
}

// GetRules returns the currently loaded rules
func (l *FileRuleLoader) GetRules() []domain.Rule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]domain.Rule, len(l.rules))
	copy(result, l.rules)
	return result
}

// GetLoadErrors returns errors from the last load operation
func (l *FileRuleLoader) GetLoadErrors() []LoadError {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]LoadError, len(l.loadErrors))
	copy(result, l.loadErrors)
	return result
}

// LoadFromDirectory loads rules from a single directory (for testing or specific use cases)
func (l *FileRuleLoader) LoadFromDirectory(ctx context.Context, dir string, sourceType domain.SourceType) ([]domain.Rule, []LoadError, error) {
	files, err := l.scanner.ScanSingleDirectory(ctx, dir)
	if err != nil {
		return nil, nil, err
	}

	var rules []domain.Rule
	var loadErrors []LoadError

	for _, filePath := range files {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		scannedFile := ScannedFile{
			Path:       filePath,
			SourceType: sourceType,
		}

		fileRules, loadErr := l.parser.ParseFile(scannedFile)
		if loadErr != nil {
			loadErrors = append(loadErrors, *loadErr)
			continue
		}

		rules = append(rules, fileRules...)
	}

	return rules, loadErrors, nil
}

// RuleCount returns the number of currently loaded rules
func (l *FileRuleLoader) RuleCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.rules)
}

// ErrorCount returns the number of load errors from the last operation
func (l *FileRuleLoader) ErrorCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.loadErrors)
}

// GetRuleByID finds a rule by its ID from the loaded rules
func (l *FileRuleLoader) GetRuleByID(id string) (*domain.Rule, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for i := range l.rules {
		if l.rules[i].ID == id {
			// Return a copy
			ruleCopy := l.rules[i]
			return &ruleCopy, true
		}
	}
	return nil, false
}

// GetRulesBySource returns all rules from a specific source type
func (l *FileRuleLoader) GetRulesBySource(sourceType domain.SourceType) []domain.Rule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []domain.Rule
	for _, rule := range l.rules {
		if rule.Source.Type == sourceType {
			result = append(result, rule)
		}
	}
	return result
}

// GetRulesByPack returns all rules from a specific pack
func (l *FileRuleLoader) GetRulesByPack(packName string) []domain.Rule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []domain.Rule
	for _, rule := range l.rules {
		if rule.Source.PackName == packName {
			result = append(result, rule)
		}
	}
	return result
}
