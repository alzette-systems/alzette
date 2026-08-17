package control

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/platform"
)

const clientDashboardSchema = "alzette.client_dashboard.v1"

type ClientDashboardResponse struct {
	Schema         string                `json:"schema"`
	Account        ClientAccount         `json:"account"`
	Period         ClientPeriod          `json:"period"`
	Source         ClientSource          `json:"source"`
	Route          ClientRoute           `json:"route"`
	Usage          ClientUsage           `json:"usage"`
	Trend          ClientTrend           `json:"trend"`
	Breakdowns     ClientBreakdowns      `json:"breakdowns"`
	RecentRequests []ClientRequest       `json:"recent_requests"`
	Export         ClientDashboardExport `json:"export"`
}

type ClientAccount struct {
	Name     string `json:"name"`
	Initials string `json:"initials"`
}

type ClientPeriod struct {
	Label    string   `json:"label"`
	Timezone string   `json:"timezone"`
	Options  []string `json:"options"`
}

type ClientSource struct {
	Kind      string    `json:"kind"`
	Label     string    `json:"label"`
	Detail    string    `json:"detail"`
	AsOf      time.Time `json:"as_of"`
	Freshness string    `json:"freshness"`
	Finality  string    `json:"finality"`
}

type ClientRoute struct {
	State             string          `json:"state"`
	StatusDetail      string          `json:"status_detail"`
	EndpointPath      string          `json:"endpoint_path"`
	ModelAlias        *string         `json:"model_alias"`
	ExecutionClass    *string         `json:"execution_class"`
	CapacityMode      *string         `json:"capacity_mode"`
	LastSuccessAt     *time.Time      `json:"last_success_at"`
	LastHealthCheckAt *time.Time      `json:"last_health_check_at"`
	Attention         ClientAttention `json:"attention"`
}

