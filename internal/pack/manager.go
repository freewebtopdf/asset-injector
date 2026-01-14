package pack

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// SourceFileName is the name of the file that tracks pack source information
const SourceFileName = ".source.json"

// PackManager handles rule pack operations including installation, updates, and removal
type PackManager struct {
	communityDir string
	overrideDir  string
	parser       *ManifestParser
	validator    *ManifestValidator
	namespacer   *Namespacer
	client       CommunityClient
	mu           sync.RWMutex
}

// CommunityClient defines the interface for community repository interactions
type CommunityClient interface {
	// FetchIndex retrieves the pack index from the repository
	FetchIndex(ctx context.Context) (*domain.PackIndex, error)
	// DownloadPack downloads a pack archive
	DownloadPack(ctx context.Context, name, version string) (io.ReadCloser, error)
	// GetLatestVersion returns the latest version of a pack
	GetLatestVersion(ctx context.Context, name string) (string, error)
}

// ManagerConfig holds configuration for the PackManager
type ManagerConfig struct {
	CommunityDir string
	OverrideDir  string
}

// NewPackManager creates a new PackManager with the given configuration
func NewPackManager(config ManagerConfig, client CommunityClient) *PackManager {
	return &PackManager{
		communityDir: config.CommunityDir,
		overrideDir:  config.OverrideDir,
		parser:       NewManifestParser(),
		validator:    NewManifestValidator(),
		namespacer:   NewNamespacer(),
		client:       client,
	}
}

// ListInstalled returns all installed packs with their metadata
func (m *PackManager) ListInstalled(ctx context.Context) ([]domain.PackInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.communityDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(m.communityDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read community directory: %w", err)
	}

	var packs []domain.PackInfo
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !entry.IsDir() {
			continue
		}

		packDir := filepath.Join(m.communityDir, entry.Name())
		packInfo, err := m.getPackInfo(packDir)
		if err != nil {
			// Skip invalid packs but continue
			continue
		}

		packs = append(packs, *packInfo)
	}

	return packs, nil
}

// getPackInfo reads pack information from a pack directory
func (m *PackManager) getPackInfo(packDir string) (*domain.PackInfo, error) {
	manifestPath := filepath.Join(packDir, ManifestFileName)
	manifest, err := m.parser.ParseFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Count rules in the pack
	ruleCount := m.countRulesInPack(packDir)

	info := ManifestToPackInfo(manifest, ruleCount)

	// Load source information if available
	sourcePath := filepath.Join(packDir, SourceFileName)
	if source, err := m.loadPackSource(sourcePath); err == nil {
		info.SourceURL = source.SourceURL
		info.InstalledAt = source.InstalledAt
	}

	return &info, nil
}

// countRulesInPack counts the number of rule files in a pack directory
func (m *PackManager) countRulesInPack(packDir string) int {
	count := 0
	_ = filepath.WalkDir(packDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isRuleFile(path) {
			count++
		}
		return nil
	})
	return count
}

// isRuleFile checks if a file is a rule file based on extension
func isRuleFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".json"
}

// InstallResult contains the result of a pack installation
type InstallResult struct {
	PackName           string                 `json:"pack_name"`
	Version            string                 `json:"version"`
	DependencyWarnings []string               `json:"dependency_warnings,omitempty"`
	DependencyCheck    *DependencyCheckResult `json:"dependency_check,omitempty"`
}

// Install downloads and installs a pack from the specified source
func (m *PackManager) Install(ctx context.Context, source string) error {
	_, err := m.InstallWithResult(ctx, source)
	return err
}

