package limiter_test

import (
	"context"
	"fmt"
	"rate_limiter/internal/limiter"
	"testing"

	"github.com/redis/go-redis/v9"
)

func initRedisForBench() *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil
	}
	return rdb
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
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
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow(ctx, fmt.Sprintf("bench_ip_%d", i%100))
	}
}

func BenchmarkTokenBucket_Allow_Parallel(b *testing.B) {
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
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tb.Allow(ctx, fmt.Sprintf("bench_ip_%d", i%1000))
			i++
		}
	})
}