type ClientAttention struct {
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

type ClientUsage struct {
	LogicalRequests    int64        `json:"logical_requests"`
	SuccessfulRequests int64        `json:"successful_requests"`
	SuccessRate        *float64     `json:"success_rate"`
	ErrorRate          *float64     `json:"error_rate"`
	BlockedRequests    int64        `json:"blocked_requests"`
	Tokens             ClientTokens `json:"tokens"`
	P50LatencyMS       *int64       `json:"p50_latency_ms"`
	P95LatencyMS       *int64       `json:"p95_latency_ms"`
	Allowance          interface{}  `json:"allowance"`
}

type ClientTokens struct {
	Input     *int64 `json:"input"`
	Output    *int64 `json:"output"`
	Cached    *int64 `json:"cached"`
	Reasoning *int64 `json:"reasoning"`
	Total     *int64 `json:"total"`
}

type ClientTrend struct {
	Unit   string             `json:"unit"`
	Points []ClientTrendPoint `json:"points"`
}

type ClientTrendPoint struct {
	Label        string   `json:"label"`
	Requests     int64    `json:"requests"`
	Tokens       *int64   `json:"tokens"`
	SuccessRate  *float64 `json:"success_rate"`
	P95LatencyMS *int64   `json:"p95_latency_ms"`
}

type ClientBreakdowns struct {
	Projects []ClientProjectBreakdown `json:"projects"`
	Models   []ClientModelBreakdown   `json:"models"`
}

type ClientProjectBreakdown struct {
	Name     string   `json:"name"`
	Requests int64    `json:"requests"`
	Tokens   *int64   `json:"tokens"`
	Share    *float64 `json:"share"`
}

type ClientModelBreakdown struct {
	Alias         string  `json:"alias"`
	ExecutedModel *string `json:"executed_model"`
	Requests      int64   `json:"requests"`
	Tokens        *int64  `json:"tokens"`
	Share         float64 `json:"share"`
}

type ClientRequest struct {
	RequestID     string    `json:"request_id"`
	OccurredAt    time.Time `json:"occurred_at"`
	Project       string    `json:"project"`
	ModelAlias    string    `json:"model_alias"`
	ExecutedModel *string   `json:"executed_model"`
	Status        string    `json:"status"`
	ErrorClass    *string   `json:"error_class"`
	LatencyMS     *int64    `json:"latency_ms"`
	Tokens        *int64    `json:"tokens"`
}

type ClientDashboardExport struct {
	Available bool     `json:"available"`
	Formats   []string `json:"formats"`
	Scope     string   `json:"scope"`
	Units     []string `json:"units"`
}

func (c *Control) clientDashboard(w http.ResponseWriter, r *http.Request, requestID string) {
	principal, err := api.AuthenticateBasic(r, c.store)
	if err != nil {
		if errors.Is(err, platform.ErrUnauthenticated) {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+api.PortalRealm+`", charset="UTF-8"`)
			api.WriteError(w, http.StatusUnauthorized, "invalid_api_key", "authentication_error", "authentication failed", requestID)
			return
		}
		api.WriteError(w, http.StatusServiceUnavailable, "authentication_unavailable", "api_error", "authentication is temporarily unavailable", requestID)
		return
	}
	if !principal.HasScope(platform.ScopeUsageRead) || !principal.HasScope(platform.ScopeRoutesRead) {
		api.WriteError(w, http.StatusForbidden, "insufficient_scope", "permission_error", "API key is not permitted to read the client dashboard", requestID)
		return
	}

	filter, recentLimit, parseErr := c.parseUsageQuery(r)
	if parseErr != nil {
		api.WriteError(w, parseErr.status, parseErr.code, "invalid_request_error", parseErr.message, requestID)
		return
	}
	filter.Limit = maximumLedgerRows
	page, err := c.store.ListInferenceRequests(r.Context(), principal, filter)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "usage_unavailable", "api_error", "usage ledger is temporarily unavailable", requestID)
		return
	}
	routes, err := c.store.ListRoutes(r.Context(), principal)
	if err != nil {
		api.WriteError(w, http.StatusServiceUnavailable, "routes_unavailable", "api_error", "route status is temporarily unavailable", requestID)
		return
	}

	asOf := c.clock().UTC()
	route, freshness := clientRoute(routes, filter.ModelAlias, page.Requests, asOf)
	partialReasons := clientPartialReasons(page, filter, asOf)
	finality := "final"
	scopeName := principal.ProjectName + " / " + principal.EnvironmentName
	detail := "Usage is scoped to the authenticated project/environment " + scopeName + " and comes from the final direct logical request ledger. Route status and observation time come from target-registry policy and the latest inference observation attributed to the current route binding in this authenticated scope, not an active health probe."
	if len(partialReasons) > 0 {
		finality = "partial"
		detail = "Usage is scoped to the authenticated project/environment " + scopeName + ". Partial snapshot: " + strings.Join(partialReasons, "; ") + ". Totals reconcile to the logical request rows represented here; unknown token totals remain unavailable. Route status and observation time come from target-registry policy and the latest inference observation attributed to the current route binding in this authenticated scope, not an active health probe."
	}
	recent := clientRecentRequests(page.Requests, principal, recentLimit)
	exportAvailable := len(partialReasons) == 0 && len(page.Requests) <= recentLimit
	formats := []string{}
	if exportAvailable {
		formats = []string{"csv", "json"}
	}

	response := ClientDashboardResponse{
		Schema:  clientDashboardSchema,
		Account: ClientAccount{Name: principal.OrganisationName, Initials: accountInitials(principal.OrganisationName)},
		Period: ClientPeriod{
			Label:    filter.From.Format("02 Jan 2006 15:04") + " – " + filter.To.Format("02 Jan 2006 15:04 UTC"),
			Timezone: "UTC",
			Options:  []string{},
		},
		Source: ClientSource{Kind: "tenant_usage", Label: "Tenant usage snapshot", Detail: detail, AsOf: asOf, Freshness: freshness, Finality: finality},
		Route:  route,
		Usage:  clientUsage(page.Requests),
		Trend:  clientTrend(page.Requests, filter),
		Breakdowns: ClientBreakdowns{
			Projects: clientProjectBreakdown(page.Requests, principal),
			Models:   clientModelBreakdown(page.Requests),
		},
		RecentRequests: recent,
		Export: ClientDashboardExport{
			Available: exportAvailable,
			Formats:   formats,
			Scope:     "authenticated_project_environment",
			Units:     []string{"logical_requests", "input_tokens", "output_tokens", "cached_tokens", "reasoning_tokens"},
		},
	}
	api.WriteJSON(w, http.StatusOK, response)
}

