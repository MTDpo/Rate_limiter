package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds validated application configuration.
type Config struct {
	// Server
	HTTPAddr        string
	MetricsAddr     string
	ShutdownTimeout time.Duration

	// Redis
	RedisAddr       string
	RedisTimeout    time.Duration
	RedisRetries    int
	RedisMinBackoff time.Duration
	RedisMaxBackoff time.Duration

	// Rate Limiter
	RateLimitCapacity int
	RateLimitRefill   float64
	RateLimitKeyTTL   int
	FailOpen          bool // allow traffic when Redis is down
}

// Load reads config from environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		MetricsAddr:       getEnv("METRICS_ADDR", ":9090"),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisTimeout:      getEnvDuration("REDIS_TIMEOUT", 3*time.Second),
		RedisRetries:      getEnvInt("REDIS_RETRIES", 3),
		RedisMinBackoff:   getEnvDuration("REDIS_MIN_BACKOFF", 100*time.Millisecond),
		RedisMaxBackoff:   getEnvDuration("REDIS_MAX_BACKOFF", 2*time.Second),
		RateLimitCapacity: getEnvInt("RATE_LIMIT_CAPACITY", 100),
		RateLimitRefill:   getEnvFloat("RATE_LIMIT_REFILL", 100.0/60.0),
		RateLimitKeyTTL:   getEnvInt("RATE_LIMIT_KEY_TTL", 120),
		FailOpen:          getEnvBool("RATE_LIMIT_FAIL_OPEN", true),
	}
	return cfg, cfg.Validate()
}

func (c *Config) Validate() error {
	if c.RateLimitCapacity <= 0 {
		return fmt.Errorf("RATE_LIMIT_CAPACITY must be positive, got %d", c.RateLimitCapacity)
	}
	if c.RateLimitRefill <= 0 {
		return fmt.Errorf("RATE_LIMIT_REFILL must be positive, got %f", c.RateLimitRefill)
	}
	if c.ShutdownTimeout < time.Second {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be >= 1s, got %v", c.ShutdownTimeout)
	}
	return nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}
