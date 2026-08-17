// Package catalogue owns the customer-safe model and offer catalogue. It
// deliberately has no provider URL, provider model, target ID, or credential
// fields: those remain operator-side routing data.
package catalogue

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"alzette/internal/platform"
)

var safeSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type Price struct {
	Currency             string `json:"currency"`
	BillingPeriod        string `json:"billing_period"`
	RecurringAmountMinor int64  `json:"recurring_amount_minor"`
	SetupAmountMinor     int64  `json:"setup_amount_minor"`
	Finality             string `json:"finality"`
	Source               string `json:"source"`
}

type Allowance struct {
	LogicalRequests *int64 `json:"logical_requests"`
	ReportedTokens  *int64 `json:"reported_tokens"`
	Period          string `json:"period,omitempty"`
	HardLimit       bool   `json:"hard_limit"`
}

type Metric struct {
	Code             string     `json:"code"`
	Unit             string     `json:"unit"`
	Minimum          *float64   `json:"minimum"`
	Target           *float64   `json:"target"`
	Maximum          *float64   `json:"maximum"`
	PerCapacityUnit  bool       `json:"per_capacity_unit"`
	ScalesWithUnits  bool       `json:"scales_with_units"`
	Finality         string     `json:"finality"`
	Source           string     `json:"source"`
	MeasuredAt       *time.Time `json:"measured_at"`
	EvidenceProvided bool       `json:"evidence_provided"`
}

type Profile struct {
	Code                 string   `json:"code"`
	Name                 string   `json:"name"`
	ExecutionClass       string   `json:"execution_class"`
	RuntimeClass         string   `json:"runtime_class"`
	AcceleratorClass     *string  `json:"accelerator_class"`
	AcceleratorsPerUnit  *int     `json:"accelerators_per_unit"`
	AcceleratorMemoryGiB *float64 `json:"accelerator_memory_gib"`
	MinimumCapacityUnits int      `json:"minimum_capacity_units"`
	MaximumCapacityUnits int      `json:"maximum_capacity_units"`
	CapacityFinality     string   `json:"capacity_finality"`
	Source               string   `json:"source"`
	EvidenceProvided     bool     `json:"evidence_provided"`
	Metrics              []Metric `json:"metrics"`
}

type PaymentCapability struct {
	Required   bool   `json:"required"`
	Configured bool   `json:"configured"`
	Provider   string `json:"provider,omitempty"`
	Mode       string `json:"mode"`
	Detail     string `json:"detail"`
}

