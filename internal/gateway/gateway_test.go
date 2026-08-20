package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alzette/internal/api"
	"alzette/internal/credentials"
	"alzette/internal/inference"
	"alzette/internal/platform"
	"alzette/internal/secrets"
	"alzette/internal/store/memory"
)

const successfulResponse = `{"id":"gen-safe_123","model":"provider/executed-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":11,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":2}}}`

type fixture struct {
	t       *testing.T
	store   *memory.Store
	handler http.Handler
	key     string
	result  platform.ProvisionResult
	server  *httptest.Server
}

type completionFailureStore struct {
	platform.Store
	failAttemptCompletion bool
	failRequestCompletion bool
}

func (store completionFailureStore) CompleteProviderAttempt(ctx context.Context, finish platform.AttemptFinish) error {
	if store.failAttemptCompletion {
		return platform.ErrUnavailable
	}
	return store.Store.CompleteProviderAttempt(ctx, finish)
}

func (store completionFailureStore) CompleteInferenceRequest(ctx context.Context, finish platform.RequestFinish) error {
	if store.failRequestCompletion {
		return platform.ErrUnavailable
	}
	return store.Store.CompleteInferenceRequest(ctx, finish)
}

func newFixture(t *testing.T, upstream http.HandlerFunc, mutate func(*platform.ProvisionSpec)) *fixture {
	t.Helper()
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)
	store := memory.New()
	spec := platform.ProvisionSpec{
		OrganisationName: "Tenant A", OrganisationSlug: "tenant-a",
		ProjectName: "Application A", ProjectSlug: "application-a",
		EnvironmentName: "Production", EnvironmentSlug: "production",
		ModelAlias: "safe-chat", ModelVersion: "2026-08",
		TargetName: "fake-openrouter-a", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: server.URL + "/api/v1", ProviderModel: "provider/model-a", SecretRef: "OPENROUTER_TEST_KEY",
		TargetTimeout: time.Second, MaxAttempts: 2, ServiceAccount: "application",
		Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead},
	}
	if mutate != nil {
		mutate(&spec)
	}
	result, err := store.Provision(context.Background(), spec)
	if err != nil {
		t.Fatalf("provision fixture: %v", err)
	}
	gateway, err := New(Config{Store: store, AllowInsecureTargets: true, SecretLookup: func(name string) (string, bool) {
		if name == "OPENROUTER_TEST_KEY" {
			return "provider-secret-value", true
		}
		return "", false
	}, RetryBaseDelay: time.Millisecond, MaxRetryDelay: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, store: store, handler: gateway, key: result.APIKey, result: result, server: server}
}

func (f *fixture) request(body string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.key)
	res := httptest.NewRecorder()
	f.handler.ServeHTTP(res, req)
	return res
}

func validBody(alias, prompt string) string {
	value, _ := json.Marshal(map[string]interface{}{"model": alias, "messages": []map[string]string{{"role": "user", "content": prompt}}, "temperature": 0.2})
	return string(value)
}

func rawText(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func principalFor(t *testing.T, store *memory.Store, key string) platform.Principal {
	t.Helper()
	principal, err := store.Authenticate(context.Background(), credentials.Digest(key))
	if err != nil {
		t.Fatal(err)
	}
	return principal
}

func requestRecord(t *testing.T, f *fixture, response *httptest.ResponseRecorder) platform.InferenceRequest {
	t.Helper()
	id := response.Header().Get("X-Alzette-Request-ID")
	if id == "" {
		t.Fatal("missing Alzette request ID")
	}
	record, err := f.store.GetInferenceRequest(context.Background(), principalFor(t, f.store, f.key), id)
	if err != nil {
		t.Fatalf("get request record: %v", err)
	}
	return record
}

func TestGatewayRoutesAliasAndProviderCredentialServerSide(t *testing.T) {
	var received ChatRequest
	var authorization, requestID, requestPath string
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		authorization, requestID, requestPath = r.Header.Get("Authorization"), r.Header.Get("X-Alzette-Request-ID"), r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Generation-Id", "header-generation-id")
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	prompt := "private prompt that must not enter metadata"
	response := fixture.request(validBody("safe-chat", prompt))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
	}
	if authorization != "Bearer provider-secret-value" {
		t.Fatal("upstream authorization did not use the configured provider credential")
	}
	if requestPath != "/api/v1/chat/completions" {
		t.Fatalf("upstream path = %q", requestPath)
	}
	if received.Model != "provider/model-a" {
		t.Fatalf("upstream model = %q", received.Model)
	}
	if !bytes.Equal(received.Messages[0].Content, rawText(prompt)) {
		t.Fatal("gateway did not proxy the request body")
	}
	if requestID == "" || requestID != response.Header().Get("X-Alzette-Request-ID") {
		t.Fatal("request correlation did not traverse the gateway")
	}

	record := requestRecord(t, fixture, response)
	if record.ModelAlias != "safe-chat" || record.ExecutedModel != "provider/executed-model" || record.ProviderRequestID != "header-generation-id" {
		t.Fatalf("request metadata = %#v", record)
	}
	if record.AttemptCount != 1 || record.Status != "succeeded" || record.UsageFinality != "final" {
		t.Fatalf("request accounting = %#v", record)
	}
	if record.Usage.InputTokens == nil || *record.Usage.InputTokens != 11 || record.Usage.CachedTokens == nil || *record.Usage.CachedTokens != 3 {
		t.Fatalf("usage = %#v", record.Usage)
	}
	metadata, _ := json.Marshal(record)
	for label, forbidden := range map[string]string{"prompt content": prompt, "provider credential": "provider-secret-value", "upstream URL": fixture.server.URL} {
		if bytes.Contains(metadata, []byte(forbidden)) {
			t.Fatalf("metadata leaked %s", label)
		}
	}
	if attempts := fixture.store.AttemptsForRequest(record.ID); len(attempts) != 1 || attempts[0].Status != "succeeded" {
		t.Fatalf("attempts = %#v", attempts)
	}
}

