package agentclient

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type Proxy struct {
	listener net.Listener
	server   *http.Server
	baseURL  string
	key      string
}

func StartProxy(session *Session) (*Proxy, error) {
	if session == nil {
		return nil, errors.New("Alzette session is required")
	}
	target, err := url.Parse(session.GatewayBaseURL())
	if err != nil || target.Host == "" || !strings.HasSuffix(target.Path, "/v1") {
		return nil, errors.New("Alzette gateway URL is invalid")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	proxyKey := opaque("alp", 32)
	reverse := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			accept := request.Header.Get("Accept")
			request.URL.Scheme = target.Scheme
			request.URL.Host = target.Host
			request.URL.Path = strings.TrimRight(target.Path, "/") + strings.TrimPrefix(request.URL.Path, "/v1")
			request.Host = target.Host
			request.Header = make(http.Header)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", accept)
		},
		Transport:     &sessionTransport{session: session, base: session.config.HTTPClient.Transport},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Alzette request could not be completed", http.StatusBadGateway)
		},
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if request.Host != listener.Addr().String() || request.URL.RawQuery != "" || request.ContentLength > 32<<20 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		authorization := request.Header.Values("Authorization")
		expected := "Bearer " + proxyKey
		if len(authorization) != 1 || subtle.ConstantTimeCompare([]byte(authorization[0]), []byte(expected)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="alzette-loopback"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if request.Method == http.MethodGet && request.URL.Path == "/v1/models" {
			models := make([]map[string]interface{}, 0, len(session.selectedContext.ModelAliases))
			for _, alias := range session.selectedContext.ModelAliases {
				models = append(models, map[string]interface{}{"id": alias, "object": "model", "created": 0, "owned_by": "alzette"})
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "list", "data": models})
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		request.Body = http.MaxBytesReader(w, request.Body, 32<<20)
		normalizeOpenAICompatibilityBody(request)
		reverse.ServeHTTP(w, request)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16 << 10}
	proxy := &Proxy{listener: listener, server: server, baseURL: "http://" + listener.Addr().String() + "/v1", key: proxyKey}
	go func() { _ = server.Serve(listener) }()
	return proxy, nil
}

// normalizeOpenAICompatibilityBody removes optional sampling controls emitted
// by desktop clients such as Jan that are outside Alzette's deliberately small
// Chat Completions subset. The gateway remains strict: fields not listed here
// are forwarded unchanged and rejected there rather than silently weakened.
func normalizeOpenAICompatibilityBody(request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))

	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&object); err != nil {
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return
	}

	changed := false
	for _, field := range []string{
		"frequency_penalty",
		"min_p",
		"presence_penalty",
		"repeat_last_n",
		"repeat_penalty",
		"top_k",
	} {
		if _, exists := object[field]; exists {
			delete(object, field)
			changed = true
		}
	}
	if !changed {
		return
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(normalized))
	request.ContentLength = int64(len(normalized))
}

func (p *Proxy) BaseURL() string { return p.baseURL }
func (p *Proxy) Key() string     { return p.key }

func (p *Proxy) Close(ctx context.Context) error {
	if p == nil || p.server == nil {
		return nil
	}
	return p.server.Shutdown(ctx)
}

type sessionTransport struct {
	session *Session
	base    http.RoundTripper
}

func (transport *sessionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	token, _, err := transport.session.EnsureHumanCredential(request.Context())
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(request)
}

func CopyStream(w io.Writer, response *http.Response) error {
	if response == nil {
		return fmt.Errorf("response is nil")
	}
	_, err := io.Copy(w, response.Body)
	return err
}
