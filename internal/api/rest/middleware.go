package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nDmitry/tgfeed/internal/app"
)

// Logger wraps an http.Handler with request/response logging
func Logger(next http.Handler, trustProxy bool) http.Handler {
	logger := app.Logger()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response wrapper to capture the status code
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default to 200 OK
		}

		logger.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"remote_addr", mustExtractClientIP(r, trustProxy),
			"user_agent", r.UserAgent(),
		)

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)

		logger.Info("HTTP response",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lrw.statusCode,
			"duration_ms", duration.Milliseconds(),
			"bytes", lrw.bytesWritten,
		)
	})
}

// loggingResponseWriter is a wrapper for http.ResponseWriter that captures status code and response size
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

// WriteHeader captures the status code
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytesWritten += int64(n)
	return n, err
}

// Unwrap returns the original ResponseWriter
func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}

// IPFilterMiddleware wraps an http.Handler with IP-based access control.
// If the filter is nil, the middleware passes all requests through unchanged.
// When a filter is provided, each request is validated using filter.IsAllowed.
// Denied requests receive a 403 Forbidden response with a JSON error message.
// The middleware logs warnings for denied requests including the remote address and path.
func IPFilterMiddleware(filter IPFilter, trustProxy bool) func(http.Handler) http.Handler {
	logger := app.Logger()

	return func(next http.Handler) http.Handler {
		if filter == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if filter.IsAllowed(r) {
				next.ServeHTTP(w, r)
				return
			}

			logger.Warn("IP not allowed",
				"remote_addr", mustExtractClientIP(r, trustProxy),
				"path", r.URL.Path,
			)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)

			response := map[string]string{"error": "access denied"}

			if err := json.NewEncoder(w).Encode(response); err != nil {
				logger.Error("Failed to encode error response", "error", err)
			}
		})
	}
}
