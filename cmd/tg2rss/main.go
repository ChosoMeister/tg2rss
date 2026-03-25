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

	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received first shutdown signal, starting graceful shutdown...")
		cancel()

		// If we receive a second signal, exit immediately
		<-sigChan
		logger.Info("Received second shutdown signal, exiting immediately...")
		os.Exit(1)
	}()

	port := os.Getenv("HTTP_SERVER_PORT")

	if port == "" {
		port = "8080"
	}

	redisHost := os.Getenv("REDIS_HOST")

	// Configure IP filtering
	allowedIPsStr := os.Getenv("ALLOWED_IPS")
	trustProxy := os.Getenv("REVERSE_PROXY") == "true" || os.Getenv("REVERSE_PROXY") == "1"

	var ipFilter rest.IPFilter

	if allowedIPsStr != "" {
		firewall, err := rest.NewFirewall(allowedIPsStr, trustProxy)

		if err != nil {
			logger.Error("Failed to create firewall", "error", err)
			os.Exit(1)
		}

		ipFilter = firewall
		logger.Info("IP filtering enabled", "allowed_ips", allowedIPsStr, "trust_proxy", trustProxy)
	}

	var c cache.Cache

	if redisHost == "" {
		c = cache.NewMemoryClient()
	} else {
		redisClient, err := cache.NewRedisClient(ctx, fmt.Sprintf("%s:6379", redisHost))

		if err != nil {
			logger.Error("Failed to connect to Redis", "error", err)
			os.Exit(1)
		}

		c = redisClient
	}

	defer c.Close()

	// Parse configurable concurrency
	maxConcurrent := 0 // use default

	if v := os.Getenv("MAX_CONCURRENT_SCRAPES"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			logger.Error("Invalid MAX_CONCURRENT_SCRAPES", "value", v, "error", err)
			os.Exit(1)
		}
		maxConcurrent = parsed
	}

	scraper := feed.NewScraper(maxConcurrent)
	generator := feed.NewGenerator()

	// Parse configurable default cache TTL
	if v := os.Getenv("DEFAULT_CACHE_TTL"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			logger.Error("Invalid DEFAULT_CACHE_TTL", "value", v, "error", err)
			os.Exit(1)
		}
		entity.SetDefaultCacheTTL(parsed)
		logger.Info("Default cache TTL configured", "minutes", parsed)
	}

	// Parse rate limiting
	var rateLimit float64

	var rateBurst int

	if v := os.Getenv("RATE_LIMIT"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			logger.Error("Invalid RATE_LIMIT", "value", v, "error", err)
			os.Exit(1)
		}
		rateLimit = parsed
	}

	if v := os.Getenv("RATE_BURST"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			logger.Error("Invalid RATE_BURST", "value", v, "error", err)
			os.Exit(1)
		}
		rateBurst = parsed
	}

	metricsEnabled := os.Getenv("METRICS_ENABLED") == "true" || os.Getenv("METRICS_ENABLED") == "1"

	config := rest.ServerConfig{
		Port:           port,
		BasePath:       os.Getenv("BASE_PATH"),
		TrustProxy:     trustProxy,
		MetricsEnabled: metricsEnabled,
		RateLimit:      rateLimit,
		RateBurst:      rateBurst,
	}

	// Initialize and run the HTTP server
	server := rest.NewServer(c, scraper, generator, ipFilter, config)

	if err := server.Run(ctx); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}

