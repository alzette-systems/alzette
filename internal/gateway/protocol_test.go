package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func protocolRequest(t *testing.T, fixture *fixture, path, body, header, credential string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(header, credential)
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	return response
}

func TestResponsesAPIUsesExistingRouteAuthorityAndAccounting(t *testing.T) {
	var captured ChatRequest
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	body := `{"model":"safe-chat","instructions":"Be concise.","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}],"max_output_tokens":64,"reasoning":{"effort":"low"}}`
	response := protocolRequest(t, fixture, "/v1/responses", body, "Authorization", "Bearer "+fixture.key)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if captured.Model != "provider/model-a" || len(captured.Messages) != 2 || captured.Messages[0].Role != "system" || captured.Messages[1].Role != "user" || captured.ReasoningEffort != "low" || captured.maxOutputTokens() == nil || *captured.maxOutputTokens() != 64 {
		t.Fatalf("translated upstream request = %#v", captured)
	}
	var result struct {
		Object string `json:"object"`
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Object != "response" || result.Status != "completed" || result.Model != "safe-chat" || len(result.Output) != 1 || result.Output[0].Type != "message" || result.Output[0].Content[0].Text != "hello" {
		t.Fatalf("Responses envelope = %#v", result)
	}
	if result.Usage.InputTokens != 11 || result.Usage.OutputTokens != 7 || bytes.Contains(response.Body.Bytes(), []byte("provider/executed-model")) {
		t.Fatal("Responses usage or public model boundary is incorrect")
	}
	record := requestRecord(t, fixture, response)
	if record.Status != "succeeded" || record.ModelAlias != "safe-chat" || record.ExecutedModel != "provider/executed-model" || record.UsageFinality != "final" {
		t.Fatalf("request record = %#v", record)
	}
}

func TestAnthropicMessagesAcceptsXAPIKeyAndTranslatesTools(t *testing.T) {
	var captured ChatRequest
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_provider_1","model":"provider/private","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_weather_1","type":"function","function":{"name":"weather","arguments":"{\"city\":\"Luxembourg\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":9,"completion_tokens":4}}`)
	}, nil)
	body := `{"model":"safe-chat","max_tokens":128,"system":"Use tools.","messages":[{"role":"user","content":"Weather?"}],"tools":[{"name":"weather","description":"Get weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}],"tool_choice":{"type":"auto"}}`
	response := protocolRequest(t, fixture, "/v1/messages", body, "X-Api-Key", fixture.key)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if captured.Model != "provider/model-a" || len(captured.Tools) != 1 || captured.Tools[0].Function.Name != "weather" || len(captured.Messages) != 2 || captured.Messages[0].Role != "system" {
		t.Fatalf("translated upstream request = %#v", captured)
	}
	var result struct {
		Type       string `json:"type"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type  string                 `json:"type"`
			ID    string                 `json:"id"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Type != "message" || result.Model != "safe-chat" || result.StopReason != "tool_use" || len(result.Content) != 1 || result.Content[0].Type != "tool_use" || result.Content[0].Name != "weather" || result.Content[0].Input["city"] != "Luxembourg" {
		t.Fatalf("Anthropic envelope = %#v", result)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("provider/private")) {
		t.Fatal("provider model leaked through Anthropic response")
	}
}

func TestTranslatedStreamingAPIsPreserveEventsUsageAndLedger(t *testing.T) {
	streamBody := sse(
		`{"id":"stream-safe-1","model":"provider/private","choices":[{"index":0,"delta":{"role":"assistant","content":"hel"},"finish_reason":null}]}`,
		`{"id":"stream-safe-1","model":"provider/private","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}`,
		`{"id":"stream-safe-1","model":"provider/private","choices":[],"usage":{"prompt_tokens":13,"completion_tokens":5}}`,
		`[DONE]`,
	)
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, streamBody)
	}, nil)
	tests := []struct {
		name, path, body, header, credential string
		required                             []string
	}{
		{
			name: "Responses", path: "/v1/responses", body: `{"model":"safe-chat","input":"Hello","stream":true}`, header: "Authorization", credential: "Bearer " + fixture.key,
			required: []string{"event: response.created", "event: response.output_text.delta", `"delta":"hel"`, "event: response.completed", `"input_tokens":13`, `"model":"safe-chat"`},
		},
		{
			name: "Anthropic", path: "/v1/messages", body: `{"model":"safe-chat","max_tokens":64,"messages":[{"role":"user","content":"Hello"}],"stream":true}`, header: "X-Api-Key", credential: fixture.key,
			required: []string{"event: message_start", "event: content_block_delta", `"text":"hel"`, "event: message_delta", "event: message_stop", `"model":"safe-chat"`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := protocolRequest(t, fixture, test.path, test.body, test.header, test.credential)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" {
				t.Fatalf("status=%d content-type=%q body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
			for _, expected := range test.required {
				if !strings.Contains(response.Body.String(), expected) {
					t.Fatalf("missing %q in stream %s", expected, response.Body.String())
				}
			}
			if strings.Contains(response.Body.String(), "provider/private") || strings.Contains(response.Body.String(), "[DONE]") {
				t.Fatal("translated stream leaked provider model or Chat terminal marker")
			}
			record := requestRecord(t, fixture, response)
			if record.Status != "succeeded" || record.Usage.InputTokens == nil || *record.Usage.InputTokens != 13 || record.Usage.OutputTokens == nil || *record.Usage.OutputTokens != 5 {
				t.Fatalf("stream ledger = %#v", record)
			}
		})
	}
}

func TestTranslatedStreamingToolCalls(t *testing.T) {
	streamBody := sse(
		`{"id":"stream-tool-1","model":"provider/private","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_weather_1","type":"function","function":{"name":"weather","arguments":"{\"city\":\""}}]},"finish_reason":null}]}`,
		`{"id":"stream-tool-1","model":"provider/private","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"Luxembourg\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`{"id":"stream-tool-1","model":"provider/private","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":6}}`,
		`[DONE]`,
	)
	tests := []struct {
		name, path, body, header string
		required                 []string
	}{
		{
			name: "Responses", path: "/v1/responses", header: "Authorization",
			body:     `{"model":"safe-chat","input":"Weather?","stream":true,"tools":[{"type":"function","name":"weather","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}]}`,
			required: []string{"event: response.output_item.added", "event: response.function_call_arguments.delta", "event: response.function_call_arguments.done", `"arguments":"{\"city\":\"Luxembourg\"}"`, "event: response.completed"},
		},
		{
			name: "Anthropic", path: "/v1/messages", header: "X-Api-Key",
			body:     `{"model":"safe-chat","max_tokens":64,"messages":[{"role":"user","content":"Weather?"}],"stream":true,"tools":[{"name":"weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}]}`,
			required: []string{"event: content_block_start", `"type":"tool_use"`, `"type":"input_json_delta"`, `"partial_json":"Luxembourg\"}"`, `"stop_reason":"tool_use"`, "event: message_stop"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, streamBody)
			}, nil)
			credential := fixture.key
			if test.header == "Authorization" {
				credential = "Bearer " + credential
			}
			response := protocolRequest(t, fixture, test.path, test.body, test.header, credential)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			for _, expected := range test.required {
				if !strings.Contains(response.Body.String(), expected) {
					t.Fatalf("missing %q in %s", expected, response.Body.String())
				}
			}
			record := requestRecord(t, fixture, response)
			if record.Status != "succeeded" || record.UsageFinality != "final" {
				t.Fatalf("tool stream record = %#v", record)
			}
		})
	}
}

