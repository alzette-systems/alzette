package postgres

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"alzette/internal/credentials"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
	"alzette/internal/workforce"
)

func TestPostgresHumanPasswordRotationRevokesSessionsAndPortalKeysOverlap(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	fixture.store.SetClock(func() time.Time { return now })
	provisioned, err := fixture.store.Provision(ctx, databaseSpec("Portal Tenant", "portal-tenant"))
	if err != nil {
		t.Fatal(err)
	}
	oldPassword := "credential-neutral-old-password"
	oldHash, err := humanauth.HashPassword(oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	user, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "portal-admin", DisplayName: "Portal Owner", PasswordHash: oldHash, OrganisationSlug: "portal-tenant", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleViewer})
	if err != nil || !user.Created {
		t.Fatalf("provision human: %v", err)
	}
	owner, err := workforce.New(fixture.store).AssignInitialOwner(ctx, workforce.InitialOwnerSpec{OrganisationSlug: "portal-tenant", Username: "portal-admin", EvidenceRef: "test/application-owner"})
	if err != nil || !owner.Created {
		t.Fatalf("assign explicit owner: result=%#v err=%v", owner, err)
	}
	employee, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "portal-employee", DisplayName: "Legacy Admin Employee", PasswordHash: oldHash, OrganisationSlug: "portal-tenant", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleOrgAdmin})
	if err != nil || !employee.Created {
		t.Fatalf("provision employee: %v", err)
	}
	employeeDigest := humanauth.Digest("credential-neutral-employee-session")
	employeeSession, err := fixture.store.CreatePortalSession(ctx, "portal-employee", oldPassword, employeeDigest, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreatePortalServiceAccount(ctx, employeeSession, "legacy admin cannot create"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("non-owner legacy org_admin created a service account: %v", err)
	}
	other, err := fixture.store.Provision(ctx, databaseSpec("Portal Tenant B", "portal-tenant-b"))
	if err != nil {
		t.Fatal(err)
	}
	otherMembership, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "portal-admin", DisplayName: "Portal Admin", PasswordHash: oldHash, OrganisationSlug: "portal-tenant-b", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleViewer})
	if err != nil || otherMembership.Created {
		t.Fatalf("second membership provision: %v", err)
	}
	digest := humanauth.Digest("credential-neutral-session-token")
	session, err := fixture.store.CreatePortalSession(ctx, "portal-admin", oldPassword, digest, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Memberships) != 2 {
		t.Fatal("portal user did not receive both authorised memberships")
	}
	switched, err := fixture.store.SwitchPortalContext(ctx, digest, otherMembership.MembershipID, now.Add(30*time.Second))
	if err != nil || switched.Current.OrganisationID != other.OrganisationID {
		t.Fatal("authorised membership context switch failed")
	}
	if _, err := fixture.store.CreatePortalServiceAccount(ctx, switched, "viewer cannot create"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatal("viewer membership created a service account")
	}
	if _, err := fixture.store.SwitchPortalContext(ctx, digest, "mem_not_authorised", now.Add(30*time.Second)); !errors.Is(err, platform.ErrForbidden) {
		t.Fatal("context switch accepted a non-membership")
	}
	session, err = fixture.store.SwitchPortalContext(ctx, digest, user.MembershipID, now.Add(30*time.Second))
	if err != nil || session.Current.OrganisationID != provisioned.OrganisationID {
		t.Fatal("context switch back to the original scope failed")
	}
	if _, err := fixture.store.AuthenticatePortalSession(ctx, digest, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.ReauthenticatePortalSession(ctx, digest, "incorrect-current-password", now.Add(70*time.Second)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("incorrect current password reauthenticated: %v", err)
	}
	reauthenticated, err := fixture.store.ReauthenticatePortalSession(ctx, digest, oldPassword, now.Add(90*time.Second))
	if err != nil || !reauthenticated.AuthenticatedAt.Equal(now.Add(90*time.Second)) || reauthenticated.ID != session.ID {
		t.Fatalf("current session reauthentication failed: %v", err)
	}
	var authenticatedAt time.Time
	if err := fixture.db.QueryRow(`SELECT authenticated_at FROM portal_sessions WHERE id=$1`, session.ID).Scan(&authenticatedAt); err != nil || !authenticatedAt.Equal(now.Add(90*time.Second)) {
		t.Fatalf("recent authentication was not stored on the current session: %v", err)
	}
	expiringDigest := humanauth.Digest("credential-neutral-expiring-session")
	if _, err := fixture.store.CreatePortalSession(ctx, "portal-admin", oldPassword, expiringDigest, now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.AuthenticatePortalSession(ctx, expiringDigest, now.Add(2*time.Minute)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatal("expired portal session authenticated")
	}

	newPassword := "credential-neutral-new-password"
	newHash, err := humanauth.HashPassword(newPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.RotateHumanPassword(ctx, "portal-admin", newHash); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.AuthenticatePortalSession(ctx, digest, now.Add(2*time.Minute)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("password rotation left the old portal session active: %v", err)
	}
	if _, err := fixture.store.CreatePortalSession(ctx, "portal-admin", oldPassword, humanauth.Digest("old-password-session"), now.Add(time.Hour), now.Add(2*time.Minute)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("old password remained valid: %v", err)
	}
	newDigest := humanauth.Digest("credential-neutral-new-session")
	session, err = fixture.store.CreatePortalSession(ctx, "portal-admin", newPassword, newDigest, now.Add(time.Hour), now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	account, err := fixture.store.CreatePortalServiceAccount(ctx, session, "production application")
	if err != nil {
		t.Fatal(err)
	}
	expires := now.Add(24 * time.Hour)
	first, err := fixture.store.IssuePortalKey(ctx, session, platform.PortalKeyIssueSpec{ServiceAccountID: account.ID, Name: "production primary", Scopes: []string{platform.ScopeInferenceWrite}, ExpiresAt: &expires})
	if err != nil || first.APIKey == "" {
		t.Fatalf("issue key: %v", err)
	}
	if _, err := fixture.store.IssuePortalKey(ctx, session, platform.PortalKeyIssueSpec{ServiceAccountID: account.ID, Name: "production primary", Scopes: []string{platform.ScopeInferenceWrite}, ExpiresAt: &expires}); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("ambiguous retry with the same key name should conflict, got %v", err)
	}
	var sameNameCount int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM api_keys WHERE service_account_id=$1 AND name=$2`, account.ID, "production primary").Scan(&sameNameCount); err != nil {
		t.Fatal(err)
	}
	if sameNameCount != 1 {
		t.Fatalf("ambiguous retry minted %d credentials; want exactly one", sameNameCount)
	}
	second, err := fixture.store.IssuePortalKey(ctx, session, platform.PortalKeyIssueSpec{ServiceAccountID: account.ID, Name: "production replacement", Scopes: []string{platform.ScopeInferenceWrite}, ExpiresAt: &expires, RotatedFromPrefix: first.Prefix})
	if err != nil || second.APIKey == "" {
		t.Fatalf("rotate key: %v", err)
	}
	if _, err := fixture.store.Authenticate(ctx, credentials.Digest(first.APIKey)); err != nil {
		t.Fatal("overlap predecessor was revoked implicitly")
	}
	if _, err := fixture.store.Authenticate(ctx, credentials.Digest(second.APIKey)); err != nil {
		t.Fatal("replacement key did not authenticate")
	}
	if err := fixture.store.RevokePortalKey(ctx, session, first.Prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.Authenticate(ctx, credentials.Digest(first.APIKey)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatal("explicitly revoked predecessor still authenticated")
	}
	if _, err := fixture.store.Authenticate(ctx, credentials.Digest(second.APIKey)); err != nil {
		t.Fatal("predecessor revoke affected replacement")
	}

	var stored []byte
	if err := fixture.db.QueryRow(`SELECT key_hash FROM api_keys WHERE key_prefix=$1`, second.Prefix).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) == second.APIKey {
		t.Fatal("plaintext API key was persisted")
	}
	var activeSessionRevocations int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM portal_sessions WHERE user_id=$1 AND revoked_at IS NOT NULL`, user.UserID).Scan(&activeSessionRevocations); err != nil {
		t.Fatal(err)
	}
	if activeSessionRevocations < 1 {
		t.Fatal("password rotation did not durably revoke sessions")
	}
	if err := fixture.store.DisableHuman(ctx, "portal-admin"); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("current owner disable error=%v", err)
	}
	if _, err := fixture.store.AuthenticatePortalSession(ctx, newDigest, now.Add(3*time.Minute)); err != nil {
		t.Fatal("rejected owner disable revoked the owner session")
	}
	if err := fixture.store.DisableHuman(ctx, "portal-employee"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.AuthenticatePortalSession(ctx, employeeDigest, now.Add(3*time.Minute)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatal("disabled employee retained an active portal session")
	}
	if _, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "portal-employee", DisplayName: "Legacy Admin Employee", PasswordHash: newHash, OrganisationSlug: "portal-tenant", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleOrgAdmin}); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("disabled user re-provision did not fail explicitly: %v", err)
	}
	var enabled bool
	if err := fixture.db.QueryRow(`SELECT enabled FROM human_users WHERE id=$1`, employee.UserID).Scan(&enabled); err != nil || enabled {
		t.Fatal("failed re-provision changed disabled-user state")
	}
}

