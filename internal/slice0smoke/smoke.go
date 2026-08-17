// Package slice0smoke runs the credential-neutral Slice 0 operator acceptance
// path against an explicitly local deterministic compatible target.
package slice0smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/credentials"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

const maximumResponseBytes = int64(1 << 20)

type Config struct {
	Store          platform.Store
	Provisioner    platform.Provisioner
	HTTPClient     *http.Client
	GatewayURL     string
	TargetBaseURL  string
	ProviderModel  string
	SecretRef      string
	ExpectedOutput string
	TargetTimeout  time.Duration
}

type Result struct {
	Status             string       `json:"status"`
	TargetKind         string       `json:"target_kind"`
	CapacityMode       string       `json:"capacity_mode"`
	LogicalRequests    int          `json:"logical_requests"`
	ProviderAttempts   int          `json:"provider_attempts"`
	TenantA            TenantResult `json:"tenant_a"`
	TenantB            TenantResult `json:"tenant_b"`
	CrossRouteRejected bool         `json:"cross_route_rejected"`
	MetadataRedacted   bool         `json:"metadata_redacted"`
}

type TenantResult struct {
	SuccessfulRequestID string `json:"successful_request_id"`
	SuccessfulAttempts  int    `json:"successful_attempts"`
	LogicalRequests     int    `json:"logical_requests"`
}

func Validate(config Config) error {
	if err := ValidateOfflineEndpoints(config.GatewayURL, config.TargetBaseURL); err != nil {
		return err
	}
	if config.Store == nil || config.Provisioner == nil {
		return errors.New("Slice 0 smoke store and provisioner are required")
	}
	if config.ProviderModel == "" || config.SecretRef == "" || config.ExpectedOutput == "" {
		return errors.New("Slice 0 smoke target contract is incomplete")
	}
	if config.TargetTimeout < 100*time.Millisecond || config.TargetTimeout > 5*time.Second {
		return errors.New("Slice 0 smoke target timeout is outside the offline bound")
	}
	return nil
}

// ValidateOfflineEndpoints is intentionally available to the operator command
// so an unsafe URL is rejected before a database connection or HTTP client is
// created. The credential-neutral smoke can address only loopback hosts or the
// fixed Compose service names.
func ValidateOfflineEndpoints(gatewayURL, targetBaseURL string) error {
	if err := validateOfflineURL(gatewayURL, "gateway"); err != nil {
		return fmt.Errorf("gateway URL: %w", err)
	}
	if err := validateOfflineURL(targetBaseURL, "fake-target"); err != nil {
		return fmt.Errorf("target base URL: %w", err)
	}
	return nil
}

