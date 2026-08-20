// Package inference owns Alzette's embedded provider-protocol engine.
//
// Bifrost parses and translates provider requests and responses. Alzette still
// owns authentication, tenant/model routing, retry policy, request lifecycle,
// and the immutable usage ledger around every call into this package.
package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/providers/deepseek"
	"github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/providers/openrouter"
	"github.com/maximhq/bifrost/core/schemas"
)

const NormalizationVersion = "bifrost-core/v1.7.13"

// Target is the operator-controlled provider endpoint selected by Alzette's
// authenticated route resolver. It never comes from a client request.
type Target struct {
	BaseURL             string
	Model               string
	Provider            string
	Timeout             time.Duration
	AllowPrivateNetwork bool
}

// Usage preserves the provider-reported dimensions needed by Alzette's ledger.
// PromptTokens is already Bifrost-normalized and includes cached reads/writes;
// callers must never add the cache dimensions to PromptTokens again.
type Usage struct {
	PromptTokens         int64
	CompletionTokens     int64
	TotalTokens          int64
	CachedReadTokens     int64
	CachedWriteTokens    int64
	CachedWriteTokens5m  int64
	CachedWriteTokens1h  int64
	ReasoningTokens      int64
	TextInputTokens      int64
	AudioInputTokens     int64
	ImageInputTokens     int64
	Finality             string
	HasPromptTokens      bool
	HasCompletionTokens  bool
	HasTotalTokens       bool
	HasPromptDetails     bool
	HasCompletionDetails bool
}

type Result struct {
	Body              []byte
	Model             string
	ProviderRequestID string
	Usage             *Usage
}

type ProviderError struct {
	StatusCode        int
	ProviderRequestID string
	BilledUsage       *Usage
	Message           string
	RetryAfter        string
}

func (e *ProviderError) Error() string {
	if e == nil || e.Message == "" {
		return "provider request failed"
	}
	return e.Message
}

// Engine caches transport-safe Bifrost provider clients by non-secret target
// configuration. Provider credentials are supplied only to the individual call
// and are never retained in the cache.
type Engine struct {
	mu        sync.Mutex
	providers map[string]schemas.Provider
	logger    schemas.Logger
}

func New() *Engine {
	return &Engine{providers: make(map[string]schemas.Provider), logger: noopLogger{}}
}

func (e *Engine) Chat(ctx context.Context, target Target, secret, requestID string, body []byte) (*Result, *ProviderError) {
	provider, providerName, err := e.provider(target)
	if err != nil {
		return nil, &ProviderError{Message: err.Error()}
	}
	bifrostContext := requestContext(ctx, requestID)
	request, err := decodeChatRequest(bifrostContext, body, target.Model, providerName)
	if err != nil {
		return nil, &ProviderError{Message: "request could not be represented for Bifrost"}
	}
	response, bifrostErr := provider.ChatCompletion(bifrostContext, providerKey(secret), request)
	if bifrostErr != nil {
		return nil, providerError(bifrostContext, bifrostErr)
	}
	if response == nil {
		return nil, &ProviderError{Message: "provider returned an empty response"}
	}
	body, err = publicChatJSON(response)
	if err != nil {
		return nil, &ProviderError{Message: "provider response could not be encoded"}
	}
	return &Result{
		Body:              body,
		Model:             response.Model,
		ProviderRequestID: responseRequestID(response.ID, response.ExtraFields.ProviderResponseHeaders),
		Usage:             usageWithEvidence(response.Usage, response.ExtraFields.RawResponse),
	}, nil
}

