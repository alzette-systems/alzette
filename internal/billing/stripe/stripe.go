// Package stripeadapter implements billing.Provider and billing.Verifier with
// the official pinned Stripe Go SDK. Alzette is the merchant; no Connect
// account or customer-supplied Stripe credential is accepted.
package stripeadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alzette/internal/billing"

	stripe "github.com/stripe/stripe-go/v76"
	stripeclient "github.com/stripe/stripe-go/v76/client"
	"github.com/stripe/stripe-go/v76/webhook"
)

type Config struct {
	APIKey     string
	HTTPClient *http.Client
}

type Provider struct {
	api *stripeclient.API
}

func NewProvider(config Config) (*Provider, error) {
	if strings.TrimSpace(config.APIKey) == "" || strings.ContainsAny(config.APIKey, "\r\n\x00") {
		return nil, billing.ErrNotConfigured
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	backendConfig := &stripe.BackendConfig{
		HTTPClient: config.HTTPClient, EnableTelemetry: stripe.Bool(false),
		MaxNetworkRetries: stripe.Int64(2),
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
	}
	api := &stripeclient.API{}
	api.Init(config.APIKey, stripe.NewBackendsWithConfig(backendConfig))
	return &Provider{api: api}, nil
}

func (p *Provider) Capability() billing.Capability {
	return billing.Capability{Configured: true, Provider: "stripe", Mode: "hosted_checkout", Detail: "Stripe-hosted Checkout is configured."}
}

func (p *Provider) EnsureCustomer(ctx context.Context, input billing.CustomerInput) (billing.CustomerResult, error) {
	params := &stripe.CustomerParams{Params: stripe.Params{Context: ctx}, Name: stripe.String(input.LegalName)}
	params.AddMetadata("alzette_organisation_id", input.OrganisationID)
	params.SetIdempotencyKey(input.IdempotencyKey)
	result, err := p.api.Customers.New(params)
	if err != nil {
		return billing.CustomerResult{}, errors.New("stripe customer request failed")
	}
	if result == nil || !strings.HasPrefix(result.ID, "cus_") {
		return billing.CustomerResult{}, errors.New("stripe customer response was invalid")
	}
	return billing.CustomerResult{ProviderCustomerRef: result.ID, ProviderRequestID: requestID(result.APIResource)}, nil
}

func (p *Provider) CreateCheckout(ctx context.Context, input billing.CheckoutInput) (billing.HostedResult, error) {
	mode := stripe.CheckoutSessionModeSubscription
	if input.Mode == "payment" {
		mode = stripe.CheckoutSessionModePayment
	}
	params := &stripe.CheckoutSessionParams{
		Params: stripe.Params{Context: ctx}, Customer: stripe.String(input.CustomerRef),
		ClientReferenceID: stripe.String(input.PaymentRequirementID),
		SuccessURL:        stripe.String(input.SuccessURL), CancelURL: stripe.String(input.CancelURL),
		Mode:      stripe.String(string(mode)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{Price: stripe.String(input.PriceRef), Quantity: stripe.Int64(input.Quantity)}},
		Metadata: map[string]string{
			"alzette_payment_requirement_id": input.PaymentRequirementID,
			"alzette_endpoint_id":            input.EndpointID,
		},
	}
	if mode == stripe.CheckoutSessionModeSubscription {
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{Metadata: map[string]string{
			"alzette_payment_requirement_id": input.PaymentRequirementID,
			"alzette_endpoint_id":            input.EndpointID,
		}}
	} else {
		params.PaymentIntentData = &stripe.CheckoutSessionPaymentIntentDataParams{Metadata: map[string]string{
			"alzette_payment_requirement_id": input.PaymentRequirementID,
			"alzette_endpoint_id":            input.EndpointID,
		}}
	}
	params.SetIdempotencyKey(input.IdempotencyKey)
	result, err := p.api.CheckoutSessions.New(params)
	if err != nil {
		return billing.HostedResult{}, errors.New("stripe checkout request failed")
	}
	if result == nil || !strings.HasPrefix(result.ID, "cs_") || !stripeHostedURL(result.URL) || result.ExpiresAt <= 0 {
		return billing.HostedResult{}, errors.New("stripe checkout response was invalid")
	}
	return billing.HostedResult{ProviderSessionRef: result.ID, URL: result.URL, ExpiresAt: time.Unix(result.ExpiresAt, 0).UTC(), ProviderRequestID: requestID(result.APIResource)}, nil
}

func (p *Provider) CreateCustomerPortal(ctx context.Context, input billing.PortalInput) (billing.HostedResult, error) {
	params := &stripe.BillingPortalSessionParams{Params: stripe.Params{Context: ctx}, Customer: stripe.String(input.CustomerRef), ReturnURL: stripe.String(input.ReturnURL)}
	params.SetIdempotencyKey(input.IdempotencyKey)
	result, err := p.api.BillingPortalSessions.New(params)
	if err != nil {
		return billing.HostedResult{}, errors.New("stripe billing portal request failed")
	}
	if result == nil || !stripeHostedURL(result.URL) {
		return billing.HostedResult{}, errors.New("stripe billing portal response was invalid")
	}
	return billing.HostedResult{ProviderSessionRef: result.ID, URL: result.URL, ExpiresAt: time.Now().UTC().Add(5 * time.Minute), ProviderRequestID: requestID(result.APIResource)}, nil
}

func requestID(resource stripe.APIResource) string {
	if resource.LastResponse == nil {
		return ""
	}
	return resource.LastResponse.RequestID
}

func stripeHostedURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "stripe.com" || strings.HasSuffix(host, ".stripe.com")
}

