package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"alzette/internal/credentials"
	"alzette/internal/platform"
)

func memorySpec() platform.ProvisionSpec {
	return platform.ProvisionSpec{OrganisationName: "Tenant", OrganisationSlug: "tenant", ProjectName: "App", ProjectSlug: "app", EnvironmentName: "Production", EnvironmentSlug: "production", ModelAlias: "chat", ModelVersion: "v1", TargetName: "target", ExecutionClass: "external_pilot", CapacityMode: "shared", TargetBaseURL: "http://127.0.0.1:9999/v1", ProviderModel: "provider/model", SecretRef: "TARGET_KEY", TargetTimeout: time.Second, MaxAttempts: 2, ServiceAccount: "app", Scopes: []string{platform.ScopeInferenceWrite, platform.ScopeUsageRead}}
}

func TestProvisionIsIdempotentAndKeyPlaintextIsReturnedOnce(t *testing.T) {
	store := New()
	first, err := store.Provision(context.Background(), memorySpec())
	if err != nil {
		t.Fatal(err)
	}
	if !first.KeyCreated || first.APIKey == "" {
		t.Fatal("first provision did not reveal a key")
	}
	second, err := store.Provision(context.Background(), memorySpec())
	if err != nil {
		t.Fatal(err)
	}
	if second.KeyCreated || second.APIKey != "" || second.KeyPrefix != first.KeyPrefix || second.RouteID != first.RouteID {
		t.Fatalf("second provision=%#v", second)
	}
	if _, err := store.Authenticate(context.Background(), credentials.Digest(first.APIKey)); err != nil {
		t.Fatal(err)
	}
}

func TestRotateAndRevokeInvalidateOldKeys(t *testing.T) {
	store := New()
	provisioned, err := store.Provision(context.Background(), memorySpec())
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := store.RotateKey(context.Background(), platform.RotateKeySpec{OrganisationSlug: "tenant", ProjectSlug: "app", EnvironmentSlug: "production", ServiceAccount: "app", Scopes: []string{platform.ScopeUsageRead}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), credentials.Digest(provisioned.APIKey)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("old key error=%v", err)
	}
	principal, err := store.Authenticate(context.Background(), credentials.Digest(rotated.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	if principal.HasScope(platform.ScopeInferenceWrite) || !principal.HasScope(platform.ScopeUsageRead) {
		t.Fatalf("scopes=%v", principal.Scopes)
	}
	if err := store.RevokeKey(context.Background(), rotated.KeyPrefix); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), credentials.Digest(rotated.APIKey)); !errors.Is(err, platform.ErrUnauthenticated) {
		t.Fatalf("revoked key error=%v", err)
	}
}

func TestSharedTargetsRequireAnExplicitTenantRoute(t *testing.T) {
	store := New()
	a := memorySpec()
	resultA, err := store.Provision(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.OrganisationName = "Other"
	b.OrganisationSlug = "other"
	b.ModelAlias = "other-chat"
	b.ServiceAccount = "other-app"
	resultB, err := store.Provision(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	principalA, err := store.Authenticate(context.Background(), credentials.Digest(resultA.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	principalB, err := store.Authenticate(context.Background(), credentials.Digest(resultB.APIKey))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveRoute(context.Background(), principalA, "other-chat"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("tenant A resolved tenant B route: %v", err)
	}
	if _, err := store.ResolveRoute(context.Background(), principalB, "chat"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("tenant B resolved tenant A route: %v", err)
	}
}