func clientPartialReasons(page platform.RequestPage, filter platform.UsageFilter, asOf time.Time) []string {
	reasons := make([]string, 0, 4)
	if page.Truncated {
		reasons = append(reasons, "the selected range exceeded the 10,000-row read limit and only the newest returned rows are represented")
	}
	if filter.To.After(asOf) {
		reasons = append(reasons, "the selected range extends beyond the snapshot time")
	}
	inProgress, incompleteUsage := false, false
	for _, record := range page.Requests {
		if record.Status == "in_progress" {
			inProgress = true
		}
		if record.Status == "succeeded" && record.UsageFinality != "final" {
			incompleteUsage = true
		}
	}
	if inProgress {
		reasons = append(reasons, "one or more logical requests are still in progress")
	}
	if incompleteUsage {
		reasons = append(reasons, "one or more successful requests have incomplete token usage")
	}
	return reasons
}

func clientRoute(routes []platform.Route, modelAlias string, requests []platform.InferenceRequest, now time.Time) (ClientRoute, string) {
	candidates := make([]platform.Route, 0, len(routes))
	for _, route := range routes {
		if modelAlias == "" || route.ModelAlias == modelAlias {
			candidates = append(candidates, route)
		}
	}
	base := ClientRoute{State: "unknown", EndpointPath: "POST /v1/chat/completions"}
	if len(candidates) == 0 {
		base.StatusDetail = "No configured route matches this authenticated project and environment."
		if modelAlias != "" {
			base.StatusDetail = "No configured route matches the selected model alias in this authenticated project and environment."
		}
		base.Attention = ClientAttention{Severity: "warning", Title: "Route not configured", Detail: "Ask the Alzette operator to configure an authorised model route before calling inference."}
		return base, "unknown"
	}
	if len(candidates) > 1 {
		base.StatusDetail = "Multiple routes are configured for this authenticated project and environment; no route health has been selected."
		base.Attention = ClientAttention{Severity: "info", Title: "Select a model alias", Detail: "Use the model query parameter to view exactly one configured route."}
		return base, "unknown"
	}

	selected := candidates[0]
	alias := selected.ModelAlias
	base.ModelAlias = &alias
	observation := scopedRouteObservation(selected, requests)
	base.LastSuccessAt = observation.lastSuccessAt
	base.LastHealthCheckAt = observation.latestAt
	if selected.Target.ExecutionClass != "external_pilot" || selected.Target.CapacityMode != "shared" {
		base.StatusDetail = "The configured route is outside the supported external shared-pilot portal boundary; target-registry policy and scoped inference observations are not represented as route health here."
		base.Attention = ClientAttention{Severity: "warning", Title: "Route boundary unsupported", Detail: "Use an operator-reviewed surface for this route configuration."}
		return base, routeFreshness(observation.latestAt, now)
	}
	executionClass, capacityMode := "external_pilot", "shared"
	base.ExecutionClass, base.CapacityMode = &executionClass, &capacityMode
	base.State = routeState(selected, observation)
	switch base.State {
	case "operational":
		base.StatusDetail = "The latest inference observation attributed to the current route binding in this authenticated project/environment recorded this external pilot route as operational; this is not an active health probe."
		base.Attention = ClientAttention{Severity: "good", Title: "Latest scoped observation is operational", Detail: "This status is scoped route evidence only; it is not a capacity or execution-locality guarantee."}
	case "degraded":
		base.StatusDetail = "The latest target/upstream failure observation attributed to the current route binding in this authenticated project/environment marks the external pilot route degraded; request-specific rejections and client or ledger failures are not treated as route health evidence. This is not an active health probe."
		base.Attention = ClientAttention{Severity: "warning", Title: "Latest scoped inference did not succeed", Detail: "Review the scoped observation time before making a call decision."}
	case "unavailable":
		base.StatusDetail = "Target-registry policy marks the configured external pilot route unavailable; this is not an active health probe or a tenant activity timestamp."
		base.Attention = ClientAttention{Severity: "warning", Title: "Registry policy blocks the route", Detail: "Do not assume another model or route will be selected automatically."}
	default:
		base.StatusDetail = "No completed inference observation attributed to the current route binding in this authenticated project/environment is available for the configured external pilot route; this is not an active health probe."
		base.Attention = ClientAttention{Severity: "info", Title: "Scoped route observation unknown", Detail: "A different tenant's activity on a shared target is not used as evidence for this scope."}
	}
	freshness := routeFreshness(observation.latestAt, now)
	if freshness == "stale" {
		base.Attention = ClientAttention{Severity: "warning", Title: "Route observation is stale", Detail: "The last route observation is older than two minutes; verify it before making a call decision."}
	}
	return base, freshness
}

