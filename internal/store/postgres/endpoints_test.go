package postgres

import (
	"crypto/sha256"
	"testing"

	"alzette/internal/endpoints"
)

func TestDeploymentRequestDigestPreservesLegacyNilTeamSize(t *testing.T) {
	contextTokens := int64(32768)
	concurrency := 4
	rpm := 120
	latency := "balanced"
	monthly := int64(50000)
	workload := endpoints.Workload{
		UseCase:                   "legacy sizing",
		ExpectedContextTokens:     &contextTokens,
		ExpectedConcurrency:       &concurrency,
		ExpectedRequestsPerMinute: &rpm,
		LatencyPriority:           &latency,
		ExpectedMonthlyRequests:   &monthly,
	}
	want := sha256.Sum256([]byte(`{"kind":"capacity_change","target_id":"end_legacy","units":2,"workload":{"use_case":"legacy sizing","expected_context_tokens":32768,"expected_concurrency":4,"expected_requests_per_minute":120,"latency_priority":"balanced","expected_monthly_requests":50000}}`))
	if got := deploymentRequestDigest("capacity_change", "end_legacy", 2, workload); got != want {
		t.Fatal("nil expected_user_count changed the legacy capacity request digest")
	}
}

func TestDeploymentRequestDigestAndComparisonIncludeTeamSize(t *testing.T) {
	one, two := 25, 26
	first := endpoints.Workload{ExpectedUserCount: &one}
	second := endpoints.Workload{ExpectedUserCount: &two}
	if workloadEqual(first, second) {
		t.Fatal("configuration idempotency comparison ignored expected_user_count")
	}
	if deploymentRequestDigest("new_endpoint", "cfg_team", 1, first) == deploymentRequestDigest("new_endpoint", "cfg_team", 1, second) {
		t.Fatal("deployment request digest ignored expected_user_count")
	}
}

func TestMergeRevisedWorkloadPatchPreservesLegacyFields(t *testing.T) {
	contextTokens := int64(65536)
	concurrency := 8
	rpm := 240
	latency := "throughput"
	monthly := int64(90000)
	users := 40
	current := endpoints.Workload{
		UseCase:                   "existing intent",
		ExpectedContextTokens:     &contextTokens,
		ExpectedConcurrency:       &concurrency,
		ExpectedRequestsPerMinute: &rpm,
		LatencyPriority:           &latency,
		ExpectedMonthlyRequests:   &monthly,
	}
	got := mergeRevisedWorkloadPatch(current, endpoints.Workload{ExpectedUserCount: &users})
	want := current
	want.ExpectedUserCount = &users
	if !workloadEqual(got, want) {
		t.Fatal("team-size-only patch cleared a legacy workload field")
	}
}
