package portal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alzette/internal/billing"
	"alzette/internal/catalogue"
	"alzette/internal/endpoints"
	"alzette/internal/platform"
)

type endpointPortalStub struct {
	*portalStub
	created       int
	createdScope  platform.PortalMembership
	configuration endpoints.Configuration
}

func (s *endpointPortalStub) ListCatalogue(context.Context, platform.PortalSession) ([]catalogue.Model, error) {
	return []catalogue.Model{{Slug: "safe-chat", Name: "Safe Chat", Family: "reviewed", Modalities: []string{"text"}, Capabilities: []string{"chat_completions"}, Offers: []catalogue.Offer{{Code: "free-evaluation", Name: "Free evaluation", Kind: "shared_evaluation", Eligible: true, Availability: "available_to_configure", Payment: catalogue.PaymentCapability{}}}}}, nil
}
func (s *endpointPortalStub) GetCatalogueModel(_ context.Context, _ platform.PortalSession, slug string) (catalogue.Model, error) {
	values, _ := s.ListCatalogue(context.Background(), platform.PortalSession{})
	if slug != values[0].Slug {
		return catalogue.Model{}, platform.ErrNotFound
	}
	return values[0], nil
}
func (s *endpointPortalStub) ListCustomerEndpoints(context.Context, platform.PortalSession) ([]endpoints.Endpoint, error) {
	return []endpoints.Endpoint{}, nil
}
func (s *endpointPortalStub) GetCustomerEndpoint(context.Context, platform.PortalSession, string) (endpoints.Endpoint, error) {
	return endpoints.Endpoint{}, platform.ErrNotFound
}
func (s *endpointPortalStub) CreateEndpointConfiguration(_ context.Context, session platform.PortalSession, input endpoints.CreateInput, _ [32]byte) (endpoints.Configuration, error) {
	s.created++
	s.createdScope = session.Current
	if input.ServiceMode == "shared" {
		input.OfferCode = "free-evaluation"
		input.ProfileCode = "shared-compatible"
		input.EndpointAlias = "safe-chat"
		input.CapacityUnits = 1
	}
	s.configuration = endpoints.Configuration{ID: "cfg_customer_safe", ModelSlug: input.ModelSlug, ModelName: "Safe Chat", ReleaseVersion: "v1", OfferCode: input.OfferCode, OfferKind: "shared_evaluation", ProfileCode: input.ProfileCode, EndpointAlias: input.EndpointAlias, CapacityUnits: input.CapacityUnits, Workload: input.Workload, Status: "draft", CreatedAt: s.now}
	return s.configuration, nil
}
func (s *endpointPortalStub) UpdateEndpointConfiguration(context.Context, platform.PortalSession, string, endpoints.PatchInput) (endpoints.Configuration, error) {
	return s.configuration, nil
}
func (s *endpointPortalStub) GetEndpointConfiguration(_ context.Context, _ platform.PortalSession, id string) (endpoints.Configuration, error) {
	if id != s.configuration.ID {
		return endpoints.Configuration{}, platform.ErrNotFound
	}
	return s.configuration, nil
}
func (s *endpointPortalStub) SubmitEndpointConfiguration(context.Context, platform.PortalSession, string, [32]byte, time.Time) (endpoints.Endpoint, error) {
	return endpoints.Endpoint{}, nil
}
func (s *endpointPortalStub) GetDeploymentRequest(context.Context, platform.PortalSession, string) (endpoints.DeploymentRequest, error) {
	return endpoints.DeploymentRequest{}, platform.ErrNotFound
}
func (s *endpointPortalStub) GetDeploymentQuote(context.Context, platform.PortalSession, string) (endpoints.Quote, error) {
	return endpoints.Quote{}, platform.ErrNotFound
}
func (s *endpointPortalStub) AcceptDeploymentQuote(context.Context, platform.PortalSession, string, [32]byte, time.Time) (endpoints.AcceptResult, error) {
	return endpoints.AcceptResult{}, platform.ErrNotFound
}
func (s *endpointPortalStub) CreateCapacityRequest(context.Context, platform.PortalSession, string, int, endpoints.Workload, [32]byte, time.Time) (endpoints.DeploymentRequest, error) {
	return endpoints.DeploymentRequest{}, platform.ErrNotFound
}
func (s *endpointPortalStub) PrepareCheckout(context.Context, platform.PortalSession, string, [32]byte, time.Time) (billing.CheckoutPlan, *billing.CheckoutResponse, error) {
	return billing.CheckoutPlan{}, nil, platform.ErrNotFound
}
func (s *endpointPortalStub) SetBillingCustomer(context.Context, platform.PortalSession, string, string, time.Time) error {
	return nil
}
func (s *endpointPortalStub) CompleteCheckout(context.Context, platform.PortalSession, billing.CheckoutPlan, billing.HostedResult, time.Time) error {
	return nil
}
func (s *endpointPortalStub) PrepareBillingPortal(context.Context, platform.PortalSession, [32]byte, time.Time) (billing.PortalPlan, error) {
	return billing.PortalPlan{}, platform.ErrNotFound
}
func (s *endpointPortalStub) GetBillingSummary(context.Context, platform.PortalSession) (billing.Summary, error) {
	return billing.Summary{State: "active", Detail: "Scoped billing state.", AccountName: "Alzette Demo", CommercialState: "none", TaxStatus: "not_collected", CheckoutState: "none", Invoices: []billing.Invoice{}}, nil
}

