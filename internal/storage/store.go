package storage

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/conflict"
	"github.com/freewebtopdf/asset-injector/internal/domain"
	"github.com/freewebtopdf/asset-injector/internal/loader"
)

// StoreConfig holds configuration for the Store
type StoreConfig struct {
	DataDir      string
	LocalDir     string
	CommunityDir string
	OverrideDir  string
}

// DefaultStoreConfig returns a default configuration
func DefaultStoreConfig(dataDir string) StoreConfig {
	return StoreConfig{
		DataDir:      dataDir,
		LocalDir:     filepath.Join(dataDir, "rules", "local"),
		CommunityDir: filepath.Join(dataDir, "rules", "community"),
		OverrideDir:  filepath.Join(dataDir, "rules", "overrides"),
	}
}

// Store implements the RuleRepository interface with dual indexing
type Store struct {
	mu       sync.RWMutex
	rules    map[string]*domain.Rule
	ruleList []*domain.Rule
	config   StoreConfig

	ruleLoader      *loader.FileRuleLoader
	ruleWriter      *loader.Writer
	conflictManager *conflict.ConflictManager
}

// NewStore creates a new Store instance
func NewStore(dataDir string) *Store {
	config := DefaultStoreConfig(dataDir)
	return NewStoreWithConfig(config)
}

// NewStoreWithConfig creates a new Store with full configuration
func NewStoreWithConfig(config StoreConfig) *Store {
	scanConfig := loader.ScanConfig{
		LocalDir:     config.LocalDir,
		CommunityDir: config.CommunityDir,
		OverrideDir:  config.OverrideDir,
	}

	return &Store{
		rules:           make(map[string]*domain.Rule),
		ruleList:        make([]*domain.Rule, 0),
		config:          config,
		ruleLoader:      loader.NewFileRuleLoader(scanConfig),
		ruleWriter:      loader.NewWriter(config.LocalDir),
		conflictManager: conflict.NewConflictManager(config.DataDir),
	}
}

// Load loads rules from file-based storage
func (s *Store) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return domain.NewAppErrorWithCause(
			domain.ErrTimeout,
			"Load cancelled",
			408,
			ctx.Err(),
			map[string]any{"operation": "load"},
		)
	default:
	}

	if err := s.conflictManager.Load(); err != nil {
		// Log warning but continue - disabled rules file might not exist yet
	}

	for _, dir := range []string{s.config.LocalDir, s.config.CommunityDir, s.config.OverrideDir} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return domain.NewAppErrorWithCause(
					domain.ErrInternal,
					"Failed to create rules directory",
					500,
					err,
					map[string]any{"dir": dir},
				).WithContext(ctx, "load")
			}
		}
	}

	rules, loadErrors, err := s.ruleLoader.LoadAll(ctx)
	if err != nil {
		return domain.NewAppErrorWithCause(
			domain.ErrInternal,
			"Failed to load rules from files",
			500,
			err,
			map[string]any{"errors": len(loadErrors)},
		).WithContext(ctx, "load")
	}

	resolvedRules := s.conflictManager.GetActiveRules(rules)

	s.rules = make(map[string]*domain.Rule, len(resolvedRules))
	s.ruleList = make([]*domain.Rule, 0, len(resolvedRules))

	for i := range resolvedRules {
		rule := resolvedRules[i]
		ruleCopy := rule
		s.rules[rule.ID] = &ruleCopy
		s.ruleList = append(s.ruleList, &ruleCopy)
	}

	return nil
}

// GetAllRules returns all rules in the repository
func (s *Store) GetAllRules(ctx context.Context) ([]domain.Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.Rule, len(s.ruleList))
	for i, rule := range s.ruleList {
		result[i] = *rule
	}

	return result, nil
}

// GetRuleByID retrieves a rule by its ID
func (s *Store) GetRuleByID(ctx context.Context, id string) (*domain.Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rule, exists := s.rules[id]
	if !exists {
		return nil, domain.NewAppError(
			domain.ErrNotFound,
			"Rule not found",
			404,
			map[string]any{"id": id},
		)
	}

	ruleCopy := *rule
	return &ruleCopy, nil
}

// CreateRule creates a new rule in the repository
func (s *Store) CreateRule(ctx context.Context, rule *domain.Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rules[rule.ID]; exists {
		return domain.NewAppError(
			domain.ErrConflict,
			"Rule already exists",
			409,
			map[string]any{"id": rule.ID},
		)
	}

	now := time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = now
	}

	if rule.Source.Type == "" {
		rule.Source.Type = domain.SourceLocal
	}

	ruleCopy := *rule

	s.rules[rule.ID] = &ruleCopy
	s.ruleList = append(s.ruleList, &ruleCopy)

	if err := s.ruleWriter.WriteRule(&ruleCopy); err != nil {
		delete(s.rules, rule.ID)
		s.ruleList = s.ruleList[:len(s.ruleList)-1]
		return domain.NewAppError(
			domain.ErrInternal,
			"Failed to write rule file",
			500,
			map[string]any{"error": err.Error(), "rule_id": rule.ID},
		)
	}

	rule.FilePath = filepath.Join(s.config.LocalDir, rule.ID+".rule.yaml")
	return nil
}

