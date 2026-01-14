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

func TestVersionChecker(t *testing.T) {
	t.Run("check pack updates", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "pack-a", Version: "2.0.0"},
				{Name: "pack-b", Version: "1.0.0"},
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
		checker := NewVersionChecker(client)

		installed := []domain.PackInfo{
			{Name: "pack-a", Version: "1.0.0"},
			{Name: "pack-b", Version: "1.0.0"},
		}

		updates, err := checker.CheckPackUpdates(context.Background(), installed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(updates) != 1 {
			t.Fatalf("expected 1 update, got %d", len(updates))
		}
		if updates[0].Name != "pack-a" {
			t.Errorf("expected pack-a, got %s", updates[0].Name)
		}
	})

	t.Run("get latest version", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "test-pack", Version: "3.2.1"},
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
		checker := NewVersionChecker(client)

		version, err := checker.GetLatestVersion(context.Background(), "test-pack")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if version != "3.2.1" {
			t.Errorf("expected 3.2.1, got %s", version)
		}
	})

	t.Run("is update available", func(t *testing.T) {
		index := domain.PackIndex{
			Version: "1.0.0",
			Packs: []domain.PackInfo{
				{Name: "test-pack", Version: "2.0.0"},
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
		checker := NewVersionChecker(client)

		// Update available
		available, latest, err := checker.IsUpdateAvailable(context.Background(), "test-pack", "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !available {
			t.Error("expected update to be available")
		}
		if latest != "2.0.0" {
			t.Errorf("expected latest 2.0.0, got %s", latest)
		}

		// No update available
		available, _, err = checker.IsUpdateAvailable(context.Background(), "test-pack", "2.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if available {
			t.Error("expected no update available")
		}
	})
}

func TestParseSemVer(t *testing.T) {
	tests := []struct {
		version string
		major   int
		minor   int
		patch   int
	}{
		{"1.0.0", 1, 0, 0},
		{"2.3.4", 2, 3, 4},
		{"v1.2.3", 1, 2, 3},
		{"1.0", 1, 0, 0},
		{"1", 1, 0, 0},
		{"10.20.30", 10, 20, 30},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor, patch, err := ParseSemVer(tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if major != tt.major {
				t.Errorf("expected major %d, got %d", tt.major, major)
			}
			if minor != tt.minor {
				t.Errorf("expected minor %d, got %d", tt.minor, minor)
			}
			if patch != tt.patch {
				t.Errorf("expected patch %d, got %d", tt.patch, patch)
			}
		})
	}
}

func TestSatisfiesConstraint(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		expected   bool
	}{
		// Greater than or equal
		{"1.0.0", ">=1.0.0", true},
		{"2.0.0", ">=1.0.0", true},
		{"0.9.0", ">=1.0.0", false},

		// Less than or equal
		{"1.0.0", "<=1.0.0", true},
		{"0.9.0", "<=1.0.0", true},
		{"2.0.0", "<=1.0.0", false},

		// Greater than
		{"2.0.0", ">1.0.0", true},
		{"1.0.0", ">1.0.0", false},
		{"0.9.0", ">1.0.0", false},

		// Less than
		{"0.9.0", "<1.0.0", true},
		{"1.0.0", "<1.0.0", false},
		{"2.0.0", "<1.0.0", false},

		// Equal
		{"1.0.0", "=1.0.0", true},
		{"1.0.1", "=1.0.0", false},
		{"0.9.0", "=1.0.0", false},

		// No operator (exact match)
		{"1.0.0", "1.0.0", true},
		{"1.0.1", "1.0.0", false},

		// Wildcard
		{"1.0.0", "*", true},
		{"2.0.0", "*", true},

		// Empty constraint
		{"1.0.0", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.version+"_"+tt.constraint, func(t *testing.T) {
			result, err := SatisfiesConstraint(tt.version, tt.constraint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("SatisfiesConstraint(%s, %s) = %v, expected %v", tt.version, tt.constraint, result, tt.expected)
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		major    int
		minor    int
		patch    int
		expected string
	}{
		{1, 0, 0, "1.0.0"},
		{2, 3, 4, "2.3.4"},
		{10, 20, 30, "10.20.30"},
		{0, 0, 1, "0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatVersion(tt.major, tt.minor, tt.patch)
			if result != tt.expected {
				t.Errorf("FormatVersion(%d, %d, %d) = %s, expected %s", tt.major, tt.minor, tt.patch, result, tt.expected)
			}
		})
	}
}

func TestIncrementVersion(t *testing.T) {
	t.Run("increment patch", func(t *testing.T) {
		result, err := IncrementPatch("1.2.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "1.2.4" {
			t.Errorf("expected 1.2.4, got %s", result)
		}
	})

	t.Run("increment minor", func(t *testing.T) {
		result, err := IncrementMinor("1.2.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "1.3.0" {
			t.Errorf("expected 1.3.0, got %s", result)
		}
	})

	t.Run("increment major", func(t *testing.T) {
		result, err := IncrementMajor("1.2.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "2.0.0" {
			t.Errorf("expected 2.0.0, got %s", result)
		}
	})
}
