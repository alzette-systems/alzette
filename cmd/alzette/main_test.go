package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alzette/internal/api"
	"alzette/internal/platform"
	"alzette/internal/store/memory"
)

type applicationTestStore struct{ *memory.Store }

func newApplicationTestStore() *applicationTestStore {
	return &applicationTestStore{Store: memory.New()}
}
func (*applicationTestStore) CreatePortalSession(context.Context, string, string, [32]byte, time.Time, time.Time) (platform.PortalSession, error) {
	return platform.PortalSession{}, platform.ErrUnauthenticated
}
func (*applicationTestStore) AuthenticatePortalSession(context.Context, [32]byte, time.Time) (platform.PortalSession, error) {
	return platform.PortalSession{}, platform.ErrUnauthenticated
}
func (*applicationTestStore) RevokePortalSession(context.Context, [32]byte, time.Time) error {
	return platform.ErrUnauthenticated
}
func (*applicationTestStore) SwitchPortalContext(context.Context, [32]byte, string, time.Time) (platform.PortalSession, error) {
	return platform.PortalSession{}, platform.ErrForbidden
}
func (*applicationTestStore) ListPortalAccess(context.Context, platform.PortalSession) ([]platform.PortalServiceAccount, error) {
	return nil, nil
}
func (*applicationTestStore) CreatePortalServiceAccount(context.Context, platform.PortalSession, string) (platform.PortalServiceAccount, error) {
	return platform.PortalServiceAccount{}, platform.ErrForbidden
}
func (*applicationTestStore) IssuePortalKey(context.Context, platform.PortalSession, platform.PortalKeyIssueSpec) (platform.PortalKeyResult, error) {
	return platform.PortalKeyResult{}, platform.ErrForbidden
}
func (*applicationTestStore) RevokePortalKey(context.Context, platform.PortalSession, string) error {
	return platform.ErrForbidden
}
func (*applicationTestStore) GetPortalServicePlan(context.Context, platform.PortalSession, string) (platform.PortalServicePlan, error) {
	return platform.PortalServicePlan{Source: "operator_registry", Finality: "unknown"}, nil
}
func (*applicationTestStore) ListPortalExport(context.Context, platform.PortalSession, platform.UsageFilter, string) ([]platform.PortalExportRow, error) {
	return nil, nil
}
func (*applicationTestStore) ListPortalRollups(context.Context, platform.PortalSession, platform.UsageFilter) ([]platform.PortalUsageRollup, error) {
	return nil, nil
}
func (*applicationTestStore) ListPortalObservations(context.Context, platform.PortalSession, string, time.Time) ([]platform.PortalObservation, error) {
	return nil, nil
}
func (*applicationTestStore) GetRollupCheckpoint(context.Context, platform.PortalSession) (platform.RollupCheckpoint, error) {
	return platform.RollupCheckpoint{Status: "unavailable"}, nil
}

