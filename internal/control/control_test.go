package control

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"alzette/internal/api"
	"alzette/internal/credentials"
	"alzette/internal/gateway"
	"alzette/internal/platform"
	"alzette/internal/store/memory"
)

type controlFixture struct {
	store   *memory.Store
	control http.Handler
	gateway http.Handler
	keyA    string
	keyB    string
	prefixA string
	serverA *httptest.Server
	serverB *httptest.Server
	mu      sync.Mutex
	retries map[string]int
}

func newControlFixture(t *testing.T) *controlFixture {
	t.Helper()
	fixture := &controlFixture{store: memory.New(), retries: make(map[string]int)}
	upstream := func(w http.ResponseWriter, r *http.Request) {
		var request gateway.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode upstream: %v", err)
			return
		}
		var prompt string
		if len(request.Messages) == 0 || json.Unmarshal(request.Messages[0].Content, &prompt) != nil {
			t.Error("upstream request did not contain supported text content")
			return
		}
		if prompt == "retry" {
			fixture.mu.Lock()
			fixture.retries[prompt]++
			count := fixture.retries[prompt]
			fixture.mu.Unlock()
			if count == 1 {
				w.Header().Set("Retry-After", "0")
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
		}
		if prompt == "fail" {
			http.Error(w, "provider failure", http.StatusInternalServerError)
			return
		}
		if prompt == "partial" {
			_, _ = io.WriteString(w, `{"id":"partial-id","model":"provider/model-a","choices":[{}],"usage":{"prompt_tokens":4}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"success-id","model":"provider/executed","choices":[{}],"usage":{"prompt_tokens":10,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":2}}}`)
	}
	fixture.serverA = httptest.NewServer(http.HandlerFunc(upstream))
	t.Cleanup(fixture.serverA.Close)
	fixture.serverB = httptest.NewServer(http.HandlerFunc(upstream))
	t.Cleanup(fixture.serverB.Close)
	base := platform.ProvisionSpec{ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Production", EnvironmentSlug: "production", ModelAlias: "safe-chat", ModelVersion: "2026-08", ExecutionClass: "external_pilot", CapacityMode: "shared", SecretRef: "TARGET_SECRET", TargetTimeout: time.Second, MaxAttempts: 2, ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead}}
	a := base
	a.OrganisationName = "Tenant A"
	a.OrganisationSlug = "tenant-a"
	a.TargetName = "target-a"
	a.TargetBaseURL = fixture.serverA.URL + "/v1"
	a.ProviderModel = "provider/model-a"
	b := base
	b.OrganisationName = "Tenant B"
	b.OrganisationSlug = "tenant-b"
	b.TargetName = "target-b"
	b.TargetBaseURL = fixture.serverB.URL + "/v1"
	b.ProviderModel = "provider/model-b"
	resultA, err := fixture.store.Provision(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := fixture.store.Provision(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	fixture.keyA, fixture.keyB, fixture.prefixA = resultA.APIKey, resultB.APIKey, resultA.KeyPrefix
	fixture.gateway, err = gateway.New(gateway.Config{Store: fixture.store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-secret", true }, RetryBaseDelay: time.Millisecond, MaxRetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	fixture.control, err = New(Config{Store: fixture.store})
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *controlFixture) infer(t *testing.T, key, prompt string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"model": "safe-chat", "messages": []map[string]string{{"role": "user", "content": prompt}}})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	res := httptest.NewRecorder()
	f.gateway.ServeHTTP(res, req)
	return res
}
func (f *controlFixture) get(path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res := httptest.NewRecorder()
	f.control.ServeHTTP(res, req)
	return res
}

func (f *controlFixture) getBasic(path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.SetBasicAuth("display-only", key)
	}
	res := httptest.NewRecorder()
	f.control.ServeHTTP(res, req)
	return res
}

func TestDashboardUsesLogicalLedgerAndIsTenantSafe(t *testing.T) {
	fixture := newControlFixture(t)
	for _, prompt := range []string{"full", "retry", "partial"} {
		if res := fixture.infer(t, fixture.keyA, prompt); res.Code != http.StatusOK {
			t.Fatalf("%s status=%d length=%d", prompt, res.Code, res.Body.Len())
		}
	}
	if res := fixture.infer(t, fixture.keyA, "fail"); res.Code != http.StatusBadGateway {
		t.Fatalf("fail status=%d", res.Code)
	}
	if res := fixture.infer(t, fixture.keyB, "full"); res.Code != http.StatusOK {
		t.Fatalf("tenant B status=%d", res.Code)
	}

	response := fixture.get("/api/v1/dashboard", fixture.keyA)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d length=%d", response.Code, response.Body.Len())
	}
	payload := response.Body.Bytes()
	var dashboard DashboardResponse
	if err := json.Unmarshal(payload, &dashboard); err != nil {
		t.Fatal(err)
	}
	if dashboard.Scope.OrganisationName != "Tenant A" || dashboard.Usage.Summary.LogicalRequests != 4 || dashboard.Usage.Summary.SuccessfulRequests != 3 || dashboard.Usage.Summary.FailedRequests != 1 {
		t.Fatalf("dashboard=%#v", dashboard)
	}
	if len(dashboard.Usage.RecentRequests) != 4 {
		t.Fatalf("recent=%d", len(dashboard.Usage.RecentRequests))
	}
	if dashboard.Usage.Summary.InputTokens.Value == nil || *dashboard.Usage.Summary.InputTokens.Value != 24 || dashboard.Usage.Summary.OutputTokens.Value == nil || *dashboard.Usage.Summary.OutputTokens.Value != 10 {
		t.Fatalf("tokens=%#v %#v", dashboard.Usage.Summary.InputTokens, dashboard.Usage.Summary.OutputTokens)
	}
	if dashboard.Usage.Summary.OutputTokens.Finality != "partial" {
		t.Fatalf("output finality=%q", dashboard.Usage.Summary.OutputTokens.Finality)
	}
	if len(dashboard.Routes) != 1 || dashboard.Routes[0].ExecutionLabel != "External pilot via OpenRouter" || dashboard.Routes[0].ServiceMode != "Shared pilot" || dashboard.Routes[0].Endpoint != "/v1/chat/completions" {
		t.Fatalf("routes=%#v", dashboard.Routes)
	}
	body := string(payload)
	for label, forbidden := range map[string]string{"first upstream URL": fixture.serverA.URL, "second upstream URL": fixture.serverB.URL, "provider credential": "provider-secret", "other tenant": "Tenant B", "target URL field": "base_url", "secret reference field": "secret_ref", "provider attempts field": "provider_attempts"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("dashboard leaked %s", label)
		}
	}
}

func TestUsageCannotOverrideAuthenticatedTenantScope(t *testing.T) {
	fixture := newControlFixture(t)
	a := fixture.infer(t, fixture.keyA, "full")
	b := fixture.infer(t, fixture.keyB, "full")
	if a.Code != http.StatusOK || b.Code != http.StatusOK {
		t.Fatal("fixture calls failed")
	}
	response := fixture.get("/api/v1/usage?organisation_id=tenant-b", fixture.keyA)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("scope override status=%d", response.Code)
	}
	response = fixture.get("/api/v1/usage", fixture.keyA)
	var usage UsageResponse
	if err := json.NewDecoder(response.Body).Decode(&usage); err != nil {
		t.Fatal(err)
	}
	if usage.Scope.OrganisationName != "Tenant A" || usage.Summary.LogicalRequests != 1 || len(usage.RecentRequests) != 1 || usage.RecentRequests[0].RequestID != a.Header().Get("X-Alzette-Request-ID") {
		t.Fatalf("usage=%#v", usage)
	}
}

