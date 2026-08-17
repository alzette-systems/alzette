package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersVaryByAuthorization(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))

	if response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("Vary=%q", response.Header().Get("Vary"))
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options=%q", response.Header().Get("X-Content-Type-Options"))
	}
}