type VerifierConfig struct {
	WebhookSecret string
	Tolerance     time.Duration
	AllowLive     bool
}

type Verifier struct {
	secret    string
	tolerance time.Duration
	allowLive bool
}

func NewVerifier(config VerifierConfig) (*Verifier, error) {
	if strings.TrimSpace(config.WebhookSecret) == "" || strings.ContainsAny(config.WebhookSecret, "\r\n\x00") {
		return nil, billing.ErrNotConfigured
	}
	if config.Tolerance == 0 {
		config.Tolerance = 5 * time.Minute
	}
	if config.Tolerance < time.Minute || config.Tolerance > 15*time.Minute {
		return nil, errors.New("Stripe webhook tolerance is outside supported bounds")
	}
	return &Verifier{secret: config.WebhookSecret, tolerance: config.Tolerance, allowLive: config.AllowLive}, nil
}

func (v *Verifier) Configured() bool { return v != nil && v.secret != "" }

func (v *Verifier) Verify(payload []byte, signature string, _ time.Time) (billing.Event, error) {
	if !v.Configured() {
		return billing.Event{}, billing.ErrNotConfigured
	}
	event, err := webhook.ConstructEventWithOptions(payload, signature, v.secret, webhook.ConstructEventOptions{Tolerance: v.tolerance})
	if err != nil || event.Data == nil || len(event.Data.Raw) == 0 || event.ID == "" || event.Created <= 0 {
		return billing.Event{}, billing.ErrInvalidEvent
	}
	if event.Livemode && !v.allowLive {
		return billing.Event{}, billing.ErrInvalidEvent
	}
	if !supportedEvent(string(event.Type)) {
		return billing.Event{}, billing.ErrInvalidEvent
	}
	var object eventObject
	decoder := json.NewDecoder(bytes.NewReader(event.Data.Raw))
	if err := decoder.Decode(&object); err != nil || object.ID == "" {
		return billing.Event{}, billing.ErrInvalidEvent
	}
	result := billing.Event{
		Provider: "stripe", ID: event.ID, Type: string(event.Type), ObjectRef: object.ID,
		PaymentRequirementID: object.Metadata["alzette_payment_requirement_id"],
		CustomerRef:          object.Customer.String(), SubscriptionRef: object.Subscription.String(),
		InvoiceRef: object.Invoice.String(), ObjectStatus: object.Status, PaymentStatus: object.PaymentStatus,
		Currency: strings.ToUpper(object.Currency), ProviderCreatedAt: time.Unix(event.Created, 0).UTC(),
	}
	if strings.HasPrefix(object.ID, "in_") {
		result.InvoiceRef = object.ID
	}
	if strings.HasPrefix(object.ID, "sub_") {
		result.SubscriptionRef = object.ID
	}
	if object.AmountPaid != nil && string(event.Type) == "invoice.paid" {
		result.AmountMinor = object.AmountPaid
	} else if object.AmountDue != nil && strings.HasPrefix(string(event.Type), "invoice.") {
		result.AmountMinor = object.AmountDue
	} else {
		result.AmountMinor = object.AmountTotal
	}
	if object.CurrentPeriodStart > 0 && object.CurrentPeriodEnd > object.CurrentPeriodStart {
		start, end := time.Unix(object.CurrentPeriodStart, 0).UTC(), time.Unix(object.CurrentPeriodEnd, 0).UTC()
		result.PeriodStart, result.PeriodEnd = &start, &end
	}
	result.CancelAtPeriodEnd = object.CancelAtPeriodEnd
	if err := validateEventReferences(result); err != nil {
		return billing.Event{}, err
	}
	return result, nil
}

