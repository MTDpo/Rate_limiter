package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MTDpo/Rate_limiter/internal/middleware"
)

type mockLimiter struct {
	allowFn func(ctx context.Context, key string) (bool, error)
}

func (m *mockLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if m.allowFn != nil {
		return m.allowFn(ctx, key)
	}
	return true, nil
}

func TestRateLimit_SkippedPaths(t *testing.T) {
	m := &mockLimiter{allowFn: func(ctx context.Context, key string) (bool, error) {
		t.Fatal("Allow must not be called for skipped paths")
		return true, nil
	}}
	h := middleware.RateLimit(m, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	for _, path := range []string{"/live", "/ready", "/health", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Errorf("%s : got %d, want %d", path, rr.Code, http.StatusTeapot)
		}
	}
}

func TestRateLimit_Allowed(t *testing.T) {
	calls := 0
	m := &mockLimiter{allowFn: func(ctx context.Context, key string) (bool, error) {
		calls++
		if key != "192.168.1.1" {
			t.Errorf("key = %q", key)
		}
		return true, nil
	}}
	h := middleware.RateLimit(m, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || calls != 1 {
		t.Fatalf("code = %d, calls = %d", rr.Code, calls)
	}
}

func TestRateLimit_Rejected(t *testing.T) {
	m := &mockLimiter{allowFn: func(ctx context.Context, key string) (bool, error) {
		return false, nil
	}}
	h := middleware.RateLimit(m, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After: %q", rr.Header().Get("Retry-After"))
	}
}

func TestRateLimit_ErrorFailOpen(t *testing.T) {
	m := &mockLimiter{allowFn: func(ctx context.Context, key string) (bool, error) {
		return false, errors.New("redis down")
	}}
	h := middleware.RateLimit(m, &middleware.RateLimitOpts{FailOpen: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("fail-open: got %d", rr.Code)
	}
}

func TestRateLimit_ErrorFailClosed(t *testing.T) {
	m := &mockLimiter{allowFn: func(ctx context.Context, key string) (bool, error) {
		return false, errors.New("redis down")
	}}
	h := middleware.RateLimit(m, &middleware.RateLimitOpts{FailOpen: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestExtractIP_Table(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		want       string
	}{
		{"xff first", "1.2.3.4:1", "203.0.113.1, 198.51.100.1", "", "203.0.113.1"},
		{"xff no comma", "1.2.3.4:1", "203.0.113.2", "", "203.0.113.2"},
		{"xff comma at start", "1.2.3.4:1", ", 203.0.113.3", "", ""},
		{"x-real-ip", "1.2.3.4:1", "", "198.51.100.2", "198.51.100.2"},
		{"remote addr", "192.168.0.5:54321", "", "", "192.168.0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			got := middleware.ExtractIP(req)
			if got != tt.want {
				t.Errorf("ExtractIP() = %q, want %q", got, tt.want)
			}
		})
	}

}
