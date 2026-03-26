package middleware_test

import (
	"net/http/httptest"
	"testing"

	"github.com/MTDpo/Rate_limiter/internal/middleware"
)

func TestExtractIPKey(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	got := middleware.ExtractIPKey(req)
	if got != "ip:10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractUserID(t *testing.T) {
	ex := middleware.ExtractUserID("X-User-ID")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", " 42 ")
	if g := ex(req); g != "user:42" {
		t.Fatalf("got %q", g)
	}
	req2 := httptest.NewRequest("GET", "/", nil)
	if g := ex(req2); g != "user:anonymous" {
		t.Fatalf("empty header: got %q", g)
	}
}

func TestExtractAPIKey_deterministicHash(t *testing.T) {
	ex := middleware.ExtractAPIKey("X-API-Key")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "secret-value")
	a := ex(req)
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-API-Key", "secret-value")
	b := ex(req2)
	if a != b || len(a) < 10 || a[:3] != "ak:" {
		t.Fatalf("got %q and %q", a, b)
	}
	req3 := httptest.NewRequest("GET", "/", nil)
	if g := ex(req3); g != "ak:anonymous" {
		t.Fatalf("no key: got %q", g)
	}
}

func TestKeyExtractorsForModes_chain(t *testing.T) {
	exs, err := middleware.KeyExtractorsForModes([]string{"ip", "user_id"}, "X-User-ID", "X-API-Key")
	if err != nil {
		t.Fatal(err)
	}
	if len(exs) != 2 {
		t.Fatalf("len = %d", len(exs))
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.2:1"
	req.Header.Set("X-User-ID", "u1")
	k0 := exs[0](req)
	k1 := exs[1](req)
	if k0 != "ip:192.168.1.2" || k1 != "user:u1" {
		t.Fatalf("%q %q", k0, k1)
	}
}