func TestCombinedHandlerExposesAuthenticatedAPIsAndSafeStaticFiles(t *testing.T) {
	directory := t.TempDir()
	writeControlSite(t, directory)
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("legacy landing page must not be served"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("secret source"), 0600); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"gen","model":"provider/model","choices":[{}]}`)
	}))
	defer upstream.Close()
	store := newApplicationTestStore()
	result, err := store.Provision(context.Background(), platform.ProvisionSpec{OrganisationName: "Tenant", OrganisationSlug: "tenant", ProjectName: "App", ProjectSlug: "app", EnvironmentName: "Prod", EnvironmentSlug: "prod", ModelAlias: "chat", ModelVersion: "v1", TargetName: "fake", ExecutionClass: "external_pilot", CapacityMode: "shared", TargetBaseURL: upstream.URL + "/v1", ProviderModel: "provider/model", SecretRef: "TEST_PROVIDER_KEY", TargetTimeout: time.Second, MaxAttempts: 1, ServiceAccount: "app", Scopes: defaultScopes()})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_KEY", "provider-secret")
	t.Setenv("ALZETTE_ALLOW_INSECURE_TARGETS", "true")
	t.Setenv("ALZETTE_PUBLIC_GATEWAY_URL", "http://127.0.0.1:8080")
	t.Setenv("ALZETTE_ALLOW_INSECURE_PUBLIC_GATEWAY", "true")
	t.Setenv("ALZETTE_PORTAL_COOKIE_SECURE", "false")
	application, err := newApplicationHandler("serve", directory, nil, store)
	if err != nil {
		t.Fatal(err)
	}
	handler := api.SecurityHeaders(application)
	root := httptest.NewRecorder()
	handler.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusSeeOther || root.Header().Get("Location") != "/app/overview" || root.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("root status=%d challenge_present=%t", root.Code, root.Header().Get("WWW-Authenticate") != "")
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), `<meta name="alzette-api-mode" content="live">`) {
		t.Fatalf("login status=%d length=%d", login.Code, login.Body.Len())
	}
	legacy := httptest.NewRecorder()
	handler.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if legacy.Code != http.StatusNotFound {
		t.Fatalf("legacy landing status=%d", legacy.Code)
	}
	private := httptest.NewRecorder()
	handler.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/go.mod", nil))
	if private.Code != http.StatusNotFound {
		t.Fatalf("private source status=%d", private.Code)
	}
	dashboardReq := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	dashboardReq.Header.Set("Authorization", "Bearer "+result.APIKey)
	dashboard := httptest.NewRecorder()
	handler.ServeHTTP(dashboard, dashboardReq)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status=%d length=%d", dashboard.Code, dashboard.Body.Len())
	}
	if dashboard.Header().Get("Content-Security-Policy") == "" || dashboard.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers are missing")
	}
}

func TestGatewayModeDoesNotServeStaticOrControlSurface(t *testing.T) {
	store := memory.New()
	handler, err := newApplicationHandler("gateway", ".", nil, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/dashboard.html", "/api/v1/usage"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status=%d", path, response.Code)
		}
	}
}

func TestGatewayModeMountsEverySupportedInferenceProtocol(t *testing.T) {
	store := memory.New()
	handler, err := newApplicationHandler("gateway", ".", nil, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
			t.Errorf("%s status=%d allow=%q", path, response.Code, response.Header().Get("Allow"))
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	store := memory.New()
	handler, err := newApplicationHandler("gateway", ".", nil, store)
	if err != nil {
		t.Fatal(err)
	}
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status=%d", health.Code)
	}
	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status=%d", ready.Code)
	}
}

func TestPublicHandlerIsStandaloneAndLinksToConfiguredPortal(t *testing.T) {
	directory := t.TempDir()
	for name, contents := range map[string]string{
		"index.html":       "public landing",
		"docs.html":        "public docs",
		"site.css":         "css",
		"alzette-mark.svg": "<svg></svg>",
		"portal.js":        "private portal asset",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	application, err := newPublicHandler(directory, "http://192.0.2.15:19081/login")
	if err != nil {
		t.Fatal(err)
	}
	handler := api.SecurityHeaders(application)
	for path, status := range map[string]int{"/": http.StatusOK, "/docs": http.StatusOK, "/healthz": http.StatusOK, "/readyz": http.StatusOK, "/portal.js": http.StatusNotFound, "/go.mod": http.StatusNotFound} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != status {
			t.Fatalf("%s status=%d, want %d", path, response.Code, status)
		}
		if response.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("%s has no content security policy", path)
		}
	}
	client := httptest.NewRecorder()
	handler.ServeHTTP(client, httptest.NewRequest(http.MethodGet, "/client", nil))
	if client.Code != http.StatusSeeOther || client.Header().Get("Location") != "http://192.0.2.15:19081/login" {
		t.Fatalf("client redirect status=%d location=%q", client.Code, client.Header().Get("Location"))
	}
}

func TestPublicPortalURLValidation(t *testing.T) {
	for _, value := range []string{"", "/login", "ftp://example.test/login", "http://user:pass@example.test/login", "https://example.test/login?next=x", "https://example.test/login#fragment"} {
		if _, err := validatePublicPortalURL(value); err == nil {
			t.Errorf("accepted unsafe portal URL %q", value)
		}
	}
	value, err := validatePublicPortalURL("https://portal.example.test")
	if err != nil || value != "https://portal.example.test/login" {
		t.Fatalf("validated URL=%q err=%v", value, err)
	}
}

func TestSlice0SmokeRejectsExternalEndpointBeforeDatabaseAccess(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	err := run([]string{"slice0-smoke", "--target-base-url", "https://openrouter.ai/api/v1"})
	if err == nil || !strings.Contains(err.Error(), "offline smoke") {
		t.Fatal("Slice 0 operator smoke did not fail closed on an external endpoint")
	}
}

func TestPortalSecurityConfigurationFailsSafe(t *testing.T) {
	t.Setenv("ALZETTE_PORTAL_COOKIE_SECURE", "definitely-not-a-boolean")
	if !envBoolDefault("ALZETTE_PORTAL_COOKIE_SECURE", true) {
		t.Fatal("invalid cookie Secure configuration disabled the secure fallback")
	}
	t.Setenv("ALZETTE_PORTAL_SESSION_TTL", "not-a-duration")
	if _, err := envDurationStrict("ALZETTE_PORTAL_SESSION_TTL", 12*time.Hour); err == nil {
		t.Fatal("malformed portal session TTL was accepted")
	}
}

func TestRealControlPortalDoesNotExposeUnverifiedHostingClaims(t *testing.T) {
	store := newApplicationTestStore()
	t.Setenv("ALZETTE_PUBLIC_GATEWAY_URL", "https://gateway.example.invalid")
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	handler, err := newApplicationHandler("control", repositoryRoot, nil, store)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/index.html", "/docs.html", "/catalog.json"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("legacy surface %s status=%d", path, response.Code)
		}
	}

	for _, path := range []string{"/login", "/login.css", "/portal.js", "/alzette-mark.svg"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("portal surface %s status=%d length=%d", path, response.Code, response.Body.Len())
		}
		body := strings.ToLower(response.Body.String())
		for label, phrase := range map[string]string{
			"MeluXina claim":                       "meluxina",
			"Luxembourg-hosted claim":              "luxembourg-hosted",
			"contractual Luxembourg hosting claim": "contractual luxembourg hosting",
			"hosted-in-Luxembourg claim":           "hosted in luxembourg",
		} {
			if strings.Contains(body, phrase) {
				t.Fatalf("portal surface %s exposed a %s", path, label)
			}
		}
	}
	for _, path := range []string{"/dashboard.html", "/dashboard.css", "/site.css", "/api/dashboard", "/portal.html"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("legacy portal surface %s status=%d", path, response.Code)
		}
	}
}

func writeControlSite(t *testing.T, directory string) {
	t.Helper()
	files := map[string]string{
		"login.html":       `<!doctype html><head><meta name="alzette-api-mode" content="fallback"></head><form></form>`,
		"login.css":        "login-css",
		"portal.html":      `<!doctype html><head><meta name="alzette-api-mode" content="fallback"></head><main></main>`,
		"portal.css":       "portal-css",
		"portal.js":        "'use strict';",
		"alzette-mark.svg": `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var _ platform.PortalStore = (*applicationTestStore)(nil)
