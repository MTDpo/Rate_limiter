package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Checker struct {
	redis  *redis.Client
	mu     sync.RWMutex
	ready  bool
	reason string
}

func NewChecker(rdb *redis.Client) *Checker {
	return &Checker{redis: rdb}
}

func (c *Checker) SetReady(ready bool, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ready = ready
	c.reason = reason
}

func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := c.redis.Ping(ctx).Err(); err != nil {
			c.SetReady(false, err.Error())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable", "reason":"redis not reachable"}`))
			return
		}
		c.SetReady(true, "")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}