func TestGatewayDefaultSecretLookupPrefersMountedFile(t *testing.T) {
	var receivedAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthorization = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, successfulResponse)
	}))
	defer server.Close()

	secretFile := filepath.Join(t.TempDir(), "provider-key")
	if err := os.WriteFile(secretFile, []byte("file-provider-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_FILE_TEST", "environment-provider-secret")
	t.Setenv("OPENROUTER_FILE_TEST_FILE", secretFile)
	store := memory.New()
	result, err := store.Provision(context.Background(), platform.ProvisionSpec{
		OrganisationName: "File Secret", OrganisationSlug: "file-secret", ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Test", EnvironmentSlug: "test",
		ModelAlias: "file-chat", ModelVersion: "v1", TargetName: "file-target", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: server.URL + "/api/v1", ProviderModel: "provider/model", SecretRef: "OPENROUTER_FILE_TEST", TargetTimeout: time.Second, MaxAttempts: 1,
		ServiceAccount: "application", Scopes: []string{platform.ScopeInferenceWrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(Config{Store: store, AllowInsecureTargets: true, AllowedSecretRefs: []string{"OPENROUTER_FILE_TEST"}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("file-chat", "deterministic fake request")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+result.APIKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("file-secret gateway status=%d length=%d", response.Code, response.Body.Len())
	}
	if receivedAuthorization != "Bearer file-provider-secret" {
		t.Fatal("gateway did not prefer the mounted provider secret file")
	}
}

func TestGatewayDefaultResolverRejectsUnapprovedEnvironmentReference(t *testing.T) {
	const unapprovedReference = "DATABASE_URL"
	t.Setenv(unapprovedReference, "header-unsafe-application-setting")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)
	store := memory.New()
	result, err := store.Provision(context.Background(), platform.ProvisionSpec{
		OrganisationName: "Secret Policy", OrganisationSlug: "secret-policy", ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Test", EnvironmentSlug: "test",
		ModelAlias: "policy-chat", ModelVersion: "v1", TargetName: "policy-target", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: server.URL + "/v1", ProviderModel: "provider/model", SecretRef: unapprovedReference, TargetTimeout: time.Second, MaxAttempts: 1,
		ServiceAccount: "application", Scopes: []string{platform.ScopeInferenceWrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(Config{Store: store, AllowInsecureTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("policy-chat", "private policy prompt")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+result.APIKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || calls.Load() != 0 {
		t.Fatalf("unapproved secret reference status=%d calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
	}
	for _, forbidden := range []string{"header-unsafe-application-setting", "private policy prompt", server.URL, result.APIKey} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatal("unapproved secret reference failure exposed sensitive material")
		}
	}
}

func TestGatewayTimeoutThenSuccessIsOneRequestAndTwoAttempts(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			time.Sleep(150 * time.Millisecond)
			return
		}
		_, _ = io.WriteString(w, successfulResponse)
	}, func(spec *platform.ProvisionSpec) { spec.TargetTimeout = 100 * time.Millisecond })
	response := fixture.request(validBody("safe-chat", "retry me"))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	if calls.Load() != 2 || record.AttemptCount != 2 {
		t.Fatalf("calls=%d request=%#v", calls.Load(), record)
	}
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if len(attempts) != 2 || attempts[0].ErrorClass != "upstream_timeout" || attempts[1].Status != "succeeded" {
		t.Fatalf("attempts = %#v", attempts)
	}
	routes, err := fixture.store.ListRoutes(context.Background(), principalFor(t, fixture.store, fixture.key))
	if err != nil || len(routes) != 1 || routes[0].Target.HealthStatus != "operational" || routes[0].Target.LastSuccessAt == nil {
		t.Fatal("successful retry did not leave the target operational")
	}
}

func TestGatewayTerminalTimeoutIsBounded(t *testing.T) {
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
	}, func(spec *platform.ProvisionSpec) {
		spec.TargetTimeout = 100 * time.Millisecond
		spec.MaxAttempts = 1
	})
	response := fixture.request(validBody("safe-chat", "terminal timeout"))
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	if record.ErrorClass != "upstream_timeout" || record.AttemptCount != 1 {
		t.Fatalf("request=%#v", record)
	}
}

func TestGatewayRetriesRetryableStatusesAndHonoursRetryAfter(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "0")
					http.Error(w, "provider detail", status)
					return
				}
				_, _ = io.WriteString(w, successfulResponse)
			}, nil)
			response := fixture.request(validBody("safe-chat", "retry status"))
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
			}
			record := requestRecord(t, fixture, response)
			if calls.Load() != 2 || record.AttemptCount != 2 {
				t.Fatalf("calls=%d attempts=%d", calls.Load(), record.AttemptCount)
			}
		})
	}
}

func TestGatewayTerminalErrorsAreNormalisedAndSafe(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":"provider-secret-value private-prompt http://internal-target"}`)
	}, nil)
	response := fixture.request(validBody("safe-chat", "private-prompt"))
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "0" {
		t.Fatalf("status=%d headers=%v", response.Code, response.Header())
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", calls.Load())
	}
	for label, forbidden := range map[string]string{"provider credential": "provider-secret-value", "prompt content": "private-prompt", "internal target": "internal-target", "upstream URL": fixture.server.URL} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("safe error leaked %s", label)
		}
	}
	record := requestRecord(t, fixture, response)
	if record.Status != "failed" || record.ErrorClass != "upstream_rate_limited" || record.AttemptCount != 2 {
		t.Fatalf("request = %#v", record)
	}
	routes, err := fixture.store.ListRoutes(context.Background(), principalFor(t, fixture.store, fixture.key))
	if err != nil || len(routes) != 1 || routes[0].Target.HealthStatus != "degraded" || routes[0].Target.LastSuccessAt != nil {
		t.Fatal("target-evidence failure did not leave the target degraded")
	}
}

func TestGatewayDoesNotRetryOrdinaryProviderErrors(t *testing.T) {
	for _, tc := range []struct {
		provider, client int
		class            string
	}{
		{http.StatusBadRequest, http.StatusBadRequest, "upstream_rejected"},
		{http.StatusUnauthorized, http.StatusBadGateway, "target_configuration"},
		{http.StatusForbidden, http.StatusBadGateway, "target_configuration"},
		{http.StatusNotFound, http.StatusBadGateway, "target_configuration"},
		{http.StatusInternalServerError, http.StatusBadGateway, "upstream_error"},
	} {
		t.Run(http.StatusText(tc.provider), func(t *testing.T) {
			var calls atomic.Int32
			fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				http.Error(w, "provider-secret-value ordinary-error-private-prompt http://internal-target", tc.provider)
			}, nil)
			const prompt = "ordinary-error-private-prompt"
			response := fixture.request(validBody("safe-chat", prompt))
			if response.Code != tc.client || calls.Load() != 1 {
				t.Fatalf("client=%d calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
			}
			for _, forbidden := range []string{"provider-secret-value", prompt, "internal-target", fixture.server.URL, fixture.key} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatal("ordinary upstream error exposed provider, request, or routing material")
				}
			}
			record := requestRecord(t, fixture, response)
			if record.ErrorClass != tc.class || record.AttemptCount != 1 {
				t.Fatalf("request=%#v", record)
			}
			if attempts := fixture.store.AttemptsForRequest(record.ID); len(attempts) != 1 || attempts[0].ErrorClass != tc.class {
				t.Fatal("ordinary upstream error did not reconcile to exactly one provider attempt")
			}
		})
	}
}

