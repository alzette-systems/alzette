package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"alzette/internal/credentials"
	"alzette/internal/platform"
)

const PortalRealm = "Alzette client portal"

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type ErrorEnvelope struct {
	Error     ErrorDetail `json:"error"`
	RequestID string      `json:"request_id"`
}

func Authenticate(r *http.Request, store platform.Store) (platform.Principal, error) {
	if len(r.Header.Values("Authorization")) != 1 {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	header := r.Header.Get("Authorization")
	if header == "" || strings.Contains(header, ",") || !strings.HasPrefix(header, "Bearer ") {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if strings.TrimSpace(token) != token || credentials.ValidateFormat(token) != nil {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	principal, err := store.Authenticate(r.Context(), credentials.Digest(token))
	if err != nil {
		if errors.Is(err, platform.ErrUnauthenticated) || errors.Is(err, platform.ErrNotFound) {
			return platform.Principal{}, platform.ErrUnauthenticated
		}
		return platform.Principal{}, err
	}
	return principal, nil
}

// AuthenticateBasic treats the Basic password as an Alzette API key. The
// username is intentionally ignored: it is display-only for this single-client
// portal seam and never contributes to tenant scope.
func AuthenticateBasic(r *http.Request, store platform.Store) (platform.Principal, error) {
	if len(r.Header.Values("Authorization")) != 1 {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	_, token, ok := r.BasicAuth()
	if !ok || strings.TrimSpace(token) != token || credentials.ValidateFormat(token) != nil {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	principal, err := store.Authenticate(r.Context(), credentials.Digest(token))
	if err != nil {
		if errors.Is(err, platform.ErrUnauthenticated) || errors.Is(err, platform.ErrNotFound) {
			return platform.Principal{}, platform.ErrUnauthenticated
		}
		return platform.Principal{}, err
	}
	return principal, nil
}

func WriteError(w http.ResponseWriter, status int, code, errorType, message, requestID string) {
	if requestID == "" {
		requestID = w.Header().Get("X-Alzette-Request-ID")
	}
	WriteJSON(w, status, ErrorEnvelope{Error: ErrorDetail{Message: message, Type: errorType, Code: code}, RequestID: requestID})
}

func WriteJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func MethodNotAllowed(w http.ResponseWriter, allow, requestID string) {
	w.Header().Set("Allow", allow)
	WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "invalid_request_error", "method not allowed", requestID)
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authenticated responses are tenant-scoped. Cache-Control is set by the
		// individual API/portal handlers; Vary is an additional guard for any
		// intermediary that still keys a response representation.
		appendVary(w.Header(), "Authorization")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func appendVary(header http.Header, value string) {
	for _, line := range header.Values("Vary") {
		for _, existing := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}
