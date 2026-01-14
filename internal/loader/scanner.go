// Package loader provides file-based rule loading functionality for the Asset Injector.
// It supports loading rules from YAML and JSON files organized in configurable directories.
package loader

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// ValidRuleExtensions defines the file extensions recognized as rule files
var ValidRuleExtensions = []string{".rule.yaml", ".rule.json"}

// ScanConfig holds configuration for directory scanning
type ScanConfig struct {
	LocalDir     string // Directory for user's custom rules (highest priority)
	CommunityDir string // Directory for installed community packs
	OverrideDir  string // Directory for local modifications to community rules
}

// ScannedFile represents a discovered rule file with its source information
type ScannedFile struct {
	Path       string            // Full path to the file
	SourceType domain.SourceType // Source type (local, community, override)
	PackName   string            // Pack name if from community directory
}

// Scanner handles recursive directory scanning for rule files
type Scanner struct {
	config ScanConfig
}

// NewScanner creates a new Scanner with the given configuration
func NewScanner(config ScanConfig) *Scanner {
	return &Scanner{config: config}
}

// Scan recursively scans all configured directories for rule files
// Returns a slice of ScannedFile with source information for each discovered file
func (s *Scanner) Scan(ctx context.Context) ([]ScannedFile, error) {
	var files []ScannedFile

	// Scan local directory (highest priority)
	if s.config.LocalDir != "" {
		localFiles, err := s.scanDirectory(ctx, s.config.LocalDir, domain.SourceLocal, "")
		if err != nil {
			// Log warning but continue - directory might not exist yet
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		files = append(files, localFiles...)
	}

	// Scan override directory (second priority)
	if s.config.OverrideDir != "" {
		overrideFiles, err := s.scanDirectory(ctx, s.config.OverrideDir, domain.SourceOverride, "")
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		files = append(files, overrideFiles...)
	}

	// Scan community directory (lowest priority)
	if s.config.CommunityDir != "" {
		communityFiles, err := s.scanCommunityDirectory(ctx)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		files = append(files, communityFiles...)
	}

	return files, nil
}

// scanDirectory recursively scans a directory for rule files
func (s *Scanner) scanDirectory(ctx context.Context, rootDir string, sourceType domain.SourceType, packName string) ([]ScannedFile, error) {
	var files []ScannedFile

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// Skip inaccessible directories/files but continue scanning
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if file has a valid rule extension
		if !isRuleFile(path) {
			return nil
		}

		files = append(files, ScannedFile{
			Path:       path,
			SourceType: sourceType,
			PackName:   packName,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// scanCommunityDirectory scans the community directory, detecting pack names from subdirectories
func (s *Scanner) scanCommunityDirectory(ctx context.Context) ([]ScannedFile, error) {
	var files []ScannedFile

	// Check if community directory exists
	entries, err := os.ReadDir(s.config.CommunityDir)
	if err != nil {
		return nil, err
	}

	// Each subdirectory in community is a pack
	for _, entry := range entries {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !entry.IsDir() {
			continue
		}

		packName := entry.Name()
		packDir := filepath.Join(s.config.CommunityDir, packName)

		// Scan the pack directory for rule files
		packFiles, err := s.scanDirectory(ctx, packDir, domain.SourceCommunity, packName)
		if err != nil {
			// Log warning but continue with other packs
			continue
		}

		files = append(files, packFiles...)
	}

	return files, nil
}

// isRuleFile checks if a file path has a valid rule file extension
func isRuleFile(path string) bool {
	lowerPath := strings.ToLower(path)
	for _, ext := range ValidRuleExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	return false
}

// ScanSingleDirectory scans a single directory without source type inference
// Useful for testing or scanning arbitrary directories
func (s *Scanner) ScanSingleDirectory(ctx context.Context, dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !isRuleFile(path) {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
