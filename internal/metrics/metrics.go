package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RateLimitDecisions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limiter_decisions_total",
			Help: "Rate limiter decisions: allowed, rejected, error",
		},
		[]string{"decision"},
	)
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limiter_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"status"},
	)
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rate_limiter_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(RateLimitDecisions, RequestsTotal, RequestDuration)
}

// Handler wraps an http.Handler and records metrics.
func Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)
		RequestsTotal.WithLabelValues(status).Inc()
		RequestDuration.WithLabelValues(status).Observe(duration)
	})
}

// PrometheusHandler returns the Prometheus metrics handler.
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.written = true
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
