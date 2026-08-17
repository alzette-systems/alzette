package portal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alzette/internal/api"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
	"alzette/internal/store/memory"
)

const (
	testSessionToken = "test-session-token-not-a-real-secret"
	testCSRFToken    = "test-csrf-token-not-a-real-secret"
	testOneTimeKey   = "alz_k_once_value_never_log_this"
)

type portalStub struct {
	*memory.Store
	now          time.Time
	digest       [32]byte
	revoked      bool
	session      platform.PortalSession
	access       []platform.PortalServiceAccount
	exportRows   []platform.PortalExportRow
	rollups      []platform.PortalUsageRollup
	rollupErr    error
	observations []platform.PortalObservation
	plan         platform.PortalServicePlan
	checkpoint   platform.RollupCheckpoint
	lastIssue    platform.PortalKeyIssueSpec
	lastFormat   string
}

func newPortalStub(now time.Time) *portalStub {
	membership := platform.PortalMembership{ID: "mem_a", OrganisationID: "org_a", OrganisationName: "Alzette Demo", OrganisationSlug: "alzette-demo", ProjectID: "prj_a", ProjectName: "Inference Pilot", ProjectSlug: "inference-pilot", EnvironmentID: "env_a", EnvironmentName: "PoC", EnvironmentSlug: "poc", Role: platform.PortalRoleProjectAdmin}
	return &portalStub{
		Store: memory.New(), now: now, digest: humanauth.Digest(testSessionToken),
		session:    platform.PortalSession{ID: "ses_a", User: platform.PortalUser{ID: "usr_a", Username: "alice", DisplayName: "Alice"}, Current: membership, Memberships: []platform.PortalMembership{membership}, ExpiresAt: now.Add(time.Hour)},
		access:     []platform.PortalServiceAccount{{ID: "sa_a", Name: "application", CreatedAt: now.Add(-time.Hour), Keys: []platform.PortalKeyRecord{}}},
		checkpoint: platform.RollupCheckpoint{Status: "succeeded", LastStartedAt: timePointer(now.Add(-time.Minute)), LastCompletedAt: timePointer(now.Add(-time.Minute)), RangeFrom: timePointer(now.Add(-24 * time.Hour).Truncate(time.Hour)), RangeTo: timePointer(now.Truncate(time.Hour)), SourceRows: intPointer(0)},
		plan:       platform.PortalServicePlan{Available: false, Source: "operator_registry", Finality: "unknown"},
	}
}

