package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tg2rss_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tg2rss_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "path"},
	)

	cacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tg2rss_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	cacheMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tg2rss_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	scrapesDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tg2rss_scrape_duration_seconds",
			Help:    "Duration of Telegram channel scrapes in seconds",
			Buckets: []float64{.5, 1, 2, 5, 10, 30, 60},
		},
	)

	activeScrapes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tg2rss_active_scrapes",
			Help: "Number of currently active scrapes",
		},
	)

	staleResponsesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tg2rss_stale_responses_total",
			Help: "Total number of stale (fallback) responses served",
		},
	)
)

// RecordCacheHit increments the cache hit counter
func RecordCacheHit() {
	cacheHitsTotal.Inc()
}

// RecordCacheMiss increments the cache miss counter
func RecordCacheMiss() {
	cacheMissesTotal.Inc()
}

// RecordScrapeDuration records the duration of a scrape operation
func RecordScrapeDuration(d time.Duration) {
	scrapesDurationSeconds.Observe(d.Seconds())
}

// RecordScrapeStart increments the active scrapes gauge
func RecordScrapeStart() {
	activeScrapes.Inc()
}

// RecordScrapeEnd decrements the active scrapes gauge
func RecordScrapeEnd() {
	activeScrapes.Dec()
}

// RecordStaleResponse increments the stale response counter
func RecordStaleResponse() {
	staleResponsesTotal.Inc()
}

// MetricsMiddleware records HTTP request metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		mrw := &metricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(mrw, r)

		duration := time.Since(start)
		path := r.URL.Path

		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(mrw.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration.Seconds())
	})
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