func TestGatewayPreservesUnknownAndPartialUsage(t *testing.T) {
	responses := []struct {
		name, body, finality string
		input, output        *int64
	}{
		{"missing", `{"id":"gen-1","model":"provider/model","choices":[{}]}`, "unknown", nil, nil},
		{"partial", `{"id":"gen-2","model":"provider/model","choices":[{}],"usage":{"prompt_tokens":9}}`, "partial", int64Pointer(9), nil},
	}
	for _, tc := range responses {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.body) }, nil)
			response := fixture.request(validBody("safe-chat", "usage"))
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
			}
			record := requestRecord(t, fixture, response)
			if record.UsageFinality != tc.finality || !sameInt(record.Usage.InputTokens, tc.input) || !sameInt(record.Usage.OutputTokens, tc.output) {
				t.Fatalf("usage=%#v finality=%s", record.Usage, record.UsageFinality)
			}
		})
	}
}

func TestGatewayRejectsUnsafeOrUnsupportedRequestFields(t *testing.T) {
	var calls atomic.Int32
	var forwardedOverride string
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		forwardedOverride = r.Header.Get("X-Upstream-URL")
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	tests := []struct {
		name string
		body string
	}{
		{"raw upstream URL", `{"model":"safe-chat","messages":[{"role":"user","content":"x"}],"upstream_url":"https://attacker.test"}`},
		{"message tenant override", `{"model":"safe-chat","messages":[{"role":"user","content":"x","tenant_id":"other"}]}`},
		{"project override", `{"model":"safe-chat","project_id":"other","messages":[{"role":"user","content":"x"}]}`},
		{"environment override", `{"model":"safe-chat","environment_id":"other","messages":[{"role":"user","content":"x"}]}`},
		{"unsupported role", `{"model":"safe-chat","messages":[{"role":"tool","content":"x"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := fixture.request(test.body)
			if response.Code != http.StatusBadRequest {
				t.Errorf("unsafe request status=%d length=%d", response.Code, response.Body.Len())
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("unsafe requests reached upstream %d times", calls.Load())
	}

	queryRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?upstream_url=https://attacker.invalid", strings.NewReader(validBody("safe-chat", "query override")))
	queryRequest.Header.Set("Content-Type", "application/json")
	queryRequest.Header.Set("Authorization", "Bearer "+fixture.key)
	queryResponse := httptest.NewRecorder()
	fixture.handler.ServeHTTP(queryResponse, queryRequest)
	if queryResponse.Code != http.StatusBadRequest || calls.Load() != 0 {
		t.Fatalf("query override status=%d calls=%d length=%d", queryResponse.Code, calls.Load(), queryResponse.Body.Len())
	}

	headerRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "header override")))
	headerRequest.Header.Set("Content-Type", "application/json")
	headerRequest.Header.Set("Authorization", "Bearer "+fixture.key)
	headerRequest.Header.Set("X-Upstream-URL", "https://attacker.invalid")
	headerResponse := httptest.NewRecorder()
	fixture.handler.ServeHTTP(headerResponse, headerRequest)
	if headerResponse.Code != http.StatusOK || calls.Load() != 1 || forwardedOverride != "" {
		t.Fatalf("header override status=%d calls=%d forwarded=%t", headerResponse.Code, calls.Load(), forwardedOverride != "")
	}
}

func TestGatewayAuthenticationScopeRevocationAndUnavailableTarget(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)

	unauthenticated := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "x")))
	unauthenticated.Header.Set("Content-Type", "application/json")
	unauthenticatedResult := httptest.NewRecorder()
	fixture.handler.ServeHTTP(unauthenticatedResult, unauthenticated)
	if unauthenticatedResult.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", unauthenticatedResult.Code)
	}

	if err := fixture.store.RevokeKey(context.Background(), fixture.result.KeyPrefix); err != nil {
		t.Fatal(err)
	}
	revoked := fixture.request(validBody("safe-chat", "x"))
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked status=%d", revoked.Code)
	}

	usageOnly := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}, func(spec *platform.ProvisionSpec) {
		spec.TargetName = "usage-only"
		spec.Scopes = []string{platform.ScopeUsageRead}
	})
	forbidden := usageOnly.request(validBody("safe-chat", "x"))
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("scope status=%d", forbidden.Code)
	}

	unavailable := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}, func(spec *platform.ProvisionSpec) { spec.TargetName = "unavailable" })
	if err := unavailable.store.UpdateTargetHealth(context.Background(), unavailable.result.TargetID, "unavailable", time.Now(), false); err != nil {
		t.Fatal(err)
	}
	failed := unavailable.request(validBody("safe-chat", "x"))
	if failed.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status=%d length=%d", failed.Code, failed.Body.Len())
	}
	if calls.Load() != 0 {
		t.Fatalf("blocked calls reached upstream: %d", calls.Load())
	}
}

func TestGatewayRejectsDuplicateBearerAuthorizationFields(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "duplicate auth")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+fixture.key)
	request.Header.Add("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate authorization status=%d length=%d", response.Code, response.Body.Len())
	}
	if response.Header().Get("WWW-Authenticate") != `Bearer realm="alzette"` {
		t.Fatal("duplicate authorization response did not issue the stable Bearer challenge")
	}
	if calls.Load() != 0 {
		t.Fatalf("duplicate authorization reached upstream calls=%d", calls.Load())
	}
}

func TestGatewayStableErrorsCarryRequestCorrelation(t *testing.T) {
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("rejected request reached upstream")
	}, nil)
	unknown, err := credentials.Generate()
	if err != nil {
		t.Fatal(err)
	}
	const promptCanary = "stable-error-private-prompt"
	tests := []struct {
		name, path, body, token, code, errorType string
		status                                   int
	}{
		{"missing credential", "/v1/chat/completions", validBody("safe-chat", promptCanary), "", "invalid_api_key", "authentication_error", http.StatusUnauthorized},
		{"unknown credential", "/v1/chat/completions", validBody("safe-chat", promptCanary), unknown.Token, "invalid_api_key", "authentication_error", http.StatusUnauthorized},
		{"query override", "/v1/chat/completions?target=elsewhere", validBody("safe-chat", promptCanary), fixture.key, "unsupported_query_parameter", "invalid_request_error", http.StatusBadRequest},
		{"malformed tools", "/v1/chat/completions", `{"model":"safe-chat","messages":[{"role":"user","content":"stable-error-private-prompt"}],"tools":[{"type":"function","function":{"name":"run","parameters":{"type":"string"}}}]}`, fixture.key, "invalid_tools", "invalid_request_error", http.StatusBadRequest},
		{"unbound model", "/v1/chat/completions", validBody("not-authorised", promptCanary), fixture.key, "model_not_authorised", "invalid_request_error", http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			fixture.handler.ServeHTTP(response, request)
			requestID := response.Header().Get("X-Alzette-Request-ID")
			if response.Code != test.status || requestID == "" || response.Header().Get("X-Request-ID") != requestID {
				t.Fatalf("status=%d request_id_present=%t correlated=%t length=%d", response.Code, requestID != "", response.Header().Get("X-Request-ID") == requestID, response.Body.Len())
			}
			var envelope api.ErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("error envelope decode failed; length=%d", response.Body.Len())
			}
			if envelope.RequestID != requestID || envelope.Error.Code != test.code || envelope.Error.Type != test.errorType || envelope.Error.Message == "" {
				t.Fatal("stable error envelope contract mismatch")
			}
			for _, forbidden := range []string{fixture.key, unknown.Token, promptCanary} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatal("stable error envelope exposed credential or request content")
				}
			}
		})
	}
}

func TestGatewayAuthorisedAbsentRouteIsOneBlockedLogicalRequestWithoutAttempt(t *testing.T) {
	var upstreamCalls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	const prompt = "absent-route-private-prompt"
	response := fixture.request(validBody("absent-route", prompt))
	if response.Code != http.StatusNotFound {
		t.Fatalf("absent route status=%d length=%d", response.Code, response.Body.Len())
	}
	var envelope api.ErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != "model_not_authorised" || envelope.RequestID == "" {
		t.Fatal("absent route did not return the stable model_not_authorised contract")
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("absent route reached upstream calls=%d", upstreamCalls.Load())
	}
	principal := principalFor(t, fixture.store, fixture.key)
	now := time.Now().UTC()
	page, err := fixture.store.ListInferenceRequests(context.Background(), principal, platform.UsageFilter{From: now.Add(-time.Minute), To: now.Add(time.Minute), Limit: 10})
	if err != nil || page.Truncated || len(page.Requests) != 1 {
		t.Fatal("absent route did not create exactly one scoped logical request")
	}
	record := page.Requests[0]
	if record.ID != envelope.RequestID || record.Status != "blocked" || record.ErrorClass != "model_not_authorised" || record.HTTPStatus != http.StatusNotFound || record.RouteID != "" || record.AttemptCount != 0 {
		t.Fatal("absent route logical request accounting did not fail closed")
	}
	if attempts := fixture.store.AttemptsForRequest(record.ID); len(attempts) != 0 {
		t.Fatal("absent route created a provider attempt")
	}
	for _, forbidden := range []string{prompt, fixture.key, fixture.server.URL, "provider-secret-value"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatal("absent route response exposed content, credentials, or target configuration")
		}
	}
}

func TestGatewayTenantIsolationAndServerControlledRoutes(t *testing.T) {
	var mu sync.Mutex
	models := make([]string, 0, 2)
	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		mu.Lock()
		models = append(models, request.Model)
		mu.Unlock()
		_, _ = io.WriteString(w, successfulResponse)
	}))
	defer upstreamA.Close()
	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		mu.Lock()
		models = append(models, request.Model)
		mu.Unlock()
		_, _ = io.WriteString(w, successfulResponse)
	}))
	defer upstreamB.Close()
	store := memory.New()
	base := platform.ProvisionSpec{ProjectName: "App", ProjectSlug: "app", EnvironmentName: "Production", EnvironmentSlug: "production", ModelAlias: "safe-chat", ModelVersion: "v1", ExecutionClass: "external_pilot", CapacityMode: "shared", SecretRef: "TARGET_KEY", TargetTimeout: time.Second, MaxAttempts: 1, ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite}}
	a := base
	a.OrganisationName = "A"
	a.OrganisationSlug = "tenant-a"
	a.TargetName = "target-a"
	a.TargetBaseURL = upstreamA.URL + "/v1"
	a.ProviderModel = "provider/model-a"
	b := base
	b.OrganisationName = "B"
	b.OrganisationSlug = "tenant-b"
	b.TargetName = "target-b"
	b.TargetBaseURL = upstreamB.URL + "/v1"
	b.ProviderModel = "provider/model-b"
	resultA, err := store.Provision(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := store.Provision(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := New(Config{Store: store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-key", true }})
	call := func(key, alias string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody(alias, "isolation")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	if res := call(resultA.APIKey, "safe-chat"); res.Code != http.StatusOK {
		t.Fatalf("tenant A status=%d", res.Code)
	}
	if res := call(resultB.APIKey, "safe-chat"); res.Code != http.StatusOK {
		t.Fatalf("tenant B status=%d", res.Code)
	}
	if res := call(resultA.APIKey, "tenant-b-secret-alias"); res.Code != http.StatusNotFound {
		t.Fatalf("wrong alias status=%d", res.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(models) != 2 || models[0] != "provider/model-a" || models[1] != "provider/model-b" {
		t.Fatalf("executed models=%v", models)
	}
}

func TestGatewayProjectEnvironmentScopesRequireExplicitSharedBindings(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_, _ = io.WriteString(w, successfulResponse)
	}))
	t.Cleanup(upstream.Close)
	store := memory.New()
	base := platform.ProvisionSpec{
		OrganisationName: "One Tenant", OrganisationSlug: "one-tenant",
		ModelVersion: "v1", TargetName: "explicit-shared-target", ExecutionClass: "private_compatible", CapacityMode: "shared",
		TargetBaseURL: upstream.URL + "/v1", ProviderModel: "provider/shared-model", SecretRef: "SHARED_TARGET_KEY",
		TargetTimeout: time.Second, MaxAttempts: 1, ServiceAccount: "application", Scopes: []string{platform.ScopeInferenceWrite},
	}
	projectA := base
	projectA.ProjectName, projectA.ProjectSlug = "Application A", "application-a"
	projectA.EnvironmentName, projectA.EnvironmentSlug = "Production", "production"
	projectA.ModelAlias = "project-a-chat"
	projectB := base
	projectB.ProjectName, projectB.ProjectSlug = "Application B", "application-b"
	projectB.EnvironmentName, projectB.EnvironmentSlug = "Staging", "staging"
	projectB.ModelAlias = "project-b-chat"
	resultA, err := store.Provision(context.Background(), projectA)
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := store.Provision(context.Background(), projectB)
	if err != nil {
		t.Fatal(err)
	}
	if resultA.ProjectID == resultB.ProjectID || resultA.EnvironmentID == resultB.EnvironmentID || resultA.RouteID == resultB.RouteID || resultA.TargetID != resultB.TargetID {
		t.Fatal("shared target did not retain distinct explicit project/environment bindings")
	}
	handler, err := New(Config{Store: store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-key", true }})
	if err != nil {
		t.Fatal(err)
	}
	call := func(key, alias string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody(alias, "scoped binding")))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+key)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if response := call(resultA.APIKey, projectA.ModelAlias); response.Code != http.StatusOK {
		t.Fatalf("project A status=%d length=%d", response.Code, response.Body.Len())
	}
	if response := call(resultB.APIKey, projectB.ModelAlias); response.Code != http.StatusOK {
		t.Fatalf("project B status=%d length=%d", response.Code, response.Body.Len())
	}
	if response := call(resultA.APIKey, projectB.ModelAlias); response.Code != http.StatusNotFound {
		t.Fatalf("project A cross-binding status=%d length=%d", response.Code, response.Body.Len())
	}
	if response := call(resultB.APIKey, projectA.ModelAlias); response.Code != http.StatusNotFound {
		t.Fatalf("project B cross-binding status=%d length=%d", response.Code, response.Body.Len())
	}
	if upstreamCalls.Load() != 2 {
		t.Fatalf("shared target calls=%d, want 2 authorised calls", upstreamCalls.Load())
	}
}

func TestGatewayDedicatedTargetCannotBeBoundToSecondTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, successfulResponse) }))
	defer server.Close()
	store := memory.New()
	base := platform.ProvisionSpec{ProjectName: "App", ProjectSlug: "app", EnvironmentName: "Prod", EnvironmentSlug: "prod", ModelAlias: "chat", ModelVersion: "v1", TargetName: "exclusive", ExecutionClass: "private_compatible", CapacityMode: "dedicated", TargetBaseURL: server.URL + "/v1", ProviderModel: "model", SecretRef: "TARGET_KEY", TargetTimeout: time.Second, MaxAttempts: 1, ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite}}
	base.CapacityEvidenceRef = "operator-test:evidence"
	a := base
	a.OrganisationName = "A"
	a.OrganisationSlug = "tenant-a"
	resultA, err := store.Provision(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(Config{Store: store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-key", true }})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody(a.ModelAlias, "dedicated pilot proof")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+resultA.APIKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("dedicated tenant call status=%d length=%d", response.Code, response.Body.Len())
	}
	b := base
	b.OrganisationName = "B"
	b.OrganisationSlug = "tenant-b"
	if _, err := store.Provision(context.Background(), b); err == nil {
		t.Fatal("dedicated target was rebound to a second tenant")
	}
}

func TestGatewayContractLimitsAndMalformedUpstream(t *testing.T) {
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{not-json`) }, nil)
	malformed := fixture.request(validBody("safe-chat", "x"))
	if malformed.Code != http.StatusBadGateway {
		t.Fatalf("malformed upstream status=%d", malformed.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	result := httptest.NewRecorder()
	fixture.handler.ServeHTTP(result, request)
	if result.Code != http.StatusMethodNotAllowed || result.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("method contract status=%d allow=%q", result.Code, result.Header().Get("Allow"))
	}

	wrongType := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	wrongType.Header.Set("Authorization", "Bearer "+fixture.key)
	wrongResult := httptest.NewRecorder()
	fixture.handler.ServeHTTP(wrongResult, wrongType)
	if wrongResult.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content type status=%d", wrongResult.Code)
	}

	smallGateway, _ := New(Config{Store: fixture.store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-secret", true }, MaxRequestBytes: 64})
	large := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", strings.Repeat("x", 200))))
	large.Header.Set("Content-Type", "application/json")
	large.Header.Set("Authorization", "Bearer "+fixture.key)
	largeResult := httptest.NewRecorder()
	smallGateway.ServeHTTP(largeResult, large)
	if largeResult.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large request status=%d length=%d", largeResult.Code, largeResult.Body.Len())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func TestPlatformUsageTreatsEmptyBifrostEvidenceAsUnknown(t *testing.T) {
	usage, finality := platformUsage(&inference.Usage{}, "partial")
	if finality != "unknown" || usage.Normalization != "" {
		t.Fatalf("empty usage=%#v finality=%q", usage, finality)
	}
}

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestGatewayRetriesDisconnectedUpstreamAndBoundsResponses(t *testing.T) {
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, successfulResponse)
	}, nil)
	var calls atomic.Int32
	disconnected, _ := New(Config{
		Store:                fixture.store,
		AllowInsecureTargets: true,
		legacyHTTPForTests:   true,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, io.ErrUnexpectedEOF
		})},
		SecretLookup:   func(string) (string, bool) { return "provider-secret", true },
		RetryBaseDelay: time.Millisecond,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "disconnect")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+fixture.key)
	disconnectedResult := httptest.NewRecorder()
	disconnected.ServeHTTP(disconnectedResult, req)
	if disconnectedResult.Code != http.StatusBadGateway || calls.Load() != 2 {
		t.Fatalf("disconnect status=%d calls=%d", disconnectedResult.Code, calls.Load())
	}

	bounded, _ := New(Config{Store: fixture.store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-secret", true }, MaxResponseBytes: 32})
	largeReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "large response")))
	largeReq.Header.Set("Content-Type", "application/json")
	largeReq.Header.Set("Authorization", "Bearer "+fixture.key)
	largeResult := httptest.NewRecorder()
	bounded.ServeHTTP(largeResult, largeReq)
	if largeResult.Code != http.StatusBadGateway {
		t.Fatalf("large response status=%d length=%d", largeResult.Code, largeResult.Body.Len())
	}
}

