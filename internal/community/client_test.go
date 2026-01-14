package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

func TestNewGitHubClient(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		config := DefaultConfig()
		client := NewGitHubClient(config)

		if client == nil {
			t.Fatal("expected client to be created")
		}
		if client.config.RepoURL != DefaultRepoURL {
			t.Errorf("expected RepoURL %s, got %s", DefaultRepoURL, client.config.RepoURL)
		}
		if client.config.Timeout != DefaultTimeout {
			t.Errorf("expected Timeout %v, got %v", DefaultTimeout, client.config.Timeout)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := ClientConfig{
			RepoURL: "https://custom.repo.com",
			Timeout: 60 * time.Second,
		}
		client := NewGitHubClient(config)

		if client.config.RepoURL != "https://custom.repo.com" {
			t.Errorf("expected custom RepoURL, got %s", client.config.RepoURL)
		}
		if client.config.Timeout != 60*time.Second {
			t.Errorf("expected custom Timeout, got %v", client.config.Timeout)
		}
	})

	t.Run("with empty config uses defaults", func(t *testing.T) {
		config := ClientConfig{}
		client := NewGitHubClient(config)

		if client.config.RepoURL != DefaultRepoURL {
			t.Errorf("expected default RepoURL, got %s", client.config.RepoURL)
		}
		if client.config.Timeout != DefaultTimeout {
			t.Errorf("expected default Timeout, got %v", client.config.Timeout)
		}
	})
}

func TestFetchIndex(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		// Create mock server
		index := domain.PackIndex{
			Version:   "1.0.0",
			UpdatedAt: time.Now(),
			Packs: []domain.PackInfo{
				{Name: "test-pack", Version: "1.0.0", Description: "Test pack"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		result, err := client.FetchIndex(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Version != "1.0.0" {
			t.Errorf("expected version 1.0.0, got %s", result.Version)
		}
		if len(result.Packs) != 1 {
			t.Errorf("expected 1 pack, got %d", len(result.Packs))
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		_, err := client.FetchIndex(context.Background())
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		_, err := client.FetchIndex(context.Background())
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestDownloadPack(t *testing.T) {
	t.Run("successful download", func(t *testing.T) {
		packContent := []byte("mock zip content")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/zip")
			w.Write(packContent)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		reader, err := client.DownloadPack(context.Background(), "test-pack", "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		// Read content
		buf := make([]byte, len(packContent))
		n, _ := reader.Read(buf)
		if string(buf[:n]) != string(packContent) {
			t.Errorf("expected content %s, got %s", packContent, buf[:n])
		}
	})

	t.Run("pack not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		_, err := client.DownloadPack(context.Background(), "nonexistent", "1.0.0")
		if err == nil {
			t.Fatal("expected error for not found pack")
		}
	})
}

func TestGetLatestVersion(t *testing.T) {
	t.Run("pack exists", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "test-pack", Version: "2.1.0"},
				{Name: "other-pack", Version: "1.0.0"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		version, err := client.GetLatestVersion(context.Background(), "test-pack")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version != "2.1.0" {
			t.Errorf("expected version 2.1.0, got %s", version)
		}
	})

	t.Run("pack not found", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs:   []domain.PackInfo{},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		_, err := client.GetLatestVersion(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent pack")
		}
	})

	t.Run("case insensitive pack name", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "Test-Pack", Version: "1.5.0"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		version, err := client.GetLatestVersion(context.Background(), "test-pack")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version != "1.5.0" {
			t.Errorf("expected version 1.5.0, got %s", version)
		}
	})
}

func TestCheckUpdates(t *testing.T) {
	t.Run("updates available", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "pack-a", Version: "2.0.0"},
				{Name: "pack-b", Version: "1.5.0"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		installed := []domain.PackInfo{
			{Name: "pack-a", Version: "1.0.0"},
			{Name: "pack-b", Version: "1.5.0"},
		}

		updates, err := client.CheckUpdates(context.Background(), installed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}
		if updates[0].Name != "pack-a" {
			t.Errorf("expected pack-a update, got %s", updates[0].Name)
		}
		if updates[0].CurrentVersion != "1.0.0" {
			t.Errorf("expected current version 1.0.0, got %s", updates[0].CurrentVersion)
		}
		if updates[0].LatestVersion != "2.0.0" {
			t.Errorf("expected latest version 2.0.0, got %s", updates[0].LatestVersion)
		}
	})

	t.Run("no updates available", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "pack-a", Version: "1.0.0"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		installed := []domain.PackInfo{
			{Name: "pack-a", Version: "1.0.0"},
		}

		updates, err := client.CheckUpdates(context.Background(), installed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(updates) != 0 {
			t.Errorf("expected no updates, got %d", len(updates))
		}
	})
}

func TestCompareSemVer(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.0.0", "1.0.1", -1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0-beta", "1.0.0", 0},
		{"1.0", "1.0.0", 0},
		{"1", "1.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			result, err := compareSemVer(tt.v1, tt.v2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("compareSemVer(%s, %s) = %d, expected %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected []int
	}{
		{"1.0.0", []int{1, 0, 0}},
		{"2.1.3", []int{2, 1, 3}},
		{"v1.0.0", []int{1, 0, 0}},
		{"1.0.0-beta", []int{1, 0, 0}},
		{"1.0.0+build", []int{1, 0, 0}},
		{"1.0", []int{1, 0}},
		{"1", []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := parseVersion(tt.version)
			if len(result) != len(tt.expected) {
				t.Fatalf("parseVersion(%s) length = %d, expected %d", tt.version, len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("parseVersion(%s)[%d] = %d, expected %d", tt.version, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestIsAvailable(t *testing.T) {
	t.Run("server available", func(t *testing.T) {
		index := domain.PackIndex{Version: "1.0.0"}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(index)
		}))
		defer server.Close()

		config := ClientConfig{
			RepoURL: server.URL,
			Timeout: 5 * time.Second,
		}
		client := NewGitHubClient(config)

		if !client.IsAvailable(context.Background()) {
			t.Error("expected server to be available")
		}
	})

	t.Run("server unavailable", func(t *testing.T) {
		config := ClientConfig{
			RepoURL: "http://localhost:99999",
			Timeout: 1 * time.Second,
		}
		client := NewGitHubClient(config)

		if client.IsAvailable(context.Background()) {
			t.Error("expected server to be unavailable")
		}
	})
}
