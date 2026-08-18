package endpoints

import (
	"encoding/json"
	"errors"
	"testing"

	"alzette/internal/platform"
)

func TestValidateWorkloadExpectedUserCountBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		value *int
		valid bool
	}{
		{name: "omitted", valid: true},
		{name: "minimum", value: intPointer(1), valid: true},
		{name: "maximum", value: intPointer(10000), valid: true},
		{name: "zero", value: intPointer(0)},
		{name: "negative", value: intPointer(-1)},
		{name: "over maximum", value: intPointer(10001)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateWorkload(Workload{ExpectedUserCount: test.value})
			if test.valid && err != nil {
				t.Fatalf("valid team size rejected: %v", err)
			}
			if !test.valid && !errors.Is(err, platform.ErrInvalid) {
				t.Fatalf("invalid team size error=%v", err)
			}
		})
	}
}

func TestWorkloadExpectedUserCountJSONContract(t *testing.T) {
	var omitted Workload
	if err := json.Unmarshal([]byte(`{"expected_concurrency":7}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.ExpectedUserCount != nil {
		t.Fatal("omitted expected_user_count became populated")
	}
	var present Workload
	if err := json.Unmarshal([]byte(`{"expected_user_count":25}`), &present); err != nil {
		t.Fatal(err)
	}
	if present.ExpectedUserCount == nil || *present.ExpectedUserCount != 25 {
		t.Fatalf("expected_user_count round trip=%v", present.ExpectedUserCount)
	}
	for _, malformed := range []string{`{"expected_user_count":1.5}`, `{"expected_user_count":"25"}`} {
		if err := json.Unmarshal([]byte(malformed), &present); err == nil {
			t.Fatal("non-integer expected_user_count decoded")
		}
	}
	encoded, err := json.Marshal(Workload{})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"use_case":"","expected_context_tokens":null,"expected_concurrency":null,"expected_requests_per_minute":null,"latency_priority":null,"expected_monthly_requests":null}` {
		t.Fatal("nil team size changed the legacy workload encoding")
	}
}

func intPointer(value int) *int { return &value }
