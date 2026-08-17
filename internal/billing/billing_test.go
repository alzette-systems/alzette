package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"alzette/internal/platform"
)

type billingStoreStub struct {
	plan       CheckoutPlan
	completed  HostedResult
	customer   string
	portalPlan PortalPlan
}

func (s *billingStoreStub) PrepareCheckout(context.Context, platform.PortalSession, string, [32]byte, time.Time) (CheckoutPlan, *CheckoutResponse, error) {
	return s.plan, nil, nil
}
func (s *billingStoreStub) SetBillingCustomer(_ context.Context, _ platform.PortalSession, customer, _ string, _ time.Time) error {
	s.customer = customer
	return nil
}
func (s *billingStoreStub) CompleteCheckout(_ context.Context, _ platform.PortalSession, _ CheckoutPlan, result HostedResult, _ time.Time) error {
	s.completed = result
	return nil
}
func (s *billingStoreStub) PrepareBillingPortal(context.Context, platform.PortalSession, [32]byte, time.Time) (PortalPlan, error) {
	return s.portalPlan, nil
}

func TestBillingServiceFakeProviderAndAuthorization(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	store := &billingStoreStub{plan: CheckoutPlan{OrganisationID: "org_a", OrganisationName: "Tenant A", EndpointID: "end_a", PaymentRequirementID: "pay_a", ProviderPriceRef: "price_test_fixed", Mode: "subscription", OperationID: "bcs_a", OperationKey: "checkout-operation-a"}}
	provider := &FakeProvider{Now: func() time.Time { return now }}
	service, err := New(Config{Store: store, Provider: provider, Clock: func() time.Time { return now }, SuccessURL: "https://portal.example.invalid/success", CancelURL: "https://portal.example.invalid/cancel", ReturnURL: "https://portal.example.invalid/billing"})
	if err != nil {
		t.Fatal(err)
	}
	session := platform.PortalSession{User: platform.PortalUser{ID: "usr_a"}, Current: platform.PortalMembership{OrganisationID: "org_a", Role: platform.PortalRoleOrgAdmin}, AuthenticatedAt: now.Add(-time.Minute)}
	result, err := service.CreateCheckout(context.Background(), session, "pay_a", "checkout-idempotency-a")
	if err != nil {
		t.Fatal(err)
	}
	if result.PaymentConfirmed || result.RuntimeReady || result.State != "action_required" || store.customer == "" || store.completed.ProviderSessionRef == "" || provider.Customers != 1 || provider.Checkouts != 1 {
		t.Fatalf("checkout result=%#v customers=%d checkouts=%d", result, provider.Customers, provider.Checkouts)
	}
	viewer := session
	viewer.Current.Role = platform.PortalRoleViewer
	if _, err := service.CreateCheckout(context.Background(), viewer, "pay_a", "checkout-idempotency-b"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("viewer checkout err=%v", err)
	}
	stale := session
	stale.AuthenticatedAt = now.Add(-16 * time.Minute)
	if _, err := service.CreateCheckout(context.Background(), stale, "pay_a", "checkout-idempotency-c"); !errors.Is(err, ErrRecentAuthenticationRequired) {
		t.Fatalf("stale-session checkout err=%v", err)
	}
}

func TestBillingUnavailableIsTruthfulAndConfiguredURLsRequireHTTPS(t *testing.T) {
	store := &billingStoreStub{}
	service, err := New(Config{Store: store, Provider: UnavailableProvider{}})
	if err != nil {
		t.Fatal(err)
	}
	if service.Capability().Configured {
		t.Fatal("unavailable provider claimed billing was configured")
	}
	session := platform.PortalSession{Current: platform.PortalMembership{Role: platform.PortalRoleOrgAdmin}, AuthenticatedAt: time.Now().UTC()}
	if _, err := service.CreateCheckout(context.Background(), session, "pay_a", "checkout-idempotency-a"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unavailable checkout err=%v", err)
	}
	if _, err := New(Config{Store: store, Provider: &FakeProvider{}, SuccessURL: "http://portal.example.invalid/success", CancelURL: "https://portal.example.invalid/cancel", ReturnURL: "https://portal.example.invalid/billing"}); err == nil {
		t.Fatal("configured billing accepted an insecure hosted return URL")
	}
}
