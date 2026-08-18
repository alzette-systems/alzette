package workforce

import (
	"context"
	"crypto/sha256"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	"alzette/internal/platform"
)

const (
	RelationshipOwner    = "owner"
	RelationshipEmployee = "employee"
)

var groupNamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9 ._:-]{0,126}[A-Za-z0-9])?$`)

type ModelAccess struct {
	RouteID     string
	Alias       string
	Project     string
	Environment string
}

type Person struct {
	ID              string
	DisplayName     string
	Email           string
	Relationship    string
	Enabled         bool
	Groups          []GroupReference
	EffectiveModels []ModelAccess
}

type GroupReference struct {
	ID   string
	Name string
}

type Group struct {
	ID          string
	Name        string
	Description string
	Enabled     bool
	Project     string
	Environment string
	People      []PersonReference
	Models      []ModelAccess
}

type PersonReference struct {
	ID           string
	DisplayName  string
	Email        string
	Relationship string
}

type Access struct {
	Configured      bool
	Relationship    string
	CanManage       bool
	CurrentPersonID string
	People          []Person
	Groups          []Group
	AvailableModels []ModelAccess
	Invitations     []Invitation
}

type Invitation struct {
	ID          string
	Email       string
	DisplayName string
	Status      string
	Groups      []GroupReference
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Delivery    string
}

type CreateInvitationInput struct {
	Email       string
	DisplayName string
	GroupIDs    []string
}

type InvitationDelivery struct {
	Invitation Invitation
	Token      string
}

type SetupSession struct {
	Token     string
	ExpiresAt time.Time
}

type OIDCTransaction struct {
	ActionSessionID string
	Nonce           string
	Verifier        string
}

type FederatedIdentity struct {
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
}

type CreateGroupInput struct {
	Name        string
	Description string
	RouteIDs    []string
}

type InitialOwnerSpec struct {
	OrganisationSlug string
	Username         string
	EvidenceRef      string
}

type InitialOwnerResult struct {
	OrganisationID string `json:"organisation_id"`
	PersonID       string `json:"person_id"`
	UserID         string `json:"user_id"`
	Username       string `json:"username"`
	OwnershipID    string `json:"ownership_id"`
	Created        bool   `json:"created"`
}

type Store interface {
	LoadAccess(context.Context, platform.PortalSession) (Access, error)
	LoadGroup(context.Context, platform.PortalSession, string) (Group, error)
	CreateGroup(context.Context, platform.PortalSession, CreateGroupInput) (Group, error)
	ReplaceGroupPeople(context.Context, platform.PortalSession, string, []string) error
	ReplaceGroupModels(context.Context, platform.PortalSession, string, []string) error
	DisableGroup(context.Context, platform.PortalSession, string) error
	CreateInvitation(context.Context, platform.PortalSession, CreateInvitationInput) (InvitationDelivery, error)
	ResendInvitation(context.Context, platform.PortalSession, string) (InvitationDelivery, error)
	RevokeInvitation(context.Context, platform.PortalSession, string) error
	AssignInitialOwner(context.Context, InitialOwnerSpec) (InitialOwnerResult, error)
}

type IdentityStore interface {
	BeginInvitationSetup(context.Context, [32]byte, time.Time) (SetupSession, error)
	CreateOIDCTransaction(context.Context, [32]byte, [32]byte, string, string, time.Time, time.Time) error
	ConsumeOIDCTransaction(context.Context, [32]byte, time.Time) (OIDCTransaction, error)
	AcceptInvitation(context.Context, string, FederatedIdentity, [32]byte, time.Time, time.Time) (platform.PortalSession, error)
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	if store == nil {
		return nil
	}
	return &Service{store: store}
}

func (s *Service) Access(ctx context.Context, session platform.PortalSession) (Access, error) {
	return s.store.LoadAccess(ctx, session)
}

func (s *Service) Group(ctx context.Context, session platform.PortalSession, id string) (Group, error) {
	if !validID(id) {
		return Group{}, platform.ErrInvalid
	}
	return s.store.LoadGroup(ctx, session, id)
}

func (s *Service) CreateGroup(ctx context.Context, session platform.PortalSession, input CreateGroupInput) (Group, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	var err error
	input.RouteIDs, err = normaliseIDs(input.RouteIDs)
	if err != nil {
		return Group{}, err
	}
	if !groupNamePattern.MatchString(input.Name) || len(input.Description) > 1000 {
		return Group{}, platform.ErrInvalid
	}
	return s.store.CreateGroup(ctx, session, input)
}

func (s *Service) ReplaceGroupPeople(ctx context.Context, session platform.PortalSession, groupID string, personIDs []string) error {
	if !validID(groupID) {
		return platform.ErrInvalid
	}
	values, err := normaliseIDs(personIDs)
	if err != nil {
		return err
	}
	return s.store.ReplaceGroupPeople(ctx, session, groupID, values)
}

func (s *Service) ReplaceGroupModels(ctx context.Context, session platform.PortalSession, groupID string, routeIDs []string) error {
	if !validID(groupID) {
		return platform.ErrInvalid
	}
	values, err := normaliseIDs(routeIDs)
	if err != nil {
		return err
	}
	return s.store.ReplaceGroupModels(ctx, session, groupID, values)
}

func (s *Service) DisableGroup(ctx context.Context, session platform.PortalSession, groupID string) error {
	if !validID(groupID) {
		return platform.ErrInvalid
	}
	return s.store.DisableGroup(ctx, session, groupID)
}

func (s *Service) CreateInvitation(ctx context.Context, session platform.PortalSession, input CreateInvitationInput) (InvitationDelivery, error) {
	input.Email = normaliseEmail(input.Email)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	var err error
	input.GroupIDs, err = normaliseIDs(input.GroupIDs)
	if err != nil {
		return InvitationDelivery{}, err
	}
	if !validEmail(input.Email) || len(input.DisplayName) > 255 || len(input.GroupIDs) == 0 {
		return InvitationDelivery{}, platform.ErrInvalid
	}
	return s.store.CreateInvitation(ctx, session, input)
}

func (s *Service) ResendInvitation(ctx context.Context, session platform.PortalSession, invitationID string) (InvitationDelivery, error) {
	if !validID(invitationID) {
		return InvitationDelivery{}, platform.ErrInvalid
	}
	return s.store.ResendInvitation(ctx, session, invitationID)
}

func (s *Service) RevokeInvitation(ctx context.Context, session platform.PortalSession, invitationID string) error {
	if !validID(invitationID) {
		return platform.ErrInvalid
	}
	return s.store.RevokeInvitation(ctx, session, invitationID)
}

func (s *Service) BeginInvitationSetup(ctx context.Context, invitationToken string, now time.Time) (SetupSession, error) {
	store, ok := s.store.(IdentityStore)
	if !ok || len(invitationToken) < 32 || len(invitationToken) > 512 {
		return SetupSession{}, platform.ErrUnavailable
	}
	return store.BeginInvitationSetup(ctx, sha256.Sum256([]byte(invitationToken)), now.UTC())
}

func (s *Service) CreateOIDCTransaction(ctx context.Context, setupToken, state, nonce, verifier string, now time.Time) error {
	store, ok := s.store.(IdentityStore)
	if !ok || len(setupToken) < 32 || len(state) < 32 || len(nonce) < 32 || len(verifier) < 43 {
		return platform.ErrInvalid
	}
	return store.CreateOIDCTransaction(ctx, sha256.Sum256([]byte(setupToken)), sha256.Sum256([]byte(state)), nonce, verifier, now.UTC(), now.UTC().Add(10*time.Minute))
}

func (s *Service) ConsumeOIDCTransaction(ctx context.Context, state string, now time.Time) (OIDCTransaction, error) {
	store, ok := s.store.(IdentityStore)
	if !ok || len(state) < 32 {
		return OIDCTransaction{}, platform.ErrInvalid
	}
	return store.ConsumeOIDCTransaction(ctx, sha256.Sum256([]byte(state)), now.UTC())
}

func (s *Service) AcceptInvitation(ctx context.Context, actionSessionID string, identity FederatedIdentity, sessionDigest [32]byte, sessionExpiresAt, now time.Time) (platform.PortalSession, error) {
	store, ok := s.store.(IdentityStore)
	identity.Email = normaliseEmail(identity.Email)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	if !ok || !validID(actionSessionID) || identity.Issuer == "" || len(identity.Issuer) > 2048 || !validOIDCSubject(identity.Subject) || !validEmail(identity.Email) || len(identity.DisplayName) > 255 || sessionExpiresAt.Sub(now) < 15*time.Minute {
		return platform.PortalSession{}, platform.ErrInvalid
	}
	return store.AcceptInvitation(ctx, actionSessionID, identity, sessionDigest, sessionExpiresAt.UTC(), now.UTC())
}

func (s *Service) AssignInitialOwner(ctx context.Context, spec InitialOwnerSpec) (InitialOwnerResult, error) {
	spec.OrganisationSlug = strings.TrimSpace(strings.ToLower(spec.OrganisationSlug))
	spec.Username = strings.TrimSpace(strings.ToLower(spec.Username))
	spec.EvidenceRef = strings.TrimSpace(spec.EvidenceRef)
	if !validSlug(spec.OrganisationSlug) || !validUsername(spec.Username) || !validEvidence(spec.EvidenceRef) {
		return InitialOwnerResult{}, platform.ErrInvalid
	}
	return s.store.AssignInitialOwner(ctx, spec)
}

func normaliseIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !validID(value) {
			return nil, platform.ErrInvalid
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func validID(value string) bool {
	if len(value) < 3 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validSlug(value string) bool {
	if len(value) < 1 || len(value) > 63 || !((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= '0' && value[0] <= '9')) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validUsername(value string) bool {
	if len(value) < 3 || len(value) > 63 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune("._-", character) {
			continue
		}
		return false
	}
	return true
}

func validEvidence(value string) bool {
	if len(value) < 1 || len(value) > 255 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune(" ._:/-", character) {
			continue
		}
		return false
	}
	return true
}

func normaliseEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validEmail(value string) bool {
	if len(value) < 3 || len(value) > 320 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Count(value, "@") == 1
}

func validOIDCSubject(value string) bool {
	if len(value) < 1 || len(value) > 255 {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}