func Run(ctx context.Context, config Config) (Result, error) {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("redirects are disabled")
			},
		}
	}
	if err := Validate(config); err != nil {
		return Result{}, err
	}

	runID, err := ids.New("smoke")
	if err != nil {
		return Result{}, errors.New("create Slice 0 smoke identifier")
	}
	suffix := strings.TrimPrefix(runID, "smoke_")
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	targetName := "slice0-fake-" + suffix
	aliasA := "slice0-a-chat-" + suffix
	aliasB := "slice0-b-chat-" + suffix

	base := platform.ProvisionSpec{
		ProjectName: "Slice 0 Application", ProjectSlug: "application",
		EnvironmentName: "Operator Smoke", EnvironmentSlug: "smoke",
		ModelVersion: "slice0-v1", TargetName: targetName,
		ExecutionClass: "private_compatible", CapacityMode: "shared",
		TargetBaseURL: config.TargetBaseURL, ProviderModel: config.ProviderModel,
		SecretRef: config.SecretRef, TargetTimeout: config.TargetTimeout, MaxAttempts: 2,
		ServiceAccount: "slice0-client", Scopes: []string{platform.ScopeInferenceWrite},
	}
	specA := base
	specA.OrganisationName, specA.OrganisationSlug = "Slice 0 Tenant A", "slice0-a-"+suffix
	specA.ModelAlias = aliasA
	specB := base
	specB.OrganisationName, specB.OrganisationSlug = "Slice 0 Tenant B", "slice0-b-"+suffix
	specB.ModelAlias = aliasB

	provisionedA, err := config.Provisioner.Provision(ctx, specA)
	if err != nil {
		return Result{}, errors.New("provision Slice 0 tenant A")
	}
	provisionedB, err := config.Provisioner.Provision(ctx, specB)
	if err != nil {
		return Result{}, errors.New("provision Slice 0 tenant B")
	}
	if !provisionedA.KeyCreated || !provisionedB.KeyCreated || provisionedA.APIKey == "" || provisionedB.APIKey == "" {
		return Result{}, errors.New("Slice 0 smoke requires fresh one-time tenant keys")
	}
	if provisionedA.TargetID != provisionedB.TargetID || provisionedA.RouteID == provisionedB.RouteID {
		return Result{}, errors.New("Slice 0 shared target bindings were not explicit and distinct")
	}

	promptA := "slice0-prompt-a-" + suffix
	promptB := "slice0-prompt-b-" + suffix
	successA, err := invoke(ctx, config, provisionedA.APIKey, aliasA, promptA, http.StatusOK)
	if err != nil {
		return Result{}, errors.New("tenant A compatible inference failed")
	}
	successB, err := invoke(ctx, config, provisionedB.APIKey, aliasB, promptB, http.StatusOK)
	if err != nil {
		return Result{}, errors.New("tenant B compatible inference failed")
	}
	deniedA, err := invoke(ctx, config, provisionedA.APIKey, aliasB, "slice0-cross-a-"+suffix, http.StatusNotFound)
	if err != nil || deniedA.errorCode != "model_not_authorised" {
		return Result{}, errors.New("tenant A cross-route call did not fail closed")
	}
	deniedB, err := invoke(ctx, config, provisionedB.APIKey, aliasA, "slice0-cross-b-"+suffix, http.StatusNotFound)
	if err != nil || deniedB.errorCode != "model_not_authorised" {
		return Result{}, errors.New("tenant B cross-route call did not fail closed")
	}

	principalA, err := config.Store.Authenticate(ctx, credentials.Digest(provisionedA.APIKey))
	if err != nil {
		return Result{}, errors.New("authenticate Slice 0 tenant A ledger scope")
	}
	principalB, err := config.Store.Authenticate(ctx, credentials.Digest(provisionedB.APIKey))
	if err != nil {
		return Result{}, errors.New("authenticate Slice 0 tenant B ledger scope")
	}
	now := time.Now().UTC()
	window := platform.UsageFilter{From: now.Add(-time.Hour), To: now.Add(time.Hour), Limit: 20}
	pageA, err := config.Store.ListInferenceRequests(ctx, principalA, window)
	if err != nil {
		return Result{}, errors.New("read Slice 0 tenant A ledger")
	}
	pageB, err := config.Store.ListInferenceRequests(ctx, principalB, window)
	if err != nil {
		return Result{}, errors.New("read Slice 0 tenant B ledger")
	}
	if pageA.Truncated || pageB.Truncated || len(pageA.Requests) != 2 || len(pageB.Requests) != 2 {
		return Result{}, errors.New("Slice 0 logical request totals did not reconcile")
	}
	recordA, err := config.Store.GetInferenceRequest(ctx, principalA, successA.requestID)
	if err != nil {
		return Result{}, errors.New("read Slice 0 tenant A successful request")
	}
	recordB, err := config.Store.GetInferenceRequest(ctx, principalB, successB.requestID)
	if err != nil {
		return Result{}, errors.New("read Slice 0 tenant B successful request")
	}
	if recordA.Status != "succeeded" || recordA.AttemptCount != 2 || recordA.UsageFinality != "final" {
		return Result{}, errors.New("Slice 0 timeout/retry accounting did not reconcile")
	}
	if recordB.Status != "succeeded" || recordB.AttemptCount != 1 || recordB.UsageFinality != "final" {
		return Result{}, errors.New("Slice 0 tenant B accounting did not reconcile")
	}
	if _, err := config.Store.GetInferenceRequest(ctx, principalB, successA.requestID); !errors.Is(err, platform.ErrNotFound) {
		return Result{}, errors.New("Slice 0 tenant B could read tenant A request")
	}
	if _, err := config.Store.GetInferenceRequest(ctx, principalA, successB.requestID); !errors.Is(err, platform.ErrNotFound) {
		return Result{}, errors.New("Slice 0 tenant A could read tenant B request")
	}
	routeA, err := config.Store.ResolveRoute(ctx, principalA, aliasA)
	if err != nil {
		return Result{}, errors.New("resolve Slice 0 tenant A route after inference")
	}
	routeB, err := config.Store.ResolveRoute(ctx, principalB, aliasB)
	if err != nil {
		return Result{}, errors.New("resolve Slice 0 tenant B route after inference")
	}
	if routeA.Target.ID != routeB.Target.ID || routeA.Target.CapacityMode != "shared" || routeA.Target.ExecutionClass != "private_compatible" || routeA.Target.HealthStatus != "operational" {
		return Result{}, errors.New("Slice 0 target binding or inference health was not truthful")
	}

	metadata, err := json.Marshal([]platform.RequestPage{pageA, pageB})
	if err != nil {
		return Result{}, errors.New("encode Slice 0 metadata proof")
	}
	for _, forbidden := range []string{promptA, promptB, config.ExpectedOutput, provisionedA.APIKey, provisionedB.APIKey, config.TargetBaseURL} {
		if bytes.Contains(metadata, []byte(forbidden)) {
			return Result{}, errors.New("Slice 0 metadata contained content, credentials, or a raw target URL")
		}
	}

	return Result{
		Status:             "ok",
		TargetKind:         "deterministic_compatible_target",
		CapacityMode:       "shared",
		LogicalRequests:    len(pageA.Requests) + len(pageB.Requests),
		ProviderAttempts:   recordA.AttemptCount + recordB.AttemptCount,
		TenantA:            TenantResult{SuccessfulRequestID: successA.requestID, SuccessfulAttempts: recordA.AttemptCount, LogicalRequests: len(pageA.Requests)},
		TenantB:            TenantResult{SuccessfulRequestID: successB.requestID, SuccessfulAttempts: recordB.AttemptCount, LogicalRequests: len(pageB.Requests)},
		CrossRouteRejected: true,
		MetadataRedacted:   true,
	}, nil
}

