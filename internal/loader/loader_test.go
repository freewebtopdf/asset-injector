package loader

import (
	"github.com/freewebtopdf/asset-injector/internal/domain"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanner_IsRuleFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"valid yaml extension", "test.rule.yaml", true},
		{"valid json extension", "test.rule.json", true},
		{"uppercase yaml", "TEST.RULE.YAML", true},
		{"uppercase json", "TEST.RULE.JSON", true},
		{"mixed case", "Test.Rule.Yaml", true},
		{"invalid extension", "test.yaml", false},
		{"invalid json", "test.json", false},
		{"no extension", "test", false},
		{"partial match", "test.rule", false},
		{"nested path yaml", "dir/subdir/test.rule.yaml", true},
		{"nested path json", "dir/subdir/test.rule.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRuleFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScanner_ScanDirectory(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()

	// Create test files
	files := []string{
		"rule1.rule.yaml",
		"rule2.rule.json",
		"subdir/rule3.rule.yaml",
		"subdir/nested/rule4.rule.json",
		"ignored.yaml",
		"ignored.json",
		"readme.md",
	}

	for _, f := range files {
		path := filepath.Join(tempDir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("test"), 0644))
	}

	scanner := NewScanner(ScanConfig{LocalDir: tempDir})
	ctx := context.Background()

	foundFiles, err := scanner.ScanSingleDirectory(ctx, tempDir)
	require.NoError(t, err)

	// Should find exactly 4 rule files
	assert.Len(t, foundFiles, 4)

	// Verify all found files have valid extensions
	for _, f := range foundFiles {
		assert.True(t, isRuleFile(f), "File should be a rule file: %s", f)
	}
}

func TestParser_ParseYAMLFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a multi-rule YAML file
	yamlContent := `rules:
  - id: "test-rule-1"
    type: "exact"
    pattern: "https://example.com"
    css: ".banner { display: none; }"
    author: "test-author"
  - id: "test-rule-2"
    type: "wildcard"
    pattern: "https://example.com/*"
    js: "console.log('test');"
`
	filePath := filepath.Join(tempDir, "test.rule.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte(yamlContent), 0644))

	parser := NewParser()
	scannedFile := ScannedFile{
		Path:       filePath,
		SourceType: domain.SourceLocal,
	}

	rules, loadErr := parser.ParseFile(scannedFile)
	require.Nil(t, loadErr)
	require.Len(t, rules, 2)

	assert.Equal(t, "test-rule-1", rules[0].ID)
	assert.Equal(t, "exact", rules[0].Type)
	assert.Equal(t, "test-author", rules[0].Author)
	assert.Equal(t, domain.SourceLocal, rules[0].Source.Type)

	assert.Equal(t, "test-rule-2", rules[1].ID)
	assert.Equal(t, "wildcard", rules[1].Type)
}

func TestParser_ParseJSONFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a multi-rule JSON file
	jsonContent := `{
  "rules": [
    {
      "id": "json-rule-1",
      "type": "regex",
      "pattern": "https://.*\\.example\\.com",
      "css": "body { background: red; }"
    }
  ]
}`
	filePath := filepath.Join(tempDir, "test.rule.json")
	require.NoError(t, os.WriteFile(filePath, []byte(jsonContent), 0644))

	parser := NewParser()
	scannedFile := ScannedFile{
		Path:       filePath,
		SourceType: domain.SourceCommunity,
		PackName:   "test-pack",
	}

	rules, loadErr := parser.ParseFile(scannedFile)
	require.Nil(t, loadErr)
	require.Len(t, rules, 1)

	assert.Equal(t, "json-rule-1", rules[0].ID)
	assert.Equal(t, "regex", rules[0].Type)
	assert.Equal(t, domain.SourceCommunity, rules[0].Source.Type)
	assert.Equal(t, "test-pack", rules[0].Source.PackName)
}

func TestParser_ParseSingleRule(t *testing.T) {
	tempDir := t.TempDir()

	// Create a single-rule YAML file (no "rules" wrapper)
	yamlContent := `id: "single-rule"
type: "exact"
pattern: "https://single.example.com"
css: ".ad { display: none; }"
`
	filePath := filepath.Join(tempDir, "single.rule.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte(yamlContent), 0644))

	parser := NewParser()
	scannedFile := ScannedFile{
		Path:       filePath,
		SourceType: domain.SourceLocal,
	}

	rules, loadErr := parser.ParseFile(scannedFile)
	require.Nil(t, loadErr)
	require.Len(t, rules, 1)

	assert.Equal(t, "single-rule", rules[0].ID)
}

func TestParser_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create an invalid YAML file
	invalidContent := `this is not valid yaml: [[[`
	filePath := filepath.Join(tempDir, "invalid.rule.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte(invalidContent), 0644))

	parser := NewParser()
	scannedFile := ScannedFile{
		Path:       filePath,
		SourceType: domain.SourceLocal,
	}

	rules, loadErr := parser.ParseFile(scannedFile)
	assert.Nil(t, rules)
	require.NotNil(t, loadErr)
	assert.Contains(t, loadErr.Error, "failed to parse YAML")
}

func TestWriter_WriteAndReadRule(t *testing.T) {
	tempDir := t.TempDir()

	// Create a rule
	priority := 100
	rule := &domain.Rule{
		ID:          "write-test-rule",
		Type:        "exact",
		Pattern:     "https://write.example.com",
		CSS:         ".test { display: none; }",
		JS:          "console.log('test');",
		Priority:    &priority,
		Author:      "test-author",
		Description: "Test description",
		Tags:        []string{"test", "example"},
	}

	// Write the rule
	writer := NewWriter(tempDir)
	err := writer.WriteRule(rule)
	require.NoError(t, err)

	// Verify file exists
	expectedPath := filepath.Join(tempDir, "write-test-rule.rule.yaml")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err)

	// Read it back using parser
	parser := NewParser()
	scannedFile := ScannedFile{
		Path:       expectedPath,
		SourceType: domain.SourceLocal,
	}

	rules, loadErr := parser.ParseFile(scannedFile)
	require.Nil(t, loadErr)
	require.Len(t, rules, 1)

	// Verify round-trip consistency
	assert.Equal(t, rule.ID, rules[0].ID)
	assert.Equal(t, rule.Type, rules[0].Type)
	assert.Equal(t, rule.Pattern, rules[0].Pattern)
	assert.Equal(t, rule.CSS, rules[0].CSS)
	assert.Equal(t, rule.JS, rules[0].JS)
	assert.Equal(t, rule.Author, rules[0].Author)
	assert.Equal(t, rule.Description, rules[0].Description)
	assert.Equal(t, rule.Tags, rules[0].Tags)
}