// UpdateRule updates an existing rule in the repository
func (s *Store) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existingRule, exists := s.rules[rule.ID]
	if !exists {
		return domain.NewAppError(
			domain.ErrNotFound,
			"Rule not found",
			404,
			map[string]any{"id": rule.ID},
		)
	}

	rule.CreatedAt = existingRule.CreatedAt
	rule.UpdatedAt = time.Now()

	if rule.Source.Type == "" {
		rule.Source = existingRule.Source
	}

	if rule.FilePath == "" {
		rule.FilePath = existingRule.FilePath
	}

	ruleCopy := *rule

	oldRule := *existingRule
	var oldIndex int
	for i, r := range s.ruleList {
		if r.ID == rule.ID {
			oldIndex = i
			break
		}
	}

	s.rules[rule.ID] = &ruleCopy
	s.ruleList[oldIndex] = &ruleCopy

	if err := s.ruleWriter.UpdateRule(&ruleCopy); err != nil {
		s.rules[rule.ID] = &oldRule
		s.ruleList[oldIndex] = &oldRule
		return domain.NewAppError(
			domain.ErrInternal,
			"Failed to update rule file",
			500,
			map[string]any{"error": err.Error(), "rule_id": rule.ID},
		)
	}

	return nil
}

// DeleteRule removes a rule from the repository
func (s *Store) DeleteRule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, exists := s.rules[id]
	if !exists {
		return domain.NewAppError(
			domain.ErrNotFound,
			"Rule not found",
			404,
			map[string]any{"id": id},
		)
	}

	if rule.FilePath != "" {
		if err := s.ruleWriter.DeleteRuleFile(rule.FilePath); err != nil {
			return domain.NewAppError(
				domain.ErrInternal,
				"Failed to delete rule file",
				500,
				map[string]any{"error": err.Error(), "rule_id": rule.ID},
			)
		}
	} else {
		_ = s.ruleWriter.DeleteRule(rule.ID)
	}

	delete(s.rules, id)

	for i, r := range s.ruleList {
		if r.ID == id {
			s.ruleList = append(s.ruleList[:i], s.ruleList[i+1:]...)
			break
		}
	}

	return nil
}

// Reload reloads rules from storage
func (s *Store) Reload(ctx context.Context) error {
	return s.Load(ctx)
}

// GetRuleLoader returns the underlying rule loader
func (s *Store) GetRuleLoader() *loader.FileRuleLoader {
	return s.ruleLoader
}

// GetConflictManager returns the conflict manager
func (s *Store) GetConflictManager() *conflict.ConflictManager {
	return s.conflictManager
}

// GetLoadErrors returns any errors from the last load operation
func (s *Store) GetLoadErrors() []loader.LoadError {
	return s.ruleLoader.GetLoadErrors()
}

// HealthCheck performs a health check on the storage system
func (s *Store) HealthCheck(ctx context.Context) domain.HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	status := "healthy"
	message := "Storage is operating normally"
	details := map[string]any{
		"rule_count": len(s.ruleList),
		"data_dir":   s.config.DataDir,
		"local_dir":  s.config.LocalDir,
	}

	if _, err := os.Stat(s.config.DataDir); err != nil {
		status = "unhealthy"
		message = "Data directory is not accessible"
		details["error"] = err.Error()
		return domain.HealthStatus{
			Status:    status,
			Message:   message,
			Details:   details,
			Timestamp: now,
		}
	}

	if len(s.rules) != len(s.ruleList) {
		status = "unhealthy"
		message = "Data structure inconsistency detected"
		details["map_size"] = len(s.rules)
		details["list_size"] = len(s.ruleList)
	}

	return domain.HealthStatus{
		Status:    status,
		Message:   message,
		Details:   details,
		Timestamp: now,
	}
}

// GetStats returns storage statistics
func (s *Store) GetStats(ctx context.Context) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]any{
		"rule_count":     len(s.ruleList),
		"data_directory": s.config.DataDir,
		"local_dir":      s.config.LocalDir,
		"community_dir":  s.config.CommunityDir,
		"override_dir":   s.config.OverrideDir,
		"load_errors":    len(s.ruleLoader.GetLoadErrors()),
	}

	typeCount := make(map[string]int)
	sourceCount := make(map[string]int)
	for _, rule := range s.ruleList {
		typeCount[rule.Type]++
		sourceCount[string(rule.Source.Type)]++
	}
	stats["rule_types"] = typeCount
	stats["rule_sources"] = sourceCount

	return stats
}
