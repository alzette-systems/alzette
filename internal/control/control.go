package control

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

const (
	maximumUsageWindow = 31 * 24 * time.Hour
	maximumLedgerRows  = 10000
)

type Config struct {
	Store platform.Store
	Clock func() time.Time
	NewID func(string) (string, error)
}

type Control struct {
	store platform.Store
	clock func() time.Time
	newID func(string) (string, error)
}

func New(config Config) (*Control, error) {
	if config.Store == nil {
		return nil, errors.New("control store is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.NewID == nil {
		config.NewID = ids.New
	}
	return &Control{store: config.Store, clock: config.Clock, newID: config.NewID}, nil
}

func (c *Control) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := c.newID("ctl")
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "internal_error", "api_error", "request could not be initialised", "")
		return
	}
	w.Header().Set("X-Alzette-Request-ID", requestID)
	w.Header().Set("X-Request-ID", requestID)
	if r.URL.Path == "/api/dashboard" {
		if r.Method != http.MethodGet {
			api.MethodNotAllowed(w, http.MethodGet, requestID)
			return
		}
		c.clientDashboard(w, r, requestID)
		return
	}
	if r.Method != http.MethodGet {
		api.MethodNotAllowed(w, http.MethodGet, requestID)
		return
	}
	principal, err := api.Authenticate(r, c.store)
	if err != nil {
		if errors.Is(err, platform.ErrUnauthenticated) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="alzette"`)
			api.WriteError(w, http.StatusUnauthorized, "invalid_api_key", "authentication_error", "authentication failed", requestID)
			return
		}
		api.WriteError(w, http.StatusServiceUnavailable, "authentication_unavailable", "api_error", "authentication is temporarily unavailable", requestID)
		return
	}
	if !principal.HasScope(platform.ScopeUsageRead) {
		api.WriteError(w, http.StatusForbidden, "insufficient_scope", "permission_error", "API key is not permitted to read usage", requestID)
		return
	}

	switch {
	case r.URL.Path == "/api/v1/dashboard":
		c.dashboard(w, r, requestID, principal)
	case r.URL.Path == "/api/v1/usage":
		c.usage(w, r, requestID, principal)
	case strings.HasPrefix(r.URL.Path, "/api/v1/requests/"):
		c.requestDetail(w, r, requestID, principal)
	default:
		api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", "resource not found", requestID)
	}
}

type Scope struct {
	OrganisationID   string `json:"organisation_id"`
	OrganisationName string `json:"organisation_name"`
	ProjectID        string `json:"project_id"`
	ProjectName      string `json:"project_name"`
	EnvironmentID    string `json:"environment_id"`
	EnvironmentName  string `json:"environment_name"`
}

type Period struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Timezone string    `json:"timezone"`
}

type TokenMetric struct {
	Value            *int64 `json:"value"`
	KnownRequests    int64  `json:"known_requests"`
	EligibleRequests int64  `json:"eligible_requests"`
	Finality         string `json:"finality"`
}

type UsageSummary struct {
	LogicalRequests    int64       `json:"logical_requests"`
	SuccessfulRequests int64       `json:"successful_requests"`
	FailedRequests     int64       `json:"failed_requests"`
	BlockedRequests    int64       `json:"blocked_requests"`
	CancelledRequests  int64       `json:"cancelled_requests"`
	ErrorRate          float64     `json:"error_rate"`
	P50LatencyMS       *int64      `json:"p50_latency_ms"`
	P95LatencyMS       *int64      `json:"p95_latency_ms"`
	PeakConcurrency    *int64      `json:"peak_concurrency"`
	ThroughputRPS      *float64    `json:"throughput_rps"`
	InputTokens        TokenMetric `json:"input_tokens"`
	OutputTokens       TokenMetric `json:"output_tokens"`
	CachedTokens       TokenMetric `json:"cached_tokens"`
	ReasoningTokens    TokenMetric `json:"reasoning_tokens"`
}

type RequestView struct {
	RequestID     string              `json:"request_id"`
	Timestamp     time.Time           `json:"timestamp"`
	CompletedAt   *time.Time          `json:"completed_at"`
	Project       string              `json:"project"`
	Environment   string              `json:"environment"`
	ModelAlias    string              `json:"model_alias"`
	ExecutedModel string              `json:"executed_model,omitempty"`
	KeyPrefix     string              `json:"key_prefix"`
	Status        string              `json:"status"`
	HTTPStatus    int                 `json:"http_status,omitempty"`
	ErrorClass    string              `json:"error_class,omitempty"`
	LatencyMS     int64               `json:"latency_ms"`
	Usage         platform.TokenUsage `json:"usage"`
	UsageFinality string              `json:"usage_finality"`
}

type UsageResponse struct {
	Scope          Scope         `json:"scope"`
	Period         Period        `json:"period"`
	Summary        UsageSummary  `json:"summary"`
	RecentRequests []RequestView `json:"recent_requests"`
	Source         string        `json:"source"`
	AsOf           time.Time     `json:"as_of"`
	LedgerFinality string        `json:"ledger_finality"`
}

type RouteView struct {
	ModelAlias        string     `json:"model_alias"`
	ModelVersion      string     `json:"model_version"`
	ConfiguredModel   string     `json:"configured_model"`
	Status            string     `json:"status"`
	ExecutionClass    string     `json:"execution_class"`
	ExecutionLabel    string     `json:"execution_label"`
	ServiceMode       string     `json:"service_mode"`
	Endpoint          string     `json:"endpoint"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
	Freshness         string     `json:"freshness"`
	Source            string     `json:"source"`
}

