package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"alzette/internal/agentauth"
	"alzette/internal/federation"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
	"alzette/internal/workforce"
)

type staticAccessProvider struct{}

func (staticAccessProvider) ValidateAccessToken(context.Context, string) (federation.Identity, error) {
	return federation.Identity{}, nil
}
func (staticAccessProvider) Issuer() string   { return "https://identity.example.test" }
func (staticAccessProvider) ClientID() string { return "test-agent-client" }

func TestWorkforceOwnerGroupPolicyAndTenantIsolation(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	company, err := fixture.store.Provision(ctx, databaseSpec("Workforce Company", "workforce-company"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := fixture.store.Provision(ctx, databaseSpec("Other Company", "other-company"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := humanauth.HashPassword("workforce-test-password")
	if err != nil {
		t.Fatal(err)
	}
	ownerUser, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "company-owner", DisplayName: "Company Owner", PasswordHash: hash, OrganisationSlug: "workforce-company", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	employeeUser, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "company-employee", DisplayName: "Company Employee", PasswordHash: hash, OrganisationSlug: "workforce-company", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleOrgAdmin})
	if err != nil {
		t.Fatal(err)
	}
	otherUser, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "other-employee", DisplayName: "Other Employee", PasswordHash: hash, OrganisationSlug: "other-company", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleViewer})
	if err != nil {
		t.Fatal(err)
	}

	ownerSession := workforceSession(company, ownerUser, "Workforce Company", "workforce-company")
	employeeSession := workforceSession(company, employeeUser, "Workforce Company", "workforce-company")
	otherSession := workforceSession(other, otherUser, "Other Company", "other-company")

	access, err := fixture.store.LoadAccess(ctx, ownerSession)
	if err != nil || access.Configured {
		t.Fatalf("unreconciled access=%#v err=%v", access, err)
	}
	assigned, err := fixture.store.AssignInitialOwner(ctx, workforce.InitialOwnerSpec{OrganisationSlug: "workforce-company", Username: "company-owner", EvidenceRef: "test/owner-reconciliation"})
	if err != nil || !assigned.Created {
		t.Fatalf("assign owner=%#v err=%v", assigned, err)
	}
	again, err := fixture.store.AssignInitialOwner(ctx, workforce.InitialOwnerSpec{OrganisationSlug: "workforce-company", Username: "company-owner", EvidenceRef: "test/idempotent"})
	if err != nil || again.Created || again.OwnershipID != assigned.OwnershipID {
		t.Fatalf("idempotent owner=%#v err=%v", again, err)
	}
	if _, err := fixture.store.AssignInitialOwner(ctx, workforce.InitialOwnerSpec{OrganisationSlug: "workforce-company", Username: "company-employee", EvidenceRef: "test/second-owner"}); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("second owner error=%v", err)
	}

	access, err = fixture.store.LoadAccess(ctx, ownerSession)
	if err != nil {
		t.Fatal(err)
	}
	if !access.Configured || !access.CanManage || access.Relationship != workforce.RelationshipOwner || len(access.People) != 2 || len(access.AvailableModels) != 1 || len(access.People[0].EffectiveModels) != 1 {
		t.Fatalf("owner access=%#v", access)
	}
	group, err := fixture.store.CreateGroup(ctx, ownerSession, workforce.CreateGroupInput{Name: "Research", Description: "Research employees", RouteIDs: []string{company.RouteID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.ReplaceGroupPeople(ctx, ownerSession, group.ID, []string{personIDForUser(t, fixture, company.OrganisationID, employeeUser.UserID)}); err != nil {
		t.Fatal(err)
	}

	employeeAccess, err := fixture.store.LoadAccess(ctx, employeeSession)
	if err != nil {
		t.Fatal(err)
	}
	if employeeAccess.CanManage || employeeAccess.Relationship != workforce.RelationshipEmployee || len(employeeAccess.People) != 1 || len(employeeAccess.Groups) != 1 || len(employeeAccess.People[0].EffectiveModels) != 1 || employeeAccess.People[0].EffectiveModels[0].Alias != "safe-chat" {
		t.Fatalf("employee access=%#v", employeeAccess)
	}
	if _, err := fixture.store.CreateGroup(ctx, employeeSession, workforce.CreateGroupInput{Name: "Escalation"}); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("employee group creation error=%v", err)
	}
	if err := fixture.store.ReplaceGroupModels(ctx, ownerSession, group.ID, []string{other.RouteID}); !errors.Is(err, platform.ErrInvalid) {
		t.Fatalf("cross-tenant route error=%v", err)
	}
	if err := fixture.store.ReplaceGroupPeople(ctx, ownerSession, group.ID, []string{personIDForUser(t, fixture, other.OrganisationID, otherUser.UserID)}); !errors.Is(err, platform.ErrInvalid) {
		t.Fatalf("cross-tenant person error=%v", err)
	}
	if _, err := fixture.store.LoadGroup(ctx, otherSession, group.ID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("cross-tenant group read error=%v", err)
	}
	if _, err := fixture.db.Exec(`UPDATE organisation_people SET enabled=false WHERE id=$1`, assigned.PersonID); err == nil {
		t.Fatal("database allowed current owner to be disabled")
	}
	if err := fixture.store.DisableHuman(ctx, "company-owner"); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("operator disable of current owner error=%v", err)
	}
	if _, err := fixture.db.Exec(`UPDATE human_users SET enabled=false WHERE id=$1`, ownerUser.UserID); err == nil {
		t.Fatal("database allowed current owner account to be disabled")
	}

	service := workforce.New(fixture.store)
	delivery, err := service.CreateInvitation(ctx, ownerSession, workforce.CreateInvitationInput{Email: "invited.employee@example.test", DisplayName: "Invited Employee", GroupIDs: []string{group.ID}})
	if err != nil || delivery.Token == "" || delivery.Invitation.Status != "pending" {
		t.Fatalf("create invitation=%#v err=%v", delivery, err)
	}
	var digestLength int
	var storedPlaintext bool
	if err := fixture.db.QueryRow(`SELECT octet_length(token_digest),token_digest=convert_to($2,'UTF8') FROM human_invitations WHERE id=$1`, delivery.Invitation.ID, delivery.Token).Scan(&digestLength, &storedPlaintext); err != nil || digestLength != 32 || storedPlaintext {
		t.Fatalf("invitation token storage length=%d plaintext=%t err=%v", digestLength, storedPlaintext, err)
	}
	now := fixture.store.now().UTC()
	setup, err := service.BeginInvitationSetup(ctx, delivery.Token, now)
	if err != nil || setup.Token == "" {
		t.Fatalf("begin invitation setup=%#v err=%v", setup, err)
	}
	var status string
	if err := fixture.db.QueryRow(`SELECT status FROM human_invitations WHERE id=$1`, delivery.Invitation.ID).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("scanner-safe GET changed invitation status=%q err=%v", status, err)
	}
	state := "state_abcdefghijklmnopqrstuvwxyz0123456789AB"
	nonce := "nonce_abcdefghijklmnopqrstuvwxyz0123456789AB"
	verifier := "verifier_abcdefghijklmnopqrstuvwxyz0123456789AB"
	if err := service.CreateOIDCTransaction(ctx, setup.Token, state, nonce, verifier, now); err != nil {
		t.Fatal(err)
	}
	transaction, err := service.ConsumeOIDCTransaction(ctx, state, now)
	if err != nil || transaction.Nonce != nonce || transaction.Verifier != verifier || transaction.ActionSessionID == "" {
		t.Fatalf("consume OIDC transaction=%#v err=%v", transaction, err)
	}
	sessionDigest := sha256.Sum256([]byte("invited-employee-portal-session"))
	acceptedSession, err := service.AcceptInvitation(ctx, transaction.ActionSessionID, workforce.FederatedIdentity{Issuer: "https://identity.example.test", Subject: "employee|subject-1", Email: "invited.employee@example.test", DisplayName: "Verified Employee"}, sessionDigest, now.Add(12*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	acceptedAccess, err := fixture.store.LoadAccess(ctx, acceptedSession)
	if err != nil || acceptedAccess.Relationship != workforce.RelationshipEmployee || acceptedAccess.CanManage || len(acceptedAccess.Groups) != 1 || len(acceptedAccess.People) != 1 || len(acceptedAccess.People[0].EffectiveModels) != 1 {
		t.Fatalf("accepted employee access=%#v err=%v", acceptedAccess, err)
	}
	var passwordMissing bool
	var identityCount, groupCount int
	if err := fixture.db.QueryRow(`SELECT password_hash IS NULL FROM human_users WHERE id=$1`, acceptedSession.User.ID).Scan(&passwordMissing); err != nil || !passwordMissing {
		t.Fatalf("invited user password state=%t err=%v", passwordMissing, err)
	}
	if err := fixture.db.QueryRow(`SELECT count(*) FROM human_federated_identities WHERE user_id=$1 AND issuer='https://identity.example.test' AND subject='employee|subject-1' AND enabled`, acceptedSession.User.ID).Scan(&identityCount); err != nil || identityCount != 1 {
		t.Fatalf("federated identity count=%d err=%v", identityCount, err)
	}
	if err := fixture.db.QueryRow(`SELECT count(*) FROM access_group_people WHERE group_id=$1 AND person_id=$2`, group.ID, acceptedAccess.CurrentPersonID).Scan(&groupCount); err != nil || groupCount != 1 {
		t.Fatalf("invited group count=%d err=%v", groupCount, err)
	}
	agentIdentity := federation.Identity{Issuer: "https://identity.example.test", Subject: "employee|subject-1", OAuthClientID: "test-agent-client"}
	agentService := agentauth.New(fixture.store, staticAccessProvider{})
	contexts, err := agentService.Contexts(ctx, agentIdentity)
	if err != nil || len(contexts) != 1 || contexts[0].MembershipID != acceptedSession.Current.ID || len(contexts[0].ModelAliases) != 1 || contexts[0].ModelAliases[0] != "safe-chat" {
		t.Fatalf("agent contexts=%#v err=%v", contexts, err)
	}
	clientInstance := "aci_" + base64.RawURLEncoding.EncodeToString([]byte("client-instance-1"))
	idempotencyKey := "agm_" + base64.RawURLEncoding.EncodeToString([]byte("idempotency-key-1"))
	minted, err := agentService.Mint(ctx, agentIdentity, agentauth.MintInput{ClientInstanceID: clientInstance, MembershipID: acceptedSession.Current.ID, ModelAliases: []string{"safe-chat"}}, idempotencyKey)
	if err != nil || minted.AccessToken == "" || time.Until(minted.ExpiresAt) < 9*time.Minute || time.Until(minted.ExpiresAt) > 10*time.Minute {
		t.Fatalf("minted credential=%#v err=%v", minted, err)
	}
	humanPrincipal, err := fixture.store.AuthenticateHuman(ctx, sha256.Sum256([]byte(minted.AccessToken)))
	if err != nil || humanPrincipal.CredentialKind != "human_agent_token" || humanPrincipal.HumanUserID != acceptedSession.User.ID || !humanPrincipal.AllowsModel("safe-chat") {
		t.Fatalf("human principal=%#v err=%v", humanPrincipal, err)
	}
	if _, err = fixture.store.ResolveRoute(ctx, humanPrincipal, "safe-chat"); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.store.ResolveRoute(ctx, humanPrincipal, "not-granted"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("ungranted route error=%v", err)
	}
	if err = fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: "req_human_agent", Principal: humanPrincipal, ModelAlias: "safe-chat", StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	if rows, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now.Add(time.Hour), now.Add(time.Minute)); err != nil || rows != 0 {
		t.Fatalf("service-account rollup must remain healthy and exclude the separately attributed human ledger row: rows=%d err=%v", rows, err)
	}
	var serviceID, humanID sql.NullString
	if err = fixture.db.QueryRow(`SELECT service_account_id,human_user_id FROM inference_requests WHERE id='req_human_agent'`).Scan(&serviceID, &humanID); err != nil || serviceID.Valid || !humanID.Valid || humanID.String != acceptedSession.User.ID {
		t.Fatalf("ledger service=%v human=%v err=%v", serviceID, humanID, err)
	}
	if _, err = agentService.Mint(ctx, agentIdentity, agentauth.MintInput{ClientInstanceID: clientInstance, MembershipID: acceptedSession.Current.ID, ModelAliases: []string{"safe-chat"}}, idempotencyKey); !errors.Is(err, agentauth.ErrResponseUnrecoverable) {
		t.Fatalf("mint replay error=%v", err)
	}
	if _, err = fixture.store.AuthenticateHuman(ctx, sha256.Sum256([]byte(minted.AccessToken))); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("replayed mint token remained active: %v", err)
	}
	minted, err = agentService.Mint(ctx, agentIdentity, agentauth.MintInput{ClientInstanceID: clientInstance, MembershipID: acceptedSession.Current.ID, ModelAliases: []string{"safe-chat"}}, "agm_"+base64.RawURLEncoding.EncodeToString([]byte("idempotency-key-2")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptInvitation(ctx, transaction.ActionSessionID, workforce.FederatedIdentity{Issuer: "https://identity.example.test", Subject: "employee|subject-1", Email: "invited.employee@example.test"}, sha256.Sum256([]byte("replay-session")), now.Add(12*time.Hour), now); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("invitation replay error=%v", err)
	}
	second, err := service.CreateInvitation(ctx, ownerSession, workforce.CreateInvitationInput{Email: "second.employee@example.test", GroupIDs: []string{group.ID}})
	if err != nil {
		t.Fatal(err)
	}
	secondSetup, err := service.BeginInvitationSetup(ctx, second.Token, now)
	if err != nil {
		t.Fatal(err)
	}
	secondState := "state_second_abcdefghijklmnopqrstuvwxyz0123456789"
	if err := service.CreateOIDCTransaction(ctx, secondSetup.Token, secondState, nonce, verifier, now); err != nil {
		t.Fatal(err)
	}
	secondTransaction, err := service.ConsumeOIDCTransaction(ctx, secondState, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptInvitation(ctx, secondTransaction.ActionSessionID, workforce.FederatedIdentity{Issuer: "https://identity.example.test", Subject: "wrong|subject", Email: "wrong.employee@example.test"}, sha256.Sum256([]byte("wrong-email-session")), now.Add(12*time.Hour), now); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("wrong-email acceptance error=%v", err)
	}
	var wrongUsers int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM human_users WHERE email_normalized='wrong.employee@example.test'`).Scan(&wrongUsers); err != nil || wrongUsers != 0 {
		t.Fatalf("wrong identity created users=%d err=%v", wrongUsers, err)
	}
	resent, err := service.ResendInvitation(ctx, ownerSession, second.Invitation.ID)
	if err != nil || resent.Token == second.Token {
		t.Fatalf("resent invitation=%#v err=%v", resent, err)
	}
	if _, err := service.BeginInvitationSetup(ctx, second.Token, now); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("old invitation token remained usable: %v", err)
	}
	if err := service.RevokeInvitation(ctx, ownerSession, second.Invitation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BeginInvitationSetup(ctx, resent.Token, now); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("revoked invitation token remained usable: %v", err)
	}

	var generationBefore int64
	employeePersonID := personIDForUser(t, fixture, company.OrganisationID, employeeUser.UserID)
	if err := fixture.db.QueryRow(`SELECT authorisation_generation FROM organisation_people WHERE id=$1`, employeePersonID).Scan(&generationBefore); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.DisableGroup(ctx, ownerSession, group.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.store.AuthenticateHuman(ctx, sha256.Sum256([]byte(minted.AccessToken))); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("disabled group left human token usable: %v", err)
	}
	employeeAccess, err = fixture.store.LoadAccess(ctx, employeeSession)
	if err != nil {
		t.Fatal(err)
	}
	if len(employeeAccess.Groups) != 1 || employeeAccess.Groups[0].Enabled || len(employeeAccess.People[0].EffectiveModels) != 0 {
		t.Fatalf("disabled group still grants access=%#v", employeeAccess)
	}
	var generationAfter int64
	if err := fixture.db.QueryRow(`SELECT authorisation_generation FROM organisation_people WHERE id=$1`, employeePersonID).Scan(&generationAfter); err != nil {
		t.Fatal(err)
	}
	if generationAfter <= generationBefore {
		t.Fatalf("authorization generation did not advance: before=%d after=%d", generationBefore, generationAfter)
	}
}

func workforceSession(company platform.ProvisionResult, human platform.HumanUserResult, organisationName, organisationSlug string) platform.PortalSession {
	membership := platform.PortalMembership{ID: human.MembershipID, OrganisationID: company.OrganisationID, OrganisationName: organisationName, OrganisationSlug: organisationSlug, ProjectID: company.ProjectID, ProjectName: "Application", ProjectSlug: "application", EnvironmentID: company.EnvironmentID, EnvironmentName: "Production", EnvironmentSlug: "production", Role: platform.PortalRoleViewer}
	return platform.PortalSession{User: platform.PortalUser{ID: human.UserID, Username: human.Username, DisplayName: human.Username}, Current: membership, Memberships: []platform.PortalMembership{membership}}
}

func personIDForUser(t *testing.T, fixture *databaseFixture, organisationID, userID string) string {
	t.Helper()
	var personID string
	if err := fixture.db.QueryRow(`SELECT id FROM organisation_people WHERE organisation_id=$1 AND user_id=$2`, organisationID, userID).Scan(&personID); err != nil {
		t.Fatal(err)
	}
	return personID
}