func (s *portalStub) CreatePortalSession(_ context.Context, username, password string, digest [32]byte, expires, _ time.Time) (platform.PortalSession, error) {
	if username != "alice" || password != "valid-human-password" {
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	s.digest = digest
	s.session.ExpiresAt = expires
	return s.session, nil
}
func (s *portalStub) AuthenticatePortalSession(_ context.Context, digest [32]byte, now time.Time) (platform.PortalSession, error) {
	if s.revoked || digest != s.digest || !s.session.ExpiresAt.After(now) {
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	return s.session, nil
}
func (s *portalStub) ReauthenticatePortalSession(_ context.Context, digest [32]byte, password string, now time.Time) (platform.PortalSession, error) {
	if digest != s.digest || password != "valid-human-password" {
		return platform.PortalSession{}, platform.ErrUnauthenticated
	}
	s.session.AuthenticatedAt = now
	return s.session, nil
}
func (s *portalStub) RevokePortalSession(_ context.Context, digest [32]byte, _ time.Time) error {
	if digest != s.digest {
		return platform.ErrUnauthenticated
	}
	s.revoked = true
	return nil
}
func (s *portalStub) SwitchPortalContext(_ context.Context, digest [32]byte, membershipID string, _ time.Time) (platform.PortalSession, error) {
	if digest != s.digest || membershipID != s.session.Current.ID {
		return platform.PortalSession{}, platform.ErrForbidden
	}
	return s.session, nil
}
func (s *portalStub) ListPortalAccess(context.Context, platform.PortalSession) ([]platform.PortalServiceAccount, error) {
	return s.access, nil
}
func (s *portalStub) CreatePortalServiceAccount(_ context.Context, _ platform.PortalSession, name string) (platform.PortalServiceAccount, error) {
	if strings.TrimSpace(name) == "" {
		return platform.PortalServiceAccount{}, platform.ErrInvalid
	}
	return platform.PortalServiceAccount{ID: "sa_new", Name: name, CreatedAt: s.now, Keys: []platform.PortalKeyRecord{}}, nil
}
func (s *portalStub) IssuePortalKey(_ context.Context, _ platform.PortalSession, spec platform.PortalKeyIssueSpec) (platform.PortalKeyResult, error) {
	s.lastIssue = spec
	return platform.PortalKeyResult{Name: spec.Name, Prefix: "alz_k_once", APIKey: testOneTimeKey, Scopes: spec.Scopes, ExpiresAt: spec.ExpiresAt}, nil
}
func (s *portalStub) RevokePortalKey(context.Context, platform.PortalSession, string) error {
	return nil
}
func (s *portalStub) GetPortalServicePlan(context.Context, platform.PortalSession, string) (platform.PortalServicePlan, error) {
	return s.plan, nil
}
func (s *portalStub) ListPortalExport(_ context.Context, _ platform.PortalSession, _ platform.UsageFilter, format string) ([]platform.PortalExportRow, error) {
	s.lastFormat = format
	return s.exportRows, nil
}
func (s *portalStub) ListPortalRollups(context.Context, platform.PortalSession, platform.UsageFilter) ([]platform.PortalUsageRollup, error) {
	return s.rollups, s.rollupErr
}
func (s *portalStub) ListPortalObservations(context.Context, platform.PortalSession, string, time.Time) ([]platform.PortalObservation, error) {
	return s.observations, nil
}
func (s *portalStub) GetRollupCheckpoint(context.Context, platform.PortalSession) (platform.RollupCheckpoint, error) {
	return s.checkpoint, nil
}

func newTestApp(t *testing.T, store *portalStub, secure bool) *App {
	t.Helper()
	directory := t.TempDir()
	writePortalAssets(t, directory)
	app, err := New(Config{Store: store, PortalStore: store, StaticDirectory: directory, CookieSecure: secure, SessionTTL: time.Hour, Clock: func() time.Time { return store.now }, GenerateSessionToken: func() (string, error) { return testSessionToken, nil }, GenerateCSRFToken: func() (string, error) { return testCSRFToken, nil }, PublicGatewayURL: "http://192.168.178.167:19080", AllowInsecurePublicGateway: true})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func writePortalAssets(t *testing.T, directory string) {
	t.Helper()
	files := map[string]string{
		"login.html": "<!doctype html><head>" + rawMarker + "</head><form></form>",
		"login.css":  "body{}", "portal.html": "<!doctype html><head>" + rawMarker + `<script src="portal.js" defer></script></head><main></main>`,
		"portal.css": "main{}", "portal.js": "'use strict';", "alzette-mark.svg": `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func authenticatedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: humanauth.SessionCookieName, Value: testSessionToken})
	request.AddCookie(&http.Cookie{Name: humanauth.CSRFCookieName, Value: testCSRFToken})
	request.Header.Set("X-CSRF-Token", testCSRFToken)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestPortalRedirectLoginCookiesAndNoBasicChallenge(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	app := newTestApp(t, store, false)
	request := httptest.NewRequest(http.MethodGet, "/app/overview", nil)
	request.SetBasicAuth("legacy", "not-an-api-key")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" || response.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("unauthenticated portal status=%d challenge_present=%t", response.Code, response.Header().Get("WWW-Authenticate") != "")
	}
	unauthenticatedAPI := httptest.NewRecorder()
	app.ServeHTTP(unauthenticatedAPI, httptest.NewRequest(http.MethodGet, "/api/portal/me", nil))
	if unauthenticatedAPI.Code != http.StatusUnauthorized || unauthenticatedAPI.Header().Get("X-Alzette-Request-ID") == "" || unauthenticatedAPI.Header().Get("X-Alzette-Request-ID") != unauthenticatedAPI.Header().Get("X-Request-ID") {
		t.Fatalf("portal error correlation status=%d", unauthenticatedAPI.Code)
	}
	var errorBody api.ErrorEnvelope
	if err := json.Unmarshal(unauthenticatedAPI.Body.Bytes(), &errorBody); err != nil || errorBody.RequestID != unauthenticatedAPI.Header().Get("X-Alzette-Request-ID") {
		t.Fatal("portal error body omitted its safe correlation ID")
	}
	bad := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=alice&password=wrong"))
	badRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.ServeHTTP(bad, badRequest)
	if bad.Code != http.StatusUnauthorized || strings.Contains(bad.Body.String(), "alice") || strings.Contains(bad.Body.String(), "wrong") {
		t.Fatalf("generic login failure status=%d length=%d", bad.Code, bad.Body.Len())
	}
	good := httptest.NewRecorder()
	goodRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=alice&password=valid-human-password"))
	goodRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.ServeHTTP(good, goodRequest)
	if good.Code != http.StatusSeeOther || len(good.Result().Cookies()) != 2 {
		t.Fatalf("login status=%d cookie_count=%d", good.Code, len(good.Result().Cookies()))
	}
	for _, cookie := range good.Result().Cookies() {
		if cookie.SameSite != http.SameSiteLaxMode || cookie.Secure {
			t.Fatalf("cookie %s flags are incorrect", cookie.Name)
		}
		if cookie.Name == humanauth.SessionCookieName && !cookie.HttpOnly {
			t.Fatal("session cookie is not HttpOnly")
		}
	}
	page := httptest.NewRecorder()
	app.ServeHTTP(page, authenticatedRequest(http.MethodGet, "/app/overview", ""))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), liveMarker) || strings.Contains(page.Body.String(), rawMarker) {
		t.Fatalf("authenticated page status=%d length=%d", page.Code, page.Body.Len())
	}
	if strings.Contains(page.Body.String(), testSessionToken) {
		t.Fatal("authenticated HTML leaked the session token")
	}
	if page.Header().Get("Cache-Control") != "no-store" || !strings.Contains(page.Header().Get("Vary"), "Cookie") || strings.Contains(page.Header().Get("Content-Security-Policy"), "unsafe-") {
		t.Fatal("session cache or CSP policy is unsafe")
	}
}

func TestOverviewIsServerRenderedEscapedAndUsableWithoutJavaScript(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	store.session.Current.OrganisationName = `<script>alert("org")</script>`
	store.session.Current.ProjectName = `Payments & Treasury`
	store.session.Current.EnvironmentName = `PoC <one>`
	store.observations = []platform.PortalObservation{{
		ModelAlias: "chat", ExecutionClass: "external_pilot", CapacityMode: "shared", RegistryStatus: "enabled",
		StatusDetail: "do-not-render-provider-secret", ProbeEnabled: true, ProbeStatus: "ready", Freshness: "fresh", FreshUntil: timePointer(now.Add(time.Minute)),
	}}
	app := newTestApp(t, store, false)

	response := httptest.NewRecorder()
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/app/overview", ""))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("overview status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	for _, required := range []string{
		`<h1 id="overview-title">Service overview</h1>`, `href="/app/models"`, `href="/app/usage"`,
		`method="post" action="/logout"`, `name="_csrf" value="` + testCSRFToken + `"`,
		`Payments &amp; Treasury`, `PoC &lt;one&gt;`, `Ready`, `Shared external execution`, `Shared service`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("server-rendered overview missing %q", required)
		}
	}
	for _, forbidden := range []string{`<script`, `/portal.js`, `<script>alert("org")</script>`, testSessionToken, "do-not-render-provider-secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("server-rendered overview contains forbidden value %q", forbidden)
		}
	}
	if !strings.Contains(body, `&lt;script&gt;alert`) {
		t.Fatal("template did not escape the organisation name")
	}
	if csp := response.Header().Get("Content-Security-Policy"); csp == "" || strings.Contains(csp, "unsafe-") {
		t.Fatalf("overview CSP is unsafe: %q", csp)
	}

	legacy := httptest.NewRecorder()
	app.ServeHTTP(legacy, authenticatedRequest(http.MethodGet, "/app/models", ""))
	if legacy.Code != http.StatusOK || !strings.Contains(legacy.Body.String(), "portal.js") {
		t.Fatal("non-Overview portal route no longer uses the existing shell")
	}
}

func TestOverviewPreservesTruthfulZeroAndUnknownTokenStates(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	app := newTestApp(t, store, false)

	zero := httptest.NewRecorder()
	app.ServeHTTP(zero, authenticatedRequest(http.MethodGet, "/app/overview", ""))
	zeroBody := zero.Body.String()
	for _, required := range []string{
		`<div><dt>Logical requests</dt><dd>0</dd></div>`,
		`<div><dt>Tokens / finality</dt><dd>0 / Not applicable</dd></div>`,
		`No inference requests were recorded for this project/environment.`,
		`<div><dt>Finality</dt><dd>Final</dd></div>`,
		`<div><dt>Freshness</dt><dd>Fresh</dd></div>`,
	} {
		if !strings.Contains(zeroBody, required) {
			t.Fatalf("zero overview missing %q", required)
		}
	}

	start := now.Add(-time.Minute)
	if err := store.CreateInferenceRequest(context.Background(), platform.RequestStart{ID: "req_failed", Principal: store.session.Current.Principal(), ModelAlias: "chat", StartedAt: start}); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteInferenceRequest(context.Background(), platform.RequestFinish{ID: "req_failed", CompletedAt: now, Status: "failed", HTTPStatus: 503, ErrorClass: "upstream_unavailable", Duration: time.Second, UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	unknown := httptest.NewRecorder()
	app.ServeHTTP(unknown, authenticatedRequest(http.MethodGet, "/app/overview", ""))
	unknownBody := unknown.Body.String()
	for _, required := range []string{
		`<div><dt>Logical requests</dt><dd>1</dd></div>`,
		`<div><dt>Successful requests</dt><dd>0</dd></div>`,
		`<div><dt>Tokens / finality</dt><dd>Unknown / Unknown</dd></div>`,
	} {
		if !strings.Contains(unknownBody, required) {
			t.Fatalf("unknown-token overview missing %q", required)
		}
	}
	if strings.Contains(unknownBody, `No inference requests were recorded for this project/environment.`) {
		t.Fatal("failed-only usage was presented as an empty ledger")
	}
}

func TestOverviewNativeLogoutRequiresMatchingFormCSRF(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	app := newTestApp(t, store, false)

	bad := authenticatedRequest(http.MethodPost, "/logout", "_csrf=wrong")
	bad.Header.Del("X-CSRF-Token")
	bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badResponse := httptest.NewRecorder()
	app.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusForbidden || store.revoked {
		t.Fatalf("invalid form CSRF status=%d revoked=%t", badResponse.Code, store.revoked)
	}

	valid := authenticatedRequest(http.MethodPost, "/logout", "_csrf="+testCSRFToken)
	valid.Header.Del("X-CSRF-Token")
	valid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validResponse := httptest.NewRecorder()
	app.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusSeeOther || validResponse.Header().Get("Location") != "/login" || !store.revoked {
		t.Fatalf("native logout status=%d location=%q revoked=%t", validResponse.Code, validResponse.Header().Get("Location"), store.revoked)
	}
}

func TestOverviewDoesNotPresentFreshDegradedProbeAsReady(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	evidence := routeEvidenceView(platform.PortalObservation{
		RegistryStatus: "enabled", ProbeEnabled: true, ProbeStatus: "degraded", Freshness: "fresh", FreshUntil: timePointer(now.Add(time.Minute)),
	}, now)
	if evidence.StateLabel != "Degraded" || evidence.StatusLabel != "Degraded" || evidence.Signal != "!" {
		t.Fatalf("fresh degraded probe was presented as ready: %#v", evidence)
	}
}

func TestOverviewDoesNotConflateInferenceAndRouteObservationTimes(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	inferenceAt := now.Add(-2 * time.Minute)
	view := newOverviewRouteView(portalDashboard{Routes: []platform.PortalObservation{{
		ModelAlias: "chat", RegistryStatus: "enabled", LatestInferenceStatus: "succeeded", LatestInferenceAt: &inferenceAt,
	}}}, now, "")
	if view.LastSuccess != formatOverviewTime(inferenceAt, "Unknown").Text {
		t.Fatalf("latest successful inference was not preserved as latest success: %q", view.LastSuccess)
	}
	if view.LastObservation != "Unknown" {
		t.Fatalf("inference timestamp was conflated with route observation evidence: %q", view.LastObservation)
	}
}

func TestOverviewToleratesRollupFailureWhileDashboardAPIRemainsStrict(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	store.rollupErr = errors.New("rollup store unavailable")
	store.plan = platform.PortalServicePlan{Available: true, ModelAlias: "chat", CapacityMode: "shared", Source: "operator-contract", Finality: "declared"}
	observedAt, freshUntil := now.Add(-time.Minute), now.Add(time.Minute)
	store.observations = []platform.PortalObservation{{
		ModelAlias: "chat", ExecutionClass: "external_pilot", RegistryStatus: "enabled", ProbeEnabled: true,
		ProbeStatus: "ready", ObservedAt: &observedAt, FreshUntil: &freshUntil, Freshness: "fresh",
	}}
	app := newTestApp(t, store, false)

	overview := httptest.NewRecorder()
	app.ServeHTTP(overview, authenticatedRequest(http.MethodGet, "/app/overview", ""))
	body := overview.Body.String()
	if overview.Code != http.StatusOK {
		t.Fatalf("rollup-only Overview failure status=%d", overview.Code)
	}
	for _, required := range []string{
		`<meta name="alzette-api-mode" content="partial">`,
		`data-api-state="partial"`,
		`class="status-dot" data-state="partial"`,
		`Partial scoped ledger`,
		`Usage rollups are temporarily unavailable`,
		`<div><dt>Tokens / finality</dt><dd>0 / Not applicable</dd></div>`,
		`Ready`,
		`Shared external execution`,
		`Shared service`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("partial Overview missing %q", required)
		}
	}
	if strings.Contains(body, "Portal data unavailable") {
		t.Fatal("rollup-only failure blanked independently available Overview data")
	}

	dashboard := httptest.NewRecorder()
	app.ServeHTTP(dashboard, authenticatedRequest(http.MethodGet, "/api/portal/dashboard", ""))
	if dashboard.Code != http.StatusServiceUnavailable {
		t.Fatalf("strict dashboard rollup failure status=%d", dashboard.Code)
	}
	var envelope api.ErrorEnvelope
	if err := json.Unmarshal(dashboard.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "rollups_unavailable" || envelope.Error.Type != "api_error" || envelope.Error.Message != "usage rollups are temporarily unavailable" {
		t.Fatalf("strict dashboard error contract changed: %#v", envelope.Error)
	}
}

func TestOverviewSourceStateDrivesPageMetadataAndRailIndicator(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	session := newPortalStub(now).session
	renderer, err := newOverviewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		source    portalSource
		wantState string
		wantBadge string
	}{
		{"unavailable", portalSource{Label: "Unavailable", Freshness: "unavailable", Finality: "unknown"}, "unavailable", "Data unavailable"},
		{"partial", portalSource{Label: "Partial", Freshness: "fresh", Finality: "partial", AsOf: now}, "partial", "Partial scoped ledger"},
		{"stale", portalSource{Label: "Stale", Freshness: "stale", Finality: "final", AsOf: now}, "stale", "Stale scoped ledger"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := newOverviewPageView(session, portalDashboard{Source: test.source, Rollups: portalRollupSeries{Freshness: "fresh"}}, "", now, "")
			contents, err := renderer.render(view)
			if err != nil {
				t.Fatal(err)
			}
			body := string(contents)
			for _, required := range []string{
				`<meta name="alzette-api-mode" content="` + test.wantState + `">`,
				`data-api-state="` + test.wantState + `"`,
				`class="status-dot" data-state="` + test.wantState + `"`,
				test.wantBadge,
			} {
				if !strings.Contains(body, required) {
					t.Fatalf("%s Overview missing %q", test.name, required)
				}
			}
		})
	}
}

func TestOverviewAmbiguousRoutesRequireModelChoice(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	store.observations = []platform.PortalObservation{{ModelAlias: "chat-a"}, {ModelAlias: "chat-b"}}
	app := newTestApp(t, store, false)

	response := httptest.NewRecorder()
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/app/overview", ""))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `<a class="button button--dark" href="/app/models">Choose a model`) {
		t.Fatalf("ambiguous Overview did not provide the safe model handoff; status=%d", response.Code)
	}
	if strings.Contains(body, `Make a first call`) {
		t.Fatal("ambiguous Overview retained the first-call primary CTA")
	}
}

func TestPortalRootCSSAndPasswordReauthentication(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	app := newTestApp(t, store, false)

	css := httptest.NewRecorder()
	app.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/portal.css", nil))
	if css.Code != http.StatusOK || !strings.Contains(css.Header().Get("Content-Type"), "text/css") || css.Body.String() != "main{}" {
		t.Fatalf("root portal CSS status=%d type=%q", css.Code, css.Header().Get("Content-Type"))
	}

	withoutCSRF := authenticatedRequest(http.MethodPost, "/api/portal/reauthenticate", `{"password":"valid-human-password"}`)
	withoutCSRF.Header.Del("X-CSRF-Token")
	denied := httptest.NewRecorder()
	app.ServeHTTP(denied, withoutCSRF)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("reauthentication without CSRF status=%d", denied.Code)
	}

	wrong := httptest.NewRecorder()
	app.ServeHTTP(wrong, authenticatedRequest(http.MethodPost, "/api/portal/reauthenticate", `{"password":"wrong-human-password"}`))
	if wrong.Code != http.StatusUnauthorized || strings.Contains(wrong.Body.String(), "alice") || strings.Contains(wrong.Body.String(), "wrong-human-password") {
		t.Fatalf("generic reauthentication failure status=%d", wrong.Code)
	}

	success := httptest.NewRecorder()
	app.ServeHTTP(success, authenticatedRequest(http.MethodPost, "/api/portal/reauthenticate", `{"password":"valid-human-password"}`))
	if success.Code != http.StatusOK || !store.session.AuthenticatedAt.Equal(now) || !strings.Contains(success.Body.String(), "alzette.portal.reauthentication.v1") {
		t.Fatalf("reauthentication status=%d authenticated_at=%s", success.Code, store.session.AuthenticatedAt)
	}
}

func TestPortalMeAccessAndStrictKeyContracts(t *testing.T) {
	store := newPortalStub(time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))
	app := newTestApp(t, store, true)
	me := httptest.NewRecorder()
	app.ServeHTTP(me, authenticatedRequest(http.MethodGet, "/api/portal/me", ""))
	if me.Code != http.StatusOK {
		t.Fatalf("me status=%d", me.Code)
	}
	var meBody map[string]interface{}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema", "user", "context", "memberships", "permissions", "csrf_token", "session", "gateway", "gateway_base_url", "allowed_scopes"} {
		if _, ok := meBody[key]; !ok {
			t.Fatalf("me contract missing %s", key)
		}
	}
	access := httptest.NewRecorder()
	app.ServeHTTP(access, authenticatedRequest(http.MethodGet, "/api/portal/access", ""))
	var accessBody map[string]interface{}
	if err := json.Unmarshal(access.Body.Bytes(), &accessBody); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema", "context", "can_manage", "permissions", "role", "allowed_scopes", "key_policy", "service_accounts"} {
		if _, ok := accessBody[key]; !ok {
			t.Fatalf("access contract missing %s", key)
		}
	}
	expires := store.now.Add(24 * time.Hour).Format(time.RFC3339)
	issueBody := `{"service_account_id":"sa_a","name":"production","scopes":["inference:write"],"expires_at":"` + expires + `"}`
	issue := httptest.NewRecorder()
	app.ServeHTTP(issue, authenticatedRequest(http.MethodPost, "/api/portal/keys/issue", issueBody))
	if issue.Code != http.StatusCreated {
		t.Fatalf("issue status=%d", issue.Code)
	}
	if strings.Count(issue.Body.String(), testOneTimeKey) != 1 {
		t.Fatalf("plaintext occurrence count=%d", strings.Count(issue.Body.String(), testOneTimeKey))
	}
	var issued map[string]interface{}
	if err := json.Unmarshal(issue.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if _, exists := issued["api_key"]; exists {
		t.Fatal("plaintext key was duplicated at the response root")
	}
	key, ok := issued["key"].(map[string]interface{})
	if !ok || key["api_key"] != testOneTimeKey {
		t.Fatal("one-time key is not present at key.api_key")
	}
	for _, field := range []string{"action", "overlap", "rotated_from_prefix"} {
		unknown := httptest.NewRecorder()
		app.ServeHTTP(unknown, authenticatedRequest(http.MethodPost, "/api/portal/keys/issue", strings.TrimSuffix(issueBody, "}")+`,"`+field+`":"unexpected"}`))
		if unknown.Code != http.StatusBadRequest {
			t.Fatalf("unknown issue field %s status=%d", field, unknown.Code)
		}
	}
	rotate := httptest.NewRecorder()
	app.ServeHTTP(rotate, authenticatedRequest(http.MethodPost, "/api/portal/keys/rotate", issueBody))
	if rotate.Code != http.StatusBadRequest {
		t.Fatalf("rotation without predecessor status=%d", rotate.Code)
	}
	validRotation := httptest.NewRecorder()
	rotationBody := strings.TrimSuffix(issueBody, "}") + `,"rotated_from_prefix":"alz_k_previous"}`
	app.ServeHTTP(validRotation, authenticatedRequest(http.MethodPost, "/api/portal/keys/rotate", rotationBody))
	if validRotation.Code != http.StatusCreated || store.lastIssue.RotatedFromPrefix != "alz_k_previous" {
		t.Fatalf("explicit rotation status=%d", validRotation.Code)
	}
	for _, field := range []string{"action", "overlap"} {
		unknown := httptest.NewRecorder()
		app.ServeHTTP(unknown, authenticatedRequest(http.MethodPost, "/api/portal/keys/rotate", strings.TrimSuffix(rotationBody, "}")+`,"`+field+`":"unexpected"}`))
		if unknown.Code != http.StatusBadRequest {
			t.Fatalf("unknown rotation field %s status=%d", field, unknown.Code)
		}
	}
	for _, field := range []string{"service_account_id", "action", "overlap"} {
		revoke := httptest.NewRecorder()
		app.ServeHTTP(revoke, authenticatedRequest(http.MethodPost, "/api/portal/keys/revoke", `{"prefix":"alz_k_once","`+field+`":"unexpected"}`))
		if revoke.Code != http.StatusBadRequest {
			t.Fatalf("unknown revoke field %s status=%d", field, revoke.Code)
		}
	}
}

func TestPortalCSRFExportPrivacyAndSpreadsheetSafety(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	store.session.Current.OrganisationName = "=formula-org"
	modelVersion, executionClass, capacityMode := "v1", "external_pilot", "shared"
	allowance, allowanceUnit, allowancePeriod := int64(100), "logical_requests", "month"
	store.exportRows = []platform.PortalExportRow{{RequestID: "req_safe", StartedAt: now.Add(-time.Hour), CompletedAt: timePointer(now), ServiceAccount: "+formula-account", ModelAlias: "@formula-model", ModelVersion: &modelVersion, ExecutedModel: "=formula-provider", ExecutionClass: &executionClass, CapacityMode: &capacityMode, Status: "succeeded", HTTPStatus: 200, DurationMS: intPointer(10), InputTokens: intPointer(1), OutputTokens: intPointer(2), UsageFinality: "final"}}
	store.plan = platform.PortalServicePlan{Available: true, Code: "pilot", Name: "Pilot", ModelAlias: "@formula-model", CapacityMode: "shared", SharedRequestAllowance: &allowance, SharedRequestAllowanceUnit: &allowanceUnit, SharedRequestAllowancePeriod: &allowancePeriod, Status: "active", Source: "operator-contract", Finality: "declared", EffectiveAt: timePointer(now.Add(-24 * time.Hour))}
	store.observations = []platform.PortalObservation{{ModelAlias: "@formula-model", ModelVersion: modelVersion, ExecutionClass: executionClass, CapacityMode: capacityMode, State: "unknown", RegistryStatus: "enabled", Source: "target_registry"}}
	app := newTestApp(t, store, false)
	noCSRF := authenticatedRequest(http.MethodGet, "/api/portal/usage/export?format=csv", "")
	noCSRF.Header.Del("X-CSRF-Token")
	denied := httptest.NewRecorder()
	app.ServeHTTP(denied, noCSRF)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("export without CSRF status=%d", denied.Code)
	}
	csvResponse := httptest.NewRecorder()
	app.ServeHTTP(csvResponse, authenticatedRequest(http.MethodGet, "/api/portal/usage/export?format=csv", ""))
	if csvResponse.Code != http.StatusOK || store.lastFormat != "csv" {
		t.Fatalf("CSV export status=%d", csvResponse.Code)
	}
	body := csvResponse.Body.String()
	for _, unsafe := range []string{"=formula-org", "+formula-account", "@formula-model", "=formula-provider"} {
		if strings.Contains(body, ","+unsafe) || strings.Contains(body, "\n"+unsafe) {
			t.Fatal("CSV contains an unsanitized formula cell")
		}
	}
	if strings.Contains(body, "provider_attempt") || strings.Contains(body, "target_id") || strings.Contains(body, "base_url") {
		t.Fatal("CSV exposed operator-only infrastructure metadata")
	}
	for _, required := range []string{"generated_at", "current_route", "current_service_plan", "shared_request_allowance", "model_version", "execution_class", "capacity_mode"} {
		if !strings.Contains(body, required) {
			t.Fatalf("CSV contract is missing %s", required)
		}
	}
	from, to := now.Add(-2*time.Hour), now.Add(-time.Hour)
	jsonTarget := "/api/portal/usage/export?format=json&from=" + from.Format(time.RFC3339) + "&to=" + to.Format(time.RFC3339)
	jsonResponse := httptest.NewRecorder()
	app.ServeHTTP(jsonResponse, authenticatedRequest(http.MethodGet, jsonTarget, ""))
	if jsonResponse.Code != http.StatusOK {
		t.Fatalf("JSON export status=%d length=%d", jsonResponse.Code, jsonResponse.Body.Len())
	}
	var envelope usageExportEnvelope
	if err := json.Unmarshal(jsonResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.GeneratedAt.Equal(now) || !envelope.Period.From.Equal(from) || !envelope.Period.To.Equal(to) || envelope.Period.Timezone != "UTC" || envelope.Context.Semantics != "current_route_and_plan_context_not_historical" || len(envelope.Context.Routes) != 1 || envelope.Context.ServicePlan.Code != "pilot" || len(envelope.Rows) != 1 {
		t.Fatal("JSON export envelope omitted scope/period/current route or plan context")
	}
	if envelope.Rows[0].ModelVersion == nil || envelope.Rows[0].ExecutionClass == nil || envelope.Rows[0].CapacityMode == nil {
		t.Fatal("JSON export omitted safe bound route attribution")
	}
	for _, forbidden := range []string{"target_id", "base_url", "secret_ref", "credential_available", "provider_attempt"} {
		if strings.Contains(jsonResponse.Body.String(), forbidden) {
			t.Fatalf("JSON export exposed forbidden field label %s", forbidden)
		}
	}
}

func TestPortalEmptyJSONExportUsesAnArray(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	app := newTestApp(t, store, false)
	store.exportRows = nil
	from := now.Add(-time.Hour)
	target := "/api/portal/usage/export?format=json&from=" + from.Format(time.RFC3339) + "&to=" + now.Format(time.RFC3339)
	response := httptest.NewRecorder()
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, target, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("empty JSON export status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Rows []platform.PortalExportRow `json:"rows"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Rows == nil || len(payload.Rows) != 0 {
		t.Fatal("empty JSON export rows must be an empty array")
	}
}

func TestPortalAggregationUnknownPartialAndDirectTrend(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Hour)
	done := start.Add(time.Second)
	in, out := int64(3), int64(2)
	requests := []platform.InferenceRequest{{ID: "req_success", ServiceAccountID: "sa_a", ModelAlias: "chat", ExecutedModel: "provider/a", StartedAt: start, CompletedAt: &done, Status: "succeeded", Duration: 100 * time.Millisecond, Usage: platform.TokenUsage{InputTokens: &in, OutputTokens: &out}, UsageFinality: "partial", AttemptCount: 2}, {ID: "req_failed", ServiceAccountID: "sa_a", ModelAlias: "chat", StartedAt: start.Add(time.Minute), CompletedAt: &done, Status: "failed", Duration: 200 * time.Millisecond, UsageFinality: "unknown", AttemptCount: 1}}
	usage, breakdowns, recent, partial := buildPortalUsage(requests, map[string]string{"sa_a": "application"}, "Inference Pilot / PoC", platform.UsageFilter{From: start.Add(-time.Hour), To: now}, now)
	if !partial || usage.LogicalRequests != 2 || usage.SuccessfulRequests != 1 || usage.FailedRequests != 1 || usage.TokenMetrics.Total.Finality != "partial" || usage.TokenMetrics.Total.Value == nil {
		t.Fatal("logical/partial aggregation is incorrect")
	}
	if len(breakdowns.Models) != 2 || breakdowns.Models[0].Alias != "chat" || len(recent) != 2 || recent[0].ExecutedModel != "provider/a" || recent[0].Project != "Inference Pilot / PoC" {
		t.Fatal("model or recent-request contract is incomplete")
	}
	trend := buildDirectTrend(requests)
	if len(trend) != 1 || trend[0].LogicalRequests != 2 || trend[0].P95LatencyMS == nil || trend[0].Finality != "partial" {
		t.Fatal("direct-ledger trend does not reconcile")
	}
}