func TestGatewayMissingProviderSecretFailsBeforeAttempt(t *testing.T) {
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request reached upstream without a provider secret")
	}, nil)
	handler, _ := New(Config{Store: fixture.store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "", false }})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "secret")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	if record.AttemptCount != 0 || record.ErrorClass != "target_unavailable" {
		t.Fatalf("request=%#v", record)
	}
}

func TestGatewayCompletionLedgerFailuresNeverReturnProviderOutput(t *testing.T) {
	tests := []struct {
		name                  string
		failAttemptCompletion bool
		failRequestCompletion bool
		wantAttemptStatus     string
	}{
		{"attempt completion", true, true, "in_progress"},
		{"request completion", false, true, "succeeded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, successfulResponse)
			}, nil)
			faultedStore := completionFailureStore{
				Store:                 fixture.store,
				failAttemptCompletion: test.failAttemptCompletion,
				failRequestCompletion: test.failRequestCompletion,
			}
			handler, err := New(Config{
				Store:                faultedStore,
				AllowInsecureTargets: true,
				SecretLookup:         func(string) (string, bool) { return "provider-secret-value", true },
				RetryBaseDelay:       time.Millisecond,
			})
			if err != nil {
				t.Fatal(err)
			}
			const prompt = "private-ledger-failure-prompt"
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", prompt)))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+fixture.key)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusServiceUnavailable || calls.Load() != 1 {
				t.Fatalf("ledger failure status=%d calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
			}
			var envelope api.ErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != "ledger_unavailable" || envelope.RequestID == "" {
				t.Fatal("ledger completion failure did not return the stable safe error contract")
			}
			for _, forbidden := range []string{"hello", prompt, "provider-secret-value", fixture.server.URL, fixture.key} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatal("ledger completion failure exposed provider output, content, or configuration")
				}
			}
			record, err := fixture.store.GetInferenceRequest(context.Background(), principalFor(t, fixture.store, fixture.key), envelope.RequestID)
			if err != nil || record.Status != "in_progress" || record.AttemptCount != 1 {
				t.Fatal("injected ledger failure did not leave the expected reconcilable logical row")
			}
			attempts := fixture.store.AttemptsForRequest(record.ID)
			if len(attempts) != 1 || attempts[0].Status != test.wantAttemptStatus {
				t.Fatal("injected ledger failure did not leave the expected provider-attempt state")
			}
			routes, err := fixture.store.ListRoutes(context.Background(), principalFor(t, fixture.store, fixture.key))
			if err != nil || len(routes) != 1 || routes[0].Target.HealthStatus != "unknown" {
				t.Fatal("ledger failure incorrectly changed target health")
			}
		})
	}
}