type routeObservation struct {
	status        string
	latestAt      *time.Time
	lastSuccessAt *time.Time
}

func scopedRouteObservation(route platform.Route, requests []platform.InferenceRequest) routeObservation {
	result := routeObservation{status: "unknown"}
	if route.ID == "" || route.Target.ID == "" || route.ModelID == "" || route.BindingGeneration <= 0 {
		return result
	}
	for _, request := range requests {
		if request.RouteID != route.ID ||
			request.BoundTargetID != route.Target.ID ||
			request.BoundModelID != route.ModelID ||
			request.RouteBindingGeneration != route.BindingGeneration ||
			request.CompletedAt == nil {
			continue
		}
		if result.latestAt == nil || request.CompletedAt.After(*result.latestAt) {
			status := ""
			if request.Status == "succeeded" {
				status = "operational"
			} else if request.Status == "failed" && isTargetFailureObservation(request.ErrorClass) {
				status = "degraded"
			}
			if status != "" {
				observedAt := *request.CompletedAt
				result.latestAt = &observedAt
				result.status = status
			}
		}
		if request.Status == "succeeded" && (result.lastSuccessAt == nil || request.CompletedAt.After(*result.lastSuccessAt)) {
			succeededAt := *request.CompletedAt
			result.lastSuccessAt = &succeededAt
		}
	}
	return result
}

func isTargetFailureObservation(errorClass string) bool {
	switch errorClass {
	case "target_configuration", "upstream_rate_limited", "upstream_timeout", "upstream_transport", "upstream_unavailable", "upstream_error", "invalid_upstream_response", "upstream_response_too_large":
		return true
	default:
		return false
	}
}

func routeState(route platform.Route, observation routeObservation) string {
	if !route.ModelEnabled || !route.Enabled || !route.Target.Enabled || route.Target.HealthStatus == "unavailable" {
		return "unavailable"
	}
	return observation.status
}

func routeFreshness(observedAt *time.Time, now time.Time) string {
	if observedAt == nil {
		return "unknown"
	}
	if now.Sub(*observedAt) > 2*time.Minute {
		return "stale"
	}
	return "fresh"
}

func clientUsage(requests []platform.InferenceRequest) ClientUsage {
	summary := summarise(requests)
	result := ClientUsage{
		LogicalRequests:    summary.LogicalRequests,
		SuccessfulRequests: summary.SuccessfulRequests,
		SuccessRate:        percentage(summary.SuccessfulRequests, summary.LogicalRequests),
		ErrorRate:          percentage(summary.FailedRequests+summary.CancelledRequests, summary.LogicalRequests),
		BlockedRequests:    summary.BlockedRequests,
		P50LatencyMS:       summary.P50LatencyMS,
		P95LatencyMS:       summary.P95LatencyMS,
		Allowance:          nil,
	}
	result.Tokens.Input = completeTokenMetric(summary.InputTokens)
	result.Tokens.Output = completeTokenMetric(summary.OutputTokens)
	result.Tokens.Cached = completeTokenMetric(summary.CachedTokens)
	result.Tokens.Reasoning = completeTokenMetric(summary.ReasoningTokens)
	result.Tokens.Total = addKnown(result.Tokens.Input, result.Tokens.Output)
	return result
}

func completeTokenMetric(metric TokenMetric) *int64 {
	if metric.EligibleRequests == 0 || metric.KnownRequests != metric.EligibleRequests || metric.Value == nil {
		return nil
	}
	value := *metric.Value
	return &value
}

func addKnown(left, right *int64) *int64 {
	if left == nil || right == nil {
		return nil
	}
	value := *left + *right
	return &value
}

