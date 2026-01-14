// Package community provides GitHub-based community repository integration
// for discovering, downloading, and managing rule packs.
package community

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// DefaultRepoURL is the default community repository URL
const DefaultRepoURL = "https://api.github.com/repos/freewebtopdf/asset-injector-community-rules"

// DefaultTimeout is the default HTTP timeout for API requests
const DefaultTimeout = 30 * time.Second

// ClientConfig holds configuration for the GitHub client
type ClientConfig struct {
	// RepoURL is the GitHub API URL for the community repository
	RepoURL string
	// Timeout is the HTTP request timeout
	Timeout time.Duration
	// CacheTTL is how long to cache the pack index
	CacheTTL time.Duration
	// CacheDir is the directory for caching pack index
	CacheDir string
}

// DefaultConfig returns a ClientConfig with default values
func DefaultConfig() ClientConfig {
	return ClientConfig{
		RepoURL:  DefaultRepoURL,
		Timeout:  DefaultTimeout,
		CacheTTL: 1 * time.Hour,
		CacheDir: "",
	}
}

// GitHubClient implements CommunityClient for GitHub-based repositories
type GitHubClient struct {
	config     ClientConfig
	httpClient *http.Client
	cache      *IndexCache
}

// NewGitHubClient creates a new GitHub client with the given configuration
func NewGitHubClient(config ClientConfig) *GitHubClient {
	if config.RepoURL == "" {
		config.RepoURL = DefaultRepoURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 1 * time.Hour
	}

	client := &GitHubClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}

	// Initialize cache if cache directory is provided
	if config.CacheDir != "" {
		client.cache = NewIndexCache(config.CacheDir, config.CacheTTL)
	}

	return client
}

// FetchIndex retrieves the pack index from the community repository
// It first checks the cache, then falls back to the remote repository
func (c *GitHubClient) FetchIndex(ctx context.Context) (*domain.PackIndex, error) {
	// Try cache first
	if c.cache != nil {
		if index, err := c.cache.Get(ctx); err == nil && index != nil {
			return index, nil
		}
	}

	// Fetch from remote
	index, err := c.fetchRemoteIndex(ctx)
	if err != nil {
		// Try to return cached version even if expired
		if c.cache != nil {
			if cachedIndex, cacheErr := c.cache.GetExpired(ctx); cacheErr == nil && cachedIndex != nil {
				return cachedIndex, nil
			}
		}
		return nil, err
	}

	// Update cache
	if c.cache != nil {
		_ = c.cache.Set(ctx, index)
	}

	return index, nil
}

// fetchRemoteIndex fetches the pack index from the GitHub repository
func (c *GitHubClient) fetchRemoteIndex(ctx context.Context) (*domain.PackIndex, error) {
	// Build the URL for the index file
	indexURL := c.buildIndexURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for GitHub API
	req.Header.Set("Accept", "application/vnd.github.v3.raw")
	req.Header.Set("User-Agent", "github.com/freewebtopdf/asset-injector")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch index: HTTP %d", resp.StatusCode)
	}

	// Limit response size to 10MB to prevent OOM
	const maxIndexSize = 10 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxIndexSize)

	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var index domain.PackIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}

	return &index, nil
}

// buildIndexURL constructs the URL for the pack index file
func (c *GitHubClient) buildIndexURL() string {
	// GitHub raw content URL format: /repos/{owner}/{repo}/contents/{path}
	return fmt.Sprintf("%s/contents/index.json", c.config.RepoURL)
}

// DownloadPack downloads a pack archive from the repository
func (c *GitHubClient) DownloadPack(ctx context.Context, name, version string) (io.ReadCloser, error) {
	// Build the URL for the pack archive
	archiveURL := c.buildPackArchiveURL(name, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for GitHub API
	req.Header.Set("Accept", "application/vnd.github.v3.raw")
	req.Header.Set("User-Agent", "github.com/freewebtopdf/asset-injector")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download pack: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, fmt.Errorf("pack not found: %s@%s", name, version)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to download pack: HTTP %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// buildPackArchiveURL constructs the URL for a pack archive
func (c *GitHubClient) buildPackArchiveURL(name, version string) string {
	// Pack archives are stored as: /packs/{name}/{name}-{version}.zip
	return fmt.Sprintf("%s/contents/packs/%s/%s-%s.zip", c.config.RepoURL, name, name, version)
}

// GetLatestVersion returns the latest version of a pack
func (c *GitHubClient) GetLatestVersion(ctx context.Context, name string) (string, error) {
	// Fetch the index to get version information
	index, err := c.FetchIndex(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch index: %w", err)
	}

	// Find the pack in the index
	for _, pack := range index.Packs {
		if strings.EqualFold(pack.Name, name) {
			return pack.Version, nil
		}
	}

	return "", fmt.Errorf("pack not found: %s", name)
}

// CheckUpdates compares local pack versions with remote versions
func (c *GitHubClient) CheckUpdates(ctx context.Context, installed []domain.PackInfo) ([]domain.PackUpdate, error) {
	// Fetch the remote index
	index, err := c.FetchIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index: %w", err)
	}

	// Build a map of remote pack versions
	remoteVersions := make(map[string]domain.PackInfo)
	for _, pack := range index.Packs {
		remoteVersions[strings.ToLower(pack.Name)] = pack
	}

	// Compare versions
	var updates []domain.PackUpdate
	for _, local := range installed {
		remote, exists := remoteVersions[strings.ToLower(local.Name)]
		if !exists {
			continue
		}

		// Compare versions using semantic versioning
		cmp, err := compareSemVer(local.Version, remote.Version)
		if err != nil {
			continue
		}

		if cmp < 0 {
			updates = append(updates, domain.PackUpdate{
				Name:           local.Name,
				CurrentVersion: local.Version,
				LatestVersion:  remote.Version,
				ChangelogURL:   remote.Homepage,
			})
		}
	}

	return updates, nil
}

// compareSemVer compares two semantic version strings
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareSemVer(v1, v2 string) (int, error) {
	// Parse version components
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare major, minor, patch
	for i := range 3 {
		p1, p2 := 0, 0
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 < p2 {
			return -1, nil
		}
		if p1 > p2 {
			return 1, nil
		}
	}

	return 0, nil
}

// parseVersion parses a semantic version string into components
func parseVersion(v string) []int {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// Split by dots
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		// Remove any pre-release suffix (e.g., "-beta")
		if idx := strings.IndexAny(part, "-+"); idx >= 0 {
			part = part[:idx]
		}

		var num int
		fmt.Sscanf(part, "%d", &num)
		result = append(result, num)
	}

	return result
}

// IsAvailable checks if the community repository is reachable
func (c *GitHubClient) IsAvailable(ctx context.Context) bool {
	// Try to fetch the index with a short timeout
	shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.fetchRemoteIndex(shortCtx)
	return err == nil
}
