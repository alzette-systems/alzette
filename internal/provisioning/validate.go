package provisioning

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"alzette/internal/platform"
)

var (
	slugPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	aliasPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	secretRefPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)
	evidencePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,254}$`)
)

func Validate(spec platform.ProvisionSpec, allowInsecureTarget bool) (platform.ProvisionSpec, error) {
	spec.OrganisationName = strings.TrimSpace(spec.OrganisationName)
	spec.ProjectName = strings.TrimSpace(spec.ProjectName)
	spec.EnvironmentName = strings.TrimSpace(spec.EnvironmentName)
	spec.ServiceAccount = strings.TrimSpace(spec.ServiceAccount)
	spec.TargetName = strings.TrimSpace(spec.TargetName)
	spec.ProviderModel = strings.TrimSpace(spec.ProviderModel)
	spec.ModelVersion = strings.TrimSpace(spec.ModelVersion)

	for field, value := range map[string]string{
		"organisation slug": spec.OrganisationSlug,
		"project slug":      spec.ProjectSlug,
		"environment slug":  spec.EnvironmentSlug,
	} {
		if !slugPattern.MatchString(value) {
			return platform.ProvisionSpec{}, fmt.Errorf("%s is invalid: %w", field, platform.ErrInvalid)
		}
	}
	for field, value := range map[string]string{
		"organisation name": spec.OrganisationName,
		"project name":      spec.ProjectName,
		"environment name":  spec.EnvironmentName,
		"service account":   spec.ServiceAccount,
		"target name":       spec.TargetName,
		"provider model":    spec.ProviderModel,
		"model version":     spec.ModelVersion,
	} {
		if value == "" || len(value) > 255 {
			return platform.ProvisionSpec{}, fmt.Errorf("%s is invalid: %w", field, platform.ErrInvalid)
		}
	}
	if !aliasPattern.MatchString(spec.ModelAlias) {
		return platform.ProvisionSpec{}, fmt.Errorf("model alias is invalid: %w", platform.ErrInvalid)
	}
	if spec.ExecutionClass != "external_pilot" && spec.ExecutionClass != "private_compatible" {
		return platform.ProvisionSpec{}, fmt.Errorf("execution class is not supported by the PoC: %w", platform.ErrInvalid)
	}
	if spec.CapacityMode != "shared" && spec.CapacityMode != "dedicated" {
		return platform.ProvisionSpec{}, fmt.Errorf("capacity mode is invalid: %w", platform.ErrInvalid)
	}
	if spec.ExecutionClass == "external_pilot" && spec.CapacityMode != "shared" {
		return platform.ProvisionSpec{}, fmt.Errorf("external pilot targets must remain shared without separate capacity evidence: %w", platform.ErrInvalid)
	}
	if spec.CapacityMode == "dedicated" {
		if !evidencePattern.MatchString(spec.CapacityEvidenceRef) || strings.Contains(spec.CapacityEvidenceRef, "://") {
			return platform.ProvisionSpec{}, fmt.Errorf("dedicated capacity requires a safe operator evidence reference: %w", platform.ErrInvalid)
		}
	} else if spec.CapacityEvidenceRef != "" {
		return platform.ProvisionSpec{}, fmt.Errorf("shared capacity must not carry a dedicated evidence reference: %w", platform.ErrInvalid)
	}
	if !secretRefPattern.MatchString(spec.SecretRef) {
		return platform.ProvisionSpec{}, fmt.Errorf("secret reference is invalid: %w", platform.ErrInvalid)
	}
	targetURL, err := url.Parse(spec.TargetBaseURL)
	if err != nil || targetURL.Host == "" || targetURL.User != nil || targetURL.RawQuery != "" || targetURL.Fragment != "" || strings.Contains(targetURL.EscapedPath(), "%") {
		return platform.ProvisionSpec{}, fmt.Errorf("target base URL is invalid: %w", platform.ErrInvalid)
	}
	if targetURL.Scheme != "https" && !(allowInsecureTarget && targetURL.Scheme == "http") {
		return platform.ProvisionSpec{}, fmt.Errorf("target base URL must use HTTPS: %w", platform.ErrInvalid)
	}
	spec.TargetBaseURL = strings.TrimRight(targetURL.String(), "/")
	if spec.TargetTimeout < 100*time.Millisecond || spec.TargetTimeout > time.Minute {
		return platform.ProvisionSpec{}, fmt.Errorf("target timeout is outside the supported range: %w", platform.ErrInvalid)
	}
	if spec.MaxAttempts < 1 || spec.MaxAttempts > 4 {
		return platform.ProvisionSpec{}, fmt.Errorf("max attempts is outside the supported range: %w", platform.ErrInvalid)
	}
	if spec.ProbeInterval == 0 {
		spec.ProbeInterval = 5 * time.Minute
	}
	if spec.ProbeInterval < 30*time.Second || spec.ProbeInterval > 24*time.Hour {
		return platform.ProvisionSpec{}, fmt.Errorf("probe interval is outside the supported range: %w", platform.ErrInvalid)
	}
	if err := validateServicePlan(spec); err != nil {
		return platform.ProvisionSpec{}, err
	}

	scopes, err := ValidateScopes(spec.Scopes)
	if err != nil {
		return platform.ProvisionSpec{}, err
	}
	spec.Scopes = scopes
	return spec, nil
}

func validateServicePlan(spec platform.ProvisionSpec) error {
	if spec.ServicePlanCode == "" {
		if spec.ServicePlanName != "" || spec.SharedRequestAllowance != nil || spec.SharedTokenAllowance != nil || spec.DedicatedResourceClass != "" || spec.DedicatedAcceleratorCount != nil || spec.ServicePlanSource != "" || spec.ServicePlanFinality != "" {
			return fmt.Errorf("service plan fields require a service plan code: %w", platform.ErrInvalid)
		}
		return nil
	}
	if !slugPattern.MatchString(spec.ServicePlanCode) || strings.TrimSpace(spec.ServicePlanName) == "" || len(spec.ServicePlanName) > 255 {
		return fmt.Errorf("service plan identity is invalid: %w", platform.ErrInvalid)
	}
	if !evidencePattern.MatchString(spec.ServicePlanSource) || strings.Contains(spec.ServicePlanSource, "://") {
		return fmt.Errorf("service plan source is invalid: %w", platform.ErrInvalid)
	}
	if spec.ServicePlanFinality != "declared" && spec.ServicePlanFinality != "unknown" {
		return fmt.Errorf("service plan finality is invalid: %w", platform.ErrInvalid)
	}
	validPeriod := func(value string) bool {
		return value == "hour" || value == "day" || value == "month" || value == "contract_term"
	}
	if spec.CapacityMode == "shared" {
		if spec.DedicatedResourceClass != "" || spec.DedicatedAcceleratorCount != nil {
			return fmt.Errorf("shared plan cannot declare dedicated allocation: %w", platform.ErrInvalid)
		}
		if spec.SharedRequestAllowance != nil {
			if *spec.SharedRequestAllowance < 0 || !validPeriod(spec.SharedRequestAllowancePeriod) {
				return fmt.Errorf("shared request allowance needs a non-negative value and period: %w", platform.ErrInvalid)
			}
		} else if spec.SharedRequestAllowancePeriod != "" {
			return fmt.Errorf("request allowance period has no value: %w", platform.ErrInvalid)
		}
		if spec.SharedTokenAllowance != nil {
			if *spec.SharedTokenAllowance < 0 || !validPeriod(spec.SharedTokenAllowancePeriod) {
				return fmt.Errorf("shared token allowance needs a non-negative value and period: %w", platform.ErrInvalid)
			}
		} else if spec.SharedTokenAllowancePeriod != "" {
			return fmt.Errorf("token allowance period has no value: %w", platform.ErrInvalid)
		}
		return nil
	}
	if spec.SharedRequestAllowance != nil || spec.SharedRequestAllowancePeriod != "" || spec.SharedTokenAllowance != nil || spec.SharedTokenAllowancePeriod != "" {
		return fmt.Errorf("dedicated plan cannot declare shared allowances: %w", platform.ErrInvalid)
	}
	if spec.DedicatedResourceClass != "" && !evidencePattern.MatchString(spec.DedicatedResourceClass) {
		return fmt.Errorf("dedicated resource class is invalid: %w", platform.ErrInvalid)
	}
	if spec.DedicatedAcceleratorCount != nil && *spec.DedicatedAcceleratorCount <= 0 {
		return fmt.Errorf("dedicated accelerator count must be positive: %w", platform.ErrInvalid)
	}
	return nil
}

func ValidateScopes(input []string) ([]string, error) {
	seen := make(map[string]bool)
	scopes := make([]string, 0, len(input))
	for _, scope := range input {
		switch scope {
		case platform.ScopeInferenceWrite, platform.ScopeUsageRead, platform.ScopeRoutesRead:
		default:
			return nil, fmt.Errorf("unsupported scope %q: %w", scope, platform.ErrInvalid)
		}
		if !seen[scope] {
			seen[scope] = true
			scopes = append(scopes, scope)
		}
	}
	if len(scopes) == 0 {
		return nil, fmt.Errorf("at least one scope is required: %w", platform.ErrInvalid)
	}
	sort.Strings(scopes)
	return scopes, nil
}

func ValidateRotate(spec platform.RotateKeySpec) (platform.RotateKeySpec, error) {
	if !slugPattern.MatchString(spec.OrganisationSlug) || !slugPattern.MatchString(spec.ProjectSlug) || !slugPattern.MatchString(spec.EnvironmentSlug) {
		return platform.RotateKeySpec{}, fmt.Errorf("scope slug is invalid: %w", platform.ErrInvalid)
	}
	if strings.TrimSpace(spec.ServiceAccount) == "" {
		return platform.RotateKeySpec{}, fmt.Errorf("service account is required: %w", platform.ErrInvalid)
	}
	validated, err := Validate(platform.ProvisionSpec{
		OrganisationName: "validation",
		OrganisationSlug: spec.OrganisationSlug,
		ProjectName:      "validation",
		ProjectSlug:      spec.ProjectSlug,
		EnvironmentName:  "validation",
		EnvironmentSlug:  spec.EnvironmentSlug,
		ModelAlias:       "validation",
		ModelVersion:     "validation",
		TargetName:       "validation",
		ExecutionClass:   "external_pilot",
		CapacityMode:     "shared",
		TargetBaseURL:    "https://validation.invalid/v1",
		ProviderModel:    "validation",
		SecretRef:        "VALIDATION_SECRET",
		TargetTimeout:    time.Second,
		MaxAttempts:      1,
		ServiceAccount:   spec.ServiceAccount,
		Scopes:           spec.Scopes,
	}, false)
	if err != nil {
		return platform.RotateKeySpec{}, err
	}
	spec.ServiceAccount = validated.ServiceAccount
	spec.Scopes = validated.Scopes
	return spec, nil
}
