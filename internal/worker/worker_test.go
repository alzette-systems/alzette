package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alzette/internal/platform"
)

type workerStoreStub struct {
	mu           sync.Mutex
	refreshes    int
	listCalls    int
	targets      []platform.ProbeTarget
	observations []platform.ProbeObservation
	recordErr    error
}

func (s *workerStoreStub) RefreshUsageRollups(context.Context, time.Time, time.Time, time.Time) (int64, error) {
	s.refreshes++
	return 0, nil
}
func (s *workerStoreStub) ListProbeTargets(context.Context, time.Time) ([]platform.ProbeTarget, error) {
	s.listCalls++
	return append([]platform.ProbeTarget(nil), s.targets...), nil
}
func (s *workerStoreStub) RecordProbeObservation(_ context.Context, value platform.ProbeObservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, value)
	return s.recordErr
}
func (*workerStoreStub) WorkerHealthy(context.Context, time.Time, time.Duration) error { return nil }

func TestWorkerProbeOptInGatesMakeNoOutboundRequest(t *testing.T) {
	var outbound atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { outbound.Add(1) }))
	defer server.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	target := platform.ProbeTarget{ID: "tgt", BaseURL: server.URL + "/v1", SecretRef: "WORKER_TEST_KEY", ProviderModel: "provider/model", Timeout: time.Second, ProbeInterval: time.Minute}

	t.Run("global probes off", func(t *testing.T) {
		store := &workerStoreStub{targets: []platform.ProbeTarget{target}}
		runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: false})
		if err != nil {
			t.Fatal(err)
		}
		if err := runner.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if outbound.Load() != 0 || store.listCalls != 0 || store.refreshes != 1 {
			t.Fatal("global probe-off gate made an outbound request")
		}
	})

	t.Run("no per-target opt in", func(t *testing.T) {
		store := &workerStoreStub{}
		runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := runner.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if outbound.Load() != 0 || store.listCalls != 1 || store.refreshes != 1 {
			t.Fatal("per-target probe-off gate made an outbound request")
		}
	})
}

func TestWorkerRecordsTargetFailureAndContinuesWithCompatibleProbe(t *testing.T) {
	t.Setenv("WORKER_TEST_KEY", "credential-neutral-fake-key")
	var firstCalls, secondCalls atomic.Int64
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"safe fake failure"}}`))
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls.Add(1)
		var body struct {
			Model     string              `json:"model"`
			MaxTokens int                 `json:"max_tokens"`
			Stream    bool                `json:"stream"`
			Messages  []map[string]string `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "provider/model" || body.MaxTokens != 1 || body.Stream || len(body.Messages) != 1 {
			http.Error(w, "bad compatible request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"probe-id","model":"provider/model","choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
	}))
	defer second.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &workerStoreStub{targets: []platform.ProbeTarget{
		{ID: "tgt_failed", BaseURL: first.URL + "/v1", SecretRef: "WORKER_TEST_KEY", ProviderModel: "provider/model", Timeout: time.Second, ProbeInterval: time.Minute},
		{ID: "tgt_ok", BaseURL: second.URL + "/v1", SecretRef: "WORKER_TEST_KEY", ProviderModel: "provider/model", Timeout: time.Second, ProbeInterval: time.Minute},
	}}
	sequence := 0
	runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, NewID: func(string) (string, error) { sequence++; return "obs_" + string(rune('0'+sequence)), nil }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: true, AllowInsecureTargets: true, AllowedSecretRefs: map[string]bool{"WORKER_TEST_KEY": true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if firstCalls.Load() != 1 || secondCalls.Load() != 1 || len(store.observations) != 2 {
		t.Fatal("target failure prevented a subsequent eligible probe")
	}
	if store.observations[0].Status != "unavailable" || store.observations[0].ErrorClass != "probe_unavailable" || store.observations[1].Status != "operational" || store.observations[1].ErrorClass != "" {
		t.Fatal("probe outcomes were not stored as safe metadata")
	}
}

func TestWorkerProbeRejectsMismatchedResponseModel(t *testing.T) {
	t.Setenv("WORKER_TEST_KEY", "credential-neutral-fake-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"probe-id","model":"provider/another-model","choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
	}))
	defer server.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &workerStoreStub{targets: []platform.ProbeTarget{{ID: "tgt", BaseURL: server.URL + "/v1", SecretRef: "WORKER_TEST_KEY", ProviderModel: "provider/model", Timeout: time.Second, ProbeInterval: time.Minute}}}
	runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, NewID: func(string) (string, error) { return "obs_mismatch", nil }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: true, AllowInsecureTargets: true, AllowedSecretRefs: map[string]bool{"WORKER_TEST_KEY": true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.observations) != 1 || store.observations[0].Status != "degraded" || store.observations[0].ErrorClass != "invalid_probe_response" {
		t.Fatal("probe marked a response for a different model operational")
	}
}

func TestWorkerIDOrStorageFailureStopsSafely(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	target := platform.ProbeTarget{ID: "tgt", BaseURL: "http://127.0.0.1:1/v1", SecretRef: "MISSING", ProviderModel: "provider/model", Timeout: time.Second, ProbeInterval: time.Minute}
	t.Run("identifier failure", func(t *testing.T) {
		store := &workerStoreStub{targets: []platform.ProbeTarget{target}}
		runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, NewID: func(string) (string, error) { return "", errors.New("id unavailable") }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := runner.RunOnce(context.Background()); err == nil || len(store.observations) != 0 {
			t.Fatal("identifier failure was ignored")
		}
	})
	t.Run("storage failure", func(t *testing.T) {
		store := &workerStoreStub{targets: []platform.ProbeTarget{target, target}, recordErr: errors.New("storage unavailable")}
		runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, NewID: func(string) (string, error) { return "obs", nil }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := runner.RunOnce(context.Background()); err == nil || len(store.observations) != 1 {
			t.Fatal("storage failure did not stop the worker")
		}
	})
}

func TestWorkerMissingCredentialRecordsUnknownWithoutOutboundCall(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &workerStoreStub{targets: []platform.ProbeTarget{{ID: "tgt", BaseURL: "https://provider.example.invalid/v1", SecretRef: "WORKER_MISSING_KEY", ProviderModel: "provider/model", Timeout: time.Second, ProbeInterval: time.Minute}}}
	runner, err := New(Config{Store: store, Clock: func() time.Time { return now }, NewID: func(string) (string, error) { return "obs_missing", nil }, RollupInterval: time.Minute, RollupLookback: time.Hour, ProbesEnabled: true, AllowedSecretRefs: map[string]bool{"WORKER_MISSING_KEY": true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.observations) != 1 || store.observations[0].Status != "unknown" || store.observations[0].ErrorClass != "credential_unavailable" || store.observations[0].CredentialAvailable {
		t.Fatal("missing credential was not recorded as safe unknown probe metadata")
	}
}

var _ platform.RollupStore = (*workerStoreStub)(nil)
