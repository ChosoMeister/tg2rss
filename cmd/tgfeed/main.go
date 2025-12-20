package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nDmitry/tgfeed/internal/api/rest"
	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/cache"
	"github.com/nDmitry/tgfeed/internal/feed"
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

		defer redisClient.Close()
		c = redisClient
	}

	scraper := feed.NewDefaultScraper()
	generator := feed.NewGenerator()

	// Initialize and run the HTTP server
	server := rest.NewServer(c, scraper, generator, ipFilter, port)

	if err := server.Run(ctx); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}
