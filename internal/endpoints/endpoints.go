// Package endpoints orchestrates customer endpoint intent and the separate
// configuration, commercial, payment, and runtime state rails.
package endpoints

import (
	"context"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"
	"time"

	"alzette/internal/platform"
)

const recentAuthenticationWindow = 15 * time.Minute

var (
	slugPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	aliasPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type Workload struct {
	UseCase                   string  `json:"use_case"`
	ExpectedContextTokens     *int64  `json:"expected_context_tokens"`
	ExpectedConcurrency       *int    `json:"expected_concurrency"`
	ExpectedRequestsPerMinute *int    `json:"expected_requests_per_minute"`
	LatencyPriority           *string `json:"latency_priority"`
	ExpectedMonthlyRequests   *int64  `json:"expected_monthly_requests"`
	ExpectedUserCount         *int    `json:"expected_user_count,omitempty"`
}

type Configuration struct {
	ID             string     `json:"id"`
	ModelSlug      string     `json:"model_slug"`
	ModelName      string     `json:"model_name"`
	ReleaseVersion string     `json:"release_version"`
	OfferCode      string     `json:"offer_code"`
	OfferKind      string     `json:"offer_kind"`
	ProfileCode    string     `json:"profile_code"`
	EndpointAlias  string     `json:"endpoint_alias"`
	CapacityUnits  int        `json:"capacity_units"`
	Workload       Workload   `json:"workload"`
	Status         string     `json:"status"`
	RequestID      *string    `json:"deployment_request_id"`
	CreatedAt      time.Time  `json:"created_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
}

type Rail struct {
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type PaymentRequirement struct {
	ID             string     `json:"id"`
	Purpose        string     `json:"purpose"`
	State          string     `json:"state"`
	AmountMinor    int64      `json:"amount_minor"`
	Currency       string     `json:"currency"`
	BillingPeriod  string     `json:"billing_period"`
	TaxTreatment   string     `json:"tax_treatment"`
	CollectionMode string     `json:"collection_mode"`
	PriceFinality  string     `json:"price_finality"`
	PaidAt         *time.Time `json:"paid_at"`
}

type Endpoint struct {
	ID                  string              `json:"id"`
	ConfigurationID     string              `json:"configuration_id"`
	DeploymentRequestID *string             `json:"deployment_request_id"`
	Alias               string              `json:"alias"`
	ModelSlug           string              `json:"model_slug"`
	ModelName           string              `json:"model_name"`
	ModelVersion        string              `json:"model_version"`
	ProfileCode         string              `json:"profile_code"`
	ServiceMode         string              `json:"service_mode"`
	ExecutionClass      string              `json:"execution_class"`
	ExecutionEvidenced  bool                `json:"execution_evidence_provided"`
	CapacityUnits       int                 `json:"capacity_units"`
	Allowance           *Allowance          `json:"allowance"`
	Configuration       Rail                `json:"configuration"`
	Commercial          Rail                `json:"commercial"`
	Payment             Rail                `json:"payment"`
	Runtime             Rail                `json:"runtime"`
	PaymentRequirement  *PaymentRequirement `json:"payment_requirement"`
	RouteBound          bool                `json:"route_bound"`
	Callable            bool                `json:"callable"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

type Allowance struct {
	LogicalRequestLimit *int64     `json:"logical_request_limit"`
	LogicalRequestsUsed int64      `json:"logical_requests_used"`
	ReportedTokenLimit  *int64     `json:"reported_token_limit"`
	Period              string     `json:"period"`
	PeriodStart         *time.Time `json:"period_start"`
	PeriodEnd           *time.Time `json:"period_end"`
	HardLimit           bool       `json:"hard_limit"`
}

type DeploymentRequest struct {
	ID                     string     `json:"id"`
	ConfigurationID        string     `json:"configuration_id"`
	EndpointID             string     `json:"endpoint_id"`
	Kind                   string     `json:"kind"`
	Status                 string     `json:"status"`
	ProfileCode            string     `json:"profile_code"`
	ModelSlug              string     `json:"model_slug"`
	ModelName              string     `json:"model_name"`
	ModelVersion           string     `json:"model_version"`
	EndpointAlias          string     `json:"endpoint_alias"`
	CurrentCapacityUnits   *int       `json:"current_capacity_units"`
	RequestedCapacityUnits int        `json:"requested_capacity_units"`
	Workload               Workload   `json:"workload"`
	QuoteID                *string    `json:"quote_id"`
	PaymentRequirementID   *string    `json:"payment_requirement_id"`
	SubmittedAt            *time.Time `json:"submitted_at"`
	ApprovedAt             *time.Time `json:"approved_at"`
	CompletedAt            *time.Time `json:"completed_at"`
}

type Quote struct {
	ID                        string      `json:"id"`
	DeploymentRequestID       string      `json:"deployment_request_id"`
	Version                   int         `json:"version"`
	Kind                      string      `json:"kind"`
	ProfileCode               string      `json:"profile_code"`
	CapacityUnits             int         `json:"capacity_units"`
	ServiceMode               string      `json:"service_mode"`
	ExecutionClass            string      `json:"execution_class"`
	AcceleratorClass          *string     `json:"accelerator_class"`
	AcceleratorCount          *int        `json:"accelerator_count"`
	Capacity                  interface{} `json:"capacity"`
	Currency                  string      `json:"currency"`
	BillingPeriod             string      `json:"billing_period"`
	RecurringUnitAmountMinor  int64       `json:"recurring_unit_amount_minor"`
	RecurringTotalAmountMinor int64       `json:"recurring_total_amount_minor"`
	SetupTotalAmountMinor     int64       `json:"setup_total_amount_minor"`
	TaxTreatment              string      `json:"tax_treatment"`
	PriceFinality             string      `json:"price_finality"`
	Status                    string      `json:"status"`
	CollectionMode            string      `json:"collection_mode"`
	PaymentDueDays            *int        `json:"payment_due_days"`
	Source                    string      `json:"source"`
	OfferedAt                 time.Time   `json:"offered_at"`
	ExpiresAt                 time.Time   `json:"expires_at"`
	AcceptedAt                *time.Time  `json:"accepted_at"`
}

type CreateInput struct {
	ModelSlug     string   `json:"model_slug"`
	ServiceMode   string   `json:"service_mode,omitempty"`
	OfferCode     string   `json:"offer_code"`
	ProfileCode   string   `json:"profile_code"`
	EndpointAlias string   `json:"endpoint_alias"`
	CapacityUnits int      `json:"capacity_units"`
	Workload      Workload `json:"workload"`
}

type PatchInput struct {
	CapacityUnits *int      `json:"capacity_units"`
	Workload      *Workload `json:"workload"`
}

type AcceptResult struct {
	Quote              Quote               `json:"quote"`
	Endpoint           Endpoint            `json:"endpoint"`
	PaymentRequirement *PaymentRequirement `json:"payment_requirement"`
}

type Store interface {
	ListCustomerEndpoints(context.Context, platform.PortalSession) ([]Endpoint, error)
	GetCustomerEndpoint(context.Context, platform.PortalSession, string) (Endpoint, error)
	CreateEndpointConfiguration(context.Context, platform.PortalSession, CreateInput, [32]byte) (Configuration, error)
	UpdateEndpointConfiguration(context.Context, platform.PortalSession, string, PatchInput) (Configuration, error)
	SubmitEndpointConfiguration(context.Context, platform.PortalSession, string, [32]byte, time.Time) (Endpoint, error)
	GetDeploymentRequest(context.Context, platform.PortalSession, string) (DeploymentRequest, error)
	GetDeploymentQuote(context.Context, platform.PortalSession, string) (Quote, error)
	AcceptDeploymentQuote(context.Context, platform.PortalSession, string, [32]byte, time.Time) (AcceptResult, error)
	CreateCapacityRequest(context.Context, platform.PortalSession, string, int, Workload, [32]byte, time.Time) (DeploymentRequest, error)
}

type ConfigurationReader interface {
	GetEndpointConfiguration(context.Context, platform.PortalSession, string) (Configuration, error)
}

type Service struct {
	store Store
	clock func() time.Time
}

func New(store Store, clock func() time.Time) (*Service, error) {
	if store == nil {
		return nil, errors.New("endpoint store is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: store, clock: clock}, nil
}

func (s *Service) List(ctx context.Context, session platform.PortalSession) ([]Endpoint, error) {
	values, err := s.store.ListCustomerEndpoints(ctx, session)
	if values == nil {
		values = []Endpoint{}
	}
	return values, err
}

func (s *Service) Get(ctx context.Context, session platform.PortalSession, id string) (Endpoint, error) {
	if !validID(id) {
		return Endpoint{}, platform.ErrNotFound
	}
	return s.store.GetCustomerEndpoint(ctx, session, id)
}

func (s *Service) Configuration(ctx context.Context, session platform.PortalSession, id string) (Configuration, error) {
	if !validID(id) {
		return Configuration{}, platform.ErrNotFound
	}
	reader, ok := s.store.(ConfigurationReader)
	if !ok {
		return Configuration{}, platform.ErrUnavailable
	}
	return reader.GetEndpointConfiguration(ctx, session, id)
}

func (s *Service) Create(ctx context.Context, session platform.PortalSession, input CreateInput, idempotencyKey string) (Configuration, error) {
	if !canConfigure(session.Current) {
		return Configuration{}, platform.ErrForbidden
	}
	input.ModelSlug = strings.TrimSpace(strings.ToLower(input.ModelSlug))
	input.ServiceMode = strings.TrimSpace(strings.ToLower(input.ServiceMode))
	input.OfferCode = strings.TrimSpace(strings.ToLower(input.OfferCode))
	input.ProfileCode = strings.TrimSpace(strings.ToLower(input.ProfileCode))
	input.EndpointAlias = strings.TrimSpace(input.EndpointAlias)
	input.Workload = normalizeWorkload(input.Workload)
	if !slugPattern.MatchString(input.ModelSlug) || validateWorkload(input.Workload) != nil {
		return Configuration{}, platform.ErrInvalid
	}
	managedSelection := input.ServiceMode != ""
	if managedSelection {
		if (input.ServiceMode != "shared" && input.ServiceMode != "dedicated") || input.OfferCode != "" || input.ProfileCode != "" || input.EndpointAlias != "" || input.CapacityUnits != 0 || input.Workload.ExpectedUserCount == nil {
			return Configuration{}, platform.ErrInvalid
		}
	} else if !slugPattern.MatchString(input.OfferCode) || !slugPattern.MatchString(input.ProfileCode) || !aliasPattern.MatchString(input.EndpointAlias) || input.CapacityUnits < 1 || input.CapacityUnits > 128 {
		return Configuration{}, platform.ErrInvalid
	}
	digest, err := idempotencyDigest(idempotencyKey)
	if err != nil {
		return Configuration{}, err
	}
	return s.store.CreateEndpointConfiguration(ctx, session, input, digest)
}

func (s *Service) Update(ctx context.Context, session platform.PortalSession, id string, input PatchInput, idempotencyKey string) (Configuration, error) {
	if !canConfigure(session.Current) || !validID(id) {
		return Configuration{}, platform.ErrForbidden
	}
	if _, err := idempotencyDigest(idempotencyKey); err != nil {
		return Configuration{}, err
	}
	if input.CapacityUnits == nil && input.Workload == nil {
		return Configuration{}, platform.ErrInvalid
	}
	if input.CapacityUnits != nil && (*input.CapacityUnits < 1 || *input.CapacityUnits > 128) {
		return Configuration{}, platform.ErrInvalid
	}
	if input.Workload != nil {
		normalized := normalizeWorkload(*input.Workload)
		input.Workload = &normalized
		if validateWorkload(normalized) != nil {
			return Configuration{}, platform.ErrInvalid
		}
	}
	return s.store.UpdateEndpointConfiguration(ctx, session, id, input)
}

func (s *Service) Submit(ctx context.Context, session platform.PortalSession, id, idempotencyKey string) (Endpoint, error) {
	if !canConfigure(session.Current) || !validID(id) {
		return Endpoint{}, platform.ErrForbidden
	}
	digest, err := idempotencyDigest(idempotencyKey)
	if err != nil {
		return Endpoint{}, err
	}
	return s.store.SubmitEndpointConfiguration(ctx, session, id, digest, s.clock().UTC())
}

func (s *Service) Request(ctx context.Context, session platform.PortalSession, id string) (DeploymentRequest, error) {
	if !validID(id) {
		return DeploymentRequest{}, platform.ErrNotFound
	}
	return s.store.GetDeploymentRequest(ctx, session, id)
}

func (s *Service) Quote(ctx context.Context, session platform.PortalSession, id string) (Quote, error) {
	if !validID(id) {
		return Quote{}, platform.ErrNotFound
	}
	return s.store.GetDeploymentQuote(ctx, session, id)
}

func (s *Service) AcceptQuote(ctx context.Context, session platform.PortalSession, id, idempotencyKey string) (AcceptResult, error) {
	if session.Current.Role != platform.PortalRoleOrgAdmin {
		return AcceptResult{}, platform.ErrForbidden
	}
	now := s.clock().UTC()
	if session.AuthenticatedAt.IsZero() || now.Sub(session.AuthenticatedAt) > recentAuthenticationWindow {
		return AcceptResult{}, ErrRecentAuthenticationRequired
	}
	digest, err := idempotencyDigest(idempotencyKey)
	if err != nil {
		return AcceptResult{}, err
	}
	return s.store.AcceptDeploymentQuote(ctx, session, id, digest, now)
}

func (s *Service) Capacity(ctx context.Context, session platform.PortalSession, endpointID string, units int, workload Workload, idempotencyKey string) (DeploymentRequest, error) {
	if session.Current.Role != platform.PortalRoleOrgAdmin {
		return DeploymentRequest{}, platform.ErrForbidden
	}
	workload = normalizeWorkload(workload)
	// Team size belongs to endpoint acquisition. Keep the separate capacity
	// increase contract unchanged even though both APIs share Workload.
	if workload.ExpectedUserCount != nil || !validID(endpointID) || units < 1 || units > 128 || validateWorkload(workload) != nil {
		return DeploymentRequest{}, platform.ErrInvalid
	}
	digest, err := idempotencyDigest(idempotencyKey)
	if err != nil {
		return DeploymentRequest{}, err
	}
	return s.store.CreateCapacityRequest(ctx, session, endpointID, units, workload, digest, s.clock().UTC())
}

var ErrRecentAuthenticationRequired = errors.New("recent authentication required")

func canConfigure(membership platform.PortalMembership) bool {
	return membership.Role == platform.PortalRoleOrgAdmin || membership.Role == platform.PortalRoleProjectAdmin
}

func idempotencyDigest(value string) ([32]byte, error) {
	if len(value) < 8 || len(value) > 128 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") {
		return [32]byte{}, platform.ErrInvalid
	}
	return sha256.Sum256([]byte(value)), nil
}

func validID(value string) bool {
	return len(value) >= 8 && len(value) <= 192 && !strings.ContainsAny(value, "/\\\x00\r\n")
}

func validateWorkload(value Workload) error {
	if len(value.UseCase) > 2000 {
		return platform.ErrInvalid
	}
	if value.ExpectedContextTokens != nil && (*value.ExpectedContextTokens < 1 || *value.ExpectedContextTokens > 10_000_000) {
		return platform.ErrInvalid
	}
	if value.ExpectedConcurrency != nil && (*value.ExpectedConcurrency < 1 || *value.ExpectedConcurrency > 10000) {
		return platform.ErrInvalid
	}
	if value.ExpectedRequestsPerMinute != nil && (*value.ExpectedRequestsPerMinute < 1 || *value.ExpectedRequestsPerMinute > 10_000_000) {
		return platform.ErrInvalid
	}
	if value.ExpectedMonthlyRequests != nil && (*value.ExpectedMonthlyRequests < 1 || *value.ExpectedMonthlyRequests > 1_000_000_000_000) {
		return platform.ErrInvalid
	}
	if value.ExpectedUserCount != nil && (*value.ExpectedUserCount < 1 || *value.ExpectedUserCount > 10000) {
		return platform.ErrInvalid
	}
	if value.LatencyPriority != nil {
		switch *value.LatencyPriority {
		case "balanced", "latency", "throughput":
		default:
			return platform.ErrInvalid
		}
	}
	return nil
}

func normalizeWorkload(value Workload) Workload {
	value.UseCase = strings.TrimSpace(value.UseCase)
	if value.LatencyPriority != nil {
		latency := strings.ToLower(strings.TrimSpace(*value.LatencyPriority))
		if latency == "" {
			value.LatencyPriority = nil
		} else {
			value.LatencyPriority = &latency
		}
	}
	return value
}

type QuoteSpec struct {
	RequestID                string
	Currency                 string
	BillingPeriod            string
	RecurringUnitAmountMinor int64
	SetupTotalAmountMinor    int64
	TaxTreatment             string
	PriceFinality            string
	CollectionMode           string
	PaymentDueDays           *int
	ExpiresAt                time.Time
	SourceLabel              string
	EvidenceRef              string
	CapacitySnapshot         interface{}
}

type TransitionSpec struct {
	RequestID   string
	State       string
	EvidenceRef string
	TargetName  string
}

type Operator interface {
	IssueDeploymentQuote(context.Context, QuoteSpec) (Quote, error)
	TransitionDeploymentRequest(context.Context, TransitionSpec) (DeploymentRequest, error)
}
