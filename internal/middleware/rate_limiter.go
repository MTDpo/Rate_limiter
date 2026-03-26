package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/MTDpo/Rate_limiter/internal/api"
	"github.com/MTDpo/Rate_limiter/internal/limiter"
	"github.com/MTDpo/Rate_limiter/internal/metrics"
)

var SkippedPaths = map[string]bool{
	"/live":    true,
	"/ready":   true,
	"/health":  true,
	"/metrics": true,
}

type RateLimitOpts struct {
	// KeyExtractors — цепочка измерений (IP, user_id, api_key); каждому соответствует отдельный ключ в Redis.
	// Пусто = только ExtractIPKey.
	KeyExtractors []func(*http.Request) string
	// KeyExtractor оставлен для обратной совместимости: если задан и KeyExtractors пуст — используется один этот экстрактор.
	KeyExtractor func(*http.Request) string
	FailOpen     bool
}

func RateLimit(l limiter.Limiter, opts *RateLimitOpts) func(http.Handler) http.Handler {
	extractors := resolveKeyExtractors(opts)
	failOpen := true
	if opts != nil {
		failOpen = opts.FailOpen
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if SkippedPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			for _, key := range rateLimitKeys(r, extractors) {
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
			}
			metrics.RateLimitDecisions.WithLabelValues("allowed").Inc()
			next.ServeHTTP(w, r)
		})
	}
}

func resolveKeyExtractors(opts *RateLimitOpts) []func(*http.Request) string {
	if opts == nil {
		return []func(*http.Request) string{ExtractIPKey}
	}
	if len(opts.KeyExtractors) > 0 {
		return opts.KeyExtractors
	}
	if opts.KeyExtractor != nil {
		return []func(*http.Request) string{opts.KeyExtractor}
	}
	return []func(*http.Request) string{ExtractIPKey}
}

func rateLimitKeys(r *http.Request, extractors []func(*http.Request) string) []string {
	keys := make([]string, 0, len(extractors))
	for _, ex := range extractors {
		k := strings.TrimSpace(ex(r))
		if k == "" {
			k = "unknown"
		}
		keys = append(keys, k)
	}
	return keys
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
