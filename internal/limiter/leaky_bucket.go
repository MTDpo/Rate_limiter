package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/leaky_bucket.lua
var luaLeakyBucketScript string

// LeakyBucketConfig holds leaky-bucket parameters (backlog capacity + drain rate).
type LeakyBucketConfig struct {
	Capacity int     // max backlog (queued "water")
	LeakRate float64 // units drained per second
	KeyTTL   int
}

// DefaultLeakyBucketConfig matches token-bucket-style defaults: 100/min effective smooth rate.
func DefaultLeakyBucketConfig() LeakyBucketConfig {
	return LeakyBucketConfig{
		Capacity: 100,
		LeakRate: 100.0 / 60.0,
		KeyTTL:   120,
	}
}

// LeakyBucketLimiter implements leaky bucket in Redis (fluid level + leak).
type LeakyBucketLimiter struct {
	client *redis.Client
	config LeakyBucketConfig
	script *redis.Script
}

// NewLeakyBucket creates a leaky-bucket limiter.
func NewLeakyBucket(client *redis.Client, config LeakyBucketConfig) *LeakyBucketLimiter {
	if config.Capacity <= 0 {
		config.Capacity = 100
	}
	if config.LeakRate <= 0 {
		config.LeakRate = 100.0 / 60.0
	}
	if config.KeyTTL <= 0 {
		config.KeyTTL = 120
	}
	return &LeakyBucketLimiter{
		client: client,
		config: config,
		script: redis.NewScript(luaLeakyBucketScript),
	}
}

func (lb *LeakyBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	fullKey := fmt.Sprintf("rate_limit:lb:%s", key)
	now := float64(time.Now().UnixMicro()) / 1e6

	result, err := lb.script.Run(ctx, lb.client, []string{fullKey},
		lb.config.Capacity,
		lb.config.LeakRate,
		now,
		1,
		lb.config.KeyTTL,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
