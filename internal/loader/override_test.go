package loader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"
)

func TestOverrideManager_CreateOverride(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "override-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	overrideDir := filepath.Join(tempDir, "overrides")
	manager := NewOverrideManager(overrideDir)

	// Create a community rule
	originalRule := &domain.Rule{
		ID:          "test-rule-123",
		Type:        "exact",
		Pattern:     "https://example.com",
		CSS:         ".banner { display: none; }",
		JS:          "",
		Author:      "original-author",
		Description: "Original description",
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		Source: domain.RuleSource{
			Type:        domain.SourceCommunity,
			PackName:    "test-pack",
			PackVersion: "1.0.0",
		},
	}

	// Create a modified version
	modifiedRule := &domain.Rule{
		ID:          "test-rule-123",
		Type:        "exact",
		Pattern:     "https://example.com",
		CSS:         ".banner { display: none !important; }",
		JS:          "console.log('modified');",
		Description: "Modified description",
		UpdatedAt:   time.Now(),
	}

	// Create the override
	err = manager.CreateOverride(originalRule, modifiedRule, "modifier-user")
	if err != nil {
		t.Fatalf("CreateOverride failed: %v", err)
	}

	// Verify the override file was created
	expectedPath := filepath.Join(overrideDir, "test-pack", "test-rule-123.rule.yaml")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Override file was not created at expected path: %s", expectedPath)
	}

	// Verify the override exists check works
	if !manager.OverrideExists("test-rule-123", "test-pack") {
		t.Error("OverrideExists returned false for existing override")
	}

	// Verify non-existent override returns false
	if manager.OverrideExists("non-existent-rule", "test-pack") {
		t.Error("OverrideExists returned true for non-existent override")
	}
}

func TestOverrideManager_CreateOverride_NonCommunityRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "override-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewOverrideManager(filepath.Join(tempDir, "overrides"))

	// Create a local rule (not community)
	localRule := &domain.Rule{
		ID:      "local-rule-123",
		Type:    "exact",
		Pattern: "https://example.com",
		Source: domain.RuleSource{
			Type: domain.SourceLocal,
		},
	}

	modifiedRule := &domain.Rule{
		ID:      "local-rule-123",
		Type:    "exact",
		Pattern: "https://example.com/modified",
	}

	// Should fail for non-community rules
	err = manager.CreateOverride(localRule, modifiedRule, "modifier")
	if err == nil {
		t.Error("CreateOverride should fail for non-community rules")
	}
}

func TestOverrideManager_DeleteOverride(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "override-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	overrideDir := filepath.Join(tempDir, "overrides")
	manager := NewOverrideManager(overrideDir)

	// Create a community rule and override
	originalRule := &domain.Rule{
		ID:      "delete-test-rule",
		Type:    "exact",
		Pattern: "https://example.com",
		Author:  "original-author",
		Source: domain.RuleSource{
			Type:     domain.SourceCommunity,
			PackName: "test-pack",
		},
	}

	modifiedRule := &domain.Rule{
		ID:      "delete-test-rule",
		Type:    "exact",
		Pattern: "https://example.com",
		CSS:     ".modified { }",
	}

	err = manager.CreateOverride(originalRule, modifiedRule, "modifier")
	if err != nil {
		t.Fatalf("CreateOverride failed: %v", err)
	}

	// Verify override exists
	if !manager.OverrideExists("delete-test-rule", "test-pack") {
		t.Fatal("Override should exist after creation")
	}

	// Delete the override
	err = manager.DeleteOverride("delete-test-rule", "test-pack")
	if err != nil {
		t.Fatalf("DeleteOverride failed: %v", err)
	}

	// Verify override no longer exists
	if manager.OverrideExists("delete-test-rule", "test-pack") {
		t.Error("Override should not exist after deletion")
	}

	// Deleting non-existent override should not error
	err = manager.DeleteOverride("non-existent", "test-pack")
	if err != nil {
		t.Errorf("DeleteOverride should not error for non-existent file: %v", err)
	}
}

func TestOverrideManager_GetOverridePath(t *testing.T) {
	manager := NewOverrideManager("/base/overrides")

	tests := []struct {
		name     string
		rule     *domain.Rule
		expected string
	}{
		{
			name: "with pack name",
			rule: &domain.Rule{
				ID: "rule-123",
				Source: domain.RuleSource{
					PackName: "my-pack",
				},
			},
			expected: filepath.Join("/base/overrides", "my-pack", "rule-123.rule.yaml"),
		},
		{
			name: "without pack name",
			rule: &domain.Rule{
				ID:     "rule-456",
				Source: domain.RuleSource{},
			},
			expected: filepath.Join("/base/overrides", "rule-456.rule.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.GetOverridePath(tt.rule)
			if result != tt.expected {
				t.Errorf("GetOverridePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsCommunityRule(t *testing.T) {
	tests := []struct {
		name     string
		rule     *domain.Rule
		expected bool
	}{
		{
			name: "community rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceCommunity},
			},
			expected: true,
		},
		{
			name: "local rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceLocal},
			},
			expected: false,
		},
		{
			name: "override rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceOverride},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCommunityRule(tt.rule); got != tt.expected {
				t.Errorf("IsCommunityRule() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsOverrideRule(t *testing.T) {
	tests := []struct {
		name     string
		rule     *domain.Rule
		expected bool
	}{
		{
			name: "override rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceOverride},
			},
			expected: true,
		},
		{
			name: "community rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceCommunity},
			},
			expected: false,
		},
		{
			name: "local rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceLocal},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOverrideRule(tt.rule); got != tt.expected {
				t.Errorf("IsOverrideRule() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsLocalRule(t *testing.T) {
	tests := []struct {
		name     string
		rule     *domain.Rule
		expected bool
	}{
		{
			name: "local rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceLocal},
			},
			expected: true,
		},
		{
			name: "community rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceCommunity},
			},
			expected: false,
		},
		{
			name: "override rule",
			rule: &domain.Rule{
				Source: domain.RuleSource{Type: domain.SourceOverride},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLocalRule(tt.rule); got != tt.expected {
				t.Errorf("IsLocalRule() = %v, want %v", got, tt.expected)
			}
		})
	}
}
