package community

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// SinglesIndex represents the index of individual contributed rules
type SinglesIndex struct {
	Version   string             `json:"version"`
	UpdatedAt time.Time          `json:"updated_at"`
	Count     int                `json:"count"`
	Rules     []SinglesIndexRule `json:"rules"`
}

// SinglesIndexRule represents a rule entry in the singles index
type SinglesIndexRule struct {
	ID          string   `json:"id"`
	Pattern     string   `json:"pattern"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	File        string   `json:"file"`
	Hash        string   `json:"hash,omitempty"` // SHA256 for update detection
}

// SinglesSyncerConfig holds configuration for the singles syncer
type SinglesSyncerConfig struct {
	RepoURL      string
	Timeout      time.Duration
	SyncInterval time.Duration
	TargetDir    string
}

// SinglesSyncer handles syncing individual contributed rules from the community repo
type SinglesSyncer struct {
	config     SinglesSyncerConfig
	httpClient *http.Client
	lastETag   string
	mu         sync.RWMutex
	stopOnce   sync.Once
	stopCh     chan struct{}
	onSync     func()
	onSyncMu   sync.RWMutex
}

// NewSinglesSyncer creates a new SinglesSyncer
func NewSinglesSyncer(config SinglesSyncerConfig) *SinglesSyncer {
	// Ensure minimum sync interval to prevent tight loops
	if config.SyncInterval < time.Minute {
		config.SyncInterval = time.Minute
	}

	return &SinglesSyncer{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		stopCh: make(chan struct{}),
	}
}

// SetOnSync sets a callback to be called when rules are synced
func (s *SinglesSyncer) SetOnSync(fn func()) {
	s.onSyncMu.Lock()
	s.onSync = fn
	s.onSyncMu.Unlock()
}

func (s *SinglesSyncer) triggerOnSync() {
	s.onSyncMu.RLock()
	fn := s.onSync
	s.onSyncMu.RUnlock()
	if fn != nil {
		fn()
	}
}

// Start begins the background sync loop
func (s *SinglesSyncer) Start(ctx context.Context) {
	go s.syncLoop(ctx)
}

// Stop stops the background sync loop
func (s *SinglesSyncer) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *SinglesSyncer) syncLoop(ctx context.Context) {
	// Initial sync
	if err := s.Sync(ctx); err != nil {
		log.Warn().Err(err).Msg("Initial singles sync failed")
	}

	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.Sync(ctx); err != nil {
				log.Warn().Err(err).Msg("Singles sync failed")
			}
		}
	}
}

// Sync checks for updates and downloads new/changed rules
func (s *SinglesSyncer) Sync(ctx context.Context) error {
	index, changed, err := s.fetchIndex(ctx)
	if err != nil {
		return err
	}

	if !changed {
		log.Debug().Msg("Singles index unchanged")
		return nil
	}

	log.Info().Int("count", index.Count).Msg("Singles index updated, syncing rules")

	// Ensure target directory exists once
	if err := os.MkdirAll(s.config.TargetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Build set of expected files from index
	expectedFiles := make(map[string]SinglesIndexRule)
	for _, rule := range index.Rules {
		expectedFiles[rule.File] = rule
	}

	// Download new/updated rules
	var added, updated int
	for _, rule := range index.Rules {
		// Check context before each download
		if ctx.Err() != nil {
			return ctx.Err()
		}

		action, err := s.syncRule(ctx, rule)
		if err != nil {
			log.Warn().Err(err).Str("rule", rule.ID).Msg("Failed to sync rule")
			continue
		}
		switch action {
		case "added":
			added++
		case "updated":
			updated++
		}
	}

	// Remove rules no longer in index
	deleted := s.cleanupRemovedRules(expectedFiles)

	totalChanges := added + updated + deleted
	log.Info().
		Int("added", added).
		Int("updated", updated).
		Int("deleted", deleted).
		Msg("Singles sync completed")

	// Only trigger callback if actual changes occurred
	if totalChanges > 0 {
		s.triggerOnSync()
	}

	return nil
}

func (s *SinglesSyncer) fetchIndex(ctx context.Context) (*SinglesIndex, bool, error) {
	indexURL := fmt.Sprintf("%s/contents/singles/index.json", s.config.RepoURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, false, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3.raw")
	req.Header.Set("User-Agent", "asset-injector")

	s.mu.RLock()
	if s.lastETag != "" {
		req.Header.Set("If-None-Match", s.lastETag)
	}
	s.mu.RUnlock()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("failed to fetch singles index: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, false, err
	}

	var index SinglesIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, false, err
	}

	s.mu.Lock()
	s.lastETag = resp.Header.Get("ETag")
	s.mu.Unlock()

	return &index, true, nil
}

// validateFilename checks that a filename is safe (no path traversal)
func validateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("empty filename")
	}
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	if !strings.HasSuffix(filename, ".rule.yaml") {
		return fmt.Errorf("invalid file extension: %s", filename)
	}
	return nil
}

// syncRule downloads a rule if new or updated. Returns action taken: "added", "updated", or "".
func (s *SinglesSyncer) syncRule(ctx context.Context, indexRule SinglesIndexRule) (string, error) {
	// Validate filename to prevent path traversal
	if err := validateFilename(indexRule.File); err != nil {
		return "", err
	}

	targetPath := filepath.Join(s.config.TargetDir, indexRule.File)
	hashPath := targetPath + ".hash"

	// Check if update needed
	action := "added"
	if _, err := os.Stat(targetPath); err == nil {
		// File exists - check if hash changed
		if indexRule.Hash == "" {
			// No hash in index, skip existing files
			return "", nil
		}
		existingHash, err := os.ReadFile(hashPath)
		if err == nil && string(existingHash) == indexRule.Hash {
			// Hash matches, no update needed
			return "", nil
		}
		action = "updated"
	}

	// Download rule file - URL encode the filename
	ruleURL := fmt.Sprintf("%s/contents/singles/rules/%s", s.config.RepoURL, url.PathEscape(indexRule.File))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ruleURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3.raw")
	req.Header.Set("User-Agent", "asset-injector")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download rule: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", err
	}

	// Validate rule content
	if err := validateRuleContent(body); err != nil {
		return "", err
	}

	// Write rule atomically
	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file on failure
		return "", err
	}

	// Store hash for future comparison
	if indexRule.Hash != "" {
		_ = os.WriteFile(hashPath, []byte(indexRule.Hash), 0644)
	}

	return action, nil
}

// validateRuleContent validates that the rule YAML is valid and has required fields
func validateRuleContent(body []byte) error {
	var rule domain.Rule
	if err := yaml.Unmarshal(body, &rule); err != nil {
		return fmt.Errorf("invalid rule YAML: %w", err)
	}

	if rule.ID == "" {
		return fmt.Errorf("rule missing required field: id")
	}
	if rule.Pattern == "" {
		return fmt.Errorf("rule missing required field: pattern")
	}
	if rule.Type == "" {
		return fmt.Errorf("rule missing required field: type")
	}
	if rule.Type != "exact" && rule.Type != "wildcard" && rule.Type != "regex" {
		return fmt.Errorf("invalid rule type: %s", rule.Type)
	}
	if rule.CSS == "" && rule.JS == "" {
		return fmt.Errorf("rule must have css or js")
	}

	return nil
}

// cleanupRemovedRules deletes local rules that are no longer in the index
func (s *SinglesSyncer) cleanupRemovedRules(expectedFiles map[string]SinglesIndexRule) int {
	deleted := 0

	entries, err := os.ReadDir(s.config.TargetDir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip hash files
		if strings.HasSuffix(name, ".hash") {
			continue
		}

		// Skip non-rule files
		if !strings.HasSuffix(name, ".rule.yaml") {
			continue
		}

		if _, exists := expectedFiles[name]; !exists {
			rulePath := filepath.Join(s.config.TargetDir, name)
			hashPath := rulePath + ".hash"

			if err := os.Remove(rulePath); err == nil {
				log.Info().Str("file", name).Msg("Removed rule no longer in index")
				deleted++
			}
			_ = os.Remove(hashPath)
		}
	}

	return deleted
}
