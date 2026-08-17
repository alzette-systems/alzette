package portal

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
)

//go:embed templates/overview.html
var overviewTemplateFS embed.FS

type overviewRenderer struct {
	template *template.Template
}

type overviewPageView struct {
	CSRFToken string
	User      overviewUserView
	Scope     overviewScopeView
	Source    overviewSourceView
	Route     overviewRouteView
	Usage     overviewUsageView
}

type overviewUserView struct {
	Role string
}

type overviewScopeView struct {
	Organisation       string
	Project            string
	Environment        string
	ProjectEnvironment string
	Detail             string
}

type overviewSourceView struct {
	State      string
	Badge      string
	Label      string
	Detail     string
	Kind       string
	Freshness  string
	Finality   string
	AsOfText   string
	AsOfValue  string
	AsOfPhrase string
}

type overviewRouteView struct {
	State               string
	Signal              string
	StateLabel          string
	StatusLabel         string
	StatusDetail        string
	LastSuccess         string
	LastObservation     string
	Freshness           string
	AttentionTitle      string
	AttentionDetail     string
	CapacityHeadline    string
	BoundaryDetail      string
	ExecutionClass      string
	CapacityMode        string
	DedicatedAllocation string
	RequiresModelChoice bool
}

type overviewUsageView struct {
	LogicalRequests string
	Successful      string
	TokensFinality  string
	P95Latency      string
	Zero            bool
	Unavailable     bool
}

func newOverviewRenderer() (*overviewRenderer, error) {
	parsed, err := template.New("overview.html").Option("missingkey=error").ParseFS(overviewTemplateFS, "templates/overview.html")
	if err != nil {
		return nil, err
	}
	return &overviewRenderer{template: parsed}, nil
}

