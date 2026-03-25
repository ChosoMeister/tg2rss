package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ChosoMeister/tg2rss/internal/app"
	"github.com/ChosoMeister/tg2rss/internal/cache"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerConfig holds all server configuration options
type ServerConfig struct {
	Port           string
	BasePath       string
	TrustProxy     bool
	MetricsEnabled bool
	RateLimit      float64 // requests per second, 0 = disabled
	RateBurst      int     // burst size for rate limiter
}

// Server represents the REST API server
type Server struct {
	mux       *http.ServeMux
	server    *http.Server
	logger    *slog.Logger
	cache     cache.Cache
	scraper   Scraper
	generator Generator
	ipFilter  IPFilter
	config    ServerConfig
}

// NewServer creates a new REST API server with the specified dependencies.
// The ipFilter parameter controls IP-based access restrictions; pass nil to disable filtering.
// The server is pre-configured with secure timeout values to mitigate common attacks.
func NewServer(c cache.Cache, s Scraper, g Generator, ipFilter IPFilter, config ServerConfig) *Server {
	mux := http.NewServeMux()
	logger := app.Logger()

	// Normalize base path
	basePath := strings.TrimRight(config.BasePath, "/")
	config.BasePath = basePath

	server := &Server{
		mux:       mux,
		logger:    logger,
		cache:     c,
		scraper:   s,
		generator: g,
		ipFilter:  ipFilter,
		config:    config,
		server: &http.Server{
			Addr:              ":" + config.Port,
			Handler:           nil,               // Will be set in Run
			ReadHeaderTimeout: 10 * time.Second,  // Mitigate Slowloris
			ReadTimeout:       30 * time.Second,  // Time to read entire request (including body)
			WriteTimeout:      300 * time.Second, // Time to process request and write response (includes semaphore wait + scraping)
			IdleTimeout:       120 * time.Second, // Keep-alive timeout
		},
	}

	server.registerHandlers()

	return server
}

// registerHandlers sets up all API routes
func (s *Server) registerHandlers() {
	prefix := s.config.BasePath

	// Health check endpoint
	s.mux.HandleFunc("GET "+prefix+"/health", s.handleHealth)

	// Telegram feed handler
	NewTelegramHandler(s.mux, s.cache, s.scraper, s.generator, prefix)

	// Metrics endpoint (if enabled)
	if s.config.MetricsEnabled {
		s.mux.Handle("GET "+prefix+"/metrics", promhttp.Handler())
		s.logger.Info("Prometheus metrics enabled", "path", prefix+"/metrics")
	}
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{"status": "ok"}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode health response", "error", err)
	}
}

// Run starts the server and blocks until the context is canceled
func (s *Server) Run(ctx context.Context) error {
	// Apply middleware chain (outermost first)
	handler := http.Handler(s.mux)
	handler = IPFilterMiddleware(s.ipFilter, s.config.TrustProxy)(handler)

	// Rate limiting
	if s.config.RateLimit > 0 {
		rl := NewRateLimiter(s.config.RateLimit, s.config.RateBurst)
		handler = RateLimitMiddleware(rl, s.config.TrustProxy)(handler)
		s.logger.Info("Rate limiting enabled", "rate", s.config.RateLimit, "burst", s.config.RateBurst)
	}

	// Gzip compression
	handler = GzipMiddleware(handler)

	// Prometheus metrics
	if s.config.MetricsEnabled {
		handler = MetricsMiddleware(handler)
	}

	// Logging
	handler = Logger(handler, s.config.TrustProxy)

	// Set the handler with middleware
	s.server.Handler = handler

	// Set BaseContext to pass the parent context
	s.server.BaseContext = func(_ net.Listener) context.Context { return ctx }

	// Register shutdown handler
	s.server.RegisterOnShutdown(func() {
		s.logger.Info("Server is shutting down...")
	})

	// Start server in a goroutine
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("Starting HTTP server", "port", s.config.Port, "base_path", s.config.BasePath)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
		close(errCh)
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Create a timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	s.logger.Info("Server exited gracefully")

	return nil
}
