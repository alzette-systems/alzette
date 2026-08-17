// Package faketarget provides a deterministic, credential-checked
// Chat Completions target for offline integration and operator smoke tests.
// It never logs or retains request or response content.
package faketarget

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync/atomic"
)

const (
	DefaultProviderModel = "deterministic/slice0-v1"
	ExpectedOutput       = "SLICE0_COMPATIBLE_OK"
	defaultMaxBodyBytes  = int64(1 << 20)
)

type Config struct {
	Secret          string
	ProviderModel   string
	TimeoutFirst    int64
	MaxRequestBytes int64
}

type Handler struct {
	secret          string
	providerModel   string
	timeoutFirst    int64
	maxRequestBytes int64
	calls           atomic.Int64
}

func New(config Config) (*Handler, error) {
	if config.Secret == "" || strings.ContainsAny(config.Secret, "\r\n") {
		return nil, errors.New("fake target secret is required and must be header-safe")
	}
	if config.ProviderModel == "" || len(config.ProviderModel) > 255 {
		return nil, errors.New("fake target provider model is required")
	}
	if config.TimeoutFirst < 0 {
		return nil, errors.New("fake target timeout count cannot be negative")
	}
	if config.MaxRequestBytes <= 0 {
		config.MaxRequestBytes = defaultMaxBodyBytes
	}
	return &Handler{
		secret:          config.Secret,
		providerModel:   config.ProviderModel,
		timeoutFirst:    config.TimeoutFirst,
		maxRequestBytes: config.MaxRequestBytes,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"status":"ready"}`)
		}
		return
	}
	if r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorized(r.Header.Values("Authorization")) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="alzette deterministic target"`)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
		return
	}
	if r.Header.Get("X-Alzette-Request-ID") == "" {
		http.Error(w, "missing request correlation", http.StatusBadRequest)
		return
	}
	body := http.MaxBytesReader(w, r.Body, h.maxRequestBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var request chatRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if request.Model != h.providerModel || len(request.Messages) == 0 || (request.Stream != nil && *request.Stream) {
		http.Error(w, "incompatible request", http.StatusBadRequest)
		return
	}
	for _, message := range request.Messages {
		if message.Content == "" {
			http.Error(w, "incompatible request", http.StatusBadRequest)
			return
		}
	}

	if h.calls.Add(1) <= h.timeoutFirst {
		// Waiting for cancellation produces a deterministic pre-output timeout
		// without emitting bytes that could make a retry unsafe.
		<-r.Context().Done()
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Generation-Id", "slice0-fake-generation")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":    "slice0-fake-response",
		"model": h.providerModel,
		"choices": []map[string]interface{}{{
			"index": 0,
			"message": map[string]string{
				"role":    "assistant",
				"content": ExpectedOutput,
			},
		}},
		"usage": map[string]interface{}{
			"prompt_tokens":     4,
			"completion_tokens": 2,
			"cached_tokens":     1,
			"reasoning_tokens":  0,
		},
	})
}

func (h *Handler) authorized(values []string) bool {
	if len(values) != 1 {
		return false
	}
	want := "Bearer " + h.secret
	if len(values[0]) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(values[0]), []byte(want)) == 1
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Stream      *bool     `json:"stream,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
