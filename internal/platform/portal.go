package platform

import (
	"context"
	"time"
)

const (
	PortalRoleOrgAdmin     = "org_admin"
	PortalRoleProjectAdmin = "project_admin"
	PortalRoleDeveloper    = "developer"
	PortalRoleViewer       = "viewer"
)

type PortalUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type PortalMembership struct {
	ID               string `json:"id"`
	OrganisationID   string `json:"organisation_id"`
	OrganisationName string `json:"organisation_name"`
	OrganisationSlug string `json:"organisation_slug"`
	ProjectID        string `json:"project_id"`
	ProjectName      string `json:"project_name"`
	ProjectSlug      string `json:"project_slug"`
	EnvironmentID    string `json:"environment_id"`
	EnvironmentName  string `json:"environment_name"`
	EnvironmentSlug  string `json:"environment_slug"`
	Role             string `json:"role"`
}

func (m PortalMembership) Principal() Principal {
	return Principal{
		OrganisationID: m.OrganisationID, OrganisationName: m.OrganisationName, OrganisationSlug: m.OrganisationSlug,
		ProjectID: m.ProjectID, ProjectName: m.ProjectName, ProjectSlug: m.ProjectSlug,
		EnvironmentID: m.EnvironmentID, EnvironmentName: m.EnvironmentName, EnvironmentSlug: m.EnvironmentSlug,
	}
}

type PortalSession struct {
	ID              string             `json:"-"`
	User            PortalUser         `json:"user"`
	Current         PortalMembership   `json:"context"`
	Memberships     []PortalMembership `json:"memberships"`
	AuthenticatedAt time.Time          `json:"authenticated_at"`
	ExpiresAt       time.Time          `json:"expires_at"`
}

type HumanUserSpec struct {
	Username         string
	DisplayName      string
	PasswordHash     string
	OrganisationSlug string
	ProjectSlug      string
	EnvironmentSlug  string
	Role             string
}

type HumanUserResult struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	MembershipID string `json:"membership_id"`
	Created      bool   `json:"created"`
}

type PortalServicePlan struct {
	Available                    bool       `json:"available"`
	Ambiguous                    bool       `json:"ambiguous"`
	Code                         string     `json:"code,omitempty"`
	Name                         string     `json:"name,omitempty"`
	ModelAlias                   string     `json:"model_alias,omitempty"`
	CapacityMode                 string     `json:"capacity_mode,omitempty"`
	SharedRequestAllowance       *int64     `json:"shared_request_allowance"`
	SharedRequestAllowanceUnit   *string    `json:"shared_request_allowance_unit"`
	SharedRequestAllowancePeriod *string    `json:"shared_request_allowance_period"`
	SharedTokenAllowance         *int64     `json:"shared_token_allowance"`
	SharedTokenAllowanceUnit     *string    `json:"shared_token_allowance_unit"`
	SharedTokenAllowancePeriod   *string    `json:"shared_token_allowance_period"`
	DedicatedResourceClass       *string    `json:"dedicated_resource_class"`
	DedicatedAcceleratorCount    *int64     `json:"dedicated_accelerator_count"`
	Status                       string     `json:"status,omitempty"`
	Source                       string     `json:"source"`
	Finality                     string     `json:"finality"`
	EffectiveAt                  *time.Time `json:"effective_at"`
}

