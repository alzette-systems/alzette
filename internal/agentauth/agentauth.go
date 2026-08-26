package agentauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"alzette/internal/credentials"
	"alzette/internal/federation"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

var (
	ErrIdempotencyConflict   = errors.New("idempotency conflict")
	ErrResponseUnrecoverable = errors.New("credential response unrecoverable")
)

type Context struct {
	MembershipID string   `json:"membership_id"`
	Organisation string   `json:"organisation"`
	Project      string   `json:"project"`
	Environment  string   `json:"environment"`
	Relationship string   `json:"relationship"`
	ModelAliases []string `json:"model_aliases"`
	Models       []Model  `json:"models,omitempty"`
}

// Model is the employee-safe capability contract for one entitled alias. It
// deliberately contains no provider, target, route, or infrastructure identity.
type Model struct {
	Alias               string   `json:"alias"`
	DisplayName         string   `json:"display_name"`
	Capabilities        []string `json:"capabilities,omitempty"`
	ContextWindowTokens *int64   `json:"context_window_tokens,omitempty"`
}

type MintInput struct {
	ClientInstanceID string   `json:"client_instance_id"`
	MembershipID     string   `json:"membership_id"`
	ModelAliases     []string `json:"model_aliases"`
}

type StoreMintInput struct {
	Identity             federation.Identity
	ClientInstanceDigest [32]byte
	IdempotencyKeyDigest [32]byte
	CanonicalRequestHash [32]byte
	MembershipID         string
	ModelAliases         []string
	GrantID              string
	TokenID              string
	MintID               string
	Credential           credentials.Key
	Now                  time.Time
	TokenExpiresAt       time.Time
	GrantExpiresAt       time.Time
}

type MintResult struct {
	Context      Context
	GrantID      string
	AccessToken  string
	ExpiresAt    time.Time
	ModelAliases []string
}

type Store interface {
	ListAgentContexts(context.Context, federation.Identity) ([]Context, error)
	MintAgentCredential(context.Context, StoreMintInput) (MintResult, error)
	RevokeAgentGrant(context.Context, federation.Identity, string, [32]byte, time.Time) error
}

type Service struct {
	store     Store
	validator federation.AccessTokenProvider
	now       func() time.Time
	newID     func(string) (string, error)
}

func New(store Store, validator federation.AccessTokenProvider) *Service {
	if store == nil || validator == nil {
		return nil
	}
	return &Service{store: store, validator: validator, now: time.Now, newID: ids.New}
}

func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *Service) Authenticate(ctx context.Context, raw string) (federation.Identity, error) {
	return s.validator.ValidateAccessToken(ctx, raw)
}

func (s *Service) Metadata(controlOrigin, gatewayOrigin string) map[string]interface{} {
	return map[string]interface{}{
		"schema": "alzette.agent-configuration.v1", "issuer": s.validator.Issuer(),
		"oauth_client_id": s.validator.ClientID(), "control_origin": controlOrigin,
		"gateway_base_url": strings.TrimRight(gatewayOrigin, "/") + "/v1",
		"login_modes":      []string{"authorization_code_pkce_s256"},
	}
}

func (s *Service) Contexts(ctx context.Context, identity federation.Identity) ([]Context, error) {
	return s.store.ListAgentContexts(ctx, identity)
}

func (s *Service) Mint(ctx context.Context, identity federation.Identity, input MintInput, idempotencyKey string) (MintResult, error) {
	input.ClientInstanceID = strings.TrimSpace(input.ClientInstanceID)
	input.MembershipID = strings.TrimSpace(input.MembershipID)
	aliases, err := normaliseAliases(input.ModelAliases)
	if err != nil || !validOpaque(input.ClientInstanceID, "aci_", 16) || !validSimpleID(input.MembershipID) || !validOpaque(idempotencyKey, "agm_", 16) {
		return MintResult{}, platform.ErrInvalid
	}
	canonical, err := json.Marshal(struct {
		Schema           string   `json:"schema"`
		Issuer           string   `json:"issuer"`
		Subject          string   `json:"subject"`
		ClientID         string   `json:"client_id"`
		ClientInstanceID string   `json:"client_instance_id"`
		MembershipID     string   `json:"membership_id"`
		ModelAliases     []string `json:"model_aliases"`
	}{"alzette.agent-credential.v1", identity.Issuer, identity.Subject, identity.OAuthClientID, input.ClientInstanceID, input.MembershipID, aliases})
	if err != nil {
		return MintResult{}, err
	}
	credential, err := credentials.GenerateHuman()
	if err != nil {
		return MintResult{}, err
	}
	grantID, err := s.newID("agr")
	if err != nil {
		return MintResult{}, err
	}
	tokenID, err := s.newID("aut")
	if err != nil {
		return MintResult{}, err
	}
	mintID, err := s.newID("agm")
	if err != nil {
		return MintResult{}, err
	}
	now := s.now().UTC()
	return s.store.MintAgentCredential(ctx, StoreMintInput{
		Identity: identity, ClientInstanceDigest: sha256.Sum256([]byte(input.ClientInstanceID)),
		IdempotencyKeyDigest: sha256.Sum256([]byte(idempotencyKey)), CanonicalRequestHash: sha256.Sum256(canonical),
		MembershipID: input.MembershipID, ModelAliases: aliases, GrantID: grantID, TokenID: tokenID, MintID: mintID,
		Credential: credential, Now: now, TokenExpiresAt: now.Add(10 * time.Minute), GrantExpiresAt: now.Add(time.Hour),
	})
}

func (s *Service) Revoke(ctx context.Context, identity federation.Identity, membershipID, clientInstanceID string) error {
	if !validSimpleID(membershipID) || !validOpaque(clientInstanceID, "aci_", 16) {
		return platform.ErrInvalid
	}
	return s.store.RevokeAgentGrant(ctx, identity, membershipID, sha256.Sum256([]byte(clientInstanceID)), s.now().UTC())
}

func normaliseAliases(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 64 {
		return nil, platform.ErrInvalid
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < 1 || len(value) > 128 || strings.ContainsAny(value, " \t\r\n/\\") {
			return nil, platform.ErrInvalid
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func validOpaque(value, marker string, minimumBytes int) bool {
	if len(value) <= len(marker) || len(value) > 160 || !strings.HasPrefix(value, marker) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, marker))
	return err == nil && len(decoded) >= minimumBytes
}

func validSimpleID(value string) bool {
	if len(value) < 3 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}