func supportedEvent(value string) bool {
	switch value {
	case "checkout.session.completed", "checkout.session.expired",
		"invoice.finalized", "invoice.paid", "invoice.payment_failed",
		"customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted",
		"charge.refunded", "charge.dispute.created", "charge.dispute.closed":
		return true
	default:
		return false
	}
}

func validateEventReferences(event billing.Event) error {
	if !strings.HasPrefix(event.ID, "evt_") || event.ObjectRef == "" {
		return billing.ErrInvalidEvent
	}
	if event.CustomerRef != "" && !strings.HasPrefix(event.CustomerRef, "cus_") {
		return billing.ErrInvalidEvent
	}
	if event.SubscriptionRef != "" && !strings.HasPrefix(event.SubscriptionRef, "sub_") {
		return billing.ErrInvalidEvent
	}
	if event.InvoiceRef != "" && !strings.HasPrefix(event.InvoiceRef, "in_") {
		return billing.ErrInvalidEvent
	}
	if event.PaymentRequirementID != "" && !strings.HasPrefix(event.PaymentRequirementID, "pay_") {
		return billing.ErrInvalidEvent
	}
	return nil
}

type expandableRef string

func (r *expandableRef) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		*r = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*r = expandableRef(value)
		return nil
	}
	var object struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*r = expandableRef(object.ID)
	return nil
}
func (r expandableRef) String() string { return string(r) }

type eventObject struct {
	ID                 string            `json:"id"`
	Status             string            `json:"status"`
	PaymentStatus      string            `json:"payment_status"`
	Currency           string            `json:"currency"`
	Customer           expandableRef     `json:"customer"`
	Subscription       expandableRef     `json:"subscription"`
	Invoice            expandableRef     `json:"invoice"`
	Metadata           map[string]string `json:"metadata"`
	AmountTotal        *int64            `json:"amount_total"`
	AmountDue          *int64            `json:"amount_due"`
	AmountPaid         *int64            `json:"amount_paid"`
	CurrentPeriodStart int64             `json:"current_period_start"`
	CurrentPeriodEnd   int64             `json:"current_period_end"`
	CancelAtPeriodEnd  *bool             `json:"cancel_at_period_end"`
}

type DisabledVerifier struct{}

func (DisabledVerifier) Configured() bool { return false }
func (DisabledVerifier) Verify([]byte, string, time.Time) (billing.Event, error) {
	return billing.Event{}, billing.ErrNotConfigured
}

var _ billing.Provider = (*Provider)(nil)
var _ billing.Verifier = (*Verifier)(nil)

func init() {
	stripe.SetAppInfo(&stripe.AppInfo{Name: "Alzette", Version: "poc-endpoints-p0", URL: "https://alzette.systems"})
	_ = fmt.Sprintf
}
