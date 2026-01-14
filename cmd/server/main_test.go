package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freewebtopdf/asset-injector/internal/config"
	"github.com/freewebtopdf/asset-injector/internal/domain"
	"github.com/freewebtopdf/asset-injector/internal/storage"
)

func TestGracefulShutdown_SIGINT(t *testing.T) {
	tempDir := t.TempDir()

	store := storage.NewStore(tempDir)

	ctx := context.Background()
	err := store.Load(ctx)
	require.NoError(t, err)

	testRule := createTestRule()
	err = store.CreateRule(ctx, testRule)
	require.NoError(t, err)

	testGracefulShutdownSignal(t, store, syscall.SIGINT)
}

func TestGracefulShutdown_SIGTERM(t *testing.T) {
	tempDir := t.TempDir()

	store := storage.NewStore(tempDir)

	ctx := context.Background()
	err := store.Load(ctx)
	require.NoError(t, err)

	testRule := createTestRule()
	err = store.CreateRule(ctx, testRule)
	require.NoError(t, err)

	testGracefulShutdownSignal(t, store, syscall.SIGTERM)
}

func TestRulePersistenceDuringShutdown(t *testing.T) {
	tempDir := t.TempDir()

	store := storage.NewStore(tempDir)

	ctx := context.Background()
	err := store.Load(ctx)
	require.NoError(t, err)

	testRule1 := createTestRule()
	testRule1.ID = "test-rule-1"
	testRule1.Pattern = "https://example1.com/*"

	testRule2 := createTestRule()
	testRule2.ID = "test-rule-2"
	testRule2.Pattern = "https://example2.com/*"

	err = store.CreateRule(ctx, testRule1)
	require.NoError(t, err)

	err = store.CreateRule(ctx, testRule2)
	require.NoError(t, err)

	newStore := storage.NewStore(tempDir)
	err = newStore.Load(ctx)
	require.NoError(t, err)

	rules, err := newStore.GetAllRules(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 2)

	ruleIDs := make(map[string]bool)
	for _, rule := range rules {
		ruleIDs[rule.ID] = true
	}
	assert.True(t, ruleIDs["test-rule-1"])
	assert.True(t, ruleIDs["test-rule-2"])
}

func TestConnectionHandlingDuringShutdown(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{}
	cfg.Server.Port = 0
	cfg.Server.ReadTimeout = 5 * time.Second
	cfg.Server.WriteTimeout = 5 * time.Second
	cfg.Server.BodyLimit = 1048576
	cfg.Cache.MaxSize = 1000
	cfg.Storage.DataDir = tempDir

	store := storage.NewStore(cfg.Storage.DataDir)

	ctx := context.Background()
	err := store.Load(ctx)
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deadline, ok := shutdownCtx.Deadline()
	assert.True(t, ok)
	assert.True(t, time.Until(deadline) > 25*time.Second)
	assert.True(t, time.Until(deadline) <= 30*time.Second)

	testRule := createTestRule()
	err = store.CreateRule(shutdownCtx, testRule)
	require.NoError(t, err)
}

func testGracefulShutdownSignal(t *testing.T, store *storage.Store, sig os.Signal) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := store.GetAllRules(ctx)
	assert.NoError(t, err)

	select {
	case <-ctx.Done():
		t.Fatal("Shutdown operations took too long")
	default:
	}
}

func createTestRule() *domain.Rule {
	return &domain.Rule{
		ID:      "test-rule-id",
		Type:    "exact",
		Pattern: "https://example.com/test",
		CSS:     "body { display: none; }",
		JS:      "console.log('test');",
	}
}

func TestStartupLogging(t *testing.T) {
	var logBuffer bytes.Buffer

	originalLogger := log.Logger
	defer func() {
		log.Logger = originalLogger
	}()

	log.Logger = zerolog.New(&logBuffer).With().Timestamp().Logger()

	cfg := &config.Config{}
	cfg.Server.Port = 8080
	cfg.Server.ReadTimeout = 5 * time.Second
	cfg.Server.WriteTimeout = 5 * time.Second
	cfg.Server.BodyLimit = 1048576
	cfg.Cache.MaxSize = 10000
	cfg.Cache.TTL = time.Hour
	cfg.Storage.DataDir = "./data"
	cfg.Security.CORSOrigins = []string{"https://example.com", "https://test.com"}
	cfg.Security.EnableHTTPS = false
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"

	logStartupConfig(cfg)

	logOutput := logBuffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(logOutput)), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "Configuration loaded successfully", logEntry["message"])

	assert.Equal(t, float64(8080), logEntry["server_port"])
	assert.Equal(t, float64(5000), logEntry["server_read_timeout"])
	assert.Equal(t, float64(5000), logEntry["server_write_timeout"])
	assert.Equal(t, float64(1048576), logEntry["server_body_limit"])
	assert.Equal(t, float64(10000), logEntry["cache_max_size"])
	assert.Equal(t, float64(3600000), logEntry["cache_ttl"])
	assert.Equal(t, "./data", logEntry["storage_data_dir"])
	assert.Equal(t, false, logEntry["security_enable_https"])
	assert.Equal(t, "info", logEntry["logging_level"])
	assert.Equal(t, "json", logEntry["logging_format"])

	corsOrigins, ok := logEntry["security_cors_origins"].([]interface{})
	require.True(t, ok)
	assert.Len(t, corsOrigins, 2)
	assert.Contains(t, corsOrigins, "https://example.com")
	assert.Contains(t, corsOrigins, "https://test.com")

	assert.NotNil(t, logEntry["time"])
}

func TestStartupLoggingWithEmptyCORSOrigins(t *testing.T) {
	var logBuffer bytes.Buffer

	originalLogger := log.Logger
	defer func() {
		log.Logger = originalLogger
	}()

	log.Logger = zerolog.New(&logBuffer).With().Timestamp().Logger()

	cfg := &config.Config{}
	cfg.Server.Port = 3000
	cfg.Server.ReadTimeout = 10 * time.Second
	cfg.Server.WriteTimeout = 10 * time.Second
	cfg.Server.BodyLimit = 2097152
	cfg.Cache.MaxSize = 5000
	cfg.Cache.TTL = 30 * time.Minute
	cfg.Storage.DataDir = "/tmp/data"
	cfg.Security.CORSOrigins = []string{}
	cfg.Security.EnableHTTPS = true
	cfg.Logging.Level = "debug"
	cfg.Logging.Format = "text"

	logStartupConfig(cfg)

	logOutput := logBuffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(logOutput)), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, float64(3000), logEntry["server_port"])
	assert.Equal(t, float64(10000), logEntry["server_read_timeout"])
	assert.Equal(t, float64(10000), logEntry["server_write_timeout"])
	assert.Equal(t, float64(2097152), logEntry["server_body_limit"])
	assert.Equal(t, float64(5000), logEntry["cache_max_size"])
	assert.Equal(t, float64(1800000), logEntry["cache_ttl"])
	assert.Equal(t, "/tmp/data", logEntry["storage_data_dir"])
	assert.Equal(t, true, logEntry["security_enable_https"])
	assert.Equal(t, "debug", logEntry["logging_level"])
	assert.Equal(t, "text", logEntry["logging_format"])

	corsOrigins, ok := logEntry["security_cors_origins"].([]interface{})
	require.True(t, ok)
	assert.Len(t, corsOrigins, 0)
}