func TestRequestDetailFailsClosedAcrossTenants(t *testing.T) {
	fixture := newControlFixture(t)
	b := fixture.infer(t, fixture.keyB, "full")
	id := b.Header().Get("X-Alzette-Request-ID")
	if response := fixture.get("/api/v1/requests/"+id, fixture.keyA); response.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant detail status=%d length=%d", response.Code, response.Body.Len())
	}
	response := fixture.get("/api/v1/requests/"+id, fixture.keyB)
	if response.Code != http.StatusOK {
		t.Fatalf("own detail status=%d", response.Code)
	}
	var detail RequestView
	if err := json.NewDecoder(response.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.RequestID != id || detail.Project != "Application" || detail.ModelAlias != "safe-chat" {
		t.Fatalf("detail=%#v", detail)
	}
}

func TestControlAuthenticationScopeAndRevocation(t *testing.T) {
	fixture := newControlFixture(t)
	if response := fixture.get("/api/v1/usage", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", response.Code)
	}
	if err := fixture.store.RevokeKey(context.Background(), fixture.prefixA); err != nil {
		t.Fatal(err)
	}
	if response := fixture.get("/api/v1/usage", fixture.keyA); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked status=%d", response.Code)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	result, err := fixture.store.Provision(context.Background(), platform.ProvisionSpec{OrganisationName: "No Usage", OrganisationSlug: "no-usage", ProjectName: "App", ProjectSlug: "app", EnvironmentName: "Prod", EnvironmentSlug: "prod", ModelAlias: "chat-no-usage", ModelVersion: "v1", TargetName: "no-usage-target", ExecutionClass: "external_pilot", CapacityMode: "shared", TargetBaseURL: server.URL + "/v1", ProviderModel: "provider/model", SecretRef: "TARGET_SECRET", TargetTimeout: time.Second, MaxAttempts: 1, ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite}})
	if err != nil {
		t.Fatal(err)
	}
	if response := fixture.get("/api/v1/usage", result.APIKey); response.Code != http.StatusForbidden {
		t.Fatalf("scope status=%d", response.Code)
	}
}

func TestControlRejectsDuplicateBearerAuthorizationFields(t *testing.T) {
	fixture := newControlFixture(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	request.Header.Add("Authorization", "Bearer "+fixture.keyA)
	request.Header.Add("Authorization", "Bearer "+fixture.keyA)
	response := httptest.NewRecorder()
	fixture.control.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate authorization status=%d length=%d", response.Code, response.Body.Len())
	}
	if response.Header().Get("WWW-Authenticate") != `Bearer realm="alzette"` {
		t.Fatal("duplicate authorization response did not issue the stable Bearer challenge")
	}
}

func TestClientDashboardRejectsDuplicateBasicAuthorizationFields(t *testing.T) {
	fixture := newControlFixture(t)
	request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	request.SetBasicAuth("display-only", fixture.keyA)
	request.Header.Add("Authorization", request.Header.Get("Authorization"))
	response := httptest.NewRecorder()
	fixture.control.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("duplicate Basic authorization status=%d length=%d", response.Code, response.Body.Len())
	}
	if response.Header().Get("WWW-Authenticate") != `Basic realm="`+api.PortalRealm+`", charset="UTF-8"` {
		t.Fatal("duplicate Basic authorization response did not issue the stable portal challenge")
	}
	if strings.Contains(response.Body.String(), fixture.keyA) || strings.Contains(response.Body.String(), "Tenant A") || strings.Contains(response.Body.String(), clientDashboardSchema) {
		t.Fatal("duplicate Basic authorization response exposed credentials or tenant dashboard data")
	}
}

func TestUsageValidationAndUnknownTokens(t *testing.T) {
	fixture := newControlFixture(t)
	if res := fixture.infer(t, fixture.keyA, "partial"); res.Code != http.StatusOK {
		t.Fatalf("partial fixture status=%d length=%d", res.Code, res.Body.Len())
	}
	response := fixture.get("/api/v1/usage?model=safe-chat&limit=1", fixture.keyA)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	var usage UsageResponse
	if err := json.NewDecoder(response.Body).Decode(&usage); err != nil {
		t.Fatal(err)
	}
	if usage.Summary.InputTokens.Value == nil || *usage.Summary.InputTokens.Value != 4 || usage.Summary.OutputTokens.Value != nil || usage.Summary.OutputTokens.Finality != "unknown" {
		t.Fatalf("summary=%#v", usage.Summary)
	}
	for _, path := range []string{"/api/v1/usage?limit=101", "/api/v1/usage?from=bad", "/api/v1/usage?from=2026-01-01T00:00:00Z&to=2026-03-01T00:00:00Z"} {
		if result := fixture.get(path, fixture.keyA); result.Code != http.StatusBadRequest {
			t.Errorf("%s status=%d", path, result.Code)
		}
	}
}