type DashboardResponse struct {
	Scope  Scope         `json:"scope"`
	Routes []RouteView   `json:"routes"`
	Usage  UsageResponse `json:"usage"`
	AsOf   time.Time     `json:"as_of"`
	Source string        `json:"source"`
}

func (c *Control) dashboard(w http.ResponseWriter, r *http.Request, requestID string, principal platform.Principal) {
	if !principal.HasScope(platform.ScopeRoutesRead) {
		api.WriteError(w, http.StatusForbidden, "insufficient_scope", "permission_error", "API key is not permitted to read routes", requestID)
		return
	}
	filter, recentLimit, parseErr := c.parseUsageQuery(r)
	if parseErr != nil {
		api.WriteError(w, parseErr.status, parseErr.code, "invalid_request_error", parseErr.message, requestID)
		return
	}
	usage, requests, ok := c.buildUsage(w, r, requestID, principal, filter, recentLimit)
	if !ok {
		return
	}
	routes, err := c.store.ListRoutes(r.Context(), principal)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "routes_unavailable", "api_error", "route status is temporarily unavailable", requestID)
		return
	}
	asOf := c.clock().UTC()
	views := make([]RouteView, 0, len(routes))
	for _, route := range routes {
		views = append(views, routeView(route, requests, asOf))
	}
	api.WriteJSON(w, http.StatusOK, DashboardResponse{Scope: scopeView(principal), Routes: views, Usage: usage, AsOf: asOf, Source: "target_registry_and_inference_request_ledger"})
}

func (c *Control) usage(w http.ResponseWriter, r *http.Request, requestID string, principal platform.Principal) {
	filter, recentLimit, parseErr := c.parseUsageQuery(r)
	if parseErr != nil {
		api.WriteError(w, parseErr.status, parseErr.code, "invalid_request_error", parseErr.message, requestID)
		return
	}
	usage, _, ok := c.buildUsage(w, r, requestID, principal, filter, recentLimit)
	if ok {
		api.WriteJSON(w, http.StatusOK, usage)
	}
}

