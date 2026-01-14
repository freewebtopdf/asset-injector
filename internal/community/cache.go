package community

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

// CacheFileName is the name of the cached index file
const CacheFileName = "pack-index.cache.json"

// CacheMetaFileName is the name of the cache metadata file
const CacheMetaFileName = "pack-index.cache.meta.json"

// IndexCache provides local caching for the pack index
// to support offline operation and reduce API calls
type IndexCache struct {
	cacheDir string
	ttl      time.Duration
	mu       sync.RWMutex
}

// CacheMeta holds metadata about the cached index
type CacheMeta struct {
	CachedAt  time.Time `json:"cached_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Version   string    `json:"version,omitempty"`
}

// NewIndexCache creates a new IndexCache with the given directory and TTL
func NewIndexCache(cacheDir string, ttl time.Duration) *IndexCache {
	return &IndexCache{
		cacheDir: cacheDir,
		ttl:      ttl,
	}
}

// Get retrieves the cached index if it exists and is not expired
func (c *IndexCache) Get(ctx context.Context) (*domain.PackIndex, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if cache is valid
	meta, err := c.loadMeta()
	if err != nil {
		return nil, fmt.Errorf("cache miss: %w", err)
	}

	if time.Now().After(meta.ExpiresAt) {
		return nil, fmt.Errorf("cache expired")
	}

	// Load the cached index
	return c.loadIndex()
}

// GetExpired retrieves the cached index even if expired
// This is useful as a fallback when the remote repository is unavailable
func (c *IndexCache) GetExpired(ctx context.Context) (*domain.PackIndex, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.loadIndex()
}

// Set stores the index in the cache
func (c *IndexCache) Set(ctx context.Context, index *domain.PackIndex) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure cache directory exists
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Save the index
	if err := c.saveIndex(index); err != nil {
		return err
	}

	// Save metadata
	meta := CacheMeta{
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
		Version:   index.Version,
	}

	return c.saveMeta(&meta)
}

// Invalidate removes the cached index
func (c *IndexCache) Invalidate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexPath := filepath.Join(c.cacheDir, CacheFileName)
	metaPath := filepath.Join(c.cacheDir, CacheMetaFileName)

	// Remove both files, ignoring errors if they don't exist
	_ = os.Remove(indexPath)
	_ = os.Remove(metaPath)

	return nil
}

// IsValid checks if the cache is valid (exists and not expired)
func (c *IndexCache) IsValid(ctx context.Context) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	meta, err := c.loadMeta()
	if err != nil {
		return false
	}

	return time.Now().Before(meta.ExpiresAt)
}

// GetMeta returns the cache metadata
func (c *IndexCache) GetMeta(ctx context.Context) (*CacheMeta, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.loadMeta()
}

// loadIndex loads the cached index from disk
func (c *IndexCache) loadIndex() (*domain.PackIndex, error) {
	indexPath := filepath.Join(c.cacheDir, CacheFileName)

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var index domain.PackIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	return &index, nil
}

// saveIndex saves the index to disk
func (c *IndexCache) saveIndex(index *domain.PackIndex) error {
	indexPath := filepath.Join(c.cacheDir, CacheFileName)

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write atomically using temp file
	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	if err := os.Rename(tmpPath, indexPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	return nil
}

// loadMeta loads the cache metadata from disk
func (c *IndexCache) loadMeta() (*CacheMeta, error) {
	metaPath := filepath.Join(c.cacheDir, CacheMetaFileName)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta file: %w", err)
	}

	var meta CacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse meta file: %w", err)
	}

	return &meta, nil
}

// saveMeta saves the cache metadata to disk
func (c *IndexCache) saveMeta(meta *CacheMeta) error {
	metaPath := filepath.Join(c.cacheDir, CacheMetaFileName)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}

	// Write atomically using temp file
	tmpPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write meta file: %w", err)
	}

	if err := os.Rename(tmpPath, metaPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename meta file: %w", err)
	}

	return nil
}

// TTL returns the cache TTL
func (c *IndexCache) TTL() time.Duration {
	return c.ttl
}

// SetTTL updates the cache TTL
func (c *IndexCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}