// InstallWithResult downloads and installs a pack, returning detailed results
func (m *PackManager) InstallWithResult(ctx context.Context, source string) (*InstallResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil, fmt.Errorf("community client not configured")
	}

	// Parse source to get pack name and optional version
	packName, version := parsePackSource(source)

	// Get latest version if not specified
	if version == "" {
		var err error
		version, err = m.client.GetLatestVersion(ctx, packName)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	// Download the pack
	reader, err := m.client.DownloadPack(ctx, packName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to download pack: %w", err)
	}
	defer reader.Close()

	// Create pack directory
	packDir := filepath.Join(m.communityDir, packName)
	if err := os.MkdirAll(packDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pack directory: %w", err)
	}

	// Extract pack contents
	if err := m.extractPack(reader, packDir); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(packDir)
		return nil, fmt.Errorf("failed to extract pack: %w", err)
	}

	// Validate the installed pack
	manifestPath := filepath.Join(packDir, ManifestFileName)
	manifest, err := m.parser.ParseFile(manifestPath)
	if err != nil {
		_ = os.RemoveAll(packDir)
		return nil, fmt.Errorf("invalid pack manifest: %w", err)
	}

	if err := m.validator.Validate(manifest); err != nil {
		_ = os.RemoveAll(packDir)
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}

	// Save source information
	packSource := domain.PackSource{
		SourceURL:   source,
		Version:     version,
		InstalledAt: time.Now(),
	}
	if err := m.savePackSource(filepath.Join(packDir, SourceFileName), &packSource); err != nil {
		// Non-fatal, continue
	}

	result := &InstallResult{
		PackName: packName,
		Version:  version,
	}

	// Check dependencies (warn but don't fail)
	depChecker := NewDependencyChecker(m)
	depResult, err := depChecker.CheckDependencies(ctx, manifest)
	if err == nil {
		result.DependencyCheck = depResult
		result.DependencyWarnings = depResult.Warnings
	}

	return result, nil
}

// parsePackSource parses a pack source string into name and version
// Supports formats: "packname", "packname@version"
func parsePackSource(source string) (name, version string) {
	parts := splitAtSign(source)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return source, ""
}