func TestControlMethodAndSafe404Contract(t *testing.T) {
	fixture := newControlFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/usage", nil)
	req.Header.Set("Authorization", "Bearer "+fixture.keyA)
	res := httptest.NewRecorder()
	fixture.control.ServeHTTP(res, req)
	if res.Code != http.StatusMethodNotAllowed || res.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("method status=%d allow=%q", res.Code, res.Header().Get("Allow"))
	}
	if response := fixture.get("/api/v1/not-a-resource", fixture.keyA); response.Code != http.StatusNotFound {
		t.Fatalf("404 status=%d", response.Code)
	}
}

func TestClientDashboardExactContractReconcilesLogicalRequests(t *testing.T) {
	fixture := newControlFixture(t)
	for _, prompt := range []string{"full", "retry", "partial"} {
		if response := fixture.infer(t, fixture.keyA, prompt); response.Code != http.StatusOK {
			t.Fatalf("fixture inference %s status=%d length=%d", prompt, response.Code, response.Body.Len())
		}
	}
	if response := fixture.infer(t, fixture.keyA, "fail"); response.Code != http.StatusBadGateway {
		t.Fatalf("fixture failure status=%d length=%d", response.Code, response.Body.Len())
	}
	other := fixture.infer(t, fixture.keyB, "full")
	if other.Code != http.StatusOK {
		t.Fatalf("other tenant inference status=%d length=%d", other.Code, other.Body.Len())
	}

	response := fixture.getBasic("/api/dashboard", fixture.keyA)
	if response.Code != http.StatusOK {
		t.Fatalf("client dashboard status=%d length=%d", response.Code, response.Body.Len())
	}
	assertObjectKeys(t, "dashboard", response.Body.Bytes(), []string{"account", "breakdowns", "export", "period", "recent_requests", "route", "schema", "source", "trend", "usage"})
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal("client dashboard was not valid JSON")
	}
	assertObjectKeys(t, "account", raw["account"], []string{"initials", "name"})
	assertObjectKeys(t, "period", raw["period"], []string{"label", "options", "timezone"})
	assertObjectKeys(t, "source", raw["source"], []string{"as_of", "detail", "finality", "freshness", "kind", "label"})
	assertObjectKeys(t, "route", raw["route"], []string{"attention", "capacity_mode", "endpoint_path", "execution_class", "last_health_check_at", "last_success_at", "model_alias", "state", "status_detail"})
	assertObjectKeys(t, "usage", raw["usage"], []string{"allowance", "blocked_requests", "error_rate", "logical_requests", "p50_latency_ms", "p95_latency_ms", "success_rate", "successful_requests", "tokens"})
	assertObjectKeys(t, "trend", raw["trend"], []string{"points", "unit"})
	assertObjectKeys(t, "breakdowns", raw["breakdowns"], []string{"models", "projects"})
	assertObjectKeys(t, "export", raw["export"], []string{"available", "formats", "scope", "units"})
	var rawRoute, rawTrend, rawBreakdowns map[string]json.RawMessage
	if err := json.Unmarshal(raw["route"], &rawRoute); err != nil {
		t.Fatal("route was not a JSON object")
	}
	assertObjectKeys(t, "route attention", rawRoute["attention"], []string{"detail", "severity", "title"})
	if err := json.Unmarshal(raw["trend"], &rawTrend); err != nil {
		t.Fatal("trend was not a JSON object")
	}
	assertArrayItemKeys(t, "trend point", rawTrend["points"], []string{"label", "p95_latency_ms", "requests", "success_rate", "tokens"})
	if err := json.Unmarshal(raw["breakdowns"], &rawBreakdowns); err != nil {
		t.Fatal("breakdowns was not a JSON object")
	}
	assertArrayItemKeys(t, "project breakdown", rawBreakdowns["projects"], []string{"name", "requests", "share", "tokens"})
	assertArrayItemKeys(t, "model breakdown", rawBreakdowns["models"], []string{"alias", "executed_model", "requests", "share", "tokens"})
	assertArrayItemKeys(t, "recent request", raw["recent_requests"], []string{"error_class", "executed_model", "latency_ms", "model_alias", "occurred_at", "project", "request_id", "status", "tokens"})
	var rawUsage map[string]json.RawMessage
	if err := json.Unmarshal(raw["usage"], &rawUsage); err != nil {
		t.Fatal("usage was not a JSON object")
	}
	assertObjectKeys(t, "tokens", rawUsage["tokens"], []string{"cached", "input", "output", "reasoning", "total"})

	var dashboard ClientDashboardResponse
	if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
		t.Fatal("client dashboard did not match its typed contract")
	}
	if dashboard.Schema != clientDashboardSchema || dashboard.Account.Name != "Tenant A" || dashboard.Account.Initials != "TA" {
		t.Fatalf("identity contract mismatch: schema=%q account=%q initials=%q", dashboard.Schema, dashboard.Account.Name, dashboard.Account.Initials)
	}
	if dashboard.Route.ModelAlias == nil || *dashboard.Route.ModelAlias != "safe-chat" || dashboard.Route.State != "degraded" || dashboard.Route.ExecutionClass == nil || *dashboard.Route.ExecutionClass != "external_pilot" || dashboard.Route.CapacityMode == nil || *dashboard.Route.CapacityMode != "shared" {
		t.Fatalf("route contract mismatch: state=%q alias_present=%v execution_present=%v capacity_present=%v", dashboard.Route.State, dashboard.Route.ModelAlias != nil, dashboard.Route.ExecutionClass != nil, dashboard.Route.CapacityMode != nil)
	}
	if dashboard.Usage.LogicalRequests != 4 || dashboard.Usage.SuccessfulRequests != 3 || dashboard.Usage.SuccessRate == nil || *dashboard.Usage.SuccessRate != 75 || dashboard.Usage.ErrorRate == nil || *dashboard.Usage.ErrorRate != 25 {
		t.Fatalf("usage reconciliation mismatch: logical=%d successful=%d success_rate_present=%v error_rate_present=%v", dashboard.Usage.LogicalRequests, dashboard.Usage.SuccessfulRequests, dashboard.Usage.SuccessRate != nil, dashboard.Usage.ErrorRate != nil)
	}
	if dashboard.Usage.Tokens.Input == nil || *dashboard.Usage.Tokens.Input != 24 || dashboard.Usage.Tokens.Output != nil || dashboard.Usage.Tokens.Total != nil {
		t.Fatalf("unknown-token contract mismatch: input_present=%v output_present=%v total_present=%v", dashboard.Usage.Tokens.Input != nil, dashboard.Usage.Tokens.Output != nil, dashboard.Usage.Tokens.Total != nil)
	}
	if dashboard.Source.Finality != "partial" || !strings.Contains(dashboard.Source.Detail, "incomplete token usage") {
		t.Fatalf("partial finality was not visible: finality=%q detail_has_reason=%v", dashboard.Source.Finality, strings.Contains(dashboard.Source.Detail, "incomplete token usage"))
	}
	if !strings.Contains(dashboard.Source.Detail, "authenticated project/environment Application / Production") || !strings.Contains(dashboard.Source.Detail, "current route binding") || !strings.Contains(dashboard.Source.Detail, "not an active health probe") {
		t.Fatalf("source provenance was not explicit: scope_present=%v binding_present=%v probe_disclaimer_present=%v", strings.Contains(dashboard.Source.Detail, "authenticated project/environment Application / Production"), strings.Contains(dashboard.Source.Detail, "current route binding"), strings.Contains(dashboard.Source.Detail, "not an active health probe"))
	}
	if len(dashboard.Breakdowns.Projects) != 1 || dashboard.Breakdowns.Projects[0].Name != "Application / Production" {
		t.Fatalf("project/environment breakdown name mismatch: count=%d", len(dashboard.Breakdowns.Projects))
	}
	if !strings.Contains(dashboard.Route.StatusDetail, "not an active health probe") {
		t.Fatal("route status did not disclose observation provenance")
	}

	trendRequests := int64(0)
	for _, point := range dashboard.Trend.Points {
		trendRequests += point.Requests
	}
	projectRequests := int64(0)
	for _, item := range dashboard.Breakdowns.Projects {
		projectRequests += item.Requests
	}
	modelRequests := int64(0)
	for _, item := range dashboard.Breakdowns.Models {
		modelRequests += item.Requests
	}
	if trendRequests != dashboard.Usage.LogicalRequests || projectRequests != dashboard.Usage.LogicalRequests || modelRequests != dashboard.Usage.LogicalRequests || len(dashboard.RecentRequests) != 4 {
		t.Fatalf("aggregate mismatch: usage=%d trend=%d projects=%d models=%d recent=%d", dashboard.Usage.LogicalRequests, trendRequests, projectRequests, modelRequests, len(dashboard.RecentRequests))
	}
	attempts := 0
	for _, request := range dashboard.RecentRequests {
		attempts += len(fixture.store.AttemptsForRequest(request.RequestID))
	}
	if attempts != 5 || dashboard.Usage.LogicalRequests != 4 {
		t.Fatalf("logical/attempt accounting mismatch: logical=%d attempts=%d", dashboard.Usage.LogicalRequests, attempts)
	}
	if dashboard.Export.Available || len(dashboard.Export.Formats) != 0 {
		t.Fatalf("partial usage range was exportable: available=%v formats=%v", dashboard.Export.Available, dashboard.Export.Formats)
	}

	body := response.Body.String()
	for label, forbidden := range map[string]string{
		"API key": fixture.keyA, "key prefix": fixture.prefixA, "other tenant": "Tenant B",
		"first upstream URL": fixture.serverA.URL, "second upstream URL": fixture.serverB.URL,
		"provider credential": "provider-secret", "secret reference": "TARGET_SECRET",
		"provider attempts field": "provider_attempts", "target URL field": "base_url",
		"prompt content": "retry",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("client dashboard leaked %s", label)
		}
	}
	if strings.Contains(body, other.Header().Get("X-Alzette-Request-ID")) {
		t.Fatal("client dashboard leaked another tenant's request ID")
	}

	limited := fixture.getBasic("/api/dashboard?limit=1", fixture.keyA)
	if limited.Code != http.StatusOK {
		t.Fatalf("limited client dashboard status=%d length=%d", limited.Code, limited.Body.Len())
	}
	var limitedDashboard ClientDashboardResponse
	if err := json.NewDecoder(limited.Body).Decode(&limitedDashboard); err != nil {
		t.Fatal("limited client dashboard was invalid JSON")
	}
	if limitedDashboard.Usage.LogicalRequests != 4 || len(limitedDashboard.RecentRequests) != 1 || limitedDashboard.Export.Available || len(limitedDashboard.Export.Formats) != 0 {
		t.Fatalf("partial export contract mismatch: logical=%d recent=%d available=%v formats=%d", limitedDashboard.Usage.LogicalRequests, len(limitedDashboard.RecentRequests), limitedDashboard.Export.Available, len(limitedDashboard.Export.Formats))
	}
}

