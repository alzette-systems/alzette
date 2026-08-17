package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alzette/internal/api"
)

const piStreamingToolsRequest = `{
  "model":"safe-chat",
  "messages":[
    {"role":"system","content":"Use tools when needed."},
    {"role":"user","content":[{"type":"text","text":"Inspect the repository."}]}
  ],
  "stream":true,
  "max_tokens":256,
  "temperature":0.2,
  "tools":[{
    "type":"function",
    "function":{
      "name":"read_file",
      "description":"Read a file",
      "parameters":{
        "type":"object",
        "properties":{"path":{"type":"string"}},
        "required":["path"],
        "additionalProperties":false
      },
      "strict":false
    }
  }],
  "tool_choice":"auto"
}`

func sse(payloads ...string) string {
	var result strings.Builder
	for _, payload := range payloads {
		result.WriteString("data: ")
		result.WriteString(payload)
		result.WriteString("\n\n")
	}
	return result.String()
}

func TestGatewayPiStreamingToolsTextAndTerminalUsage(t *testing.T) {
	const promptCanary = "Inspect the repository."
	streamBody := sse(
		`{"id":"stream-safe-1","object":"chat.completion.chunk","model":"provider/deepseek-shape","choices":[{"index":0,"delta":{"role":"assistant","content":"hel"},"finish_reason":null}]}`,
		`{"id":"stream-safe-1","object":"chat.completion.chunk","model":"provider/deepseek-shape","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}`,
		`{"id":"stream-safe-1","object":"chat.completion.chunk","model":"provider/deepseek-shape","choices":[],"usage":{"prompt_tokens":31,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":7},"completion_tokens_details":{"reasoning_tokens":2}}}`,
		`[DONE]`,
	)
	var captured ChatRequest
	var authorization, accept, correlation string
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		accept = r.Header.Get("Accept")
		correlation = r.Header.Get("X-Alzette-Request-ID")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode compatible upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, streamBody)
	}, nil)

	response := fixture.request(piStreamingToolsRequest)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" || response.Body.String() != streamBody {
		t.Fatalf("stream contract status=%d content_type_set=%t length=%d", response.Code, response.Header().Get("Content-Type") != "", response.Body.Len())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Alzette-Request-ID") == "" || correlation != response.Header().Get("X-Alzette-Request-ID") {
		t.Fatal("stream correlation or cache-control contract failed")
	}
	if authorization != "Bearer provider-secret-value" || accept != "text/event-stream" {
		t.Fatal("stream upstream authentication or content negotiation failed")
	}
	if captured.Model != "provider/model-a" || !captured.streaming() || captured.MaxTokens == nil || *captured.MaxTokens != 256 || len(captured.Tools) != 1 || captured.Tools[0].Function.Name != "read_file" {
		t.Fatal("Pi-shaped request was not forwarded through the server-owned model route")
	}
	if !bytes.Equal(captured.Messages[1].Content, json.RawMessage(`[{"type":"text","text":"Inspect the repository."}]`)) {
		t.Fatal("supported text-part content shape changed during forwarding")
	}

	record := requestRecord(t, fixture, response)
	if record.Status != "succeeded" || record.AttemptCount != 1 || record.ExecutedModel != "provider/deepseek-shape" || record.UsageFinality != "final" {
		t.Fatalf("stream accounting status=%s attempts=%d finality=%s", record.Status, record.AttemptCount, record.UsageFinality)
	}
	if record.Usage.InputTokens == nil || *record.Usage.InputTokens != 31 || record.Usage.OutputTokens == nil || *record.Usage.OutputTokens != 5 || record.Usage.CachedTokens == nil || *record.Usage.CachedTokens != 7 || record.Usage.ReasoningTokens == nil || *record.Usage.ReasoningTokens != 2 {
		t.Fatal("terminal streaming usage was not metered exactly")
	}
	if attempts := fixture.store.AttemptsForRequest(record.ID); len(attempts) != 1 || attempts[0].Status != "succeeded" {
		t.Fatal("streaming request did not preserve logical-request/provider-attempt accounting")
	}
	metadata, _ := json.Marshal(record)
	for label, forbidden := range map[string]string{"prompt": promptCanary, "output": "hello", "provider credential": "provider-secret-value", "target URL": fixture.server.URL} {
		if bytes.Contains(metadata, []byte(forbidden)) {
			t.Fatalf("stream ledger leaked %s", label)
		}
	}
}