func newEndpointPortalApp(t *testing.T, store *endpointPortalStub) *App {
	t.Helper()
	directory := t.TempDir()
	writePortalAssets(t, directory)
	catalogueService, err := catalogue.New(store, func() (bool, string) { return false, "" })
	if err != nil {
		t.Fatal(err)
	}
	endpointService, err := endpoints.New(store, func() time.Time { return store.now })
	if err != nil {
		t.Fatal(err)
	}
	billingService, err := billing.New(billing.Config{Store: store, Provider: billing.UnavailableProvider{}, Clock: func() time.Time { return store.now }})
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(Config{Store: store, PortalStore: store, StaticDirectory: directory, SessionTTL: time.Hour, Clock: func() time.Time { return store.now }, PublicGatewayURL: "http://127.0.0.1:8080", AllowInsecurePublicGateway: true, Catalogue: catalogueService, Endpoints: endpointService, Billing: billingService})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestPortalCatalogueEndpointContractsCSRFAndServerScope(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	base := newPortalStub(now)
	base.session.Current.Role = platform.PortalRoleOrgAdmin
	base.session.AuthenticatedAt = now.Add(-time.Minute)
	store := &endpointPortalStub{portalStub: base}
	app := newEndpointPortalApp(t, store)
	catalogueResponse := httptest.NewRecorder()
	app.ServeHTTP(catalogueResponse, authenticatedRequest(http.MethodGet, "/api/portal/catalogue/models", ""))
	if catalogueResponse.Code != http.StatusOK {
		t.Fatalf("catalogue status=%d", catalogueResponse.Code)
	}
	for _, forbidden := range []string{"target_id", "provider_model", "base_url", "secret_ref", "http://"} {
		if strings.Contains(strings.ToLower(catalogueResponse.Body.String()), forbidden) {
			t.Fatalf("catalogue response exposed an operator-only field")
		}
	}
	var catalogueBody map[string]interface{}
	if err := json.Unmarshal(catalogueResponse.Body.Bytes(), &catalogueBody); err != nil || catalogueBody["schema"] != "alzette.portal.catalogue.v1" {
		t.Fatal("catalogue response schema mismatch")
	}
	const body = `{"model_slug":"safe-chat","service_mode":"shared","workload":{"expected_user_count":20}}`
	noCSRF := authenticatedRequest(http.MethodPost, "/api/portal/endpoint-configurations", body)
	noCSRF.Header.Del("X-CSRF-Token")
	noCSRFResponse := httptest.NewRecorder()
	app.ServeHTTP(noCSRFResponse, noCSRF)
	if noCSRFResponse.Code != http.StatusForbidden || store.created != 0 {
		t.Fatalf("missing CSRF status=%d created=%d", noCSRFResponse.Code, store.created)
	}
	noIdempotency := httptest.NewRecorder()
	app.ServeHTTP(noIdempotency, authenticatedRequest(http.MethodPost, "/api/portal/endpoint-configurations", body))
	if noIdempotency.Code != http.StatusBadRequest || store.created != 0 {
		t.Fatalf("missing idempotency status=%d created=%d", noIdempotency.Code, store.created)
	}
	mixedSelection := authenticatedRequest(http.MethodPost, "/api/portal/endpoint-configurations", `{"model_slug":"safe-chat","service_mode":"shared","offer_code":"free-evaluation","workload":{"expected_user_count":20}}`)
	mixedSelection.Header.Set("Idempotency-Key", "configuration-operation-mixed")
	mixedSelectionResponse := httptest.NewRecorder()
	app.ServeHTTP(mixedSelectionResponse, mixedSelection)
	if mixedSelectionResponse.Code != http.StatusBadRequest || store.created != 0 {
		t.Fatalf("browser selected an internal offer status=%d created=%d", mixedSelectionResponse.Code, store.created)
	}
	unsafe := authenticatedRequest(http.MethodPost, "/api/portal/endpoint-configurations", strings.TrimSuffix(body, "}")+`,"target_url":"https://forbidden.invalid"}`)
	unsafe.Header.Set("Idempotency-Key", "configuration-operation-unsafe")
	unsafeResponse := httptest.NewRecorder()
	app.ServeHTTP(unsafeResponse, unsafe)
	if unsafeResponse.Code != http.StatusBadRequest || store.created != 0 {
		t.Fatalf("unsafe routing field status=%d created=%d", unsafeResponse.Code, store.created)
	}
	valid := authenticatedRequest(http.MethodPost, "/api/portal/endpoint-configurations", body)
	valid.Header.Set("Idempotency-Key", "configuration-operation-valid")
	validResponse := httptest.NewRecorder()
	app.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusCreated || store.created != 1 || store.createdScope.OrganisationID != base.session.Current.OrganisationID || store.createdScope.ProjectID != base.session.Current.ProjectID || store.createdScope.EnvironmentID != base.session.Current.EnvironmentID || store.configuration.Workload.ExpectedUserCount == nil || *store.configuration.Workload.ExpectedUserCount != 20 {
		t.Fatalf("valid create status=%d created=%d", validResponse.Code, store.created)
	}
	restored := httptest.NewRecorder()
	app.ServeHTTP(restored, authenticatedRequest(http.MethodGet, "/api/portal/endpoint-configurations/cfg_customer_safe", ""))
	if restored.Code != http.StatusOK || !strings.Contains(restored.Body.String(), `"endpoint_alias":"safe-chat"`) {
		t.Fatalf("restore configuration status=%d body=%s", restored.Code, restored.Body.String())
	}
	billingSummary := httptest.NewRecorder()
	app.ServeHTTP(billingSummary, authenticatedRequest(http.MethodGet, "/api/portal/billing", ""))
	if billingSummary.Code != http.StatusOK || strings.Contains(billingSummary.Body.String(), "customer_ref") || strings.Contains(billingSummary.Body.String(), "provider_invoice_ref") {
		t.Fatalf("billing summary status=%d", billingSummary.Code)
	}
	me := httptest.NewRecorder()
	app.ServeHTTP(me, authenticatedRequest(http.MethodGet, "/api/portal/me", ""))
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"configured":false`) {
		t.Fatal("portal identity did not expose truthful disabled billing capability")
	}
	checkout := authenticatedRequest(http.MethodPost, "/api/portal/payment-requirements/pay_customer_safe/checkout-session", `{}`)
	checkout.Header.Set("Idempotency-Key", "checkout-operation-disabled")
	checkoutResponse := httptest.NewRecorder()
	app.ServeHTTP(checkoutResponse, checkout)
	if checkoutResponse.Code != http.StatusServiceUnavailable || !strings.Contains(checkoutResponse.Body.String(), "payment_not_configured") {
		t.Fatalf("disabled checkout status=%d", checkoutResponse.Code)
	}
}