func TestClientDashboardBasicIsIsolatedFromBearerMachineAPIs(t *testing.T) {
	fixture := newControlFixture(t)
	if response := fixture.get("/api/v1/dashboard", fixture.keyA); response.Code != http.StatusOK {
		t.Fatalf("Bearer dashboard regression status=%d length=%d", response.Code, response.Body.Len())
	}
	basicMachine := fixture.getBasic("/api/v1/dashboard", fixture.keyA)
	if basicMachine.Code != http.StatusUnauthorized || basicMachine.Header().Get("WWW-Authenticate") != `Bearer realm="alzette"` {
		t.Fatalf("machine API accepted Basic or changed challenge: status=%d challenge=%q", basicMachine.Code, basicMachine.Header().Get("WWW-Authenticate"))
	}
	bearerCompat := fixture.get("/api/dashboard", fixture.keyA)
	if bearerCompat.Code != http.StatusUnauthorized || bearerCompat.Header().Get("WWW-Authenticate") != `Basic realm="`+api.PortalRealm+`", charset="UTF-8"` {
		t.Fatalf("compatibility API accepted Bearer or changed challenge: status=%d challenge=%q", bearerCompat.Code, bearerCompat.Header().Get("WWW-Authenticate"))
	}
	if response := fixture.get("/api/v1/usage", fixture.keyA); response.Code != http.StatusOK {
		t.Fatalf("Bearer usage regression status=%d length=%d", response.Code, response.Body.Len())
	}
}