type invocation struct {
	requestID string
	errorCode string
}

func invoke(ctx context.Context, config Config, key, alias, prompt string, expectedStatus int) (invocation, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model": alias,
		"messages": []map[string]string{{
			"role": "user", "content": prompt,
		}},
		"stream": false,
	})
	if err != nil {
		return invocation{}, errors.New("encode compatible request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(config.GatewayURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return invocation{}, errors.New("create compatible request")
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	response, err := config.HTTPClient.Do(request)
	if err != nil {
		return invocation{}, errors.New("call local Slice 0 gateway")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maximumResponseBytes+1))
	if err != nil || int64(len(responseBody)) > maximumResponseBytes {
		return invocation{}, errors.New("read bounded Slice 0 gateway response")
	}
	requestID := response.Header.Get("X-Alzette-Request-ID")
	if requestID == "" || response.Header.Get("X-Request-ID") != requestID {
		return invocation{}, errors.New("Slice 0 gateway request correlation was inconsistent")
	}
	if response.StatusCode != expectedStatus {
		return invocation{}, fmt.Errorf("Slice 0 gateway returned status %d with length %d", response.StatusCode, len(responseBody))
	}
	if expectedStatus == http.StatusOK {
		var completion struct {
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				Input  *int64 `json:"prompt_tokens"`
				Output *int64 `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(responseBody, &completion); err != nil || len(completion.Choices) != 1 {
			return invocation{}, errors.New("Slice 0 gateway response was not compatible JSON")
		}
		if completion.Model != config.ProviderModel || completion.Choices[0].Message.Content != config.ExpectedOutput || completion.Usage.Input == nil || completion.Usage.Output == nil {
			return invocation{}, errors.New("Slice 0 compatible response fields did not match the approved adapter contract")
		}
		return invocation{requestID: requestID}, nil
	}
	var envelope api.ErrorEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil || envelope.RequestID != requestID || envelope.Error.Code == "" {
		return invocation{}, errors.New("Slice 0 gateway error envelope was inconsistent")
	}
	return invocation{requestID: requestID, errorCode: envelope.Error.Code}, nil
}

func validateOfflineURL(rawURL, serviceName string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("offline smoke requires a plain-HTTP local service URL")
	}
	hostname := parsed.Hostname()
	if hostname == serviceName || hostname == "localhost" {
		return nil
	}
	if address := net.ParseIP(hostname); address != nil && address.IsLoopback() {
		return nil
	}
	return errors.New("offline smoke refuses non-local service hosts")
}