func TestPostgresServicePlansAreRouteBoundAndOrganisationIsolated(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	aAllowance, bAllowance := int64(100), int64(900)
	aSpec := databaseSpec("Plan Tenant A", "plan-tenant-a")
	aSpec.ServicePlanCode, aSpec.ServicePlanName = "pilot", "Tenant A pilot"
	aSpec.SharedRequestAllowance, aSpec.SharedRequestAllowancePeriod = &aAllowance, "month"
	aSpec.ServicePlanSource, aSpec.ServicePlanFinality = "operator-contract-a", "declared"
	a, err := fixture.store.Provision(ctx, aSpec)
	if err != nil {
		t.Fatal(err)
	}
	bSpec := databaseSpec("Plan Tenant B", "plan-tenant-b")
	bSpec.ServicePlanCode, bSpec.ServicePlanName = "pilot", "Tenant B pilot"
	bSpec.SharedRequestAllowance, bSpec.SharedRequestAllowancePeriod = &bAllowance, "month"
	bSpec.ServicePlanSource, bSpec.ServicePlanFinality = "operator-contract-b", "declared"
	b, err := fixture.store.Provision(ctx, bSpec)
	if err != nil {
		t.Fatal(err)
	}
	aSession := portalSessionFor(a, "Plan Tenant A", "plan-tenant-a")
	bSession := portalSessionFor(b, "Plan Tenant B", "plan-tenant-b")
	aPlan, err := fixture.store.GetPortalServicePlan(ctx, aSession, aSpec.ModelAlias)
	if err != nil {
		t.Fatal(err)
	}
	bPlan, err := fixture.store.GetPortalServicePlan(ctx, bSession, bSpec.ModelAlias)
	if err != nil {
		t.Fatal(err)
	}
	if aPlan.SharedRequestAllowance == nil || *aPlan.SharedRequestAllowance != aAllowance || bPlan.SharedRequestAllowance == nil || *bPlan.SharedRequestAllowance != bAllowance || aPlan.Name == bPlan.Name {
		t.Fatal("organisation-scoped plan codes contaminated another tenant")
	}
	mixedDedicated := aSpec
	mixedDedicated.ModelAlias = "dedicated-chat"
	mixedDedicated.TargetName = "plan-a-mixed-dedicated"
	mixedDedicated.TargetBaseURL = "https://mixed-dedicated.example.invalid/v1"
	mixedDedicated.ProviderModel = "provider/mixed-dedicated"
	mixedDedicated.ExecutionClass = "private_compatible"
	mixedDedicated.CapacityMode = "dedicated"
	mixedDedicated.CapacityEvidenceRef = "operator-evidence:mixed-dedicated-a"
	mixedDedicated.ServicePlanCode, mixedDedicated.ServicePlanName = "mixed-dedicated", "Tenant A mixed dedicated"
	mixedDedicated.SharedRequestAllowance, mixedDedicated.SharedRequestAllowancePeriod = nil, ""
	mixedDedicated.DedicatedResourceClass = "gpu-mixed-class"
	if _, err := fixture.store.Provision(ctx, mixedDedicated); err != nil {
		t.Fatal(err)
	}
	mixedPlan, err := fixture.store.GetPortalServicePlan(ctx, aSession, mixedDedicated.ModelAlias)
	if err != nil || mixedPlan.CapacityMode != "dedicated" || mixedPlan.Code != "mixed-dedicated" {
		t.Fatal("mixed dedicated route did not retain its route-bound plan")
	}
	ambiguous, err := fixture.store.GetPortalServicePlan(ctx, aSession, "")
	if err != nil || !ambiguous.Ambiguous || ambiguous.Available {
		t.Fatal("multiple mixed-capacity routes silently selected a service plan")
	}

	dedicated := aSpec
	dedicated.TargetName = "plan-a-dedicated"
	dedicated.TargetBaseURL = "https://dedicated.example.invalid/v1"
	dedicated.ProviderModel = "provider/dedicated"
	dedicated.ExecutionClass = "private_compatible"
	dedicated.CapacityMode = "dedicated"
	dedicated.CapacityEvidenceRef = "operator-evidence:dedicated-a"
	dedicated.ServicePlanCode, dedicated.ServicePlanName = "dedicated", "Tenant A dedicated"
	dedicated.SharedRequestAllowance, dedicated.SharedRequestAllowancePeriod = nil, ""
	dedicated.DedicatedResourceClass = "gpu-test-class"
	dedicated.ServicePlanSource = "operator-contract-a"
	transitioned, err := fixture.store.Provision(ctx, dedicated)
	if err != nil {
		t.Fatal(err)
	}
	if transitioned.RouteID != a.RouteID || transitioned.TargetID == a.TargetID {
		t.Fatal("same route was not atomically transitioned to a new capacity binding")
	}
	dedicatedPlan, err := fixture.store.GetPortalServicePlan(ctx, aSession, aSpec.ModelAlias)
	if err != nil || dedicatedPlan.CapacityMode != "dedicated" || dedicatedPlan.Code != "dedicated" {
		t.Fatal("dedicated route plan was not activated")
	}

	returned := aSpec
	returned.TargetName = "plan-a-shared-v2"
	returned.TargetBaseURL = "https://shared-v2.example.invalid/v1"
	returned.ProviderModel = "provider/shared-v2"
	back, err := fixture.store.Provision(ctx, returned)
	if err != nil {
		t.Fatal(err)
	}
	if back.RouteID != a.RouteID || back.TargetID == transitioned.TargetID {
		t.Fatal("dedicated-to-shared transition failed")
	}
	sharedAgain, err := fixture.store.GetPortalServicePlan(ctx, aSession, aSpec.ModelAlias)
	if err != nil || sharedAgain.Code != "pilot" || sharedAgain.CapacityMode != "shared" {
		t.Fatal("shared plan was not reactivated")
	}
	if aPlan.EffectiveAt == nil || sharedAgain.EffectiveAt == nil || !sharedAgain.EffectiveAt.After(*aPlan.EffectiveAt) {
		t.Fatal("reactivated plan did not receive a truthful new effective_at")
	}
	if _, err := fixture.store.Provision(ctx, returned); err != nil {
		t.Fatal(err)
	}
	idempotent, err := fixture.store.GetPortalServicePlan(ctx, aSession, aSpec.ModelAlias)
	if err != nil || idempotent.EffectiveAt == nil || !idempotent.EffectiveAt.Equal(*sharedAgain.EffectiveAt) {
		t.Fatal("idempotent re-provision spuriously reset effective_at")
	}
}