func TestClientDashboardInvalidRevokedAndInsufficientBasicKeysFailClosed(t *testing.T) {
	fixture := newControlFixture(t)
	generated, err := credentials.Generate()
	if err != nil {
		t.Fatal(err)
	}
	for name, key := range map[string]string{"missing": "", "invalid": generated.Token} {
		t.Run(name, func(t *testing.T) {
			response := fixture.getBasic("/api/dashboard", key)
			if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != `Basic realm="`+api.PortalRealm+`", charset="UTF-8"` {
				t.Fatalf("status=%d challenge_present=%v", response.Code, response.Header().Get("WWW-Authenticate") != "")
			}
			if key != "" && strings.Contains(response.Body.String(), key) {
				t.Fatal("authentication failure response leaked the supplied key")
			}
		})
	}

	limited, err := fixture.store.Provision(context.Background(), platform.ProvisionSpec{
		OrganisationName: "Limited", OrganisationSlug: "limited", ProjectName: "App", ProjectSlug: "app", EnvironmentName: "Prod", EnvironmentSlug: "prod",
		ModelAlias: "limited-chat", ModelVersion: "v1", TargetName: "limited-target", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: "https://provider.example.invalid/api/v1", ProviderModel: "provider/model", SecretRef: "TARGET_SECRET", TargetTimeout: time.Second, MaxAttempts: 1,
		ServiceAccount: "portal", Scopes: []string{platform.ScopeUsageRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	insufficient := fixture.getBasic("/api/dashboard", limited.APIKey)
	if insufficient.Code != http.StatusForbidden {
		t.Fatalf("insufficient-scope status=%d", insufficient.Code)
	}
	if strings.Contains(insufficient.Body.String(), limited.APIKey) {
		t.Fatal("insufficient-scope response leaked the API key")
	}

	if err := fixture.store.RevokeKey(context.Background(), fixture.prefixA); err != nil {
		t.Fatal(err)
	}
	revoked := fixture.getBasic("/api/dashboard", fixture.keyA)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked-key status=%d", revoked.Code)
	}
	if strings.Contains(revoked.Body.String(), fixture.keyA) {
		t.Fatal("revoked-key response leaked the API key")
	}
}

func TestClientDashboardEmptyUnknownAndAmbiguousRouteStates(t *testing.T) {
	fixture := newControlFixture(t)
	empty := fixture.getBasic("/api/dashboard", fixture.keyA)
	if empty.Code != http.StatusOK {
		t.Fatalf("empty dashboard status=%d length=%d", empty.Code, empty.Body.Len())
	}
	var dashboard ClientDashboardResponse
	if err := json.NewDecoder(empty.Body).Decode(&dashboard); err != nil {
		t.Fatal("empty dashboard was invalid JSON")
	}
	if dashboard.Usage.LogicalRequests != 0 || dashboard.Usage.SuccessRate != nil || dashboard.Usage.ErrorRate != nil || dashboard.Usage.Tokens.Input != nil || dashboard.Usage.Tokens.Output != nil || dashboard.Usage.Tokens.Total != nil {
		t.Fatalf("empty usage was coerced: logical=%d success_rate=%v error_rate=%v input=%v output=%v total=%v", dashboard.Usage.LogicalRequests, dashboard.Usage.SuccessRate != nil, dashboard.Usage.ErrorRate != nil, dashboard.Usage.Tokens.Input != nil, dashboard.Usage.Tokens.Output != nil, dashboard.Usage.Tokens.Total != nil)
	}
	if len(dashboard.Trend.Points) != 0 || len(dashboard.Breakdowns.Projects) != 0 || len(dashboard.Breakdowns.Models) != 0 || len(dashboard.RecentRequests) != 0 || dashboard.Source.Finality != "final" || dashboard.Route.State != "unknown" || !dashboard.Export.Available {
		t.Fatalf("empty state mismatch: trend=%d projects=%d models=%d recent=%d finality=%q route=%q export=%v", len(dashboard.Trend.Points), len(dashboard.Breakdowns.Projects), len(dashboard.Breakdowns.Models), len(dashboard.RecentRequests), dashboard.Source.Finality, dashboard.Route.State, dashboard.Export.Available)
	}

	_, err := fixture.store.Provision(context.Background(), platform.ProvisionSpec{
		OrganisationName: "Tenant A", OrganisationSlug: "tenant-a", ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Production", EnvironmentSlug: "production",
		ModelAlias: "second-chat", ModelVersion: "v1", TargetName: "target-a-second", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: fixture.serverA.URL + "/v1", ProviderModel: "provider/model-second", SecretRef: "TARGET_SECRET", TargetTimeout: time.Second, MaxAttempts: 1,
		ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	ambiguous := fixture.getBasic("/api/dashboard", fixture.keyA)
	if err := json.NewDecoder(ambiguous.Body).Decode(&dashboard); err != nil {
		t.Fatal("ambiguous dashboard was invalid JSON")
	}
	if dashboard.Route.State != "unknown" || dashboard.Route.ModelAlias != nil || dashboard.Route.Attention.Title != "Select a model alias" {
		t.Fatalf("ambiguous route was silently selected: state=%q alias_present=%v attention=%q", dashboard.Route.State, dashboard.Route.ModelAlias != nil, dashboard.Route.Attention.Title)
	}
	selected := fixture.getBasic("/api/dashboard?model=safe-chat", fixture.keyA)
	if err := json.NewDecoder(selected.Body).Decode(&dashboard); err != nil {
		t.Fatal("selected dashboard was invalid JSON")
	}
	if dashboard.Route.ModelAlias == nil || *dashboard.Route.ModelAlias != "safe-chat" {
		t.Fatalf("model query did not select the exact route; alias_present=%v", dashboard.Route.ModelAlias != nil)
	}

	principal, err := fixture.store.Authenticate(context.Background(), credentials.Digest(fixture.keyA))
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	if err := fixture.store.CreateInferenceRequest(context.Background(), platform.RequestStart{ID: "req_in_progress_test", Principal: principal, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	inProgress := fixture.getBasic("/api/dashboard?model=safe-chat", fixture.keyA)
	if err := json.NewDecoder(inProgress.Body).Decode(&dashboard); err != nil {
		t.Fatal("in-progress dashboard was invalid JSON")
	}
	if dashboard.Source.Finality != "partial" || !strings.Contains(dashboard.Source.Detail, "still in progress") || dashboard.RecentRequests[0].Tokens != nil || dashboard.RecentRequests[0].LatencyMS != nil {
		t.Fatalf("in-progress finality mismatch: finality=%q visible_reason=%v token_present=%v latency_present=%v", dashboard.Source.Finality, strings.Contains(dashboard.Source.Detail, "still in progress"), dashboard.RecentRequests[0].Tokens != nil, dashboard.RecentRequests[0].LatencyMS != nil)
	}

	from, to := started.Add(-time.Hour), started.Add(time.Hour)
	future := fixture.getBasic("/api/dashboard?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339)+"&model=safe-chat", fixture.keyA)
	if err := json.NewDecoder(future.Body).Decode(&dashboard); err != nil {
		t.Fatal("future-range dashboard was invalid JSON")
	}
	if dashboard.Source.Finality != "partial" || !strings.Contains(dashboard.Source.Detail, "extends beyond the snapshot time") {
		t.Fatalf("future range finality mismatch: finality=%q visible_reason=%v", dashboard.Source.Finality, strings.Contains(dashboard.Source.Detail, "extends beyond the snapshot time"))
	}
}

type truncatedLedgerStore struct {
	platform.Store
}

func (s truncatedLedgerStore) ListInferenceRequests(context.Context, platform.Principal, platform.UsageFilter) (platform.RequestPage, error) {
	return platform.RequestPage{Requests: []platform.InferenceRequest{}, Truncated: true}, nil
}

func TestClientDashboardMakesTruncatedLedgerFinalityVisible(t *testing.T) {
	fixture := newControlFixture(t)
	handler, err := New(Config{Store: truncatedLedgerStore{Store: fixture.store}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	request.SetBasicAuth("display-only", fixture.keyA)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("truncated dashboard status=%d length=%d", response.Code, response.Body.Len())
	}
	var dashboard ClientDashboardResponse
	if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
		t.Fatal("truncated dashboard was invalid JSON")
	}
	if dashboard.Source.Finality != "partial" || !strings.Contains(dashboard.Source.Detail, "10,000-row read limit") || dashboard.Export.Available {
		t.Fatalf("truncation finality mismatch: finality=%q visible_reason=%v export=%v", dashboard.Source.Finality, strings.Contains(dashboard.Source.Detail, "10,000-row read limit"), dashboard.Export.Available)
	}
}

func TestSharedTargetRouteObservationsRemainTenantScoped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"shared-success","model":"provider/shared","choices":[{}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`)
	}))
	defer server.Close()
	store := memory.New()
	base := platform.ProvisionSpec{
		ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Production", EnvironmentSlug: "production",
		ModelAlias: "shared-chat", ModelVersion: "v1", TargetName: "one-shared-target", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: server.URL + "/v1", ProviderModel: "provider/shared", SecretRef: "SHARED_TARGET_SECRET", TargetTimeout: time.Second, MaxAttempts: 1,
		ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead},
	}
	specA := base
	specA.OrganisationName, specA.OrganisationSlug = "Shared A", "shared-a"
	specB := base
	specB.OrganisationName, specB.OrganisationSlug = "Shared B", "shared-b"
	a, err := store.Provision(context.Background(), specA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Provision(context.Background(), specB)
	if err != nil {
		t.Fatal(err)
	}
	gatewayHandler, err := gateway.New(gateway.Config{Store: store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-secret", true }})
	if err != nil {
		t.Fatal(err)
	}
	call := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"shared-chat","messages":[{"role":"user","content":"scoped observation"}]}`))
	call.Header.Set("Content-Type", "application/json")
	call.Header.Set("Authorization", "Bearer "+a.APIKey)
	callResponse := httptest.NewRecorder()
	gatewayHandler.ServeHTTP(callResponse, call)
	if callResponse.Code != http.StatusOK {
		t.Fatalf("tenant A inference status=%d length=%d", callResponse.Code, callResponse.Body.Len())
	}

	principalB, err := store.Authenticate(context.Background(), credentials.Digest(b.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	routesB, err := store.ListRoutes(context.Background(), principalB)
	if err != nil {
		t.Fatal(err)
	}
	if len(routesB) != 1 || routesB[0].Target.LastHealthCheckAt == nil || routesB[0].Target.LastSuccessAt == nil {
		t.Fatal("fixture did not establish target-global activity from tenant A")
	}
	controlHandler, err := New(Config{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	getPortal := func(key string) ClientDashboardResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
		request.SetBasicAuth("display-only", key)
		response := httptest.NewRecorder()
		controlHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("portal status=%d length=%d", response.Code, response.Body.Len())
		}
		var dashboard ClientDashboardResponse
		if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
			t.Fatal("portal response was not valid JSON")
		}
		return dashboard
	}
	portalA, portalB := getPortal(a.APIKey), getPortal(b.APIKey)
	if portalA.Route.LastHealthCheckAt == nil || portalA.Route.LastSuccessAt == nil || portalA.Route.State != "operational" {
		t.Fatal("tenant A did not receive its own scoped successful observation")
	}
	if portalB.Route.LastHealthCheckAt != nil || portalB.Route.LastSuccessAt != nil || portalB.Route.State != "unknown" || portalB.Source.Freshness != "unknown" {
		t.Fatalf("tenant B inherited shared-target activity: observation_present=%v success_present=%v state=%q freshness=%q", portalB.Route.LastHealthCheckAt != nil, portalB.Route.LastSuccessAt != nil, portalB.Route.State, portalB.Source.Freshness)
	}

	getMachine := func(key string) DashboardResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		request.Header.Set("Authorization", "Bearer "+key)
		response := httptest.NewRecorder()
		controlHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("machine dashboard status=%d length=%d", response.Code, response.Body.Len())
		}
		var dashboard DashboardResponse
		if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
			t.Fatal("machine dashboard response was not valid JSON")
		}
		return dashboard
	}
	machineA, machineB := getMachine(a.APIKey), getMachine(b.APIKey)
	if len(machineA.Routes) != 1 || machineA.Routes[0].LastHealthCheckAt == nil || machineA.Routes[0].LastSuccessAt == nil || machineA.Routes[0].Status != "operational" {
		t.Fatal("Bearer route view omitted tenant A's scoped observation")
	}
	if len(machineB.Routes) != 1 || machineB.Routes[0].LastHealthCheckAt != nil || machineB.Routes[0].LastSuccessAt != nil || machineB.Routes[0].Status != "unknown" {
		t.Fatal("Bearer route view exposed another tenant's shared-target observation")
	}
	if machineB.Routes[0].Source != "target_registry_policy_and_authenticated_request_ledger" {
		t.Fatalf("Bearer route source=%q", machineB.Routes[0].Source)
	}
}

func TestRouteObservationFollowsCurrentBindingGeneration(t *testing.T) {
	newUpstream := func(id string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"id":"`+id+`","model":"provider/executed","choices":[{}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`)
		}))
	}
	oldUpstream := newUpstream("old-success")
	defer oldUpstream.Close()
	newTargetUpstream := newUpstream("new-success")
	defer newTargetUpstream.Close()

	store := memory.New()
	spec := platform.ProvisionSpec{
		OrganisationName: "Binding Tenant", OrganisationSlug: "binding-tenant",
		ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Production", EnvironmentSlug: "production",
		ModelAlias: "binding-chat", ModelVersion: "v1", TargetName: "binding-target-old", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: oldUpstream.URL + "/v1", ProviderModel: "provider/old", SecretRef: "BINDING_TARGET_SECRET", TargetTimeout: time.Second, MaxAttempts: 1,
		ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead},
	}
	provisioned, err := store.Provision(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := store.Authenticate(context.Background(), credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	gatewayHandler, err := gateway.New(gateway.Config{Store: store, AllowInsecureTargets: true, SecretLookup: func(string) (string, bool) { return "provider-secret", true }})
	if err != nil {
		t.Fatal(err)
	}
	controlHandler, err := New(Config{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	infer := func() string {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"binding-chat","messages":[{"role":"user","content":"binding observation"}]}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+provisioned.APIKey)
		response := httptest.NewRecorder()
		gatewayHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("inference status=%d length=%d", response.Code, response.Body.Len())
		}
		return response.Header().Get("X-Alzette-Request-ID")
	}
	portal := func() ClientDashboardResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/dashboard?model=binding-chat", nil)
		request.SetBasicAuth("display-only", provisioned.APIKey)
		response := httptest.NewRecorder()
		controlHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("portal status=%d length=%d", response.Code, response.Body.Len())
		}
		var dashboard ClientDashboardResponse
		if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
			t.Fatal("portal response was not valid JSON")
		}
		return dashboard
	}
	machine := func() DashboardResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?model=binding-chat", nil)
		request.Header.Set("Authorization", "Bearer "+provisioned.APIKey)
		response := httptest.NewRecorder()
		controlHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("machine dashboard status=%d length=%d", response.Code, response.Body.Len())
		}
		var dashboard DashboardResponse
		if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
			t.Fatal("machine dashboard response was not valid JSON")
		}
		return dashboard
	}

	oldRequestID := infer()
	oldRoutes, err := store.ListRoutes(context.Background(), principal)
	if err != nil || len(oldRoutes) != 1 {
		t.Fatal("old route was not available")
	}
	if dashboard := portal(); dashboard.Route.State != "operational" || dashboard.Route.LastSuccessAt == nil {
		t.Fatal("old binding did not expose its successful scoped observation")
	}

	replacement := spec
	replacement.TargetName = "binding-target-new"
	replacement.TargetBaseURL = newTargetUpstream.URL + "/v1"
	replacement.ProviderModel = "provider/new"
	retargeted, err := store.Provision(context.Background(), replacement)
	if err != nil {
		t.Fatal(err)
	}
	if retargeted.RouteID != provisioned.RouteID || retargeted.TargetID == provisioned.TargetID {
		t.Fatal("retarget did not preserve the route and create a new target")
	}
	currentRoutes, err := store.ListRoutes(context.Background(), principal)
	if err != nil || len(currentRoutes) != 1 {
		t.Fatal("retargeted route was not available")
	}
	currentRoute := currentRoutes[0]
	if currentRoute.BindingGeneration != oldRoutes[0].BindingGeneration+1 {
		t.Fatalf("binding generation=%d want=%d", currentRoute.BindingGeneration, oldRoutes[0].BindingGeneration+1)
	}
	idempotent, err := store.Provision(context.Background(), replacement)
	if err != nil || idempotent.RouteID != currentRoute.ID || idempotent.TargetID != currentRoute.Target.ID {
		t.Fatal("idempotent reprovision changed the selected binding identity")
	}
	idempotentRoutes, err := store.ListRoutes(context.Background(), principal)
	if err != nil || len(idempotentRoutes) != 1 || idempotentRoutes[0].BindingGeneration != currentRoute.BindingGeneration {
		t.Fatal("idempotent reprovision advanced the binding generation")
	}
	oldRecord, err := store.GetInferenceRequest(context.Background(), principal, oldRequestID)
	if err != nil || oldRecord.BoundTargetID != oldRoutes[0].Target.ID || oldRecord.RouteBindingGeneration != oldRoutes[0].BindingGeneration {
		t.Fatal("old logical request lost its durable route binding attribution")
	}
	unknownPortal := portal()
	if unknownPortal.Route.State != "unknown" || unknownPortal.Route.LastHealthCheckAt != nil || unknownPortal.Route.LastSuccessAt != nil {
		t.Fatal("portal reused an observation from the previous route binding")
	}
	unknownMachine := machine()
	if len(unknownMachine.Routes) != 1 || unknownMachine.Routes[0].Status != "unknown" || unknownMachine.Routes[0].LastHealthCheckAt != nil || unknownMachine.Routes[0].LastSuccessAt != nil {
		t.Fatal("machine dashboard reused an observation from the previous route binding")
	}

	newRequestID := infer()
	newRecord, err := store.GetInferenceRequest(context.Background(), principal, newRequestID)
	if err != nil || newRecord.BoundTargetID != currentRoute.Target.ID || newRecord.BoundModelID != currentRoute.ModelID || newRecord.RouteBindingGeneration != currentRoute.BindingGeneration {
		t.Fatal("new logical request was not attributed to the current route binding")
	}
	if dashboard := portal(); dashboard.Route.State != "operational" || dashboard.Route.LastHealthCheckAt == nil || dashboard.Route.LastSuccessAt == nil {
		t.Fatal("portal did not expose the new binding's successful observation")
	}
	if dashboard := machine(); len(dashboard.Routes) != 1 || dashboard.Routes[0].Status != "operational" || dashboard.Routes[0].LastHealthCheckAt == nil || dashboard.Routes[0].LastSuccessAt == nil {
		t.Fatal("machine dashboard did not expose the new binding's successful observation")
	}
}

func TestScopedRouteObservationClassifiesOnlyTargetEvidence(t *testing.T) {
	completed := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	route := platform.Route{ID: "rte_test", ModelID: "mdl_test", BindingGeneration: 3, Target: platform.Target{ID: "tgt_test"}}
	request := func(status, errorClass string, at time.Time) platform.InferenceRequest {
		return platform.InferenceRequest{
			RouteID: "rte_test", BoundTargetID: "tgt_test", BoundModelID: "mdl_test", RouteBindingGeneration: 3,
			Status: status, ErrorClass: errorClass, CompletedAt: &at,
		}
	}
	degradedClasses := []string{"target_configuration", "upstream_rate_limited", "upstream_timeout", "upstream_transport", "upstream_unavailable", "upstream_error", "invalid_upstream_response", "upstream_response_too_large"}
	for _, errorClass := range degradedClasses {
		t.Run("degraded_"+errorClass, func(t *testing.T) {
			observation := scopedRouteObservation(route, []platform.InferenceRequest{request("failed", errorClass, completed)})
			if observation.status != "degraded" || observation.latestAt == nil || observation.lastSuccessAt != nil {
				t.Fatalf("target failure classification status=%q observed=%v success=%v", observation.status, observation.latestAt != nil, observation.lastSuccessAt != nil)
			}
		})
	}
	excluded := []struct {
		name, status, errorClass string
	}{
		{"request_rejected", "failed", "upstream_rejected"},
		{"ledger_failure", "failed", "ledger_error"},
		{"client_cancelled", "cancelled", "client_cancelled"},
		{"blocked", "blocked", "model_not_authorised"},
		{"other_failure", "failed", "other"},
	}
	for _, test := range excluded {
		t.Run("unknown_"+test.name, func(t *testing.T) {
			observation := scopedRouteObservation(route, []platform.InferenceRequest{request(test.status, test.errorClass, completed)})
			if observation.status != "unknown" || observation.latestAt != nil || observation.lastSuccessAt != nil {
				t.Fatalf("non-target failure became route evidence status=%q observed=%v success=%v", observation.status, observation.latestAt != nil, observation.lastSuccessAt != nil)
			}
		})
	}
	t.Run("success", func(t *testing.T) {
		observation := scopedRouteObservation(route, []platform.InferenceRequest{request("succeeded", "", completed)})
		if observation.status != "operational" || observation.latestAt == nil || observation.lastSuccessAt == nil {
			t.Fatalf("successful inference classification status=%q observed=%v success=%v", observation.status, observation.latestAt != nil, observation.lastSuccessAt != nil)
		}
	})
	t.Run("later request rejection does not replace target evidence", func(t *testing.T) {
		later := completed.Add(time.Minute)
		observation := scopedRouteObservation(route, []platform.InferenceRequest{
			request("succeeded", "", completed),
			request("failed", "upstream_rejected", later),
		})
		if observation.status != "operational" || observation.latestAt == nil || !observation.latestAt.Equal(completed) {
			t.Fatal("request-specific rejection replaced the latest target observation")
		}
	})
	t.Run("previous binding is ignored", func(t *testing.T) {
		old := request("succeeded", "", completed)
		old.BoundTargetID = "tgt_previous"
		old.RouteBindingGeneration = 2
		observation := scopedRouteObservation(route, []platform.InferenceRequest{old})
		if observation.status != "unknown" || observation.latestAt != nil || observation.lastSuccessAt != nil {
			t.Fatal("a previous route binding became current route evidence")
		}
	})
}

func assertObjectKeys(t *testing.T, label string, raw []byte, want []string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("%s was not a JSON object", label)
	}
	got := make([]string, 0, len(object))
	for key := range object {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s keys=%v want=%v", label, got, want)
	}
}

func assertArrayItemKeys(t *testing.T, label string, raw []byte, want []string) {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		t.Fatalf("%s was not a non-empty JSON array", label)
	}
	assertObjectKeys(t, label, items[0], want)
}
