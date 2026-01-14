package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DefaultValues(t *testing.T) {
	clearEnvVars()
	defer clearEnvVars()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 5*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 5*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 1048576, cfg.Server.BodyLimit)
	assert.Equal(t, 10000, cfg.Cache.MaxSize)
	assert.Equal(t, time.Hour, cfg.Cache.TTL)
	assert.Equal(t, "./data", cfg.Storage.DataDir)
	assert.Empty(t, cfg.Security.CORSOrigins)
	assert.False(t, cfg.Security.EnableHTTPS)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)

	assert.Equal(t, "./rules", cfg.Community.RulesDir)
	assert.Equal(t, "./rules/local", cfg.Community.LocalDir)
	assert.Equal(t, "./rules/community", cfg.Community.CommunityDir)
	assert.Equal(t, "./rules/overrides", cfg.Community.OverrideDir)
	assert.Equal(t, "https://api.github.com/repos/asset-injector/community-rules", cfg.Community.RepoURL)
	assert.Equal(t, 30*time.Second, cfg.Community.RepoTimeout)
	assert.False(t, cfg.Community.AutoUpdate)
	assert.False(t, cfg.Community.WatchFiles)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	clearEnvVars()
	defer clearEnvVars()

	os.Setenv("PORT", "9090")
	os.Setenv("READ_TIMEOUT", "10s")
	os.Setenv("CACHE_MAX_SIZE", "5000")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("CORS_ORIGINS", "https://example.com,https://test.com")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 5000, cfg.Cache.MaxSize)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, []string{"https://example.com", "https://test.com"}, cfg.Security.CORSOrigins)
}

func TestLoad_CommunityEnvironmentVariables(t *testing.T) {
	clearEnvVars()
	defer clearEnvVars()

	tempDir := t.TempDir()
	os.Setenv("RULES_DIR", tempDir+"/rules")
	os.Setenv("LOCAL_RULES_DIR", tempDir+"/rules/local")
	os.Setenv("COMMUNITY_RULES_DIR", tempDir+"/rules/community")
	os.Setenv("OVERRIDE_RULES_DIR", tempDir+"/rules/overrides")
	os.Setenv("COMMUNITY_REPO_URL", "https://api.github.com/repos/custom/rules")
	os.Setenv("COMMUNITY_REPO_TIMEOUT", "60s")
	os.Setenv("AUTO_UPDATE_PACKS", "true")
	os.Setenv("WATCH_RULE_FILES", "true")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, tempDir+"/rules", cfg.Community.RulesDir)
	assert.Equal(t, tempDir+"/rules/local", cfg.Community.LocalDir)
	assert.Equal(t, tempDir+"/rules/community", cfg.Community.CommunityDir)
	assert.Equal(t, tempDir+"/rules/overrides", cfg.Community.OverrideDir)
	assert.Equal(t, "https://api.github.com/repos/custom/rules", cfg.Community.RepoURL)
	assert.Equal(t, 60*time.Second, cfg.Community.RepoTimeout)
	assert.True(t, cfg.Community.AutoUpdate)
	assert.True(t, cfg.Community.WatchFiles)
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 0

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Port must be at least 1")
}

func TestValidate_InvalidCacheSize(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Cache.MaxSize = 50

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MaxSize must be at least 100")
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Cache.MaxSize = 1000
	cfg.Logging.Level = "invalid"

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Level must be one of: debug info warn error")
}

func TestValidate_InvalidCORSOrigins(t *testing.T) {
	cfg := createValidConfig(t.TempDir())
	cfg.Security.CORSOrigins = []string{"invalid-origin"}

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CORSOrigins contains invalid origin format")
}

func TestValidate_ValidCORSOrigins(t *testing.T) {
	cfg := createValidConfig(t.TempDir())
	cfg.Security.CORSOrigins = []string{"*", "https://example.com", "http://localhost:3000"}

	err := Validate(cfg)
	assert.NoError(t, err)
}

func TestValidate_InvalidPortRange(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createValidConfig(t.TempDir())
			cfg.Server.Port = tt.port
			err := Validate(cfg)
			assert.Error(t, err)
		})
	}
}

func TestValidate_ValidPortRange(t *testing.T) {
	tests := []int{1, 80, 443, 8080, 65535}

	for _, port := range tests {
		t.Run(string(rune(port)), func(t *testing.T) {
			cfg := createValidConfig(t.TempDir())
			cfg.Server.Port = port
			err := Validate(cfg)
			assert.NoError(t, err)
		})
	}
}

func TestValidate_CacheSizeRange(t *testing.T) {
	t.Run("below minimum", func(t *testing.T) {
		cfg := createValidConfig(t.TempDir())
		cfg.Cache.MaxSize = 99
		err := Validate(cfg)
		assert.Error(t, err)
	})

	t.Run("at minimum", func(t *testing.T) {
		cfg := createValidConfig(t.TempDir())
		cfg.Cache.MaxSize = 100
		err := Validate(cfg)
		assert.NoError(t, err)
	})
}

func TestLoad_CORSOriginsParsing(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{"single wildcard", "*", []string{"*"}},
		{"single origin", "https://example.com", []string{"https://example.com"}},
		{"multiple origins", "https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()
			defer clearEnvVars()

			os.Setenv("CORS_ORIGINS", tt.envValue)

			cfg, err := Load()
			require.NoError(t, err)

			assert.Equal(t, tt.expected, cfg.Security.CORSOrigins)
		})
	}
}

func TestEnsureDirectories(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createValidConfig(tempDir)

	err := cfg.EnsureDirectories()
	require.NoError(t, err)

	// Verify directories were created
	for _, dir := range []string{cfg.Storage.DataDir, cfg.Community.RulesDir, cfg.Community.LocalDir} {
		_, err := os.Stat(dir)
		assert.NoError(t, err, "directory should exist: %s", dir)
	}
}

func clearEnvVars() {
	envVars := []string{
		"PORT", "READ_TIMEOUT", "WRITE_TIMEOUT", "BODY_LIMIT",
		"CACHE_MAX_SIZE", "CACHE_TTL",
		"DATA_DIR",
		"CORS_ORIGINS", "ENABLE_HTTPS",
		"LOG_LEVEL", "LOG_FORMAT",
		"RULES_DIR", "LOCAL_RULES_DIR", "COMMUNITY_RULES_DIR", "OVERRIDE_RULES_DIR",
		"COMMUNITY_REPO_URL", "COMMUNITY_REPO_TIMEOUT",
		"AUTO_UPDATE_PACKS", "WATCH_RULE_FILES",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

func createValidConfig(tempDir string) *Config {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Server.BodyLimit = 1048576
	cfg.Server.ReadTimeout = time.Second
	cfg.Server.WriteTimeout = time.Second
	cfg.Cache.MaxSize = 1000
	cfg.Cache.TTL = time.Hour
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	cfg.Storage.DataDir = tempDir + "/data"
	cfg.Security.CORSOrigins = []string{"*"}
	cfg.Security.EnableHTTPS = false
	cfg.Community.RulesDir = tempDir + "/rules"
	cfg.Community.LocalDir = tempDir + "/rules/local"
	cfg.Community.CommunityDir = tempDir + "/rules/community"
	cfg.Community.OverrideDir = tempDir + "/rules/overrides"
	cfg.Community.RepoURL = "https://api.github.com/repos/asset-injector/community-rules"
	cfg.Community.RepoTimeout = 30 * time.Second
	cfg.Community.AutoUpdate = false
	cfg.Community.WatchFiles = false
	return cfg
}