func TestPostgresPortalMigrationIntegrityGuards(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	result, err := fixture.store.Provision(ctx, databaseSpec("Guard Tenant", "guard-tenant"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := humanauth.HashPassword("guard-owner-password")
	if err != nil {
		t.Fatal(err)
	}
	human, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{Username: "guard-owner", DisplayName: "Guard Owner", PasswordHash: hash, OrganisationSlug: "guard-tenant", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workforce.New(fixture.store).AssignInitialOwner(ctx, workforce.InitialOwnerSpec{OrganisationSlug: "guard-tenant", Username: "guard-owner", EvidenceRef: "test/guard-owner"}); err != nil {
		t.Fatal(err)
	}
	session := workforceSession(result, human, "Guard Tenant", "guard-tenant")
	account, err := fixture.store.CreatePortalServiceAccount(ctx, session, "second account")
	if err != nil {
		t.Fatal(err)
	}
	var originalKeyID string
	if err := fixture.db.QueryRow(`SELECT id FROM api_keys WHERE key_prefix=$1`, result.KeyPrefix).Scan(&originalKeyID); err != nil {
		t.Fatal(err)
	}
	digest := credentials.Digest("credential-neutral-guard-key")
	now := time.Now().UTC()
	if _, err := fixture.db.Exec(`INSERT INTO api_keys(id,service_account_id,key_prefix,key_hash,scopes,name,created_at,expires_at) VALUES('key_bad_expiry',$1,'alz_k_bad_expiry',$2,'["inference:write"]','bad expiry',$3,$3)`, account.ID, digest[:], now); err == nil {
		t.Fatal("api key expiry lifecycle constraint accepted expires_at <= created_at")
	}
	if _, err := fixture.db.Exec(`INSERT INTO api_keys(id,service_account_id,key_prefix,key_hash,scopes,name,rotated_from_key_id) VALUES('key_cross_rotation',$1,'alz_k_cross_rotation',$2,'["inference:write"]','cross rotation',$3)`, account.ID, digest[:], originalKeyID); err == nil {
		t.Fatal("rotated_from_key_id crossed service-account ancestry")
	}

	baseArgs := []interface{}{result.OrganisationID, result.ProjectID, result.EnvironmentID, result.ServiceAccountID, time.Now().UTC().Truncate(time.Hour)}
	for name, values := range map[string]string{
		"known exceeds success":   `1,0,1,0,0,0,0,0,NULL,1`,
		"retry evidence mismatch": `1,1,0,0,0,0,1,1,0,0`,
	} {
		t.Run(name, func(t *testing.T) {
			query := `INSERT INTO usage_rollups_hourly_v2(organisation_id,project_id,environment_id,service_account_id,model_alias,bucket_start,logical_requests,successful_requests,failed_requests,blocked_requests,cancelled_requests,in_progress_requests,provider_attempts,retried_requests,input_tokens,input_known_requests,output_known_requests,cached_known_requests,reasoning_known_requests,p50_latency_ms,p95_latency_ms,source_row_count,source,finality,refreshed_at) VALUES($1,$2,$3,$4,'safe-chat',$5,` + values + `,0,0,0,5,10,1,'inference_requests','final',now())`
			if _, err := fixture.db.Exec(query, baseArgs...); err == nil {
				t.Fatal("corrupt rollup row was accepted")
			}
		})
	}
	if _, err := fixture.db.Exec(`INSERT INTO usage_rollups_hourly_v2(organisation_id,project_id,environment_id,service_account_id,model_alias,bucket_start,logical_requests,successful_requests,failed_requests,blocked_requests,cancelled_requests,in_progress_requests,provider_attempts,retried_requests,input_known_requests,output_known_requests,cached_known_requests,reasoning_known_requests,p50_latency_ms,p95_latency_ms,source_row_count,source,finality,refreshed_at) VALUES($1,$2,$3,$4,'safe-chat',$5,1,1,0,0,0,0,1,0,0,0,0,0,20,10,1,'inference_requests','final',now())`, baseArgs...); err == nil {
		t.Fatal("rollup accepted p95 below p50")
	}
	if _, err := fixture.db.Exec(`INSERT INTO service_plans(id,organisation_id,code,name,capacity_mode,dedicated_resource_class,source_label,finality) VALUES('plan_guard_dedicated',$1,'guard-dedicated','Guard dedicated','dedicated','gpu-test','operator-test','declared')`, result.OrganisationID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO tenant_service_plans(organisation_id,project_id,environment_id,route_id,service_plan_id,source_label,finality) VALUES($1,$2,$3,$4,'plan_guard_dedicated','operator-test','declared')`, result.OrganisationID, result.ProjectID, result.EnvironmentID, result.RouteID); err == nil {
		t.Fatal("dedicated service plan was bound to a shared target route")
	}
	if _, err := fixture.db.Exec(`INSERT INTO service_plans(id,organisation_id,code,name,capacity_mode,source_label,finality) VALUES('plan_guard_shared',$1,'guard-shared','Guard shared','shared','operator-test','declared')`, result.OrganisationID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO tenant_service_plans(organisation_id,project_id,environment_id,route_id,service_plan_id,source_label,finality) VALUES($1,$2,$3,$4,'plan_guard_shared','operator-test','declared')`, result.OrganisationID, result.ProjectID, result.EnvironmentID, result.RouteID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE service_plans SET capacity_mode='dedicated',dedicated_resource_class='gpu-test' WHERE id='plan_guard_shared'`); err == nil {
		t.Fatal("direct service-plan edit drifted from its active route target")
	}
	if _, err := fixture.db.Exec(`UPDATE tenant_routes SET enabled=false WHERE id=$1`, result.RouteID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET capacity_mode='dedicated' WHERE id=$1`, result.TargetID); err == nil {
		t.Fatal("disabled route allowed its target to drift from an active service plan")
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET probe_interval_seconds=1 WHERE id=$1`, result.TargetID); err == nil {
		t.Fatal("invalid per-target probe interval was accepted")
	}
}

func TestPostgresRollupReconciliationCheckpointAndProbeOptIn(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	spec := databaseSpec("Worker Tenant", "worker-tenant")
	spec.ProbeEnabled = false
	result, err := fixture.store.Provision(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(result.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	createCompletedRequest(t, fixture.store, principal, "req_rollup_retry", spec.ModelAlias, now.Add(-20*time.Minute), "succeeded", "final", 2, int64Pointer(7), int64Pointer(3))
	createCompletedRequest(t, fixture.store, principal, "req_rollup_failed", spec.ModelAlias, now.Add(-10*time.Minute), "failed", "unknown", 1, nil, nil)
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now.Add(time.Second)); err != nil {
		t.Fatal("repeated rollup run failed lifecycle checks")
	}
	session := portalSessionFor(result, "Worker Tenant", "worker-tenant")
	rows, err := fixture.store.ListPortalRollups(ctx, session, platform.UsageFilter{From: now.Add(-time.Hour), To: now})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rollup rows=%d err=%v", len(rows), err)
	}
	row := rows[0]
	if row.LogicalRequests != 2 || row.SuccessfulRequests != 1 || row.FailedRequests != 1 || row.InputTokens == nil || *row.InputTokens != 7 || row.InputKnownRequests != 1 || row.TokenEligibleRequests != 1 || row.P50LatencyMS == nil || row.P95LatencyMS == nil || *row.P95LatencyMS < *row.P50LatencyMS {
		t.Fatal("hourly rollup did not reconcile logical requests and known usage")
	}
	checkpoint, err := fixture.store.GetRollupCheckpoint(ctx, session)
	if err != nil || checkpoint.Status != "succeeded" || checkpoint.SourceRows == nil || *checkpoint.SourceRows != 2 || checkpoint.RangeFrom == nil || checkpoint.RangeTo == nil {
		t.Fatal("tenant rollup checkpoint is incomplete")
	}

	insertTx, err := fixture.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := insertTx.Exec(`INSERT INTO inference_requests(id,organisation_id,project_id,environment_id,service_account_id,api_key_id,key_prefix,model_alias,started_at,status,usage_finality) VALUES('req_rollup_concurrent_insert',$1,$2,$3,$4,$5,$6,$7,$8,'in_progress','unknown')`, principal.OrganisationID, principal.ProjectID, principal.EnvironmentID, principal.ServiceAccountID, principal.APIKeyID, principal.KeyPrefix, spec.ModelAlias, now.Add(-5*time.Minute)); err != nil {
		insertTx.Rollback()
		t.Fatal(err)
	}
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now.Add(2*time.Second)); err != nil {
		insertTx.Rollback()
		t.Fatal(err)
	}
	checkpoint, err = fixture.store.GetRollupCheckpoint(ctx, session)
	if err != nil || checkpoint.SourceRows == nil || *checkpoint.SourceRows != 2 {
		insertTx.Rollback()
		t.Fatal("checkpoint and rollup snapshot diverged during concurrent insertion")
	}
	if err := insertTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	checkpoint, err = fixture.store.GetRollupCheckpoint(ctx, session)
	if err != nil || checkpoint.SourceRows == nil || *checkpoint.SourceRows != 3 {
		t.Fatal("next rollup did not reconcile the committed insertion")
	}

	completionTx, err := fixture.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completionTx.Exec(`UPDATE inference_requests SET completed_at=$2,status='blocked',http_status=403,error_class='insufficient_scope' WHERE id=$1`, "req_rollup_concurrent_insert", now); err != nil {
		completionTx.Rollback()
		t.Fatal(err)
	}
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now.Add(4*time.Second)); err != nil {
		completionTx.Rollback()
		t.Fatal(err)
	}
	var snapshotInProgress int64
	if err := fixture.db.QueryRow(`SELECT COALESCE(sum(in_progress_requests),0) FROM usage_rollups_hourly_v2 WHERE organisation_id=$1`, result.OrganisationID).Scan(&snapshotInProgress); err != nil || snapshotInProgress != 1 {
		completionTx.Rollback()
		t.Fatal("rollup did not retain one consistent pre-completion snapshot")
	}
	if err := completionTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	var reconciledBlocked, reconciledInProgress int64
	if err := fixture.db.QueryRow(`SELECT COALESCE(sum(blocked_requests),0),COALESCE(sum(in_progress_requests),0) FROM usage_rollups_hourly_v2 WHERE organisation_id=$1`, result.OrganisationID).Scan(&reconciledBlocked, &reconciledInProgress); err != nil || reconciledBlocked != 1 || reconciledInProgress != 0 {
		t.Fatal("next rollup did not reconcile the committed completion")
	}

	connection, err := fixture.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, `SELECT pg_advisory_lock(719922430168292743)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.RefreshUsageRollups(ctx, now.Add(-time.Hour), now, now.Add(2*time.Second)); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("overlapping worker error=%v", err)
	}
	if _, err := connection.ExecContext(ctx, `SELECT pg_advisory_unlock(719922430168292743)`); err != nil {
		t.Fatal(err)
	}

	targets, err := fixture.store.ListProbeTargets(ctx, now)
	if err != nil || len(targets) != 0 {
		t.Fatal("probe-disabled target became eligible")
	}
	spec.ProbeEnabled = true
	if _, err := fixture.store.Provision(ctx, spec); err != nil {
		t.Fatal(err)
	}
	targets, err = fixture.store.ListProbeTargets(ctx, now)
	if err != nil || len(targets) != 1 || targets[0].ProviderModel != spec.ProviderModel {
		t.Fatal("explicit target probe opt-in was not actionable")
	}
}

func TestPostgresPortalExportIsTenantScopedAndUsesSafeBoundRouteFields(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	a, err := fixture.store.Provision(ctx, databaseSpec("Export Tenant A", "export-a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := fixture.store.Provision(ctx, databaseSpec("Export Tenant B", "export-b"))
	if err != nil {
		t.Fatal(err)
	}
	principalA, _ := fixture.store.Authenticate(ctx, credentials.Digest(a.APIKey))
	principalB, _ := fixture.store.Authenticate(ctx, credentials.Digest(b.APIKey))
	createCompletedRequest(t, fixture.store, principalA, "req_export_a", "safe-chat", now.Add(-time.Minute), "succeeded", "final", 1, int64Pointer(2), int64Pointer(1))
	createCompletedRequest(t, fixture.store, principalB, "req_export_b", "safe-chat", now.Add(-time.Minute), "succeeded", "final", 1, int64Pointer(4), int64Pointer(2))
	filter := platform.UsageFilter{From: now.Add(-time.Hour), To: now.Add(time.Hour), Limit: 100}
	aRows, err := fixture.store.ListPortalExport(ctx, portalSessionFor(a, "Export Tenant A", "export-a"), filter, "json")
	if err != nil || len(aRows) != 1 || aRows[0].RequestID != "req_export_a" {
		t.Fatal("tenant A export scope is incorrect")
	}
	if aRows[0].ModelVersion == nil || aRows[0].ExecutionClass == nil || aRows[0].CapacityMode == nil || *aRows[0].ExecutionClass != "external_pilot" || *aRows[0].CapacityMode != "shared" {
		t.Fatal("bound route export attribution is missing")
	}
	bRows, err := fixture.store.ListPortalExport(ctx, portalSessionFor(b, "Export Tenant B", "export-b"), filter, "json")
	if err != nil || len(bRows) != 1 || bRows[0].RequestID != "req_export_b" {
		t.Fatal("tenant B export received another tenant's row")
	}

	blockedID := "req_export_unrouted"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: blockedID, Principal: principalA, ModelAlias: "absent", StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: blockedID, CompletedAt: now.Add(time.Millisecond), Status: "blocked", HTTPStatus: http.StatusNotFound, ErrorClass: "model_not_authorised", UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	aRows, err = fixture.store.ListPortalExport(ctx, portalSessionFor(a, "Export Tenant A", "export-a"), filter, "csv")
	if err != nil || len(aRows) != 2 {
		t.Fatal("legacy/null export row was not returned")
	}
	for _, row := range aRows {
		if row.RequestID == blockedID && (row.ModelVersion != nil || row.ExecutionClass != nil || row.CapacityMode != nil) {
			t.Fatal("unrouted request received inferred historical route values")
		}
	}
}

func TestPostgresPortalSharedTargetSeparatesRegistryProbeAndTenantInferenceEvidence(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	specA := databaseSpec("Observation Tenant A", "observation-a")
	specA.ProbeEnabled = true
	a, err := fixture.store.Provision(ctx, specA)
	if err != nil {
		t.Fatal(err)
	}
	specB := databaseSpec("Observation Tenant B", "observation-b")
	specB.ProbeEnabled = true
	b, err := fixture.store.Provision(ctx, specB)
	if err != nil {
		t.Fatal(err)
	}
	principalA, err := fixture.store.Authenticate(ctx, credentials.Digest(a.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	createCompletedRequest(t, fixture.store, principalA, "req_observation_a", specA.ModelAlias, now.Add(-time.Minute), "succeeded", "final", 1, int64Pointer(1), int64Pointer(1))
	bSession := portalSessionFor(b, "Observation Tenant B", "observation-b")
	views, err := fixture.store.ListPortalObservations(ctx, bSession, specB.ModelAlias, now)
	if err != nil || len(views) != 1 {
		t.Fatalf("tenant B observations=%d err=%v", len(views), err)
	}
	if views[0].RegistryStatus != "enabled" || views[0].State != "unknown" || views[0].LatestInferenceAt != nil || views[0].LastSuccessAt != nil || views[0].LastObservationAt != nil {
		t.Fatal("tenant B received tenant A inference activity or a fabricated current state")
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET health_status='unavailable',last_health_check_at=$2 WHERE id=$1`, a.TargetID, now); err != nil {
		t.Fatal(err)
	}
	views, err = fixture.store.ListPortalObservations(ctx, bSession, specB.ModelAlias, now)
	if err != nil || views[0].RegistryStatus != "enabled" || views[0].State != "unknown" {
		t.Fatal("global/stale target health was folded into registry policy")
	}
	observation := platform.ProbeObservation{ID: "obs_shared_target", TargetID: a.TargetID, ObservedAt: now, FreshUntil: now.Add(time.Minute), Status: "operational", CredentialAvailable: true, HTTPStatus: http.StatusOK, Latency: 10 * time.Millisecond}
	if err := fixture.store.RecordProbeObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}
	views, err = fixture.store.ListPortalObservations(ctx, bSession, specB.ModelAlias, now.Add(30*time.Second))
	if err != nil || views[0].State != "operational" || views[0].ProbeStatus != "operational" || views[0].Freshness != "fresh" || views[0].LatestInferenceAt != nil || views[0].LastSuccessAt != nil {
		t.Fatal("fresh explicit probe and tenant inference evidence were not separated")
	}
	views, err = fixture.store.ListPortalObservations(ctx, bSession, specB.ModelAlias, now.Add(2*time.Minute))
	if err != nil || views[0].State != "unknown" || views[0].Freshness != "stale" {
		t.Fatal("stale probe continued to imply current readiness")
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET enabled=false WHERE id=$1`, a.TargetID); err != nil {
		t.Fatal(err)
	}
	views, err = fixture.store.ListPortalObservations(ctx, bSession, specB.ModelAlias, now.Add(30*time.Second))
	if err != nil || views[0].RegistryStatus != "unavailable" || views[0].State != "unavailable" {
		t.Fatal("operator registry block did not override observation state")
	}
}

func portalSessionFor(result platform.ProvisionResult, orgName, orgSlug string) platform.PortalSession {
	membership := platform.PortalMembership{ID: "membership", OrganisationID: result.OrganisationID, OrganisationName: orgName, OrganisationSlug: orgSlug, ProjectID: result.ProjectID, ProjectName: "Application", ProjectSlug: "application", EnvironmentID: result.EnvironmentID, EnvironmentName: "Production", EnvironmentSlug: "production", Role: platform.PortalRoleProjectAdmin}
	return platform.PortalSession{User: platform.PortalUser{ID: "user"}, Current: membership, Memberships: []platform.PortalMembership{membership}}
}

func createCompletedRequest(t *testing.T, store *Store, principal platform.Principal, requestID, modelAlias string, started time.Time, status, finality string, attempts int, input, output *int64) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: modelAlias, StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	route, err := store.ResolveRoute(ctx, principal, modelAlias)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetInferenceRequestRoute(ctx, requestID, route.ID); err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= attempts; number++ {
		attemptID := requestID + "_attempt_" + string(rune('0'+number))
		attemptStarted := started.Add(time.Duration(number) * time.Millisecond)
		if err := store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: attemptID, InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: number, StartedAt: attemptStarted}); err != nil {
			t.Fatal(err)
		}
		attemptStatus, class, providerStatus := "failed", "upstream_timeout", 0
		if number == attempts {
			attemptStatus, class, providerStatus = status, "", http.StatusOK
			if status != "succeeded" {
				attemptStatus, class, providerStatus = "failed", "upstream_error", http.StatusBadGateway
			}
		}
		if err := store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: attemptID, CompletedAt: attemptStarted.Add(5 * time.Millisecond), Status: attemptStatus, ProviderHTTPStatus: providerStatus, ErrorClass: class, Duration: 5 * time.Millisecond}); err != nil {
			t.Fatal(err)
		}
	}
	finish := platform.RequestFinish{ID: requestID, CompletedAt: started.Add(20 * time.Millisecond), Status: status, HTTPStatus: http.StatusOK, ExecutedModel: "provider/model", Duration: 20 * time.Millisecond, UsageFinality: finality, Usage: platform.TokenUsage{InputTokens: input, OutputTokens: output}}
	if status != "succeeded" {
		finish.HTTPStatus, finish.ErrorClass, finish.ExecutedModel = http.StatusBadGateway, "upstream_error", ""
	}
	if err := store.CompleteInferenceRequest(ctx, finish); err != nil {
		t.Fatal(err)
	}
}

func int64Pointer(value int64) *int64 { return &value }
