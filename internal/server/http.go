package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"alzette/internal/api"
)

func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			api.MethodNotAllowed(w, "GET, HEAD", "")
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func ReadinessHandler(database *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			api.MethodNotAllowed(w, "GET, HEAD", "")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if database == nil || database.PingContext(ctx) != nil {
			api.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

func StaticHandler(directory string) http.Handler {
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			api.MethodNotAllowed(w, "GET, HEAD", "")
			return
		}
		if !PublicAssetPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// PublicSiteHandler serves only the committed public landing-page assets. The
// directory is deliberately separate from the authenticated portal directory,
// and this exact allow-list prevents source files or portal JavaScript from
// becoming reachable if an image is assembled incorrectly.
func PublicSiteHandler(directory string) http.Handler {
	assets := map[string]string{
		"/":                 "index.html",
		"/index.html":       "index.html",
		"/docs":             "docs.html",
		"/docs.html":        "docs.html",
		"/site.css":         "site.css",
		"/alzette-mark.svg": "alzette-mark.svg",
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			api.MethodNotAllowed(w, "GET, HEAD", "")
			return
		}
		name, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		file, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(name, ".html") {
			w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
		}
		http.ServeContent(w, r, name, info.ModTime(), file)
	})
}

func PublicAssetPath(requestPath string) bool {
	clean := strings.TrimPrefix(path.Clean(requestPath), "/")
	if clean == "." || clean == "" {
		return true
	}
	for _, segment := range strings.Split(clean, "/") {
		if strings.HasPrefix(segment, ".") || segment == "cmd" || segment == "internal" || segment == "migrations" {
			return false
		}
	}
	switch path.Ext(clean) {
	case ".html", ".css", ".js", ".json", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".ico", ".woff", ".woff2", ".ttf":
		return true
	default:
		return false
	}
}

func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]string{"code": "not_found", "message": "resource not found", "type": "invalid_request_error"}})
	})
}
