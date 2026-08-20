package inference

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBifrostBufferedExecutionPreservesUsageDimensionsWithoutLeakingInternals(t *testing.T) {
	var authorization, correlation, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		correlation = r.Header.Get("X-Alzette-Request-ID")
		path = r.URL.Path
		w.Header().Set("X-Generation-ID", "generation-safe-1")
		_, _ = io.WriteString(w, `{
			"id":"provider-body-id","model":"provider/served","choices":[{"index":0,"message":{"role":"assistant","content":"hello"}}],
			"usage":{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25,
				"prompt_tokens_details":{"cached_read_tokens":7,"cached_write_tokens":3,"text_tokens":10,"image_tokens":2,"audio_tokens":1,"cached_write_token_details":{"cached_write_tokens_5m":2,"cached_write_tokens_1h":1}},
				"completion_tokens_details":{"reasoning_tokens":4}}
		}`)
	}))
	t.Cleanup(server.Close)

	result, providerErr := New().Chat(context.Background(), Target{
		BaseURL: server.URL + "/api/v1", Model: "provider/requested",
		Timeout: time.Second, AllowPrivateNetwork: true,
	}, "provider-secret-canary", "req_safe_1", []byte(`{"model":"public-alias","messages":[{"role":"user","content":"private prompt"}]}`))
	if providerErr != nil {
		t.Fatalf("Bifrost execution: %v", providerErr)
	}
	if path != "/api/v1/chat/completions" || authorization != "Bearer provider-secret-canary" || correlation != "req_safe_1" {
		t.Fatalf("provider request path=%q authorization=%q correlation=%q", path, authorization, correlation)
	}
	if result.ProviderRequestID != "generation-safe-1" || result.Model != "provider/served" || result.Usage == nil {
		t.Fatalf("result metadata = %#v", result)
	}
	usage := result.Usage
	if usage.Finality != "final" || usage.PromptTokens != 20 || usage.CompletionTokens != 5 || usage.TotalTokens != 25 || usage.CachedReadTokens != 7 || usage.CachedWriteTokens != 3 || usage.CachedWriteTokens5m != 2 || usage.CachedWriteTokens1h != 1 || usage.ReasoningTokens != 4 || usage.TextInputTokens != 10 || usage.ImageInputTokens != 2 || usage.AudioInputTokens != 1 {
		t.Fatalf("usage = %#v", usage)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(result.Body, &body); err != nil {
		t.Fatal(err)
	}
	if _, exists := body["extra_fields"]; exists || strings.Contains(string(result.Body), "provider-secret-canary") || strings.Contains(string(result.Body), server.URL) {
		t.Fatalf("public response leaked Bifrost/provider internals: %s", result.Body)
	}
}

func TestBifrostUsageKeepsMissingCompletionPartial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"id":"partial","model":"provider/model","choices":[{}],"usage":{"prompt_tokens":9}}`)
	}))
	t.Cleanup(server.Close)
	result, providerErr := New().Chat(context.Background(), Target{BaseURL: server.URL + "/v1", Model: "provider/model", Timeout: time.Second, AllowPrivateNetwork: true}, "secret", "req_partial", []byte(`{"model":"alias","messages":[{"role":"user","content":"x"}]}`))
	if providerErr != nil {
		t.Fatal(providerErr)
	}
	if result.Usage == nil || result.Usage.Finality != "partial" || !result.Usage.HasPromptTokens || result.Usage.HasCompletionTokens {
		t.Fatalf("partial usage = %#v", result.Usage)
	}
}

func TestBifrostErrorPreservesSafeRetryEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.Header().Set("X-Request-ID", "provider-error-id")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"provider-only detail","type":"rate_limit_error"}}`)
	}))
	t.Cleanup(server.Close)
	_, providerErr := New().Chat(context.Background(), Target{BaseURL: server.URL + "/v1", Model: "provider/model", Timeout: time.Second, AllowPrivateNetwork: true}, "secret", "req_error", []byte(`{"model":"alias","messages":[{"role":"user","content":"x"}]}`))
	if providerErr == nil || providerErr.StatusCode != http.StatusTooManyRequests || providerErr.RetryAfter != "7" || providerErr.ProviderRequestID != "provider-error-id" {
		t.Fatalf("provider error = %#v", providerErr)
	}
}

func TestBifrostEngineRejectsStreamingBeforeProviderCall(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)

	_, providerErr := New().Chat(context.Background(), Target{
		BaseURL: server.URL + "/v1", Model: "provider/model",
		Timeout: time.Second, AllowPrivateNetwork: true,
	}, "secret", "req_stream_guard", []byte(`{"model":"alias","messages":[{"role":"user","content":"x"}],"stream":true}`))
	if providerErr == nil || !strings.Contains(providerErr.Message, "could not be represented") {
		t.Fatalf("streaming Bifrost request error = %#v", providerErr)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("streaming request reached Bifrost provider %d time(s)", got)
	}
}

func TestBifrostDeepSeekAdapterUsesCompatiblePathAndForcedToolChoice(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = io.WriteString(w, `{"id":"deepseek-tool","model":"deepseek-v4-flash","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"demo_weather","arguments":"{\"city\":\"Luxembourg\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`)
	}))
	t.Cleanup(server.Close)

	result, providerErr := New().Chat(context.Background(), Target{
		BaseURL: server.URL, Model: "deepseek-v4-flash", Provider: "deepseek",
		Timeout: time.Second, AllowPrivateNetwork: true,
	}, "secret", "req_deepseek_tool", []byte(`{"model":"alias","messages":[{"role":"user","content":"use the tool"}],"tools":[{"type":"function","function":{"name":"demo_weather","parameters":{"type":"object"}}}],"tool_choice":"required"}`))
	if providerErr != nil {
		t.Fatalf("DeepSeek Bifrost execution: %v", providerErr)
	}
	if path != "/chat/completions" {
		t.Fatalf("DeepSeek path = %q", path)
	}
	if result == nil || !strings.Contains(string(result.Body), `"name":"demo_weather"`) || result.Usage == nil || result.Usage.TotalTokens != 20 {
		t.Fatalf("DeepSeek result = %#v body=%s", result, result.Body)
	}
}

func TestBifrostProviderSelectionIsOperatorTargetBound(t *testing.T) {
	for _, test := range []struct {
		target Target
		want   string
	}{
		{Target{BaseURL: "https://api.deepseek.com"}, "deepseek"},
		{Target{BaseURL: "https://openrouter.ai/api/v1"}, "openrouter"},
		{Target{BaseURL: "https://private.example.test/v1"}, "openai"},
		{Target{BaseURL: "https://private.example.test/v1", Provider: "deepseek"}, "deepseek"},
	} {
		got, err := providerForTarget(test.target)
		if err != nil || string(got) != test.want {
			t.Fatalf("providerForTarget(%#v) = %q, %v; want %q", test.target, got, err, test.want)
		}
	}
	if _, err := providerForTarget(Target{BaseURL: "https://private.example.test", Provider: "unknown"}); err == nil {
		t.Fatal("unsupported explicit Bifrost provider was accepted")
	}
}