func (e *Engine) provider(target Target) (schemas.Provider, schemas.ModelProvider, error) {
	if target.BaseURL == "" {
		return nil, "", errors.New("Bifrost target base URL is required")
	}
	providerName, err := providerForTarget(target)
	if err != nil {
		return nil, "", err
	}
	key := fmt.Sprintf("%s\x00%s\x00%t\x00%d", providerName, target.BaseURL, target.AllowPrivateNetwork, target.Timeout)
	e.mu.Lock()
	defer e.mu.Unlock()
	if provider := e.providers[key]; provider != nil {
		return provider, providerName, nil
	}
	timeoutSeconds := int(target.Timeout / time.Second)
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	baseURL := strings.TrimRight(target.BaseURL, "/")
	if providerName == schemas.OpenAI || providerName == schemas.OpenRouter {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	config := &schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:                        baseURL,
			DefaultRequestTimeoutInSeconds: timeoutSeconds,
			MaxRetries:                     0,
			AllowPrivateNetwork:            target.AllowPrivateNetwork,
		},
		ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{Concurrency: 64, BufferSize: 256},
		SendBackRawRequest:       false,
		// Capture is transient and used only to retain numeric field presence.
		// publicChatJSON removes it before returning and Alzette never persists it.
		SendBackRawResponse:     true,
		StoreRawRequestResponse: false,
	}
	var provider schemas.Provider
	switch providerName {
	case schemas.DeepSeek:
		provider, err = deepseek.NewDeepSeekProvider(config, e.logger)
	case schemas.OpenRouter:
		provider = openrouter.NewOpenRouterProvider(config, e.logger)
	case schemas.OpenAI:
		provider = openai.NewOpenAIProvider(config, e.logger)
	default:
		return nil, "", fmt.Errorf("Bifrost provider %q is not supported", providerName)
	}
	if err != nil {
		return nil, "", fmt.Errorf("initialise Bifrost provider %q: %w", providerName, err)
	}
	e.providers[key] = provider
	return provider, providerName, nil
}

func providerForTarget(target Target) (schemas.ModelProvider, error) {
	if target.Provider != "" {
		switch schemas.ModelProvider(strings.ToLower(target.Provider)) {
		case schemas.DeepSeek:
			return schemas.DeepSeek, nil
		case schemas.OpenRouter:
			return schemas.OpenRouter, nil
		case schemas.OpenAI:
			return schemas.OpenAI, nil
		default:
			return "", fmt.Errorf("Bifrost provider %q is not supported", target.Provider)
		}
	}
	parsed, err := url.Parse(target.BaseURL)
	if err != nil || parsed.Hostname() == "" {
		return "", errors.New("Bifrost target base URL is invalid")
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "api.deepseek.com":
		return schemas.DeepSeek, nil
	case "openrouter.ai", "www.openrouter.ai":
		return schemas.OpenRouter, nil
	default:
		// Private compatible endpoints deliberately use Bifrost's generic OpenAI
		// adapter. The URL remains operator-owned and is still validated by the
		// gateway before execution.
		return schemas.OpenAI, nil
	}
}

func requestContext(ctx context.Context, requestID string) *schemas.BifrostContext {
	deadline := schemas.NoDeadline
	if value, ok := ctx.Deadline(); ok {
		deadline = value
	}
	result := schemas.NewBifrostContext(ctx, deadline)
	result.SetValue(schemas.BifrostContextKeyRequestID, requestID)
	result.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{"X-Alzette-Request-ID": {requestID}})
	return result
}

func decodeChatRequest(ctx *schemas.BifrostContext, body []byte, model string, providerName schemas.ModelProvider) (*schemas.BifrostChatRequest, error) {
	var wire openai.OpenAIChatRequest
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, err
	}
	// Bifrost Core v1.7.13 releases its pooled fasthttp request stream from a
	// cancellation goroutine while the SSE reader may still be inside Read.
	// Alzette therefore keeps streaming on its context-aware net/http transport.
	// Reject streaming here as a second, package-level boundary so a future
	// gateway refactor cannot accidentally reintroduce the unsafe code path.
	if wire.Stream != nil && *wire.Stream {
		return nil, errors.New("streaming requests require Alzette's streaming transport")
	}
	wire.Model = model
	wire.Provider = providerName
	request := wire.ToBifrostChatRequest(ctx)
	if request == nil || request.Input == nil {
		return nil, errors.New("empty Bifrost request")
	}
	request.Provider = providerName
	request.Model = model
	return request, nil
}

func providerKey(secret string) schemas.Key {
	return schemas.Key{
		ID:     "alzette-route-key",
		Name:   "alzette-route-key",
		Value:  *schemas.NewSecretVar(secret),
		Models: schemas.WhiteList{"*"},
		Weight: 1,
	}
}

