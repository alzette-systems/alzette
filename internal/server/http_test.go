package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicAssetPathFailsClosed(t *testing.T) {
	for _, value := range []string{"/", "/index.html", "/dashboard.css", "/assets/logo.svg"} {
		if !PublicAssetPath(value) {
			t.Errorf("expected %q to be public", value)
		}
	}
	for _, value := range []string{"/.env", "/go.mod", "/README.md", "/cmd/alzette/main.go", "/internal/store/postgres/store.go", "/migrations/0001.up.sql", "/.git/config"} {
		if PublicAssetPath(value) {
			t.Errorf("expected %q to be private", value)
		}
	}
}

func TestPublicSiteHandlerServesOnlyExplicitAssets(t *testing.T) {
	directory := t.TempDir()
	for name, contents := range map[string]string{
		"index.html":       "public landing marker",
		"docs.html":        "public docs marker",
		"site.css":         "public-css",
		"alzette-mark.svg": "<svg></svg>",
		"portal.js":        "must remain private",
		"go.mod":           "must remain private",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	handler := PublicSiteHandler(directory)
	for path, marker := range map[string]string{
		"/":           "public landing marker",
		"/index.html": "public landing marker",
		"/docs":       "public docs marker",
		"/docs.html":  "public docs marker",
		"/site.css":   "public-css",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), marker) {
			t.Fatalf("%s status=%d body=%q", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") == "" {
			t.Fatalf("%s has no cache policy", path)
		}
	}
	for _, path := range []string{"/.env", "/go.mod", "/portal.js", "/login", "/assets/unknown.svg", "/docs/"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status=%d", path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST / status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}
}