type Offer struct {
	Code         string            `json:"code"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Eligible     bool              `json:"eligible"`
	Availability string            `json:"availability"`
	Profile      Profile           `json:"profile"`
	Price        *Price            `json:"price"`
	Allowance    *Allowance        `json:"allowance"`
	Payment      PaymentCapability `json:"payment"`
	Source       string            `json:"source"`
	PublishedAt  time.Time         `json:"published_at"`
}

type Release struct {
	Version             string     `json:"version"`
	ContextWindowTokens *int64     `json:"context_window_tokens"`
	LicenceName         string     `json:"licence_name"`
	LicenceStatus       string     `json:"licence_status"`
	SupportStatus       string     `json:"support_status"`
	LifecycleStatus     string     `json:"lifecycle_status"`
	Source              string     `json:"source"`
	PublishedAt         *time.Time `json:"published_at"`
}

type Model struct {
	Slug          string   `json:"slug"`
	EndpointAlias string   `json:"endpoint_alias"`
	Name          string   `json:"name"`
	Family        string   `json:"family"`
	Description   string   `json:"description"`
	Modalities    []string `json:"modalities"`
	Capabilities  []string `json:"capabilities"`
	Lifecycle     string   `json:"lifecycle_status"`
	Release       Release  `json:"recommended_release"`
	Offers        []Offer  `json:"offers"`
}

type Store interface {
	ListCatalogue(context.Context, platform.PortalSession) ([]Model, error)
	GetCatalogueModel(context.Context, platform.PortalSession, string) (Model, error)
}

// BillingCapability reports runtime adapter availability without implying that
// an offer, payment, or endpoint exists.
type BillingCapability func() (configured bool, provider string)

type Service struct {
	store      Store
	capability BillingCapability
}

func New(store Store, capability BillingCapability) (*Service, error) {
	if store == nil {
		return nil, errors.New("catalogue store is required")
	}
	if capability == nil {
		capability = func() (bool, string) { return false, "" }
	}
	return &Service{store: store, capability: capability}, nil
}

func (s *Service) List(ctx context.Context, session platform.PortalSession) ([]Model, error) {
	models, err := s.store.ListCatalogue(ctx, session)
	if err != nil {
		return nil, err
	}
	s.decorate(models)
	if models == nil {
		models = []Model{}
	}
	return models, nil
}

func (s *Service) Get(ctx context.Context, session platform.PortalSession, slug string) (Model, error) {
	if !safeSlug.MatchString(slug) {
		return Model{}, platform.ErrNotFound
	}
	model, err := s.store.GetCatalogueModel(ctx, session, slug)
	if err != nil {
		return Model{}, err
	}
	models := []Model{model}
	s.decorate(models)
	return models[0], nil
}

func (s *Service) decorate(models []Model) {
	configured, provider := s.capability()
	for modelIndex := range models {
		if models[modelIndex].Modalities == nil {
			models[modelIndex].Modalities = []string{}
		}
		if models[modelIndex].Capabilities == nil {
			models[modelIndex].Capabilities = []string{}
		}
		if models[modelIndex].Offers == nil {
			models[modelIndex].Offers = []Offer{}
		}
		for offerIndex := range models[modelIndex].Offers {
			offer := &models[modelIndex].Offers[offerIndex]
			offer.Payment.Mode = "not_required"
			offer.Payment.Detail = "No payment is required for this offer."
			if offer.Kind == "shared_subscription" {
				offer.Payment.Required = true
				offer.Payment.Configured = configured
				offer.Payment.Provider = provider
				offer.Payment.Mode = "hosted_checkout"
				if configured {
					offer.Payment.Detail = "Hosted checkout is configured; payment confirmation and runtime readiness remain separate."
				} else {
					offer.Payment.Detail = "Hosted checkout is not configured for this deployment."
					offer.Availability = "payment_not_configured"
				}
			}
			if offer.Kind == "dedicated_quote" {
				offer.Payment.Required = false
				offer.Payment.Configured = configured
				offer.Payment.Provider = provider
				offer.Payment.Mode = "quote_specific"
				offer.Payment.Detail = "Payment terms are defined only by an accepted versioned quote."
			}
		}
	}
}

type SeedSpec struct {
	ModelAlias                string
	TargetName                string
	ModelSlug                 string
	ModelName                 string
	ModelFamily               string
	Description               string
	ReleaseVersion            string
	ContextWindowTokens       *int64
	LicenceName               string
	LicenceStatus             string
	SupportStatus             string
	SourceLabel               string
	EvidenceRef               string
	SharedProfileCode         string
	SharedProfileName         string
	RuntimeClass              string
	EvaluationOfferCode       string
	EvaluationOfferName       string
	EvaluationRequestLimit    int64
	EvaluationTokenLimit      *int64
	EligibleEvaluation        bool
	EligibleCustomer          bool
	PaidOfferCode             string
	PaidOfferName             string
	PaidCurrency              string
	PaidAmountMinor           int64
	PaidRequestLimit          int64
	StripePriceRef            string
	DedicatedProfileCode      string
	DedicatedProfileName      string
	DedicatedExecutionClass   string
	DedicatedRuntimeClass     string
	AcceleratorClass          string
	AcceleratorsPerUnit       int
	AcceleratorMemoryGiB      *float64
	MinimumCapacityUnits      int
	MaximumCapacityUnits      int
	DedicatedCapacityFinality string
	DedicatedEvidenceRef      string
}

type SeedResult struct {
	ModelSlug            string `json:"model_slug"`
	ReleaseVersion       string `json:"release_version"`
	SharedProfileCode    string `json:"shared_profile_code"`
	EvaluationOfferCode  string `json:"evaluation_offer_code"`
	PaidOfferCode        string `json:"paid_offer_code,omitempty"`
	DedicatedProfileCode string `json:"dedicated_profile_code,omitempty"`
	CreatedOrUpdated     bool   `json:"created_or_updated"`
}

type Provisioner interface {
	SeedCatalogue(context.Context, SeedSpec) (SeedResult, error)
}

func ValidateSeed(input SeedSpec) (SeedSpec, error) {
	input.ModelAlias = strings.TrimSpace(input.ModelAlias)
	input.TargetName = strings.TrimSpace(input.TargetName)
	input.ModelSlug = strings.TrimSpace(strings.ToLower(input.ModelSlug))
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.ModelFamily = strings.TrimSpace(input.ModelFamily)
	input.ReleaseVersion = strings.TrimSpace(input.ReleaseVersion)
	input.SourceLabel = strings.TrimSpace(input.SourceLabel)
	input.EvidenceRef = strings.TrimSpace(input.EvidenceRef)
	if input.ModelAlias == "" || input.TargetName == "" || !safeSlug.MatchString(input.ModelSlug) || input.ModelName == "" || input.ModelFamily == "" || input.ReleaseVersion == "" || input.LicenceName == "" || input.SourceLabel == "" || input.EvidenceRef == "" {
		return SeedSpec{}, platform.ErrInvalid
	}
	if !safeSlug.MatchString(input.SharedProfileCode) || !safeSlug.MatchString(input.EvaluationOfferCode) || input.SharedProfileName == "" || input.EvaluationOfferName == "" || input.RuntimeClass == "" || input.EvaluationRequestLimit <= 0 {
		return SeedSpec{}, platform.ErrInvalid
	}
	if input.LicenceStatus != "approved" && input.LicenceStatus != "restricted" {
		return SeedSpec{}, platform.ErrInvalid
	}
	if input.SupportStatus != "supported" && input.SupportStatus != "limited" {
		return SeedSpec{}, platform.ErrInvalid
	}
	if !input.EligibleEvaluation && !input.EligibleCustomer {
		return SeedSpec{}, platform.ErrInvalid
	}
	if input.PaidOfferCode != "" {
		if !safeSlug.MatchString(input.PaidOfferCode) || input.PaidOfferName == "" || input.PaidCurrency == "" || input.PaidAmountMinor < 0 || input.PaidRequestLimit <= 0 || input.StripePriceRef == "" {
			return SeedSpec{}, platform.ErrInvalid
		}
	}
	if input.DedicatedProfileCode != "" {
		if !safeSlug.MatchString(input.DedicatedProfileCode) || input.DedicatedProfileName == "" || input.DedicatedRuntimeClass == "" || input.AcceleratorClass == "" || input.AcceleratorsPerUnit <= 0 || input.MinimumCapacityUnits <= 0 || input.MaximumCapacityUnits < input.MinimumCapacityUnits || input.DedicatedEvidenceRef == "" {
			return SeedSpec{}, platform.ErrInvalid
		}
		if input.DedicatedExecutionClass != "private_compatible" && input.DedicatedExecutionClass != "meluxina" {
			return SeedSpec{}, platform.ErrInvalid
		}
	}
	return input, nil
}
