package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"alzette/internal/ids"
	"alzette/internal/inference"
	"alzette/internal/platform"
	"alzette/internal/secrets"
)

const (
	defaultMaxRequestBytes  = int64(1 << 20)
	defaultMaxResponseBytes = int64(4 << 20)
)

type Config struct {
	Store                platform.Store
	HTTPClient           *http.Client
	SecretLookup         func(string) (string, bool)
	AllowedSecretRefs    []string
	Clock                func() time.Time
	NewID                func(string) (string, error)
	MaxRequestBytes      int64
	MaxResponseBytes     int64
	RetryBaseDelay       time.Duration
	MaxRetryDelay        time.Duration
	AllowInsecureTargets bool
	// legacyHTTPForTests permits deterministic transport fault injection in this
	// package. Production callers cannot select it; buffered execution uses Bifrost.
	legacyHTTPForTests bool
}

type Gateway struct {
	store                platform.Store
	client               *http.Client
	secretLookup         func(string) (string, bool)
	clock                func() time.Time
	newID                func(string) (string, error)
	maxRequestBytes      int64
	maxResponseBytes     int64
	retryBaseDelay       time.Duration
	maxRetryDelay        time.Duration
	allowInsecureTargets bool
	bifrost              *inference.Engine
	legacyHTTPForTests   bool
}

