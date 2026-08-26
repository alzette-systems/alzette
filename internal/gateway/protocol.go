package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/platform"
)

// wireProtocol identifies the client-facing API dialect. Alzette always
// executes the bounded Chat Completions representation internally; protocol
// adapters never participate in authentication, routing, or provider secret
// selection.
type wireProtocol string

const (
	protocolChat      wireProtocol = "openai_chat_completions"
	protocolResponses wireProtocol = "openai_responses"
	protocolAnthropic wireProtocol = "anthropic_messages"
)

func protocolForPath(path string) (wireProtocol, bool) {
	switch path {
	case "/v1/chat/completions":
		return protocolChat, true
	case "/v1/responses":
		return protocolResponses, true
	case "/v1/messages":
		return protocolAnthropic, true
	default:
		return "", false
	}
}

func (g *Gateway) decodeProtocolRequest(w http.ResponseWriter, r *http.Request, protocol wireProtocol) (ChatRequest, *requestError) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return ChatRequest{}, &requestError{http.StatusUnsupportedMediaType, "unsupported_content_type", "Content-Type must be application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, g.maxRequestBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return ChatRequest{}, &requestError{http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the configured limit"}
		}
		return ChatRequest{}, &requestError{http.StatusBadRequest, "invalid_json", "request body could not be read"}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return ChatRequest{}, &requestError{http.StatusBadRequest, "invalid_json", "request body must contain exactly one JSON object"}
	}

	var request ChatRequest
	switch protocol {
	case protocolChat:
		if err := decodeStrict(data, &request); err != nil {
			if strings.Contains(err.Error(), "json: unknown field") {
				return ChatRequest{}, &requestError{http.StatusBadRequest, "unsupported_request_field", "request contains a field outside the supported chat completion subset"}
			}
			return ChatRequest{}, &requestError{http.StatusBadRequest, "invalid_json", "request body is not a valid supported chat completion"}
		}
	case protocolResponses:
		request, err = decodeResponsesRequest(data)
	case protocolAnthropic:
		request, err = decodeAnthropicRequest(data)
	default:
		err = errors.New("unsupported protocol")
	}
	if err != nil {
		var conversion *protocolConversionError
		if errors.As(err, &conversion) {
			return ChatRequest{}, &requestError{conversion.status, conversion.code, conversion.message}
		}
		return ChatRequest{}, &requestError{http.StatusBadRequest, "invalid_request", "request is not a valid supported protocol request"}
	}
	if validation := request.validate(); validation != nil {
		return ChatRequest{}, validation
	}
	return request, nil
}

type protocolConversionError struct {
	status        int
	code, message string
}

func (e *protocolConversionError) Error() string { return e.message }

func unsupportedProtocolField(protocol, field string) error {
	return &protocolConversionError{
		status:  http.StatusBadRequest,
		code:    "unsupported_request_field",
		message: fmt.Sprintf("%s field %q is outside Alzette's supported protocol subset", protocol, field),
	}
}

func invalidProtocolRequest(protocol, message string) error {
	return &protocolConversionError{status: http.StatusBadRequest, code: "invalid_request", message: protocol + " request " + message}
}

func rawString(value string) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func rawNull() json.RawMessage { return json.RawMessage("null") }

func joinTextParts(parts []string) json.RawMessage {
	if len(parts) == 1 {
		return rawString(parts[0])
	}
	values := make([]textPart, 0, len(parts))
	for _, value := range parts {
		values = append(values, textPart{Type: "text", Text: value})
	}
	data, _ := json.Marshal(values)
	return data
}

func decodeOneJSON(raw json.RawMessage, destination interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func encodeProtocolResponse(protocol wireProtocol, body []byte, requestID, publicModel string, now time.Time, aliases map[string]responsesToolIdentity) ([]byte, error) {
	switch protocol {
	case protocolChat:
		return body, nil
	case protocolResponses:
		return encodeResponsesResponse(body, requestID, publicModel, now, aliases)
	case protocolAnthropic:
		return encodeAnthropicResponse(body, requestID, publicModel, now)
	default:
		return nil, errors.New("unsupported response protocol")
	}
}

func writeProtocolError(w http.ResponseWriter, protocol wireProtocol, status int, code, errorType, message, requestID string) {
	if protocol != protocolAnthropic {
		api.WriteError(w, status, code, errorType, message, requestID)
		return
	}
	if requestID == "" {
		requestID = w.Header().Get("X-Alzette-Request-ID")
	}
	type anthropicError struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	type anthropicErrorEnvelope struct {
		Type      string         `json:"type"`
		Error     anthropicError `json:"error"`
		RequestID string         `json:"request_id,omitempty"`
	}
	api.WriteJSON(w, status, anthropicErrorEnvelope{
		Type:      "error",
		Error:     anthropicError{Type: anthropicErrorType(status, code), Message: message},
		RequestID: requestID,
	})
}

func authenticateProtocol(r *http.Request, protocol wireProtocol, store platform.Store) (platform.Principal, error) {
	if protocol != protocolAnthropic || len(r.Header.Values("X-Api-Key")) == 0 {
		return api.Authenticate(r, store)
	}
	// Anthropic-compatible clients send x-api-key. Accept exactly one
	// credential representation and pass it through the same Alzette token
	// validator; the provider never sees this client credential.
	if len(r.Header.Values("X-Api-Key")) != 1 || len(r.Header.Values("Authorization")) != 0 {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	token := r.Header.Get("X-Api-Key")
	if token == "" || strings.TrimSpace(token) != token || strings.Contains(token, ",") {
		return platform.Principal{}, platform.ErrUnauthenticated
	}
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Header.Del("X-Api-Key")
	clone.Header.Set("Authorization", "Bearer "+token)
	return api.Authenticate(clone, store)
}

func anthropicErrorType(status int, code string) string {
	switch {
	case status == http.StatusUnauthorized:
		return "authentication_error"
	case status == http.StatusForbidden:
		return "permission_error"
	case status == http.StatusNotFound:
		return "not_found_error"
	case status == http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case status == http.StatusTooManyRequests:
		return "rate_limit_error"
	case status == http.StatusServiceUnavailable || code == "target_unavailable":
		return "overloaded_error"
	case status >= 500:
		return "api_error"
	default:
		return "invalid_request_error"
	}
}
