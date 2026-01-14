package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/freewebtopdf/asset-injector/internal/api"
	"github.com/freewebtopdf/asset-injector/internal/cache"
	"github.com/freewebtopdf/asset-injector/internal/community"
	"github.com/freewebtopdf/asset-injector/internal/config"
	"github.com/freewebtopdf/asset-injector/internal/domain"
	"github.com/freewebtopdf/asset-injector/internal/health"
	"github.com/freewebtopdf/asset-injector/internal/matcher"
	"github.com/freewebtopdf/asset-injector/internal/pack"
	"github.com/freewebtopdf/asset-injector/internal/storage"

	docs "github.com/freewebtopdf/asset-injector/docs"
)

// @title Asset Injector Microservice API
// @version 1.0
// @description High-performance microservice for URL pattern matching and CSS/JS asset injection in Web-to-PDF pipelines
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @BasePath /
// @schemes http https

// @tag.name Resolution
// @tag.description URL pattern resolution operations

// @tag.name Rules
// @tag.description Rule management operations

// @tag.name System
// @tag.description System health and metrics operations

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	healthCheck := flag.Bool("health-check", false, "Perform health check and exit")
	flag.Parse()

	if *healthCheck {
		performHealthCheck()
		return
	}

	setupLogger()

	log.Info().Msg("Asset Injector Microservice starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	if err := cfg.EnsureDirectories(); err != nil {
		log.Fatal().Err(err).Msg("Failed to create required directories")
	}

	docs.SwaggerInfo.Host = os.Getenv("DOMAIN")

	logStartupConfig(cfg)

	storeConfig := storage.StoreConfig{
		DataDir:      cfg.Storage.DataDir,
		LocalDir:     cfg.Community.LocalDir,
		CommunityDir: cfg.Community.CommunityDir,
		OverrideDir:  cfg.Community.OverrideDir,
	}
	store := storage.NewStoreWithConfig(storeConfig)

	ctx := context.Background()
	if err := store.Load(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to load rules")
	}

	if cfg.Community.AutoUpdate {
		autoUpdatePacks(ctx, cfg)
	}

	lruCache := cache.NewLRUCache(cfg.Cache.MaxSize)

	patternMatcher := matcher.NewMatcher(store, lruCache)

	if err := patternMatcher.LoadRules(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to load rules into matcher")
	}

	// Start singles syncer if enabled
	var singlesSyncer *community.SinglesSyncer
	if cfg.Community.SinglesSyncEnabled {
		singlesSyncer = community.NewSinglesSyncer(community.SinglesSyncerConfig{
			RepoURL:      cfg.Community.RepoURL,
			Timeout:      cfg.Community.RepoTimeout,
			SyncInterval: cfg.Community.SinglesSyncInterval,
			TargetDir:    cfg.Community.CommunityDir + "/singles",
		})
		singlesSyncer.SetOnSync(func() {
			if err := store.Load(context.Background()); err != nil {
				log.Warn().Err(err).Msg("Failed to reload rules after singles sync")
				return
			}
			if err := patternMatcher.LoadRules(context.Background()); err != nil {
				log.Warn().Err(err).Msg("Failed to reload matcher after singles sync")
			}
			lruCache.Clear()
		})
		singlesSyncer.Start(ctx)
		log.Info().Dur("interval", cfg.Community.SinglesSyncInterval).Msg("Singles syncer started")
	}

	validator := domain.NewValidator()

	healthChecker := health.NewSystemHealthChecker(store, patternMatcher, lruCache)

	routerConfig := api.RouterConfig{
		CORSOrigins:    cfg.Security.CORSOrigins,
		BodyLimit:      cfg.Server.BodyLimit,
		RateLimitRPS:   100,
		RateLimitBurst: 200,
	}

	app := api.SetupRouter(patternMatcher, store, lruCache, validator, healthChecker, routerConfig)

	app.Server().ReadTimeout = cfg.Server.ReadTimeout
	app.Server().WriteTimeout = cfg.Server.WriteTimeout

	setupGracefulShutdown(app, singlesSyncer)

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info().
		Int("port", cfg.Server.Port).
		Str("addr", serverAddr).
		Msg("Starting HTTP server")

	if err := app.Listen(serverAddr); err != nil {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
}

func setupLogger() {
	zerolog.TimeFieldFormat = time.RFC3339

	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if os.Getenv("LOG_FORMAT") == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

func logStartupConfig(cfg *config.Config) {
	log.Info().
		Int("server_port", cfg.Server.Port).
		Dur("server_read_timeout", cfg.Server.ReadTimeout).
		Dur("server_write_timeout", cfg.Server.WriteTimeout).
		Int("server_body_limit", cfg.Server.BodyLimit).
		Int("cache_max_size", cfg.Cache.MaxSize).
		Dur("cache_ttl", cfg.Cache.TTL).
		Str("storage_data_dir", cfg.Storage.DataDir).
		Strs("security_cors_origins", cfg.Security.CORSOrigins).
		Bool("security_enable_https", cfg.Security.EnableHTTPS).
		Str("logging_level", cfg.Logging.Level).
		Str("logging_format", cfg.Logging.Format).
		Msg("Configuration loaded successfully")
}

func setupGracefulShutdown(app *fiber.App, singlesSyncer *community.SinglesSyncer) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		stop()

		log.Info().Msg("Received shutdown signal, initiating graceful shutdown")

		// Stop singles syncer if running
		if singlesSyncer != nil {
			log.Info().Msg("Stopping singles syncer...")
			singlesSyncer.Stop()
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Info().Msg("Stopping HTTP server...")
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during HTTP server shutdown")
		}

		log.Info().Msg("Graceful shutdown completed")
		os.Exit(0)
	}()
}

func performHealthCheck() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("Health check passed")
	os.Exit(0)
}

func autoUpdatePacks(ctx context.Context, cfg *config.Config) {
	client := community.NewGitHubClient(community.ClientConfig{
		RepoURL:  cfg.Community.RepoURL,
		Timeout:  cfg.Community.RepoTimeout,
		CacheDir: cfg.Storage.DataDir,
	})

	pm := pack.NewPackManager(pack.ManagerConfig{
		CommunityDir: cfg.Community.CommunityDir,
		OverrideDir:  cfg.Community.OverrideDir,
	}, client)

	updates, err := pm.CheckUpdates(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check for pack updates")
		return
	}

	for _, u := range updates {
		log.Info().Str("pack", u.Name).Str("from", u.CurrentVersion).Str("to", u.LatestVersion).Msg("Updating pack")
		if err := pm.Update(ctx, u.Name); err != nil {
			log.Warn().Err(err).Str("pack", u.Name).Msg("Failed to update pack")
		}
	}
}
