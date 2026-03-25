package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ChosoMeister/tg2rss/internal/api/rest"
	"github.com/ChosoMeister/tg2rss/internal/app"
	"github.com/ChosoMeister/tg2rss/internal/cache"
	"github.com/ChosoMeister/tg2rss/internal/entity"
	"github.com/ChosoMeister/tg2rss/internal/feed"
)

func main() {
	logger := app.Logger()
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel, logger)

	port := envOrDefault("HTTP_SERVER_PORT", "8080")
	trustProxy := envBool("REVERSE_PROXY")

	ipFilter := setupIPFilter(logger, trustProxy)
	c := setupCache(ctx, logger)
	defer c.Close()

	scraper := feed.NewScraper(envInt(logger, "MAX_CONCURRENT_SCRAPES", 0))
	generator := feed.NewGenerator()

	setupDefaultCacheTTL(logger)

	config := rest.ServerConfig{
		Port:           port,
		BasePath:       os.Getenv("BASE_PATH"),
		TrustProxy:     trustProxy,
		MetricsEnabled: envBool("METRICS_ENABLED"),
		RateLimit:      envFloat(logger, "RATE_LIMIT", 0),
		RateBurst:      envInt(logger, "RATE_BURST", 0),
	}

	server := rest.NewServer(c, scraper, generator, ipFilter, config)

	if err := server.Run(ctx); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}

func setupSignalHandler(cancel context.CancelFunc, logger *slog.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received first shutdown signal, starting graceful shutdown...")
		cancel()

		<-sigChan
		logger.Info("Received second shutdown signal, exiting immediately...")
		os.Exit(1)
	}()
}

func setupIPFilter(logger *slog.Logger, trustProxy bool) rest.IPFilter {
	allowedIPsStr := os.Getenv("ALLOWED_IPS")
	if allowedIPsStr == "" {
		return nil
	}

	firewall, err := rest.NewFirewall(allowedIPsStr, trustProxy)
	if err != nil {
		logger.Error("Failed to create firewall", "error", err)
		os.Exit(1)
	}

	logger.Info("IP filtering enabled", "allowed_ips", allowedIPsStr, "trust_proxy", trustProxy)

	return firewall
}

func setupCache(ctx context.Context, logger *slog.Logger) cache.Cache {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		return cache.NewMemoryClient()
	}

	redisClient, err := cache.NewRedisClient(ctx, fmt.Sprintf("%s:6379", redisHost))
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	return redisClient
}

func setupDefaultCacheTTL(logger *slog.Logger) {
	v := os.Getenv("DEFAULT_CACHE_TTL")
	if v == "" {
		return
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		logger.Error("Invalid DEFAULT_CACHE_TTL", "value", v, "error", err)
		os.Exit(1)
	}

	entity.SetDefaultCacheTTL(parsed)
	logger.Info("Default cache TTL configured", "minutes", parsed)
}

// Helper functions for environment variable parsing

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultVal
}

func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1"
}

func envInt(logger *slog.Logger, key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		logger.Error("Invalid "+key, "value", v, "error", err)
		os.Exit(1)
	}

	return parsed
}

func envFloat(logger *slog.Logger, key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}

	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		logger.Error("Invalid "+key, "value", v, "error", err)
		os.Exit(1)
	}

	return parsed
}
