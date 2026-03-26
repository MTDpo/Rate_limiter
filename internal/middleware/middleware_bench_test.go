package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/MTDpo/Rate_limiter/internal/limiter"
	"github.com/MTDpo/Rate_limiter/internal/middleware"
)

func initRedisForBench() *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil
	}
	return rdb
}

func BenchmarkRateLimitMiddleware_Allowed(b *testing.B) {
	rdb := initRedisForBench()
	if rdb == nil {
		b.Skip("redis not available")
	}
	defer rdb.Close()

	tb := limiter.NewTokenBucket(rdb, limiter.TokenBucketConfig{
		Capacity:   10000,
		RefillRate: 1000,
		KeyTTL:     120,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	stack := middleware.RateLimit(tb, nil)(mux)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		stack.ServeHTTP(w, req)
	}
}

func BenchmarkRateLimitMiddleware_Parallel(b *testing.B) {
	rdb := initRedisForBench()
	if rdb == nil {
		b.Skip("redis not available")
	}
	defer rdb.Close()

	tb := limiter.NewTokenBucket(rdb, limiter.TokenBucketConfig{
		Capacity:   100000,
		RefillRate: 10000,
		KeyTTL:     120,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	stack := middleware.RateLimit(tb, nil)(mux)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", i%255)
			w := httptest.NewRecorder()
			stack.ServeHTTP(w, req)
			i++
		}
	})
}