func (r *overviewRenderer) render(view overviewPageView) ([]byte, error) {
	var output bytes.Buffer
	if err := r.template.ExecuteTemplate(&output, "overview.html", view); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (a *App) serveOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		api.MethodNotAllowed(w, "GET, HEAD", "")
		return
	}
	session, _, err := a.session(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	dashboard, loadErr := a.loadDashboardWithOptions(r, session, dashboardLoadOptions{tolerateRollupFailure: true})
	status := http.StatusOK
	if loadErr != nil {
		status = loadErr.status
		dashboard = unavailableOverviewDashboard(session, loadErr.message)
	}
	view := newOverviewPageView(session, dashboard, csrfCookieValue(r), a.clock().UTC(), r.URL.Query().Get("model"))
	contents, err := a.overview.render(view)
	if err != nil {
		http.Error(w, "Portal page could not be rendered", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(contents)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(contents)
	}
}

func unavailableOverviewDashboard(session platform.PortalSession, detail string) portalDashboard {
	return portalDashboard{
		Schema:  "alzette.portal.dashboard.v1",
		Context: session.Current,
		Source: portalSource{
			Kind: "Unavailable", Label: "Portal data unavailable", Freshness: "unavailable", Finality: "unknown", Detail: detail,
		},
	}
}

func csrfCookieValue(r *http.Request) string {
	values := make([]string, 0, 1)
	for _, cookie := range r.Cookies() {
		if cookie.Name == humanauth.CSRFCookieName {
			values = append(values, cookie.Value)
		}
	}
	if len(values) != 1 || len(values[0]) > 128 {
		return ""
	}
	return values[0]
}

func newOverviewPageView(session platform.PortalSession, data portalDashboard, csrf string, now time.Time, selectedModel string) overviewPageView {
	scope := overviewScopeView{
		Organisation:       fallbackText(session.Current.OrganisationName, "Organisation unavailable"),
		Project:            fallbackText(session.Current.ProjectName, "Project unavailable"),
		Environment:        fallbackText(session.Current.EnvironmentName, "Environment unavailable"),
		ProjectEnvironment: joinScope(session.Current.ProjectName, session.Current.EnvironmentName),
		Detail:             "This membership/session is scoped by the server to one project/environment.",
	}
	return overviewPageView{
		CSRFToken: csrf,
		User:      overviewUserView{Role: humanPortalRole(session.Current.Role)},
		Scope:     scope,
		Source:    newOverviewSourceView(data.Source, data.Rollups),
		Route:     newOverviewRouteView(data, now, selectedModel),
		Usage:     newOverviewUsageView(data.Source, data.Usage),
	}
}

func newOverviewSourceView(source portalSource, rollups portalRollupSeries) overviewSourceView {
	state := overviewSourceState(source, rollups.Freshness)
	badges := map[string]string{
		"live": "Live scoped ledger", "stale": "Stale scoped ledger", "partial": "Partial scoped ledger", "unavailable": "Data unavailable",
	}
	detail := fallbackText(source.Detail, "Source detail was not supplied.")
	if normaliseStatus(rollups.Freshness) == "unavailable" && overviewSourceState(source, "") != "unavailable" {
		detail += " Usage rollups are temporarily unavailable; direct-ledger values, service-plan evidence, and route evidence remain independently available."
	}
	asOf := formatOverviewTime(source.AsOf, "Not available")
	asOfPhrase := "As of not available"
	if !source.AsOf.IsZero() {
		asOfPhrase = "As of " + asOf.Text
	}
	return overviewSourceView{
		State: state, Badge: badges[state], Label: fallbackText(source.Label, "Source unavailable"), Detail: detail,
		Kind: humanStatus(source.Kind), Freshness: humanStatus(source.Freshness), Finality: humanStatus(source.Finality),
		AsOfText: asOf.Text, AsOfValue: asOf.Value, AsOfPhrase: asOfPhrase,
	}
}

func overviewSourceState(source portalSource, rollupFreshness string) string {
	value := normaliseStatus(source.Freshness + " " + source.Finality)
	switch {
	case strings.Contains(value, "unavailable") || strings.Contains(value, "unknown") && source.AsOf.IsZero():
		return "unavailable"
	case strings.Contains(value, "stale"):
		return "stale"
	case strings.Contains(value, "partial") || strings.Contains(value, "incomplete"):
		return "partial"
	case normaliseStatus(rollupFreshness) == "unavailable":
		return "partial"
	default:
		return "live"
	}
}

func newOverviewUsageView(source portalSource, usage portalUsage) overviewUsageView {
	if overviewSourceState(source, "") == "unavailable" {
		return overviewUsageView{LogicalRequests: "Unknown", Successful: "Unknown", TokensFinality: "Unknown / Unknown", P95Latency: "Unavailable", Unavailable: true}
	}
	tokens := "Unknown"
	if usage.TokenMetrics.Total.Value != nil {
		tokens = formatInteger(*usage.TokenMetrics.Total.Value)
	}
	finality := humanStatus(usage.TokenMetrics.Total.Finality)
	if usage.TokenMetrics.Total.Finality == "" {
		finality = "Unknown"
	}
	p95 := "Unavailable"
	if usage.P95LatencyMS != nil {
		p95 = fmt.Sprintf("%s ms", formatInteger(*usage.P95LatencyMS))
	}
	return overviewUsageView{
		LogicalRequests: formatInteger(usage.LogicalRequests), Successful: formatInteger(usage.SuccessfulRequests),
		TokensFinality: tokens + " / " + finality, P95Latency: p95, Zero: usage.LogicalRequests == 0,
	}
}

func newOverviewRouteView(data portalDashboard, now time.Time, selectedModel string) overviewRouteView {
	route, ambiguous := selectOverviewRoute(data.Routes, selectedModel, data.ServicePlan.ModelAlias)
	if ambiguous {
		return overviewRouteView{
			State: "unknown", Signal: "?", StateLabel: "Select a model", StatusLabel: "Select a model to resolve the route",
			StatusDetail: "More than one route is available. Select a model alias before reading status, execution, or capacity.",
			LastSuccess:  "Unknown", LastObservation: "Unknown", Freshness: "Unknown", AttentionTitle: "Select a model",
			AttentionDetail:  "More than one route is available. Select a model alias before interpreting probe evidence or freshness.",
			CapacityHeadline: "Select a model to resolve execution and capacity.",
			BoundaryDetail:   "Execution and capacity are not shown until one selected route and service plan are supplied.",
			ExecutionClass:   "Unavailable — select a model", CapacityMode: "Unavailable — select a model",
			DedicatedAllocation: dedicatedAllocationLabel(data.ServicePlan),
			RequiresModelChoice: true,
		}
	}
	if route == nil {
		return overviewRouteView{
			State: "unavailable", Signal: "?", StateLabel: "Unavailable", StatusLabel: "Route evidence is unavailable",
			StatusDetail: "The connected source did not provide a selected route; no callability or capacity claim is made.",
			LastSuccess:  "Unknown", LastObservation: "Unknown", Freshness: "Unknown", AttentionTitle: "Route evidence unavailable",
			AttentionDetail:  "Connect a selected route to see probe state and evidence freshness.",
			CapacityHeadline: "Execution and capacity details are unavailable.",
			BoundaryDetail:   "Execution and capacity are not shown until the backend supplies a selected route and service plan.",
			ExecutionClass:   "Unavailable — selected route not supplied", CapacityMode: "Unavailable — service plan not supplied",
			DedicatedAllocation: dedicatedAllocationLabel(data.ServicePlan),
		}
	}

	evidence := routeEvidenceView(*route, now)
	execution := humanExecutionClass(route.ExecutionClass)
	capacity := humanCapacityMode(route.CapacityMode, data.ServicePlan)
	capacityHeadline := "Execution and capacity details are unavailable."
	if !strings.HasPrefix(execution, "Unavailable") && !strings.HasPrefix(capacity, "Unavailable") {
		capacityHeadline = execution + " / " + capacity
	}
	lastSuccess := route.LastSuccessAt
	if lastSuccess == nil && successfulStatus(route.LatestInferenceStatus) {
		lastSuccess = route.LatestInferenceAt
	}
	lastObservation := route.LastObservationAt
	if lastObservation == nil {
		lastObservation = route.ObservedAt
	}
	return overviewRouteView{
		State: evidence.State, Signal: evidence.Signal, StateLabel: evidence.StateLabel, StatusLabel: evidence.StatusLabel, StatusDetail: evidence.StatusDetail,
		LastSuccess: formatOverviewTimePointer(lastSuccess), LastObservation: formatOverviewTimePointer(lastObservation), Freshness: evidence.Freshness,
		AttentionTitle: evidence.AttentionTitle, AttentionDetail: evidence.AttentionDetail,
		CapacityHeadline: capacityHeadline,
		BoundaryDetail:   "Execution and service mode are operator-declared for this selected route; live readiness is reported separately.",
		ExecutionClass:   execution, CapacityMode: capacity, DedicatedAllocation: dedicatedAllocationLabel(data.ServicePlan),
	}
}

type overviewRouteEvidence struct {
	State, Signal, StateLabel, StatusLabel, StatusDetail, Freshness, AttentionTitle, AttentionDetail string
}

func routeEvidenceView(route platform.PortalObservation, now time.Time) overviewRouteEvidence {
	freshness := normaliseStatus(route.Freshness)
	freshnessUnknown := freshness == "" || freshness == "unknown" || freshness == "unavailable" || freshness == "not_available"
	stale := strings.Contains(freshness, "stale") || route.FreshUntil != nil && route.FreshUntil.Before(now)
	registryRaw := fallbackText(route.RegistryStatus, route.State)
	registry := humanStatus(registryRaw)
	latest := normaliseStatus(route.LatestInferenceStatus)
	observation := ""
	if latest != "" && latest != "unknown" {
		observation = "Latest inference observation: " + humanStatus(latest) + ". "
	}
	freshnessLabel := "Unknown"
	if !freshnessUnknown {
		freshnessLabel = humanStatus(route.Freshness)
	}
	if normaliseStatus(registryRaw) == "unavailable" {
		return overviewRouteEvidence{"unavailable", "×", "Unavailable", "Unavailable", "Registry state is unavailable and policy blocks calls through this route.", freshnessLabel, "Route unavailable", "The target registry marks this route unavailable; no call should be attempted until the registry changes."}
	}
	if stale {
		return overviewRouteEvidence{"stale", "?", "Stale", "Stale route evidence", "The latest route evidence is stale; current callability is unknown until telemetry refreshes.", "Stale", "Route evidence needs refresh", "The selected route evidence is stale. Current callability is unknown until the route telemetry refreshes."}
	}
	if !route.ProbeEnabled {
		return overviewRouteEvidence{"observation-only", "?", "Observation only", "Live readiness unknown", "Registry state: " + registry + ". " + observation + "No active probe is configured. This view combines registry state with the latest inference observation.", freshnessLabel, "Inference observation only", "No active probe is configured. Registry state and the latest inference observation do not establish live readiness."}
	}
	probe := normaliseStatus(route.ProbeStatus)
	probeUnknown := probe == "" || probe == "unknown" || probe == "not_available" || probe == "not_configured" || probe == "not_running" || probe == "disabled" || probe == "pending"
	freshnessKnown := !freshnessUnknown && !stale
	if freshnessKnown && probe == "unavailable" {
		return overviewRouteEvidence{"unavailable", "×", "Unavailable", "Unavailable", "The opted-in route probe reports unavailable. Current calls should not be attempted.", freshnessLabel, "Probe reports unavailable", "Fresh route evidence reports the opted-in probe as unavailable; this route is not currently callable."}
	}
	if probeUnknown || !freshnessKnown {
		return overviewRouteEvidence{"observation-only", "?", "Observation only", "Live readiness unknown", "Registry state: " + registry + ". " + observation + "The opted-in probe has unknown or unavailable status/freshness, so no active readiness claim is made.", freshnessLabel, "Probe evidence incomplete", "An opted-in probe is present, but its status or freshness is unknown. Live readiness remains unconfirmed."}
	}
	probeReady := probe == "ready" || probe == "healthy" || probe == "operational" || probe == "success" || probe == "succeeded" || probe == "ok"
	probeLabel := humanStatus(probe)
	signal := "!"
	if probeReady {
		probeLabel = "Ready"
		signal = "✓"
	}
	until := ""
	if route.FreshUntil != nil {
		until = " through " + formatOverviewTime(*route.FreshUntil, "Unknown").Text
	}
	return overviewRouteEvidence{normaliseStatus(probeLabel), signal, probeLabel, probeLabel, "Opted-in route probe reports " + probeLabel + ". " + observation + "Registry state: " + registry + ".", freshnessLabel, "Fresh probe evidence", "The opted-in route probe reports " + probeLabel + "; evidence is " + freshnessLabel + until + ". This is selected-route evidence, not a capacity guarantee."}
}

func selectOverviewRoute(routes []platform.PortalObservation, selected, planModel string) (*platform.PortalObservation, bool) {
	model := fallbackText(selected, planModel)
	if model != "" {
		for index := range routes {
			if routes[index].ModelAlias == model {
				return &routes[index], false
			}
		}
	}
	if len(routes) == 1 {
		return &routes[0], false
	}
	return nil, len(routes) > 1
}

func humanExecutionClass(value string) string {
	switch normaliseStatus(value) {
	case "external_pilot", "external_pilot_via_openrouter", "shared", "shared_pilot":
		return "Shared external execution"
	case "private_compatible", "private_compatible_target":
		return "Private execution not yet evidenced"
	case "dedicated", "dedicated_allocation", "dedicated_private":
		return "Dedicated execution not yet evidenced"
	case "", "unknown", "unavailable":
		return "Unavailable — selected route did not supply it"
	default:
		return "Execution boundary not evidenced"
	}
}

func humanCapacityMode(value string, plan platform.PortalServicePlan) string {
	if value == "" {
		value = plan.CapacityMode
	}
	switch normaliseStatus(value) {
	case "shared", "shared_pilot":
		return "Shared service"
	case "dedicated", "dedicated_allocation":
		if plan.Available && !plan.Ambiguous && (plan.DedicatedResourceClass != nil || plan.DedicatedAcceleratorCount != nil) {
			return "Dedicated allocation"
		}
		return "Dedicated allocation not yet evidenced"
	case "", "unknown", "unavailable":
		return "Unavailable — service plan did not supply it"
	default:
		return "Capacity mode not evidenced"
	}
}

func dedicatedAllocationLabel(plan platform.PortalServicePlan) string {
	if !plan.Available || plan.Ambiguous || normaliseStatus(plan.CapacityMode) != "dedicated" {
		return "Unknown — no dedicated allocation evidenced"
	}
	parts := make([]string, 0, 2)
	if plan.DedicatedResourceClass != nil && strings.TrimSpace(*plan.DedicatedResourceClass) != "" {
		parts = append(parts, strings.TrimSpace(*plan.DedicatedResourceClass))
	}
	if plan.DedicatedAcceleratorCount != nil {
		parts = append(parts, fmt.Sprintf("%s accelerator(s)", formatInteger(*plan.DedicatedAcceleratorCount)))
	}
	if len(parts) == 0 {
		return "Unknown — no dedicated allocation evidenced"
	}
	return strings.Join(parts, " · ")
}

func successfulStatus(value string) bool {
	switch normaliseStatus(value) {
	case "success", "succeeded", "ok", "complete", "completed":
		return true
	default:
		return false
	}
}

func humanPortalRole(value string) string {
	switch value {
	case platform.PortalRoleOrgAdmin:
		return "Organisation admin"
	case platform.PortalRoleProjectAdmin:
		return "Project admin"
	case platform.PortalRoleDeveloper:
		return "Developer"
	case platform.PortalRoleViewer:
		return "Viewer"
	default:
		return "Portal member"
	}
}

func humanStatus(value string) string {
	key := normaliseStatus(value)
	labels := map[string]string{
		"ready": "Ready", "active": "Active", "healthy": "Ready", "operational": "Ready", "running": "Running", "fresh": "Fresh",
		"degraded": "Degraded", "unavailable": "Unavailable", "stale": "Stale", "unknown": "Unknown", "partial": "Partial",
		"pending": "Pending", "failed": "Failed", "not_applicable": "Not applicable", "inference_requests": "Authenticated logical request ledger",
	}
	if label, ok := labels[key]; ok {
		return label
	}
	if key == "" {
		return "Unknown"
	}
	words := strings.Fields(strings.ReplaceAll(key, "_", " "))
	for index := range words {
		words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
	}
	return strings.Join(words, " ")
}

func normaliseStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "_", " ", "_").Replace(value)
	return value
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func joinScope(project, environment string) string {
	return fallbackText(project, "Project unavailable") + " / " + fallbackText(environment, "Environment unavailable")
}

func formatInteger(value int64) string {
	return strconv.FormatInt(value, 10)
}

type overviewTimeView struct {
	Text  string
	Value string
}

func formatOverviewTime(value time.Time, fallback string) overviewTimeView {
	if value.IsZero() {
		return overviewTimeView{Text: fallback}
	}
	value = value.UTC()
	return overviewTimeView{Text: value.Format("2 Jan 2006, 15:04 UTC"), Value: value.Format(time.RFC3339)}
}

func formatOverviewTimePointer(value *time.Time) string {
	if value == nil {
		return "Unknown"
	}
	return formatOverviewTime(*value, "Unknown").Text
}
