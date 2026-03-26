package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// ExtractIPKey — лимит по IP; префикс ip: изолирует ключи от user_id / api_key в Redis.
func ExtractIPKey(r *http.Request) string {
	ip := ExtractIP(r)
	if ip == "" {
		return "ip:unknown"
	}
	return "ip:" + ip
}

// ExtractUserID возвращает замыкание: лимит по заголовку (например X-User-ID).
func ExtractUserID(headerName string) func(*http.Request) string {
	h := strings.TrimSpace(headerName)
	if h == "" {
		h = "X-User-ID"
	}
	return func(r *http.Request) string {
		v := strings.TrimSpace(r.Header.Get(h))
		if v == "" {
			return "user:anonymous"
		}
		return "user:" + v
	}
}

// ExtractAPIKey — лимит по значению API key из заголовка; в Redis ключ по SHA-256 (сырой ключ не светим).
func ExtractAPIKey(headerName string) func(*http.Request) string {
	h := strings.TrimSpace(headerName)
	if h == "" {
		h = "X-API-Key"
	}
	return func(r *http.Request) string {
		v := strings.TrimSpace(r.Header.Get(h))
		if v == "" {
			return "ak:anonymous"
		}
		sum := sha256.Sum256([]byte(v))
		return "ak:" + hex.EncodeToString(sum[:])
	}
}

// KeyExtractorsForModes строит цепочку экстракторов в порядке modes (например ip,user_id).
func KeyExtractorsForModes(modes []string, userIDHeader, apiKeyHeader string) ([]func(*http.Request) string, error) {
	if len(modes) == 0 {
		modes = []string{"ip"}
	}
	out := make([]func(*http.Request) string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case "ip":
			out = append(out, ExtractIPKey)
		case "user_id":
			out = append(out, ExtractUserID(userIDHeader))
		case "api_key":
			out = append(out, ExtractAPIKey(apiKeyHeader))
		default:
			return nil, fmt.Errorf("unknown rate limit key mode %q (use ip, user_id, api_key)", m)
		}
	}
	return out, nil
}
