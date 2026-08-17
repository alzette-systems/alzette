package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alzette/internal/ids"
	"alzette/internal/platform"
	"alzette/internal/secrets"
)

const (
	maximumProbeBody = 64 << 10
	probePrompt      = "Reply with the single word OK."
)

type Config struct {
	Store                platform.RollupStore
	Clock                func() time.Time
	NewID                func(string) (string, error)
	RollupInterval       time.Duration
	RollupLookback       time.Duration
	ProbesEnabled        bool
	AllowInsecureTargets bool
	AllowedSecretRefs    map[string]bool
	HTTPClient           *http.Client
}

type Worker struct {
	store                platform.RollupStore
	clock                func() time.Time
	newID                func(string) (string, error)
	rollupInterval       time.Duration
	rollupLookback       time.Duration
	probesEnabled        bool
	allowInsecureTargets bool
	allowedSecretRefs    map[string]bool
	httpClient           *http.Client
}

func New(config Config) (*Worker, error) {
	if config.Store == nil {
		return nil, errors.New("worker store is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.NewID == nil {
		config.NewID = ids.New
	}
	if config.RollupInterval == 0 {
		config.RollupInterval = time.Minute
	}
	if config.RollupLookback == 0 {
		config.RollupLookback = 48 * time.Hour
	}
	if config.RollupInterval < 10*time.Second || config.RollupLookback < time.Hour || config.RollupLookback > 31*24*time.Hour {
		return nil, errors.New("worker intervals are outside supported bounds")
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("probe redirects are disabled") }}
	}
	return &Worker{
		store: config.Store, clock: config.Clock, newID: config.NewID, rollupInterval: config.RollupInterval, rollupLookback: config.RollupLookback,
		probesEnabled: config.ProbesEnabled, allowInsecureTargets: config.AllowInsecureTargets,
		allowedSecretRefs: config.AllowedSecretRefs, httpClient: config.HTTPClient,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.RunOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(w.rollupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.RunOnce(ctx); err != nil && !errors.Is(err, platform.ErrConflict) {
				return err
			}
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	now := w.clock().UTC()
	from := now.Add(-w.rollupLookback).Truncate(time.Hour)
	if _, err := w.store.RefreshUsageRollups(ctx, from, now, now); err != nil {
		return err
	}
	if !w.probesEnabled {
		return nil
	}
	targets, err := w.store.ListProbeTargets(ctx, now)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if err := w.probe(ctx, target, now); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) probe(parent context.Context, target platform.ProbeTarget, observedAt time.Time) error {
	observation := platform.ProbeObservation{TargetID: target.ID, ObservedAt: observedAt, FreshUntil: observedAt.Add(target.ProbeInterval), Status: "unknown"}
	var err error
	observation.ID, err = w.newID("obs")
	if err != nil {
		return err
	}
	if !w.allowedSecretRefs[target.SecretRef] {
		observation.ErrorClass = "credential_unavailable"
		return w.store.RecordProbeObservation(parent, observation)
	}
	secret, ok := secrets.Lookup(target.SecretRef)
	if !ok || secret == "" {
		observation.ErrorClass = "credential_unavailable"
		return w.store.RecordProbeObservation(parent, observation)
	}
	endpoint, err := compatibleEndpoint(target.BaseURL, w.allowInsecureTargets)
	if err != nil {
		observation.Status = "degraded"
		observation.CredentialAvailable = true
		observation.ErrorClass = "invalid_probe_response"
		return w.store.RecordProbeObservation(parent, observation)
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"model": target.ProviderModel, "stream": false, "max_tokens": 1,
		"messages": []map[string]string{{"role": "user", "content": probePrompt}},
	})
	timeout := target.Timeout
	if timeout <= 0 || timeout > 15*time.Second {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Alzette-Probe", "compatible-chat-readiness")
	started := time.Now()
	response, err := w.httpClient.Do(request)
	observation.Latency = time.Since(started)
	observation.CredentialAvailable = true
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			observation.Status, observation.ErrorClass = "unavailable", "probe_timeout"
		} else {
			observation.Status, observation.ErrorClass = "unavailable", "probe_transport"
		}
		return w.store.RecordProbeObservation(parent, observation)
	}
	defer response.Body.Close()
	observation.HTTPStatus = response.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maximumProbeBody+1))
	if readErr != nil || len(body) > maximumProbeBody {
		observation.Status, observation.ErrorClass = "degraded", "invalid_probe_response"
		return w.store.RecordProbeObservation(parent, observation)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		observation.Status, observation.ErrorClass = "degraded", "probe_rejected"
		if response.StatusCode >= 500 {
			observation.Status, observation.ErrorClass = "unavailable", "probe_unavailable"
		}
		return w.store.RecordProbeObservation(parent, observation)
	}
	var compatible struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &compatible) != nil || compatible.ID == "" || strings.TrimSpace(compatible.Model) != strings.TrimSpace(target.ProviderModel) || len(compatible.Choices) == 0 || compatible.Choices[0].Message.Role != "assistant" {
		observation.Status, observation.ErrorClass = "degraded", "invalid_probe_response"
		return w.store.RecordProbeObservation(parent, observation)
	}
	observation.Status = "operational"
	return w.store.RecordProbeObservation(parent, observation)
}

func compatibleEndpoint(base string, allowInsecure bool) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid target")
	}
	if parsed.Scheme != "https" && !(allowInsecure && parsed.Scheme == "http") {
		return "", errors.New("insecure target")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	return parsed.String(), nil
}

func (w *Worker) String() string { return fmt.Sprintf("rollup worker interval=%s", w.rollupInterval) }
