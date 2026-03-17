package middleware

import (
	"log/slog"
	"net/http"
	"rate_limiter/internal/api"
	"rate_limiter/internal/metrics"
	"runtime/debug"
)

func Recovery(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(r.Context())
				log.Error("panic recovered", "error", err, "request_id", requestID, "path", r.URL.Path, "stack", string(debug.Stack()))
				metrics.RateLimitDecisions.WithLabelValues("error").Inc()
				api.WriteJSONError(w, http.StatusInternalServerError, "internal server error", requestID)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
