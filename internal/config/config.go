package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

// Config holds all configuration for the Asset Injector Microservice
type Config struct {
	Server struct {
		Port         int           `env:"PORT" envDefault:"8080" validate:"min=1,max=65535"`
		ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"5s"`
		WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"5s"`
		BodyLimit    int           `env:"BODY_LIMIT" envDefault:"1048576" validate:"min=1"` // 1MB
	}

	Cache struct {
		MaxSize int           `env:"CACHE_MAX_SIZE" envDefault:"10000" validate:"min=100"`
		TTL     time.Duration `env:"CACHE_TTL" envDefault:"1h"`
	}

	Storage struct {
		DataDir string `env:"DATA_DIR" envDefault:"./data"`
	}

	Security struct {
		CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:"," validate:"cors_origins"`
		EnableHTTPS bool     `env:"ENABLE_HTTPS" envDefault:"false"`
	}

	Logging struct {
		Level  string `env:"LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
		Format string `env:"LOG_FORMAT" envDefault:"json" validate:"oneof=json text"`
	}

	// Community configuration for community sharing features
	Community CommunityConfig
}

// CommunityConfig holds configuration for community sharing features
type CommunityConfig struct {
	// Directory paths for rule storage
	RulesDir     string `env:"RULES_DIR" envDefault:"./rules"`
	LocalDir     string `env:"LOCAL_RULES_DIR" envDefault:"./rules/local"`
	CommunityDir string `env:"COMMUNITY_RULES_DIR" envDefault:"./rules/community"`
	OverrideDir  string `env:"OVERRIDE_RULES_DIR" envDefault:"./rules/overrides"`

	// Community repository settings
	RepoURL     string        `env:"COMMUNITY_REPO_URL" envDefault:"https://api.github.com/repos/freewebtopdf/asset-injector-community-rules"`
	RepoTimeout time.Duration `env:"COMMUNITY_REPO_TIMEOUT" envDefault:"30s"`

	// Behavior settings
	AutoUpdate bool `env:"AUTO_UPDATE_PACKS" envDefault:"false"`
	WatchFiles bool `env:"WATCH_RULE_FILES" envDefault:"false"`

	// Singles sync settings
	SinglesSyncEnabled  bool          `env:"SINGLES_SYNC_ENABLED" envDefault:"false"`
	SinglesSyncInterval time.Duration `env:"SINGLES_SYNC_INTERVAL" envDefault:"5m"`
}

// Load loads configuration from environment variables and .env files
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration using struct tags
func Validate(cfg *Config) error {
	validator := validator.New()

	if err := validator.RegisterValidation("cors_origins", validateCORSOrigins); err != nil {
		return fmt.Errorf("failed to register cors_origins validation: %w", err)
	}

	if err := validator.Struct(cfg); err != nil {
		return formatValidationError(err)
	}

	if err := validateCustomRules(cfg); err != nil {
		return err
	}

	return nil
}

// validateCORSOrigins validates CORS origins format
func validateCORSOrigins(fl validator.FieldLevel) bool {
	origins := fl.Field().Interface().([]string)
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin != "*" && !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
			return false
		}
	}
	return true
}

// validateCustomRules performs additional validation beyond struct tags
func validateCustomRules(cfg *Config) error {
	if cfg.Storage.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	if cfg.Server.ReadTimeout < time.Millisecond {
		return fmt.Errorf("read timeout must be at least 1ms")
	}
	if cfg.Server.WriteTimeout < time.Millisecond {
		return fmt.Errorf("write timeout must be at least 1ms")
	}
	if cfg.Cache.TTL < time.Second {
		return fmt.Errorf("cache TTL must be at least 1 second")
	}

	if err := validateCommunityConfig(&cfg.Community); err != nil {
		return err
	}

	return nil
}

// validateCommunityConfig validates community-specific configuration
func validateCommunityConfig(cfg *CommunityConfig) error {
	if cfg.RepoTimeout < time.Second {
		return fmt.Errorf("community repository timeout must be at least 1 second")
	}

	if cfg.RepoURL == "" {
		return fmt.Errorf("community repository URL cannot be empty")
	}

	return nil
}

// EnsureDirectories creates all required directories
func (cfg *Config) EnsureDirectories() error {
	dirs := []string{
		cfg.Storage.DataDir,
		cfg.Community.RulesDir,
		cfg.Community.LocalDir,
		cfg.Community.CommunityDir,
		cfg.Community.OverrideDir,
	}

	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("cannot create directory %s: %w", dir, err)
			}
		}
	}
	return nil
}

// StoreConfig returns storage configuration derived from community paths
func (cfg *Config) StoreConfig() struct {
	DataDir      string
	LocalDir     string
	CommunityDir string
	OverrideDir  string
} {
	return struct {
		DataDir      string
		LocalDir     string
		CommunityDir string
		OverrideDir  string
	}{
		DataDir:      cfg.Storage.DataDir,
		LocalDir:     cfg.Community.LocalDir,
		CommunityDir: cfg.Community.CommunityDir,
		OverrideDir:  cfg.Community.OverrideDir,
	}
}

// formatValidationError formats validation errors into readable messages
func formatValidationError(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, e := range validationErrors {
			switch e.Tag() {
			case "required":
				messages = append(messages, fmt.Sprintf("%s is required", e.Field()))
			case "min":
				messages = append(messages, fmt.Sprintf("%s must be at least %s", e.Field(), e.Param()))
			case "max":
				messages = append(messages, fmt.Sprintf("%s must be at most %s", e.Field(), e.Param()))
			case "oneof":
				messages = append(messages, fmt.Sprintf("%s must be one of: %s", e.Field(), e.Param()))
			case "cors_origins":
				messages = append(messages, fmt.Sprintf("%s contains invalid origin format", e.Field()))
			default:
				messages = append(messages, fmt.Sprintf("%s failed validation: %s", e.Field(), e.Tag()))
			}
		}
		return fmt.Errorf("validation errors: %s", strings.Join(messages, "; "))
	}
	return err
}
