package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	RateLimitAlgorithm string // token_bucket | sliding_window | leaky_bucket
	RateLimitCapacity  int
	RateLimitRefill    float64
	RateLimitWindow    time.Duration // sliding_window only
	RateLimitKeyTTL    int
	FailOpen           bool // allow traffic when Redis is down

	// Ключи лимита (цепочка): ip, user_id, api_key — порядок задаёт порядок проверок Allow.
	RateLimitKeys         []string
	RateLimitUserIDHeader string
	RateLimitAPIKeyHeader string
}

// Load reads config from environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:              getEnv("HTTP_ADDR", ":8080"),
		MetricsAddr:           getEnv("METRICS_ADDR", ":9090"),
		ShutdownTimeout:       getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		RedisTimeout:          getEnvDuration("REDIS_TIMEOUT", 3*time.Second),
		RedisRetries:          getEnvInt("REDIS_RETRIES", 3),
		RedisMinBackoff:       getEnvDuration("REDIS_MIN_BACKOFF", 100*time.Millisecond),
		RedisMaxBackoff:       getEnvDuration("REDIS_MAX_BACKOFF", 2*time.Second),
		RateLimitAlgorithm:    strings.ToLower(strings.TrimSpace(getEnv("RATE_LIMIT_ALGORITHM", "token_bucket"))),
		RateLimitCapacity:     getEnvInt("RATE_LIMIT_CAPACITY", 100),
		RateLimitRefill:       getEnvFloat("RATE_LIMIT_REFILL", 100.0/60.0),
		RateLimitWindow:       getEnvDuration("RATE_LIMIT_WINDOW", time.Minute),
		RateLimitKeyTTL:       getEnvInt("RATE_LIMIT_KEY_TTL", 120),
		FailOpen:              getEnvBool("RATE_LIMIT_FAIL_OPEN", true),
		RateLimitKeys:         parseRateLimitKeys(getEnv("RATE_LIMIT_KEYS", "ip")),
		RateLimitUserIDHeader: getEnv("RATE_LIMIT_USER_ID_HEADER", "X-User-ID"),
		RateLimitAPIKeyHeader: getEnv("RATE_LIMIT_API_KEY_HEADER", "X-API-Key"),
	}
	return cfg, cfg.Validate()
}

func parseRateLimitKeys(s string) []string {
	parts := strings.Split(s, ",")
	var keys []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		keys = append(keys, p)
	}
	if len(keys) == 0 {
		return []string{"ip"}
	}
	return keys
}

func (c *Config) Validate() error {
	switch c.RateLimitAlgorithm {
	case "", "token_bucket", "sliding_window", "leaky_bucket":
	default:
		return fmt.Errorf("unknown RATE_LIMIT_ALGORITHM %q (use token_bucket, sliding_window, leaky_bucket)", c.RateLimitAlgorithm)
	}
	if c.RateLimitCapacity <= 0 {
		return fmt.Errorf("RATE_LIMIT_CAPACITY must be positive, got %d", c.RateLimitCapacity)
	}
	algo := c.RateLimitAlgorithm
	if algo == "" {
		algo = "token_bucket"
	}
	if algo != "sliding_window" && c.RateLimitRefill <= 0 {
		return fmt.Errorf("RATE_LIMIT_REFILL must be positive for token_bucket/leaky_bucket, got %f", c.RateLimitRefill)
	}
	if c.ShutdownTimeout < time.Second {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be >= 1s, got %v", c.ShutdownTimeout)
	}
	if algo == "sliding_window" {
		if c.RateLimitWindow < time.Second {
			return fmt.Errorf("RATE_LIMIT_WINDOW must be >= 1s for sliding_window, got %v", c.RateLimitWindow)
		}
		minTTL := int(c.RateLimitWindow/time.Second) + 1
		if c.RateLimitKeyTTL < minTTL {
			return fmt.Errorf("RATE_LIMIT_KEY_TTL must be >= window+1s (%ds) for sliding_window, got %d", minTTL, c.RateLimitKeyTTL)
		}
	}
	for _, k := range c.RateLimitKeys {
		switch k {
		case "ip", "user_id", "api_key":
		default:
			return fmt.Errorf("unknown RATE_LIMIT_KEYS part %q (use ip, user_id, api_key, comma-separated)", k)
		}
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