func percentage(value, total int64) *float64 {
	if total == 0 {
		return nil
	}
	result := float64(value) / float64(total) * 100
	return &result
}

func clientTrend(requests []platform.InferenceRequest, filter platform.UsageFilter) ClientTrend {
	interval := 24 * time.Hour
	labelLayout := "02 Jan"
	if filter.To.Sub(filter.From) <= 48*time.Hour {
		interval = time.Hour
		labelLayout = "02 Jan 15:00"
	}
	buckets := make(map[time.Time][]platform.InferenceRequest)
	for _, record := range requests {
		bucket := record.StartedAt.UTC().Truncate(interval)
		buckets[bucket] = append(buckets[bucket], record)
	}
	keys := make([]time.Time, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	points := make([]ClientTrendPoint, 0, len(keys))
	for _, key := range keys {
		items := buckets[key]
		usage := clientUsage(items)
		points = append(points, ClientTrendPoint{Label: key.Format(labelLayout), Requests: usage.LogicalRequests, Tokens: usage.Tokens.Total, SuccessRate: usage.SuccessRate, P95LatencyMS: usage.P95LatencyMS})
	}
	return ClientTrend{Unit: "requests", Points: points}
}

func clientProjectBreakdown(requests []platform.InferenceRequest, principal platform.Principal) []ClientProjectBreakdown {
	if len(requests) == 0 {
		return []ClientProjectBreakdown{}
	}
	share := 100.0
	return []ClientProjectBreakdown{{Name: principal.ProjectName + " / " + principal.EnvironmentName, Requests: int64(len(requests)), Tokens: clientUsage(requests).Tokens.Total, Share: &share}}
}

func clientModelBreakdown(requests []platform.InferenceRequest) []ClientModelBreakdown {
	type key struct{ alias, executed string }
	groups := make(map[key][]platform.InferenceRequest)
	for _, record := range requests {
		groups[key{alias: record.ModelAlias, executed: record.ExecutedModel}] = append(groups[key{alias: record.ModelAlias, executed: record.ExecutedModel}], record)
	}
	keys := make([]key, 0, len(groups))
	for value := range groups {
		keys = append(keys, value)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].alias == keys[j].alias {
			return keys[i].executed < keys[j].executed
		}
		return keys[i].alias < keys[j].alias
	})
	result := make([]ClientModelBreakdown, 0, len(keys))
	for _, value := range keys {
		items := groups[value]
		var executed *string
		if value.executed != "" {
			copy := value.executed
			executed = &copy
		}
		result = append(result, ClientModelBreakdown{Alias: value.alias, ExecutedModel: executed, Requests: int64(len(items)), Tokens: clientUsage(items).Tokens.Total, Share: float64(len(items)) / float64(len(requests)) * 100})
	}
	return result
}

func clientRecentRequests(requests []platform.InferenceRequest, principal platform.Principal, limit int) []ClientRequest {
	count := min(limit, len(requests))
	result := make([]ClientRequest, 0, count)
	for index := 0; index < count; index++ {
		record := requests[index]
		var executed, errorClass *string
		if record.ExecutedModel != "" {
			value := record.ExecutedModel
			executed = &value
		}
		if record.ErrorClass != "" {
			value := record.ErrorClass
			errorClass = &value
		}
		var latency *int64
		if record.CompletedAt != nil {
			value := record.Duration.Milliseconds()
			latency = &value
		}
		result = append(result, ClientRequest{RequestID: record.ID, OccurredAt: record.StartedAt, Project: principal.ProjectName, ModelAlias: record.ModelAlias, ExecutedModel: executed, Status: record.Status, ErrorClass: errorClass, LatencyMS: latency, Tokens: addKnown(record.Usage.InputTokens, record.Usage.OutputTokens)})
	}
	return result
}

func accountInitials(name string) string {
	words := strings.Fields(name)
	initials := make([]rune, 0, 2)
	for _, word := range words {
		letters := []rune(word)
		if len(letters) > 0 {
			initials = append(initials, letters[0])
		}
		if len(initials) == 2 {
			break
		}
	}
	if len(initials) == 0 {
		return "—"
	}
	return strings.ToUpper(string(initials))
}