func TestGatewayRejectsInsecureStoredTargetUnlessExplicitlyEnabled(t *testing.T) {
	var calls atomic.Int32
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}, nil)
	strict, _ := New(Config{Store: fixture.store, SecretLookup: func(string) (string, bool) { return "provider-secret", true }})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "strict target")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	strict.ServeHTTP(response, req)
	if response.Code != http.StatusBadGateway || calls.Load() != 0 {
		t.Fatalf("status=%d upstream calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
	}
	if record := requestRecord(t, fixture, response); record.AttemptCount != 0 || record.ErrorClass != "target_configuration" {
		t.Fatalf("request=%#v", record)
	}
}

type blockingTransport struct{ started chan struct{} }

func (transport blockingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	close(transport.started)
	<-request.Context().Done()
	return nil, request.Context().Err()
}

type contextCancelledBody struct {
	context context.Context
	started chan struct{}
	once    sync.Once
}

func (body *contextCancelledBody) Read([]byte) (int, error) {
	body.once.Do(func() { close(body.started) })
	<-body.context.Done()
	return 0, body.context.Err()
}

func (*contextCancelledBody) Close() error { return nil }

type truncatedResponseBody struct {
	contents []byte
	read     bool
}

func (body *truncatedResponseBody) Read(destination []byte) (int, error) {
	if body.read {
		return 0, io.EOF
	}
	body.read = true
	return copy(destination, body.contents), io.ErrUnexpectedEOF
}

