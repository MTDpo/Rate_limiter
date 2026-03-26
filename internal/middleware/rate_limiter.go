package middleware

import (
	"net"
	"net/http"
	"strings"

	"rate_limiter/internal/api"
	"rate_limiter/internal/limiter"
	"rate_limiter/internal/metrics"
)

var SkippedPaths = map[string]bool{
	"/live":    true,
	"/ready":   true,
	"/health":  true,
	"/metrics": true,
}

type RateLimitOpts struct {
	KeyExtractor func(*http.Request) string
	FailOpen     bool
}

func RateLimit(l limiter.Limiter, opts *RateLimitOpts) func(http.Handler) http.Handler {
	keyExtractor := ExtractIP
	failOpen := true
	if opts != nil {
		if opts.KeyExtractor != nil {
			keyExtractor = opts.KeyExtractor
		}
		failOpen = opts.FailOpen
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if SkippedPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			key := keyExtractor(r)
			if key == "" {
				key = "unknown"
			}
			allowed, err := l.Allow(r.Context(), key)
			if err != nil {
				metrics.RateLimitDecisions.WithLabelValues("error").Inc()
				if failOpen {
					metrics.RateLimitDecisions.WithLabelValues("allowed").Inc()
					next.ServeHTTP(w, r)
					return
				}
				api.WriteJSONError(w, http.StatusInternalServerError, "rate limiter error", GetRequestID(r.Context()))
				return
			}
			if !allowed {
				metrics.RateLimitDecisions.WithLabelValues("rejected").Inc()
				w.Header().Set("Retry-After", "60")
				api.WriteJSONError(w, http.StatusTooManyRequests, "Too Many Requests", GetRequestID(r.Context()))
				return
			}
			metrics.RateLimitDecisions.WithLabelValues("allowed").Inc()
			next.ServeHTTP(w, r)
		})
	}
}

func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