func TestGatewayPiToolCallDeltasAndToolResultHistory(t *testing.T) {
	toolStream := sse(
		`{"id":"stream-tool-1","model":"provider/deepseek-shape","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_safe_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\""}}]},"finish_reason":null}]}`,
		`{"id":"stream-tool-1","model":"provider/deepseek-shape","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"README.md\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	)
	textStream := sse(
		`{"id":"stream-text-2","model":"provider/deepseek-shape","choices":[{"index":0,"delta":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`,
		`[DONE]`,
	)
	var calls atomic.Int32
	var history ChatRequest
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var captured ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode compatible upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			_, _ = io.WriteString(w, toolStream)
			return
		}
		history = captured
		_, _ = io.WriteString(w, textStream)
	}, nil)

	first := fixture.request(piStreamingToolsRequest)
	if first.Code != http.StatusOK || first.Body.String() != toolStream {
		t.Fatalf("tool delta stream status=%d length=%d", first.Code, first.Body.Len())
	}
	firstRecord := requestRecord(t, fixture, first)
	if firstRecord.Status != "succeeded" || firstRecord.UsageFinality != "unknown" || firstRecord.Usage.InputTokens != nil || firstRecord.Usage.OutputTokens != nil {
		t.Fatalf("missing stream usage status=%s finality=%s", firstRecord.Status, firstRecord.UsageFinality)
	}

	historyBody := `{
  "model":"safe-chat",
  "messages":[
    {"role":"user","content":"Inspect the repository."},
    {"role":"assistant","content":null,"tool_calls":[{"id":"call_safe_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}]},
    {"role":"tool","tool_call_id":"call_safe_1","content":"repository contents"},
    {"role":"user","content":"Summarise it."}
  ],
  "stream":true,
  "max_tokens":256,
  "tools":[{"type":"function","function":{"name":"read_file","description":"Read a file","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]},"strict":false}}]
}`
	second := fixture.request(historyBody)
	if second.Code != http.StatusOK || second.Body.String() != textStream || calls.Load() != 2 {
		t.Fatalf("tool history status=%d calls=%d length=%d", second.Code, calls.Load(), second.Body.Len())
	}
	if len(history.Messages) != 4 || len(history.Messages[1].ToolCalls) != 1 || history.Messages[1].ToolCalls[0].ID != "call_safe_1" || history.Messages[2].Role != "tool" || history.Messages[2].ToolCallID != "call_safe_1" {
		t.Fatal("assistant tool-call history or tool result was not forwarded intact")
	}
	secondRecord := requestRecord(t, fixture, second)
	if secondRecord.Status != "succeeded" || secondRecord.AttemptCount != 1 || secondRecord.UsageFinality != "unknown" {
		t.Fatalf("tool history accounting status=%s attempts=%d finality=%s", secondRecord.Status, secondRecord.AttemptCount, secondRecord.UsageFinality)
	}
}

func TestGatewayRejectsMalformedOrUnsafeAgentShapes(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse(`{"id":"unexpected","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":"stop"}]}`, `[DONE]`))
	}, nil)
	tests := []struct {
		name, body, code string
	}{
		{"malformed schema type", `{"model":"safe-chat","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"run","parameters":{"type":"string"}}}]}`, "invalid_tools"},
		{"malformed schema required", `{"model":"safe-chat","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"run","parameters":{"type":"object","properties":{},"required":"x"}}}]}`, "invalid_tools"},
		{"custom tool", `{"model":"safe-chat","messages":[{"role":"user","content":"x"}],"tools":[{"type":"custom","function":{"name":"run","parameters":{"type":"object","properties":{}}}}]}`, "invalid_tools"},
		{"unknown target URL", `{"model":"safe-chat","messages":[{"role":"user","content":"x"}],"stream":true,"target_url":"https://attacker.invalid"}`, "unsupported_request_field"},
		{"unknown provider model", `{"model":"safe-chat","messages":[{"role":"user","content":"x","provider_model":"attacker/model"}],"stream":true}`, "unsupported_request_field"},
		{"image content", `{"model":"safe-chat","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,eA=="}}]}],"stream":true}`, "invalid_messages"},
		{"unpaired tool result", `{"model":"safe-chat","messages":[{"role":"tool","tool_call_id":"call_1","content":"x"}],"stream":true}`, "invalid_messages"},
		{"unknown tool choice", `{"model":"safe-chat","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"run","parameters":{"type":"object","properties":{}}}}],"tool_choice":{"type":"function","function":{"name":"other"}}}`, "invalid_tool_choice"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := fixture.request(test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("agent validation status=%d length=%d", response.Code, response.Body.Len())
			}
			var envelope api.ErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != test.code || envelope.RequestID == "" {
				t.Fatal("agent validation did not return the stable error contract")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("rejected agent requests reached upstream calls=%d", calls.Load())
	}
}

type signallingRecorder struct {
	*httptest.ResponseRecorder
	wrote chan struct{}
	once  sync.Once
}

func (recorder *signallingRecorder) Write(body []byte) (int, error) {
	written, err := recorder.ResponseRecorder.Write(body)
	recorder.once.Do(func() { close(recorder.wrote) })
	return written, err
}

func TestGatewayStreamingCancellationStopsUpstreamWithoutRetry(t *testing.T) {
	upstreamStarted := make(chan struct{})
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse(`{"id":"stream-cancel","model":"provider/model","choices":[{"index":0,"delta":{"role":"assistant","content":"partial"},"finish_reason":null}]}`))
		w.(http.Flusher).Flush()
		close(upstreamStarted)
		<-r.Context().Done()
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(piStreamingToolsRequest)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	response := &signallingRecorder{ResponseRecorder: httptest.NewRecorder(), wrote: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		fixture.handler.ServeHTTP(response, request)
		close(done)
	}()
	select {
	case <-upstreamStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("streaming upstream did not start")
	}
	select {
	case <-response.wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("gateway did not commit the first streaming frame")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("gateway did not stop after streaming client cancellation")
	}
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "[DONE]") {
		t.Fatalf("cancelled stream status=%d terminal_marker_present=%t length=%d", response.Code, strings.Contains(response.Body.String(), "[DONE]"), response.Body.Len())
	}
	record := requestRecord(t, fixture, response.ResponseRecorder)
	if record.Status != "cancelled" || record.ErrorClass != "client_cancelled" || record.AttemptCount != 1 || record.UsageFinality != "unknown" {
		t.Fatalf("cancelled stream status=%s class=%s attempts=%d finality=%s", record.Status, record.ErrorClass, record.AttemptCount, record.UsageFinality)
	}
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if len(attempts) != 1 || attempts[0].Status != "cancelled" || attempts[0].ErrorClass != "client_cancelled" {
		t.Fatal("cancelled streaming provider attempt was not reconciled")
	}
	routes, err := fixture.store.ListRoutes(context.Background(), principalFor(t, fixture.store, fixture.key))
	if err != nil || len(routes) != 1 || routes[0].Target.HealthStatus != "unknown" {
		t.Fatal("client cancellation incorrectly degraded target health")
	}
}

func TestGatewayStreamingDisconnectBeforeFirstByteRetries(t *testing.T) {
	fixture := newFixture(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("custom transport unexpectedly reached fixture upstream")
	}, nil)
	complete := sse(
		`{"id":"stream-retry","model":"provider/model","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`,
		`[DONE]`,
	)
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := io.ReadCloser(&truncatedResponseBody{})
		if calls.Add(1) == 2 {
			body = io.NopCloser(strings.NewReader(complete))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: body, Request: request}, nil
	})}
	handler, err := New(Config{Store: fixture.store, AllowInsecureTargets: true, HTTPClient: client, SecretLookup: func(string) (string, bool) { return "provider-secret", true }, RetryBaseDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(piStreamingToolsRequest))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || calls.Load() != 2 || response.Body.String() != complete {
		t.Fatalf("pre-output disconnect status=%d calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	if record.Status != "succeeded" || record.AttemptCount != 2 {
		t.Fatalf("pre-output retry status=%s attempts=%d", record.Status, record.AttemptCount)
	}
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if len(attempts) != 2 || attempts[0].Status != "failed" || attempts[0].ErrorClass != "upstream_transport" || attempts[1].Status != "succeeded" {
		t.Fatal("pre-output disconnect attempt accounting is incorrect")
	}
}

func TestGatewayStreamingDisconnectAfterFirstByteDoesNotRetry(t *testing.T) {
	fixture := newFixture(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("custom transport unexpectedly reached fixture upstream")
	}, nil)
	firstFrame := sse(`{"id":"stream-partial","model":"provider/model","choices":[{"index":0,"delta":{"role":"assistant","content":"partial"},"finish_reason":null}]}`)
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: &truncatedResponseBody{contents: []byte(firstFrame)}, Request: request}, nil
	})}
	handler, err := New(Config{Store: fixture.store, AllowInsecureTargets: true, HTTPClient: client, SecretLookup: func(string) (string, bool) { return "provider-secret", true }, RetryBaseDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(piStreamingToolsRequest))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || calls.Load() != 1 || response.Body.String() != firstFrame || strings.Contains(response.Body.String(), `"error"`) {
		t.Fatalf("post-output disconnect status=%d calls=%d error_appended=%t length=%d", response.Code, calls.Load(), strings.Contains(response.Body.String(), `"error"`), response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	if record.Status != "failed" || record.ErrorClass != "upstream_transport" || record.AttemptCount != 1 || record.UsageFinality != "unknown" {
		t.Fatalf("post-output disconnect status=%s class=%s attempts=%d finality=%s", record.Status, record.ErrorClass, record.AttemptCount, record.UsageFinality)
	}
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if len(attempts) != 1 || attempts[0].Status != "failed" || attempts[0].ErrorClass != "upstream_transport" {
		t.Fatal("post-output disconnect provider attempt was not reconciled")
	}
}
