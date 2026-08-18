package agentauth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"alzette/internal/api"
	"alzette/internal/federation"
	"alzette/internal/platform"
)

type Handler struct {
	service       *Service
	controlOrigin string
	gatewayOrigin string
	redirectURL   string
}

func NewHandler(service *Service, controlOrigin, gatewayOrigin string, redirectURL ...string) *Handler {
	if service == nil {
		return nil
	}
	handler := &Handler{service: service, controlOrigin: strings.TrimRight(controlOrigin, "/"), gatewayOrigin: strings.TrimRight(gatewayOrigin, "/")}
	if len(redirectURL) != 0 {
		handler.redirectURL = strings.TrimSpace(redirectURL[0])
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.URL.Path {
	case "/.well-known/alzette-agent-configuration":
		if r.Method != http.MethodGet {
			api.MethodNotAllowed(w, "GET", "")
			return
		}
		metadata := h.service.Metadata(h.controlOrigin, h.gatewayOrigin)
		if h.redirectURL != "" {
			metadata["oauth_redirect_uri"] = h.redirectURL
		}
		api.WriteJSON(w, http.StatusOK, metadata)
		return
	case "/api/agent/contexts":
		if r.Method != http.MethodGet {
			api.MethodNotAllowed(w, "GET", "")
			return
		}
	case "/api/agent/credentials", "/api/agent/credentials/revoke":
		if r.Method != http.MethodPost {
			api.MethodNotAllowed(w, "POST", "")
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	identity, err := h.authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="alzette-agent"`)
		api.WriteError(w, http.StatusUnauthorized, "invalid_identity_token", "authentication_error", "authentication failed", "")
		return
	}
	if r.URL.Path == "/api/agent/contexts" {
		contexts, err := h.service.Contexts(r.Context(), identity)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]interface{}{"schema": "alzette.agent-contexts.v1", "contexts": contexts})
		return
	}
	var input MintInput
	if err := decodeExactJSON(w, r, &input); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", "request body is invalid", "")
		return
	}
	if r.URL.Path == "/api/agent/credentials/revoke" {
		if len(input.ModelAliases) != 0 || h.service.Revoke(r.Context(), identity, input.MembershipID, input.ClientInstanceID) != nil {
			api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", "request is invalid", "")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(r.Header.Values("Idempotency-Key")) != 1 {
		api.WriteError(w, http.StatusBadRequest, "invalid_idempotency_key", "invalid_request_error", "one idempotency key is required", "")
		return
	}
	result, err := h.service.Mint(r.Context(), identity, input, r.Header.Get("Idempotency-Key"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"schema":     "alzette.agent-credential.v1",
		"credential": map[string]interface{}{"access_token": result.AccessToken, "token_type": "Bearer", "expires_at": result.ExpiresAt, "scope": []string{platform.ScopeInferenceWrite}},
		"context":    result.Context, "gateway_base_url": h.gatewayOrigin + "/v1", "model_aliases": result.ModelAliases,
	})
}

func (h *Handler) authenticate(r *http.Request) (identity federation.Identity, err error) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") || strings.Contains(values[0], ",") {
		return identity, platform.ErrUnauthenticated
	}
	raw := strings.TrimPrefix(values[0], "Bearer ")
	if raw == "" || strings.TrimSpace(raw) != raw {
		return identity, platform.ErrUnauthenticated
	}
	return h.service.Authenticate(r.Context(), raw)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrIdempotencyConflict):
		api.WriteError(w, http.StatusConflict, "idempotency_conflict", "invalid_request_error", "idempotency key was already used for another request", "")
	case errors.Is(err, ErrResponseUnrecoverable):
		api.WriteError(w, http.StatusConflict, "credential_response_unrecoverable", "invalid_request_error", "the prior one-time credential response cannot be recovered; retry with a new idempotency key", "")
	case errors.Is(err, platform.ErrInvalid):
		api.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_request_error", "request is invalid", "")
	case errors.Is(err, platform.ErrForbidden), errors.Is(err, platform.ErrNotFound), errors.Is(err, platform.ErrUnauthenticated):
		api.WriteError(w, http.StatusForbidden, "context_unavailable", "permission_error", "requested context or model access is unavailable", "")
	default:
		api.WriteError(w, http.StatusServiceUnavailable, "agent_access_unavailable", "api_error", "agent access is temporarily unavailable", "")
	}
}

func decodeExactJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON value")
	}
	return nil
}