func (c *Control) buildUsage(w http.ResponseWriter, r *http.Request, requestID string, principal platform.Principal, filter platform.UsageFilter, recentLimit int) (UsageResponse, []platform.InferenceRequest, bool) {
	filter.Limit = maximumLedgerRows
	page, err := c.store.ListInferenceRequests(r.Context(), principal, filter)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "usage_unavailable", "api_error", "usage ledger is temporarily unavailable", requestID)
		return UsageResponse{}, nil, false
	}
	if page.Truncated {
		api.WriteError(w, http.StatusUnprocessableEntity, "usage_range_too_large", "invalid_request_error", "usage range contains too many requests; select a narrower period", requestID)
		return UsageResponse{}, nil, false
	}
	requests := page.Requests
	views := make([]RequestView, 0, min(recentLimit, len(requests)))
	for index, record := range requests {
		if index >= recentLimit {
			break
		}
		views = append(views, requestView(record, principal))
	}
	finality := "final"
	for _, record := range requests {
		if record.Status == "in_progress" {
			finality = "partial"
			break
		}
	}
	return UsageResponse{Scope: scopeView(principal), Period: Period{From: filter.From, To: filter.To, Timezone: "UTC"}, Summary: summarise(requests), RecentRequests: views, Source: "inference_requests", AsOf: c.clock().UTC(), LedgerFinality: finality}, requests, true
}

func (c *Control) requestDetail(w http.ResponseWriter, r *http.Request, requestID string, principal platform.Principal) {
	if r.URL.RawQuery != "" {
		api.WriteError(w, http.StatusBadRequest, "unknown_query_parameter", "invalid_request_error", "request detail does not accept query parameters", requestID)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/requests/")
	if id == "" || strings.Contains(id, "/") {
		api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", "request not found", requestID)
		return
	}
	record, err := c.store.GetInferenceRequest(r.Context(), principal, id)
	if err != nil {
		if errors.Is(err, platform.ErrNotFound) {
			api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", "request not found", requestID)
			return
		}
		api.WriteError(w, http.StatusServiceUnavailable, "usage_unavailable", "api_error", "request metadata is temporarily unavailable", requestID)
		return
	}
	api.WriteJSON(w, http.StatusOK, requestView(record, principal))
}

type queryError struct {
	status        int
	code, message string
}

func (c *Control) parseUsageQuery(r *http.Request) (platform.UsageFilter, int, *queryError) {
	allowed := map[string]bool{"from": true, "to": true, "model": true, "limit": true}
	for name, values := range r.URL.Query() {
		if !allowed[name] {
			return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "unknown_query_parameter", "unsupported query parameter: " + name}
		}
		if len(values) != 1 {
			return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "invalid_query_parameter", "query parameters may appear only once"}
		}
	}
	now := c.clock().UTC()
	filter := platform.UsageFilter{From: now.Add(-24 * time.Hour), To: now}
	var err error
	if value := r.URL.Query().Get("from"); value != "" {
		filter.From, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "invalid_period", "from must be an RFC3339 timestamp"}
		}
	}
	if value := r.URL.Query().Get("to"); value != "" {
		filter.To, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "invalid_period", "to must be an RFC3339 timestamp"}
		}
	}
	if !filter.From.Before(filter.To) || filter.To.Sub(filter.From) > maximumUsageWindow {
		return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "invalid_period", "usage period must be positive and no longer than 31 days"}
	}
	filter.From, filter.To = filter.From.UTC(), filter.To.UTC()
	filter.ModelAlias = r.URL.Query().Get("model")
	if len(filter.ModelAlias) > 128 {
		return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "invalid_model", "model filter is invalid"}
	}
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			return platform.UsageFilter{}, 0, &queryError{http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100"}
		}
	}
	return filter, limit, nil
}