func providerError(ctx *schemas.BifrostContext, value *schemas.BifrostError) *ProviderError {
	if value == nil {
		return nil
	}
	result := &ProviderError{
		BilledUsage: usage(value.ExtraFields.BilledUsage),
		Message:     "provider request failed",
	}
	if value.EventID != nil {
		result.ProviderRequestID = *value.EventID
	}
	if value.StatusCode != nil {
		result.StatusCode = *value.StatusCode
	}
	if value.Error != nil && value.Error.Message != "" {
		// This string is internal only. The gateway maps it to a safe public error.
		result.Message = value.Error.Message
	}
	if headers, ok := ctx.Value(schemas.BifrostContextKeyProviderResponseHeaders).(map[string]string); ok {
		result.ProviderRequestID = responseRequestID(result.ProviderRequestID, headers)
		for key, value := range headers {
			if strings.EqualFold(key, "retry-after") {
				result.RetryAfter = value
				break
			}
		}
	}
	return result
}

func usage(value *schemas.BifrostLLMUsage) *Usage {
	if value == nil {
		return nil
	}
	result := &Usage{
		PromptTokens:        int64(value.PromptTokens),
		CompletionTokens:    int64(value.CompletionTokens),
		TotalTokens:         int64(value.TotalTokens),
		HasPromptTokens:     value.PromptTokens != 0,
		HasCompletionTokens: value.CompletionTokens != 0,
		HasTotalTokens:      value.TotalTokens != 0,
	}
	if value.PromptTokensDetails != nil {
		result.HasPromptDetails = true
		details := value.PromptTokensDetails
		result.CachedReadTokens = int64(details.CachedReadTokens)
		result.CachedWriteTokens = int64(details.CachedWriteTokens)
		result.TextInputTokens = int64(details.TextTokens)
		result.AudioInputTokens = int64(details.AudioTokens)
		result.ImageInputTokens = int64(details.ImageTokens)
		if details.CachedWriteTokenDetails != nil {
			result.CachedWriteTokens5m = int64(details.CachedWriteTokenDetails.CachedWriteTokens5m)
			result.CachedWriteTokens1h = int64(details.CachedWriteTokenDetails.CachedWriteTokens1h)
		}
	}
	if value.CompletionTokensDetails != nil {
		result.HasCompletionDetails = true
		result.ReasoningTokens = int64(value.CompletionTokensDetails.ReasoningTokens)
	}
	result.Finality = "partial"
	return result
}

func usageWithEvidence(value *schemas.BifrostLLMUsage, raw interface{}) *Usage {
	result := usage(value)
	if result == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err == nil {
		var envelope struct {
			Usage map[string]json.RawMessage `json:"usage"`
		}
		if json.Unmarshal(data, &envelope) == nil {
			_, result.HasPromptTokens = envelope.Usage["prompt_tokens"]
			_, result.HasCompletionTokens = envelope.Usage["completion_tokens"]
			_, result.HasTotalTokens = envelope.Usage["total_tokens"]
			_, result.HasPromptDetails = envelope.Usage["prompt_tokens_details"]
			_, result.HasCompletionDetails = envelope.Usage["completion_tokens_details"]
		}
	}
	if result.HasPromptTokens && result.HasCompletionTokens {
		result.Finality = "final"
	}
	return result
}

func responseRequestID(id string, headers map[string]string) string {
	for _, name := range []string{"x-generation-id", "x-request-id", "request-id"} {
		if value := headers[name]; value != "" {
			return value
		}
		if value := headers[canonicalHeader(name)]; value != "" {
			return value
		}
	}
	return id
}

func canonicalHeader(name string) string {
	result := make([]byte, len(name))
	upper := true
	for index := range name {
		char := name[index]
		if upper && char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}
		result[index] = char
		upper = char == '-'
	}
	return string(result)
}

func publicChatJSON(response *schemas.BifrostChatResponse) ([]byte, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	delete(object, "extra_fields")
	delete(object, "diagnostics")
	return json.Marshal(object)
}

// noopLogger prevents provider internals from writing prompts, responses, or
// credentials into Alzette process logs. Safe request lifecycle logging remains
// Alzette-owned outside the inference engine.
type noopLogger struct{}

func (noopLogger) Debug(string, ...any)                   {}
func (noopLogger) Info(string, ...any)                    {}
func (noopLogger) Warn(string, ...any)                    {}
func (noopLogger) Error(string, ...any)                   {}
func (noopLogger) Fatal(string, ...any)                   {}
func (noopLogger) SetLevel(schemas.LogLevel)              {}
func (noopLogger) SetOutputType(schemas.LoggerOutputType) {}
func (noopLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}
