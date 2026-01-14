package community

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

func TestIndexCache(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Hour)

		index := &domain.PackIndex{
			Version:   "1.0.0",
			UpdatedAt: time.Now(),
			Packs: []domain.PackInfo{
				{Name: "test-pack", Version: "1.0.0"},
			},
		}

		ctx := context.Background()

		// Set the cache
		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Get from cache
		result, err := cache.Get(ctx)
		if err != nil {
			t.Fatalf("failed to get cache: %v", err)
		}

		if result.Version != "1.0.0" {
			t.Errorf("expected version 1.0.0, got %s", result.Version)
		}
		if len(result.Packs) != 1 {
			t.Errorf("expected 1 pack, got %d", len(result.Packs))
		}
	})

	t.Run("cache miss when empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Hour)

		ctx := context.Background()

		_, err := cache.Get(ctx)
		if err == nil {
			t.Error("expected cache miss error")
		}
	})

	t.Run("cache expired", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Millisecond)

		index := &domain.PackIndex{
			Version: "1.0.0",
		}

		ctx := context.Background()

		// Set the cache
		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Get should fail due to expiration
		_, err = cache.Get(ctx)
		if err == nil {
			t.Error("expected cache expired error")
		}
	})

	t.Run("get expired returns cached data", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Millisecond)

		index := &domain.PackIndex{
			Version: "1.0.0",
		}

		ctx := context.Background()

		// Set the cache
		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// GetExpired should still return data
		result, err := cache.GetExpired(ctx)
		if err != nil {
			t.Fatalf("failed to get expired cache: %v", err)
		}

		if result.Version != "1.0.0" {
			t.Errorf("expected version 1.0.0, got %s", result.Version)
		}
	})

	t.Run("invalidate removes cache", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Hour)

		index := &domain.PackIndex{
			Version: "1.0.0",
		}

		ctx := context.Background()

		// Set the cache
		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Invalidate
		err = cache.Invalidate(ctx)
		if err != nil {
			t.Fatalf("failed to invalidate cache: %v", err)
		}

		// Get should fail
		_, err = cache.Get(ctx)
		if err == nil {
			t.Error("expected cache miss after invalidation")
		}
	})

	t.Run("is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Hour)

		ctx := context.Background()

		// Initially not valid
		if cache.IsValid(ctx) {
			t.Error("expected cache to be invalid initially")
		}

		// Set cache
		index := &domain.PackIndex{Version: "1.0.0"}
		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Now should be valid
		if !cache.IsValid(ctx) {
			t.Error("expected cache to be valid after set")
		}
	})

	t.Run("get meta", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Hour)

		index := &domain.PackIndex{
			Version: "2.0.0",
		}

		ctx := context.Background()

		// Set the cache
		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Get meta
		meta, err := cache.GetMeta(ctx)
		if err != nil {
			t.Fatalf("failed to get meta: %v", err)
		}

		if meta.Version != "2.0.0" {
			t.Errorf("expected version 2.0.0, got %s", meta.Version)
		}
		if meta.CachedAt.IsZero() {
			t.Error("expected CachedAt to be set")
		}
		if meta.ExpiresAt.IsZero() {
			t.Error("expected ExpiresAt to be set")
		}
	})

	t.Run("creates cache directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "nested", "cache", "dir")
		cache := NewIndexCache(cacheDir, 1*time.Hour)

		index := &domain.PackIndex{Version: "1.0.0"}

		ctx := context.Background()

		err := cache.Set(ctx, index)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}

		// Verify directory was created
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			t.Error("expected cache directory to be created")
		}
	})

	t.Run("ttl getter and setter", func(t *testing.T) {
		tmpDir := t.TempDir()
		cache := NewIndexCache(tmpDir, 1*time.Hour)

		if cache.TTL() != 1*time.Hour {
			t.Errorf("expected TTL 1h, got %v", cache.TTL())
		}

		cache.SetTTL(2 * time.Hour)

		if cache.TTL() != 2*time.Hour {
			t.Errorf("expected TTL 2h, got %v", cache.TTL())
		}
	})
}

func TestIndexCacheConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewIndexCache(tmpDir, 1*time.Hour)

	ctx := context.Background()

	// Set initial cache
	index := &domain.PackIndex{Version: "1.0.0"}
	err := cache.Set(ctx, index)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				if n%2 == 0 {
					_, _ = cache.Get(ctx)
				} else {
					idx := &domain.PackIndex{Version: "1.0.0"}
					_ = cache.Set(ctx, idx)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