func summarise(requests []platform.InferenceRequest) UsageSummary {
	summary := UsageSummary{LogicalRequests: int64(len(requests))}
	latencies := make([]int64, 0, len(requests))
	eligible := int64(0)
	var input, output, cached, reasoning metricBuilder
	for _, record := range requests {
		switch record.Status {
		case "succeeded":
			summary.SuccessfulRequests++
			eligible++
		case "failed":
			summary.FailedRequests++
		case "blocked":
			summary.BlockedRequests++
		case "cancelled":
			summary.CancelledRequests++
		}
		if record.CompletedAt != nil {
			latencies = append(latencies, record.Duration.Milliseconds())
		}
		if record.Status == "succeeded" {
			input.add(record.Usage.InputTokens)
			output.add(record.Usage.OutputTokens)
			cached.add(record.Usage.CachedTokens)
			reasoning.add(record.Usage.ReasoningTokens)
		}
	}
	errorsCount := summary.FailedRequests + summary.CancelledRequests
	if summary.LogicalRequests > 0 {
		summary.ErrorRate = float64(errorsCount) / float64(summary.LogicalRequests)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	summary.P50LatencyMS = percentile(latencies, 0.50)
	summary.P95LatencyMS = percentile(latencies, 0.95)
	summary.InputTokens = input.result(eligible)
	summary.OutputTokens = output.result(eligible)
	summary.CachedTokens = cached.result(eligible)
	summary.ReasoningTokens = reasoning.result(eligible)
	return summary
}

type metricBuilder struct {
	value int64
	known int64
}

func (m *metricBuilder) add(value *int64) {
	if value != nil {
		m.value += *value
		m.known++
	}
}
func (m metricBuilder) result(eligible int64) TokenMetric {
	result := TokenMetric{KnownRequests: m.known, EligibleRequests: eligible, Finality: "unknown"}
	if m.known > 0 {
		value := m.value
		result.Value = &value
		result.Finality = "partial"
	}
	if eligible > 0 && m.known == eligible {
		result.Finality = "final"
	}
	return result
}
func percentile(values []int64, p float64) *int64 {
	if len(values) == 0 {
		return nil
	}
	index := int(math.Ceil(float64(len(values))*p)) - 1
	if index < 0 {
		index = 0
	}
	value := values[index]
	return &value
}
func scopeView(p platform.Principal) Scope {
	return Scope{OrganisationID: p.OrganisationID, OrganisationName: p.OrganisationName, ProjectID: p.ProjectID, ProjectName: p.ProjectName, EnvironmentID: p.EnvironmentID, EnvironmentName: p.EnvironmentName}
}
func requestView(record platform.InferenceRequest, p platform.Principal) RequestView {
	return RequestView{RequestID: record.ID, Timestamp: record.StartedAt, CompletedAt: record.CompletedAt, Project: p.ProjectName, Environment: p.EnvironmentName, ModelAlias: record.ModelAlias, ExecutedModel: record.ExecutedModel, KeyPrefix: record.KeyPrefix, Status: record.Status, HTTPStatus: record.HTTPStatus, ErrorClass: record.ErrorClass, LatencyMS: record.Duration.Milliseconds(), Usage: record.Usage, UsageFinality: record.UsageFinality}
}
func routeView(route platform.Route, requests []platform.InferenceRequest, now time.Time) RouteView {
	observation := scopedRouteObservation(route, requests)
	status := observation.status
	if !route.ModelEnabled || !route.Enabled || !route.Target.Enabled {
		status = "unavailable"
	}
	label := "External pilot via OpenRouter"
	if route.Target.ExecutionClass == "private_compatible" {
		label = "Private compatible target (operator evidence pending)"
	}
	mode := "Shared pilot"
	if route.Target.CapacityMode == "dedicated" && route.Target.CapacityEvidenceRef != "" {
		mode = "Dedicated"
	} else if route.Target.CapacityMode == "dedicated" {
		mode = "Unknown"
	}
	freshness := routeFreshness(observation.latestAt, now)
	if freshness == "fresh" {
		freshness = "live"
	}
	return RouteView{ModelAlias: route.ModelAlias, ModelVersion: route.ModelVersion, ConfiguredModel: route.Target.ProviderModel, Status: status, ExecutionClass: route.Target.ExecutionClass, ExecutionLabel: label, ServiceMode: mode, Endpoint: "/v1/chat/completions", LastHealthCheckAt: observation.latestAt, LastSuccessAt: observation.lastSuccessAt, Freshness: freshness, Source: "target_registry_policy_and_authenticated_request_ledger"}
}
func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
