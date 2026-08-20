package platform

import (
	"context"
	"errors"
	"time"
)

const (
	ScopeInferenceWrite = "inference:write"
	ScopeUsageRead      = "usage:read"
	ScopeRoutesRead     = "routes:read"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
	ErrUnavailable     = errors.New("unavailable")
	ErrConflict        = errors.New("conflict")
	ErrInvalid         = errors.New("invalid")
)

type Principal struct {
	CredentialKind      string
	OrganisationID      string
	OrganisationName    string
	OrganisationSlug    string
	ProjectID           string
	ProjectName         string
	ProjectSlug         string
	EnvironmentID       string
	EnvironmentName     string
	EnvironmentSlug     string
	ServiceAccountID    string
	ServiceAccount      string
	APIKeyID            string
	KeyPrefix           string
	HumanUserID         string
	HumanMembershipID   string
	AgentGrantID        string
	AgentTokenID        string
	AllowedModelAliases []string
	Scopes              []string
}

func (p Principal) HasScope(scope string) bool {
	for _, candidate := range p.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

func (p Principal) AllowsModel(alias string) bool {
	if p.CredentialKind != "human_agent_token" {
		return true
	}
	for _, candidate := range p.AllowedModelAliases {
		if candidate == alias {
			return true
		}
	}
	return false
}

type Target struct {
	ID                  string
	Name                string
	ExecutionClass      string
	CapacityMode        string
	CapacityEvidenceRef string
	OwnerOrganisationID string
	BaseURL             string
	ProviderModel       string
	SecretRef           string
	Timeout             time.Duration
	MaxAttempts         int
	Enabled             bool
	HealthStatus        string
	LastHealthCheckAt   *time.Time
	LastSuccessAt       *time.Time
}

type Route struct {
	ID                string
	OrganisationID    string
	ProjectID         string
	EnvironmentID     string
	ModelID           string
	ModelAlias        string
	ModelVersion      string
	ModelEnabled      bool
	BindingGeneration int64 `json:"-"`
	Enabled           bool
	Target            Target
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TokenUsage struct {
	InputTokens         *int64 `json:"input_tokens"`
	OutputTokens        *int64 `json:"output_tokens"`
	TotalTokens         *int64 `json:"total_tokens"`
	CachedTokens        *int64 `json:"cached_tokens"` // provider cache reads; retained for API compatibility
	CachedWriteTokens   *int64 `json:"cached_write_tokens"`
	CachedWriteTokens5m *int64 `json:"cached_write_tokens_5m"`
	CachedWriteTokens1h *int64 `json:"cached_write_tokens_1h"`
	ReasoningTokens     *int64 `json:"reasoning_tokens"`
	TextInputTokens     *int64 `json:"text_input_tokens"`
	AudioInputTokens    *int64 `json:"audio_input_tokens"`
	ImageInputTokens    *int64 `json:"image_input_tokens"`
	Normalization       string `json:"normalization,omitempty"`
}

type InferenceRequest struct {
	ID                     string
	OrganisationID         string
	ProjectID              string
	EnvironmentID          string
	RouteID                string
	BoundTargetID          string `json:"-"`
	BoundModelID           string `json:"-"`
	RouteBindingGeneration int64  `json:"-"`
	ServiceAccountID       string
	APIKeyID               string
	KeyPrefix              string
	ModelAlias             string
	ExecutedModel          string
	ProviderRequestID      string
	StartedAt              time.Time
	CompletedAt            *time.Time
	Status                 string
	HTTPStatus             int
	ErrorClass             string
	Duration               time.Duration
	Usage                  TokenUsage
	UsageFinality          string
	AttemptCount           int
}

type ProviderAttempt struct {
	ID                 string
	InferenceRequestID string
	TargetID           string
	AttemptNumber      int
	StartedAt          time.Time
	CompletedAt        *time.Time
	Status             string
	ProviderHTTPStatus int
	ErrorClass         string
	Duration           time.Duration
	ProviderRequestID  string
	Usage              TokenUsage
	UsageFinality      string
}

type RequestStart struct {
	ID         string
	Principal  Principal
	ModelAlias string
	StartedAt  time.Time
}

type RequestFinish struct {
	ID                string
	CompletedAt       time.Time
	Status            string
	HTTPStatus        int
	ErrorClass        string
	ExecutedModel     string
	ProviderRequestID string
	Duration          time.Duration
	Usage             TokenUsage
	UsageFinality     string
}

type AttemptStart struct {
	ID                 string
	InferenceRequestID string
	TargetID           string
	AttemptNumber      int
	StartedAt          time.Time
}

type AttemptFinish struct {
	ID                 string
	CompletedAt        time.Time
	Status             string
	ProviderHTTPStatus int
	ErrorClass         string
	Duration           time.Duration
	ProviderRequestID  string
	Usage              TokenUsage
	UsageFinality      string
}

type UsageFilter struct {
	From       time.Time
	To         time.Time
	ModelAlias string
	Limit      int
}

type RequestPage struct {
	Requests  []InferenceRequest
	Truncated bool
}

type Store interface {
	Authenticate(context.Context, [32]byte) (Principal, error)
	ResolveRoute(context.Context, Principal, string) (Route, error)
	CreateInferenceRequest(context.Context, RequestStart) error
	SetInferenceRequestRoute(context.Context, string, string) error
	CompleteInferenceRequest(context.Context, RequestFinish) error
	CreateProviderAttempt(context.Context, AttemptStart) error
	CompleteProviderAttempt(context.Context, AttemptFinish) error
	UpdateTargetHealth(context.Context, string, string, time.Time, bool) error
	ListRoutes(context.Context, Principal) ([]Route, error)
	ListInferenceRequests(context.Context, Principal, UsageFilter) (RequestPage, error)
	GetInferenceRequest(context.Context, Principal, string) (InferenceRequest, error)
}

type ProvisionSpec struct {
	OrganisationName             string
	OrganisationSlug             string
	ProjectName                  string
	ProjectSlug                  string
	EnvironmentName              string
	EnvironmentSlug              string
	ModelAlias                   string
	ModelVersion                 string
	TargetName                   string
	ExecutionClass               string
	CapacityMode                 string
	CapacityEvidenceRef          string
	TargetBaseURL                string
	ProviderModel                string
	SecretRef                    string
	TargetTimeout                time.Duration
	MaxAttempts                  int
	ProbeEnabled                 bool
	ProbeInterval                time.Duration
	ServiceAccount               string
	Scopes                       []string
	ServicePlanCode              string
	ServicePlanName              string
	SharedRequestAllowance       *int64
	SharedRequestAllowancePeriod string
	SharedTokenAllowance         *int64
	SharedTokenAllowancePeriod   string
	DedicatedResourceClass       string
	DedicatedAcceleratorCount    *int64
	ServicePlanSource            string
	ServicePlanFinality          string
}

type ProvisionResult struct {
	OrganisationID   string   `json:"organisation_id"`
	ProjectID        string   `json:"project_id"`
	EnvironmentID    string   `json:"environment_id"`
	RouteID          string   `json:"route_id"`
	TargetID         string   `json:"target_id"`
	ServiceAccountID string   `json:"service_account_id"`
	KeyPrefix        string   `json:"key_prefix"`
	APIKey           string   `json:"api_key,omitempty"`
	KeyCreated       bool     `json:"key_created"`
	Scopes           []string `json:"scopes"`
	ServicePlanCode  string   `json:"service_plan_code,omitempty"`
}

type RotateKeySpec struct {
	OrganisationSlug string
	ProjectSlug      string
	EnvironmentSlug  string
	ServiceAccount   string
	Scopes           []string
}

type KeyResult struct {
	KeyPrefix string   `json:"key_prefix"`
	APIKey    string   `json:"api_key"`
	Scopes    []string `json:"scopes"`
}

type Provisioner interface {
	Provision(context.Context, ProvisionSpec) (ProvisionResult, error)
	RotateKey(context.Context, RotateKeySpec) (KeyResult, error)
	RevokeKey(context.Context, string) error
}
