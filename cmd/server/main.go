package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MTDpo/Rate_limiter/internal/config"
	"github.com/MTDpo/Rate_limiter/internal/health"
	"github.com/MTDpo/Rate_limiter/internal/limiter"
	"github.com/MTDpo/Rate_limiter/internal/logger"
	"github.com/MTDpo/Rate_limiter/internal/metrics"
	"github.com/MTDpo/Rate_limiter/internal/middleware"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	log := logger.New(os.Getenv("LOG_LEVEL"))
	slog.SetDefault(log)

	rdb := redis.NewClient(&redis.Options{
		Addr:            cfg.RedisAddr,
		DialTimeout:     cfg.RedisTimeout,
		ReadTimeout:     cfg.RedisTimeout,
		WriteTimeout:    cfg.RedisTimeout,
		MaxRetries:      cfg.RedisRetries,
		MinRetryBackoff: cfg.RedisMinBackoff,
		MaxRetryBackoff: cfg.RedisMaxBackoff,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		log.Error("redis connection failed", "addr", cfg.RedisAddr, "error", err)
		os.Exit(1)
	}
	cancel()

	hc := health.NewChecker(rdb)
	hc.SetReady(true, "")

	algo := cfg.RateLimitAlgorithm
	if algo == "" {
		algo = "token_bucket"
	}
	var lim limiter.Limiter
	switch algo {
	case "sliding_window":
		lim = limiter.NewSlidingWindow(rdb, limiter.SlidingWindowConfig{
			MaxPerWindow: cfg.RateLimitCapacity,
			Window:       cfg.RateLimitWindow,
			KeyTTL:       cfg.RateLimitKeyTTL,
		})
	case "leaky_bucket":
		lim = limiter.NewLeakyBucket(rdb, limiter.LeakyBucketConfig{
			Capacity: cfg.RateLimitCapacity,
			LeakRate: cfg.RateLimitRefill,
			KeyTTL:   cfg.RateLimitKeyTTL,
		})
	default:
		lim = limiter.NewTokenBucket(rdb, limiter.TokenBucketConfig{
			Capacity:   cfg.RateLimitCapacity,
			RefillRate: cfg.RateLimitRefill,
			KeyTTL:     cfg.RateLimitKeyTTL,
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/live", hc.LivenessHandler())
	mux.HandleFunc("/ready", hc.ReadinessHandler())
	mux.HandleFunc("/health", hc.ReadinessHandler())

	extractors, err := middleware.KeyExtractorsForModes(cfg.RateLimitKeys, cfg.RateLimitUserIDHeader, cfg.RateLimitAPIKeyHeader)
	if err != nil {
		slog.Error("rate limit key modes invalid", "error", err)
		os.Exit(1)
	}
	rateLimitOpts := &middleware.RateLimitOpts{KeyExtractors: extractors, FailOpen: cfg.FailOpen}
	handler := middleware.RateLimit(lim, rateLimitOpts)(mux)
	handler = metrics.Handler(handler)
	handler = middleware.Recovery(log, handler)
	handler = middleware.RequestID(handler) // outermost: adds ID for all downstream

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	metricsSrv := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metrics.PrometheusHandler(),
	}

	go func() {
		log.Info("metrics server listening", "addr", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server failed", "error", err)
		}
	}()

	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "algorithm", algo, "capacity", cfg.RateLimitCapacity, "rate_limit_keys", cfg.RateLimitKeys)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	ctx, cancel = context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("http shutdown error", "error", err)
	}
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Error("metrics shutdown error", "error", err)
	}
	log.Info("server stopped")
}
