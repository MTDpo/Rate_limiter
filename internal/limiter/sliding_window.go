package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/sliding_window.lua
var luaSlidingWindowScript string

// SlidingWindowConfig holds sliding-window log parameters.
type SlidingWindowConfig struct {
	MaxPerWindow int
	Window       time.Duration
	KeyTTL       int // seconds, should be > window
}

// DefaultSlidingWindowConfig uses capacity as max per minute (align with token bucket defaults).
func DefaultSlidingWindowConfig() SlidingWindowConfig {
	return SlidingWindowConfig{
		MaxPerWindow: 100,
		Window:       time.Minute,
		KeyTTL:       120,
	}
}

// SlidingWindowLimiter implements a sliding window via Redis sorted set + Lua.
type SlidingWindowLimiter struct {
	client *redis.Client
	config SlidingWindowConfig
	script *redis.Script
	seq    atomic.Int64
}

// NewSlidingWindow creates a sliding-window limiter.
func NewSlidingWindow(client *redis.Client, config SlidingWindowConfig) *SlidingWindowLimiter {
	if config.MaxPerWindow <= 0 {
		config.MaxPerWindow = 100
	}
	if config.Window < time.Second {
		config.Window = time.Minute
	}
	if config.KeyTTL <= 0 {
		config.KeyTTL = int(config.Window.Seconds()) + 60
	}
	return &SlidingWindowLimiter{
		client: client,
		config: config,
		script: redis.NewScript(luaSlidingWindowScript),
	}
}

func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	fullKey := fmt.Sprintf("rate_limit:sw:%s", key)
	now := float64(time.Now().UnixMicro()) / 1e6
	windowSec := s.config.Window.Seconds()
	member := fmt.Sprintf("%d-%d", time.Now().UnixNano(), s.seq.Add(1))

	result, err := s.script.Run(ctx, s.client, []string{fullKey},
		windowSec,
		s.config.MaxPerWindow,
		now,
		member,
		s.config.KeyTTL,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