// splitAtSign splits a string at the @ character
func splitAtSign(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// extractPack extracts a pack archive to the target directory
func (m *PackManager) extractPack(reader io.Reader, targetDir string) error {
	// Create a temporary file to store the archive
	tmpFile, err := os.CreateTemp("", "pack-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy archive to temp file
	if _, err := io.Copy(tmpFile, reader); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Open as zip
	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer zipReader.Close()

	// Extract files
	for _, file := range zipReader.File {
		if err := m.extractZipFile(file, targetDir); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive
func (m *PackManager) extractZipFile(file *zip.File, targetDir string) error {
	// Sanitize path to prevent zip slip
	destPath := filepath.Join(targetDir, file.Name)
	if !isSubPath(targetDir, destPath) {
		return fmt.Errorf("invalid file path in archive: %s", file.Name)
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(destPath, file.Mode())
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Extract file
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// isSubPath checks if child is a subpath of parent
func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && !hasPathPrefix(rel, "..")
}

// hasPathPrefix checks if a path starts with a dangerous prefix
func hasPathPrefix(path, prefix string) bool {
	// Check for exact prefix match or prefix followed by separator
	if path == prefix {
		return true
	}
	if len(path) > len(prefix) && path[:len(prefix)] == prefix && (path[len(prefix)] == '/' || path[len(prefix)] == filepath.Separator) {
		return true
	}
	// Also check for embedded ..
	return strings.Contains(path, prefix+"/") || strings.Contains(path, prefix+string(filepath.Separator))
}

// Uninstall removes an installed pack
func (m *PackManager) Uninstall(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	packDir := filepath.Join(m.communityDir, name)

	// Check if pack exists
	if _, err := os.Stat(packDir); os.IsNotExist(err) {
		return fmt.Errorf("pack not found: %s", name)
	}

	// Remove the pack directory
	if err := os.RemoveAll(packDir); err != nil {
		return fmt.Errorf("failed to remove pack: %w", err)
	}

	// Also remove any overrides for this pack
	if m.overrideDir != "" {
		overridePackDir := filepath.Join(m.overrideDir, name)
		_ = os.RemoveAll(overridePackDir) // Ignore errors
	}

	return nil
}

// Update updates a pack to the latest version
func (m *PackManager) Update(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return fmt.Errorf("community client not configured")
	}

	packDir := filepath.Join(m.communityDir, name)

	// Check if pack exists
	if _, err := os.Stat(packDir); os.IsNotExist(err) {
		return fmt.Errorf("pack not found: %s", name)
	}

	// Get current version
	currentInfo, err := m.getPackInfo(packDir)
	if err != nil {
		return fmt.Errorf("failed to get current pack info: %w", err)
	}

	// Get latest version
	latestVersion, err := m.client.GetLatestVersion(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	// Check if update is needed
	cmp, err := CompareSemVer(currentInfo.Version, latestVersion)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}
	if cmp >= 0 {
		// Already at latest version
		return nil
	}

	// Backup overrides before update
	if err := m.backupOverrides(name); err != nil {
		// Non-fatal, continue
	}

	// Download and extract new version
	reader, err := m.client.DownloadPack(ctx, name, latestVersion)
	if err != nil {
		return fmt.Errorf("failed to download pack: %w", err)
	}
	defer reader.Close()

	// Create temp directory for new version
	tmpDir, err := os.MkdirTemp("", "pack-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract to temp directory
	if err := m.extractPack(reader, tmpDir); err != nil {
		return fmt.Errorf("failed to extract pack: %w", err)
	}

	// Validate new version
	manifestPath := filepath.Join(tmpDir, ManifestFileName)
	manifest, err := m.parser.ParseFile(manifestPath)
	if err != nil {
		return fmt.Errorf("invalid pack manifest: %w", err)
	}

	if err := m.validator.Validate(manifest); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	// Remove old version and move new version in place
	if err := os.RemoveAll(packDir); err != nil {
		return fmt.Errorf("failed to remove old version: %w", err)
	}

	if err := os.Rename(tmpDir, packDir); err != nil {
		// Try copy if rename fails (cross-device)
		if err := copyDir(tmpDir, packDir); err != nil {
			return fmt.Errorf("failed to install new version: %w", err)
		}
	}

	// Update source information
	sourcePath := filepath.Join(packDir, SourceFileName)
	source, _ := m.loadPackSource(sourcePath)
	if source == nil {
		source = &domain.PackSource{}
	}
	source.Version = latestVersion
	source.UpdatedAt = time.Now()
	_ = m.savePackSource(sourcePath, source)

	return nil
}

// backupOverrides backs up override files for a pack
func (m *PackManager) backupOverrides(packName string) error {
	if m.overrideDir == "" {
		return nil
	}

	overridePackDir := filepath.Join(m.overrideDir, packName)
	if _, err := os.Stat(overridePackDir); os.IsNotExist(err) {
		return nil
	}

	// Overrides are preserved in place, no backup needed
	// They will continue to work after update
	return nil
}

// CheckUpdates returns packs with available updates
func (m *PackManager) CheckUpdates(ctx context.Context) ([]domain.PackUpdate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.client == nil {
		return nil, fmt.Errorf("community client not configured")
	}

	installed, err := m.ListInstalled(ctx)
	if err != nil {
		return nil, err
	}

	var updates []domain.PackUpdate
	for _, pack := range installed {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		latestVersion, err := m.client.GetLatestVersion(ctx, pack.Name)
		if err != nil {
			continue // Skip packs we can't check
		}

		cmp, err := CompareSemVer(pack.Version, latestVersion)
		if err != nil {
			continue
		}

		if cmp < 0 {
			updates = append(updates, domain.PackUpdate{
				Name:           pack.Name,
				CurrentVersion: pack.Version,
				LatestVersion:  latestVersion,
			})
		}
	}

	return updates, nil
}

// loadPackSource loads pack source information from a .source.json file
func (m *PackManager) loadPackSource(path string) (*domain.PackSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var source domain.PackSource
	if err := json.Unmarshal(data, &source); err != nil {
		return nil, err
	}

	return &source, nil
}

// savePackSource saves pack source information to a .source.json file
func (m *PackManager) savePackSource(path string, source *domain.PackSource) error {
	data, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetPackSource returns the source information for an installed pack
func (m *PackManager) GetPackSource(ctx context.Context, name string) (*domain.PackSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	packDir := filepath.Join(m.communityDir, name)
	sourcePath := filepath.Join(packDir, SourceFileName)

	return m.loadPackSource(sourcePath)
}

// GetPackManifest returns the manifest for an installed pack
func (m *PackManager) GetPackManifest(ctx context.Context, name string) (*domain.PackManifest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	packDir := filepath.Join(m.communityDir, name)
	manifestPath := filepath.Join(packDir, ManifestFileName)

	return m.parser.ParseFile(manifestPath)
}

// IsInstalled checks if a pack is installed
func (m *PackManager) IsInstalled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	packDir := filepath.Join(m.communityDir, name)
	_, err := os.Stat(packDir)
	return err == nil
}

// GetInstalledVersion returns the installed version of a pack
func (m *PackManager) GetInstalledVersion(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	packDir := filepath.Join(m.communityDir, name)
	manifestPath := filepath.Join(packDir, ManifestFileName)

	manifest, err := m.parser.ParseFile(manifestPath)
	if err != nil {
		return "", err
	}

	return manifest.Version, nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// ListAvailable fetches available packs from the community repository
func (m *PackManager) ListAvailable(ctx context.Context) ([]domain.PackInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("community client not configured")
	}

	index, err := m.client.FetchIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pack index: %w", err)
	}

	return index.Packs, nil
}
