package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"rate_limiter/internal/config"
	"rate_limiter/internal/health"
	"rate_limiter/internal/limiter"
	"rate_limiter/internal/logger"
	"rate_limiter/internal/metrics"
	"rate_limiter/internal/middleware"
	"syscall"
	"time"

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
		log.Error("redis connectiion failed", "addr", cfg.RedisAddr, "error", err)
		os.Exit(1)
	}
	cancel()

	hc := health.NewChecker(rdb)
	hc.SetReady(true, "")

	tbCfg := limiter.TokenBucketConfig{
		Capacity:   cfg.RateLimitCapacity,
		RefillRate: cfg.RateLimitRefill,
		KeyTTL:     cfg.RateLimitKeyTTL,
	}
	tb := limiter.NewTokenBucket(rdb, tbCfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/live", hc.LivenessHandler())
	mux.HandleFunc("/ready", hc.ReadinessHandler())
	mux.HandleFunc("/health", hc.ReadinessHandler())

	rateLimitOpts := &middleware.RateLimitOpts{FailOpen: cfg.FailOpen}
	handler := middleware.RateLimit(tb, rateLimitOpts)(mux)
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
		log.Info("http server listening", "addr", cfg.HTTPAddr, "limit_per_min", cfg.RateLimitCapacity)
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