func (*truncatedResponseBody) Close() error { return nil }

func TestGatewayCancellationIsNotRetried(t *testing.T) {
	fixture := newFixture(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	started := make(chan struct{})
	handler, _ := New(Config{Store: fixture.store, AllowInsecureTargets: true, legacyHTTPForTests: true, HTTPClient: &http.Client{Transport: blockingTransport{started: started}}, SecretLookup: func(string) (string, bool) { return "provider-secret", true }})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "cancel"))).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+fixture.key)
	res := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { handler.ServeHTTP(res, req); close(done) }()
	<-started
	cancel()
	<-done
	if res.Code != 499 {
		t.Fatalf("cancel status=%d length=%d", res.Code, res.Body.Len())
	}
	record := requestRecord(t, fixture, res)
	if record.Status != "cancelled" || record.AttemptCount != 1 {
		t.Fatalf("request=%#v", record)
	}
}

func TestGatewayResponseBodyClientCancellationIsNotTargetFailure(t *testing.T) {
	fixture := newFixture(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("custom response transport unexpectedly reached the fixture server")
	}, nil)
	bodyStarted := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       &contextCancelledBody{context: request.Context(), started: bodyStarted},
			Request:    request,
		}, nil
	})}
	handler, err := New(Config{
		Store:                fixture.store,
		AllowInsecureTargets: true,
		legacyHTTPForTests:   true,
		HTTPClient:           client,
		SecretLookup:         func(string) (string, bool) { return "provider-secret", true },
		RetryBaseDelay:       time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "cancel while buffering"))).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	select {
	case <-bodyStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("gateway did not begin buffering the upstream response")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("gateway did not terminate after client cancellation")
	}
	if response.Code != 499 {
		t.Fatalf("response-body cancellation status=%d length=%d", response.Code, response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	if record.Status != "cancelled" || record.ErrorClass != "client_cancelled" || record.AttemptCount != 1 {
		t.Fatalf("request status=%s class=%s attempts=%d", record.Status, record.ErrorClass, record.AttemptCount)
	}
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if len(attempts) != 1 || attempts[0].Status != "cancelled" || attempts[0].ErrorClass != "client_cancelled" {
		t.Fatal("response-body cancellation attempt state did not reconcile")
	}
	routes, err := fixture.store.ListRoutes(context.Background(), principalFor(t, fixture.store, fixture.key))
	if err != nil || len(routes) != 1 {
		t.Fatal("read route after response-body cancellation")
	}
	if routes[0].Target.HealthStatus != "unknown" || routes[0].Target.LastHealthCheckAt != nil || routes[0].Target.LastSuccessAt != nil {
		t.Fatal("client cancellation changed target health observation")
	}
}

func TestGatewayResponseBodyTargetDeadlineRetriesBeforeOutput(t *testing.T) {
	fixture := newFixture(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("custom response transport unexpectedly reached the fixture server")
	}, func(spec *platform.ProvisionSpec) { spec.TargetTimeout = 100 * time.Millisecond })
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       &contextCancelledBody{context: request.Context(), started: make(chan struct{})},
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(successfulResponse)),
			Request:    request,
		}, nil
	})}
	handler, err := New(Config{
		Store:                fixture.store,
		AllowInsecureTargets: true,
		legacyHTTPForTests:   true,
		HTTPClient:           client,
		SecretLookup:         func(string) (string, bool) { return "provider-secret", true },
		RetryBaseDelay:       time.Millisecond,
		MaxRetryDelay:        time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", "target deadline while buffering")))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || calls.Load() != 2 {
		t.Fatalf("body deadline status=%d calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
	}
	record := requestRecord(t, fixture, response)
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if record.Status != "succeeded" || record.AttemptCount != 2 || len(attempts) != 2 || attempts[0].ErrorClass != "upstream_timeout" || attempts[1].Status != "succeeded" {
		t.Fatal("body deadline retry accounting did not reconcile")
	}
}

func TestGatewayTruncatedResponseBodyFailsClosedWithoutRetry(t *testing.T) {
	const partialProviderBody = `{"choices":[{"message":{"content":"private-partial-output"}}]`
	fixture := newFixture(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("custom response transport unexpectedly reached the fixture server")
	}, nil)
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       &truncatedResponseBody{contents: []byte(partialProviderBody)},
			Request:    request,
		}, nil
	})}
	handler, err := New(Config{
		Store:                fixture.store,
		AllowInsecureTargets: true,
		legacyHTTPForTests:   true,
		HTTPClient:           client,
		SecretLookup:         func(string) (string, bool) { return "provider-secret", true },
		RetryBaseDelay:       time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	const prompt = "private-truncated-request"
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validBody("safe-chat", prompt)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+fixture.key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || calls.Load() != 1 {
		t.Fatalf("truncated response status=%d calls=%d length=%d", response.Code, calls.Load(), response.Body.Len())
	}
	for _, forbidden := range []string{"private-partial-output", prompt, "provider-secret", fixture.server.URL} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatal("truncated response failure exposed buffered content or configuration")
		}
	}
	record := requestRecord(t, fixture, response)
	attempts := fixture.store.AttemptsForRequest(record.ID)
	if record.Status != "failed" || record.ErrorClass != "invalid_upstream_response" || record.AttemptCount != 1 || len(attempts) != 1 || attempts[0].ErrorClass != "invalid_upstream_response" {
		t.Fatal("truncated response accounting did not reconcile")
	}
}