type PortalKeyRecord struct {
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Status     string     `json:"status"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type PortalServiceAccount struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	CreatedAt time.Time         `json:"created_at"`
	Keys      []PortalKeyRecord `json:"keys"`
}

type PortalKeyIssueSpec struct {
	ServiceAccountID  string
	Name              string
	Scopes            []string
	ExpiresAt         *time.Time
	RotatedFromPrefix string
}

type PortalKeyResult struct {
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	APIKey    string     `json:"api_key"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type PortalExportRow struct {
	RequestID       string     `json:"request_id"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	ServiceAccount  string     `json:"service_account"`
	ModelAlias      string     `json:"model_alias"`
	ModelVersion    *string    `json:"model_version"`
	ExecutedModel   string     `json:"executed_model,omitempty"`
	ExecutionClass  *string    `json:"execution_class"`
	CapacityMode    *string    `json:"capacity_mode"`
	Status          string     `json:"status"`
	HTTPStatus      int        `json:"http_status,omitempty"`
	ErrorClass      string     `json:"error_class,omitempty"`
	DurationMS      *int64     `json:"duration_ms"`
	InputTokens     *int64     `json:"input_tokens"`
	OutputTokens    *int64     `json:"output_tokens"`
	CachedTokens    *int64     `json:"cached_tokens"`
	ReasoningTokens *int64     `json:"reasoning_tokens"`
	UsageFinality   string     `json:"usage_finality"`
}

type RollupCheckpoint struct {
	Status          string     `json:"status"`
	LastStartedAt   *time.Time `json:"last_started_at"`
	LastCompletedAt *time.Time `json:"last_completed_at"`
	RangeFrom       *time.Time `json:"range_from"`
	RangeTo         *time.Time `json:"range_to"`
	SourceRows      *int64     `json:"source_rows"`
	SafeErrorClass  *string    `json:"safe_error_class"`
}

type PortalUsageRollup struct {
	BucketStart            time.Time `json:"bucket_start"`
	ServiceAccount         string    `json:"service_account"`
	ModelAlias             string    `json:"model_alias"`
	LogicalRequests        int64     `json:"logical_requests"`
	SuccessfulRequests     int64     `json:"successful_requests"`
	FailedRequests         int64     `json:"failed_requests"`
	BlockedRequests        int64     `json:"blocked_requests"`
	CancelledRequests      int64     `json:"cancelled_requests"`
	InProgressRequests     int64     `json:"in_progress_requests"`
	InputTokens            *int64    `json:"input_tokens"`
	InputKnownRequests     int64     `json:"input_known_requests"`
	OutputTokens           *int64    `json:"output_tokens"`
	OutputKnownRequests    int64     `json:"output_known_requests"`
	CachedTokens           *int64    `json:"cached_tokens"`
	CachedKnownRequests    int64     `json:"cached_known_requests"`
	ReasoningTokens        *int64    `json:"reasoning_tokens"`
	ReasoningKnownRequests int64     `json:"reasoning_known_requests"`
	TokenEligibleRequests  int64     `json:"token_eligible_requests"`
	PeakConcurrency        *int64    `json:"peak_concurrency"`
	ThroughputRPS          *float64  `json:"throughput_rps"`
	P50LatencyMS           *int64    `json:"p50_latency_ms"`
	P95LatencyMS           *int64    `json:"p95_latency_ms"`
	Source                 string    `json:"source"`
	Finality               string    `json:"finality"`
	RefreshedAt            time.Time `json:"refreshed_at"`
}

type PortalObservation struct {
	ModelAlias            string     `json:"model_alias"`
	ModelVersion          string     `json:"model_version"`
	ExecutionClass        string     `json:"execution_class"`
	CapacityMode          string     `json:"capacity_mode"`
	State                 string     `json:"state"`
	StatusDetail          string     `json:"status_detail"`
	EndpointPath          string     `json:"endpoint_path"`
	RegistryStatus        string     `json:"registry_status"`
	LatestInferenceStatus string     `json:"latest_inference_status"`
	LatestInferenceAt     *time.Time `json:"latest_inference_at"`
	LastObservationAt     *time.Time `json:"last_observation_at"`
	LastSuccessAt         *time.Time `json:"last_success_at"`
	ProbeEnabled          bool       `json:"probe_enabled"`
	ProbeStatus           string     `json:"probe_status"`
	ObservedAt            *time.Time `json:"observed_at"`
	FreshUntil            *time.Time `json:"fresh_until"`
	Freshness             string     `json:"freshness"`
	LatencyMS             *int64     `json:"latency_ms"`
	Source                string     `json:"source"`
}

type ProbeTarget struct {
	ID            string
	BaseURL       string
	SecretRef     string
	ProviderModel string
	Timeout       time.Duration
	ProbeInterval time.Duration
}

type ProbeObservation struct {
	ID                  string
	TargetID            string
	ObservedAt          time.Time
	FreshUntil          time.Time
	Status              string
	CredentialAvailable bool
	HTTPStatus          int
	ErrorClass          string
	Latency             time.Duration
}

type PortalStore interface {
	CreatePortalSession(context.Context, string, string, [32]byte, time.Time, time.Time) (PortalSession, error)
	AuthenticatePortalSession(context.Context, [32]byte, time.Time) (PortalSession, error)
	RevokePortalSession(context.Context, [32]byte, time.Time) error
	SwitchPortalContext(context.Context, [32]byte, string, time.Time) (PortalSession, error)
	ListPortalAccess(context.Context, PortalSession) ([]PortalServiceAccount, error)
	CreatePortalServiceAccount(context.Context, PortalSession, string) (PortalServiceAccount, error)
	IssuePortalKey(context.Context, PortalSession, PortalKeyIssueSpec) (PortalKeyResult, error)
	RevokePortalKey(context.Context, PortalSession, string) error
	GetPortalServicePlan(context.Context, PortalSession, string) (PortalServicePlan, error)
	ListPortalExport(context.Context, PortalSession, UsageFilter, string) ([]PortalExportRow, error)
	ListPortalRollups(context.Context, PortalSession, UsageFilter) ([]PortalUsageRollup, error)
	ListPortalObservations(context.Context, PortalSession, string, time.Time) ([]PortalObservation, error)
	GetRollupCheckpoint(context.Context, PortalSession) (RollupCheckpoint, error)
}

type HumanProvisioner interface {
	ProvisionHuman(context.Context, HumanUserSpec) (HumanUserResult, error)
	RotateHumanPassword(context.Context, string, string) error
	DisableHuman(context.Context, string) error
}

type RollupStore interface {
	RefreshUsageRollups(context.Context, time.Time, time.Time, time.Time) (int64, error)
	ListProbeTargets(context.Context, time.Time) ([]ProbeTarget, error)
	RecordProbeObservation(context.Context, ProbeObservation) error
	WorkerHealthy(context.Context, time.Time, time.Duration) error
}
