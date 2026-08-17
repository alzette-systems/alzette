package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alzette/internal/control"
	"alzette/internal/credentials"
	"alzette/internal/gateway"
	"alzette/internal/humanauth"
	"alzette/internal/ids"
	"alzette/internal/platform"
	"alzette/migrations"
)

var safeSchema = regexp.MustCompile(`^[a-z0-9_]+$`)

type databaseFixture struct {
	admin, db *sql.DB
	store     *Store
	schema    string
}

func newDatabaseFixture(t *testing.T) *databaseFixture {
	t.Helper()
	fixture := newUnmigratedDatabaseFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := Migrate(ctx, fixture.db); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func newUnmigratedDatabaseFixture(t *testing.T) *databaseFixture {
	t.Helper()
	databaseURL := os.Getenv("ALZETTE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ALZETTE_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	suffix, err := ids.New("test")
	if err != nil {
		t.Fatal(err)
	}
	schema := "alzette_" + suffix
	if !safeSchema.MatchString(schema) {
		t.Fatalf("unsafe schema %q", schema)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(cleanupContext, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
		_ = admin.Close()
	})
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	database, err := Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return &databaseFixture{admin: admin, db: database, store: New(database, false), schema: schema}
}

func databaseSpec(orgName, orgSlug string) platform.ProvisionSpec {
	return platform.ProvisionSpec{OrganisationName: orgName, OrganisationSlug: orgSlug, ProjectName: "Application", ProjectSlug: "application", EnvironmentName: "Production", EnvironmentSlug: "production", ModelAlias: "safe-chat", ModelVersion: "v1", TargetName: "shared-openrouter", ExecutionClass: "external_pilot", CapacityMode: "shared", TargetBaseURL: "https://openrouter.example.invalid/api/v1", ProviderModel: "provider/model", SecretRef: "OPENROUTER_API_KEY", TargetTimeout: time.Second, MaxAttempts: 2, ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead}}
}

func TestPostgresMigrationProvisioningIsolationAndAccounting(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	a, err := fixture.store.Provision(ctx, databaseSpec("Tenant A", "tenant-a"))
	if err != nil {
		t.Fatal(err)
	}
	if !a.KeyCreated || a.APIKey == "" {
		t.Fatal("first provision did not reveal a key")
	}
	again, err := fixture.store.Provision(ctx, databaseSpec("Tenant A", "tenant-a"))
	if err != nil {
		t.Fatal(err)
	}
	if again.KeyCreated || again.APIKey != "" || again.RouteID != a.RouteID || again.KeyPrefix != a.KeyPrefix {
		t.Fatalf("idempotent result=%#v", again)
	}
	b, err := fixture.store.Provision(ctx, databaseSpec("Tenant B", "tenant-b"))
	if err != nil {
		t.Fatal(err)
	}

	var storedHash []byte
	var storedPrefix string
	if err := fixture.db.QueryRow(`SELECT key_hash,key_prefix FROM api_keys WHERE key_prefix=$1`, a.KeyPrefix).Scan(&storedHash, &storedPrefix); err != nil {
		t.Fatal(err)
	}
	digest := credentials.Digest(a.APIKey)
	if !bytes.Equal(storedHash, digest[:]) || storedPrefix != a.KeyPrefix || bytes.Contains(storedHash, []byte(a.APIKey)) {
		t.Fatal("API key was not stored as a one-way digest")
	}
	var contentColumns int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('inference_requests','provider_attempts') AND column_name IN ('prompt','prompt_content','request_body','response','response_content','response_body')`).Scan(&contentColumns); err != nil {
		t.Fatal(err)
	}
	if contentColumns != 0 {
		t.Fatal("content persistence columns exist")
	}

	principalA, err := fixture.store.Authenticate(ctx, digest)
	if err != nil {
		t.Fatal(err)
	}
	principalB, err := fixture.store.Authenticate(ctx, credentials.Digest(b.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	route, err := fixture.store.ResolveRoute(ctx, principalA, "safe-chat")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-time.Second)
	requestID := "req_integration_a"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, route.ID); err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= 2; number++ {
		attemptID := fmt.Sprintf("att_integration_%d", number)
		if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: attemptID, InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: number, StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		status, class := "failed", "upstream_timeout"
		providerStatus := 0
		if number == 2 {
			status, class, providerStatus = "succeeded", "", httpStatusOK
		}
		if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: attemptID, CompletedAt: time.Now().UTC(), Status: status, ProviderHTTPStatus: providerStatus, ErrorClass: class, Duration: 10 * time.Millisecond}); err != nil {
			t.Fatal(err)
		}
	}
	input, output := int64(12), int64(4)
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: time.Now().UTC(), Status: "succeeded", HTTPStatus: httpStatusOK, ExecutedModel: "provider/model", ProviderRequestID: "provider-id", Duration: 25 * time.Millisecond, Usage: platform.TokenUsage{InputTokens: &input, OutputTokens: &output}, UsageFinality: "final"}); err != nil {
		t.Fatal(err)
	}
	pageA, err := fixture.store.ListInferenceRequests(ctx, principalA, platform.UsageFilter{From: started.Add(-time.Minute), To: time.Now().Add(time.Minute), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(pageA.Requests) != 1 || pageA.Requests[0].AttemptCount != 2 {
		t.Fatalf("tenant A requests=%#v", pageA)
	}
	pageB, err := fixture.store.ListInferenceRequests(ctx, principalB, platform.UsageFilter{From: started.Add(-time.Minute), To: time.Now().Add(time.Minute), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(pageB.Requests) != 0 {
		t.Fatalf("tenant B saw tenant A requests: %#v", pageB)
	}
	if _, err := fixture.store.GetInferenceRequest(ctx, principalB, requestID); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("cross-tenant detail error=%v", err)
	}
	var logicalCount, attemptCount int
	if err := fixture.db.QueryRow(`SELECT (SELECT count(*) FROM inference_requests),(SELECT count(*) FROM provider_attempts)`).Scan(&logicalCount, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if logicalCount != 1 || attemptCount != 2 {
		t.Fatalf("logical=%d attempts=%d", logicalCount, attemptCount)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_requests SET error_class='tampered' WHERE id=$1`, requestID); err == nil {
		t.Fatal("completed logical request was mutable")
	}
	if _, err := fixture.db.Exec(`UPDATE provider_attempts SET error_class='tampered' WHERE inference_request_id=$1`, requestID); err == nil {
		t.Fatal("completed provider attempt was mutable")
	}

	dedicated := databaseSpec("Tenant A", "tenant-a")
	dedicated.ModelAlias = "dedicated-chat"
	dedicated.TargetName = "dedicated-private"
	dedicated.ExecutionClass = "private_compatible"
	dedicated.CapacityMode = "dedicated"
	dedicated.CapacityEvidenceRef = "operator-test:evidence"
	dedicated.TargetBaseURL = "https://private-target.example.invalid/v1"
	dedicatedResult, err := fixture.store.Provision(ctx, dedicated)
	if err != nil {
		t.Fatal(err)
	}
	conflict := dedicated
	conflict.OrganisationName = "Tenant C"
	conflict.OrganisationSlug = "tenant-c"
	if _, err := fixture.store.Provision(ctx, conflict); err == nil {
		t.Fatal("dedicated target was rebound to another tenant")
	}
	var tenantCCount int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM organisations WHERE slug='tenant-c'`).Scan(&tenantCCount); err != nil {
		t.Fatal(err)
	}
	if tenantCCount != 0 {
		t.Fatal("failed provision did not roll back")
	}
	aliasGuardID := "req_alias_guard"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: aliasGuardID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.SetInferenceRequestRoute(ctx, aliasGuardID, dedicatedResult.RouteID); err == nil {
		t.Fatal("request accepted a route for a different alias")
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: aliasGuardID, CompletedAt: time.Now().UTC(), Status: "blocked", HTTPStatus: http.StatusNotFound, ErrorClass: "model_not_authorised", UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.db.Exec(`INSERT INTO models(id,alias,version) VALUES ('mdl_trigger_test','trigger-test','v1')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.db.Exec(`INSERT INTO tenant_routes(id,organisation_id,project_id,environment_id,model_id,target_id) VALUES ('rte_trigger_test',$1,$2,$3,'mdl_trigger_test',$4)`, b.OrganisationID, b.ProjectID, b.EnvironmentID, dedicatedResult.TargetID)
	if err == nil {
		t.Fatal("database trigger accepted cross-tenant dedicated binding")
	}

	var auditText string
	if err := fixture.db.QueryRow(`SELECT string_agg(safe_metadata::text,' ') FROM audit_events`).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE audit_events SET action='tampered'`); err == nil {
		t.Fatal("audit event was mutable")
	}
	for label, forbidden := range map[string]string{"upstream host": "openrouter.example.invalid", "Alzette API key": a.APIKey, "provider secret reference": "OPENROUTER_API_KEY"} {
		if bytes.Contains([]byte(auditText), []byte(forbidden)) {
			t.Fatalf("audit leaked %s", label)
		}
	}
	if err := fixture.store.RevokeKey(ctx, a.KeyPrefix); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.Authenticate(ctx, digest); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("revoked authentication error=%v", err)
	}
	rotated, err := fixture.store.RotateKey(ctx, platform.RotateKeySpec{OrganisationSlug: "tenant-a", ProjectSlug: "application", EnvironmentSlug: "production", ServiceAccount: "app", Scopes: []string{platform.ScopeUsageRead}})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.APIKey == "" {
		t.Fatal("rotated key was not revealed")
	}
	if err := Migrate(ctx, fixture.db); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}
}

func TestPostgresSlice0TargetModeAndUsageFinalityGuards(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	provisioned, err := fixture.store.Provision(ctx, databaseSpec("Slice 0 Guards", "slice0-guards"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.db.Exec(`INSERT INTO inference_targets (
		id,name,execution_class,capacity_mode,capacity_evidence_ref,owner_organisation_id,
		base_url,provider_model,secret_ref,timeout_ms,max_attempts
	) VALUES (
		'tgt_invalid_external_dedicated','invalid-external-dedicated','external_pilot','dedicated',
		'operator-test:evidence',$1,'https://invalid.example/v1','provider/model','OPENROUTER_API_KEY',1000,1
	)`, provisioned.OrganisationID); err == nil {
		t.Fatal("database accepted an external pilot as dedicated capacity")
	}
	if _, err := fixture.db.Exec(`INSERT INTO inference_targets (
		id,name,execution_class,capacity_mode,base_url,provider_model,secret_ref,timeout_ms,max_attempts
	) VALUES (
		'tgt_invalid_execution_class','invalid-execution-class','meluxina','shared',
		'https://invalid.example/v1','provider/model','OPENROUTER_API_KEY',1000,1
	)`); err == nil {
		t.Fatal("database accepted an execution class outside the Slice 0 contract")
	}

	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	const requestID = "req_slice0_finality_guard"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: "safe-chat", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	for name, statement := range map[string]string{
		"final without totals":  `UPDATE inference_requests SET usage_finality='final' WHERE id='req_slice0_finality_guard'`,
		"partial without usage": `UPDATE inference_requests SET usage_finality='partial' WHERE id='req_slice0_finality_guard'`,
		"unknown with a token":  `UPDATE inference_requests SET input_tokens=1 WHERE id='req_slice0_finality_guard'`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := fixture.db.Exec(statement); err == nil {
				t.Fatal("database accepted usage fields inconsistent with finality")
			}
		})
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: time.Now().UTC(), Status: "blocked", HTTPStatus: http.StatusNotFound, ErrorClass: "model_not_authorised", UsageFinality: "unknown"}); err != nil {
		t.Fatal("valid unknown-token blocked request was rejected")
	}
}

func TestPostgresDisabledTargetFailsClosedBeforeProviderAttempt(t *testing.T) {
	fixture := newDatabaseFixture(t)
	var upstreamCalls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_, _ = io.WriteString(w, `{"id":"unexpected","model":"provider/model","choices":[{}]}`)
	}))
	t.Cleanup(upstream.Close)
	spec := databaseSpec("Disabled Target", "disabled-target")
	spec.TargetBaseURL = upstream.URL + "/v1"
	provisioned, err := fixture.store.Provision(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET enabled=false WHERE id=$1`, provisioned.TargetID); err != nil {
		t.Fatal(err)
	}
	handler, err := gateway.New(gateway.Config{Store: fixture.store, HTTPClient: upstream.Client(), SecretLookup: func(string) (string, bool) { return "provider-secret", true }})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"safe-chat","messages":[{"role":"user","content":"must not reach disabled target"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+provisioned.APIKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || upstreamCalls.Load() != 0 {
		t.Fatalf("disabled target status=%d calls=%d length=%d", response.Code, upstreamCalls.Load(), response.Body.Len())
	}
	principal, err := fixture.store.Authenticate(context.Background(), credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	page, err := fixture.store.ListInferenceRequests(context.Background(), principal, platform.UsageFilter{From: now.Add(-time.Minute), To: now.Add(time.Minute), Limit: 10})
	if err != nil || len(page.Requests) != 1 || page.Requests[0].Status != "blocked" || page.Requests[0].ErrorClass != "target_unavailable" || page.Requests[0].AttemptCount != 0 {
		t.Fatal("disabled target logical request did not fail closed before an attempt")
	}
}

func TestPostgresBoundTargetCannotBecomeMissing(t *testing.T) {
	fixture := newDatabaseFixture(t)
	provisioned, err := fixture.store.Provision(context.Background(), databaseSpec("Bound Target", "bound-target"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`DELETE FROM inference_targets WHERE id=$1`, provisioned.TargetID); err == nil {
		t.Fatal("database deleted a target that is still referenced by a tenant route")
	}
	var routeTargetID string
	if err := fixture.db.QueryRow(`SELECT target_id FROM tenant_routes WHERE id=$1`, provisioned.RouteID).Scan(&routeTargetID); err != nil {
		t.Fatal(err)
	}
	if routeTargetID != provisioned.TargetID {
		t.Fatal("failed target deletion changed the bound route")
	}
}

func TestPostgresSelfServiceCatalogueAndCapacityContracts(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	shared, err := fixture.store.Provision(ctx, databaseSpec("Catalogue Tenant", "catalogue-tenant"))
	if err != nil {
		t.Fatal(err)
	}

	var accountKind, lifecycle, createdVia, sharedModelID string
	if err := fixture.db.QueryRow(`SELECT account_kind,lifecycle_status,created_via FROM organisations WHERE id=$1`, shared.OrganisationID).Scan(&accountKind, &lifecycle, &createdVia); err != nil {
		t.Fatal(err)
	}
	if accountKind != "customer" || lifecycle != "active" || createdVia != "operator" {
		t.Fatalf("existing organisation migration defaults=%s/%s/%s", accountKind, lifecycle, createdVia)
	}
	if err := fixture.db.QueryRow(`SELECT model_id FROM tenant_routes WHERE id=$1`, shared.RouteID).Scan(&sharedModelID); err != nil {
		t.Fatal(err)
	}

	passwordHash, err := humanauth.HashPassword("catalogue-contract-test-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := fixture.store.ProvisionHuman(ctx, platform.HumanUserSpec{
		Username: "catalogue-admin", DisplayName: "Catalogue Admin", PasswordHash: passwordHash,
		OrganisationSlug: "catalogue-tenant", ProjectSlug: "application", EnvironmentSlug: "production", Role: platform.PortalRoleOrgAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE human_users SET email='admin@example.test',email_normalized='admin@example.test',email_verified_at=now(),identity_origin='self_service' WHERE id=$1`, user.UserID); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.db.Exec(`INSERT INTO catalogue_models(
		id,slug,name,family,description,modalities,capabilities,lifecycle_status,published_at
	) VALUES (
		'cat_shared','shared-evaluation','Shared evaluation','evaluation','Bounded shared evaluation',
		'["text"]','["chat"]','published',now()
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO catalogue_model_versions(
		id,catalogue_model_id,version,routable_model_id,context_window_tokens,
		licence_name,licence_status,support_status,lifecycle_status,source_label,evidence_ref,published_at
	) VALUES (
		'cmv_shared','cat_shared','v1',$1,8192,'operator-reviewed','approved','supported',
		'available','operator-test','operator-test:shared-model',now()
	)`, sharedModelID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_profiles(
		id,catalogue_model_version_id,code,name,service_mode,execution_class,runtime_class,
		min_capacity_units,max_capacity_units,capacity_finality,status,source_label,evidence_ref
	) VALUES (
		'prf_shared','cmv_shared','evaluation','Shared evaluation','shared_evaluation',
		'external_pilot','compatible-provider',1,1,'estimated','quotable','operator-test','operator-test:shared-profile'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO evaluation_offer_templates(
		id,code,name,deployment_profile_id,routable_model_id,target_id,status,is_default,
		request_allowance,token_allowance,rate_limit_requests_per_minute,concurrency_limit,
		expires_after_days,privacy_notice_version,acceptable_use_version,source_label,evidence_ref
	) VALUES (
		'off_eval','evaluation','Evaluation','prf_shared',$1,$2,'enabled',true,
		100,100000,10,2,30,'privacy-v1','acceptable-v1','operator-test','operator-test:evaluation-offer'
	)`, sharedModelID, shared.TargetID); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.db.Exec(`INSERT INTO self_service_registrations(
		id,email,email_normalized,proposed_display_name,proposed_organisation_name,
		privacy_notice_version,acceptable_use_version,accepted_at,verification_token_hash,expires_at
	) VALUES (
		'reg_one','prospect@example.test','prospect@example.test','Prospect','Prospect Company',
		'privacy-v1','acceptable-v1',now(),decode(repeat('01',32),'hex'),now()+interval '1 hour'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO self_service_registrations(
		id,email,email_normalized,proposed_display_name,proposed_organisation_name,
		privacy_notice_version,acceptable_use_version,accepted_at,verification_token_hash,expires_at
	) VALUES (
		'reg_duplicate','prospect@example.test','prospect@example.test','Prospect','Another Company',
		'privacy-v1','acceptable-v1',now(),decode(repeat('02',32),'hex'),now()+interval '1 hour'
	)`); err == nil {
		t.Fatal("database accepted a second active registration for one email")
	}
	if _, err := fixture.db.Exec(`INSERT INTO organisations(
		id,slug,name,account_kind,lifecycle_status,created_via
	) VALUES ('org_evaluation_guard','evaluation-guard','Evaluation Guard','evaluation','evaluation','self_service')`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE organisations SET account_kind='customer',lifecycle_status='active' WHERE id='org_evaluation_guard'`); err == nil {
		t.Fatal("database promoted a self-service organisation without approval evidence")
	}

	dedicatedSpec := databaseSpec("Catalogue Tenant", "catalogue-tenant")
	dedicatedSpec.ModelAlias = "private-chat"
	dedicatedSpec.ModelVersion = "private-v1"
	dedicatedSpec.TargetName = "catalogue-private-target"
	dedicatedSpec.ExecutionClass = "private_compatible"
	dedicatedSpec.CapacityMode = "dedicated"
	dedicatedSpec.CapacityEvidenceRef = "operator-test:dedicated-target"
	dedicatedSpec.TargetBaseURL = "https://catalogue-private.example.invalid/v1"
	dedicatedSpec.ProviderModel = "provider/private-model"
	dedicatedSpec.ServicePlanCode = "dedicated-private"
	dedicatedSpec.ServicePlanName = "Dedicated private"
	dedicatedSpec.ServicePlanSource = "operator-test"
	dedicatedSpec.ServicePlanFinality = "declared"
	dedicatedSpec.DedicatedResourceClass = "gpu-test"
	dedicatedAcceleratorCount := int64(1)
	dedicatedSpec.DedicatedAcceleratorCount = &dedicatedAcceleratorCount
	dedicated, err := fixture.store.Provision(ctx, dedicatedSpec)
	if err != nil {
		t.Fatal(err)
	}
	var dedicatedModelID string
	if err := fixture.db.QueryRow(`SELECT model_id FROM tenant_routes WHERE id=$1`, dedicated.RouteID).Scan(&dedicatedModelID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO catalogue_models(
		id,slug,name,family,description,modalities,capabilities,lifecycle_status,published_at
	) VALUES (
		'cat_private','private-model','Private model','private-family','Dedicated private model',
		'["text"]','["chat"]','published',now()
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO catalogue_model_versions(
		id,catalogue_model_id,version,routable_model_id,context_window_tokens,
		licence_name,licence_status,support_status,lifecycle_status,source_label,evidence_ref,published_at
	) VALUES (
		'cmv_private','cat_private','v1',$1,32768,'operator-reviewed','approved','supported',
		'available','operator-test','operator-test:private-model',now()
	)`, dedicatedModelID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_profiles(
		id,catalogue_model_version_id,code,name,service_mode,execution_class,runtime_class,
		accelerator_class,accelerators_per_unit,accelerator_memory_gib,min_capacity_units,max_capacity_units,
		capacity_finality,status,source_label,evidence_ref
	) VALUES (
		'prf_private','cmv_private','dedicated-unit','Dedicated unit','dedicated_private','private_compatible',
		'vllm-pinned','GPU-Test',1,80,1,4,'measured','quotable','operator-test','operator-test:private-profile'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_profile_metrics(
		id,deployment_profile_id,metric_code,unit,minimum_value,target_value,maximum_value,
		per_capacity_unit,scales_with_units,finality,source_label,evidence_ref,measured_at
	) VALUES (
		'met_private','prf_private','concurrent_requests','requests',1,4,8,true,false,
		'measured','operator-test','operator-test:benchmark',now()
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_profile_prices(
		id,deployment_profile_id,currency,billing_period,recurring_unit_amount_minor,
		setup_amount_minor,visibility,finality,source_label,evidence_ref,effective_from
	) VALUES (
		'prc_private','prf_private','EUR','month',100000,25000,'authenticated','contractual',
		'operator-test','operator-test:price',now()-interval '1 day'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO evaluation_offer_templates(
		id,code,name,deployment_profile_id,routable_model_id,target_id,status,is_default,
		request_allowance,rate_limit_requests_per_minute,concurrency_limit,expires_after_days,
		privacy_notice_version,acceptable_use_version,source_label,evidence_ref
	) VALUES (
		'off_invalid','invalid-private','Invalid private offer','prf_private',$1,$2,'disabled',false,
		10,2,1,7,'privacy-v1','acceptable-v1','operator-test','operator-test:invalid'
	)`, dedicatedModelID, dedicated.TargetID); err == nil {
		t.Fatal("database accepted a dedicated profile/target as a shared evaluation offer")
	}

	quoteInsert := `INSERT INTO deployment_quotes(
		id,organisation_id,project_id,environment_id,quote_version,quote_kind,
		deployment_profile_id,profile_price_id,capacity_units,service_mode_snapshot,
		execution_class_snapshot,accelerator_class_snapshot,accelerator_count_snapshot,
		capacity_snapshot,currency,billing_period,recurring_unit_amount_minor,
		recurring_total_amount_minor,setup_total_amount_minor,tax_treatment,price_finality,
		status,source_label,evidence_ref,offered_at,expires_at
	) VALUES ($1,$2,$3,$4,1,$5,'prf_private','prc_private',$6,'dedicated_private',
		'private_compatible','GPU-Test',$6,jsonb_build_object('concurrent_requests',4*$6),
		'EUR','month',100000,100000*$6,25000,'exclusive','contractual','offered',
		'operator-test','operator-test:quote',now(),now()+interval '7 days')`
	if _, err := fixture.db.Exec(quoteInsert, "qte_initial", shared.OrganisationID, shared.ProjectID, shared.EnvironmentID, "new_endpoint", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE deployment_quotes SET status='accepted',accepted_at=now(),accepted_by_user_id=$2 WHERE id=$1`, "qte_initial", user.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE deployment_quotes SET recurring_total_amount_minor=1 WHERE id='qte_initial'`); err == nil {
		t.Fatal("database mutated an accepted deployment quote")
	}

	if _, err := fixture.db.Exec(`INSERT INTO model_deployments(
		id,organisation_id,project_id,environment_id,deployment_profile_id,quote_id,
		target_id,route_id,state,validation_evidence_ref,last_verified_at,ready_at
	) VALUES (
		'dpl_private',$1,$2,$3,'prf_private','qte_initial',$4,$5,'ready',
		'operator-test:deployment-ready',now(),now()
	)`, shared.OrganisationID, shared.ProjectID, shared.EnvironmentID, dedicated.TargetID, dedicated.RouteID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_capacity_revisions(
		id,organisation_id,project_id,environment_id,deployment_id,quote_id,capacity_units,
		state,resource_evidence_ref,effective_at
	) VALUES (
		'cap_initial',$1,$2,$3,'dpl_private','qte_initial',1,'active','operator-test:gpu-one',now()
	)`, shared.OrganisationID, shared.ProjectID, shared.EnvironmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_capacity_revisions(
		id,organisation_id,project_id,environment_id,deployment_id,quote_id,capacity_units,
		state,resource_evidence_ref,effective_at
	) VALUES (
		'cap_duplicate',$1,$2,$3,'dpl_private','qte_initial',1,'active','operator-test:gpu-duplicate',now()
	)`, shared.OrganisationID, shared.ProjectID, shared.EnvironmentID); err == nil {
		t.Fatal("database accepted two active capacity revisions")
	}

	if _, err := fixture.db.Exec(quoteInsert, "qte_scale", shared.OrganisationID, shared.ProjectID, shared.EnvironmentID, "scale_up", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE deployment_quotes SET status='accepted',accepted_at=now(),accepted_by_user_id=$2 WHERE id=$1`, "qte_scale", user.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO deployment_requests(
		id,organisation_id,project_id,environment_id,request_kind,deployment_profile_id,
		deployment_id,quote_id,current_capacity_units,requested_capacity_units,status,
		requested_by_user_id,submitted_at
	) VALUES (
		'dpr_scale',$1,$2,$3,'scale_up','prf_private','dpl_private','qte_scale',1,2,
		'accepted',$4,now()
	)`, shared.OrganisationID, shared.ProjectID, shared.EnvironmentID, user.UserID); err != nil {
		t.Fatal(err)
	}
	tx, err := fixture.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE deployment_capacity_revisions SET state='superseded',ended_at=now() WHERE id='cap_initial'`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO deployment_capacity_revisions(
		id,organisation_id,project_id,environment_id,deployment_id,quote_id,capacity_units,
		state,resource_evidence_ref,effective_at
	) VALUES (
		'cap_scaled',$1,$2,$3,'dpl_private','qte_scale',2,'active','operator-test:gpu-two',now()
	)`, shared.OrganisationID, shared.ProjectID, shared.EnvironmentID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var activeUnits int
	var stableRoute string
	if err := fixture.db.QueryRow(`SELECT r.capacity_units,d.route_id
		FROM deployment_capacity_revisions r
		JOIN model_deployments d ON d.id=r.deployment_id
		WHERE r.deployment_id='dpl_private' AND r.state='active'`).Scan(&activeUnits, &stableRoute); err != nil {
		t.Fatal(err)
	}
	if activeUnits != 2 || stableRoute != dedicated.RouteID {
		t.Fatalf("scaled endpoint units=%d route=%q", activeUnits, stableRoute)
	}

	other, err := fixture.store.Provision(ctx, databaseSpec("Catalogue Tenant B", "catalogue-tenant-b"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(quoteInsert, "qte_cross_scope", other.OrganisationID, shared.ProjectID, shared.EnvironmentID, "new_endpoint", 1); err == nil {
		t.Fatal("database accepted a quote with cross-tenant project/environment scope")
	}
}

func TestPostgresMigrationDownIsSafeInIsolatedSchema(t *testing.T) {
	fixture := newDatabaseFixture(t)
	if _, err := fixture.db.Exec(migrationScript(t, "0010_capacity_request_intent.down.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0009_endpoint_billing_control_plane.down.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0008_self_service_catalogue.down.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0007_slice2_contract_closure.down.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0006_usage_rollups_and_target_probes.down.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0005_portal_identity_and_service_plans.down.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0004_slice0_contract_guards.down.sql")); err != nil {
		t.Fatal(err)
	}
	var slice0Constraint bool
	if err := fixture.db.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM pg_constraint
		 WHERE conrelid = 'inference_requests'::regclass
		   AND conname = 'inference_requests_usage_finality_0004_check'
	)`).Scan(&slice0Constraint); err != nil {
		t.Fatal(err)
	}
	if slice0Constraint {
		t.Fatal("0004 down left its usage finality constraint installed")
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0003_route_binding_observations.down.sql")); err != nil {
		t.Fatal(err)
	}
	var bindingColumn bool
	if err := fixture.db.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		 WHERE table_schema=current_schema()
		   AND table_name='tenant_routes'
		   AND column_name='binding_generation'
	)`).Scan(&bindingColumn); err != nil {
		t.Fatal(err)
	}
	if bindingColumn {
		t.Fatal("0003 down left its route binding generation installed")
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0002_ledger_integrity.down.sql")); err != nil {
		t.Fatal(err)
	}
	var integrityConstraint bool
	if err := fixture.db.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM pg_constraint
		 WHERE conrelid = 'inference_requests'::regclass
		   AND conname = 'inference_requests_api_key_tuple_0002_fkey'
	)`).Scan(&integrityConstraint); err != nil {
		t.Fatal(err)
	}
	if integrityConstraint {
		t.Fatal("0002 down left its request/key integrity constraint installed")
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0001_openrouter_poc.down.sql")); err != nil {
		t.Fatal(err)
	}
	var table sql.NullString
	if err := fixture.db.QueryRow(`SELECT to_regclass(current_schema() || '.organisations')`).Scan(&table); err != nil {
		t.Fatal(err)
	}
	if table.Valid {
		t.Fatalf("organisations table remains: %s", table.String)
	}
	if err := Migrate(context.Background(), fixture.db); err != nil {
		t.Fatalf("reapply after down migration: %v", err)
	}
	if err := fixture.db.QueryRow(`SELECT to_regclass(current_schema() || '.organisations')`).Scan(&table); err != nil {
		t.Fatal(err)
	}
	if !table.Valid {
		t.Fatal("migration did not reapply after down")
	}
	var versions int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version IN ('0001_openrouter_poc','0002_ledger_integrity','0003_route_binding_observations','0004_slice0_contract_guards','0005_portal_identity_and_service_plans','0006_usage_rollups_and_target_probes','0007_slice2_contract_closure','0008_self_service_catalogue','0009_endpoint_billing_control_plane','0010_capacity_request_intent')`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 10 {
		t.Fatalf("reapplied migration versions=%d", versions)
	}
}

func TestPostgresMigrationUpgradesExisting0001Schema(t *testing.T) {
	fixture := newUnmigratedDatabaseFixture(t)
	ctx := context.Background()
	if _, err := fixture.db.Exec(`CREATE TABLE schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0001_openrouter_poc.up.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`INSERT INTO schema_migrations(version) VALUES ('0001_openrouter_poc')`); err != nil {
		t.Fatal(err)
	}

	provisioned, err := fixture.store.Provision(ctx, databaseSpec("Upgrade Tenant", "upgrade-tenant"))
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	route := platform.Route{ID: provisioned.RouteID, Target: platform.Target{ID: provisioned.TargetID}}
	started := time.Now().UTC().Add(-time.Second)
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: "req_upgrade_existing", Principal: principal, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_requests SET route_id=$2 WHERE id=$1`, "req_upgrade_existing", route.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_upgrade_existing", InferenceRequestID: "req_upgrade_existing", TargetID: route.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	completed := time.Now().UTC()
	if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: "att_upgrade_existing", CompletedAt: completed, Status: "succeeded", ProviderHTTPStatus: http.StatusOK, Duration: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: "req_upgrade_existing", CompletedAt: completed, Status: "succeeded", HTTPStatus: http.StatusOK, UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	replacementSpec := databaseSpec("Upgrade Tenant", "upgrade-tenant")
	replacementSpec.TargetName = "upgrade-replacement-target"
	replacementSpec.ProviderModel = "provider/replacement"
	replacementSpec.TargetBaseURL = "https://replacement.example.invalid/api/v1"
	replacement, err := fixture.store.Provision(ctx, replacementSpec)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.RouteID != route.ID || replacement.TargetID == route.Target.ID {
		t.Fatal("0001 fixture did not establish a completed historical attempt followed by route retarget")
	}

	if err := Migrate(ctx, fixture.db); err != nil {
		t.Fatalf("upgrade existing 0001 schema: %v", err)
	}
	var versions int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version IN ('0001_openrouter_poc','0002_ledger_integrity','0003_route_binding_observations','0004_slice0_contract_guards','0005_portal_identity_and_service_plans','0006_usage_rollups_and_target_probes','0007_slice2_contract_closure','0008_self_service_catalogue','0009_endpoint_billing_control_plane','0010_capacity_request_intent')`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 10 {
		t.Fatalf("upgraded migration versions=%d", versions)
	}
	var requestCount, attemptCount int
	if err := fixture.db.QueryRow(`SELECT
		(SELECT count(*) FROM inference_requests WHERE id='req_upgrade_existing'),
		(SELECT count(*) FROM provider_attempts WHERE id='att_upgrade_existing')`).Scan(&requestCount, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if requestCount != 1 || attemptCount != 1 {
		t.Fatalf("upgrade did not preserve ledger rows requests=%d attempts=%d", requestCount, attemptCount)
	}
	var currentRouteTarget, historicalAttemptTarget string
	if err := fixture.db.QueryRow(`SELECT target_id FROM tenant_routes WHERE id=$1`, route.ID).Scan(&currentRouteTarget); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.QueryRow(`SELECT target_id FROM provider_attempts WHERE id='att_upgrade_existing'`).Scan(&historicalAttemptTarget); err != nil {
		t.Fatal(err)
	}
	if currentRouteTarget != replacement.TargetID || historicalAttemptTarget != route.Target.ID {
		t.Fatal("0001 upgrade did not preserve retargeted route and historical attempt meaning")
	}
	var boundTarget, boundModel string
	var requestGeneration, routeGeneration int64
	if err := fixture.db.QueryRow(`SELECT ir.bound_target_id,ir.bound_model_id,ir.route_binding_generation,r.binding_generation
		FROM inference_requests ir JOIN tenant_routes r ON r.id=ir.route_id
		WHERE ir.id='req_upgrade_existing'`).Scan(&boundTarget, &boundModel, &requestGeneration, &routeGeneration); err != nil {
		t.Fatal(err)
	}
	if boundTarget != historicalAttemptTarget || boundModel == "" || requestGeneration != 1 || routeGeneration != 2 {
		t.Fatal("0003 upgrade did not isolate historical route evidence from the migrated current generation")
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET provider_model='provider/tampered' WHERE id=$1`, historicalAttemptTarget); err == nil {
		t.Fatal("0002 upgrade did not freeze historically used target configuration")
	}
}

func TestEndpointControlMigrationDisambiguatesLegacyKeyNames(t *testing.T) {
	fixture := newUnmigratedDatabaseFixture(t)
	ctx := context.Background()
	if _, err := fixture.db.Exec(`CREATE TABLE schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		t.Fatal(err)
	}
	versions := []string{
		"0001_openrouter_poc",
		"0002_ledger_integrity",
		"0003_route_binding_observations",
		"0004_slice0_contract_guards",
		"0005_portal_identity_and_service_plans",
		"0006_usage_rollups_and_target_probes",
		"0007_slice2_contract_closure",
		"0008_self_service_catalogue",
	}
	for _, version := range versions {
		if _, err := fixture.db.Exec(migrationScript(t, version+".up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`INSERT INTO schema_migrations(version) VALUES ($1)`, version); err != nil {
			t.Fatal(err)
		}
	}
	provisioned, err := fixture.store.Provision(ctx, databaseSpec("Legacy Key Names", "legacy-key-names"))
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 2; index++ {
		token := fmt.Sprintf("legacy-default-key-%d", index)
		digest := credentials.Digest(token)
		if _, err := fixture.db.Exec(`INSERT INTO api_keys(id,service_account_id,key_prefix,key_hash,scopes,name) VALUES($1,$2,$3,$4,'["inference:write"]','default')`, fmt.Sprintf("key_legacy_default_%d", index), provisioned.ServiceAccountID, fmt.Sprintf("alz_k_legacy_default_%d", index), digest[:]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.db.Exec(migrationScript(t, "0009_endpoint_billing_control_plane.up.sql")); err != nil {
		t.Fatalf("upgrade with legacy duplicate key names: %v", err)
	}
	var total, distinct int
	if err := fixture.db.QueryRow(`SELECT count(*),count(DISTINCT name) FROM api_keys WHERE service_account_id=$1`, provisioned.ServiceAccountID).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total != distinct || total != 3 {
		t.Fatalf("legacy key names were not preserved uniquely: total=%d distinct=%d", total, distinct)
	}
	digest := credentials.Digest("another-default-key")
	if _, err := fixture.db.Exec(`INSERT INTO api_keys(id,service_account_id,key_prefix,key_hash,scopes,name) VALUES('key_duplicate_name',$1,'alz_k_duplicate_name',$2,'["inference:write"]','default')`, provisioned.ServiceAccountID, digest[:]); err == nil {
		t.Fatal("0009 accepted a duplicate key name after upgrade")
	}
}

func TestPostgresMigrationUpgradesExisting0002RouteHistory(t *testing.T) {
	fixture := newUnmigratedDatabaseFixture(t)
	ctx := context.Background()
	if _, err := fixture.db.Exec(`CREATE TABLE schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_openrouter_poc", "0002_ledger_integrity"} {
		if _, err := fixture.db.Exec(migrationScript(t, version+".up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`INSERT INTO schema_migrations(version) VALUES ($1)`, version); err != nil {
			t.Fatal(err)
		}
	}

	base := databaseSpec("Existing 0002 Tenant", "existing-0002")
	base.ModelAlias = "existing-0002-chat"
	base.TargetName = "existing-0002-old"
	base.ProviderModel = "provider/old"
	provisioned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-time.Second)
	const requestID = "req_existing_0002_history"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: base.ModelAlias, StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_requests SET route_id=$2 WHERE id=$1`, requestID, provisioned.RouteID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_existing_0002_history", InferenceRequestID: requestID, TargetID: provisioned.TargetID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	completed := time.Now().UTC()
	if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: "att_existing_0002_history", CompletedAt: completed, Status: "succeeded", ProviderHTTPStatus: http.StatusOK, Duration: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: completed, Status: "succeeded", HTTPStatus: http.StatusOK, UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	replacement := base
	replacement.TargetName = "existing-0002-new"
	replacement.TargetBaseURL = "https://existing-0002-new.example.invalid/api/v1"
	replacement.ProviderModel = "provider/new"
	retargeted, err := fixture.store.Provision(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if retargeted.RouteID != provisioned.RouteID || retargeted.TargetID == provisioned.TargetID {
		t.Fatal("0002 fixture did not establish a same-route target replacement")
	}
	returned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if returned.RouteID != provisioned.RouteID || returned.TargetID != provisioned.TargetID {
		t.Fatal("0002 fixture did not establish the historical A -> B -> A route cycle")
	}
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: "req_existing_0002_active", Principal: principal, ModelAlias: base.ModelAlias, StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_requests SET route_id=$2 WHERE id=$1`, "req_existing_0002_active", returned.RouteID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: "req_existing_0002_blocked", Principal: principal, ModelAlias: "not-authorised", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: "req_existing_0002_blocked", CompletedAt: time.Now().UTC(), Status: "blocked", HTTPStatus: http.StatusNotFound, ErrorClass: "model_not_authorised", UsageFinality: "unknown"}); err != nil {
		t.Fatal("0002 blocked request could not complete without a route")
	}

	if err := Migrate(ctx, fixture.db); err != nil {
		t.Fatalf("upgrade existing 0002 schema: %v", err)
	}
	var versions int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version IN ('0001_openrouter_poc','0002_ledger_integrity','0003_route_binding_observations','0004_slice0_contract_guards')`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 4 {
		t.Fatalf("upgraded migration versions=%d", versions)
	}
	var boundTarget, currentTarget string
	var requestGeneration, routeGeneration int64
	if err := fixture.db.QueryRow(`SELECT ir.bound_target_id,ir.route_binding_generation,r.target_id,r.binding_generation
		FROM inference_requests ir JOIN tenant_routes r ON r.id=ir.route_id
		WHERE ir.id=$1`, requestID).Scan(&boundTarget, &requestGeneration, &currentTarget, &routeGeneration); err != nil {
		t.Fatal(err)
	}
	if boundTarget != provisioned.TargetID || currentTarget != returned.TargetID || boundTarget != currentTarget || requestGeneration != 1 || routeGeneration != 2 {
		t.Fatal("0003 did not isolate same-target historical evidence from the migrated current route generation")
	}
	var activeTarget, activeModel string
	var activeGeneration int64
	if err := fixture.db.QueryRow(`SELECT bound_target_id,bound_model_id,route_binding_generation
		FROM inference_requests WHERE id='req_existing_0002_active'`).Scan(&activeTarget, &activeModel, &activeGeneration); err != nil {
		t.Fatal(err)
	}
	if activeTarget != returned.TargetID || activeModel == "" || activeGeneration != routeGeneration {
		t.Fatal("0003 did not bind an active zero-attempt request to the current route generation")
	}
	var blockedRoute, blockedTarget, blockedModel sql.NullString
	var blockedGeneration sql.NullInt64
	if err := fixture.db.QueryRow(`SELECT route_id,bound_target_id,bound_model_id,route_binding_generation
		FROM inference_requests WHERE id='req_existing_0002_blocked'`).Scan(&blockedRoute, &blockedTarget, &blockedModel, &blockedGeneration); err != nil {
		t.Fatal(err)
	}
	if blockedRoute.Valid || blockedTarget.Valid || blockedModel.Valid || blockedGeneration.Valid {
		t.Fatal("0003 invented a route binding for an intentionally blocked request")
	}
	if err := Migrate(ctx, fixture.db); err != nil {
		t.Fatalf("idempotent 0003 migration: %v", err)
	}
}

func TestPostgresLedgerIntegrityRejectsCrossTupleAndLifecycleWrites(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	a, err := fixture.store.Provision(ctx, databaseSpec("Integrity A", "integrity-a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := fixture.store.Provision(ctx, databaseSpec("Integrity B", "integrity-b"))
	if err != nil {
		t.Fatal(err)
	}
	principalA, err := fixture.store.Authenticate(ctx, credentials.Digest(a.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	principalB, err := fixture.store.Authenticate(ctx, credentials.Digest(b.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	routeA, err := fixture.store.ResolveRoute(ctx, principalA, "safe-chat")
	if err != nil {
		t.Fatal(err)
	}
	alternateSpec := databaseSpec("Integrity A", "integrity-a")
	alternateSpec.ModelAlias = "alternate-chat"
	alternateSpec.TargetName = "integrity-alternate-target"
	alternateSpec.ProviderModel = "provider/alternate"
	alternate, err := fixture.store.Provision(ctx, alternateSpec)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now().UTC().Add(-time.Second)
	t.Run("mismatched API key tuple", func(t *testing.T) {
		_, err := fixture.db.Exec(`INSERT INTO inference_requests (
			id,organisation_id,project_id,environment_id,service_account_id,
			api_key_id,key_prefix,model_alias,started_at
		) VALUES ('req_bad_key',$1,$2,$3,$4,$5,$6,'safe-chat',$7)`,
			principalA.OrganisationID, principalA.ProjectID, principalA.EnvironmentID,
			principalA.ServiceAccountID, principalB.APIKeyID, principalB.KeyPrefix, started)
		if err == nil {
			t.Fatal("request with a mismatched API key tuple was accepted")
		}
	})

	t.Run("provider attempt target", func(t *testing.T) {
		const requestID = "req_bad_attempt_target"
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, routeA.ID); err != nil {
			t.Fatal(err)
		}
		tx, err := fixture.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`UPDATE inference_requests SET attempt_count=1 WHERE id=$1`, requestID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO provider_attempts(id,inference_request_id,target_id,attempt_number,started_at)
			VALUES ('att_bad_target',$1,$2,1,$3)`, requestID, alternate.TargetID, started.Add(time.Millisecond)); err == nil {
			t.Fatal("provider attempt with the wrong route target was accepted")
		}
	})

	t.Run("provider attempt completed parent", func(t *testing.T) {
		const requestID = "req_completed_parent"
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, routeA.ID); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: time.Now().UTC(), Status: "failed", HTTPStatus: http.StatusBadGateway, ErrorClass: "upstream_error", UsageFinality: "unknown"}); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`INSERT INTO provider_attempts(id,inference_request_id,target_id,attempt_number,started_at)
			VALUES ('att_completed_parent',$1,$2,1,$3)`, requestID, routeA.Target.ID, time.Now().UTC()); err == nil {
			t.Fatal("provider attempt on a completed request was accepted")
		}
	})

	t.Run("rollup tenant route tuple", func(t *testing.T) {
		bucket := time.Now().UTC().Truncate(time.Hour)
		_, err := fixture.db.Exec(`INSERT INTO usage_rollups_hourly (
			organisation_id,project_id,environment_id,route_id,bucket_start,
			logical_requests,successful_requests,failed_requests,blocked_requests,finality,refreshed_at
		) VALUES ($1,$2,$3,$4,$5,1,1,0,0,'final',$6)`,
			principalB.OrganisationID, principalB.ProjectID, principalB.EnvironmentID, routeA.ID, bucket, time.Now().UTC())
		if err == nil {
			t.Fatal("rollup with a mismatched tenant route tuple was accepted")
		}
	})

	t.Run("audit organisation project tuple", func(t *testing.T) {
		_, err := fixture.db.Exec(`INSERT INTO audit_events (
			id,actor_type,actor_id,organisation_id,project_id,action,result,correlation_id
		) VALUES ('aud_bad_tuple','operator','test',$1,$2,'test.invalid','failed','op_bad_tuple')`,
			principalA.OrganisationID, principalB.ProjectID)
		if err == nil {
			t.Fatal("audit event with a mismatched organisation/project tuple was accepted")
		}
	})

	t.Run("completion with in-progress attempt", func(t *testing.T) {
		const requestID = "req_incomplete_attempt"
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, routeA.ID); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_still_running", InferenceRequestID: requestID, TargetID: routeA.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`UPDATE inference_requests
			SET status='failed',completed_at=now(),http_status=502
			WHERE id=$1`, requestID); err == nil {
			t.Fatal("request completed while a provider attempt was in progress")
		}
	})

	t.Run("completion with count mismatch", func(t *testing.T) {
		const requestID = "req_bad_completion_count"
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, routeA.ID); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_bad_completion_count", InferenceRequestID: requestID, TargetID: routeA.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: "att_bad_completion_count", CompletedAt: time.Now().UTC(), Status: "failed", ErrorClass: "upstream_timeout", Duration: time.Millisecond}); err != nil {
			t.Fatal(err)
		}
		tx, err := fixture.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`UPDATE inference_requests SET attempt_count=2 WHERE id=$1`, requestID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`UPDATE inference_requests SET status='failed',completed_at=now(),http_status=502 WHERE id=$1`, requestID); err == nil {
			t.Fatal("request completed with a mismatched provider attempt count")
		}
	})

	t.Run("attempt count commit consistency", func(t *testing.T) {
		const requestID = "req_count_without_attempt"
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, routeA.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`UPDATE inference_requests SET attempt_count=1 WHERE id=$1`, requestID); err == nil {
			t.Fatal("request attempt count committed without a matching provider attempt")
		}
	})

	t.Run("attempt numbering is contiguous", func(t *testing.T) {
		const requestID = "req_skipped_attempt_number"
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principalA, ModelAlias: "safe-chat", StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, routeA.ID); err != nil {
			t.Fatal(err)
		}
		tx, err := fixture.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`UPDATE inference_requests SET attempt_count=1 WHERE id=$1`, requestID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO provider_attempts(id,inference_request_id,target_id,attempt_number,started_at)
			VALUES ('att_skipped_number',$1,$2,2,$3)`, requestID, routeA.Target.ID, started.Add(time.Millisecond)); err == nil {
			t.Fatal("provider attempt numbering skipped the first attempt")
		}
	})
}

func TestPostgresRouteAttachmentAndActiveRetargetLifecycle(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	base := databaseSpec("Route Lifecycle", "route-lifecycle")
	base.ModelAlias = "mutable-chat"
	base.TargetName = "mutable-target-old"
	base.ProviderModel = "provider/old"
	provisioned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	alternateSpec := base
	alternateSpec.ModelAlias = "alternate-chat"
	alternateSpec.TargetName = "mutable-target-new"
	alternateSpec.ProviderModel = "provider/new"
	alternate, err := fixture.store.Provision(ctx, alternateSpec)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	route, err := fixture.store.ResolveRoute(ctx, principal, "mutable-chat")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-time.Second)
	if _, err := fixture.db.Exec(`INSERT INTO inference_requests (
		id,organisation_id,project_id,environment_id,route_id,service_account_id,
		api_key_id,key_prefix,model_alias,started_at
	) VALUES ('req_created_routed',$1,$2,$3,$4,$5,$6,$7,'mutable-chat',$8)`,
		principal.OrganisationID, principal.ProjectID, principal.EnvironmentID, route.ID,
		principal.ServiceAccountID, principal.APIKeyID, principal.KeyPrefix, started); err == nil {
		t.Fatal("inference request was created with a route instead of using one-time attachment")
	}
	const requestID = "req_route_lifecycle"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: "mutable-chat", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, route.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_requests SET route_id=$2 WHERE id=$1`, requestID, alternate.RouteID); err == nil {
		t.Fatal("attached request route was changed")
	}
	if _, err := fixture.db.Exec(`UPDATE inference_requests SET route_id=NULL WHERE id=$1`, requestID); err == nil {
		t.Fatal("attached request route was cleared")
	}
	if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_route_lifecycle", InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: "att_route_lifecycle", CompletedAt: time.Now().UTC(), Status: "failed", ErrorClass: "upstream_timeout", Duration: time.Millisecond}); err != nil {
		t.Fatal(err)
	}

	var alternateModelID string
	if err := fixture.db.QueryRow(`SELECT model_id FROM tenant_routes WHERE id=$1`, alternate.RouteID).Scan(&alternateModelID); err != nil {
		t.Fatal(err)
	}
	for label, statement := range map[string]string{
		"target": `UPDATE tenant_routes SET target_id='` + alternate.TargetID + `' WHERE id='` + route.ID + `'`,
		"model":  `UPDATE tenant_routes SET model_id='` + alternateModelID + `' WHERE id='` + route.ID + `'`,
	} {
		if _, err := fixture.db.Exec(statement); err == nil {
			t.Fatalf("active route %s change was accepted", label)
		}
	}
	if _, err := fixture.db.Exec(`UPDATE tenant_routes
		SET organisation_id=$2,project_id=$3,environment_id=$4 WHERE id=$1`,
		route.ID, "org_nonexistent", "prj_nonexistent", "env_nonexistent"); err == nil {
		t.Fatal("active route tenant scope change was accepted")
	}

	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: time.Now().UTC(), Status: "failed", HTTPStatus: http.StatusGatewayTimeout, ErrorClass: "upstream_timeout", UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE tenant_routes SET target_id=$2 WHERE id=$1`, route.ID, alternate.TargetID); err != nil {
		t.Fatalf("completed historical request prevented a later route retarget: %v", err)
	}
	var historicalTarget string
	if err := fixture.db.QueryRow(`SELECT target_id FROM provider_attempts WHERE id='att_route_lifecycle'`).Scan(&historicalTarget); err != nil {
		t.Fatal(err)
	}
	if historicalTarget != route.Target.ID {
		t.Fatal("route retarget changed the historical provider attempt target")
	}
}

func TestPostgresTargetExecutionConfigurationIsImmutableWhenActiveOrHistorical(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	base := databaseSpec("Target Lifecycle", "target-lifecycle")
	base.ModelAlias = "target-chat"
	base.TargetName = "target-lifecycle-old"
	base.ProviderModel = "provider/old"
	provisioned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	route, err := fixture.store.ResolveRoute(ctx, principal, "target-chat")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-time.Second)
	const requestID = "req_target_config_lifecycle"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: "target-chat", StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET base_url='https://changed.example.invalid/v1' WHERE id=$1`, route.Target.ID); err == nil {
		t.Fatal("target configuration changed while an unresolved matching request was in progress")
	}
	if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, route.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET provider_model='provider/changed' WHERE id=$1`, route.Target.ID); err == nil {
		t.Fatal("target configuration changed while a routed request was in progress before its first attempt")
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET enabled=false,health_status='degraded',last_health_check_at=now() WHERE id=$1`, route.Target.ID); err != nil {
		t.Fatalf("enabled/health observation fields should remain mutable: %v", err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET enabled=true WHERE id=$1`, route.Target.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_target_config_lifecycle", InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: "att_target_config_lifecycle", CompletedAt: time.Now().UTC(), Status: "failed", ErrorClass: "upstream_timeout", Duration: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: time.Now().UTC(), Status: "failed", HTTPStatus: http.StatusGatewayTimeout, ErrorClass: "upstream_timeout", UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE inference_targets SET secret_ref='REPLACED_SECRET' WHERE id=$1`, route.Target.ID); err == nil {
		t.Fatal("historically used target execution configuration was changed")
	}

	replacement := base
	replacement.TargetName = "target-lifecycle-replacement"
	replacement.ProviderModel = "provider/replacement"
	replacement.TargetBaseURL = "https://replacement.example.invalid/api/v1"
	replacement.SecretRef = "REPLACEMENT_PROVIDER_KEY"
	replacementResult, err := fixture.store.Provision(ctx, replacement)
	if err != nil {
		t.Fatalf("create replacement target and retarget completed route: %v", err)
	}
	if replacementResult.RouteID != route.ID || replacementResult.TargetID == route.Target.ID {
		t.Fatal("supported replacement path did not retain the route and create a new target")
	}
	var currentTarget, historicalTarget string
	if err := fixture.db.QueryRow(`SELECT target_id FROM tenant_routes WHERE id=$1`, route.ID).Scan(&currentTarget); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.QueryRow(`SELECT target_id FROM provider_attempts WHERE id='att_target_config_lifecycle'`).Scan(&historicalTarget); err != nil {
		t.Fatal(err)
	}
	if currentTarget != replacementResult.TargetID || historicalTarget != route.Target.ID {
		t.Fatal("new-target retarget path did not preserve historical attempt meaning")
	}
}

func TestPostgresResolveRouteSerializesConcurrentRetargetAndAttemptsFailClosed(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	base := databaseSpec("Retarget Race", "retarget-race")
	base.ModelAlias = "race-chat"
	base.TargetName = "race-target-old"
	base.ProviderModel = "provider/old"
	provisioned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	replacementSpec := base
	replacementSpec.ModelAlias = "race-replacement-chat"
	replacementSpec.TargetName = "race-target-new"
	replacementSpec.ProviderModel = "provider/new"
	replacement, err := fixture.store.Provision(ctx, replacementSpec)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	oldRoute, err := fixture.store.ResolveRoute(ctx, principal, "race-chat")
	if err != nil {
		t.Fatal(err)
	}

	operatorTx, err := fixture.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer operatorTx.Rollback()
	if _, err := operatorTx.Exec(`UPDATE tenant_routes SET target_id=$2 WHERE id=$1`, oldRoute.ID, replacement.TargetID); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	const requestID = "req_concurrent_retarget"
	if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: "race-chat", StartedAt: started}); err != nil {
		t.Fatal(err)
	}

	type routeResult struct {
		route platform.Route
		err   error
	}
	resolved := make(chan routeResult, 1)
	go func() {
		resolveContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		route, err := fixture.store.ResolveRoute(resolveContext, principal, "race-chat")
		resolved <- routeResult{route: route, err: err}
	}()
	select {
	case result := <-resolved:
		t.Fatalf("route resolution did not wait for concurrent retarget: err=%v target=%q", result.err, result.route.Target.ID)
	case <-time.After(100 * time.Millisecond):
	}
	if err := operatorTx.Commit(); err != nil {
		t.Fatal(err)
	}
	var result routeResult
	select {
	case result = <-resolved:
	case <-time.After(5 * time.Second):
		t.Fatal("route resolution did not resume after concurrent retarget committed")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.route.Target.ID != replacement.TargetID {
		t.Fatal("route resolution returned stale target configuration after retarget")
	}
	if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, result.route.ID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_stale_cached_target", InferenceRequestID: requestID, TargetID: oldRoute.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err == nil {
		t.Fatal("provider-attempt guard accepted a stale cached target after retarget")
	}
	if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: "att_current_target", InferenceRequestID: requestID, TargetID: result.route.Target.ID, AttemptNumber: 1, StartedAt: started.Add(2 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: "att_current_target", CompletedAt: time.Now().UTC(), Status: "succeeded", ProviderHTTPStatus: http.StatusOK, Duration: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: time.Now().UTC(), Status: "succeeded", HTTPStatus: http.StatusOK, UsageFinality: "unknown"}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresDashboardObservationsFollowCurrentRouteBinding(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	base := databaseSpec("Observation Binding", "observation-binding")
	base.ModelAlias = "observation-chat"
	base.TargetName = "observation-target-old"
	base.ProviderModel = "provider/old"
	provisioned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	oldRoute, err := fixture.store.ResolveRoute(ctx, principal, base.ModelAlias)
	if err != nil {
		t.Fatal(err)
	}
	completeSuccess := func(requestID string, route platform.Route) {
		t.Helper()
		started := time.Now().UTC().Add(-time.Second)
		if err := fixture.store.CreateInferenceRequest(ctx, platform.RequestStart{ID: requestID, Principal: principal, ModelAlias: base.ModelAlias, StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		if err := fixture.store.SetInferenceRequestRoute(ctx, requestID, route.ID); err != nil {
			t.Fatal(err)
		}
		attemptID := "att_" + requestID
		if err := fixture.store.CreateProviderAttempt(ctx, platform.AttemptStart{ID: attemptID, InferenceRequestID: requestID, TargetID: route.Target.ID, AttemptNumber: 1, StartedAt: started.Add(time.Millisecond)}); err != nil {
			t.Fatal(err)
		}
		completed := time.Now().UTC()
		if err := fixture.store.CompleteProviderAttempt(ctx, platform.AttemptFinish{ID: attemptID, CompletedAt: completed, Status: "succeeded", ProviderHTTPStatus: http.StatusOK, Duration: time.Millisecond}); err != nil {
			t.Fatal(err)
		}
		input, output := int64(2), int64(1)
		if err := fixture.store.CompleteInferenceRequest(ctx, platform.RequestFinish{ID: requestID, CompletedAt: completed, Status: "succeeded", HTTPStatus: http.StatusOK, ExecutedModel: route.Target.ProviderModel, Duration: 2 * time.Millisecond, Usage: platform.TokenUsage{InputTokens: &input, OutputTokens: &output}, UsageFinality: "final"}); err != nil {
			t.Fatal(err)
		}
	}
	completeSuccess("req_observation_old", oldRoute)

	controlHandler, err := control.New(control.Config{Store: fixture.store})
	if err != nil {
		t.Fatal(err)
	}
	portal := func() control.ClientDashboardResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/dashboard?model="+base.ModelAlias, nil)
		request.SetBasicAuth("display-only", provisioned.APIKey)
		response := httptest.NewRecorder()
		controlHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("portal status=%d length=%d", response.Code, response.Body.Len())
		}
		var dashboard control.ClientDashboardResponse
		if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
			t.Fatal("portal response was not valid JSON")
		}
		return dashboard
	}
	machine := func() control.DashboardResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?model="+base.ModelAlias, nil)
		request.Header.Set("Authorization", "Bearer "+provisioned.APIKey)
		response := httptest.NewRecorder()
		controlHandler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("machine dashboard status=%d length=%d", response.Code, response.Body.Len())
		}
		var dashboard control.DashboardResponse
		if err := json.NewDecoder(response.Body).Decode(&dashboard); err != nil {
			t.Fatal("machine dashboard response was not valid JSON")
		}
		return dashboard
	}
	if dashboard := portal(); dashboard.Route.State != "operational" || dashboard.Route.LastSuccessAt == nil {
		t.Fatal("old binding did not expose its successful scoped observation")
	}

	replacement := base
	replacement.TargetName = "observation-target-new"
	replacement.TargetBaseURL = "https://observation-new.example.invalid/api/v1"
	replacement.ProviderModel = "provider/new"
	retargeted, err := fixture.store.Provision(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if retargeted.RouteID != oldRoute.ID || retargeted.TargetID == oldRoute.Target.ID {
		t.Fatal("retarget did not preserve the route while selecting a new target")
	}
	currentRoute, err := fixture.store.ResolveRoute(ctx, principal, base.ModelAlias)
	if err != nil {
		t.Fatal(err)
	}
	if currentRoute.BindingGeneration != oldRoute.BindingGeneration+1 {
		t.Fatalf("binding generation=%d want=%d", currentRoute.BindingGeneration, oldRoute.BindingGeneration+1)
	}
	if _, err := fixture.store.Provision(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	idempotentRoute, err := fixture.store.ResolveRoute(ctx, principal, base.ModelAlias)
	if err != nil || idempotentRoute.BindingGeneration != currentRoute.BindingGeneration {
		t.Fatal("idempotent reprovision advanced the PostgreSQL binding generation")
	}
	oldRecord, err := fixture.store.GetInferenceRequest(ctx, principal, "req_observation_old")
	if err != nil || oldRecord.BoundTargetID != oldRoute.Target.ID || oldRecord.BoundModelID != oldRoute.ModelID || oldRecord.RouteBindingGeneration != oldRoute.BindingGeneration {
		t.Fatal("old PostgreSQL request lost its exact route binding attribution")
	}
	unknownPortal := portal()
	if unknownPortal.Route.State != "unknown" || unknownPortal.Route.LastHealthCheckAt != nil || unknownPortal.Route.LastSuccessAt != nil {
		t.Fatal("portal reused a PostgreSQL observation from the previous route binding")
	}
	unknownMachine := machine()
	if len(unknownMachine.Routes) != 1 || unknownMachine.Routes[0].Status != "unknown" || unknownMachine.Routes[0].LastHealthCheckAt != nil || unknownMachine.Routes[0].LastSuccessAt != nil {
		t.Fatal("machine dashboard reused a PostgreSQL observation from the previous route binding")
	}

	completeSuccess("req_observation_new", currentRoute)
	newRecord, err := fixture.store.GetInferenceRequest(ctx, principal, "req_observation_new")
	if err != nil || newRecord.BoundTargetID != currentRoute.Target.ID || newRecord.RouteBindingGeneration != currentRoute.BindingGeneration {
		t.Fatal("new PostgreSQL request was not bound to the current generation")
	}
	if dashboard := portal(); dashboard.Route.State != "operational" || dashboard.Route.LastHealthCheckAt == nil || dashboard.Route.LastSuccessAt == nil {
		t.Fatal("portal did not expose the current PostgreSQL binding observation")
	}
	if dashboard := machine(); len(dashboard.Routes) != 1 || dashboard.Routes[0].Status != "operational" || dashboard.Routes[0].LastHealthCheckAt == nil || dashboard.Routes[0].LastSuccessAt == nil {
		t.Fatal("machine dashboard did not expose the current PostgreSQL binding observation")
	}
}

func TestPostgresRouteBindingGenerationIsDatabaseControlled(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	base := databaseSpec("Binding Generation", "binding-generation")
	base.ModelAlias = "binding-generation-v1"
	base.TargetName = "binding-generation-target-v1"
	provisioned, err := fixture.store.Provision(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	var originalGeneration int64
	if err := fixture.db.QueryRow(`SELECT binding_generation FROM tenant_routes WHERE id=$1`, provisioned.RouteID).Scan(&originalGeneration); err != nil {
		t.Fatal(err)
	}
	if originalGeneration != 1 {
		t.Fatalf("new route binding generation=%d", originalGeneration)
	}
	if _, err := fixture.db.Exec(`INSERT INTO models(id,alias,version) VALUES ('mdl_binding_generation_v2','binding-generation-v2','v2')`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE tenant_routes SET model_id='mdl_binding_generation_v2' WHERE id=$1`, provisioned.RouteID); err != nil {
		t.Fatal(err)
	}
	var modelGeneration int64
	if err := fixture.db.QueryRow(`SELECT binding_generation FROM tenant_routes WHERE id=$1`, provisioned.RouteID).Scan(&modelGeneration); err != nil {
		t.Fatal(err)
	}
	if modelGeneration != originalGeneration+1 {
		t.Fatalf("model retarget generation=%d want=%d", modelGeneration, originalGeneration+1)
	}
	if _, err := fixture.db.Exec(`UPDATE tenant_routes SET binding_generation=binding_generation+1 WHERE id=$1`, provisioned.RouteID); err == nil {
		t.Fatal("operator write changed the database-controlled binding generation")
	}
	if _, err := fixture.db.Exec(`UPDATE tenant_routes SET model_id=model_id,target_id=target_id WHERE id=$1`, provisioned.RouteID); err != nil {
		t.Fatal(err)
	}
	var unchangedGeneration int64
	if err := fixture.db.QueryRow(`SELECT binding_generation FROM tenant_routes WHERE id=$1`, provisioned.RouteID).Scan(&unchangedGeneration); err != nil {
		t.Fatal(err)
	}
	if unchangedGeneration != modelGeneration {
		t.Fatal("idempotent binding update advanced the generation")
	}

	replacement := base
	replacement.ModelAlias = "binding-generation-v2"
	replacement.ModelVersion = "v2"
	replacement.TargetName = "binding-generation-target-v2"
	replacement.TargetBaseURL = "https://binding-generation-v2.example.invalid/api/v1"
	replacement.ProviderModel = "provider/v2"
	retargeted, err := fixture.store.Provision(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if retargeted.RouteID != provisioned.RouteID || retargeted.TargetID == provisioned.TargetID {
		t.Fatal("target replacement did not retain the model-retargeted route")
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	currentRoute, err := fixture.store.ResolveRoute(ctx, principal, replacement.ModelAlias)
	if err != nil {
		t.Fatal(err)
	}
	if currentRoute.BindingGeneration != modelGeneration+1 {
		t.Fatalf("target retarget generation=%d want=%d", currentRoute.BindingGeneration, modelGeneration+1)
	}
	if _, err := fixture.store.Provision(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	idempotentRoute, err := fixture.store.ResolveRoute(ctx, principal, replacement.ModelAlias)
	if err != nil || idempotentRoute.BindingGeneration != currentRoute.BindingGeneration {
		t.Fatal("idempotent provisioning reset the binding boundary")
	}
}

func TestPostgresProvisionAndResolveUseRouteBeforeTargetLockOrder(t *testing.T) {
	fixture := newDatabaseFixture(t)
	ctx := context.Background()
	spec := databaseSpec("Lock Order", "lock-order")
	spec.ModelAlias = "lock-order-chat"
	spec.TargetName = "lock-order-target"
	provisioned, err := fixture.store.Provision(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(ctx, credentials.Digest(provisioned.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	originalRoute, err := fixture.store.ResolveRoute(ctx, principal, spec.ModelAlias)
	if err != nil {
		t.Fatal(err)
	}

	// Hold the target first. Provision must acquire the route and then wait here;
	// ResolveRoute must consequently wait on the route. Reversing Provision's
	// order forms the historical target->route / route->target deadlock cycle.
	targetLocker, err := fixture.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer targetLocker.Rollback()
	var lockedTarget string
	if err := targetLocker.QueryRow(`SELECT id FROM inference_targets WHERE id=$1 FOR UPDATE`, provisioned.TargetID).Scan(&lockedTarget); err != nil {
		t.Fatal(err)
	}

	type provisionResult struct {
		result platform.ProvisionResult
		err    error
	}
	provisionDone := make(chan provisionResult, 1)
	go func() {
		operationContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result, err := fixture.store.Provision(operationContext, spec)
		provisionDone <- provisionResult{result: result, err: err}
	}()
	select {
	case result := <-provisionDone:
		t.Fatalf("provision did not wait for the target lock: err=%v route=%q", result.err, result.result.RouteID)
	case <-time.After(150 * time.Millisecond):
	}

	type resolveResult struct {
		route platform.Route
		err   error
	}
	resolveDone := make(chan resolveResult, 1)
	go func() {
		operationContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		route, err := fixture.store.ResolveRoute(operationContext, principal, spec.ModelAlias)
		resolveDone <- resolveResult{route: route, err: err}
	}()
	select {
	case result := <-resolveDone:
		t.Fatalf("route resolution bypassed the provisioning route lock: err=%v target=%q", result.err, result.route.Target.ID)
	case <-time.After(150 * time.Millisecond):
	}
	if err := targetLocker.Commit(); err != nil {
		t.Fatal(err)
	}

	var provisionedAgain provisionResult
	select {
	case provisionedAgain = <-provisionDone:
	case <-time.After(5 * time.Second):
		t.Fatal("provisioning did not complete after the target lock was released")
	}
	if provisionedAgain.err != nil || provisionedAgain.result.RouteID != originalRoute.ID || provisionedAgain.result.TargetID != originalRoute.Target.ID {
		t.Fatalf("concurrent idempotent provision failed: err=%v route_match=%v target_match=%v", provisionedAgain.err, provisionedAgain.result.RouteID == originalRoute.ID, provisionedAgain.result.TargetID == originalRoute.Target.ID)
	}
	var resolvedAgain resolveResult
	select {
	case resolvedAgain = <-resolveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("route resolution did not complete after provisioning")
	}
	if resolvedAgain.err != nil || resolvedAgain.route.Target.ID != originalRoute.Target.ID || resolvedAgain.route.BindingGeneration != originalRoute.BindingGeneration {
		t.Fatalf("concurrent route resolution failed: err=%v target_match=%v generation_match=%v", resolvedAgain.err, resolvedAgain.route.Target.ID == originalRoute.Target.ID, resolvedAgain.route.BindingGeneration == originalRoute.BindingGeneration)
	}
}

func TestPostgresBackedHTTPVerticalSlice(t *testing.T) {
	fixture := newDatabaseFixture(t)
	var upstreamCalls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if upstreamCalls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "temporary provider detail", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"id":"provider-id","model":"provider/executed","choices":[{}],"usage":{"prompt_tokens":8,"completion_tokens":3}}`)
	}))
	defer upstream.Close()

	specA := databaseSpec("Tenant A", "tenant-a")
	specA.TargetBaseURL = upstream.URL + "/api/v1"
	provisionedA, err := fixture.store.Provision(context.Background(), specA)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := gateway.New(gateway.Config{Store: fixture.store, HTTPClient: upstream.Client(), SecretLookup: func(string) (string, bool) { return "provider-secret", true }, RetryBaseDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	prompt := "content that must not enter PostgreSQL metadata"
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"safe-chat","messages":[{"role":"user","content":"`+prompt+`"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+provisionedA.APIKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("gateway status=%d length=%d", response.Code, response.Body.Len())
	}
	if upstreamCalls.Load() != 2 {
		t.Fatalf("upstream calls=%d", upstreamCalls.Load())
	}
	var logicalRequests, providerAttempts int
	if err := fixture.db.QueryRow(`SELECT (SELECT count(*) FROM inference_requests),(SELECT count(*) FROM provider_attempts)`).Scan(&logicalRequests, &providerAttempts); err != nil {
		t.Fatal(err)
	}
	if logicalRequests != 1 || providerAttempts != 2 {
		t.Fatalf("logical=%d attempts=%d", logicalRequests, providerAttempts)
	}

	controlHandler, err := control.New(control.Config{Store: fixture.store})
	if err != nil {
		t.Fatal(err)
	}
	usageRequest := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	usageRequest.Header.Set("Authorization", "Bearer "+provisionedA.APIKey)
	usageResponse := httptest.NewRecorder()
	controlHandler.ServeHTTP(usageResponse, usageRequest)
	if usageResponse.Code != http.StatusOK {
		t.Fatalf("usage status=%d length=%d", usageResponse.Code, usageResponse.Body.Len())
	}
	usagePayload := usageResponse.Body.Bytes()
	var usage control.UsageResponse
	if err := json.Unmarshal(usagePayload, &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Summary.LogicalRequests != 1 || usage.Summary.SuccessfulRequests != 1 || usage.Summary.InputTokens.Value == nil || *usage.Summary.InputTokens.Value != 8 {
		t.Fatalf("usage=%#v", usage)
	}
	for label, forbidden := range map[string]string{"prompt content": prompt, "upstream URL": upstream.URL, "provider credential": "provider-secret"} {
		if strings.Contains(string(usagePayload), forbidden) {
			t.Fatalf("usage response leaked %s", label)
		}
	}

	specB := specA
	specB.OrganisationName, specB.OrganisationSlug = "Tenant B", "tenant-b"
	provisionedB, err := fixture.store.Provision(context.Background(), specB)
	if err != nil {
		t.Fatal(err)
	}
	tenantBRequest := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	tenantBRequest.Header.Set("Authorization", "Bearer "+provisionedB.APIKey)
	tenantBResponse := httptest.NewRecorder()
	controlHandler.ServeHTTP(tenantBResponse, tenantBRequest)
	var tenantBUsage control.UsageResponse
	if err := json.NewDecoder(tenantBResponse.Body).Decode(&tenantBUsage); err != nil {
		t.Fatal(err)
	}
	if tenantBUsage.Summary.LogicalRequests != 0 {
		t.Fatalf("tenant B saw tenant A usage: %#v", tenantBUsage)
	}
}

func TestPostgresBackedStreamingAgentAccountingAndIsolation(t *testing.T) {
	fixture := newDatabaseFixture(t)
	const prompt = "postgres streaming content canary"
	const output = "deterministic-stream-output-marker"
	streamBody := "data: {\"id\":\"provider-stream-id\",\"model\":\"provider/executed\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"" + output + "\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"id\":\"provider-stream-id\",\"model\":\"provider/executed\",\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":2}}\n\n" +
		"data: [DONE]\n\n"
	var upstreamCalls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		var request gateway.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode streaming gateway request: %v", err)
		}
		if request.Model != "provider/model" || request.Stream == nil || !*request.Stream || len(request.Tools) != 1 || r.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Error("streaming gateway bypassed configured model, tool, or credential forwarding")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, streamBody)
	}))
	defer upstream.Close()

	specA := databaseSpec("Streaming Tenant A", "streaming-a")
	specA.TargetBaseURL = upstream.URL + "/api/v1"
	a, err := fixture.store.Provision(context.Background(), specA)
	if err != nil {
		t.Fatal(err)
	}
	specB := specA
	specB.OrganisationName, specB.OrganisationSlug = "Streaming Tenant B", "streaming-b"
	b, err := fixture.store.Provision(context.Background(), specB)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := gateway.New(gateway.Config{Store: fixture.store, HTTPClient: upstream.Client(), SecretLookup: func(string) (string, bool) { return "provider-secret", true }, RetryBaseDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"model":"safe-chat","messages":[{"role":"user","content":"` + prompt + `"}],"stream":true,"max_tokens":32,"tools":[{"type":"function","function":{"name":"read_file","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]},"strict":false}}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+a.APIKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != streamBody || upstreamCalls.Load() != 1 {
		t.Fatalf("PostgreSQL stream status=%d calls=%d length=%d", response.Code, upstreamCalls.Load(), response.Body.Len())
	}
	var status, finality, errorClass string
	var logicalRequests, providerAttempts, inputTokens, outputTokens int
	if err := fixture.db.QueryRow(`
		SELECT ir.status, ir.usage_finality, COALESCE(ir.error_class,''), ir.input_tokens, ir.output_tokens,
		       (SELECT count(*) FROM inference_requests), (SELECT count(*) FROM provider_attempts)
		  FROM inference_requests ir
		 WHERE ir.id=$1`, response.Header().Get("X-Alzette-Request-ID")).Scan(&status, &finality, &errorClass, &inputTokens, &outputTokens, &logicalRequests, &providerAttempts); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || finality != "final" || errorClass != "" || inputTokens != 12 || outputTokens != 2 || logicalRequests != 1 || providerAttempts != 1 {
		t.Fatalf("PostgreSQL stream status=%s finality=%s logical=%d attempts=%d input=%d output=%d", status, finality, logicalRequests, providerAttempts, inputTokens, outputTokens)
	}
	principalB, err := fixture.store.Authenticate(context.Background(), credentials.Digest(b.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	pageB, err := fixture.store.ListInferenceRequests(context.Background(), principalB, platform.UsageFilter{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), Limit: 10})
	if err != nil || len(pageB.Requests) != 0 {
		t.Fatal("streaming logical request crossed the authenticated tenant boundary")
	}
	var persistedMetadata string
	if err := fixture.db.QueryRow(`SELECT row_to_json(ir)::text FROM inference_requests ir WHERE ir.id=$1`, response.Header().Get("X-Alzette-Request-ID")).Scan(&persistedMetadata); err != nil {
		t.Fatal(err)
	}
	for label, forbidden := range map[string]string{"prompt": prompt, "output": output, "provider credential": "provider-secret", "target URL": upstream.URL} {
		if strings.Contains(persistedMetadata, forbidden) {
			t.Fatalf("PostgreSQL streaming metadata leaked %s", label)
		}
	}
}

const httpStatusOK = 200

func migrationScript(t *testing.T, name string) string {
	t.Helper()
	script, err := migrations.Files.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(script)
}