type liveSmokeConfig struct {
	providerModel  string
	providerSecret string
}

func approvedLiveSmokeConfig() (liveSmokeConfig, bool, error) {
	if os.Getenv("OPENROUTER_LIVE_TEST") != "1" || os.Getenv("ALZETTE_EXTERNAL_SMOKE_APPROVED") != "1" {
		return liveSmokeConfig{}, false, nil
	}
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY_FILE")) == "" {
		return liveSmokeConfig{}, true, errors.New("approved live smoke requires OPENROUTER_API_KEY_FILE")
	}
	providerSecret, ok := secrets.Lookup("OPENROUTER_API_KEY")
	if !ok {
		return liveSmokeConfig{}, true, errors.New("approved live smoke provider key file is unavailable")
	}
	providerModel := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if providerModel == "" {
		return liveSmokeConfig{}, true, errors.New("approved live smoke provider model is required")
	}
	return liveSmokeConfig{providerModel: providerModel, providerSecret: providerSecret}, true, nil
}

func TestApprovedLiveSmokeGateRequiresFileAndDualOptIn(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "unapproved-ambient-provider-key")
	t.Setenv("OPENROUTER_API_KEY_FILE", "")
	t.Setenv("OPENROUTER_MODEL", "provider/test-model")
	t.Setenv("OPENROUTER_LIVE_TEST", "1")
	t.Setenv("ALZETTE_EXTERNAL_SMOKE_APPROVED", "")
	if _, enabled, err := approvedLiveSmokeConfig(); enabled || err != nil {
		t.Fatal("single opt-in unexpectedly enabled the external smoke")
	}
	t.Setenv("ALZETTE_EXTERNAL_SMOKE_APPROVED", "1")
	if _, enabled, err := approvedLiveSmokeConfig(); !enabled || err == nil {
		t.Fatal("approved external smoke accepted an ambient-only provider key")
	}
	secretFile := filepath.Join(t.TempDir(), "approved-provider-key")
	if err := os.WriteFile(secretFile, []byte("approved-file-provider-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_API_KEY_FILE", secretFile)
	config, enabled, err := approvedLiveSmokeConfig()
	if err != nil || !enabled || config.providerModel != "provider/test-model" || config.providerSecret != "approved-file-provider-key" {
		t.Fatal("approved file-backed external smoke configuration was not resolved")
	}
}

func TestLiveOpenRouterSmoke(t *testing.T) {
	live, enabled, err := approvedLiveSmokeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Skip("requires OPENROUTER_LIVE_TEST=1 plus coordinator ALZETTE_EXTERNAL_SMOKE_APPROVED=1 and a file-backed provider key")
	}
	store := memory.New()
	result, err := store.Provision(context.Background(), platform.ProvisionSpec{OrganisationName: "Smoke", OrganisationSlug: "smoke", ProjectName: "Smoke", ProjectSlug: "smoke", EnvironmentName: "Test", EnvironmentSlug: "test", ModelAlias: "smoke-chat", ModelVersion: "live", TargetName: "openrouter-live", ExecutionClass: "external_pilot", CapacityMode: "shared", TargetBaseURL: "https://openrouter.ai/api/v1", ProviderModel: live.providerModel, SecretRef: "OPENROUTER_API_KEY", TargetTimeout: 30 * time.Second, MaxAttempts: 1, ServiceAccount: "smoke", Scopes: []string{platform.ScopeInferenceWrite}})
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := New(Config{Store: store})
	const livePrompt = "Reply with exactly: OK"
	maximumTokens := 8
	stream := false
	requestBody, err := json.Marshal(ChatRequest{Model: "smoke-chat", Messages: []Message{{Role: "user", Content: rawText(livePrompt)}}, Stream: &stream, MaxTokens: &maximumTokens})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+result.APIKey)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("live status=%d length=%d", res.Code, res.Body.Len())
	}
	requestID := res.Header().Get("X-Alzette-Request-ID")
	if requestID == "" || res.Header().Get("X-Request-ID") != requestID {
		t.Fatal("live response request correlation was inconsistent")
	}
	meta, err := parseProviderResponse(res.Body.Bytes())
	if err != nil || meta.model == "" || meta.finality != "final" || meta.usage.InputTokens == nil || meta.usage.OutputTokens == nil {
		t.Fatal("live response did not satisfy the bounded compatible response and usage contract")
	}
	principal := principalFor(t, store, result.APIKey)
	now := time.Now().UTC()
	page, err := store.ListInferenceRequests(context.Background(), principal, platform.UsageFilter{From: now.Add(-time.Minute), To: now.Add(time.Minute), Limit: 10})
	if err != nil || page.Truncated || len(page.Requests) != 1 {
		t.Fatal("live logical request ledger did not reconcile")
	}
	record, err := store.GetInferenceRequest(context.Background(), principal, requestID)
	if err != nil || record.Status != "succeeded" || record.AttemptCount != 1 || record.UsageFinality != "final" || record.ExecutedModel != meta.model {
		t.Fatal("live logical request and provider attempt accounting did not reconcile")
	}
	attempts := store.AttemptsForRequest(requestID)
	if len(attempts) != 1 || attempts[0].Status != "succeeded" {
		t.Fatal("live provider attempt ledger did not reconcile")
	}
	route, err := store.ResolveRoute(context.Background(), principal, "smoke-chat")
	if err != nil || route.Target.ExecutionClass != "external_pilot" || route.Target.CapacityMode != "shared" {
		t.Fatal("live external/shared target labels did not match the approved route")
	}
	metadata, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{livePrompt, live.providerSecret, result.APIKey, "openrouter.ai"} {
		if bytes.Contains(metadata, []byte(forbidden)) {
			t.Fatal("live ledger metadata exposed content, a credential, or target URL")
		}
	}
}

