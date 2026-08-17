// Package billing defines the provider-neutral, hosted-payment boundary. It
// never accepts card data and never makes endpoint readiness a consequence of
// a provider payment response.
package billing

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"alzette/internal/platform"
)

var (
	ErrNotConfigured                = errors.New("billing provider is not configured")
	ErrInvalidEvent                 = errors.New("invalid billing event")
	ErrRecentAuthenticationRequired = errors.New("recent authentication required")
)

type Capability struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider,omitempty"`
	Mode       string `json:"mode"`
	Detail     string `json:"detail"`
}

type Invoice struct {
	Reference       string     `json:"reference,omitempty"`
	Status          string     `json:"status"`
	AmountDueMinor  int64      `json:"amount_due_minor"`
	AmountPaidMinor int64      `json:"amount_paid_minor"`
	Currency        string     `json:"currency"`
	DueAt           *time.Time `json:"due_at,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	IssuedAt        time.Time  `json:"issued_at"`
}

type Summary struct {
	Schema          string     `json:"schema"`
	Configured      bool       `json:"configured"`
	Provider        string     `json:"provider,omitempty"`
	State           string     `json:"state"`
	Detail          string     `json:"detail"`
	AccountName     string     `json:"account_name"`
	CommercialState string     `json:"commercial_state"`
	TaxStatus       string     `json:"tax_status"`
	CheckoutState   string     `json:"checkout_state"`
	ConfirmedAt     *time.Time `json:"confirmed_at,omitempty"`
	CanManage       bool       `json:"can_manage"`
	Invoices        []Invoice  `json:"invoices"`
}

type CustomerInput struct {
	OrganisationID string
	LegalName      string
	IdempotencyKey string
}

type CustomerResult struct {
	ProviderCustomerRef string
	ProviderRequestID   string
}

type CheckoutInput struct {
	PaymentRequirementID string
	EndpointID           string
	CustomerRef          string
	PriceRef             string
	Mode                 string
	Quantity             int64
	SuccessURL           string
	CancelURL            string
	IdempotencyKey       string
}

type HostedResult struct {
	ProviderSessionRef string
	URL                string
	ExpiresAt          time.Time
	ProviderRequestID  string
}

type PortalInput struct {
	CustomerRef    string
	ReturnURL      string
	IdempotencyKey string
}

type Provider interface {
	Capability() Capability
	EnsureCustomer(context.Context, CustomerInput) (CustomerResult, error)
	CreateCheckout(context.Context, CheckoutInput) (HostedResult, error)
	CreateCustomerPortal(context.Context, PortalInput) (HostedResult, error)
}

type CheckoutPlan struct {
	OrganisationID       string
	OrganisationName     string
	EndpointID           string
	PaymentRequirementID string
	Provider             string
	ProviderCustomerRef  string
	ProviderPriceRef     string
	Mode                 string
	OperationID          string
	OperationKey         string
}

type CheckoutResponse struct {
	Schema               string     `json:"schema"`
	PaymentRequirementID string     `json:"payment_requirement_id"`
	State                string     `json:"state"`
	HostedURL            string     `json:"hosted_url"`
	ExpiresAt            *time.Time `json:"expires_at"`
	PaymentConfirmed     bool       `json:"payment_confirmed"`
	RuntimeReady         bool       `json:"runtime_ready"`
}

type PortalPlan struct {
	OrganisationID      string
	ProviderCustomerRef string
	OperationKey        string
}

type PortalResponse struct {
	Schema    string     `json:"schema"`
	HostedURL string     `json:"hosted_url"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type Store interface {
	PrepareCheckout(context.Context, platform.PortalSession, string, [32]byte, time.Time) (CheckoutPlan, *CheckoutResponse, error)
	SetBillingCustomer(context.Context, platform.PortalSession, string, string, time.Time) error
	CompleteCheckout(context.Context, platform.PortalSession, CheckoutPlan, HostedResult, time.Time) error
	PrepareBillingPortal(context.Context, platform.PortalSession, [32]byte, time.Time) (PortalPlan, error)
}

type SummaryReader interface {
	GetBillingSummary(context.Context, platform.PortalSession) (Summary, error)
}

type Service struct {
	store      Store
	provider   Provider
	clock      func() time.Time
	successURL string
	cancelURL  string
	returnURL  string
}

type Config struct {
	Store      Store
	Provider   Provider
	Clock      func() time.Time
	SuccessURL string
	CancelURL  string
	ReturnURL  string
}

func New(config Config) (*Service, error) {
	if config.Store == nil || config.Provider == nil {
		return nil, errors.New("billing store and provider are required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	capability := config.Provider.Capability()
	if capability.Configured {
		for name, value := range map[string]string{"success URL": config.SuccessURL, "cancel URL": config.CancelURL, "return URL": config.ReturnURL} {
			parsed, err := url.Parse(value)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
				return nil, fmt.Errorf("billing %s must be an absolute HTTPS URL", name)
			}
		}
	}
	return &Service{store: config.Store, provider: config.Provider, clock: config.Clock, successURL: config.SuccessURL, cancelURL: config.CancelURL, returnURL: config.ReturnURL}, nil
}

func (s *Service) Capability() Capability { return s.provider.Capability() }

func (s *Service) Summary(ctx context.Context, session platform.PortalSession) (Summary, error) {
	reader, ok := s.store.(SummaryReader)
	if !ok {
		return Summary{}, platform.ErrUnavailable
	}
	result, err := reader.GetBillingSummary(ctx, session)
	if err != nil {
		return Summary{}, err
	}
	capability := s.provider.Capability()
	result.Schema = "alzette.portal.billing.v1"
	result.Configured = capability.Configured
	result.Provider = capability.Provider
	result.CanManage = session.Current.Role == platform.PortalRoleOrgAdmin && capability.Configured && result.State != ""
	if result.Invoices == nil {
		result.Invoices = []Invoice{}
	}
	if !capability.Configured {
		result.State = "not_configured"
		result.Detail = "Hosted billing is not configured for this deployment. No payment action is available."
		result.CanManage = false
	} else if result.State == "" {
		result.State = "not_started"
		result.Detail = "Billing is available, but this organisation has not started a hosted billing relationship."
	}
	return result, nil
}

func (s *Service) CreateCheckout(ctx context.Context, session platform.PortalSession, requirementID, idempotencyKey string) (CheckoutResponse, error) {
	if session.Current.Role != platform.PortalRoleOrgAdmin {
		return CheckoutResponse{}, platform.ErrForbidden
	}
	if !recent(session, s.clock().UTC()) {
		return CheckoutResponse{}, ErrRecentAuthenticationRequired
	}
	capability := s.provider.Capability()
	if !capability.Configured {
		return CheckoutResponse{}, ErrNotConfigured
	}
	digest, err := operationDigest(idempotencyKey)
	if err != nil {
		return CheckoutResponse{}, err
	}
	now := s.clock().UTC()
	plan, existing, err := s.store.PrepareCheckout(ctx, session, requirementID, digest, now)
	if err != nil {
		return CheckoutResponse{}, err
	}
	if existing != nil {
		return *existing, nil
	}
	if plan.ProviderCustomerRef == "" {
		customer, err := s.provider.EnsureCustomer(ctx, CustomerInput{OrganisationID: plan.OrganisationID, LegalName: plan.OrganisationName, IdempotencyKey: "alzette-customer-" + plan.OrganisationID})
		if err != nil {
			return CheckoutResponse{}, mapProviderError(err)
		}
		if err := s.store.SetBillingCustomer(ctx, session, customer.ProviderCustomerRef, "stripe", now); err != nil {
			return CheckoutResponse{}, err
		}
		plan.ProviderCustomerRef = customer.ProviderCustomerRef
	}
	hosted, err := s.provider.CreateCheckout(ctx, CheckoutInput{
		PaymentRequirementID: plan.PaymentRequirementID,
		EndpointID:           plan.EndpointID, CustomerRef: plan.ProviderCustomerRef,
		PriceRef: plan.ProviderPriceRef, Mode: plan.Mode, Quantity: 1,
		SuccessURL: s.successURL, CancelURL: s.cancelURL, IdempotencyKey: plan.OperationKey,
	})
	if err != nil {
		return CheckoutResponse{}, mapProviderError(err)
	}
	if err := validateHostedResult(hosted); err != nil {
		return CheckoutResponse{}, err
	}
	if err := s.store.CompleteCheckout(ctx, session, plan, hosted, now); err != nil {
		return CheckoutResponse{}, err
	}
	expires := hosted.ExpiresAt.UTC()
	return CheckoutResponse{Schema: "alzette.portal.checkout.v1", PaymentRequirementID: requirementID, State: "action_required", HostedURL: hosted.URL, ExpiresAt: &expires, PaymentConfirmed: false, RuntimeReady: false}, nil
}

func (s *Service) CreatePortal(ctx context.Context, session platform.PortalSession, idempotencyKey string) (PortalResponse, error) {
	if session.Current.Role != platform.PortalRoleOrgAdmin {
		return PortalResponse{}, platform.ErrForbidden
	}
	if !recent(session, s.clock().UTC()) {
		return PortalResponse{}, ErrRecentAuthenticationRequired
	}
	if !s.provider.Capability().Configured {
		return PortalResponse{}, ErrNotConfigured
	}
	digest, err := operationDigest(idempotencyKey)
	if err != nil {
		return PortalResponse{}, err
	}
	plan, err := s.store.PrepareBillingPortal(ctx, session, digest, s.clock().UTC())
	if err != nil {
		return PortalResponse{}, err
	}
	hosted, err := s.provider.CreateCustomerPortal(ctx, PortalInput{CustomerRef: plan.ProviderCustomerRef, ReturnURL: s.returnURL, IdempotencyKey: plan.OperationKey})
	if err != nil {
		return PortalResponse{}, mapProviderError(err)
	}
	if err := validateHostedResult(hosted); err != nil {
		return PortalResponse{}, err
	}
	expires := hosted.ExpiresAt.UTC()
	return PortalResponse{Schema: "alzette.portal.billing_session.v1", HostedURL: hosted.URL, ExpiresAt: &expires}, nil
}

func recent(session platform.PortalSession, now time.Time) bool {
	return !session.AuthenticatedAt.IsZero() && !session.AuthenticatedAt.After(now) && now.Sub(session.AuthenticatedAt) <= 15*time.Minute
}

func operationDigest(value string) ([32]byte, error) {
	if len(value) < 8 || len(value) > 128 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") {
		return [32]byte{}, platform.ErrInvalid
	}
	return sha256.Sum256([]byte(value)), nil
}

func validateHostedResult(value HostedResult) error {
	parsed, err := url.Parse(value.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || value.ProviderSessionRef == "" || value.ExpiresAt.IsZero() {
		return platform.ErrUnavailable
	}
	return nil
}

func mapProviderError(err error) error {
	if errors.Is(err, ErrNotConfigured) {
		return ErrNotConfigured
	}
	return platform.ErrUnavailable
}

// UnavailableProvider is the truthful runtime default. FakeProvider below is
// opt-in test evidence and is never silently used as customer payment truth.
type UnavailableProvider struct{}

func (UnavailableProvider) Capability() Capability {
	return Capability{Configured: false, Mode: "hosted_checkout", Detail: "Hosted billing is not configured."}
}
func (UnavailableProvider) EnsureCustomer(context.Context, CustomerInput) (CustomerResult, error) {
	return CustomerResult{}, ErrNotConfigured
}
func (UnavailableProvider) CreateCheckout(context.Context, CheckoutInput) (HostedResult, error) {
	return HostedResult{}, ErrNotConfigured
}
func (UnavailableProvider) CreateCustomerPortal(context.Context, PortalInput) (HostedResult, error) {
	return HostedResult{}, ErrNotConfigured
}

// FakeProvider is deterministic and credential-free. It supports unit and
// integration tests without representing itself as a real payment result.
type FakeProvider struct {
	mu        sync.Mutex
	Customers int
	Checkouts int
	Portals   int
	Now       func() time.Time
	Failure   error
}

func (f *FakeProvider) Capability() Capability {
	return Capability{Configured: true, Provider: "fake", Mode: "hosted_checkout", Detail: "Deterministic test billing adapter."}
}
func (f *FakeProvider) EnsureCustomer(_ context.Context, input CustomerInput) (CustomerResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Failure != nil {
		return CustomerResult{}, f.Failure
	}
	f.Customers++
	digest := sha256.Sum256([]byte(input.OrganisationID))
	return CustomerResult{ProviderCustomerRef: fmt.Sprintf("cus_test_%x", digest[:6]), ProviderRequestID: "req_fake_customer"}, nil
}
func (f *FakeProvider) CreateCheckout(_ context.Context, input CheckoutInput) (HostedResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Failure != nil {
		return HostedResult{}, f.Failure
	}
	f.Checkouts++
	now := time.Now().UTC()
	if f.Now != nil {
		now = f.Now().UTC()
	}
	digest := sha256.Sum256([]byte(input.IdempotencyKey))
	ref := fmt.Sprintf("cs_test_%x", digest[:8])
	return HostedResult{ProviderSessionRef: ref, URL: "https://checkout.example.invalid/session/" + ref, ExpiresAt: now.Add(30 * time.Minute), ProviderRequestID: "req_fake_checkout"}, nil
}
func (f *FakeProvider) CreateCustomerPortal(_ context.Context, input PortalInput) (HostedResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Failure != nil {
		return HostedResult{}, f.Failure
	}
	f.Portals++
	now := time.Now().UTC()
	if f.Now != nil {
		now = f.Now().UTC()
	}
	digest := sha256.Sum256([]byte(input.IdempotencyKey))
	ref := fmt.Sprintf("bps_test_%x", digest[:8])
	return HostedResult{ProviderSessionRef: ref, URL: "https://billing.example.invalid/session/" + ref, ExpiresAt: now.Add(30 * time.Minute), ProviderRequestID: "req_fake_portal"}, nil
}