func TestWriter_WriteMultipleRules(t *testing.T) {
	tempDir := t.TempDir()

	rules := []domain.Rule{
		{ID: "multi-1", Type: "exact", Pattern: "https://a.com"},
		{ID: "multi-2", Type: "wildcard", Pattern: "https://b.com/*"},
		{ID: "multi-3", Type: "regex", Pattern: "https://.*\\.c\\.com"},
	}

	writer := NewWriter(tempDir)
	err := writer.WriteRules(rules, "multi.rule.yaml")
	require.NoError(t, err)

	// Read back
	parser := NewParser()
	scannedFile := ScannedFile{
		Path:       filepath.Join(tempDir, "multi.rule.yaml"),
		SourceType: domain.SourceLocal,
	}

	loadedRules, loadErr := parser.ParseFile(scannedFile)
	require.Nil(t, loadErr)
	require.Len(t, loadedRules, 3)

	assert.Equal(t, "multi-1", loadedRules[0].ID)
	assert.Equal(t, "multi-2", loadedRules[1].ID)
	assert.Equal(t, "multi-3", loadedRules[2].ID)
}

func TestFileRuleLoader_LoadAll(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory structure
	localDir := filepath.Join(tempDir, "local")
	communityDir := filepath.Join(tempDir, "community")
	overrideDir := filepath.Join(tempDir, "overrides")

	require.NoError(t, os.MkdirAll(localDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(communityDir, "test-pack", "rules"), 0755))
	require.NoError(t, os.MkdirAll(overrideDir, 0755))

	// Create local rule
	localRule := `id: "local-rule"
type: "exact"
pattern: "https://local.example.com"
`
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "local.rule.yaml"), []byte(localRule), 0644))

	// Create community rule
	communityRule := `rules:
  - id: "community-rule"
    type: "wildcard"
    pattern: "https://community.example.com/*"
`
	require.NoError(t, os.WriteFile(filepath.Join(communityDir, "test-pack", "rules", "rules.rule.yaml"), []byte(communityRule), 0644))

	// Create override rule
	overrideRule := `id: "override-rule"
type: "regex"
pattern: "https://override\\.example\\.com"
`
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "override.rule.yaml"), []byte(overrideRule), 0644))

	// Load all rules
	loader := NewFileRuleLoader(ScanConfig{
		LocalDir:     localDir,
		CommunityDir: communityDir,
		OverrideDir:  overrideDir,
	})

	ctx := context.Background()
	rules, loadErrors, err := loader.LoadAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, loadErrors)
	assert.Len(t, rules, 3)

	// Verify source types
	localRules := loader.GetRulesBySource(domain.SourceLocal)
	assert.Len(t, localRules, 1)
	assert.Equal(t, "local-rule", localRules[0].ID)

	communityRules := loader.GetRulesBySource(domain.SourceCommunity)
	assert.Len(t, communityRules, 1)
	assert.Equal(t, "community-rule", communityRules[0].ID)
	assert.Equal(t, "test-pack", communityRules[0].Source.PackName)

	overrideRules := loader.GetRulesBySource(domain.SourceOverride)
	assert.Len(t, overrideRules, 1)
	assert.Equal(t, "override-rule", overrideRules[0].ID)
}

func TestFileRuleLoader_InvalidFilesSkipped(t *testing.T) {
	tempDir := t.TempDir()

	// Create valid and invalid files
	validRule := `id: "valid-rule"
type: "exact"
pattern: "https://valid.example.com"
`
	invalidRule := `this is not valid yaml: [[[`

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "valid.rule.yaml"), []byte(validRule), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "invalid.rule.yaml"), []byte(invalidRule), 0644))

	loader := NewFileRuleLoader(ScanConfig{LocalDir: tempDir})
	ctx := context.Background()

	rules, loadErrors, err := loader.LoadAll(ctx)
	require.NoError(t, err)

	// Valid rule should be loaded
	assert.Len(t, rules, 1)
	assert.Equal(t, "valid-rule", rules[0].ID)

	// Invalid file should be reported as error
	assert.Len(t, loadErrors, 1)
	assert.Contains(t, loadErrors[0].FilePath, "invalid.rule.yaml")
}
