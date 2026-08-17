package slice0smoke

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alzette/internal/faketarget"
	"alzette/internal/gateway"
	"alzette/internal/store/memory"
)

const fakeProviderSecret = "slice0-deterministic-provider-token"

func TestRunProvesCredentialNeutralSlice0Vertical(t *testing.T) {
	target, err := faketarget.New(faketarget.Config{
		Secret:        fakeProviderSecret,
		ProviderModel: faketarget.DefaultProviderModel,
		TimeoutFirst:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	targetServer := httptest.NewServer(target)
	t.Cleanup(targetServer.Close)

	store := memory.New()
	gatewayHandler, err := gateway.New(gateway.Config{
		Store:                store,
		AllowInsecureTargets: true,
		SecretLookup: func(reference string) (string, bool) {
			return fakeProviderSecret, reference == "SLICE0_FAKE_TARGET_KEY"
		},
		RetryBaseDelay: time.Millisecond,
		MaxRetryDelay:  time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewServer(gatewayHandler)
	t.Cleanup(gatewayServer.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := Run(ctx, Config{
		Store:          store,
		Provisioner:    store,
		GatewayURL:     gatewayServer.URL,
		TargetBaseURL:  targetServer.URL + "/v1",
		ProviderModel:  faketarget.DefaultProviderModel,
		SecretRef:      "SLICE0_FAKE_TARGET_KEY",
		ExpectedOutput: faketarget.ExpectedOutput,
		TargetTimeout:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.TargetKind != "deterministic_compatible_target" || result.CapacityMode != "shared" {
		t.Fatal("Slice 0 smoke returned an unexpected safe summary")
	}
	if result.LogicalRequests != 4 || result.ProviderAttempts != 3 || result.TenantA.LogicalRequests != 2 || result.TenantB.LogicalRequests != 2 {
		t.Fatalf("logical=%d attempts=%d tenant_a=%d tenant_b=%d", result.LogicalRequests, result.ProviderAttempts, result.TenantA.LogicalRequests, result.TenantB.LogicalRequests)
	}
	if result.TenantA.SuccessfulAttempts != 2 || result.TenantB.SuccessfulAttempts != 1 || !result.CrossRouteRejected || !result.MetadataRedacted {
		t.Fatal("Slice 0 retry, isolation, or redaction proof failed")
	}

	safeSummary, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for label, forbidden := range map[string]string{
		"provider credential": fakeProviderSecret,
		"provider output":     faketarget.ExpectedOutput,
		"target URL":          targetServer.URL,
	} {
		if strings.Contains(string(safeSummary), forbidden) {
			t.Fatalf("safe smoke summary exposed %s", label)
		}
	}
}

func TestValidateOfflineEndpointsFailsClosed(t *testing.T) {
	tests := []struct {
		name, gatewayURL, targetURL string
	}{
		{"external target", "http://gateway:8080", "https://openrouter.ai/api/v1"},
		{"external gateway", "http://example.com", "http://fake-target:8090/v1"},
		{"credentials in URL", "http://user:password@localhost:8080", "http://localhost:8090/v1"},
		{"target query", "http://localhost:8080", "http://localhost:8090/v1?override=true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateOfflineEndpoints(test.gatewayURL, test.targetURL); err == nil {
				t.Fatal("unsafe offline smoke endpoint was accepted")
			}
		})
	}
	if err := ValidateOfflineEndpoints("http://gateway:8080", "http://fake-target:8090/v1"); err != nil {
		t.Fatalf("fixed Compose endpoints rejected: %v", err)
	}
}
