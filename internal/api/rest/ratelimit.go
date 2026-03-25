package rest

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/ChosoMeister/tg2rss/internal/app"
	"golang.org/x/time/rate"
)

// RateLimiter provides per-IP rate limiting using token bucket algorithm.
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter with the specified rate (requests/second) and burst size.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if burst <= 0 {
		burst = int(rps) * 2
		if burst < 5 {
			burst = 5
		}
	}

	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.limiters[ip]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[ip] = limiter

	return limiter
}

// RateLimitMiddleware applies per-IP rate limiting to HTTP requests.
// Returns 429 Too Many Requests if the rate limit is exceeded.
func RateLimitMiddleware(rl *RateLimiter, trustProxy bool) func(http.Handler) http.Handler {
	logger := app.Logger()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := mustExtractClientIP(r, trustProxy)

			if clientIP == "" {
				next.ServeHTTP(w, r)
				return
			}

			limiter := rl.getLimiter(clientIP)

			if !limiter.Allow() {
				logger.Warn("Rate limit exceeded",
					"remote_addr", clientIP,
					"path", r.URL.Path,
				)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)

				response := map[string]string{"error": "rate limit exceeded"}

				if err := json.NewEncoder(w).Encode(response); err != nil {
					logger.Error("Failed to encode rate limit response", "error", err)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
