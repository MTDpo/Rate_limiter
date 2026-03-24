package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"rate_limiter/internal/api"
	"testing"
)

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	api.WriteJSONError(rr, http.StatusBadRequest, "bad", "req-123")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: %q", ct)
	}
	var body api.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "bad" || body.Code != 400 || body.RequestID != "req-123" {
		t.Fatalf("%+v", body)
	}
}
