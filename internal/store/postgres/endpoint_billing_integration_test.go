package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alzette/internal/billing"
	"alzette/internal/catalogue"
	"alzette/internal/endpoints"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
)

func TestPostgresCatalogueEndpointAndBillingVerticalSlice(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	fixture.store.SetClock(func() time.Time { return now })
	provisioned, err := fixture.store.Provision(ctx, databaseSpec("Endpoint Tenant A", "endpoint-tenant-a"))
	if err != nil {
		t.Fatal(err)
	}
	session := endpointAdminSession(t, fixture, provisioned, "Endpoint Tenant A", "endpoint-tenant-a", "endpoint-admin-a", now)
	seed := endpointSeedSpec()
	if _, err := fixture.store.SeedCatalogue(ctx, seed); err != nil {
		t.Fatalf("seed catalogue: %v", err)
	}
	if _, err := fixture.store.SeedCatalogue(ctx, seed); err != nil {
		t.Fatalf("idempotent catalogue seed: %v", err)
	}
	provider := &billing.FakeProvider{Now: func() time.Time { return now }}
	catalogueService, err := catalogue.New(fixture.store, func() (bool, string) { return true, "fake" })
	if err != nil {
		t.Fatal(err)
	}
	models, err := catalogueService.List(ctx, session)
	if err != nil || len(models) != 1 || len(models[0].Offers) != 3 {
		t.Fatalf("catalogue models=%d err=%v", len(models), err)
	}
	for _, offer := range models[0].Offers {
		encoded := strings.ToLower(strings.Join([]string{offer.Code, offer.Name, offer.Source, offer.Availability}, " "))
		for _, forbidden := range []string{"http://", "https://", "provider/model", "openrouter", "shared-openrouter"} {
			if strings.Contains(encoded, forbidden) {
				t.Fatalf("customer catalogue exposed operator routing detail")
			}
		}
	}
	endpointService, err := endpoints.New(fixture.store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	balanced := " Balanced "
	configurationInput := endpoints.CreateInput{
		ModelSlug: "safe-chat", OfferCode: "paid-shared", ProfileCode: "shared-compatible",
		EndpointAlias: "safe-chat", CapacityUnits: 1,
		Workload: endpoints.Workload{UseCase: "  Bounded application evaluation  ", LatencyPriority: &balanced},
	}
	configuration, err := endpointService.Create(ctx, session, configurationInput, "endpoint-config-create-0001")
	if err != nil {
		t.Fatalf("create paid configuration: %v", err)
	}
	if configuration.Workload.UseCase != "Bounded application evaluation" || configuration.Workload.LatencyPriority == nil || *configuration.Workload.LatencyPriority != "balanced" {
		t.Fatalf("workload normalization=%#v", configuration.Workload)
	}
	replayedConfiguration, err := endpointService.Create(ctx, session, configurationInput, "endpoint-config-create-0001")
	if err != nil || replayedConfiguration.ID != configuration.ID {
		t.Fatalf("normalized configuration retry=%#v err=%v", replayedConfiguration, err)
	}
	restored, err := endpointService.Configuration(ctx, session, configuration.ID)
	if err != nil || restored.ID != configuration.ID || restored.EndpointAlias != configuration.EndpointAlias || restored.OfferCode != configuration.OfferCode || restored.ProfileCode != configuration.ProfileCode {
		t.Fatalf("restore scoped configuration: %#v err=%v", restored, err)
	}
	endpoint, err := endpointService.Submit(ctx, session, configuration.ID, "endpoint-submit-0001")
	if err != nil {
		t.Fatalf("submit paid endpoint: %v", err)
	}
	if endpoint.Commercial.State != "payment_action_required" || endpoint.Runtime.State != "awaiting_payment" || endpoint.RouteBound || endpoint.Callable || endpoint.PaymentRequirement == nil {
		t.Fatalf("paid endpoint advanced runtime before payment: %#v", endpoint)
	}
	billingService, err := billing.New(billing.Config{
		Store: fixture.store, Provider: provider, Clock: func() time.Time { return now },
		SuccessURL: "https://portal.example.invalid/billing/success",
		CancelURL:  "https://portal.example.invalid/billing/cancel",
		ReturnURL:  "https://portal.example.invalid/app/billing",
	})
	if err != nil {
		t.Fatal(err)
	}
	checkout, err := billingService.CreateCheckout(ctx, session, endpoint.PaymentRequirement.ID, "checkout-operation-0001")
	if err != nil {
		t.Fatalf("create fake checkout: %v", err)
	}
	if checkout.HostedURL == "" || checkout.PaymentConfirmed || checkout.RuntimeReady || provider.Checkouts != 1 {
		t.Fatalf("checkout contract=%#v calls=%d", checkout, provider.Checkouts)
	}
	paidAmount := endpoint.PaymentRequirement.AmountMinor
	periodStart, periodEnd := now, now.AddDate(0, 1, 0)
	paid := billing.Event{
		Provider: "stripe", ID: "evt_paid_endpoint_0001", Type: "invoice.paid", ObjectRef: "in_paid_endpoint_0001",
		PaymentRequirementID: endpoint.PaymentRequirement.ID, CustomerRef: fakeCustomerRef(provisioned.OrganisationID),
		SubscriptionRef: "sub_paid_endpoint_0001", InvoiceRef: "in_paid_endpoint_0001",
		AmountMinor: &paidAmount, Currency: endpoint.PaymentRequirement.Currency,
		PeriodStart: &periodStart, PeriodEnd: &periodEnd, ProviderCreatedAt: now.Add(time.Minute), SignatureVerifiedAt: now.Add(time.Minute),
	}
	paid.PayloadDigest = [32]byte{1, 2, 3}
	duplicate, processed, err := fixture.store.ReceiveBillingEvent(ctx, paid)
	if err != nil || duplicate || processed {
		t.Fatalf("receive payment duplicate=%t processed=%t err=%v", duplicate, processed, err)
	}
	if err := fixture.store.ApplyBillingEvent(ctx, paid.Provider, paid.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("apply payment: %v", err)
	}
	duplicate, processed, err = fixture.store.ReceiveBillingEvent(ctx, paid)
	if err != nil || !duplicate || !processed {
		t.Fatalf("replay payment duplicate=%t processed=%t err=%v", duplicate, processed, err)
	}
	endpoint, err = endpointService.Get(ctx, session, endpoint.ID)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Commercial.State != "paid" || endpoint.Runtime.State != "route_bound" || !endpoint.RouteBound || !endpoint.Callable || endpoint.Payment.State != "paid" {
		t.Fatalf("paid shared endpoint states=%#v", endpoint)
	}
	summary, err := billingService.Summary(ctx, session)
	if err != nil || !summary.Configured || summary.AccountName != "Endpoint Tenant A" || len(summary.Invoices) != 1 || summary.Invoices[0].Status != "paid" || summary.Invoices[0].Reference != "" {
		t.Fatalf("tenant-safe billing summary=%#v err=%v", summary, err)
	}
	olderFailure := billing.Event{Provider: "stripe", ID: "evt_old_failure_0001", Type: "invoice.payment_failed", ObjectRef: "in_old_failure_0001", PaymentRequirementID: endpoint.PaymentRequirement.ID, CustomerRef: fakeCustomerRef(provisioned.OrganisationID), InvoiceRef: "in_old_failure_0001", ProviderCreatedAt: now, SignatureVerifiedAt: now}
	olderFailure.PayloadDigest = [32]byte{4, 5, 6}
	if _, _, err := fixture.store.ReceiveBillingEvent(ctx, olderFailure); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.ApplyBillingEvent(ctx, olderFailure.Provider, olderFailure.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	endpoint, err = endpointService.Get(ctx, session, endpoint.ID)
	if err != nil || endpoint.Commercial.State != "paid" {
		t.Fatal("out-of-order failure regressed a paid entitlement")
	}
	other, err := fixture.store.Provision(ctx, databaseSpec("Endpoint Tenant B", "endpoint-tenant-b"))
	if err != nil {
		t.Fatal(err)
	}
	otherSession := endpointAdminSession(t, fixture, other, "Endpoint Tenant B", "endpoint-tenant-b", "endpoint-admin-b", now)
	if _, err := endpointService.Configuration(ctx, otherSession, configuration.ID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("other tenant restored endpoint configuration: %v", err)
	}
	if _, err := endpointService.Get(ctx, otherSession, endpoint.ID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("other tenant read endpoint: %v", err)
	}
	otherSummary, err := billingService.Summary(ctx, otherSession)
	if err != nil || len(otherSummary.Invoices) != 0 || otherSummary.AccountName != "Endpoint Tenant B" {
		t.Fatalf("other tenant billing summary leaked records: %#v err=%v", otherSummary, err)
	}
	if _, err := fixture.db.Exec(`UPDATE payment_requirements SET organisation_id=$2 WHERE id=$1`, endpoint.PaymentRequirement.ID, other.OrganisationID); err == nil {
		t.Fatal("database accepted cross-tenant payment ownership mutation")
	}
	var rawPayloadColumns int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='billing_webhook_receipts' AND column_name IN ('payload','raw_body','secret','card_number')`).Scan(&rawPayloadColumns); err != nil || rawPayloadColumns != 0 {
		t.Fatal("webhook ledger has a raw payload/secret/card column")
	}
}

func TestPostgresFreeEvaluationAndDedicatedQuoteFulfilmentRemainSeparate(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sharedSpec := databaseSpec("Endpoint Workflow", "endpoint-workflow")
	shared, err := fixture.store.Provision(ctx, sharedSpec)
	if err != nil {
		t.Fatal(err)
	}
	session := endpointAdminSession(t, fixture, shared, "Endpoint Workflow", "endpoint-workflow", "workflow-admin", now)
	if _, err := fixture.store.SeedCatalogue(ctx, endpointSeedSpec()); err != nil {
		t.Fatal(err)
	}
	service, _ := endpoints.New(fixture.store, func() time.Time { return now })
	freeConfiguration, err := service.Create(ctx, session, endpoints.CreateInput{ModelSlug: "safe-chat", OfferCode: "free-evaluation", ProfileCode: "shared-compatible", EndpointAlias: "safe-chat", CapacityUnits: 1, Workload: endpoints.Workload{UseCase: "Evaluation"}}, "free-config-operation-0001")
	if err != nil {
		t.Fatal(err)
	}
	freeEndpoint, err := service.Submit(ctx, session, freeConfiguration.ID, "free-submit-operation-0001")
	if err != nil {
		t.Fatal(err)
	}
	if freeEndpoint.Commercial.State != "not_required" || !freeEndpoint.RouteBound || !freeEndpoint.Callable || freeEndpoint.PaymentRequirement != nil {
		t.Fatal("free evaluation did not activate without inventing payment")
	}
	otherSpec := databaseSpec("Dedicated Workflow", "dedicated-workflow")
	otherShared, err := fixture.store.Provision(ctx, otherSpec)
	if err != nil {
		t.Fatal(err)
	}
	otherSession := endpointAdminSession(t, fixture, otherShared, "Dedicated Workflow", "dedicated-workflow", "dedicated-admin", now)
	if _, err := fixture.store.SeedCatalogue(ctx, endpointSeedSpec()); err != nil {
		t.Fatal(err)
	}
	dedicatedSpec := otherSpec
	dedicatedSpec.TargetName = "dedicated-workflow-target"
	dedicatedSpec.CapacityMode = "dedicated"
	dedicatedSpec.ExecutionClass = "private_compatible"
	dedicatedSpec.CapacityEvidenceRef = "operator-allocation:dedicated-workflow"
	dedicatedSpec.TargetBaseURL = "https://dedicated.example.invalid/v1"
	dedicatedSpec.ServicePlanCode = "preallocated-dedicated"
	dedicatedSpec.ServicePlanName = "Preallocated dedicated"
	dedicatedSpec.DedicatedResourceClass = "test-accelerator"
	dedicatedCount := int64(1)
	dedicatedSpec.DedicatedAcceleratorCount = &dedicatedCount
	dedicatedSpec.ServicePlanSource = "operator_allocation"
	dedicatedSpec.ServicePlanFinality = "declared"
	if _, err := fixture.store.Provision(ctx, dedicatedSpec); err != nil {
		t.Fatalf("provision dedicated target: %v", err)
	}
	dedicatedConfiguration, err := service.Create(ctx, otherSession, endpoints.CreateInput{ModelSlug: "safe-chat", OfferCode: "dedicated-compatible", ProfileCode: "dedicated-compatible", EndpointAlias: "safe-chat", CapacityUnits: 1, Workload: endpoints.Workload{UseCase: "Private workload", ExpectedConcurrency: integerPointer(2)}}, "dedicated-config-operation-0001")
	if err != nil {
		t.Fatal(err)
	}
	dedicatedEndpoint, err := service.Submit(ctx, otherSession, dedicatedConfiguration.ID, "dedicated-submit-operation-0001")
	if err != nil {
		t.Fatal(err)
	}
	if dedicatedEndpoint.Commercial.State != "quote_pending" || dedicatedEndpoint.Runtime.State != "awaiting_allocation" || dedicatedEndpoint.RouteBound {
		t.Fatal("dedicated request bypassed quote/allocation rails")
	}
	initialRequest, err := service.Request(ctx, otherSession, *dedicatedEndpoint.DeploymentRequestID)
	if err != nil || initialRequest.ModelSlug != "safe-chat" || initialRequest.ModelName != "Safe Chat" || initialRequest.EndpointAlias != "safe-chat" || initialRequest.Workload.UseCase != "Private workload" || initialRequest.Workload.ExpectedConcurrency == nil || *initialRequest.Workload.ExpectedConcurrency != 2 {
		t.Fatalf("initial deployment request lost sizing intent: %#v err=%v", initialRequest, err)
	}
	quote, err := fixture.store.IssueDeploymentQuote(ctx, endpoints.QuoteSpec{RequestID: *dedicatedEndpoint.DeploymentRequestID, Currency: "EUR", BillingPeriod: "month", RecurringUnitAmountMinor: 120000, SetupTotalAmountMinor: 10000, TaxTreatment: "exclusive", PriceFinality: "contractual", CollectionMode: "not_required", ExpiresAt: now.Add(7 * 24 * time.Hour), SourceLabel: "operator_quote", EvidenceRef: "quote-evidence:dedicated-workflow"})
	if err != nil {
		t.Fatalf("issue quote: %v", err)
	}
	accepted, err := service.AcceptQuote(ctx, otherSession, quote.ID, "accept-quote-operation-0001")
	if err != nil {
		t.Fatalf("accept quote: %v", err)
	}
	if accepted.PaymentRequirement != nil || accepted.Endpoint.Commercial.State != "quote_accepted" || accepted.Endpoint.Runtime.State != "awaiting_allocation" {
		t.Fatal("accepted no-payment quote implied paid or ready")
	}
	for _, state := range []string{"approved", "allocating", "deploying", "validating"} {
		if _, err := fixture.store.TransitionDeploymentRequest(ctx, endpoints.TransitionSpec{RequestID: *dedicatedEndpoint.DeploymentRequestID, State: state}); err != nil {
			t.Fatalf("transition %s: %v", state, err)
		}
	}
	if _, err := fixture.store.TransitionDeploymentRequest(ctx, endpoints.TransitionSpec{RequestID: *dedicatedEndpoint.DeploymentRequestID, State: "ready", TargetName: dedicatedSpec.TargetName, EvidenceRef: "validation:dedicated-workflow"}); err != nil {
		t.Fatalf("transition ready: %v", err)
	}
	dedicatedEndpoint, err = service.Get(ctx, otherSession, dedicatedEndpoint.ID)
	if err != nil || dedicatedEndpoint.Runtime.State != "ready" || !dedicatedEndpoint.RouteBound || !dedicatedEndpoint.Callable || dedicatedEndpoint.Commercial.State != "quote_accepted" {
		t.Fatalf("dedicated endpoint final state=%#v err=%v", dedicatedEndpoint, err)
	}
	contextTokens := int64(65536)
	latencyPriority := "throughput"
	capacityWorkload := endpoints.Workload{UseCase: "Quarter-end document processing", ExpectedContextTokens: &contextTokens, ExpectedConcurrency: integerPointer(4), LatencyPriority: &latencyPriority}
	capacityRequest, err := service.Capacity(ctx, otherSession, dedicatedEndpoint.ID, 2, capacityWorkload, "capacity-operation-0001")
	if err != nil {
		t.Fatalf("create capacity request: %v", err)
	}
	if capacityRequest.EndpointID != dedicatedEndpoint.ID || capacityRequest.RequestedCapacityUnits != 2 || capacityRequest.Workload.UseCase != capacityWorkload.UseCase || capacityRequest.Workload.ExpectedContextTokens == nil || *capacityRequest.Workload.ExpectedContextTokens != contextTokens || capacityRequest.Workload.ExpectedConcurrency == nil || *capacityRequest.Workload.ExpectedConcurrency != 4 || capacityRequest.Workload.LatencyPriority == nil || *capacityRequest.Workload.LatencyPriority != latencyPriority {
		t.Fatalf("capacity request did not preserve bounded sizing intent: %#v", capacityRequest)
	}
	replayedCapacity, err := service.Capacity(ctx, otherSession, dedicatedEndpoint.ID, 2, capacityWorkload, "capacity-operation-0001")
	if err != nil || replayedCapacity.ID != capacityRequest.ID {
		t.Fatalf("capacity retry created or returned a different request: %#v err=%v", replayedCapacity, err)
	}
	if _, err := service.Capacity(ctx, otherSession, dedicatedEndpoint.ID, 3, capacityWorkload, "capacity-operation-0001"); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("changed capacity payload reused an idempotency key: %v", err)
	}
	if _, err := service.Capacity(ctx, otherSession, dedicatedEndpoint.ID, 2, capacityWorkload, "capacity-operation-0002"); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("second operation key bypassed the active capacity request: %v", err)
	}
	if _, err := service.Capacity(ctx, session, dedicatedEndpoint.ID, 2, capacityWorkload, "capacity-cross-tenant-0001"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("other tenant reached dedicated capacity request: %v", err)
	}
	if _, err := fixture.db.Exec(`UPDATE deployment_requests SET workload_use_case='tampered' WHERE id=$1`, capacityRequest.ID); err == nil {
		t.Fatal("database mutated immutable capacity sizing intent")
	}
	var keyDigestLength, requestDigestLength int
	if err := fixture.db.QueryRow(`SELECT octet_length(idempotency_key_hash),octet_length(idempotency_request_hash) FROM deployment_requests WHERE id=$1`, capacityRequest.ID).Scan(&keyDigestLength, &requestDigestLength); err != nil || keyDigestLength != 32 || requestDigestLength != 32 {
		t.Fatalf("capacity idempotency digests=%d/%d err=%v", keyDigestLength, requestDigestLength, err)
	}
}

func endpointSeedSpec() catalogue.SeedSpec {
	return catalogue.SeedSpec{
		ModelAlias: "safe-chat", TargetName: "shared-openrouter", ModelSlug: "safe-chat", ModelName: "Safe Chat", ModelFamily: "reviewed-chat",
		Description: "Reviewed compatible chat release.", ReleaseVersion: "v1", LicenceName: "operator-reviewed", LicenceStatus: "approved", SupportStatus: "supported",
		SourceLabel: "operator_catalogue", EvidenceRef: "operator-catalogue:v1", SharedProfileCode: "shared-compatible", SharedProfileName: "Shared compatible", RuntimeClass: "compatible-chat",
		EvaluationOfferCode: "free-evaluation", EvaluationOfferName: "Free shared evaluation", EvaluationRequestLimit: 100, EligibleEvaluation: true, EligibleCustomer: true,
		PaidOfferCode: "paid-shared", PaidOfferName: "Paid shared", PaidCurrency: "EUR", PaidAmountMinor: 2500, PaidRequestLimit: 1000, StripePriceRef: "price_test_fixed_001",
		DedicatedProfileCode: "dedicated-compatible", DedicatedProfileName: "Dedicated compatible", DedicatedExecutionClass: "private_compatible", DedicatedRuntimeClass: "private-compatible-chat",
		AcceleratorClass: "test-accelerator", AcceleratorsPerUnit: 1, MinimumCapacityUnits: 1, MaximumCapacityUnits: 4, DedicatedCapacityFinality: "contractual", DedicatedEvidenceRef: "operator-capacity:v1",
	}
}

func endpointAdminSession(t *testing.T, fixture *databaseFixture, result platform.ProvisionResult, orgName, orgSlug, username string, now time.Time) platform.PortalSession {
	t.Helper()
	hash, err := humanauth.HashPassword("credential-neutral-endpoint-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := fixture.store.ProvisionHuman(context.Background(), platform.HumanUserSpec{Username: username, DisplayName: "Endpoint Admin", PasswordHash: hash, OrganisationSlug: orgSlug, ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleOrgAdmin})
	if err != nil {
		t.Fatal(err)
	}
	membership := platform.PortalMembership{ID: user.MembershipID, OrganisationID: result.OrganisationID, OrganisationName: orgName, OrganisationSlug: orgSlug, ProjectID: result.ProjectID, ProjectName: "Application", ProjectSlug: "application", EnvironmentID: result.EnvironmentID, EnvironmentName: "Production", EnvironmentSlug: "production", Role: platform.PortalRoleOrgAdmin}
	return platform.PortalSession{User: platform.PortalUser{ID: user.UserID, Username: username, DisplayName: "Endpoint Admin"}, Current: membership, Memberships: []platform.PortalMembership{membership}, AuthenticatedAt: now, ExpiresAt: now.Add(time.Hour)}
}

func fakeCustomerRef(organisationID string) string {
	provider := &billing.FakeProvider{}
	result, _ := provider.EnsureCustomer(context.Background(), billing.CustomerInput{OrganisationID: organisationID})
	return result.ProviderCustomerRef
}

func integerPointer(value int) *int { return &value }
