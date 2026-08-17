package provisioning

import (
	"testing"
	"time"

	"alzette/internal/platform"
)

func validSpec() platform.ProvisionSpec {
	return platform.ProvisionSpec{
		OrganisationName: "Tenant A", OrganisationSlug: "tenant-a",
		ProjectName: "Project", ProjectSlug: "project",
		EnvironmentName: "Production", EnvironmentSlug: "production",
		ModelAlias: "chat", ModelVersion: "v1",
		TargetName: "openrouter", ExecutionClass: "external_pilot", CapacityMode: "shared",
		TargetBaseURL: "https://openrouter.ai/api/v1/", ProviderModel: "provider/model",
		SecretRef: "OPENROUTER_API_KEY", TargetTimeout: time.Second, MaxAttempts: 2,
		ServiceAccount: "application", Scopes: []string{platform.ScopeUsageRead, platform.ScopeInferenceWrite, platform.ScopeUsageRead},
	}
}

func TestValidateNormalisesSafeConfiguration(t *testing.T) {
	got, err := Validate(validSpec(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetBaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base URL = %q", got.TargetBaseURL)
	}
	if len(got.Scopes) != 2 {
		t.Fatalf("scopes = %#v", got.Scopes)
	}
}

func TestValidateRejectsCustomerControlledOrUnsafeTargetValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*platform.ProvisionSpec)
	}{
		{"plain HTTP", func(s *platform.ProvisionSpec) { s.TargetBaseURL = "http://openrouter.example/v1" }},
		{"credentials in URL", func(s *platform.ProvisionSpec) { s.TargetBaseURL = "https://secret@example.test/v1" }},
		{"URL query", func(s *platform.ProvisionSpec) { s.TargetBaseURL = "https://example.test/v1?key=secret" }},
		{"ambiguous escaped path", func(s *platform.ProvisionSpec) { s.TargetBaseURL = "https://example.test/api%2fv1" }},
		{"secret value instead of reference", func(s *platform.ProvisionSpec) { s.SecretRef = "sk-secret" }},
		{"unsupported execution claim", func(s *platform.ProvisionSpec) { s.ExecutionClass = "meluxina" }},
		{"unsupported external dedicated claim", func(s *platform.ProvisionSpec) { s.CapacityMode = "dedicated" }},
		{"dedicated without evidence", func(s *platform.ProvisionSpec) { s.ExecutionClass = "private_compatible"; s.CapacityMode = "dedicated" }},
		{"unknown scope", func(s *platform.ProvisionSpec) { s.Scopes = []string{"admin"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := validSpec()
			tc.edit(&spec)
			if _, err := Validate(spec, false); err == nil {
				t.Fatal("validation unexpectedly succeeded")
			}
		})
	}
}
