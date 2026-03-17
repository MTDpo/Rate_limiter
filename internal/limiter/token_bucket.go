package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/token_bucket.lua
var luaTokenBucketScript string

// TokenBucketConfig holds Token Bucket algorithm parameters.
type TokenBucketConfig struct {
	Capacity   int
	RefillRate float64
	KeyTTL     int
}

// DefaultTokenBucketConfig returns sensible defaults: 100 req/min = ~1.67 tokens/sec
func DefaultTokenBucketConfig() TokenBucketConfig {
	return TokenBucketConfig{
		Capacity:   100,
		RefillRate: 100.0 / 60.0,
		KeyTTL:     120,
	}
}

// TokenBucket implements Token Bucket rate limiting using Redis.
type TokenBucketLimiter struct {
	client *redis.Client
	config TokenBucketConfig
	script *redis.Script
}

// NewTokenBucket creates a new Token Bucket limiter.
func NewTokenBucket(client *redis.Client, config TokenBucketConfig) *TokenBucketLimiter {
	if config.Capacity <= 0 {
		config.Capacity = 100
	}
	if config.RefillRate <= 0 {
		config.RefillRate = 100.0 / 60.0
	}
	if config.KeyTTL <= 0 {
		config.KeyTTL = 120
	}
	return &TokenBucketLimiter{
		client: client,
		config: config,
		script: redis.NewScript(luaTokenBucketScript),
	}
}

func (tb *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	fullKey := fmt.Sprintf("rate_limit:%s", key)
	now := float64(time.Now().UnixMicro()) / 1e6

	result, err := tb.script.Run(ctx, tb.client, []string{fullKey},
		tb.config.Capacity,
		tb.config.RefillRate,
		now,
		1,
		tb.config.KeyTTL,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