type liveAgentGatewayConfig struct {
	endpoint, apiKey, modelAlias string
}

func approvedLiveAgentGatewayConfig() (liveAgentGatewayConfig, bool, error) {
	if os.Getenv("ALZETTE_PI_LIVE_TEST") != "1" || os.Getenv("ALZETTE_EXTERNAL_SMOKE_APPROVED") != "1" {
		return liveAgentGatewayConfig{}, false, nil
	}
	keyFile := strings.TrimSpace(os.Getenv("ALZETTE_CLIENT_API_KEY_FILE"))
	if keyFile == "" {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke requires ALZETTE_CLIENT_API_KEY_FILE")
	}
	info, err := os.Stat(keyFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke key file must be a readable mode-0600 regular file")
	}
	apiKey, ok := secrets.Lookup("ALZETTE_CLIENT_API_KEY")
	if !ok || apiKey == "" {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke client key file is unavailable")
	}
	base, err := url.Parse(strings.TrimSpace(os.Getenv("ALZETTE_LIVE_GATEWAY_URL")))
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke gateway URL is invalid")
	}
	if base.Scheme == "http" {
		if os.Getenv("ALZETTE_INSECURE_LAN_SMOKE_APPROVED") != "1" {
			return liveAgentGatewayConfig{}, true, errors.New("HTTP Pi live smoke requires explicit insecure-LAN approval")
		}
	} else if base.Scheme != "https" {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke gateway URL must use HTTP or HTTPS")
	}
	path := strings.TrimRight(base.Path, "/")
	if path == "" {
		path = "/v1"
	}
	if path != "/v1" {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke gateway URL must end at /v1")
	}
	base.Path = path + "/chat/completions"
	modelAlias := strings.TrimSpace(os.Getenv("ALZETTE_CLIENT_MODEL_ALIAS"))
	if modelAlias == "" || len(modelAlias) > 128 {
		return liveAgentGatewayConfig{}, true, errors.New("approved Pi live smoke model alias is required")
	}
	return liveAgentGatewayConfig{endpoint: base.String(), apiKey: apiKey, modelAlias: modelAlias}, true, nil
}

func TestApprovedLiveAgentGatewayGateRequiresFileAndExplicitLANOptIn(t *testing.T) {
	t.Setenv("ALZETTE_PI_LIVE_TEST", "1")
	t.Setenv("ALZETTE_EXTERNAL_SMOKE_APPROVED", "")
	t.Setenv("ALZETTE_CLIENT_API_KEY", "unapproved-ambient-client-key")
	t.Setenv("ALZETTE_CLIENT_API_KEY_FILE", "")
	t.Setenv("ALZETTE_LIVE_GATEWAY_URL", "http://befree:19080/v1")
	t.Setenv("ALZETTE_CLIENT_MODEL_ALIAS", "alzette-chat")
	if _, enabled, err := approvedLiveAgentGatewayConfig(); enabled || err != nil {
		t.Fatal("single opt-in unexpectedly enabled the Pi gateway smoke")
	}
	t.Setenv("ALZETTE_EXTERNAL_SMOKE_APPROVED", "1")
	if _, enabled, err := approvedLiveAgentGatewayConfig(); !enabled || err == nil {
		t.Fatal("approved Pi gateway smoke accepted an ambient-only client key")
	}
	keyFile := filepath.Join(t.TempDir(), "approved-client-key")
	if err := os.WriteFile(keyFile, []byte("approved-file-client-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALZETTE_CLIENT_API_KEY_FILE", keyFile)
	if _, enabled, err := approvedLiveAgentGatewayConfig(); !enabled || err == nil {
		t.Fatal("HTTP Pi gateway smoke did not require insecure-LAN approval")
	}
	t.Setenv("ALZETTE_INSECURE_LAN_SMOKE_APPROVED", "1")
	config, enabled, err := approvedLiveAgentGatewayConfig()
	if err != nil || !enabled || config.endpoint != "http://befree:19080/v1/chat/completions" || config.modelAlias != "alzette-chat" || config.apiKey != "approved-file-client-key" {
		t.Fatal("approved file-backed Pi gateway smoke configuration was not resolved")
	}
}

func TestLivePiAgentGatewaySmoke(t *testing.T) {
	live, enabled, err := approvedLiveAgentGatewayConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Skip("requires explicit Pi/external approval, a mode-0600 client key file, and explicit insecure-LAN approval for HTTP")
	}
	requestPayload := map[string]interface{}{
		"model":      live.modelAlias,
		"messages":   []map[string]string{{"role": "user", "content": "Reply briefly; use the supplied tool only if necessary."}},
		"stream":     true,
		"max_tokens": 16,
		"tools": []map[string]interface{}{{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "lookup_constant",
				"description": "Return a deterministic test constant",
				"parameters":  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "additionalProperties": false},
				"strict":      false,
			},
		}},
	}
	body, err := json.Marshal(requestPayload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, live.endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+live.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 60 * time.Second}).Do(request)
	if err != nil {
		t.Fatal("approved Pi gateway smoke request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 4097))
		t.Fatalf("approved Pi gateway smoke status=%d length=%d", response.StatusCode, len(limited))
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	requestID := response.Header.Get("X-Alzette-Request-ID")
	if err != nil || mediaType != "text/event-stream" || requestID == "" {
		t.Fatal("approved Pi gateway smoke response headers were incompatible")
	}
	reader := bufio.NewReader(io.LimitReader(response.Body, defaultMaxResponseBytes+1))
	state := streamState{finality: "unknown"}
	total := int64(0)
	done := false
	for !done {
		frame, readErr := readSSEFrame(reader)
		total += int64(len(frame.raw))
		if total > defaultMaxResponseBytes {
			t.Fatal("approved Pi gateway smoke response exceeded the safe limit")
		}
		if readErr != nil {
			t.Fatal("approved Pi gateway smoke stream ended before completion")
		}
		if bytes.Contains(frame.raw, []byte(live.apiKey)) {
			t.Fatal("approved Pi gateway smoke response exposed the client key")
		}
		if !frame.hasData {
			continue
		}
		if frame.data == "[DONE]" {
			done = true
			continue
		}
		if err := state.consume(frame.data); err != nil {
			t.Fatal("approved Pi gateway smoke returned an incompatible SSE chunk")
		}
	}
	if !state.sawChoice || !state.sawFinish {
		t.Fatal("approved Pi gateway smoke lacked a terminal compatible choice")
	}
}

func int64Pointer(value int64) *int64 { return &value }
func sameInt(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