func TestPortalExactZeroIsFreshWhenRollupWorkerIsUnavailable(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := newPortalStub(now)
	store.checkpoint = platform.RollupCheckpoint{Status: "unavailable"}
	app := newTestApp(t, store, false)
	from, to := now.Add(-2*time.Hour), now.Add(-time.Hour)
	response := httptest.NewRecorder()
	target := "/api/portal/dashboard?from=" + from.Format(time.RFC3339) + "&to=" + to.Format(time.RFC3339)
	app.ServeHTTP(response, authenticatedRequest(http.MethodGet, target, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d length=%d", response.Code, response.Body.Len())
	}
	var body struct {
		Source struct {
			Freshness string                    `json:"freshness"`
			Finality  string                    `json:"finality"`
			Detail    string                    `json:"detail"`
			Rollup    platform.RollupCheckpoint `json:"rollup"`
		} `json:"source"`
		Period  portalPeriod       `json:"period"`
		Usage   portalUsage        `json:"usage"`
		Trend   []portalTrendPoint `json:"trend"`
		Rollups portalRollupSeries `json:"rollups"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source.Freshness != "fresh" || body.Source.Finality != "final" || body.Source.Rollup.Status != "unavailable" || body.Rollups.Freshness != "unavailable" {
		t.Fatal("direct-ledger and rollup freshness were conflated")
	}
	if !body.Period.From.Equal(from) || !body.Period.To.Equal(to) || body.Period.Timezone != "UTC" {
		t.Fatal("dashboard period does not preserve the exact selected range")
	}
	if body.Usage.LogicalRequests != 0 || body.Usage.Tokens.Total == nil || *body.Usage.Tokens.Total != 0 || body.Usage.TokenMetrics.Total.Finality != "not_applicable" || len(body.Trend) != 0 {
		t.Fatal("zero logical-request period is not represented as exact zero/not-applicable")
	}
	if !strings.Contains(body.Source.Detail, "zero requests") {
		t.Fatal("zero-state source detail is not explicit")
	}
}

func TestPortalFailedOnlyTokensRemainUnknown(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	completed := now.Add(-time.Minute)
	requests := []platform.InferenceRequest{{ID: "req_failed", Status: "failed", StartedAt: now.Add(-2 * time.Minute), CompletedAt: &completed, UsageFinality: "unknown"}}
	usage, _, _, _ := buildPortalUsage(requests, nil, "Inference Pilot / PoC", platform.UsageFilter{From: now.Add(-time.Hour), To: now}, now)
	if usage.LogicalRequests != 1 || usage.Tokens.Total != nil || usage.TokenMetrics.Total.Value != nil || usage.TokenMetrics.Total.Finality != "unknown" {
		t.Fatal("failed-only token state was coerced into an observed zero")
	}
}

func TestPortalStaticAllowListAndSymlinkRejection(t *testing.T) {
	store := newPortalStub(time.Now().UTC())
	directory := t.TempDir()
	writePortalAssets(t, directory)
	if err := os.WriteFile(filepath.Join(directory, ".env"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	app, err := New(Config{Store: store, PortalStore: store, StaticDirectory: directory, SessionTTL: time.Hour, PublicGatewayURL: "https://gateway.example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/.env", "/index.html", "/../portal.html", "/app/../../portal.html", "/unknown.js"} {
		response := httptest.NewRecorder()
		app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("denied path %s status=%d", path, response.Code)
		}
	}
	for _, name := range staticAssetNames {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writePortalAssets(t, dir)
			target := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(target, []byte("outside"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			if _, err := New(Config{Store: store, PortalStore: store, StaticDirectory: dir, SessionTTL: time.Hour, PublicGatewayURL: "https://gateway.example.invalid"}); err == nil {
				t.Fatal("symlinked allow-list asset was accepted")
			}
		})
	}
}

func TestPublicGatewayValidation(t *testing.T) {
	for _, test := range []struct {
		value string
		allow bool
		valid bool
	}{{"https://gateway.example.invalid", false, true}, {"http://192.168.1.2:8080", true, true}, {"http://gateway.example.invalid", false, false}, {"https://user:pass@gateway.example.invalid", false, false}, {"https://gateway.example.invalid/path", false, false}} {
		_, _, err := validatePublicGatewayURL(test.value, test.allow)
		if (err == nil) != test.valid {
			t.Fatalf("gateway validation valid=%t", test.valid)
		}
	}
}

func timePointer(value time.Time) *time.Time { return &value }
func intPointer(value int64) *int64          { return &value }

var _ platform.PortalStore = (*portalStub)(nil)
var _ = errors.Is
