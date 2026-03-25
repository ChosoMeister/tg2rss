package rest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ChosoMeister/tg2rss/internal/app"
	"github.com/ChosoMeister/tg2rss/internal/cache"
	"github.com/ChosoMeister/tg2rss/internal/entity"
)

// Scraper defines the interface for scraping Telegram channel data.
type Scraper interface {
	Scrape(ctx context.Context, username string) (*entity.Channel, error)
}

// Generator defines the interface for generating RSS/Atom feeds from channel data.
type Generator interface {
	Generate(channel *entity.Channel, params *entity.FeedParams) ([]byte, error)
}

// IPFilter defines the interface for IP-based access control.
type IPFilter interface {
	IsAllowed(r *http.Request) bool
}

// telegramHandler handles routes for Telegram feeds
type telegramHandler struct {
	cache     cache.Cache
	scraper   Scraper
	generator Generator
	logger    *slog.Logger
}

// NewTelegramHandler registers all Telegram-related handlers
func NewTelegramHandler(
	mux *http.ServeMux,
	c cache.Cache, s Scraper, g Generator,
	basePath string,
) {
	handler := &telegramHandler{
		cache:     c,
		scraper:   s,
		generator: g,
		logger:    app.Logger(),
	}

	mux.HandleFunc("GET "+basePath+"/telegram/channel/{username}", handler.getChannelFeed)
}

// getChannelFeed handles requests for Telegram channel feeds
func (h *telegramHandler) getChannelFeed(w http.ResponseWriter, r *http.Request) {
	params, err := entity.NewFeedParamFromRequest(r)
	if err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	// Try cache → stale cache → scrape → stale fallback
	if params.CacheTTL > 0 {
		if h.tryCache(w, r, params) {
			return
		}

		if h.tryStaleCache(w, r, params) {
			return
		}
	}

	// No cache available — scrape the channel
	content, err := h.scrapeAndGenerate(r.Context(), params)
	if err != nil {
		if params.CacheTTL > 0 && h.tryStaleFallback(w, r, params, err) {
			return
		}

		h.handleError(w, err, http.StatusInternalServerError)

		return
	}

	h.cacheContent(params, content)

	w.Header().Set("X-CACHE-STATUS", "MISS")
	h.respondWithContent(w, r, content, params)
}

// tryCache attempts to serve from fresh cache. Returns true if served.
func (h *telegramHandler) tryCache(w http.ResponseWriter, r *http.Request, params *entity.FeedParams) bool {
	cacheKey := h.buildCacheKey(params)
	cachedContent, cacheErr := h.cache.Get(r.Context(), cacheKey)

	if cacheErr != nil {
		if cacheErr != cache.ErrCacheMiss {
			h.logger.Error("Cache error", "error", cacheErr)
		}

		RecordCacheMiss()

		return false
	}

	RecordCacheHit()
	w.Header().Set("X-CACHE-STATUS", "HIT")
	h.respondWithContent(w, r, cachedContent, params)

	return true
}

// tryStaleCache serves expired cache content and triggers background refresh. Returns true if served.
func (h *telegramHandler) tryStaleCache(w http.ResponseWriter, r *http.Request, params *entity.FeedParams) bool {
	cacheKey := h.buildCacheKey(params)
	staleContent, staleErr := h.cache.GetStale(r.Context(), cacheKey)

	if staleErr != nil {
		return false
	}

	h.logger.Info("Serving stale cache", "channel", params.Username)
	RecordStaleResponse()

	w.Header().Set("X-CACHE-STATUS", "STALE")
	w.Header().Set("X-Data-Stale", "true")

	go h.refreshCache(params)

	h.respondWithContent(w, r, staleContent, params)

	return true
}

// tryStaleFallback serves stale content when scraping fails. Returns true if served.
func (h *telegramHandler) tryStaleFallback(w http.ResponseWriter, r *http.Request, params *entity.FeedParams, scrapeErr error) bool {
	cacheKey := h.buildCacheKey(params)
	staleContent, staleErr := h.cache.GetStale(r.Context(), cacheKey)

	if staleErr != nil {
		return false
	}

	h.logger.Warn("Scraper failed, serving stale cache",
		"channel", params.Username, "error", scrapeErr)
	RecordStaleResponse()

	w.Header().Set("X-CACHE-STATUS", "STALE")
	w.Header().Set("X-Data-Stale", "true")

	h.respondWithContent(w, r, staleContent, params)

	return true
}