func TestProtocolAdaptersFailClosedBeforeUpstream(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	tests := []struct {
		path, body, header, credential, errorType string
	}{
		{"/v1/responses", `{"model":"safe-chat","input":"x","previous_response_id":"resp_unsafe"}`, "Authorization", "Bearer " + fixture.key, ""},
		{"/v1/messages", `{"model":"safe-chat","max_tokens":8,"messages":[{"role":"user","content":"x"}],"thinking":{"type":"enabled","budget_tokens":1024}}`, "X-Api-Key", fixture.key, "invalid_request_error"},
	}
	for _, test := range tests {
		response := protocolRequest(t, fixture, test.path, test.body, test.header, test.credential)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
		if test.errorType != "" {
			var envelope struct {
				Type  string `json:"type"`
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}
			if json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.Type != "error" || envelope.Error.Type != test.errorType {
				t.Fatalf("Anthropic error envelope = %s", response.Body.String())
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("unsupported protocol input reached upstream %d times", calls.Load())
	}

	mixed := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"safe-chat","max_tokens":8,"messages":[{"role":"user","content":"x"}]}`))
	mixed.Header.Set("Content-Type", "application/json")
	mixed.Header.Set("Authorization", "Bearer "+fixture.key)
	mixed.Header.Set("X-Api-Key", fixture.key)
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, mixed)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("mixed credentials status=%d", response.Code)
	}
}