func New(config Config) (*Gateway, error) {
	if config.Store == nil {
		return nil, errors.New("gateway store is required")
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Transport:     http.DefaultTransport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("upstream redirects are disabled") },
		}
	}
	if config.SecretLookup == nil {
		allowed := make(map[string]struct{}, len(config.AllowedSecretRefs)+1)
		if len(config.AllowedSecretRefs) == 0 {
			config.AllowedSecretRefs = []string{"OPENROUTER_API_KEY"}
		}
		for _, reference := range config.AllowedSecretRefs {
			if reference != "" {
				allowed[reference] = struct{}{}
			}
		}
		config.SecretLookup = func(reference string) (string, bool) {
			if _, ok := allowed[reference]; !ok {
				return "", false
			}
			return secrets.Lookup(reference)
		}
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.NewID == nil {
		config.NewID = ids.New
	}
	if config.MaxRequestBytes <= 0 {
		config.MaxRequestBytes = defaultMaxRequestBytes
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	if config.RetryBaseDelay <= 0 {
		config.RetryBaseDelay = 50 * time.Millisecond
	}
	if config.MaxRetryDelay <= 0 {
		config.MaxRetryDelay = 5 * time.Second
	}
	return &Gateway{store: config.Store, client: config.HTTPClient, secretLookup: config.SecretLookup, clock: config.Clock, newID: config.NewID, maxRequestBytes: config.MaxRequestBytes, maxResponseBytes: config.MaxResponseBytes, retryBaseDelay: config.RetryBaseDelay, maxRetryDelay: config.MaxRetryDelay, allowInsecureTargets: config.AllowInsecureTargets, bifrost: inference.New(), legacyHTTPForTests: config.legacyHTTPForTests}, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	protocol, ok := protocolForPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeError := func(status int, code, errorType, message, requestID string) {
		writeProtocolError(w, protocol, status, code, errorType, message, requestID)
	}
	requestID, err := g.newID("req")
	if err != nil {
		writeError(http.StatusInternalServerError, "internal_error", "api_error", "request could not be initialised", "")
		return
	}
	w.Header().Set("X-Alzette-Request-ID", requestID)
	w.Header().Set("X-Request-ID", requestID)
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(http.StatusMethodNotAllowed, "method_not_allowed", "invalid_request_error", "method not allowed", requestID)
		return
	}
	if r.URL.RawQuery != "" {
		writeError(http.StatusBadRequest, "unsupported_query_parameter", "invalid_request_error", "query parameters are not supported by this endpoint", requestID)
		return
	}
	if protocol == protocolAnthropic {
		w.Header().Add("Vary", "X-Api-Key")
	}
	principal, err := authenticateProtocol(r, protocol, g.store)
	if err != nil {
		if errors.Is(err, platform.ErrUnauthenticated) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="alzette"`)
			writeError(http.StatusUnauthorized, "invalid_api_key", "authentication_error", "authentication failed", requestID)
			return
		}
		writeError(http.StatusServiceUnavailable, "authentication_unavailable", "api_error", "authentication is temporarily unavailable", requestID)
		return
	}

	request, parseErr := g.decodeProtocolRequest(w, r, protocol)
	if parseErr != nil {
		writeError(parseErr.status, parseErr.code, "invalid_request_error", parseErr.message, requestID)
		return
	}
	publicModel := request.Model
	startedAt := g.clock().UTC()
	if err := g.store.CreateInferenceRequest(r.Context(), platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: request.Model, StartedAt: startedAt}); err != nil {
		writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request ledger is temporarily unavailable", requestID)
		return
	}
	finish := func(status string, httpStatus int, class, model, providerID, finality string, usage platform.TokenUsage) bool {
		finishContext, cancel := detachedContext()
		defer cancel()
		err := g.store.CompleteInferenceRequest(finishContext, platform.RequestFinish{ID: requestID, CompletedAt: g.clock().UTC(), Status: status, HTTPStatus: httpStatus, ErrorClass: class, ExecutedModel: model, ProviderRequestID: providerID, Duration: elapsed(startedAt, g.clock().UTC()), Usage: usage, UsageFinality: finality})
		return err == nil
	}

	if !principal.HasScope(platform.ScopeInferenceWrite) {
		if !finish("blocked", http.StatusForbidden, "insufficient_scope", "", "", "unknown", platform.TokenUsage{}) {
			writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		writeError(http.StatusForbidden, "insufficient_scope", "permission_error", "API key is not permitted to run inference", requestID)
		return
	}
	route, err := g.store.ResolveRoute(r.Context(), principal, request.Model)
	if err != nil {
		status, code, message := routeError(err)
		if !finish("blocked", status, code, "", "", "unknown", platform.TokenUsage{}) {
			writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		writeError(status, code, "invalid_request_error", message, requestID)
		return
	}
	if err := g.store.SetInferenceRequestRoute(r.Context(), requestID, route.ID); err != nil {
		if !finish("failed", http.StatusInternalServerError, "ledger_error", "", "", "unknown", platform.TokenUsage{}) {
			writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		writeError(http.StatusInternalServerError, "ledger_error", "api_error", "request could not be recorded", requestID)
		return
	}
	if _, err := targetEndpoint(route.Target.BaseURL, g.allowInsecureTargets); err != nil {
		if !finish("failed", http.StatusBadGateway, "target_configuration", "", "", "unknown", platform.TokenUsage{}) {
			writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		healthContext, cancel := detachedContext()
		_ = g.store.UpdateTargetHealth(healthContext, route.Target.ID, "degraded", g.clock().UTC(), false)
		cancel()
		writeError(http.StatusBadGateway, "target_configuration", "api_error", "configured inference target is invalid", requestID)
		return
	}
	secret, ok := g.secretLookup(route.Target.SecretRef)
	if !ok || secret == "" {
		if !finish("failed", http.StatusServiceUnavailable, "target_unavailable", "", "", "unknown", platform.TokenUsage{}) {
			writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		healthContext, cancel := detachedContext()
		_ = g.store.UpdateTargetHealth(healthContext, route.Target.ID, "degraded", g.clock().UTC(), false)
		cancel()
		writeError(http.StatusServiceUnavailable, "target_unavailable", "api_error", "configured inference target is unavailable", requestID)
		return
	}
	request.Model = route.Target.ProviderModel
	upstreamBody, err := json.Marshal(request)
	if err != nil {
		if !finish("failed", http.StatusInternalServerError, "internal_error", "", "", "unknown", platform.TokenUsage{}) {
			writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
			return
		}
		writeError(http.StatusInternalServerError, "internal_error", "api_error", "request could not be prepared", requestID)
		return
	}
	if request.streaming() {
		g.serveStreaming(w, r, requestID, publicModel, protocol, route, secret, upstreamBody, finish)
		return
	}

	var terminal attemptResult
	for attemptNumber := 1; attemptNumber <= route.Target.MaxAttempts; attemptNumber++ {
		terminal = g.performAttempt(r.Context(), requestID, route, secret, upstreamBody, attemptNumber)
		if terminal.success {
			responseBody, encodeErr := encodeProtocolResponse(protocol, terminal.body, requestID, publicModel, g.clock().UTC())
			if encodeErr != nil {
				if !finish("failed", http.StatusBadGateway, "invalid_upstream_response", "", terminal.providerID, "unknown", platform.TokenUsage{}) {
					writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
					return
				}
				writeError(http.StatusBadGateway, "invalid_upstream_response", "upstream_error", "upstream inference could not be represented in the requested API", requestID)
				return
			}
			if !finish("succeeded", http.StatusOK, "", terminal.model, terminal.providerID, terminal.finality, terminal.usage) {
				writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
				return
			}
			healthContext, cancel := detachedContext()
			_ = g.store.UpdateTargetHealth(healthContext, route.Target.ID, "operational", g.clock().UTC(), true)
			cancel()
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(responseBody)
			return
		}
		if !terminal.retryable || attemptNumber == route.Target.MaxAttempts {
			break
		}
		delay := terminal.retryAfter
		if !terminal.retryAfterSet {
			delay = g.retryBaseDelay * time.Duration(1<<(attemptNumber-1))
		}
		if delay > g.maxRetryDelay {
			delay = g.maxRetryDelay
		}
		if err := wait(r.Context(), delay); err != nil {
			terminal = attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled"}
			break
		}
	}
	status := "failed"
	if terminal.class == "client_cancelled" {
		status = "cancelled"
	}
	if terminal.clientStatus == 0 {
		terminal.clientStatus = http.StatusBadGateway
	}
	if terminal.message == "" {
		terminal.message = "upstream inference failed"
	}
	if !finish(status, terminal.clientStatus, terminal.class, "", terminal.providerID, "unknown", platform.TokenUsage{}) {
		writeError(http.StatusServiceUnavailable, "ledger_unavailable", "api_error", "request result could not be recorded", requestID)
		return
	}
	if healthStatus := failureHealthStatus(terminal.class); healthStatus != "" {
		healthContext, cancel := detachedContext()
		_ = g.store.UpdateTargetHealth(healthContext, route.Target.ID, healthStatus, g.clock().UTC(), false)
		cancel()
	}
	if terminal.retryAfterHeader != "" {
		w.Header().Set("Retry-After", terminal.retryAfterHeader)
	}
	writeError(terminal.clientStatus, terminal.class, "upstream_error", terminal.message, requestID)
}

type requestError struct {
	status        int
	code, message string
}

type attemptResult struct {
	success          bool
	retryable        bool
	class            string
	clientStatus     int
	message          string
	body             []byte
	model            string
	providerID       string
	usage            platform.TokenUsage
	finality         string
	retryAfter       time.Duration
	retryAfterSet    bool
	retryAfterHeader string
}

func (g *Gateway) performAttempt(parent context.Context, requestID string, route platform.Route, secret string, body []byte, number int) attemptResult {
	if !g.legacyHTTPForTests {
		return g.performBifrostAttempt(parent, requestID, route, secret, body, number)
	}
	attemptID, err := g.newID("att")
	if err != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusInternalServerError, message: "request attempt could not be initialised"}
	}
	started := g.clock().UTC()
	if err := g.store.CreateProviderAttempt(parent, platform.AttemptStart{ID: attemptID, InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: number, StartedAt: started}); err != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt could not be recorded"}
	}
	finish := func(status, class, providerID string, providerStatus int) error {
		finishContext, cancel := detachedContext()
		defer cancel()
		return g.store.CompleteProviderAttempt(finishContext, platform.AttemptFinish{ID: attemptID, CompletedAt: g.clock().UTC(), Status: status, ProviderHTTPStatus: providerStatus, ErrorClass: class, Duration: elapsed(started, g.clock().UTC()), ProviderRequestID: providerID})
	}
	ledgerFailure := func() attemptResult {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt result could not be recorded"}
	}
	attemptContext, cancel := context.WithTimeout(parent, route.Target.Timeout)
	defer cancel()
	upstreamURL, err := targetEndpoint(route.Target.BaseURL, g.allowInsecureTargets)
	if err != nil {
		if finish("failed", "target_configuration", "", 0) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "target_configuration", clientStatus: http.StatusBadGateway, message: "configured inference target is invalid"}
	}
	request, err := http.NewRequestWithContext(attemptContext, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		if finish("failed", "target_configuration", "", 0) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "target_configuration", clientStatus: http.StatusBadGateway, message: "configured inference target is invalid"}
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Alzette-Request-ID", requestID)
	response, err := g.client.Do(request)
	if err != nil {
		if parent.Err() != nil {
			if finish("cancelled", "client_cancelled", "", 0) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled"}
		}
		if errors.Is(attemptContext.Err(), context.DeadlineExceeded) {
			if finish("failed", "upstream_timeout", "", 0) != nil {
				return ledgerFailure()
			}
			return attemptResult{retryable: true, class: "upstream_timeout", clientStatus: http.StatusGatewayTimeout, message: "upstream inference timed out"}
		}
		if finish("failed", "upstream_transport", "", 0) != nil {
			return ledgerFailure()
		}
		return attemptResult{retryable: true, class: "upstream_transport", clientStatus: http.StatusBadGateway, message: "upstream inference connection failed"}
	}
	defer response.Body.Close()
	providerID := safeProviderID(response.Header.Get("X-Generation-Id"))
	limited := io.LimitReader(response.Body, g.maxResponseBytes+1)
	responseBody, readErr := io.ReadAll(limited)
	if readErr != nil {
		if parent.Err() != nil {
			if finish("cancelled", "client_cancelled", providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled", providerID: providerID}
		}
		if errors.Is(attemptContext.Err(), context.DeadlineExceeded) {
			if finish("failed", "upstream_timeout", providerID, response.StatusCode) != nil {
				return ledgerFailure()
			}
			return attemptResult{retryable: true, class: "upstream_timeout", clientStatus: http.StatusGatewayTimeout, message: "upstream inference timed out", providerID: providerID}
		}
		if finish("failed", "invalid_upstream_response", providerID, response.StatusCode) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference returned an incomplete response", providerID: providerID}
	}
	if int64(len(responseBody)) > g.maxResponseBytes {
		if finish("failed", "upstream_response_too_large", providerID, response.StatusCode) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "upstream_response_too_large", clientStatus: http.StatusBadGateway, message: "upstream inference response exceeded the configured limit", providerID: providerID}
	}
	if response.StatusCode != http.StatusOK {
		result := classifyProviderStatus(response.StatusCode)
		result.providerID = providerID
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
			result.retryAfter, result.retryAfterSet, result.retryAfterHeader = parseRetryAfter(response.Header.Get("Retry-After"), g.clock().UTC())
		}
		if finish("failed", result.class, providerID, response.StatusCode) != nil {
			return ledgerFailure()
		}
		return result
	}
	meta, err := parseProviderResponse(responseBody)
	if err != nil {
		if finish("failed", "invalid_upstream_response", providerID, response.StatusCode) != nil {
			return ledgerFailure()
		}
		return attemptResult{class: "invalid_upstream_response", clientStatus: http.StatusBadGateway, message: "upstream inference returned an invalid response", providerID: providerID}
	}
	if providerID == "" {
		providerID = meta.providerID
	}
	if finish("succeeded", "", providerID, response.StatusCode) != nil {
		return ledgerFailure()
	}
	return attemptResult{success: true, body: responseBody, model: meta.model, providerID: providerID, usage: meta.usage, finality: meta.finality}
}

func (g *Gateway) performBifrostAttempt(parent context.Context, requestID string, route platform.Route, secret string, body []byte, number int) attemptResult {
	attemptID, err := g.newID("att")
	if err != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusInternalServerError, message: "request attempt could not be initialised"}
	}
	started := g.clock().UTC()
	if err := g.store.CreateProviderAttempt(parent, platform.AttemptStart{ID: attemptID, InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: number, StartedAt: started}); err != nil {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt could not be recorded"}
	}
	finish := func(status, class, providerID string, providerStatus int, usage platform.TokenUsage, finality string) error {
		finishContext, cancel := detachedContext()
		defer cancel()
		return g.store.CompleteProviderAttempt(finishContext, platform.AttemptFinish{
			ID: attemptID, CompletedAt: g.clock().UTC(), Status: status,
			ProviderHTTPStatus: providerStatus, ErrorClass: class,
			Duration: elapsed(started, g.clock().UTC()), ProviderRequestID: providerID,
			Usage: usage, UsageFinality: finality,
		})
	}
	ledgerFailure := func() attemptResult {
		return attemptResult{class: "ledger_error", clientStatus: http.StatusServiceUnavailable, message: "request attempt result could not be recorded"}
	}
	attemptContext, cancel := context.WithTimeout(parent, route.Target.Timeout)
	defer cancel()
	result, providerErr := g.bifrost.Chat(attemptContext, inference.Target{
		BaseURL: route.Target.BaseURL, Model: route.Target.ProviderModel,
		Timeout: route.Target.Timeout, AllowPrivateNetwork: g.allowInsecureTargets,
	}, secret, requestID, body)
	if providerErr != nil {
		usage, finality := platformUsage(providerErr.BilledUsage, "partial")
		providerID := safeProviderID(providerErr.ProviderRequestID)
		if parent.Err() != nil {
			if finish("cancelled", "client_cancelled", providerID, providerErr.StatusCode, usage, finality) != nil {
				return ledgerFailure()
			}
			return attemptResult{class: "client_cancelled", clientStatus: 499, message: "request was cancelled", providerID: providerID}
		}
		if errors.Is(attemptContext.Err(), context.DeadlineExceeded) {
			if finish("failed", "upstream_timeout", providerID, providerErr.StatusCode, usage, finality) != nil {
				return ledgerFailure()
			}
			return attemptResult{retryable: true, class: "upstream_timeout", clientStatus: http.StatusGatewayTimeout, message: "upstream inference timed out", providerID: providerID}
		}
		terminal := classifyBifrostError(providerErr)
		terminal.providerID = providerID
		if providerErr.RetryAfter != "" {
			terminal.retryAfter, terminal.retryAfterSet, terminal.retryAfterHeader = parseRetryAfter(providerErr.RetryAfter, g.clock().UTC())
		}
		if finish("failed", terminal.class, providerID, providerErr.StatusCode, usage, finality) != nil {
			return ledgerFailure()
		}
		return terminal
	}
	if result == nil || int64(len(result.Body)) > g.maxResponseBytes {
		class := "invalid_upstream_response"
		message := "upstream inference returned an invalid response"
		if result != nil && int64(len(result.Body)) > g.maxResponseBytes {
			class, message = "upstream_response_too_large", "upstream inference response exceeded the configured limit"
		}
		if finish("failed", class, "", http.StatusOK, platform.TokenUsage{}, "unknown") != nil {
			return ledgerFailure()
		}
		return attemptResult{class: class, clientStatus: http.StatusBadGateway, message: message}
	}
	usage, finality := platformUsage(result.Usage, "final")
	providerID := safeProviderID(result.ProviderRequestID)
	if finish("succeeded", "", providerID, http.StatusOK, usage, finality) != nil {
		return ledgerFailure()
	}
	return attemptResult{success: true, body: result.Body, model: safeModel(result.Model), providerID: providerID, usage: usage, finality: finality}
}

func classifyBifrostError(value *inference.ProviderError) attemptResult {
	if value == nil || value.StatusCode == 0 {
		return attemptResult{retryable: true, class: "upstream_transport", clientStatus: http.StatusBadGateway, message: "upstream inference connection failed"}
	}
	return classifyProviderStatus(value.StatusCode)
}

func platformUsage(value *inference.Usage, knownFinality string) (platform.TokenUsage, string) {
	if value == nil {
		return platform.TokenUsage{}, "unknown"
	}
	copyValue := func(value int64) *int64 { result := value; return &result }
	if value.Finality != "" {
		knownFinality = value.Finality
	}
	result := platform.TokenUsage{Normalization: inference.NormalizationVersion}
	if value.HasPromptTokens {
		result.InputTokens = copyValue(value.PromptTokens)
	}
	if value.HasCompletionTokens {
		result.OutputTokens = copyValue(value.CompletionTokens)
	}
	if value.HasTotalTokens {
		result.TotalTokens = copyValue(value.TotalTokens)
	}
	if value.HasPromptDetails {
		result.CachedTokens = copyValue(value.CachedReadTokens)
		result.CachedWriteTokens = copyValue(value.CachedWriteTokens)
		result.CachedWriteTokens5m = copyValue(value.CachedWriteTokens5m)
		result.CachedWriteTokens1h = copyValue(value.CachedWriteTokens1h)
		result.TextInputTokens = copyValue(value.TextInputTokens)
		result.AudioInputTokens = copyValue(value.AudioInputTokens)
		result.ImageInputTokens = copyValue(value.ImageInputTokens)
	}
	if value.HasCompletionDetails {
		result.ReasoningTokens = copyValue(value.ReasoningTokens)
	}
	if result.InputTokens == nil && result.OutputTokens == nil && result.TotalTokens == nil &&
		result.CachedTokens == nil && result.CachedWriteTokens == nil &&
		result.ReasoningTokens == nil && result.TextInputTokens == nil &&
		result.AudioInputTokens == nil && result.ImageInputTokens == nil {
		return platform.TokenUsage{}, "unknown"
	}
	return result, knownFinality
}

type providerResponse struct {
	ID      string            `json:"id"`
	Model   string            `json:"model"`
	Choices []json.RawMessage `json:"choices"`
	Usage   *providerUsage    `json:"usage"`
}

type providerUsage struct {
	PromptTokens        *int64 `json:"prompt_tokens"`
	CompletionTokens    *int64 `json:"completion_tokens"`
	CachedTokens        *int64 `json:"cached_tokens"`
	ReasoningTokens     *int64 `json:"reasoning_tokens"`
	PromptTokensDetails *struct {
		CachedTokens *int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens *int64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

type providerMeta struct {
	model, providerID string
	usage             platform.TokenUsage
	finality          string
}

func parseProviderResponse(body []byte) (providerMeta, error) {
	var response providerResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&response); err != nil {
		return providerMeta{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return providerMeta{}, errors.New("multiple JSON values")
	}
	if len(response.Choices) == 0 {
		return providerMeta{}, errors.New("missing choices")
	}
	meta := providerMeta{model: safeModel(response.Model), providerID: safeProviderID(response.ID), finality: "unknown"}
	if response.Usage == nil {
		return meta, nil
	}
	usage, finality, err := parseProviderUsage(response.Usage)
	if err != nil {
		return providerMeta{}, err
	}
	meta.usage, meta.finality = usage, finality
	return meta, nil
}

func parseProviderUsage(response *providerUsage) (platform.TokenUsage, string, error) {
	var usage platform.TokenUsage
	if response == nil {
		return usage, "unknown", nil
	}
	usage.InputTokens, usage.OutputTokens = response.PromptTokens, response.CompletionTokens
	usage.CachedTokens, usage.ReasoningTokens = response.CachedTokens, response.ReasoningTokens
	if usage.CachedTokens == nil && response.PromptTokensDetails != nil {
		usage.CachedTokens = response.PromptTokensDetails.CachedTokens
	}
	if usage.ReasoningTokens == nil && response.CompletionTokensDetails != nil {
		usage.ReasoningTokens = response.CompletionTokensDetails.ReasoningTokens
	}
	for _, value := range []*int64{usage.InputTokens, usage.OutputTokens, usage.CachedTokens, usage.ReasoningTokens} {
		if value != nil && *value < 0 {
			return platform.TokenUsage{}, "unknown", errors.New("negative usage")
		}
	}
	known := usage.InputTokens != nil || usage.OutputTokens != nil || usage.CachedTokens != nil || usage.ReasoningTokens != nil
	if usage.InputTokens != nil && usage.OutputTokens != nil {
		return usage, "final", nil
	}
	if known {
		return usage, "partial", nil
	}
	return usage, "unknown", nil
}

func classifyProviderStatus(status int) attemptResult {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return attemptResult{class: "upstream_rejected", clientStatus: http.StatusBadRequest, message: "upstream inference rejected the supported request"}
	case http.StatusTooManyRequests:
		return attemptResult{retryable: true, class: "upstream_rate_limited", clientStatus: http.StatusTooManyRequests, message: "upstream inference is rate limited"}
	case http.StatusServiceUnavailable:
		return attemptResult{retryable: true, class: "upstream_unavailable", clientStatus: http.StatusServiceUnavailable, message: "upstream inference is unavailable"}
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return attemptResult{class: "target_configuration", clientStatus: http.StatusBadGateway, message: "configured inference target rejected its server-side configuration"}
	default:
		return attemptResult{class: "upstream_error", clientStatus: http.StatusBadGateway, message: "upstream inference failed"}
	}
}

func routeError(err error) (int, string, string) {
	switch {
	case errors.Is(err, platform.ErrUnavailable):
		return http.StatusServiceUnavailable, "target_unavailable", "configured inference target is unavailable"
	case errors.Is(err, platform.ErrForbidden):
		return http.StatusForbidden, "route_forbidden", "request is not authorised for this model"
	case errors.Is(err, platform.ErrNotFound):
		return http.StatusNotFound, "model_not_authorised", "request is not authorised for this model"
	default:
		return http.StatusServiceUnavailable, "route_unavailable", "route resolution is temporarily unavailable"
	}
}

func failureHealthStatus(class string) string {
	switch class {
	case "target_configuration", "upstream_rate_limited", "upstream_timeout", "upstream_transport", "upstream_unavailable", "upstream_error", "invalid_upstream_response", "upstream_response_too_large":
		return "degraded"
	default:
		return ""
	}
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool, string) {
	if value == "" {
		return 0, false, ""
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true, strconv.Itoa(seconds)
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := when.Sub(now)
		if delay < 0 {
			delay = 0
		}
		seconds := int64((delay + time.Second - 1) / time.Second)
		return delay, true, strconv.FormatInt(seconds, 10)
	}
	return 0, false, ""
}

func safeProviderID(value string) string {
	if len(value) == 0 || len(value) > 200 || !safeMetadata(value, "/") {
		return ""
	}
	return value
}

func targetEndpoint(baseURL string, allowInsecure bool) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.EscapedPath(), "%") {
		return "", errors.New("invalid target URL")
	}
	if parsed.Scheme != "https" && !(allowInsecure && parsed.Scheme == "http") {
		return "", errors.New("target URL must use HTTPS")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	parsed.RawPath = ""
	return parsed.String(), nil
}
func safeModel(value string) string {
	if len(value) == 0 || len(value) > 255 || !safeMetadata(value, "/@") {
		return ""
	}
	return value
}
func safeMetadata(value, extra string) bool {
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("._:-"+extra, char) {
			continue
		}
		return false
	}
	return true
}
func elapsed(start, end time.Time) time.Duration {
	if end.Before(start) {
		return 0
	}
	return end.Sub(start)
}
func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func detachedContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