// respondWithContent sends content with ETag check
func (h *telegramHandler) respondWithContent(w http.ResponseWriter, r *http.Request, content []byte, params *entity.FeedParams) {
	if h.handleETag(w, r, content) {
		return
	}

	h.serveContent(w, content, params.Format, params.CacheTTL)
}

// cacheContent stores content in cache if caching is enabled
func (h *telegramHandler) cacheContent(params *entity.FeedParams, content []byte) {
	if params.CacheTTL <= 0 {
		return
	}

	cacheKey := h.buildCacheKey(params)
	cacheTTL := time.Duration(params.CacheTTL) * time.Minute

	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.cache.Set(cacheCtx, cacheKey, content, cacheTTL); err != nil {
		h.logger.Error("Failed to cache content", "error", err)
	}
}

// scrapeAndGenerate performs the scraping and feed generation with metrics
func (h *telegramHandler) scrapeAndGenerate(ctx context.Context, params *entity.FeedParams) ([]byte, error) {
	RecordScrapeStart()
	scrapeStart := time.Now()

	channel, err := h.scraper.Scrape(ctx, params.Username)

	RecordScrapeDuration(time.Since(scrapeStart))
	RecordScrapeEnd()

	if err != nil {
		return nil, err
	}

	content, err := h.generator.Generate(channel, params)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// refreshCache performs a background refresh of the cache
func (h *telegramHandler) refreshCache(params *entity.FeedParams) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	content, err := h.scrapeAndGenerate(ctx, params)
	if err != nil {
		h.logger.Error("Background refresh failed",
			"channel", params.Username, "error", err)
		return
	}

	cacheKey := h.buildCacheKey(params)
	cacheTTL := time.Duration(params.CacheTTL) * time.Minute

	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cacheCancel()

	if err := h.cache.Set(cacheCtx, cacheKey, content, cacheTTL); err != nil {
		h.logger.Error("Background cache update failed", "error", err)
		return
	}

	h.logger.Info("Background cache refresh completed", "channel", params.Username)
}

// handleETag checks if the client already has the current content via ETag.
// Returns true if a 304 Not Modified was sent.
func (h *telegramHandler) handleETag(w http.ResponseWriter, r *http.Request, content []byte) bool {
	// nolint: gosec
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(content))
	w.Header().Set("ETag", etag)

	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	return false
}

// buildCacheKey generates a cache key based on request parameters
func (h *telegramHandler) buildCacheKey(params *entity.FeedParams) string {
	excludeWords := ""

	if len(params.ExcludeWords) > 0 {
		excludeWords = strings.Join(params.ExcludeWords, "|")
	}

	caseSensitive := "0"

	if params.ExcludeCaseSensitive {
		caseSensitive = "1"
	}

	return fmt.Sprintf("telegram:channel:%s:%s:%s:%s",
		params.Username,
		params.Format,
		excludeWords,
		caseSensitive)
}

// serveContent sends the content to the client with appropriate headers
func (h *telegramHandler) serveContent(w http.ResponseWriter, content []byte, format string, cacheTTL int) {
	var contentType string
	switch format {
	case entity.FormatRSS:
		contentType = "application/rss+xml"
	case entity.FormatAtom:
		contentType = "application/atom+xml"
	default:
		contentType = "application/xml"
	}

	w.Header().Set("Content-Type", contentType+"; charset=utf-8")

	if cacheTTL > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheTTL*60))
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(content); err != nil {
		h.logger.Error("Failed to write response", "error", err, "content_length", len(content))
	}
}

// handleError responds with an error message
func (h *telegramHandler) handleError(w http.ResponseWriter, err error, statusCode int) {
	h.logger.Error("Request error", "error", err, "status", statusCode)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{"error": err.Error()}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		handleBadErrorResponse(err, response)
	}
}

func handleBadErrorResponse(err error, resp any) {
	app.Logger().Error(
		"failed to encode an error response",
		"error", err,
		"response", resp,
	)
}
