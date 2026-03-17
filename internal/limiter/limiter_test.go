package limiter_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"

	"rate_limiter/internal/limiter"
)

func setupRedis(t *testing.T) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestTokenBucket_AllowWithinLimit(t *testing.T) {
	rdb := setupRedis(t)
	tb := limiter.NewTokenBucket(rdb, limiter.TokenBucketConfig{
		Capacity:   10,
		RefillRate: 1,
		KeyTTL:     60,
	})
	ctx := context.Background()
	key := "test_ip_1"

	for i := 0; i < 10; i++ {
		allowed, err := tb.Allow(ctx, key)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !allowed {
			t.Errorf("request %d: expected allowed, got rejected", i+1)
		}
	}
}

func TestTokenBucket_RejectWhenExceeded(t *testing.T) {
	rdb := setupRedis(t)
	tb := limiter.NewTokenBucket(rdb, limiter.TokenBucketConfig{
		Capacity:   5,
		RefillRate: 0.1,
		KeyTTL:     60,
	})
	ctx := context.Background()
	key := "test_ip_exceed"

	for i := 0; i < 5; i++ {
		allowed, _ := tb.Allow(ctx, key)
		if !allowed {
			t.Errorf("request %d: expected allowed", i+1)
		}
	}
	allowed, err := tb.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if allowed {
		t.Error("expected 6th request to be rejected")
	}
}
